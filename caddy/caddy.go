package caddy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	docker "github.com/docker/docker/client"
	"github.com/krystal/guvnor/ready"
	"go.uber.org/zap"
)

const (
	guvnorCaddyContainerName = "guvnor-caddy"
	guvnorServerName         = "guvnor"
)

type Config struct {
	// Image is the container image that should be deployed as caddy
	Image string `yaml:"image"`
	// ListenIP is the IP that the caddy listener should bind to. By default,
	// this will bind to all interfaces/IPs.
	ListenIP           string                             `yaml:"listenIP"`
	ACME               ACMEConfig                         `yaml:"acme"`
	Ports              PortsConfig                        `yaml:"ports"`
	AdditionalBackends map[string]AdditionalBackendConfig `yaml:"additionalBackends"`
}

type AdditionalBackendConfig struct {
	Hostnames []string `yaml:"hostnames"`
	Path      string   `yaml:"path"`
	Upstreams []string `yaml:"upstreams"`
}

type ACMEConfig struct {
	// CA is the URL of the ACME service to request certificates from.
	CA string `yaml:"ca"`
	// Email is the address that should be provided to the acme service for
	// contacting us.
	Email string `yaml:"email"`
}

type PortsConfig struct {
	// HTTP is the port Caddy should listen on for unencrypted HTTP traffic.
	// By default this is 80.
	HTTP int `yaml:"http"`
	// HTTPS is the port Caddy should listen on for encrypted HTTPS traffic.
	// By default this is 443.
	HTTPS int `yaml:"https"`
}

type caddyConfigurator interface {
	getRoutes(ctx context.Context) ([]route, error)
	updateRoutes(ctx context.Context, routes []route) error
	getConfig(ctx context.Context) (*caddy.Config, error)
	updateConfig(ctx context.Context, cfg *caddy.Config) error
}

// Manager creates and manages a Caddy container. It provides a Init() method
// for creating the container, and reconciling its initial configuration, and
// methods for reconciling Guvnor services in the caddy configuration.
type Manager struct {
	Log *zap.Logger

	// CaddyConfigurator is used by manager for making changes to a caddy
	// configuration
	CaddyConfigurator caddyConfigurator
	// Docker is the implementation of Docker that the manager should use to
	// create and query containers.
	Docker docker.APIClient
	// Config controls how the manager behaves.
	Config Config
	// ContainerLabels is a map of labels to add to any containers created by
	// the manager.
	ContainerLabels map[string]string
}

func (cm *Manager) calculateConfigChanges(config *caddy.Config) (bool, error) {
	// TODO: Swap this out for some hashing or string comparison.
	//
	// I originally tried this with a JSONified version of the config, but
	// unfortunately this did not work as the keys changed position. This could
	// be worked around but in the interest of time, I went with a bool.
	hasChanged := false

	if config.AppsRaw == nil {
		config.AppsRaw = caddy.ModuleMap{}
		hasChanged = true
	}

	httpConfig := &caddyhttp.App{}
	currentHTTPConfigRaw, ok := config.AppsRaw["http"]
	if ok {
		if err := json.Unmarshal(currentHTTPConfigRaw, httpConfig); err != nil {
			return hasChanged, err
		}
	}

	if httpConfig.HTTPPort != cm.Config.Ports.HTTP {
		httpConfig.HTTPPort = cm.Config.Ports.HTTP
		hasChanged = true
	}

	if httpConfig.HTTPSPort != cm.Config.Ports.HTTPS {
		httpConfig.HTTPSPort = cm.Config.Ports.HTTPS
		hasChanged = true
	}

	if httpConfig.Servers == nil {
		httpConfig.Servers = map[string]*caddyhttp.Server{}
		hasChanged = true
	}

	serverConfig := httpConfig.Servers[guvnorServerName]
	if serverConfig == nil {
		serverConfig = &caddyhttp.Server{}
		httpConfig.Servers[guvnorServerName] = serverConfig
		hasChanged = true
	}

	listenAddr := fmt.Sprintf(
		"%s:%d",
		cm.Config.ListenIP,
		cm.Config.Ports.HTTPS,
	)
	if len(serverConfig.Listen) != 1 || serverConfig.Listen[0] != listenAddr {
		serverConfig.Listen = []string{listenAddr}
		hasChanged = true
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
			return hasChanged, err
		}

		serverConfig.Routes = append(serverConfig.Routes,
			caddyhttp.Route{
				HandlersRaw: []json.RawMessage{
					json.RawMessage(defaultHandlerRaw),
				},
			},
		)
		hasChanged = true
	}

	// TODO: Clean up servers we don't recognise ??

	// Persist the HTTP config to the main config
	modifiedHTTPConfigRaw, err := json.Marshal(httpConfig)
	if err != nil {
		return hasChanged, err
	}
	config.AppsRaw["http"] = modifiedHTTPConfigRaw

	return hasChanged, err
}

func (cm *Manager) reconcileCaddyConfig(ctx context.Context) error {
	config, err := cm.CaddyConfigurator.getConfig(ctx)
	if err != nil {
		return err
	}

	hasChanged, err := cm.calculateConfigChanges(config)
	if err != nil {
		return err
	}

	if hasChanged {
		cm.Log.Info("reconciliation found changes, updating caddy config")
		if err := cm.CaddyConfigurator.updateConfig(ctx, config); err != nil {
			return err
		}
	}

	for backendName, additionalBackend := range cm.Config.AdditionalBackends {
		err := cm.ConfigureBackend(
			ctx,
			backendName,
			additionalBackend.Hostnames,
			additionalBackend.Upstreams,
			additionalBackend.Path,
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

func (cm *Manager) generateRouteforBackend(backendName string, hostnames []string, upstreams []string, path string) route {
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
	for _, u := range upstreams {
		handler.Upstreams = append(handler.Upstreams, upstream{
			Dial: u,
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
			// Sort the default handler last (it has no matcher sets)
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

// ConfigureBackend sets up the appropriate routes in Caddy for a specific
// process/service
func (cm *Manager) ConfigureBackend(
	ctx context.Context,
	backendName string,
	hostNames []string,
	upstreams []string,
	path string,
) error {
	cm.Log.Info("configuring caddy for backend",
		zap.String("backend", backendName),
		zap.Strings("hostnames", hostNames),
		zap.String("path", path),
		zap.Strings("upstreams", upstreams),
	)
	// Fetch current config
	routes, err := cm.CaddyConfigurator.getRoutes(ctx)
	if err != nil {
		return err
	}

	routeConfig := cm.generateRouteforBackend(backendName, hostNames, upstreams, path)

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

	return cm.CaddyConfigurator.updateRoutes(ctx, routes)
}
