package main

import (
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
		engine, _, err := eP()
		if err != nil {
			return err
		}

		serviceName := ""
		if len(args) == 0 {
			_, err = infoColour.Fprintln(
				cmd.OutOrStdout(),
				"⚠️  No service argument provided. Finding default.",
			)
			if err != nil {
				return err
			}
			res, err := engine.GetDefaultService()
			if err != nil {
				return err
			}
			serviceName = res.Name
		} else {
			serviceName = args[0]
		}

		_, err = infoColour.Fprintf(
			cmd.OutOrStdout(),
			"🔨 Deploying '%s'. Hold on tight!\n",
			serviceName,
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
