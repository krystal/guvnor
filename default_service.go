package guvnor

import (
	"errors"
	"os"
	"strings"
)

type GetDefaultServiceResult struct {
	Name string
}

var (
	ErrMultipleServices = errors.New("multiple services found, no default")
	ErrNoService        = errors.New("no service found")
)

func (e *Engine) GetDefaultService() (*GetDefaultServiceResult, error) {
	entries, err := os.ReadDir(e.config.Paths.Config)
	if err != nil {
		return nil, err
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
			return nil, ErrMultipleServices
		}

		serviceName = strings.TrimSuffix(entry.Name(), ".yaml")
	}

	if serviceName == "" {
		return nil, ErrNoService
	}

	return &GetDefaultServiceResult{Name: serviceName}, nil
}
