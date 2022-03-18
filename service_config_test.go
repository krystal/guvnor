package guvnor

import (
	"testing"
	"time"

	"github.com/docker/docker/api/types/mount"
	"github.com/stretchr/testify/assert"
)

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

func Test_ServiceProcessConfig_GetQuantity(t *testing.T) {
	tests := []struct {
		name string
		spc  ServiceProcessConfig
		want int
	}{
		{
			name: "fallback",
			spc:  ServiceProcessConfig{},
			want: 1,
		},
		{
			name: "overriden",
			spc: ServiceProcessConfig{
				Quantity: 12,
			},
			want: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.spc.GetQuantity()
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_ServiceProcessConfig_GetImage(t *testing.T) {
	tests := []struct {
		name    string
		spc     ServiceProcessConfig
		want    string
		wantErr string
	}{
		{
			name: "fallback",
			spc: ServiceProcessConfig{
				parent: &ServiceConfig{
					Defaults: ServiceDefaultsConfig{
						Image:    "foo",
						ImageTag: "bar",
					},
				},
			},
			want: "foo:bar",
		},
		{
			name: "overriden",
			spc: ServiceProcessConfig{
				parent: &ServiceConfig{
					Defaults: ServiceDefaultsConfig{
						Image:    "foo",
						ImageTag: "bar",
					},
				},
				Image:    "fizz",
				ImageTag: "buzz",
			},
			want: "fizz:buzz",
		},
		{
			name: "unspecified imageTag",
			spc: ServiceProcessConfig{
				parent: &ServiceConfig{
					Defaults: ServiceDefaultsConfig{
						Image:    "foo",
						ImageTag: "bar",
					},
				},
				Image: "fizz",
			},
			wantErr: "imageTag must be specified when image specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.spc.GetImage()
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_ServiceTaskConfig_GetImage(t *testing.T) {
	tests := []struct {
		name    string
		stc     ServiceTaskConfig
		want    string
		wantErr string
	}{
		{
			name: "fallback",
			stc: ServiceTaskConfig{
				parent: &ServiceConfig{
					Defaults: ServiceDefaultsConfig{
						Image:    "foo",
						ImageTag: "bar",
					},
				},
			},
			want: "foo:bar",
		},
		{
			name: "overriden",
			stc: ServiceTaskConfig{
				parent: &ServiceConfig{
					Defaults: ServiceDefaultsConfig{
						Image:    "foo",
						ImageTag: "bar",
					},
				},
				Image:    "fizz",
				ImageTag: "buzz",
			},
			want: "fizz:buzz",
		},
		{
			name: "unspecified imageTag",
			stc: ServiceTaskConfig{
				parent: &ServiceConfig{
					Defaults: ServiceDefaultsConfig{
						Image:    "foo",
						ImageTag: "bar",
					},
				},
				Image: "fizz",
			},
			wantErr: "imageTag must be specified when image specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.stc.GetImage()
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_ServiceProcessConfig_GetMounts(t *testing.T) {
	tests := []struct {
		name string
		spc  ServiceProcessConfig
		want []mount.Mount
	}{
		{
			name: "merged",
			spc: ServiceProcessConfig{
				parent: &ServiceConfig{
					Defaults: ServiceDefaultsConfig{
						Mounts: []ServiceMountConfig{
							{
								Host:      "/host/path/a",
								Container: "/container/path/a",
							},
						},
					},
				},
				Mounts: []ServiceMountConfig{
					{
						Host:      "/host/path/b",
						Container: "/container/path/b",
					},
				},
			},
			want: []mount.Mount{
				{
					Type:   "bind",
					Source: "/host/path/a",
					Target: "/container/path/a",
				},
				{
					Type:   "bind",
					Source: "/host/path/b",
					Target: "/container/path/b",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.spc.GetMounts()
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func Test_ServiceProcessConfig_GetShutdownGracePeriod(t *testing.T) {
	tests := []struct {
		name string
		spc  ServiceProcessConfig
		want time.Duration
	}{
		{
			name: "default",
			spc:  ServiceProcessConfig{},
			want: time.Minute,
		},
		{
			name: "value specified",
			spc: ServiceProcessConfig{
				ShutdownGracePeriod: time.Second * 12,
			},
			want: time.Second * 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.spc.GetShutdownGracePeriod()
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_ServiceTaskConfig_GetMounts(t *testing.T) {
	tests := []struct {
		name string
		stc  ServiceTaskConfig
		want []mount.Mount
	}{
		{
			name: "merged",
			stc: ServiceTaskConfig{
				parent: &ServiceConfig{
					Defaults: ServiceDefaultsConfig{
						Mounts: []ServiceMountConfig{
							{
								Host:      "/host/path/a",
								Container: "/container/path/a",
							},
						},
					},
				},
				Mounts: []ServiceMountConfig{
					{
						Host:      "/host/path/b",
						Container: "/container/path/b",
					},
				},
			},
			want: []mount.Mount{
				{
					Type:   "bind",
					Source: "/host/path/a",
					Target: "/container/path/a",
				},
				{
					Type:   "bind",
					Source: "/host/path/b",
					Target: "/container/path/b",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.stc.GetMounts()
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func Test_ServiceTaskConfig_GetNetworkMode(t *testing.T) {
	var otherNetworkMode NetworkMode = "other"
	tests := []struct {
		name string
		stc  ServiceTaskConfig
		want NetworkMode
	}{
		{
			name: "fallback",
			stc: ServiceTaskConfig{
				parent: &ServiceConfig{
					Defaults: ServiceDefaultsConfig{
						Network: NetworkConfig{},
					},
				},
			},
			want: NetworkModeDefault,
		},
		{
			name: "defaults",
			stc: ServiceTaskConfig{
				parent: &ServiceConfig{
					Defaults: ServiceDefaultsConfig{
						Network: NetworkConfig{
							Mode: &otherNetworkMode,
						},
					},
				},
			},
			want: otherNetworkMode,
		},
		{
			name: "overriden",
			stc: ServiceTaskConfig{
				parent: &ServiceConfig{
					Defaults: ServiceDefaultsConfig{
						Network: NetworkConfig{
							Mode: &otherNetworkMode,
						},
					},
				},
				Network: NetworkConfig{
					Mode: &NetworkModeHost,
				},
			},
			want: NetworkModeHost,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.stc.GetNetworkMode()
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_ServiceProcessConfig_GetNetworkMode(t *testing.T) {
	var otherNetworkMode NetworkMode = "other"
	tests := []struct {
		name string
		spc  ServiceProcessConfig
		want NetworkMode
	}{
		{
			name: "fallback",
			spc: ServiceProcessConfig{
				parent: &ServiceConfig{
					Defaults: ServiceDefaultsConfig{
						Network: NetworkConfig{},
					},
				},
			},
			want: NetworkModeDefault,
		},
		{
			name: "defaults",
			spc: ServiceProcessConfig{
				parent: &ServiceConfig{
					Defaults: ServiceDefaultsConfig{
						Network: NetworkConfig{
							Mode: &otherNetworkMode,
						},
					},
				},
			},
			want: otherNetworkMode,
		},
		{
			name: "overriden",
			spc: ServiceProcessConfig{
				parent: &ServiceConfig{
					Defaults: ServiceDefaultsConfig{
						Network: NetworkConfig{
							Mode: &otherNetworkMode,
						},
					},
				},
				Network: NetworkConfig{
					Mode: &NetworkModeHost,
				},
			},
			want: NetworkModeHost,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.spc.GetNetworkMode()
			assert.Equal(t, tt.want, got)
		})
	}
}
