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

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
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
	HTTP  int `yaml:"http"`
	HTTPS int `yaml:"https"`
}

type Manager struct {
	Docker          *client.Client
	Log             *zap.Logger
	Config          Config
	ContainerLabels map[string]string
}

func (cm *Manager) defaultConfiguration() ([]byte, error) {
	httpConfig := &caddyhttp.App{
		HTTPPort:  cm.Config.Ports.HTTP,
		HTTPSPort: cm.Config.Ports.HTTPS,
		Servers: map[string]*caddyhttp.Server{
			guvnorServerName: {
				Routes: caddyhttp.RouteList{
					caddyhttp.Route{},
				},
			},
		},
	}
	httpConfigBytes, err := json.Marshal(httpConfig)
	if err != nil {
		return nil, err
	}

	cfg := caddy.Config{
		Admin: &caddy.AdminConfig{
			// We can rely on the default values here for now.
		},
		Logging: &caddy.Logging{
			// We can rely on the default values here for now.
		},
		AppsRaw: caddy.ModuleMap{
			"http": json.RawMessage(httpConfigBytes),
		},
	}

	return json.Marshal(cfg)
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
	defaultConfig, err := cm.defaultConfiguration()
	if err != nil {
		return err
	}
	err = cm.adminRequest(ctx, http.MethodPost, &url.URL{Path: "config/"}, defaultConfig, nil)
	if err != nil {
		return err
	}

	return nil
}

type rpHandler reverseproxy.Handler

func (rp rpHandler) MarshalJSON() ([]byte, error) {
	// If there is a higher power, I hope they forgive me for this.
	// Unfortunately, the types exposed by Caddy actually do not marshal by
	// default in a way that Caddy itself can understand, a "handler" key must
	// be injected to identify the type of the handler.
	data, err := json.Marshal(reverseproxy.Handler(rp))
	if err != nil {
		return nil, err
	}

	jsonMap := map[string]interface{}{}
	if err := json.Unmarshal(data, &jsonMap); err != nil {
		return nil, err
	}

	jsonMap["handler"] = "reverse_proxy"

	return json.Marshal(jsonMap)
}

func (cm *Manager) generateBackendConfig(backendName string, hostnames []string, ports []string) (*caddyhttp.Route, error) {
	handler := rpHandler{
		Upstreams: reverseproxy.UpstreamPool{},
	}

	for _, port := range ports {
		handler.Upstreams = append(handler.Upstreams, &reverseproxy.Upstream{
			Dial: fmt.Sprintf("localhost:%s", port),
		})
	}

	matcherJson, err := json.Marshal(caddyhttp.MatchHost(hostnames))
	if err != nil {
		return nil, err
	}
	handlerJson, err := json.Marshal(handler)
	if err != nil {
		return nil, err
	}
	route := caddyhttp.Route{
		Group: backendName,
		MatcherSetsRaw: caddyhttp.RawMatcherSets{
			{
				"host": json.RawMessage(matcherJson),
			},
		},
		HandlersRaw: []json.RawMessage{
			json.RawMessage(handlerJson),
		},
	}

	return &route, nil
}

// ConfigureBackend sets up the appropriate routes in Caddy for a
// specific process/service
func (cm *Manager) ConfigureBackend(
	ctx context.Context,
	backendName string,
	hostNames []string,
	ports []string,
) error {
	cm.Log.Debug("configuring caddy for process",
		zap.String("backend", backendName),
		zap.Strings("hostnames", hostNames),
		zap.Strings("ports", ports),
	)
	// Fetch current config
	currentRoutes := caddyhttp.RouteList{}
	routesConfigPath := fmt.Sprintf(
		"config/apps/http/servers/%s/routes",
		guvnorServerName,
	)
	err := cm.adminRequest(ctx, http.MethodGet, &url.URL{Path: routesConfigPath}, nil, &currentRoutes)
	if err != nil {
		return err
	}

	routeConfig, err := cm.generateBackendConfig(backendName, hostNames, ports)
	if err != nil {
		return err
	}

	// Find and update existing route group
	for i, route := range currentRoutes {
		if route.Group == backendName {
			cm.Log.Debug("found existing route, patching", zap.Int("i", i))

			routeConfigPath := fmt.Sprintf(
				"config/apps/http/servers/%s/routes/%d",
				guvnorServerName,
				i,
			)
			return cm.adminRequest(
				ctx,
				http.MethodPatch,
				&url.URL{Path: routeConfigPath},
				routeConfig,
				nil,
			)
		}
	}

	cm.Log.Debug("no existing route group found, appending")
	return cm.adminRequest(
		ctx,
		http.MethodPost,
		&url.URL{Path: routesConfigPath},
		routeConfig,
		nil,
	)
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
		} else if v, ok := body.([]byte); ok {
			bodyToSend = bytes.NewBuffer(v)
		} else {
			// If not a string, JSONify it and send it
			data, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("marshalling body: %w", err)
			}
			cm.Log.Info("sending", zap.String("data", string(data)))
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