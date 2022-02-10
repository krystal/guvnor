package main

import (
	"log"

	"github.com/spf13/cobra"
)

var version = "indev"

func NewRootCmd(subCommands ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "guvnor",
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	for _, subCmd := range subCommands {
		cmd.AddCommand(subCmd)
	}

	return cmd
}

func main() {
	root := NewRootCmd()

	err := root.Execute()
	if err != nil {
		log.Fatalf(err.Error())
	}
}
