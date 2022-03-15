package guvnor

import (
	"context"
	"errors"
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
	"github.com/krystal/guvnor/state"
	"go.uber.org/zap"
)

type DeployArgs struct {
	ServiceName string
	Tag         string
}

type DeployRes struct {
	ServiceName  string
	DeploymentID int
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

// purgePreviousProcessDeployment destroys containers of a specific svc-process
// combination of a specific deployment ID. This allows us to clean up older
// deployments.
func (e *Engine) purgePreviousProcessDeployment(
	ctx context.Context,
	deploymentToPurgeID int,
	svc *ServiceConfig,
	processName string,
) error {
	e.log.Debug("purging previous process deployment",
		zap.String("process", processName),
		zap.String("service", svc.Name),
		zap.Int("deployment", deploymentToPurgeID),
	)
	listToShutdown, err := e.docker.ContainerList(ctx, types.ContainerListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", fmt.Sprintf("%s=%s", serviceLabel, svc.Name)),
			filters.Arg("label", fmt.Sprintf("%s=%s", processLabel, processName)),
			filters.Arg(
				"label",
				fmt.Sprintf("%s=%d", deploymentLabel, deploymentToPurgeID),
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

	return nil
}

func (e *Engine) deployServiceProcess(ctx context.Context, svc *ServiceConfig, svcState *state.ServiceState, processName string, process *ServiceProcessConfig) error {
	e.log.Debug("deploying process",
		zap.String("process", processName),
		zap.String("service", svc.Name),
	)

	deploymentID := svcState.DeploymentID

	newPorts := []string{}

	for i := 0; i < process.GetQuantity(); i++ {
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
		if process.Image != "" {
			if process.ImageTag == "" {
				return errors.New(
					"imageTag must be specified when image specified",
				)
			}
			image = fmt.Sprintf(
				"%s:%s",
				process.Image,
				process.ImageTag,
			)
		}
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
			User:         process.GetUser(),
		}
		hostConfig := &container.HostConfig{
			PortBindings: nat.PortMap{},
			RestartPolicy: container.RestartPolicy{
				Name: "always",
			},
			Mounts:     mounts,
			Privileged: process.Privileged,
		}
		if process.Network.Mode.IsHost(svc.Defaults.Network.Mode) {
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

			hostConfig.ExtraHosts = append(hostConfig.ExtraHosts,
				// host-gateway is a special argument that tells docker to insert
				// the IP of the host's gateway on the container network.
				"host.docker.internal:host-gateway",
			)
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

		if process.ReadyCheck != nil {
			if process.ReadyCheck.HTTP != nil {
				process.ReadyCheck.HTTP.Host = "localhost:" + selectedPort
			}
			if err := process.ReadyCheck.Wait(
				ctx, e.log.Named("ready"),
			); err != nil {
				return err
			}
		}
	}

	caddyBackendName := fmt.Sprintf("%s-%s", svc.Name, processName)
	if len(process.Caddy.Hostnames) > 0 {
		// Sync caddy configuration with new ports
		err := e.caddy.ConfigureBackend(
			ctx, caddyBackendName, process.Caddy.Hostnames, newPorts,
		)
		if err != nil {
			return err
		}
	} else {
		// Clear out any caddy config associated with this process
		err := e.caddy.DeleteBackend(ctx, caddyBackendName)
		if err != nil {
			return err
		}
	}

	// Shut down containers from previous generation
	if deploymentID > 1 {
		err := e.purgePreviousProcessDeployment(
			ctx, deploymentID-1, svc, processName,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) runCallbacks(
	ctx context.Context,
	svc *ServiceConfig,
	preDeploy bool,
	deploymentID int,
) error {
	var callbacks []string
	var stage string
	if preDeploy {
		callbacks = svc.Callbacks.PreDeployment
		stage = "PRE_DEPLOYMENT"
	} else {
		callbacks = svc.Callbacks.PostDeployment
		stage = "POST_DEPLOYMENT"
	}
	e.log.Info("running callbacks for deployment",
		zap.String("stage", stage),
		zap.Strings("callbacks", callbacks),
	)

	injectEnv := map[string]string{
		"GUVNOR_DEPLOYMENT": fmt.Sprintf("%d", deploymentID),
		"GUVNOR_CALLBACK":   stage,
	}

	for _, taskName := range callbacks {
		task := svc.Tasks[taskName]
		e.log.Info("running callback task",
			zap.String("task", taskName),
		)
		err := e.runTask(ctx, taskName, &task, svc, injectEnv)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) Deploy(ctx context.Context, args DeployArgs) (*DeployRes, error) {
	// Load config & state
	svc, err := e.loadServiceConfig(args.ServiceName)
	if err != nil {
		return nil, err
	}

	svcState, err := e.state.LoadServiceState(svc.Name)
	if err != nil {
		return nil, err
	}

	// Prepare state with values we will want to persist
	svcState.DeploymentID += 1
	svcState.LastDeployedAt = time.Now()
	// Default to failure, we will set to success if we make it to the end.
	svcState.DeploymentStatus = state.StatusFailure
	defer func() {
		if err := e.state.SaveServiceState(svc.Name, svcState); err != nil {
			e.log.Error("failed to persist service state", zap.Error(err))
		}
	}()

	if err := e.runCallbacks(ctx, svc, true, svcState.DeploymentID); err != nil {
		return nil, err
	}

	// Setup caddy
	if err := e.caddy.Init(ctx); err != nil {
		return nil, err
	}

	for processName, process := range svc.Processes {
		err = e.deployServiceProcess(ctx, svc, svcState, processName, &process)
		if err != nil {
			return nil, err
		}
	}

	if err := e.runCallbacks(ctx, svc, false, svcState.DeploymentID); err != nil {
		return nil, err
	}

	// TODO: Tidy up any processes/containers that may have been removed from
	// the spec.

	svcState.DeploymentStatus = state.StatusSuccess
	return &DeployRes{
		ServiceName:  svc.Name,
		DeploymentID: svcState.DeploymentID,
	}, nil
}
