package main

import (
	"fmt"
	"time"

	"github.com/krystal/guvnor"
	"github.com/spf13/cobra"
)

func newStatusCmd(eP engineProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <service>",
		Short: "Shows status of a specific service",
		Args:  cobra.ExactArgs(1),
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		engine, err := eP()
		if err != nil {
			return err
		}
		serviceName := args[0]

		res, err := engine.Status(
			cmd.Context(),
			guvnor.StatusArgs{ServiceName: serviceName},
		)
		if err != nil {
			return err
		}

		// TODO: Come back and make this output prettier :)
		fmt.Fprintf(
			cmd.OutOrStdout(),
			"------ Service: %s ------\n",
			serviceName,
		)
		fmt.Fprintf(
			cmd.OutOrStdout(),
			"Deployment count: %d\n",
			res.DeploymentID,
		)
		fmt.Fprintf(
			cmd.OutOrStdout(),
			"Last deployed at: %s\n",
			res.LastDeployedAt.Format(time.RFC1123),
		)
		for processName, process := range res.Processes {
			fmt.Fprintf(cmd.OutOrStdout(), "---- Process: %s ----\n", processName)
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Desired replicas: %d\nContainers:\n",
				process.WantReplicas,
			)
			for _, container := range process.Containers {
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"| %s | %s | %s |\n",
					container.ContainerName,
					container.ContainerID,
					container.Status,
				)
			}
		}

		return nil
	}

	return cmd
}
