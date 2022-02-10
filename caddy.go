package guvnor

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const guvnorCaddyContainerName = "guvnor-caddy"

// ensureCaddy ensures a caddy container is running and configured to accept
// config at the expected path.
func (e *Engine) caddyInit(ctx context.Context) error {
	e.log.Debug("initializing caddy")
	res, err := e.docker.ContainerList(ctx, types.ContainerListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("name", guvnorCaddyContainerName),
		),
	})
	if err != nil {
		return err
	}

	if len(res) > 1 {
		return errors.New("multiple caddy containers")
	}

	// If there's only one caddy container, there's nothing for us to do
	if len(res) == 1 {
		e.log.Debug("caddy container already running, no action required")
		// TODO: We should check the health and global config options of caddy
		return nil
	}

	e.log.Debug("no caddy container detected, creating one")
	// This will not fetch unless it's not present in the local cache.
	image := e.config.Caddy.Image
	_, err = e.docker.ImagePull(
		ctx, image, types.ImagePullOptions{},
	)
	if err != nil {
		return err
	}

	createRes, err := e.docker.ContainerCreate(
		ctx,
		&container.Config{
			Image: image,
		},
		&container.HostConfig{
			NetworkMode: "host",
		},
		&network.NetworkingConfig{},
		nil,
		guvnorCaddyContainerName,
	)
	if err != nil {
		return err
	}
	e.log.Debug("created caddy container, starting",
		zap.String("image", image),
		zap.String("containerId", createRes.ID),
	)

	err = e.docker.ContainerStart(ctx, createRes.ID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	e.log.Debug("started caddy container")

	return nil
}

// caddyReconcileService sets up the appropriate routes in Caddy for a
// specific process/service
func (e *Engine) caddyReconcileService(ctx context.Context, serviceName string, processName string, ports []string) error {
	return nil
}
