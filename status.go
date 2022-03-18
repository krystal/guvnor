package guvnor

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
)

type StatusArgs struct {
	ServiceName string
}

type ContainerStatus struct {
	ContainerName string
	ContainerID   string
	Status        string
}

type ProcessStatus struct {
	WantReplicas int
	Containers   []ContainerStatus
}

type StatusResult struct {
	DeploymentID   int
	LastDeployedAt time.Time
	Processes      ProcessStatuses
}

type ProcessStatuses map[string]ProcessStatus

func (ps ProcessStatuses) OrderedKeys() []string {
	keys := make([]string, 0, len(ps))
	for k := range ps {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	return keys
}

func (e *Engine) Status(
	ctx context.Context, args StatusArgs,
) (*StatusResult, error) {
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
			WantReplicas: process.GetQuantity(),
			Containers:   []ContainerStatus{},
		}

		for _, container := range containers {
			containerProcess := container.Labels[processLabel]
			if containerProcess == processName {
				ps.Containers = append(ps.Containers, ContainerStatus{
					ContainerName: container.Names[0],
					ContainerID:   container.ID,
					Status:        container.State,
				})
			}
		}

		processStatuses[processName] = ps
	}

	return &StatusResult{
		DeploymentID:   svcState.DeploymentID,
		LastDeployedAt: svcState.LastDeployedAt,
		Processes:      processStatuses,
	}, nil
}
