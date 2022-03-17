package guvnor

import (
	"fmt"
)

type DeploymentStrategy int

const (
	// DefaultStrategy
	//
	// Start new replica of process
	// Wait for that replica to become healthy
	// Direct traffic towards the new replica
	// Send SIGTERM to an old replica of process
	// Repeat until the count of new replicas meets the specified quantity
	// Clear up any remaining old replicas
	DefaultStrategy DeploymentStrategy = iota
	// ReplaceStrategy
	//
	// Remove an existing replica of the process from the loadbalancer
	// Send SIGTERM to the old replica
	// Wait for it stop, kiling it after X seconds if not stopped.
	// Start new replica of the process
	// Wait for it to become healthy
	// Direct traffic towards the new replica
	// Repeat until the count of new replicas meets the specified quantity
	// Clear up any remaining old replicas
	ReplaceStrategy
)

func (s DeploymentStrategy) String() string {
	return strategyToString[s]
}

var strategyToString = map[DeploymentStrategy]string{
	DefaultStrategy: "default",
	ReplaceStrategy: "replace",
}

var stringToStrategy = map[string]DeploymentStrategy{
	"default": DefaultStrategy,
	"replace": ReplaceStrategy,
}

func (s DeploymentStrategy) MarshalYAML() (interface{}, error) {
	return strategyToString[s], nil
}

func (s *DeploymentStrategy) UnmarshalYAML(unmarshal func(interface{}) error) error {
	stringValue := ""
	if err := unmarshal(&stringValue); err != nil {
		return err
	}

	strategy, ok := stringToStrategy[stringValue]
	if !ok {
		return fmt.Errorf(
			"deployment strategy '%s' not recognised", stringValue,
		)
	}
	*s = strategy

	return nil
}
