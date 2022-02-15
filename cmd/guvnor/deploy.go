package main

import (
	"context"

	"github.com/krystal/guvnor"
	"github.com/spf13/cobra"
)

type deployer interface {
	Deploy(ctx context.Context, cfg guvnor.DeployArgs) error
}

func newDeployCmd(d deployer) *cobra.Command {
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
		serviceName := ""
		if len(args) == 1 {
			serviceName = args[0]
		}

		return d.Deploy(cmd.Context(), guvnor.DeployArgs{
			ServiceName: serviceName,
			Tag:         *tagFlag,
		})
	}

	return cmd
}
