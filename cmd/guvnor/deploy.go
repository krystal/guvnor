package main

import (
	"fmt"

	"github.com/krystal/guvnor"
	"github.com/spf13/cobra"
)

func newDeployCmd(eP engineProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "deploy [service]",
		Short:        "Runs a deployment for a given service",
		Args:         cobra.RangeArgs(0, 1),
		SilenceUsage: true,
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
		deployingMessage := "the default service"
		if len(args) == 1 {
			serviceName = args[0]
			deployingMessage = fmt.Sprintf("'%s'", serviceName)
		}

		_, err = infoColour.Fprintf(
			cmd.OutOrStdout(),
			"🔨 Deploying %s. Hold on tight!\n",
			deployingMessage,
		)
		if err != nil {
			return err
		}

		res, err := engine.Deploy(cmd.Context(), guvnor.DeployArgs{
			ServiceName: serviceName,
			Tag:         *tagFlag,
		})
		if err != nil {
			return err
		}

		_, err = successColour.Fprintf(
			cmd.OutOrStdout(),
			"✅ Succesfully deployed '%s'. Deployment ID is %d.\n",
			res.ServiceName,
			res.DeploymentID,
		)
		return err
	}

	return cmd
}
