package main

import (
	"context"
	"fmt"
	goLog "log"
	"os"
	"os/signal"

	"github.com/docker/docker/client"
	"github.com/fatih/color"
	"github.com/go-playground/validator/v10"
	"github.com/krystal/guvnor"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var version = "indev"

var (
	errorColour   = color.New(color.FgRed)
	infoColour    = color.New(color.FgCyan)
	labelColour   = color.New(color.FgBlue)
	normalColour  = color.New(color.FgWhite)
	successColour = color.New(color.FgGreen)
	tableColour   = color.New(color.FgWhite)
)

func init() {
	// This ensures colour is output during tests.
	infoColour.EnableColor()
	successColour.EnableColor()
	normalColour.EnableColor()
	tableColour.EnableColor()
	labelColour.EnableColor()
	errorColour.EnableColor()
}

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

func stdEngineProvider(log *zap.Logger, serviceRootOverride *string) func() (engine, *guvnor.EngineConfig, error) {
	return func() (engine, *guvnor.EngineConfig, error) {
		dockerClient, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			return nil, nil, fmt.Errorf("connecting to docker: %w", err)
		}

		v := validator.New()

		// TODO: Add a way to override which config is loaded :)
		cfg, err := guvnor.LoadConfig(v, "")
		if err != nil {
			return nil, nil, fmt.Errorf("load config: %w", err)
		}

		if *serviceRootOverride != "" {
			cfg.Paths.Config = *serviceRootOverride
		}

		e := guvnor.NewEngine(log, dockerClient, *cfg, v)

		return e, cfg, nil
	}
}

type engineProvider = func() (engine, *guvnor.EngineConfig, error)

type engine interface {
	Cleanup(context.Context, guvnor.CleanupArgs) error
	Deploy(context.Context, guvnor.DeployArgs) (*guvnor.DeployResult, error)
	GetDefaultService() (*guvnor.GetDefaultServiceResult, error)
	Purge(context.Context) error
	RunTask(context.Context, guvnor.RunTaskArgs) error
	Status(context.Context, guvnor.StatusArgs) (*guvnor.StatusResult, error)
}

func main() {
	log, err := zap.NewDevelopment()
	if err != nil {
		goLog.Fatalf("failed to setup logger: %s", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	serviceRootOverride := ""

	eProv := stdEngineProvider(log, &serviceRootOverride)
	root := newRootCmd(
		newCleanupCommand(eProv),
		newDeployCmd(eProv),
		newEditCommand(eProv),
		newInitCmd(),
		newPurgeCmd(eProv),
		newRunCmd(eProv),
		newStatusCmd(eProv),
	)

	root.PersistentFlags().StringVar(
		&serviceRootOverride,
		"service-root",
		"",
		"overrides Guvnor to search for service configs in an alternate directory",
	)

	if err := root.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
