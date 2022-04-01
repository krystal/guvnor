package guvnor

import (
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
)

func Test_getIndexforImage(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  string
	}{
		{
			name:  "fully qualified",
			image: "ghcr.io/strideynet/fizz:buzz",
			want:  "ghcr.io",
		},
		{
			name:  "short",
			image: "ubuntu",
			want:  "docker.io",
		},
	}

	for _, tt := range tests {
		t.Run(t.Name(), func(t *testing.T) {
			got, err := getIndexForImage(tt.image)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_extractAuthCfg(t *testing.T) {
	tests := []struct {
		name      string
		dockerCfg []byte
		want      *types.AuthConfig
		wantErr   string
	}{
		{
			name:      "blank docker config",
			dockerCfg: []byte("{}"),
			wantErr:   "no auth configured for registry",
		},
		{
			name:      "no auth configured for registry",
			wantErr:   "no auth configured for registry",
			dockerCfg: []byte(`{"auths":{}}`),
		},
		{
			name:      "user/password provided",
			dockerCfg: []byte(`{"auths":{"test.com":{"username":"user","password":"pass"}}}`),
			want: &types.AuthConfig{
				ServerAddress: "test.com",
				Username:      "user",
				Password:      "pass",
			},
		},
		{
			name:      "auth provided",
			dockerCfg: []byte(`{"auths":{"test.com":{"auth":"dXNlcjpwYXNz"}}}`),
			want: &types.AuthConfig{
				ServerAddress: "test.com",
				Username:      "user",
				Password:      "pass",
			},
		},
		{
			name:      "malformed auth provided",
			dockerCfg: []byte(`{"auths":{"test.com":{"auth":"dXNlcjpwYXNzOmhlaA=="}}}`),
			wantErr:   "auth string malformed, expected 2 parts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractAuthCfg(tt.dockerCfg, "test.com")
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
