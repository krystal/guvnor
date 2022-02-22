package guvnor

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/registry"
	"go.uber.org/zap"
)

type dockerAuthConfig struct {
	Auths map[string]types.AuthConfig `json:"auths"`
}

func loadCredentialsFromDockerConfig(image string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configPath := path.Join(home, "/.docker/config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	dockerConf := &dockerAuthConfig{}
	if err := json.Unmarshal(data, dockerConf); err != nil {
		return "", err
	}

	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return "", err
	}

	reg, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return "", err
	}

	// TODO: Parse correct registry from the image
	registryAuth, ok := dockerConf.Auths[reg.Index.Name]
	if !ok || registryAuth.Auth == "" {
		return "", errors.New("no auth configured")
	}

	outBytes, err := json.Marshal(registryAuth)
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(outBytes), nil
}

// pullImage will ensure that an image exists in the local store. This means
// it will not pull if it is already present.
func (e *Engine) pullImage(ctx context.Context, image string) error {
	authStr, err := loadCredentialsFromDockerConfig(image)
	if err != nil {
		e.log.Warn(
			"could not load docker credentials, using no auth",
			zap.String("reason", err.Error()),
		)
	}

	pullStream, err := e.docker.ImagePull(
		ctx, image, types.ImagePullOptions{
			RegistryAuth: authStr,
		},
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
