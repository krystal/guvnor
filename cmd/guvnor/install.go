package main

import (
	"os"
	"path"

	"github.com/krystal/guvnor"
	"github.com/krystal/guvnor/caddy"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Installs Guvnor on a host, with a default configuration",
		Args:  cobra.NoArgs,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		configPath := "/etc/guvnor"
		servicesPath := path.Join(configPath, "services")
		statePath := "/var/lib/guvnor"

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
				State:  "/var/lib/guvnor",
			},
		}

		configBytes, err := yaml.Marshal(defaultConfig)
		if err != nil {
			return err
		}

		if err := os.Mkdir(configPath, 0o644); err != nil {
			return err
		}

		if err := os.Mkdir(servicesPath, 0o644); err != nil {
			return err
		}

		if err := os.Mkdir(statePath, 0o644); err != nil {
			return err
		}

		return os.WriteFile("/etc/guvnor/config.yaml", configBytes, 0o644)
	}

	return cmd
}
