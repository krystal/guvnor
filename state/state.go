package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"time"

	"go.uber.org/zap"
)

type DeploymentStatus string

var (
	StatusSuccess DeploymentStatus = "SUCCESS"
	StatusFailure DeploymentStatus = "FAILURE"
)

type FileBasedStore struct {
	RootPath string
	Log      *zap.Logger
}

type ServiceState struct {
	DeploymentID     int              `json:"deploymentID"`
	LastDeployedAt   time.Time        `json:"lastDeployedAt"`
	DeploymentStatus DeploymentStatus `json:"deploymentStatus"`
}

func (fbs *FileBasedStore) servicePath(service string) string {
	return path.Join(fbs.RootPath, fmt.Sprintf("%s.json", service))
}

func (fbs *FileBasedStore) LoadServiceState(service string) (*ServiceState, error) {
	data, err := os.ReadFile(fbs.servicePath(service))
	if errors.Is(err, os.ErrNotExist) {
		// Return default state
		return &ServiceState{
			DeploymentID: 0,
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

func (fbs *FileBasedStore) SaveServiceState(service string, state *ServiceState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return os.WriteFile(fbs.servicePath(service), data, 0o644)
}

func (fbs *FileBasedStore) Purge() error {
	fbs.Log.Debug("purging state")
	files, err := os.ReadDir(fbs.RootPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		fullPath := path.Join(fbs.RootPath, file.Name())
		fbs.Log.Debug("purging file", zap.String("path", fullPath))
		if err := os.Remove(fullPath); err != nil {
			return err
		}
	}

	return nil
}
