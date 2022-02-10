package guvnor

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"go.uber.org/zap"
)

const (
	serviceLabel    = "io.k.guvnor/service"
	processLabel    = "io.k.guvnor/process"
	deploymentLabel = "io.k.guvnor/deployment"
)

type DeployConfig struct {
	ServiceName string
	Tag         string
}

func containerFullName(
	serviceName string,
	deploymentID int,
	processName string,
	count int,
) string {
	return fmt.Sprintf(
		"%s-%d-%s-%d",
		serviceName,
		deploymentID,
		processName,
		count,
	)
}

func mergeEnv(toMerge ...map[string]string) []string {
	outMap := map[string]string{}
	for _, mp := range toMerge {
		for k, v := range mp {
			outMap[k] = v
		}
	}

	outSlice := make([]string, 0, len(outMap))
	for k, v := range outMap {
		outSlice = append(outSlice, fmt.Sprintf("%s=%s", k, v))
	}

	return outSlice
}

func (e *Engine) Deploy(ctx context.Context, cfg DeployConfig) error {
	svcName := cfg.ServiceName
	if svcName == "" {
		var err error
		svcName, err = findDefaultService(e.config.Paths.Config)
		if err != nil {
			return err
		}
		e.log.Debug(
			"no service name provided, defaulting",
			zap.String("default", svcName),
		)
	}

	svcCfg, err := loadServiceConfig(e.config.Paths.Config, svcName)
	if err != nil {
		return err
	}
	e.log.Debug("svcCfg", zap.Any("cfg", svcCfg))

	if err := e.caddyInit(ctx); err != nil {
		return err
	}

	deploymentID := 4 // TODO: Fetch/store this
	for processName, process := range svcCfg.Processes {
		e.log.Debug("deploying process",
			zap.String("process", processName),
			zap.String("service", svcName),
		)

		for i := 0; i < int(process.Quantity); i++ {
			e.log.Debug("deploying process instance",
				zap.String("process", processName),
				zap.String("service", svcName),
				zap.Int("i", i),
			)
			fullName := containerFullName(svcName, deploymentID, processName, i)

			image := fmt.Sprintf(
				"%s:%s",
				svcCfg.Defaults.Image,
				svcCfg.Defaults.ImageTag,
			)

			// This will not fetch unless it's not present in the local cache.
			_, err = e.docker.ImagePull(
				ctx, image, types.ImagePullOptions{},
			)
			if err != nil {
				return err
			}

			env := mergeEnv(
				svcCfg.Defaults.Env,
				process.Env,
				map[string]string{
					"PORT":              "", // TODO: Insert port
					"GUVNOR_SERVICE":    svcName,
					"GUVNOR_PROCESS":    processName,
					"GUVNOR_DEPLOYMENT": fmt.Sprintf("%s", deploymentID),
				},
			)

			res, err := e.docker.ContainerCreate(
				ctx,
				&container.Config{
					Cmd:   process.Command,
					Image: image,
					Env:   env,
					Labels: map[string]string{
						serviceLabel:    svcName,
						processLabel:    processName,
						deploymentLabel: fmt.Sprintf("%d", deploymentID),
					},
				},
				&container.HostConfig{},
				&network.NetworkingConfig{},
				nil,
				fullName,
			)
			if err != nil {
				return err
			}

			err = e.docker.ContainerStart(
				ctx, res.ID, types.ContainerStartOptions{},
			)
			if err != nil {
				return err
			}

			// TODO: Verify it comes online
		}

		if len(process.Caddy.Hostnames) > 0 {
			// TODO: Point caddy at new containers
		} else {
			// TODO: Ensure process is no longer listed in caddy
		}

		// Shut down containers from previous generation
		if deploymentID > 1 {
			e.log.Debug("removing previous deployment containers",
				zap.String("process", processName),
				zap.String("service", svcName),
			)
			listToShutdown, err := e.docker.ContainerList(ctx, types.ContainerListOptions{
				All: true,
				Filters: filters.NewArgs(
					filters.Arg("label", fmt.Sprintf("%s=%s", serviceLabel, svcName)),
					filters.Arg("label", fmt.Sprintf("%s=%s", processLabel, processName)),
					filters.Arg(
						"label",
						fmt.Sprintf("%s=%d", deploymentLabel, deploymentID-1),
					),
				),
			})
			if err != nil {
				return err
			}

			for _, containerToShutdown := range listToShutdown {
				err = e.docker.ContainerRemove(
					ctx,
					containerToShutdown.ID,
					types.ContainerRemoveOptions{
						Force: true,
					},
				)
				if err != nil {
					return err
				}
			}
		}
	}

	// TODO: Tidy up any processes/containers that may have been removed from
	// the spec.

	return nil
}
