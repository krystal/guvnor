package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [service]",
		Short: "Shows status of all services, or a specific one",
		Args:  cobra.RangeArgs(0, 1),
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		serviceName := ""
		if len(args) == 1 {
			serviceName = args[0]
		}

		_, err := fmt.Fprintln(
			cmd.OutOrStdout(), "not yet implemented.", serviceName,
		)
		if err != nil {
			return err
		}

		return errors.New("unimplemented")
	}

	return cmd
}
