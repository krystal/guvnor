package guvnor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
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

	e.log.Info("creating container",
		zap.String("taskRun", fullName),
	)

	containerConfig := &container.Config{
		Cmd:   task.Command,
		Image: image,
		Env:   env,
		Labels: map[string]string{
			serviceLabel: svc.Name,
			taskLabel:    args.TaskName,
			managedLabel: "1",
		},
	}
	hostConfig := &container.HostConfig{
		Mounts: mounts,
	}
	if task.Network.Mode.IsHost() {
		hostConfig.NetworkMode = "host"
	}
	createRes, err := e.docker.ContainerCreate(
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

	e.log.Info("starting task run container",
		zap.String("taskRun", fullName),
	)
	err = e.docker.ContainerStart(ctx, createRes.ID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	waitChan, errChan := e.docker.ContainerWait(
		ctx, createRes.ID, container.WaitConditionNotRunning,
	)
	select {
	case err := <-errChan:
		if err != nil {
			return err
		}
	case <-waitChan:
	}

	e.log.Info("task run complete, fetching logs",
		zap.String("taskRun", fullName),
	)
	// TODO: Show these logs live
	logs, err := e.docker.ContainerLogs(ctx, createRes.ID,
		types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
		},
	)
	if err != nil {
		return err
	}

	// TODO: Pass this out so the CLI can handle it as it wants.
	_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, logs)
	if err != nil {
		return err
	}

	e.log.Info("deleting task run container",
		zap.String("taskRun", fullName),
	)
	return e.docker.ContainerRemove(ctx, createRes.ID, types.ContainerRemoveOptions{
		Force: true,
	})
}
