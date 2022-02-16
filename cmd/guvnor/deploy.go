package main

import (
	"github.com/krystal/guvnor"
	"github.com/spf13/cobra"
)

func newDeployCmd(eP engineProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [service]",
		Short: "Runs a deployment for a given service",
		Args:  cobra.RangeArgs(0, 1),
	}

	tagFlag := cmd.Flags().String(
		"tag",
		"",
		"Configures a specific image tag to deploy",
	)

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		engine, err := eP()
		if err != nil {
			return err
		}

		serviceName := ""
		if len(args) == 1 {
			serviceName = args[0]
		}

		return engine.Deploy(cmd.Context(), guvnor.DeployArgs{
			ServiceName: serviceName,
			Tag:         *tagFlag,
		})
	}

	return cmd
}
