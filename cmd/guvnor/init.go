package main

import (
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/krystal/guvnor"
	"github.com/krystal/guvnor/caddy"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialises Guvnor on a host, with a default configuration",
		Args:  cobra.NoArgs,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		configPath := "/etc/guvnor"
		configFilePath := path.Join(configPath, "config.yaml")
		servicesPath := path.Join(configPath, "services")
		statePath := "/var/lib/guvnor"

		_, err := os.Stat(configFilePath)
		if err == nil {
			return errors.New("guvnor install detected")
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		defaultConfig := guvnor.EngineConfig{
			Caddy: caddy.Config{
				Image: "docker.io/library/caddy:2.4.6-alpine",
				Ports: caddy.PortsConfig{
					HTTP:  80,
					HTTPS: 443,
				},
			},
			Paths: guvnor.PathsConfig{
				Config: servicesPath,
				State:  statePath,
			},
		}

		configBytes, err := yaml.Marshal(defaultConfig)
		if err != nil {
			return err
		}

		if err := os.Mkdir(configPath, 0o755); err != nil {
			return err
		}

		if err := os.Mkdir(servicesPath, 0o755); err != nil {
			return err
		}

		if err := os.Mkdir(statePath, 0o755); err != nil {
			return err
		}

		if err := os.WriteFile(configFilePath, configBytes, 0o644); err != nil {
			return err
		}

		_, err = fmt.Fprintln(
			cmd.OutOrStderr(),
			"Guvnor succesfully initialized",
		)

		return err
	}

	return cmd
}
