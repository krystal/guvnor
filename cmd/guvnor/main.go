package main

import (
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

func main() {
	var e *guvnor.Engine
	{ // TODO: Only set this up if the command needs it
		log, err := zap.NewDevelopment()
		if err != nil {
			goLog.Fatalf("failed to setup logger: %s", err)
		}

		dockerClient, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			goLog.Fatalf("failed to connect to docker: %s", err)
		}

		cfg, err := guvnor.LoadConfig("")
		if err != nil {
			goLog.Fatalf("failed to load config: %s", err)
		}

		e = guvnor.NewEngine(log, dockerClient, *cfg)
	}

	deployCmd := newDeployCmd(e)
	purgeCmd := newPurgeCmd(e)
	root := newRootCmd(deployCmd, purgeCmd)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
