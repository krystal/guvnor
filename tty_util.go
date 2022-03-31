package guvnor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
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

func updateTTYSize(ctx context.Context, ID string, client client.ContainerAPIClient) error {
	w, h, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("getting terminal size: %w", err)
	}

	return client.ContainerResize(ctx, ID, types.ResizeOptions{
		Width:  uint(w),
		Height: uint(h),
	})
}

func manageTTYSize(ctx context.Context, log *zap.Logger, ID string, client client.ContainerAPIClient) error {
	err := updateTTYSize(ctx, ID, client)
	if err != nil {
		return err
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, unix.SIGWINCH)
	go func() {
		defer signal.Stop(sigs)
		for {
			select {
			case <-ctx.Done():
				return
			case <-sigs:
				err := updateTTYSize(ctx, ID, client)
				if err != nil {
					log.Error("failed to update tty size", zap.Error(err))
				}
			}
		}
	}()

	return nil
}
