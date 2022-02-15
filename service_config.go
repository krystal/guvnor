package guvnor

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/pkg/errors"
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

type ServiceProcessConfig struct {
	Command  []string             `yaml:"command"`
	Quantity uint                 `yaml:"quantity"`
	Env      map[string]string    `yaml:"env"`
	Mounts   []ServiceMountConfig `yaml:"mounts"`
	Caddy    ProcessCaddyConfig   `yaml:"caddy"`
}

type ServiceTaskConfig struct {
	Image       string               `yaml:"image"`
	ImageTag    string               `yaml:"imageTag"`
	Command     []string             `yaml:"command"`
	Interactive bool                 `yaml:"interactive"`
	Env         map[string]string    `yaml:"env"`
	Mounts      []ServiceMountConfig `yaml:"mounts"`
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

func loadServiceConfig(
	configPath string,
	serviceName string,
) (*ServiceConfig, error) {
	svcPath := path.Join(configPath, fmt.Sprintf("%s.yaml", serviceName))
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
