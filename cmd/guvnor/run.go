package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [service] [task]",
		Short: "Run a task for a given service.",
		Args:  cobra.RangeArgs(1, 2),
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		serviceName := ""
		taskName := ""
		if len(args) == 2 {
			serviceName = args[0]
			taskName = args[1]
		} else if len(args) == 1 {
			taskName = args[0]
		}

		if serviceName == "" {
			_, err := fmt.Fprintf(
				cmd.OutOrStdout(),
				"running %s on the default service",
				taskName,
			)
			if err != nil {
				return err
			}
		} else {
			_, err := fmt.Fprintf(
				cmd.OutOrStdout(),
				"running %s on %s",
				taskName, serviceName,
			)
			if err != nil {
				return err
			}
		}

		return nil
	}

	return cmd
}
