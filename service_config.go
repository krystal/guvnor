package guvnor

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/krystal/guvnor/ready"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type ServiceConfig struct {
	// Name is the unique identifier of the service, usually the name of the
	// file it has been retrieved from.
	Name string `yaml:"_"`
	// Defaults is a series of configuration values to use by default in
	// configuring process and task containers.
	Defaults ServiceDefaultsConfig `yaml:"defaults"`

	// Processes is a map of process name to configuration to deploy as part of
	// this service.
	Processes map[string]ServiceProcessConfig `yaml:"processes" validate:"dive"`
	// Tasks is a map of task names to configuration that are available for
	// invoking as part of this service.
	Tasks map[string]ServiceTaskConfig `yaml:"tasks" validate:"dive"`

	// Callbacks are definitions of Tasks to run when specific events occur,
	// e.g before a deployment.
	Callbacks ServiceCallbacksConfig `yaml:"callbacks"`
}

func (sc *ServiceConfig) Validate(v *validator.Validate) error {
	if err := v.Struct(sc); err != nil {
		return err
	}

	// call custom validations
	return sc.validateCallbacks()
}

// validateCallbacks ensures all callbacks are valid tasks
func (sc *ServiceConfig) validateCallbacks() error {
	for _, set := range [][]string{
		sc.Callbacks.PostDeployment,
		sc.Callbacks.PreDeployment,
	} {
		for _, taskName := range set {
			task, ok := sc.Tasks[taskName]
			if !ok {
				return fmt.Errorf(
					"task (%s) specified in callback not found",
					taskName,
				)
			}

			if task.Interactive {
				return fmt.Errorf(
					"interactive tasks may not be callbacks (%s)",
					taskName,
				)
			}
		}
	}

	return nil
}

type ServiceCallbacksConfig struct {
	PreDeployment  []string `yaml:"preDeployment"`
	PostDeployment []string `yaml:"postDeployment"`
}

type ServiceDefaultsConfig struct {
	Image    string               `yaml:"image"`
	ImageTag string               `yaml:"imageTag"`
	Env      map[string]string    `yaml:"env"`
	Mounts   []ServiceMountConfig `yaml:"mounts"`
	Network  NetworkConfig        `yaml:"network"`

	// User allows the default User/Group to be specified for task and
	// process containers.
	//
	// The following formats are valid:
	// [ user | user:group | uid | uid:gid | user:gid | uid:group ]
	User string `yaml:"user"`
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

func (nm *NetworkMode) IsHost(defaultConfig *NetworkMode) bool {
	if nm != nil {
		return *nm == NetworkModeHost
	}

	if defaultConfig != nil {
		return *defaultConfig == NetworkModeHost
	}

	return false
}

type NetworkConfig struct {
	Mode *NetworkMode `yaml:"mode"`
}

type ServiceProcessConfig struct {
	Image    string               `yaml:"image"`
	ImageTag string               `yaml:"imageTag"`
	Command  []string             `yaml:"command"`
	Quantity int                  `yaml:"quantity"`
	Env      map[string]string    `yaml:"env"`
	Mounts   []ServiceMountConfig `yaml:"mounts"`
	Caddy    ProcessCaddyConfig   `yaml:"caddy"`

	// Privileged grants all capabilities to the container.
	Privileged bool `yaml:"privileged"`

	// User allows the User/Group to be configured for the process container.
	//
	// The following formats are valid:
	// [ user | user:group | uid | uid:gid | user:gid | uid:group ]
	User string `yaml:"user"`

	Network    NetworkConfig `yaml:"network"`
	ReadyCheck *ready.Check  `yaml:"readyCheck"`
}

func (spc ServiceProcessConfig) GetQuantity() int {
	if spc.Quantity != 0 {
		return spc.Quantity
	}

	return 1
}

type ServiceTaskConfig struct {
	Image       string               `yaml:"image"`
	ImageTag    string               `yaml:"imageTag"`
	Command     []string             `yaml:"command"`
	Interactive bool                 `yaml:"interactive"`
	Env         map[string]string    `yaml:"env"`
	Mounts      []ServiceMountConfig `yaml:"mounts"`
	Network     NetworkConfig        `yaml:"network"`

	// User allows the User/Group to be configured for the task container.
	//
	// The following formats are valid:
	// [ user | user:group | uid | uid:gid | user:gid | uid:group ]
	User string `yaml:"user"`
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
	configData, err := os.ReadFile(svcPath)
	if err != nil {
		return nil, err
	}

	decoder := yaml.NewDecoder(bytes.NewBuffer(configData))
	decoder.KnownFields(true)

	cfg := &ServiceConfig{}
	if err := decoder.Decode(cfg); err != nil {
		return nil, err
	}

	cfg.Name = serviceName

	if err := cfg.Validate(e.validate); err != nil {
		return nil, err
	}

	return cfg, nil
}
