package guvnor

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

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

// findFreePort is pretty hacky way of finding ports but avoids storing state
// for now. We may want to replace this in future.
func findFreePort() (string, error) {
	a, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return "", err
	}

	l, err := net.ListenTCP("tcp", a)
	if err != nil {
		return "", err
	}
	defer l.Close()

	lAddr := l.Addr().(*net.TCPAddr)
	return strconv.Itoa(lAddr.Port), nil
}

func (e *Engine) Deploy(ctx context.Context, args DeployArgs) error {
	svc, err := e.loadServiceConfig(args.ServiceName)
	if err != nil {
		return err
	}

	svcState, err := e.state.LoadServiceState(svc.Name)
	if err != nil {
		return err
	}
	svcState.DeploymentID += 1
	svcState.LastDeployedAt = time.Now()

	if err := e.caddy.Init(ctx); err != nil {
		return err
	}

	deploymentID := svcState.DeploymentID
	for processName, process := range svc.Processes {
		e.log.Debug("deploying process",
			zap.String("process", processName),
			zap.String("service", svc.Name),
		)

		newPorts := []string{}
		for i := 0; i < int(process.Quantity); i++ {
			fullName := containerFullName(svc.Name, deploymentID, processName, i)
			e.log.Debug("deploying process instance",
				zap.String("process", processName),
				zap.String("service", svc.Name),
				zap.Int("i", i),
				zap.String("containerName", fullName),
			)

			image := fmt.Sprintf(
				"%s:%s",
				svc.Defaults.Image,
				svc.Defaults.ImageTag,
			)
			if err := e.pullImage(ctx, image); err != nil {
				return err
			}

			selectedPort, err := findFreePort()
			if err != nil {
				return err
			}

			// Merge default, process and guvnor provided environment
			env := mergeEnv(
				svc.Defaults.Env,
				process.Env,
				map[string]string{
					"PORT":              selectedPort,
					"GUVNOR_SERVICE":    svc.Name,
					"GUVNOR_PROCESS":    processName,
					"GUVNOR_DEPLOYMENT": fmt.Sprintf("%d", deploymentID),
				},
			)

			// Merge mounts and convert to docker API mounts
			mounts := []mount.Mount{}
			for _, mnt := range mergeMounts(
				svc.Defaults.Mounts, process.Mounts,
			) {
				mounts = append(mounts, mount.Mount{
					Type:   mount.TypeBind,
					Source: mnt.Host,
					Target: mnt.Container,
				})
			}

			portProtocolBinding := selectedPort + "/tcp"
			containerConfig := &container.Config{
				Cmd:   process.Command,
				Image: image,
				Env:   env,
				Labels: map[string]string{
					serviceLabel:    svc.Name,
					processLabel:    processName,
					deploymentLabel: fmt.Sprintf("%d", deploymentID),
					managedLabel:    "1",
				},
				ExposedPorts: nat.PortSet{},
			}
			hostConfig := &container.HostConfig{
				PortBindings: nat.PortMap{},
				RestartPolicy: container.RestartPolicy{
					Name: "always",
				},
				Mounts:     mounts,
				Privileged: process.Privileged,
			}
			if process.Network.Mode.IsHost() {
				hostConfig.NetworkMode = "host"
			} else {
				natPort := nat.Port(portProtocolBinding)
				hostConfig.PortBindings[natPort] = []nat.PortBinding{
					{
						HostPort: portProtocolBinding,
						HostIP:   "127.0.0.1",
					},
				}
				containerConfig.ExposedPorts[natPort] = struct{}{}
			}

			res, err := e.docker.ContainerCreate(
				ctx,
				containerConfig,
				hostConfig,
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

			newPorts = append(newPorts, selectedPort)

			// TODO: Verify it comes online
		}

		caddyBackendName := fmt.Sprintf("%s-%s", svc.Name, processName)
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
					filters.Arg("label", fmt.Sprintf("%s=%s", serviceLabel, svc.Name)),
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
					zap.String("service", svc.Name),
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

	return e.state.SaveServiceState(svc.Name, svcState)
}
