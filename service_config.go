package guvnor

type ServiceConfig struct {
	Name      string `yaml:"_"`
	StartPort uint   `yaml:"startPort"`

	Processes map[string]ServiceProcessConfig
}

type ServiceDefaultsConfig struct {
	Image    string            `yaml:"image"`
	ImageTag string            `yaml:"imageTag"`
	Env      map[string]string `yaml:"env"`
}

type ServiceMountConfig struct {
	Host      string `yaml:"host"`
	Container string `yaml:"container"`
}

type ServiceProcessConfig struct {
	Command  string
	Quantity uint
	Env      map[string]string
}
