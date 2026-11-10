package guvnor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_listenIPs(t *testing.T) {
	tests := []struct {
		name    string
		caddy   string
		want    []string
		wantErr string
	}{
		{
			name:  "ipv4 and ipv6",
			caddy: "  listenIPs:\n    - 192.0.2.10\n    - 2001:db8::10\n",
			want:  []string{"192.0.2.10", "2001:db8::10"},
		},
		{
			name:    "address with a port",
			caddy:   "  listenIPs:\n    - 192.0.2.10:443\n",
			wantErr: "'ip' tag",
		},
		{
			name:    "combined with listenIP",
			caddy:   "  listenIP: 192.0.2.10\n  listenIPs:\n    - 2001:db8::10\n",
			wantErr: "'excluded_with' tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			data := "caddy:\n" + tt.caddy +
				"paths:\n  config: /etc/guvnor/services\n  state: /var/lib/guvnor\n"
			require.NoError(t, os.WriteFile(path, []byte(data), 0o600))

			cfg, err := LoadConfig(validator.New(), path)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, cfg.Caddy.ListenIPs)
		})
	}
}
