package guvnor

import (
	"context"
	"errors"

	"go.uber.org/zap"
)

type RunTaskArgs struct {
	ServiceName string
	TaskName    string
}

func (e *Engine) RunTask(ctx context.Context, args RunTaskArgs) error {
	svcName := args.ServiceName
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

	task, ok := svcCfg.Tasks[args.TaskName]
	if !ok {
		return errors.New("specified task cannot be found in config")
	}

	e.log.Debug("loaded task", zap.Strings("cmd", task.Command))

	return nil
}
