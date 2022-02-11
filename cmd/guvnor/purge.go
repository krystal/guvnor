package main

import (
	"context"
	"errors"

	"github.com/spf13/cobra"
)

type purger interface {
	Purge(ctx context.Context) error
}

func newPurgeCmd(p purger) *cobra.Command {
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
		if !*confirmFlag {
			return errors.New("confirm flag must be specified to trigger purge")
		}

		return p.Purge(cmd.Context())
	}

	return cmd
}
