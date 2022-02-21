package guvnor

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type ServiceConfig struct {
	Name      string                `yaml:"_"`
	StartPort uint                  `yaml:"startPort"`
	Defaults  ServiceDefaultsConfig `yaml:"defaults"`

	Processes map[string]ServiceProcessConfig `yaml:"processes"`
	Tasks     map[string]ServiceTaskConfig    `yaml:"tasks"`
}

type ServiceDefaultsConfig struct {
	Image    string               `yaml:"image"`
	ImageTag string               `yaml:"imageTag"`
	Env      map[string]string    `yaml:"env"`
	Mounts   []ServiceMountConfig `yaml:"mounts"`
}

type ServiceMountConfig struct {
	Host      string `yaml:"host"`
	Container string `yaml:"container"`
}

type ProcessCaddyConfig struct {
	Hostnames []string `yaml:"hostnames"`
}

type NetworkMode string

var (
	NetworkModeDefault NetworkMode = ""
	NetworkModeHost    NetworkMode = "host"
)

func (nm NetworkMode) IsHost() bool {
	return nm == NetworkModeHost
}

type ProcessNetworkConfig struct {
	Mode NetworkMode `yaml:"mode"`
}

type ServiceProcessConfig struct {
	Command  []string             `yaml:"command"`
	Quantity uint                 `yaml:"quantity"`
	Env      map[string]string    `yaml:"env"`
	Mounts   []ServiceMountConfig `yaml:"mounts"`
	Caddy    ProcessCaddyConfig   `yaml:"caddy"`

	// Privileged grants all capabilities to the container.
	Privileged bool `yaml:"privileged"`

	Network ProcessNetworkConfig `yaml:"network"`
}

type TaskNetworkConfig struct {
	Mode NetworkMode `yaml:"mode"`
}

type ServiceTaskConfig struct {
	Image       string               `yaml:"image"`
	ImageTag    string               `yaml:"imageTag"`
	Command     []string             `yaml:"command"`
	Interactive bool                 `yaml:"interactive"`
	Env         map[string]string    `yaml:"env"`
	Mounts      []ServiceMountConfig `yaml:"mounts"`
	Network     TaskNetworkConfig    `yaml:"network"`
}

var (
	ErrMultipleServices = errors.New("multiple services found, no default")
	ErrNoService        = errors.New("no service found")
)

func findDefaultService(configPath string) (string, error) {
	entries, err := os.ReadDir(configPath)
	if err != nil {
		return "", err
	}

	serviceName := ""
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		isYaml := strings.HasSuffix(entry.Name(), ".yaml")
		if !isYaml {
			continue
		}

		if serviceName != "" {
			return "", ErrMultipleServices
		}

		serviceName = strings.TrimSuffix(entry.Name(), ".yaml")
	}

	if serviceName == "" {
		return "", ErrNoService
	}

	return serviceName, nil
}

func (e *Engine) loadServiceConfig(serviceName string) (*ServiceConfig, error) {
	if serviceName == "" {
		var err error
		serviceName, err = findDefaultService(e.config.Paths.Config)
		if err != nil {
			return nil, err
		}
		e.log.Debug(
			"no service specified, defaulting",
			zap.String("default", serviceName),
		)
	}

	svcPath := path.Join(
		e.config.Paths.Config,
		fmt.Sprintf("%s.yaml", serviceName),
	)
	bytes, err := os.ReadFile(svcPath)
	if err != nil {
		return nil, err
	}

	cfg := &ServiceConfig{}
	if err := yaml.Unmarshal(bytes, cfg); err != nil {
		return nil, err
	}

	cfg.Name = serviceName

	return cfg, nil
}
