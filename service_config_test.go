package guvnor

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_findDefaultService(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr string
	}{
		{
			name: "valid",
			want: "one",
		},
		{
			name:    "multiple",
			wantErr: ErrMultipleServices.Error(),
		},
		{
			name:    "none",
			wantErr: ErrNoService.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDataPath := path.Join("./testdata/", t.Name())
			got, gotErr := findDefaultService(testDataPath)
			if tt.wantErr != "" {
				assert.EqualError(t, gotErr, tt.wantErr)
			} else {
				assert.NoError(t, gotErr)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_ServiceProcessConfig_GetUser(t *testing.T) {
	tests := []struct {
		name string
		spc  ServiceProcessConfig
		want string
	}{
		{
			name: "fallback",
			spc: ServiceProcessConfig{
				parent: &ServiceConfig{
					Defaults: ServiceDefaultsConfig{
						User: "fizz",
					},
				},
			},
			want: "fizz",
		},
		{
			name: "overriden",
			spc: ServiceProcessConfig{
				parent: &ServiceConfig{
					Defaults: ServiceDefaultsConfig{
						User: "fizz",
					},
				},
				User: "buzz",
			},
			want: "buzz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.spc.GetUser()
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_ServiceTaskConfig_GetUser(t *testing.T) {
	tests := []struct {
		name string
		stc  ServiceTaskConfig
		want string
	}{
		{
			name: "fallback",
			stc: ServiceTaskConfig{
				parent: &ServiceConfig{
					Defaults: ServiceDefaultsConfig{
						User: "fizz",
					},
				},
			},
			want: "fizz",
		},
		{
			name: "overriden",
			stc: ServiceTaskConfig{
				parent: &ServiceConfig{
					Defaults: ServiceDefaultsConfig{
						User: "fizz",
					},
				},
				User: "buzz",
			},
			want: "buzz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.stc.GetUser()
			assert.Equal(t, tt.want, got)
		})
	}
}
