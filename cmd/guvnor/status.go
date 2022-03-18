package main

import (
	"io"
	"time"

	"github.com/fatih/color"
	"github.com/krystal/guvnor"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

type colorWriter struct {
	io.Writer
	*color.Color
}

func (c colorWriter) Write(p []byte) (n int, err error) {
	return c.Color.Fprint(c.Writer, string(p))
}

func newStatusCmd(eP engineProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [service]",
		Short: "Shows status of a specific service",
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
			"🔎 Checking status of '%s'! Will be just a tick.\n",
			serviceName,
		)
		if err != nil {
			return err
		}

		res, err := engine.Status(
			cmd.Context(),
			guvnor.StatusArgs{ServiceName: serviceName},
		)
		if err != nil {
			return err
		}

		_, err = successColour.Fprintln(
			cmd.OutOrStdout(),
			"✅ Succesfully fetched status.",
		)
		if err != nil {
			return err
		}
		infoColour.Fprintf(
			cmd.OutOrStdout(),
			"------ Service: %s ------\n",
			serviceName,
		)
		labelColour.Fprint(
			cmd.OutOrStdout(),
			"Deployment count: ",
		)
		normalColour.Fprintln(
			cmd.OutOrStdout(),
			res.DeploymentID,
		)
		labelColour.Fprint(
			cmd.OutOrStdout(),
			"Last deployed at: ",
		)
		normalColour.Fprintln(
			cmd.OutOrStdout(),
			res.LastDeployedAt.Format(time.RFC1123),
		)

		for _, processName := range res.Processes.OrderedKeys() {
			process := res.Processes[processName]
			infoColour.Fprintf(cmd.OutOrStdout(), "---- Process: %s ----\n", processName)
			labelColour.Fprint(
				cmd.OutOrStdout(),
				"Desired replicas: ",
			)
			normalColour.Fprintln(
				cmd.OutOrStdout(),
				process.WantReplicas,
			)
			labelColour.Fprintln(
				cmd.OutOrStdout(),
				"Containers: ",
			)
			tw := tablewriter.NewWriter(colorWriter{cmd.OutOrStdout(), tableColour})
			tw.SetHeader([]string{"Name", "ID", "Status"})
			tw.SetBorder(false)
			tw.SetRowLine(false)
			tw.SetHeaderLine(false)
			tw.SetColumnSeparator("")
			for _, container := range process.Containers {
				status := container.Status
				switch status {
				case "running":
					status = successColour.Sprint(status)
				case "stopped":
				case "dead":
					status = errorColour.Sprint(status)
				}
				tw.Append([]string{
					container.ContainerName,
					container.ContainerID,
					status,
				})
			}
			tw.Render()
		}

		return nil
	}

	return cmd
}
