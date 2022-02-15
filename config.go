package guvnor

import (
	"os"

	"github.com/krystal/guvnor/caddy"
	"gopkg.in/yaml.v3"
)

type EngineConfig struct {
	Caddy caddy.Config `yaml:"caddy"`
	Paths PathsConfig  `yaml:"paths"`
}

type PathsConfig struct {
	// Config is the path to the directory containing service configs
	Config string `yaml:"config"`
	// State is the path to store state about deployments etc
	State string `yaml:"state"`
}

func LoadConfig(pathOverride string) (*EngineConfig, error) {
	path := "/etc/guvna/config.yaml"
	if pathOverride != "" {
		path = pathOverride
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &EngineConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
