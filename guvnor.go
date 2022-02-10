package guvnor

import (
	"github.com/docker/docker/client"
	"go.uber.org/zap"
)

type Engine struct {
	log    *zap.Logger
	docker *client.Client
	config EngineConfig
}

func NewEngine(log *zap.Logger, docker *client.Client) *Engine {
	return &Engine{
		log:    log,
		docker: docker,
		config: EngineConfig{
			Caddy: CaddyConfig{
				Image: "docker.io/library/caddy:2.4.6-alpine",
			},
			Paths: PathsConfig{
				Config: "./local/services",
			},
		},
	}
}
