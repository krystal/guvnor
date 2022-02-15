package guvnor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_containerFullName(t *testing.T) {
	serviceName := "foo"
	deploymentID := 3
	processName := "web"
	count := 0

	got := containerFullName(serviceName, deploymentID, processName, count)
	assert.Equal(t, "foo-web-3-0", got)
}

func Test_mergeEnv(t *testing.T) {
	tests := []struct {
		name string
		a, b map[string]string
		want []string
	}{
		{
			name: "merge",
			a: map[string]string{
				"aOnly":     "foo",
				"overrided": "foo",
			},
			b: map[string]string{
				"bOnly":     "bar",
				"overrided": "bar",
			},
			want: []string{
				"aOnly=foo",
				"bOnly=bar",
				"overrided=bar",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := mergeEnv(tt.a, tt.b)
			assert.ElementsMatch(t, tt.want, out)
		})
	}
}

func Test_mergeMounts(t *testing.T) {
	tests := []struct {
		name string
		a, b []ServiceMountConfig
		want []ServiceMountConfig
	}{
		{
			name: "only in a",
			a: []ServiceMountConfig{
				{
					Host:      "/path/on/host/a",
					Container: "/path/on/container",
				},
			},
			b: []ServiceMountConfig{},
			want: []ServiceMountConfig{
				{
					Host:      "/path/on/host/a",
					Container: "/path/on/container",
				},
			},
		},
		{
			name: "only in b",
			a:    []ServiceMountConfig{},
			b: []ServiceMountConfig{
				{
					Host:      "/path/on/host/b",
					Container: "/path/on/container",
				},
			},
			want: []ServiceMountConfig{
				{
					Host:      "/path/on/host/b",
					Container: "/path/on/container",
				},
			},
		},
		{
			name: "both",
			a: []ServiceMountConfig{
				{
					Host:      "/path/on/host/a",
					Container: "/path/on/container/a",
				},
			},
			b: []ServiceMountConfig{
				{
					Host:      "/path/on/host/b",
					Container: "/path/on/container/b",
				},
			},
			want: []ServiceMountConfig{
				{
					Host:      "/path/on/host/a",
					Container: "/path/on/container/a",
				},
				{
					Host:      "/path/on/host/b",
					Container: "/path/on/container/b",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := mergeMounts(tt.a, tt.b)
			assert.ElementsMatch(t, tt.want, out)
		})
	}
}
