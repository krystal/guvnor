package guvnor

import (
	"github.com/docker/docker/client"
	"github.com/krystal/guvnor/caddy"
	"github.com/krystal/guvnor/state"
	"go.uber.org/zap"
)

const (
	serviceLabel    = "io.k.guvnor.service"
	processLabel    = "io.k.guvnor.process"
	deploymentLabel = "io.k.guvnor.deployment"
	managedLabel    = "io.k.guvnor.managed"
)

type Engine struct {
	log    *zap.Logger
	docker *client.Client
	config EngineConfig
	caddy  *caddy.Manager
	state  *state.FileBasedStore
}

func NewEngine(log *zap.Logger, docker *client.Client) *Engine {
	// TODO: Load this from disk
	cfg := EngineConfig{
		Caddy: caddy.Config{
			Image: "docker.io/library/caddy:2.4.6-alpine",
		},
		Paths: PathsConfig{
			Config: "./local/services",
			State:  "./local/state",
		},
	}

	return &Engine{
		log:    log,
		docker: docker,
		config: cfg,
		caddy: &caddy.Manager{
			Docker: docker,
			Log:    log.Named("caddy"),
			Config: cfg.Caddy,
			ContainerLabels: map[string]string{
				managedLabel: "1",
			},
		},
		state: &state.FileBasedStore{
			RootPath: cfg.Paths.State,
			Log:      log.Named("state"),
		},
	}
}
