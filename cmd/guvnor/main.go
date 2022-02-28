package main

import (
	"context"
	"fmt"
	goLog "log"
	"os"

	"github.com/docker/docker/client"
	"github.com/fatih/color"
	"github.com/go-playground/validator/v10"
	"github.com/krystal/guvnor"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var version = "indev"

var (
	infoColour    = color.New(color.FgCyan)
	labelColour   = color.New(color.FgBlue)
	successColour = color.New(color.FgGreen)
	errorColour   = color.New(color.FgRed)
	normalColour  = color.New(color.FgWhite)
	tableColour   = color.New(color.FgWhite)
)

func newRootCmd(subCommands ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "guvnor",
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		SilenceUsage: true,
	}

	for _, subCmd := range subCommands {
		cmd.AddCommand(subCmd)
	}

	return cmd
}

func stdEngineProvider(log *zap.Logger, serviceRootOverride *string) func() (engine, error) {
	return func() (engine, error) {
		dockerClient, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			return nil, fmt.Errorf("connecting to docker: %w", err)
		}

		v := validator.New()

		// TODO: Add a way to override which config is loaded :)
		cfg, err := guvnor.LoadConfig(v, "")
		if err != nil {
			return nil, fmt.Errorf("load config: %w", err)
		}

		if *serviceRootOverride != "" {
			cfg.Paths.Config = *serviceRootOverride
		}

		e := guvnor.NewEngine(log, dockerClient, *cfg, v)

		return e, nil
	}
}

type engineProvider = func() (engine, error)

type engine interface {
	Purge(context.Context) error
	Deploy(context.Context, guvnor.DeployArgs) (*guvnor.DeployRes, error)
	Status(context.Context, guvnor.StatusArgs) (*guvnor.StatusRes, error)
	RunTask(context.Context, guvnor.RunTaskArgs) error
}

func main() {
	log, err := zap.NewDevelopment()
	if err != nil {
		goLog.Fatalf("failed to setup logger: %s", err)
	}

	serviceRootOverride := ""

	eProv := stdEngineProvider(log, &serviceRootOverride)
	root := newRootCmd(
		newDeployCmd(eProv),
		newPurgeCmd(eProv),
		newRunCmd(eProv),
		newStatusCmd(eProv),
		newInitCmd(),
	)

	root.PersistentFlags().StringVar(
		&serviceRootOverride,
		"service-root",
		"",
		"overrides Guvnor to search for service configs in an alternate directory",
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
