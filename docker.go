package guvnor

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/registry"
	"go.uber.org/zap"
)

type dockerAuthConfig struct {
	Auths map[string]types.AuthConfig `json:"auths"`
}

func getIndexForImage(image string) (string, error) {
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return "", err
	}

	reg, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return "", err
	}

	return reg.Index.Name, nil
}

func extractAuthCfg(configBytes []byte, index string) (*types.AuthConfig, error) {
	// Useful excerpts from dockers source code:
	// https://github.com/docker/cli/blob/ae3b0b34c838ff37ead3b43a6081d33656a57c07/cli/config/configfile/file.go#L119
	// https://github.com/docker/cli/blob/ae3b0b34c838ff37ead3b43a6081d33656a57c07/cli/config/configfile/file.go#L274

	dockerCfg := &dockerAuthConfig{}
	if err := json.Unmarshal(configBytes, dockerCfg); err != nil {
		return nil, err
	}

	authCfg, ok := dockerCfg.Auths[index]
	if !ok {
		return nil, errors.New("no auth configured for registry")
	}

	authCfg.ServerAddress = index

	// If "auth" field is provided, then we can extract username and password from this field
	if authCfg.Auth != "" {
		authBytes, err := base64.StdEncoding.DecodeString(authCfg.Auth)
		if err != nil {
			return nil, err
		}

		arr := strings.Split(string(authBytes), ":")
		if len(arr) != 2 {
			return nil, fmt.Errorf("auth string malformed, expected 2 parts")
		}

		authCfg.Username = arr[0]
		authCfg.Password = arr[1]
		authCfg.Auth = ""
	}

	if authCfg.Username == "" {
		return nil, fmt.Errorf(
			"no auth options provided for registry",
		)
	}

	return &authCfg, nil
}

func authStringForImage(image string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configPath := path.Join(home, "/.docker/config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	indexName, err := getIndexForImage(image)
	if err != nil {
		return "", err
	}

	authConfig, err := extractAuthCfg(data, indexName)
	if err != nil {
		return "", err
	}

	outBytes, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(outBytes), nil
}

// pullImage will ensure that an image exists in the local store. This means
// it will not pull if it is already present.
func (e *Engine) pullImage(ctx context.Context, image string) error {
	authStr, err := authStringForImage(image)
	if err != nil {
		e.log.Info(
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
