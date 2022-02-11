package guvnor

import "github.com/krystal/guvnor/caddy"

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
