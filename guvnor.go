package guvnor

import (
	"github.com/docker/docker/client"
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
	caddy  *CaddyManager
}

func NewEngine(log *zap.Logger, docker *client.Client) *Engine {
	// TODO: Load this from disk
	cfg := EngineConfig{
		Caddy: CaddyConfig{
			Image: "docker.io/library/caddy:2.4.6-alpine",
		},
		Paths: PathsConfig{
			Config: "./local/services",
		},
	}

	return &Engine{
		log:    log,
		docker: docker,
		config: cfg,
		caddy: &CaddyManager{
			docker: docker,
			log:    log.Named("caddy"),
			config: cfg.Caddy,
		},
	}
}
