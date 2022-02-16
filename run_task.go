package guvnor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"go.uber.org/zap"
)

type RunTaskArgs struct {
	ServiceName string
	TaskName    string
}

func (e *Engine) RunTask(ctx context.Context, args RunTaskArgs) error {
	svc, err := e.loadServiceConfig(args.ServiceName)
	if err != nil {
		return err
	}

	task, ok := svc.Tasks[args.TaskName]
	if !ok {
		return errors.New("specified task cannot be found in config")
	}

	if task.Interactive {
		// TODO: support interactive :)
		return errors.New("interactive not yet supported")
	}

	e.log.Debug("loaded task", zap.Strings("cmd", task.Command))

	image := fmt.Sprintf(
		"%s:%s",
		svc.Defaults.Image,
		svc.Defaults.ImageTag,
	)
	if task.Image != "" {
		if task.ImageTag == "" {
			return errors.New(
				"imageTag must be specified for task when image specified",
			)
		}
		image = fmt.Sprintf(
			"%s:%s",
			task.Image,
			task.ImageTag,
		)
	}

	if err := e.pullImage(ctx, image); err != nil {
		return err
	}

	env := mergeEnv(
		svc.Defaults.Env,
		task.Env,
	)

	mounts := []mount.Mount{}
	for _, mnt := range mergeMounts(
		svc.Defaults.Mounts, task.Mounts,
	) {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: mnt.Host,
			Target: mnt.Container,
		})
	}

	fullName := fmt.Sprintf(
		"%s-task-%s-%d",
		svc.Name,
		args.TaskName,
		time.Now().Unix(),
	)

	_, err = e.docker.ContainerCreate(
		ctx,
		&container.Config{
			Cmd:   task.Command,
			Image: image,
			Env:   env,
			Labels: map[string]string{
				serviceLabel: svc.Name,
				taskLabel:    args.TaskName,
				managedLabel: "1",
			},
		},
		&container.HostConfig{
			Mounts: mounts,
		},
		&network.NetworkingConfig{},
		nil,
		fullName,
	)
	if err != nil {
		return err
	}

	return nil
}
