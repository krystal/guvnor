package main

import (
	"github.com/krystal/guvnor"
	"github.com/spf13/cobra"
)

func newRunCmd(eP engineProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [service] <task>",
		Short: "Run a task for a given service",
		Args:  cobra.RangeArgs(1, 2),
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		engine, _, err := eP()
		if err != nil {
			return err
		}

		serviceName := ""
		taskName := ""
		if len(args) == 2 {
			serviceName = args[0]
			taskName = args[1]
		} else if len(args) == 1 {
			taskName = args[0]
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
		}

		return engine.RunTask(cmd.Context(), guvnor.RunTaskArgs{
			ServiceName: serviceName,
			TaskName:    taskName,
		})
	}

	return cmd
}
