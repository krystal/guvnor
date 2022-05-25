package guvnor

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/docker/docker/api/types/mount"
	"github.com/go-playground/validator/v10"
	"github.com/krystal/guvnor/ready"
	"github.com/pkg/errors"
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
	Image     string               `yaml:"image"`
	ImageTag  string               `yaml:"imageTag"`
	ImagePull *bool                `yaml:"imagePull,omitempty"`
	Env       map[string]string    `yaml:"env"`
	Mounts    []ServiceMountConfig `yaml:"mounts"`
	Network   NetworkConfig        `yaml:"network"`

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
	Path      string   `yaml:"path"`
}

type NetworkMode string

var (
	NetworkModeDefault NetworkMode = ""
	NetworkModeHost    NetworkMode = "host"
)

type NetworkConfig struct {
	Mode *NetworkMode `yaml:"mode"`
}

type ServiceProcessConfig struct {
	parent *ServiceConfig `yaml:"_"`
	name   string         `yaml:"_"`

	Image     string               `yaml:"image"`
	ImageTag  string               `yaml:"imageTag"`
	ImagePull *bool                `yaml:"imagePull,omitempty"`
	Command   []string             `yaml:"command"`
	Quantity  int                  `yaml:"quantity"`
	Env       map[string]string    `yaml:"env"`
	Mounts    []ServiceMountConfig `yaml:"mounts"`
	Caddy     ProcessCaddyConfig   `yaml:"caddy"`

	// Privileged grants all capabilities to the container.
	Privileged bool `yaml:"privileged"`

	// User allows the User/Group to be configured for the process container.
	//
	// The following formats are valid:
	// [ user | user:group | uid | uid:gid | user:gid | uid:group ]
	User string `yaml:"user"`

	Network    NetworkConfig `yaml:"network"`
	ReadyCheck *ready.Check  `yaml:"readyCheck"`

	// TODO: add validation to constrain this value
	DeploymentStrategy  DeploymentStrategy `yaml:"deploymentStrategy"`
	ShutdownGracePeriod time.Duration      `yaml:"shutdownGracePeriod"`
}

func (spc ServiceProcessConfig) GetShutdownGracePeriod() time.Duration {
	if spc.ShutdownGracePeriod == time.Duration(0) {
		return time.Minute
	}

	return spc.ShutdownGracePeriod
}

func (spc ServiceProcessConfig) GetQuantity() int {
	if spc.Quantity != 0 {
		return spc.Quantity
	}

	return 1
}

func (spc ServiceProcessConfig) GetUser() string {
	if spc.User != "" {
		return spc.User
	}
	return spc.parent.Defaults.User
}

func (spc ServiceProcessConfig) GetImage() (string, bool, error) {
	pull := true
	if spc.ImagePull != nil {
		pull = *spc.ImagePull
	} else if spc.parent.Defaults.ImagePull != nil {
		pull = *spc.parent.Defaults.ImagePull
	}

	image := fmt.Sprintf(
		"%s:%s",
		spc.parent.Defaults.Image,
		spc.parent.Defaults.ImageTag,
	)
	if spc.Image != "" {
		if spc.ImageTag == "" {
			return "", false, errors.New(
				"imageTag must be specified when image specified",
			)
		}
		image = fmt.Sprintf(
			"%s:%s",
			spc.Image,
			spc.ImageTag,
		)
	}

	return image, pull, nil
}

func (spc ServiceProcessConfig) GetMounts() []mount.Mount {
	mounts := []mount.Mount{}
	for _, mnt := range mergeMounts(
		spc.parent.Defaults.Mounts, spc.Mounts,
	) {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: mnt.Host,
			Target: mnt.Container,
		})
	}

	return mounts
}

func (spc ServiceProcessConfig) GetNetworkMode() NetworkMode {
	if spc.Network.Mode != nil {
		return *spc.Network.Mode
	}

	if spc.parent.Defaults.Network.Mode != nil {
		return *spc.parent.Defaults.Network.Mode
	}

	return NetworkModeDefault
}

type ServiceTaskConfig struct {
	parent *ServiceConfig `yaml:"_"`
	name   string         `yaml:"_"`

	Image       string               `yaml:"image"`
	ImageTag    string               `yaml:"imageTag"`
	ImagePull   *bool                `yaml:"imagePull,omitempty"`
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

func (stc ServiceTaskConfig) GetUser() string {
	if stc.User != "" {
		return stc.User
	}
	return stc.parent.Defaults.User
}

func (stc ServiceTaskConfig) GetImage() (string, bool, error) {
	pull := true
	if stc.ImagePull != nil {
		pull = *stc.ImagePull
	} else if stc.parent.Defaults.ImagePull != nil {
		pull = *stc.parent.Defaults.ImagePull
	}

	image := fmt.Sprintf(
		"%s:%s",
		stc.parent.Defaults.Image,
		stc.parent.Defaults.ImageTag,
	)
	if stc.Image != "" {
		if stc.ImageTag == "" {
			return "", false, errors.New(
				"imageTag must be specified when image specified",
			)
		}
		image = fmt.Sprintf(
			"%s:%s",
			stc.Image,
			stc.ImageTag,
		)
	}

	return image, pull, nil
}

func (stc ServiceTaskConfig) GetMounts() []mount.Mount {
	mounts := []mount.Mount{}
	for _, mnt := range mergeMounts(
		stc.parent.Defaults.Mounts, stc.Mounts,
	) {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: mnt.Host,
			Target: mnt.Container,
		})
	}

	return mounts
}

func (stc ServiceTaskConfig) GetNetworkMode() NetworkMode {
	if stc.Network.Mode != nil {
		return *stc.Network.Mode
	}

	if stc.parent.Defaults.Network.Mode != nil {
		return *stc.parent.Defaults.Network.Mode
	}

	return NetworkModeDefault
}

func (e *Engine) loadServiceConfig(serviceName string) (*ServiceConfig, error) {
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

	for processName, process := range cfg.Processes {
		process.parent = cfg
		process.name = processName
		cfg.Processes[processName] = process
	}

	for taskName, task := range cfg.Tasks {
		task.parent = cfg
		task.name = taskName
		cfg.Tasks[taskName] = task
	}

	return cfg, nil
}
