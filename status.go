package guvnor

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
)

type StatusArgs struct {
	ServiceName string
}

type ProcessContainer struct {
	ContainerName string
	ContainerID   string
	Status        string
}

type ProcessStatus struct {
	WantReplicas int
	Containers   []ProcessContainer
}

type StatusRes struct {
	DeploymentID int
	Processes    map[string]ProcessStatus
}

func (e *Engine) Status(
	ctx context.Context, args StatusArgs,
) (*StatusRes, error) {
	svc, err := e.loadServiceConfig(args.ServiceName)
	if err != nil {
		return nil, err
	}
	svcState, err := e.state.LoadServiceState(svc.Name)
	if err != nil {
		return nil, err
	}

	e.log.Debug("fetching container list for service")
	containers, err := e.docker.ContainerList(ctx, types.ContainerListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", fmt.Sprintf("%s=%s", serviceLabel, svc.Name)),
		),
	})
	if err != nil {
		return nil, err
	}

	processStatuses := map[string]ProcessStatus{}
	for processName, process := range svc.Processes {
		ps := ProcessStatus{
			WantReplicas: int(process.Quantity),
			Containers:   []ProcessContainer{},
		}

		for _, container := range containers {
			containerProcess, _ := container.Labels[processLabel]
			if containerProcess == processName {
				ps.Containers = append(ps.Containers, ProcessContainer{
					ContainerName: container.Names[0],
					ContainerID:   container.ID,
					Status:        container.State,
				})
			}
		}

		processStatuses[processName] = ps
	}

	return &StatusRes{
		DeploymentID: svcState.DeploymentID,
		Processes:    processStatuses,
	}, nil
}
