package guvnor

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"go.uber.org/zap"
)

func (e *Engine) Purge(ctx context.Context) error {
	e.log.Debug("purging all containers owned by guvnor")
	listToShutdown, err := e.docker.ContainerList(ctx, types.ContainerListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", managedLabel),
		),
	})
	if err != nil {
		return err
	}

	for _, containerToShutdown := range listToShutdown {
		e.log.Debug("purging container",
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

	return e.state.Purge()
}
