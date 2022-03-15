package main

import (
	"fmt"

	"github.com/krystal/guvnor"
	"github.com/spf13/cobra"
)

func newCleanupCommand(eP engineProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup <service>",
		Short: "Force kill zombie containers belonging to the service.",
		Args:  cobra.RangeArgs(0, 1),
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		engine, _, err := eP()
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
			"🗑 Cleaning up %s's zombie containers.\n",
			deployingMessage,
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
