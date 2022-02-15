package guvnor

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"go.uber.org/zap"
)

type DeployArgs struct {
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
		"%s-%s-%d-%d",
		serviceName,
		processName,
		deploymentID,
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

func mergeMounts(a, b []ServiceMountConfig) []ServiceMountConfig {
	out := make([]ServiceMountConfig, 0, len(a)+len(b))
	out = append(out, a...)
	out = append(out, b...)

	return out
}

func (e *Engine) Deploy(ctx context.Context, args DeployArgs) error {
	svcName := args.ServiceName
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

	svcState, err := e.state.LoadServiceState(svcName)
	if err != nil {
		return err
	}
	svcState.DeploymentID += 1

	if err := e.caddy.Init(ctx); err != nil {
		return err
	}

	deploymentID := svcState.DeploymentID
	for processName, process := range svcCfg.Processes {
		e.log.Debug("deploying process",
			zap.String("process", processName),
			zap.String("service", svcName),
		)

		newPorts := []string{} // TODO: Fetch these as we create containers
		for i := 0; i < int(process.Quantity); i++ {
			fullName := containerFullName(svcName, deploymentID, processName, i)
			e.log.Debug("deploying process instance",
				zap.String("process", processName),
				zap.String("service", svcName),
				zap.Int("i", i),
				zap.String("containerName", fullName),
			)

			image := fmt.Sprintf(
				"%s:%s",
				svcCfg.Defaults.Image,
				svcCfg.Defaults.ImageTag,
			)

			// Pulls the image if not already in the local cache
			pullStream, err := e.docker.ImagePull(
				ctx, image, types.ImagePullOptions{},
			)
			if err != nil {
				return err
			}
			defer pullStream.Close()
			if _, err := io.Copy(os.Stdout, pullStream); err != nil {
				return err
			}

			containerPort := "9000"

			// Merge default, process and guvnor provided environment
			env := mergeEnv(
				svcCfg.Defaults.Env,
				process.Env,
				map[string]string{
					"PORT":              containerPort,
					"GUVNOR_SERVICE":    svcName,
					"GUVNOR_PROCESS":    processName,
					"GUVNOR_DEPLOYMENT": fmt.Sprintf("%d", deploymentID),
				},
			)

			// Merge mounts and convert to docker API mounts
			mounts := []mount.Mount{}
			for _, mnt := range mergeMounts(
				svcCfg.Defaults.Mounts, process.Mounts,
			) {
				mounts = append(mounts, mount.Mount{
					Type:   mount.TypeBind,
					Source: mnt.Host,
					Target: mnt.Container,
				})
			}

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
						managedLabel:    "1",
					},
					ExposedPorts: nat.PortSet{
						nat.Port(containerPort + "/tcp"): struct{}{},
					},
				},
				&container.HostConfig{
					// TODO: Don't use port bindings if host networking mode
					PortBindings: nat.PortMap{
						nat.Port(containerPort + "/tcp"): []nat.PortBinding{
							{
								// Right now, select a random port on the host.
								// Eventually we need to pre-select this in
								// order to allow host networking to work
								// nicely :3
								HostPort: "0",
								HostIP:   "127.0.0.1",
							},
						},
					},
					Mounts: mounts,
				},
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

			inspectRes, err := e.docker.ContainerInspect(ctx, res.ID)
			if err != nil {
				return err
			}

			randomHostPort := inspectRes.NetworkSettings.Ports[nat.Port(containerPort+"/tcp")][0].HostPort
			newPorts = append(newPorts, randomHostPort)

			// TODO: Verify it comes online
		}

		caddyBackendName := fmt.Sprintf("%s-%s", svcName, processName)
		if len(process.Caddy.Hostnames) > 0 {
			// Sync caddy configuration with new ports
			err = e.caddy.ConfigureBackend(
				ctx, caddyBackendName, process.Caddy.Hostnames, newPorts,
			)
			if err != nil {
				return err
			}
		} else {
			// Clear out any caddy config associated with this process
			err = e.caddy.DeleteBackend(ctx, caddyBackendName)
			if err != nil {
				return err
			}
		}

		// Shut down containers from previous generation
		if deploymentID > 1 {
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
				e.log.Debug("removing previous deployment container",
					zap.String("process", processName),
					zap.String("service", svcName),
					zap.String("container", containerToShutdown.ID),
				)
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

	return e.state.SaveServiceState(svcName, svcState)
}
