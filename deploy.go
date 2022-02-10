package guvnor

import "context"

type DeployConfig struct {
	ServiceName string
	Tag         string
}

func (e *Engine) Deploy(ctx context.Context, cfg DeployConfig) error {
	if cfg.ServiceName == "" {
		// Find only service or error if more than one
	}
	return nil
}
