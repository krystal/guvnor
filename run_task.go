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
	svc, err := e.loadServiceConfig(args.ServiceName)
	if err != nil {
		return err
	}

	task, ok := svc.Tasks[args.TaskName]
	if !ok {
		return errors.New("specified task cannot be found in config")
	}

	e.log.Debug("loaded task", zap.Strings("cmd", task.Command))

	return nil
}
