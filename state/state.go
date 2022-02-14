package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
)

type FileBasedStore struct {
	RootPath string
}

type ServiceState struct {
	LastDeploymentID int `json:"lastDeploymentId"`
}

func (fbs *FileBasedStore) LoadServiceState(service string) (*ServiceState, error) {
	fullPath := path.Join(fbs.RootPath, fmt.Sprintf("%s.json", service))

	data, err := os.ReadFile(fullPath)
	if errors.Is(err, os.ErrNotExist) {
		// Return default state
		return &ServiceState{
			LastDeploymentID: 0,
		}, nil
	} else if err != nil {
		return nil, err
	}

	out := &ServiceState{}
	if err := json.Unmarshal(data, out); err != nil {
		return nil, err
	}

	return out, nil
}

func (fbs *FileBasedStore) SetDeploymentID(service string, deploymentID int) error {
	return nil
}
