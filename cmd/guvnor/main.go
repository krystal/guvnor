package main

import (
	"fmt"
	goLog "log"
	"os"

	"github.com/docker/docker/client"
	"github.com/krystal/guvnor"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var version = "indev"

func newRootCmd(subCommands ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "guvnor",
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	for _, subCmd := range subCommands {
		cmd.AddCommand(subCmd)
	}

	return cmd
}

func stdEngineProvider(log *zap.Logger) func() (*guvnor.Engine, error) {
	return func() (*guvnor.Engine, error) {
		dockerClient, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			return nil, fmt.Errorf("connecting to docker: %w", err)
		}

		// TODO: Add a way to override which config is loaded :)
		cfg, err := guvnor.LoadConfig("")
		if err != nil {
			return nil, fmt.Errorf("load config: %w", err)
		}

		e := guvnor.NewEngine(log, dockerClient, *cfg)

		return e, nil
	}
}

type engineProvider = func() (*guvnor.Engine, error)

func main() {
	log, err := zap.NewDevelopment()
	if err != nil {
		goLog.Fatalf("failed to setup logger: %s", err)
	}

	eProv := stdEngineProvider(log)
	root := newRootCmd(
		newDeployCmd(eProv),
		newPurgeCmd(eProv),
		newRunCmd(eProv),
		newStatusCmd(eProv),
		newInitCmd(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
