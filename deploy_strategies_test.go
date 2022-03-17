package guvnor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDeploymentStrategy_String(t *testing.T) {
	tests := []struct {
		strategy DeploymentStrategy
		want     string
	}{
		{
			strategy: DefaultStrategy,
			want:     "default",
		},
		{
			strategy: ReplaceStrategy,
			want:     "replace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.strategy.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDeploymentStrategy_MarshalYAML(t *testing.T) {
	type testStruct struct {
		DeploymentStrategy DeploymentStrategy `yaml:"deploymentStrategy"`
	}

	tests := []struct {
		name      string
		toMarshal testStruct
		want      string
	}{
		{
			name: "standard",
			toMarshal: testStruct{
				DeploymentStrategy: ReplaceStrategy,
			},
			want: "deploymentStrategy: replace\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := yaml.Marshal(tt.toMarshal)
			require.NoError(t, err)

			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestDeploymentStrategy_UnmarshalYAML(t *testing.T) {
	type testStruct struct {
		DeploymentStrategy DeploymentStrategy `yaml:"deploymentStrategy"`
	}

	tests := []struct {
		name    string
		data    string
		want    testStruct
		wantErr string
	}{
		{
			name: "success",
			data: "deploymentStrategy: replace\n",
			want: testStruct{
				DeploymentStrategy: ReplaceStrategy,
			},
		},
		{
			name:    "unknown type",
			data:    "deploymentStrategy: buzzcock\n",
			want:    testStruct{},
			wantErr: "deployment strategy 'buzzcock' not recognised",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := testStruct{}
			err := yaml.Unmarshal([]byte(tt.data), &got)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.wantErr)
			}

			assert.Equal(t, tt.want, got)
		})
	}
}
