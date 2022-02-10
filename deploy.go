package guvnor

import (
	"context"

	"github.com/docker/docker/api/types"
	"go.uber.org/zap"
)

type DeployConfig struct {
	ServiceName string
	Tag         string
}

func (e *Engine) Deploy(ctx context.Context, cfg DeployConfig) error {
	svcName := cfg.ServiceName
	if svcName == "" {
		var err error
		svcName, err = findDefaultService(e.config.Paths.Config)
		if err != nil {
			return err
		}
		e.log.Debug(
			"no service name provided, defaulting",
			zap.String("default", svcName),
		)
	}

	svcCfg, err := loadServiceConfig(e.config.Paths.Config, svcName)
	if err != nil {
		return err
	}
	e.log.Debug("svcCfg", zap.Any("cfg", svcCfg))

	containers, err := e.docker.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return err
	}

	for _, c := range containers {
		e.log.Debug("container", zap.Strings("names", c.Names))
	}
	return nil
}
