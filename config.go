package guvnor

import (
	"bytes"
	"os"

	"github.com/go-playground/validator/v10"
	"github.com/krystal/guvnor/caddy"
	"gopkg.in/yaml.v3"
)

type EngineConfig struct {
	Caddy caddy.Config `yaml:"caddy"`
	Paths PathsConfig  `yaml:"paths"`
}

type PathsConfig struct {
	// Config is the path to the directory containing service configs
	Config string `yaml:"config" validate:"required"`
	// State is the path to store state about deployments etc
	State string `yaml:"state" validate:"required"`
}

func LoadConfig(validate *validator.Validate, pathOverride string) (*EngineConfig, error) {
	path := "/etc/guvnor/config.yaml"
	if pathOverride != "" {
		path = pathOverride
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	decoder := yaml.NewDecoder(bytes.NewBuffer(data))
	decoder.KnownFields(true)

	cfg := &EngineConfig{}
	if err := decoder.Decode(cfg); err != nil {
		return nil, err
	}

	if err := validate.Struct(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
