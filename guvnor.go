package guvnor

import "go.uber.org/zap"

type docker interface {
}

type Engine struct {
	log    *zap.Logger
	docker docker
}

func NewEngine(log *zap.Logger, docker docker) *Engine {
	return &Engine{
		log:    log,
		docker: docker,
	}
}
