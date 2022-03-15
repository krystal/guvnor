package guvnor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
	"go.uber.org/zap"
	"golang.org/x/term"
)

// Useful references on interactive tasks:
// - https://github.com/docker/cli/blob/master/cli/command/container/run.go
// - https://github.com/docker/cli/blob/master/cli/command/container/hijack.go

type hijackStreamer struct {
	log    *zap.Logger
	stdin  io.ReadCloser
	stdout io.Writer

	hijacked types.HijackedResponse
}

// setRaw puts the terminal into raw mode. This enables more control, and
// prevents an "echoing" style effect where the user sees their own input twice
// when executing shell applications like `bash`.
//
// It returns a restore function that MUST be called once streaming from stdin
// has ended, or the user's terminal will be left in a borked state.
func (h *hijackStreamer) setRaw() (func(), error) {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}

	restoreTerm := func() {
		if err := term.Restore(int(os.Stdin.Fd()), oldState); err != nil {
			h.log.Error("failed to restore terminal", zap.Error(err))
		}
	}

	return restoreTerm, nil
}

// stream connects the hijacked response to the specified stdin/stdout and
// blocks until the connection goes away or the context is cancelled.
func (h *hijackStreamer) stream(ctx context.Context) error {
	restoreTerm, err := h.setRaw()
	if err != nil {
		return err
	}
	defer restoreTerm()

	stdinChan := make(chan error)
	go func() {
		_, err := io.Copy(h.hijacked.Conn, h.stdin)
		if err != nil {
			err = fmt.Errorf("streaming input: %w", err)
		}
		stdinChan <- err
	}()

	stdoutChan := make(chan error)
	go func() {
		_, err := io.Copy(h.stdout, h.hijacked.Reader)
		if err != nil {
			err = fmt.Errorf("streaming output: %w", err)
		}

		if err := h.hijacked.CloseWrite(); err != nil {
			h.log.Error("failed to send EOF", zap.Error(err))
		}
		stdoutChan <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-stdinChan:
		return err
	case err := <-stdoutChan:
		return err
	}
}

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

func (e *Engine) runTask(ctx context.Context, taskName string, task *ServiceTaskConfig, svc *ServiceConfig, injectEnv map[string]string) error {
	image := fmt.Sprintf(
		"%s:%s",
		svc.Defaults.Image,
		svc.Defaults.ImageTag,
	)
	if task.Image != "" {
		if task.ImageTag == "" {
			return errors.New(
				"imageTag must be specified when image specified",
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
		injectEnv,
		map[string]string{
			"GUVNOR_TASK":    taskName,
			"GUVNOR_SERVICE": svc.Name,
		},
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
		taskName,
		time.Now().Unix(),
	)

	user := svc.Defaults.User
	if task.User != "" {
		user = svc.Defaults.User
	}

	containerConfig := &container.Config{
		Cmd:   task.Command,
		Image: image,
		Env:   env,

		Tty:       task.Interactive,
		OpenStdin: task.Interactive,

		Labels: map[string]string{
			serviceLabel: svc.Name,
			taskLabel:    taskName,
			managedLabel: "1",
		},

		User: user,
	}
	hostConfig := &container.HostConfig{
		Mounts: mounts,
	}
	if task.Network.Mode.IsHost(svc.Defaults.Network.Mode) {
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

	return e.runTask(ctx, args.TaskName, &task, svc, nil)
}
