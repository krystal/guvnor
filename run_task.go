package guvnor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
	"go.uber.org/zap"
)

type RunTaskArgs struct {
	ServiceName string
	TaskName    string
}

func (e *Engine) interactiveAttach(ctx context.Context, id string) (chan struct{}, error) {
	resp, err := e.docker.ContainerAttach(ctx, id, types.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return nil, err
	}

	hs := &hijackStreamer{
		log:      e.log,
		stdin:    os.Stdin,
		stdout:   os.Stdout,
		hijacked: resp,
	}

	doneChan := make(chan struct{})
	go func() {
		if err := hs.stream(ctx); err != nil {
			e.log.Error("failed in streaming interactive session", zap.Error(err))
		}
		resp.Close()
		close(doneChan)
	}()

	return doneChan, nil
}

func (e *Engine) runTask(ctx context.Context, task *ServiceTaskConfig, svc *ServiceConfig, injectEnv map[string]string) error {
	image, pull, err := task.GetImage()
	if err != nil {
		return err
	}
	if pull {
		if err = e.pullImage(ctx, image); err != nil {
			return err
		}
	}

	env := mergeEnv(
		svc.Defaults.Env,
		task.Env,
		injectEnv,
		map[string]string{
			"GUVNOR_TASK":    task.name,
			"GUVNOR_SERVICE": svc.Name,
		},
	)

	fullName := fmt.Sprintf(
		"%s-task-%s-%d",
		svc.Name,
		task.name,
		time.Now().Unix(),
	)

	containerConfig := &container.Config{
		Cmd:   task.Command,
		Image: image,
		Env:   env,

		Tty:       task.Interactive,
		OpenStdin: task.Interactive,

		Labels: map[string]string{
			serviceLabel: svc.Name,
			taskLabel:    task.name,
			managedLabel: "1",
		},

		User: task.GetUser(),
	}
	hostConfig := &container.HostConfig{
		Mounts: task.GetMounts(),
	}
	if task.GetNetworkMode() == NetworkModeHost {
		hostConfig.NetworkMode = "host"
	} else {
		hostConfig.ExtraHosts = append(hostConfig.ExtraHosts,
			// host-gateway is a special argument that tells docker to insert
			// the IP of the host's gateway on the container network.
			"host.docker.internal:host-gateway",
		)
	}

	e.log.Info("creating container",
		zap.String("taskRun", fullName),
	)
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

	var streamingDoneChan chan struct{}
	if task.Interactive {
		streamingDoneChan, err = e.interactiveAttach(ctx, createRes.ID)
		if err != nil {
			return err
		}
	}

	e.log.Info("starting task run container",
		zap.String("taskRun", fullName),
	)
	err = e.docker.ContainerStart(ctx, createRes.ID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	// Set initial TTY size
	if task.Interactive {
		if err := manageTTYSize(ctx, e.log, createRes.ID, e.docker); err != nil {
			e.log.Error("failed to manage TTY size", zap.Error(err))
		}
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

	if streamingDoneChan != nil {
		// Wait for the interactive streams to close up
		<-streamingDoneChan
	}

	if !task.Interactive {
		// TODO: Stream these logs live rather than fetching at the end.
		e.log.Info("task run complete, fetching logs",
			zap.String("taskRun", fullName),
		)

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
	}

	e.log.Info("deleting task run container",
		zap.String("taskRun", fullName),
	)
	return e.docker.ContainerRemove(ctx, createRes.ID, types.ContainerRemoveOptions{
		Force: true,
	})
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

	return e.runTask(ctx, &task, svc, nil)
}
