package guvnor

type EngineConfig struct {
	Caddy CaddyConfig `yaml:"caddy"`
	Paths PathsConfig `yaml:"paths"`
}

type CaddyConfig struct {
	// Image is the container image that should be deployed as caddy
	Image string           `yaml:"image"`
	ACME  CaddyACMEConfig  `yaml:"acme"`
	Ports CaddyPortsConfig `yaml:"ports"`
}

type CaddyACMEConfig struct {
	// CA is the URL of the ACME service.
	CA string `yaml:"ca"`
	// Email is the address that should be provided to the acme service for
	// contacting us.
	Email string `yaml:"email"`
}

type CaddyPortsConfig struct {
	HTTP  uint `yaml:"http"`
	HTTPS uint `yaml:"https"`
}

type PathsConfig struct {
	// Config is the path to the directory containing service configs
	Config string `yaml:"config"`
	// State is the path to store state about deployments etc
	State string `yaml:"state"`
}
