package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	guvnorCaddyContainerName = "guvnor-caddy"
	guvnorServerName         = "guvnor"
)

type Config struct {
	// Image is the container image that should be deployed as caddy
	Image string      `yaml:"image"`
	ACME  ACMEConfig  `yaml:"acme"`
	Ports PortsConfig `yaml:"ports"`
}

type ACMEConfig struct {
	// CA is the URL of the ACME service.
	CA string `yaml:"ca"`
	// Email is the address that should be provided to the acme service for
	// contacting us.
	Email string `yaml:"email"`
}

type PortsConfig struct {
	HTTP  uint `yaml:"http"`
	HTTPS uint `yaml:"https"`
}

type Manager struct {
	Docker          *client.Client
	Log             *zap.Logger
	Config          Config
	ContainerLabels map[string]string
}

// Init ensures a caddy container is running and configured to accept
// config at the expected path.
func (cm *Manager) Init(ctx context.Context) error {
	cm.Log.Debug("initializing caddy")
	res, err := cm.Docker.ContainerList(ctx, types.ContainerListOptions{
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
		cm.Log.Debug("caddy container already running, no action required")
		// TODO: We should check the health and global config options of caddy
		return nil
	}

	cm.Log.Debug("no caddy container detected, creating one")
	// This will not fetch unless it's not present in the local cache.
	image := cm.Config.Image
	pullStream, err := cm.Docker.ImagePull(
		ctx, image, types.ImagePullOptions{},
	)
	if err != nil {
		return err
	}
	defer pullStream.Close()
	io.Copy(os.Stdout, pullStream)

	createRes, err := cm.Docker.ContainerCreate(
		ctx,
		&container.Config{
			Image:  image,
			Labels: cm.ContainerLabels,
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
	cm.Log.Debug("created caddy container, starting",
		zap.String("image", image),
		zap.String("containerId", createRes.ID),
	)

	err = cm.Docker.ContainerStart(ctx, createRes.ID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	cm.Log.Debug("started caddy container")

	// Give caddy time to start..
	// TODO: Detect caddy coming online
	time.Sleep(1 * time.Second)

	// TODO: actually build this config from structs
	defaultConfig := fmt.Sprintf(
		`{"apps":{"http":{"servers":{"%s":{"listen":[":80"],"routes":[]}}}}}`,
		guvnorServerName,
	)

	err = cm.adminRequest(ctx, http.MethodPost, &url.URL{Path: "config/"}, defaultConfig, nil)
	if err != nil {
		return err
	}

	return nil
}

type apiRoute struct {
	Match  []apiMatch  `json:"match`
	Handle []apiHandle `json:"handle"`
}

type apiMatch struct {
	Host []string `json:"host"`
}

type apiHandle struct {
	Handler   string        `json:"handler"`
	Upstreams []apiUpstream `json:"upstreams`
}

type apiUpstream struct {
	Dial string `json:"dial"`
}

// ConfigureBackend sets up the appropriate routes in Caddy for a
// specific process/service
func (cm *Manager) ConfigureBackend(ctx context.Context, backendName string, hostNames []string, ports []string) error {
	cm.Log.Debug("configuring caddy for process",
		zap.String("backend", backendName),
		zap.Strings("hostnames", hostNames),
		zap.Strings("ports", ports),
	)
	// Fetch current config
	currentRoutes := []apiRoute{}
	routesConfigPath := fmt.Sprintf(
		"config/apps/http/servers/%s/routes",
		guvnorServerName,
	)
	err := cm.adminRequest(ctx, http.MethodGet, &url.URL{Path: routesConfigPath}, nil, &currentRoutes)
	if err != nil {
		return err
	}

	// TODO: Find and update existing route group

	// If no existing route group, add one
	proxyHandler := apiHandle{
		Handler:   "reverse_proxy",
		Upstreams: []apiUpstream{},
	}

	for _, port := range ports {
		proxyHandler.Upstreams = append(proxyHandler.Upstreams, apiUpstream{
			Dial: fmt.Sprintf("localhost:%s", port),
		})
	}

	route := apiRoute{
		Match: []apiMatch{
			{
				Host: hostNames,
			},
		},
		Handle: []apiHandle{
			proxyHandler,
		},
	}

	// Persist to caddy
	err = cm.adminRequest(ctx, http.MethodPost, &url.URL{Path: routesConfigPath}, &route, nil)
	if err != nil {
		return err
	}

	return nil
}

func (cm *Manager) DeleteBackend(ctx context.Context, backendName string) error {
	// Fetch current config

	// Find and filter out route group

	// Persist to caddy
	return nil
}

func (cm *Manager) adminRequest(ctx context.Context, method string, path *url.URL, body interface{}, out interface{}) error {
	var bodyToSend io.Reader
	if body != nil {
		if v, ok := body.(string); ok {
			// Send string directly
			bodyToSend = bytes.NewBufferString(v)
		} else {
			// If not a string, JSONify it and send it
			data, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("marshalling body: %w", err)
			}
			bodyToSend = bytes.NewBuffer(data)
		}
	}

	// TODO: Pull this into the config for Manager
	rootPath, err := url.Parse("http://localhost:2019")
	if err != nil {
		return err
	}

	fullPath := rootPath.ResolveReference(path).String()

	req, err := http.NewRequestWithContext(ctx, method, fullPath, bodyToSend)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")

	cm.Log.Debug("making request to caddy",
		zap.String("url", req.URL.String()),
		zap.String("method", req.Method),
	)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer res.Body.Close()

	// TODO: Check status codes
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}
	cm.Log.Debug("response from caddy",
		zap.String("body", string(data)),
		zap.Int("status", res.StatusCode),
	)
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("unmarshalling response: %w", err)
		}
	}

	return nil
}
