package main

import (
	"errors"

	"github.com/spf13/cobra"
)

func newPurgeCmd(eP engineProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "purge",
		Short: "Purges all containers created by Guvnor",
	}

	confirmFlag := cmd.Flags().Bool(
		"confirm",
		false,
		"Confirms you wish to run this destructive action",
	)

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		engine, err := eP()
		if err != nil {
			return err
		}

		if !*confirmFlag {
			return errors.New("confirm flag must be specified to trigger purge")
		}

		return engine.Purge(cmd.Context())
	}

	return cmd
}
