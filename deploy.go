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

func (e *Engine) updateLoadbalancerForDeployment(ctx context.Context, svcName, processName string, process *ServiceProcessConfig, containers []deployedProcessContainer) error {
	caddyBackendName := fmt.Sprintf("%s-%s", svcName, processName)
	ports := []string{}
	for _, container := range containers {
		if container.Port != "" {
			ports = append(ports, container.Port)
		}
	}

	return e.caddy.ConfigureBackend(
		ctx, caddyBackendName, process.Caddy.Hostnames, ports,
	)
}

func (e *Engine) getLastDeploymentContainers(ctx context.Context, svc, process string, deploymentID int) ([]deployedProcessContainer, error) {
	dockerContainers, err := e.docker.ContainerList(ctx, types.ContainerListOptions{
		Filters: filters.NewArgs(
			filters.Arg(
				"label",
				fmt.Sprintf("%s=%s", serviceLabel, svc),
			),
			filters.Arg(
				"label",
				fmt.Sprintf("%s=%s", processLabel, process),
			),
			filters.Arg(
				"label",
				fmt.Sprintf("%s=%d", deploymentLabel, deploymentID-1),
			),
		),
	})
	if err != nil {
		return nil, err
	}

	deployedContainers := []deployedProcessContainer{}
	for _, container := range dockerContainers {
		deployedContainers = append(deployedContainers, deployedProcessContainer{
			ID:   container.ID,
			Name: container.Names[0],
			Port: container.Labels[portLabel],
		})
	}

	return deployedContainers, nil
}

func (e *Engine) startContainerForProcess(ctx context.Context, i int, processName string, svc *ServiceConfig, process *ServiceProcessConfig, deploymentID int, image string) (*deployedProcessContainer, error) {
	fullName := containerFullName(svc.Name, deploymentID, processName, i)
	selectedPort, err := findFreePort()
	if err != nil {
		return nil, err
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
			portLabel:       selectedPort,
		},
		ExposedPorts: nat.PortSet{},
		User:         process.GetUser(),
	}
	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{},
		RestartPolicy: container.RestartPolicy{
			Name: "always",
		},
		Mounts:     process.GetMounts(),
		Privileged: process.Privileged,
	}
	if process.GetNetworkMode() == NetworkModeHost {
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
		return nil, err
	}

	err = e.docker.ContainerStart(
		ctx, res.ID, types.ContainerStartOptions{},
	)
	if err != nil {
		return nil, err
	}

	inspect, err := e.docker.ContainerInspect(ctx, res.ID)
	if err != nil {
		return nil, err
	}

	return &deployedProcessContainer{
		ID:   inspect.ID,
		Name: inspect.Name,
		Port: selectedPort,
	}, nil
}

type deployedProcessContainer struct {
	ID   string
	Name string
	Port string
}

// TODO: It would be nice to extract these out and make them part of the
// Strategy type to try and curtail the growth of this package
func (e *Engine) deployServiceProcessDefaultStrategy(
	ctx context.Context,
	i int,
	processName string,
	svc *ServiceConfig,
	process *ServiceProcessConfig,
	deploymentID int,
	image string,
	lastDeploymentContainers *[]deployedProcessContainer,
	newDeploymentContainers *[]deployedProcessContainer,
) error {
	container, err := e.startContainerForProcess(
		ctx, i, processName, svc, process, deploymentID, image,
	)
	if err != nil {
		return err
	}
	*newDeploymentContainers = append(*newDeploymentContainers, *container)

	// Ensure new container is ready
	if process.ReadyCheck != nil {
		if process.ReadyCheck.HTTP != nil {
			process.ReadyCheck.HTTP.Host = "localhost:" + container.Port
		}
		if err := process.ReadyCheck.Wait(
			ctx, e.log.Named("ready"),
		); err != nil {
			return err
		}
	}

	var containerToReplace *deployedProcessContainer
	if len(*lastDeploymentContainers) > 0 {
		containerToReplace = &(*lastDeploymentContainers)[0]
		*lastDeploymentContainers = (*lastDeploymentContainers)[1:]
		e.log.Debug("new container will replace old container",
			zap.String("process", processName),
			zap.String("service", svc.Name),
			zap.String("oldContainer", containerToReplace.Name),
		)
	}

	// Add new healthy container to load balancer, replacing the old container
	if len(process.Caddy.Hostnames) > 0 {
		e.log.Debug("updating loadbalancer with new container",
			zap.String("process", processName),
			zap.String("service", svc.Name),
		)
		// Sync caddy configuration with new ports
		err := e.updateLoadbalancerForDeployment(
			ctx,
			svc.Name,
			processName,
			process,
			append(*lastDeploymentContainers, *newDeploymentContainers...),
		)
		if err != nil {
			return err
		}
	}

	// Shutdown old container
	if containerToReplace != nil {
		e.log.Debug("sending SIGTERM to old container",
			zap.String("process", processName),
			zap.String("service", svc.Name),
			zap.String("oldContainer", containerToReplace.Name),
		)
		err = e.docker.ContainerKill(ctx, containerToReplace.ID, "SIGTERM")
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) deployServiceProcessReplaceStrategy(
	ctx context.Context,
	i int,
	processName string,
	svc *ServiceConfig,
	process *ServiceProcessConfig,
	deploymentID int,
	image string,
	lastDeploymentContainers *[]deployedProcessContainer,
	newDeploymentContainers *[]deployedProcessContainer,
) error {
	// Determine if theres an old container to remove
	var containerToReplace *deployedProcessContainer
	if len(*lastDeploymentContainers) > 0 {
		containerToReplace = &(*lastDeploymentContainers)[0]
		*lastDeploymentContainers = (*lastDeploymentContainers)[1:]
		e.log.Debug("new container will replace old container",
			zap.String("process", processName),
			zap.String("service", svc.Name),
			zap.String("oldContainer", containerToReplace.Name),
		)
	}

	// Remove old container from loadbalancer and shut it down
	if containerToReplace != nil {
		if len(process.Caddy.Hostnames) > 0 {
			e.log.Debug("removing old container from load balancer",
				zap.String("process", processName),
				zap.String("service", svc.Name),
			)
			// Sync caddy configuration with new ports
			err := e.updateLoadbalancerForDeployment(
				ctx,
				svc.Name,
				processName,
				process,
				append(*lastDeploymentContainers, *newDeploymentContainers...),
			)
			if err != nil {
				return err
			}
		}
		e.log.Debug("killing old container, will wait",
			zap.String("process", processName),
			zap.String("service", svc.Name),
		)

		duration := time.Second * time.Duration(10)
		err := e.docker.ContainerStop(
			ctx,
			containerToReplace.ID,
			&duration,
		)
		if err != nil {
			return err
		}

	}

	container, err := e.startContainerForProcess(
		ctx, i, processName, svc, process, deploymentID, image,
	)
	if err != nil {
		return err
	}
	*newDeploymentContainers = append(*newDeploymentContainers, *container)

	// Ensure new container is ready
	if process.ReadyCheck != nil {
		if process.ReadyCheck.HTTP != nil {
			process.ReadyCheck.HTTP.Host = "localhost:" + container.Port
		}
		if err := process.ReadyCheck.Wait(
			ctx, e.log.Named("ready"),
		); err != nil {
			return err
		}
	}

	// Add new healthy container to load balancer
	if len(process.Caddy.Hostnames) > 0 {
		e.log.Debug("updating loadbalancer with new container",
			zap.String("process", processName),
			zap.String("service", svc.Name),
		)
		// Sync caddy configuration with new ports
		err := e.updateLoadbalancerForDeployment(
			ctx,
			svc.Name,
			processName,
			process,
			append(*lastDeploymentContainers, *newDeploymentContainers...),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) deployServiceProcess(
	ctx context.Context,
	svc *ServiceConfig,
	svcState *state.ServiceState,
	processName string,
	process *ServiceProcessConfig,
) error {
	e.log.Debug("deploying process",
		zap.String("process", processName),
		zap.String("service", svc.Name),
	)

	deploymentID := svcState.DeploymentID

	// Get containers from last deployment so we can replace them.
	var err error
	lastDeploymentContainers := []deployedProcessContainer{}
	newDeploymentContainers := []deployedProcessContainer{}
	if svcState.DeploymentID > 1 {
		lastDeploymentContainers, err = e.getLastDeploymentContainers(
			ctx, svc.Name, processName, deploymentID,
		)
		if err != nil {
			return err
		}
	}

	// Calculate and pull image for new containers
	image, err := process.GetImage()
	if err != nil {
		return err
	}
	if err := e.pullImage(ctx, image); err != nil {
		return err
	}

	for i := 0; i < process.GetQuantity(); i++ {
		e.log.Debug("deploying process instance",
			zap.String("process", processName),
			zap.String("service", svc.Name),
		)

		switch process.DeploymentStrategy {
		case DefaultStrategy:
			err := e.deployServiceProcessDefaultStrategy(
				ctx,
				i,
				processName,
				svc,
				process,
				deploymentID,
				image,
				&lastDeploymentContainers,
				&newDeploymentContainers,
			)
			if err != nil {
				return err
			}
		case ReplaceStrategy:
			err := e.deployServiceProcessReplaceStrategy(
				ctx,
				i,
				processName,
				svc,
				process,
				deploymentID,
				image,
				&lastDeploymentContainers,
				&newDeploymentContainers,
			)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf(
				"unknown strategy '%s'", process.DeploymentStrategy,
			)
		}

	}

	// Perform a full reconciliation of the Caddy configuration with just the
	// new containers, this removes any replicas that have not been replaced
	// during the roll out when the new deployment has less replicas
	if len(process.Caddy.Hostnames) > 0 {
		e.log.Debug("performing full reconciliation of process loadbalancer",
			zap.String("process", processName),
			zap.String("service", svc.Name),
		)
		// Sync caddy configuration with new ports
		err := e.updateLoadbalancerForDeployment(
			ctx,
			svc.Name,
			processName,
			process,
			newDeploymentContainers,
		)
		if err != nil {
			return err
		}
	}

	// Clean up any remaining containers from the last deployment that were
	// not replaced during the roll out. This deals with cases where the
	// replica count has decreased in the new deployment.
	for _, oldContainer := range lastDeploymentContainers {
		e.log.Debug("shutting down previous deployment container",
			zap.String("process", processName),
			zap.String("service", svc.Name),
			zap.String("container", oldContainer.Name),
		)
		err = e.docker.ContainerKill(ctx, oldContainer.ID, "SIGTERM")
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
