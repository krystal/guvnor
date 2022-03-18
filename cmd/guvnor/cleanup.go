package main

import (
	"github.com/krystal/guvnor"
	"github.com/spf13/cobra"
)

func newCleanupCommand(eP engineProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup [service]",
		Short: "Force kill zombie containers belonging to the service.",
		Args:  cobra.RangeArgs(0, 1),
	}

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
			"🗑 Cleaning up %s's zombie containers.\n",
			serviceName,
		)
		if err != nil {
			return err
		}

		err = engine.Cleanup(
			cmd.Context(),
			guvnor.CleanupArgs{ServiceName: serviceName},
		)
		if err != nil {
			return err
		}

		return nil
	}

	return cmd
}
