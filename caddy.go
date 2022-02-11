package guvnor

import (
	"context"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const guvnorCaddyContainerName = "guvnor-caddy"

type CaddyManager struct {
	docker *client.Client
	log    *zap.Logger
	config CaddyConfig
}

// Init ensures a caddy container is running and configured to accept
// config at the expected path.
func (cm *CaddyManager) Init(ctx context.Context) error {
	cm.log.Debug("initializing caddy")
	res, err := cm.docker.ContainerList(ctx, types.ContainerListOptions{
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
		cm.log.Debug("caddy container already running, no action required")
		// TODO: We should check the health and global config options of caddy
		return nil
	}

	cm.log.Debug("no caddy container detected, creating one")
	// This will not fetch unless it's not present in the local cache.
	image := cm.config.Image
	pullStream, err := cm.docker.ImagePull(
		ctx, image, types.ImagePullOptions{},
	)
	if err != nil {
		return err
	}
	defer pullStream.Close()
	io.Copy(os.Stdout, pullStream)

	createRes, err := cm.docker.ContainerCreate(
		ctx,
		&container.Config{
			Image: image,
			Labels: map[string]string{
				managedLabel: "1",
			},
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
	cm.log.Debug("created caddy container, starting",
		zap.String("image", image),
		zap.String("containerId", createRes.ID),
	)

	err = cm.docker.ContainerStart(ctx, createRes.ID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	cm.log.Debug("started caddy container")

	return nil
}

// ConfigureProcess sets up the appropriate routes in Caddy for a
// specific process/service
func (cm *CaddyManager) ConfigureProcess(ctx context.Context, serviceName string, processName string, hostNames []string, ports []string) error {
	cm.log.Debug("configuring caddy for process",
		zap.String("service", serviceName),
		zap.String("process", processName),
		zap.Strings("hostnames", hostNames),
		zap.Strings("ports", ports),
	)
	return nil
}

func (cm *CaddyManager) PurgeProcess(ctx context.Context, serviceName string, processName string) error {
	return nil
}
