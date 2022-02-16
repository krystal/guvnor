package guvnor

import (
	"context"
	"io"
	"os"

	"github.com/docker/docker/api/types"
)

// pullImage will ensure that an image exists in the local store. This means
// it will not pull if it is already present.
func (e *Engine) pullImage(ctx context.Context, image string) error {
	pullStream, err := e.docker.ImagePull(
		ctx, image, types.ImagePullOptions{},
	)
	if err != nil {
		return err
	}
	defer pullStream.Close()

	if _, err := io.Copy(os.Stdout, pullStream); err != nil {
		return err
	}

	return nil
}
