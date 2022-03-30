package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/krystal/guvnor/ready"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/rand"
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

func hashConfig(cfg *caddy.Config) string {
	hasher := fnv.New32()
	fmt.Fprintf(hasher, "%#v", cfg)
	return rand.SafeEncodeString(fmt.Sprint(hasher.Sum32()))
}

func (cm *Manager) reconcileCaddyConfig(ctx context.Context) error {
	config, err := cm.getConfig(ctx)
	if err != nil {
		return err
	}
	initialHash := hashConfig(config)

	httpConfig := &caddyhttp.App{}
	currentHTTPConfigRaw, ok := config.AppsRaw["http"]
	if ok {
		if err := json.Unmarshal(currentHTTPConfigRaw, httpConfig); err != nil {
			return err
		}
	}

	if httpConfig.HTTPPort != cm.Config.Ports.HTTP {
		httpConfig.HTTPPort = cm.Config.Ports.HTTP
	}

	if httpConfig.HTTPSPort != cm.Config.Ports.HTTPS {
		httpConfig.HTTPSPort = cm.Config.Ports.HTTPS
	}

	if httpConfig.Servers == nil {
		httpConfig.Servers = map[string]*caddyhttp.Server{}
	}

	serverConfig := httpConfig.Servers[guvnorServerName]
	if serverConfig == nil {
		serverConfig = &caddyhttp.Server{}
		httpConfig.Servers[guvnorServerName] = serverConfig
	}

	listenAddr := ":" + strconv.Itoa(cm.Config.Ports.HTTPS)
	if len(serverConfig.Listen) != 1 || serverConfig.Listen[0] != listenAddr {
		serverConfig.Listen = []string{listenAddr}
	}

	// TODO: Be a bit smarter and create/update the default route
	// Add the default route if there are currently no routes
	if len(serverConfig.Routes) == 0 {
		defaultHandler := map[string]interface{}{
			"handler":     "static_response",
			"body":        "Welcome to Guvnor. We found no backend matching your request.",
			"status_code": "404",
		}
		defaultHandlerRaw, err := json.Marshal(defaultHandler)
		if err != nil {
			return err
		}

		serverConfig.Routes = append(serverConfig.Routes,
			caddyhttp.Route{
				HandlersRaw: []json.RawMessage{
					json.RawMessage(defaultHandlerRaw),
				},
			},
		)
	}

	// TODO: Clean up servers we don't recognise ??

	// Persist the HTTP config to the main config
	modifiedHTTPConfigRaw, err := json.Marshal(httpConfig)
	if err != nil {
		return err
	}
	config.AppsRaw["http"] = modifiedHTTPConfigRaw

	finalHash := hashConfig(config)
	// Compare hash of confeeg ?
	if initialHash != finalHash {
		err = cm.doRequest(
			ctx, http.MethodPost, &url.URL{Path: "config/"}, config, nil,
		)
		if err != nil {
			return err
		}
	}

	return nil
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
		cm.Log.Debug("caddy container already running")

		return cm.reconcileCaddyConfig(ctx)
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
	if _, err := io.Copy(os.Stdout, pullStream); err != nil {
		return err
	}

	dataVolume := "guvnor-caddy-data"
	configVolume := "guvnor-caddy-config"
	createRes, err := cm.Docker.ContainerCreate(
		ctx,
		&container.Config{
			Image:  image,
			Labels: cm.ContainerLabels,
			Volumes: map[string]struct{}{
				dataVolume:   {},
				configVolume: {},
			},
			Entrypoint: []string{"caddy"},
			Cmd:        []string{"run", "--resume"},
		},
		&container.HostConfig{
			NetworkMode: "host",
			RestartPolicy: container.RestartPolicy{
				Name: "always",
			},
			Mounts: []mount.Mount{
				{
					Type:   mount.TypeVolume,
					Target: "/data",
					Source: dataVolume,
				},
				{
					Type:   mount.TypeVolume,
					Target: "/config",
					Source: configVolume,
				},
			},
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

	check := ready.Check{
		Frequency: time.Millisecond * 500,
		Maximum:   20,
		HTTP: &ready.HTTPCheck{
			Host:           "localhost:2019",
			Path:           "/config/",
			ExpectedStatus: 200,
			Timeout:        500 * time.Millisecond,
		},
	}
	if err := check.Wait(ctx, cm.Log.Named("ready")); err != nil {
		return err
	}

	if err := cm.reconcileCaddyConfig(ctx); err != nil {
		return err
	}

	return nil
}

func (cm *Manager) generateRouteforBackend(backendName string, hostnames []string, ports []string, path string) route {
	route := route{
		Group:       backendName,
		MatcherSets: []matcherSet{},
		Handlers:    handlers{},
		Terminal:    true,
	}

	// Configure handler
	handler := reverseProxyHandler{
		Upstreams: []upstream{},
	}
	for _, port := range ports {
		handler.Upstreams = append(handler.Upstreams, upstream{
			Dial: fmt.Sprintf("localhost:%s", port),
		})
	}
	route.Handlers = append(route.Handlers, handler)

	matcher := matcherSet{
		Host: hostnames,
	}

	if path != "" {
		matcher.Path = []string{path}
	}

	route.MatcherSets = append(route.MatcherSets, matcher)

	return route
}

// Sorts routes by the length of their path segment. This ensures they are
// matched in the correct order.
func sortRoutes(routes []route) {
	pathLength := func(route route) int {
		if len(route.MatcherSets) == 0 {
			return -1
		}
		matcher := route.MatcherSets[0]

		if len(matcher.Path) == 0 || matcher.Path[0] == "" {
			return 0
		}

		segments := len(strings.Split(matcher.Path[0], "/"))

		return segments
	}
	sort.SliceStable(routes, func(i, j int) bool {
		return pathLength(routes[i]) > pathLength(routes[j])
	})
}

// ConfigureBackend sets up the appropriate routes in Caddy for a
// specific process/service
func (cm *Manager) ConfigureBackend(
	ctx context.Context,
	backendName string,
	hostNames []string,
	ports []string,
	path string,
) error {
	cm.Log.Info("configuring caddy for backend",
		zap.String("backend", backendName),
		zap.Strings("hostnames", hostNames),
		zap.String("path", path),
		zap.Strings("ports", ports),
	)
	// Fetch current config
	routes, err := cm.getRoutes(ctx)
	if err != nil {
		return err
	}

	routeConfig := cm.generateRouteforBackend(backendName, hostNames, ports, path)

	// Find and update existing route group
	existingRoute := false
	for i, route := range routes {
		if route.Group == backendName {
			routes[i] = routeConfig
			existingRoute = true
		}
	}
	if !existingRoute {
		routes = append(routes, routeConfig)
	}

	sortRoutes(routes)

	return cm.patchRoutes(ctx, routes)
}

// getRoutes returns an slice of routes configured on the caddy server
func (cm *Manager) getRoutes(ctx context.Context) ([]route, error) {
	currentRoutes := []route{}
	routesConfigPath := fmt.Sprintf(
		"config/apps/http/servers/%s/routes",
		guvnorServerName,
	)
	err := cm.doRequest(ctx, http.MethodGet, &url.URL{Path: routesConfigPath}, nil, &currentRoutes)
	if err != nil {
		return nil, err
	}

	return currentRoutes, nil
}

// prependRoute adds a new route to the start of the route array in the server
func (cm *Manager) patchRoutes(ctx context.Context, route []route) error {
	prependRoutePath := fmt.Sprintf(
		"config/apps/http/servers/%s/routes",
		guvnorServerName,
	)
	return cm.doRequest(
		ctx,
		http.MethodPatch,
		&url.URL{Path: prependRoutePath},
		route,
		nil,
	)
}

func (cm *Manager) getConfig(ctx context.Context) (*caddy.Config, error) {
	cfg := &caddy.Config{}
	err := cm.doRequest(
		ctx,
		http.MethodGet,
		&url.URL{Path: "config/"},
		nil,
		cfg,
	)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func (cm *Manager) doRequest(ctx context.Context, method string, path *url.URL, body interface{}, out interface{}) error {
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
