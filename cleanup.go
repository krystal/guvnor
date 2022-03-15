package guvnor

import (
	"context"
	"fmt"
	"strconv"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"go.uber.org/zap"
)

type CleanupArgs struct {
	ServiceName string
}

func (e *Engine) Cleanup(ctx context.Context, args CleanupArgs) error {
	svc, err := e.loadServiceConfig(args.ServiceName)
	if err != nil {
		return err
	}

	svcState, err := e.state.LoadServiceState(svc.Name)
	if err != nil {
		return err
	}

	e.log.Debug(
		"finding process containers for service",
		zap.String("service", svc.Name),
	)
	serviceFilter := fmt.Sprintf("%s=%s", serviceLabel, svc.Name)
	serviceProcessContainers, err := e.docker.ContainerList(
		ctx,
		types.ContainerListOptions{
			All: true,
			Filters: filters.NewArgs(
				filters.Arg("label", managedLabel),
				filters.Arg("label", serviceFilter),
				// Ensure they are affiliated with a process
				filters.Arg("label", processLabel),
			),
		},
	)
	if err != nil {
		return err
	}

	e.log.Debug(
		"found process containers for service",
		zap.String("service", svc.Name),
		zap.Int("count", len(serviceProcessContainers)),
	)
	deleteCount := 0
	for _, container := range serviceProcessContainers {
		deploy, ok := container.Labels[deploymentLabel]
		if !ok {
			continue
		}

		if deploy != strconv.Itoa(svcState.DeploymentID) {
			e.log.Debug(
				"zombie container found; removing",
				zap.String("service", svc.Name),
				zap.String("container", container.ID),
			)
			err = e.docker.ContainerRemove(
				ctx,
				container.ID,
				types.ContainerRemoveOptions{
					Force: true,
				},
			)
			if err != nil {
				return err
			}
			deleteCount++
		}
	}
	e.log.Debug("deleted zombie containers",
		zap.String("service", svc.Name),
		zap.Int("count", deleteCount),
	)

	return nil
}
