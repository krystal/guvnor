package caddy

import (
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/stretchr/testify/assert"
)

func TestManager_calculateConfigChanges(t *testing.T) {
	cm := Manager{
		Config: Config{
			Ports: PortsConfig{
				HTTP:  80,
				HTTPS: 443,
			},
		},
	}

	tests := []struct {
		name   string
		config *caddy.Config

		want        *caddy.Config
		wantErr     string
		wantChanged bool
	}{
		{
			name:        "blank to full",
			wantChanged: true,
			config:      &caddy.Config{},
			want: &caddy.Config{
				AppsRaw: caddy.ModuleMap{
					"http": []byte("{\"http_port\":80,\"https_port\":443,\"servers\":{\"guvnor\":{\"listen\":[\":443\"],\"routes\":[{\"handle\":[{\"body\":\"Welcome to Guvnor. We found no backend matching your request.\",\"handler\":\"static_response\",\"status_code\":\"404\"}]}]}}}"),
				},
			},
		},
		{
			name:        "no change",
			wantChanged: false,
			config: &caddy.Config{
				AppsRaw: caddy.ModuleMap{
					"http": []byte("{\"http_port\":80,\"https_port\":443,\"servers\":{\"guvnor\":{\"listen\":[\":443\"],\"routes\":[{\"handle\":[{\"body\":\"Welcome to Guvnor. We found no backend matching your request.\",\"handler\":\"static_response\",\"status_code\":\"404\"}]}]}}}"),
				},
			},
			want: &caddy.Config{
				AppsRaw: caddy.ModuleMap{
					"http": []byte("{\"http_port\":80,\"https_port\":443,\"servers\":{\"guvnor\":{\"listen\":[\":443\"],\"routes\":[{\"handle\":[{\"body\":\"Welcome to Guvnor. We found no backend matching your request.\",\"handler\":\"static_response\",\"status_code\":\"404\"}]}]}}}"),
				},
			},
		},
		{
			name:        "update ports",
			wantChanged: true,
			config: &caddy.Config{
				AppsRaw: caddy.ModuleMap{
					"http": []byte("{\"http_port\":8080,\"https_port\":1443,\"servers\":{\"guvnor\":{\"listen\":[\":443\"],\"routes\":[{\"handle\":[{\"body\":\"Welcome to Guvnor. We found no backend matching your request.\",\"handler\":\"static_response\",\"status_code\":\"404\"}]}]}}}"),
				},
			},
			want: &caddy.Config{
				AppsRaw: caddy.ModuleMap{
					"http": []byte("{\"http_port\":80,\"https_port\":443,\"servers\":{\"guvnor\":{\"listen\":[\":443\"],\"routes\":[{\"handle\":[{\"body\":\"Welcome to Guvnor. We found no backend matching your request.\",\"handler\":\"static_response\",\"status_code\":\"404\"}]}]}}}"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotChanged, err := cm.calculateConfigChanges(tt.config)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.wantChanged, gotChanged)
			assert.Equal(t, tt.want, tt.config)
		})
	}
}

func TestManager_generateRouteForBackend(t *testing.T) {
	cm := Manager{}

	got := cm.generateRouteforBackend(
		"fizzbuzz",
		[]string{"foo.com", "buzz.com"},
		[]string{"1337", "8080"},
		"/path",
	)

	want := route{
		Group:    "fizzbuzz",
		Terminal: true,
		MatcherSets: []matcherSet{
			{
				Host: []string{"foo.com", "buzz.com"},
				Path: []string{"/path"},
			},
		},
		Handlers: []handler{
			reverseProxyHandler{
				Upstreams: []upstream{
					{
						Dial: "localhost:1337",
					},
					{
						Dial: "localhost:8080",
					},
				},
			},
		},
	}

	assert.Equal(t, want, got)
}

func Test_sortRoutes(t *testing.T) {
	routes := []route{
		{
			Group: "fallback",
		},
		{
			MatcherSets: []matcherSet{
				{
					Host: []string{"foo.com"},
					Path: []string{"/path/fizz"},
				},
			},
		},
		{
			MatcherSets: []matcherSet{
				{
					Host: []string{"foo.com"},
				},
			},
		},
		{
			MatcherSets: []matcherSet{
				{
					Host: []string{"foo.com"},
					Path: []string{"/path"},
				},
			},
		},
	}

	sortRoutes(routes)

	want := []route{
		{
			MatcherSets: []matcherSet{
				{
					Host: []string{"foo.com"},
					Path: []string{"/path/fizz"},
				},
			},
		},
		{
			MatcherSets: []matcherSet{
				{
					Host: []string{"foo.com"},
					Path: []string{"/path"},
				},
			},
		},
		{
			MatcherSets: []matcherSet{
				{
					Host: []string{"foo.com"},
				},
			},
		},
		{
			Group: "fallback",
		},
	}
	assert.Equal(t, want, routes)
}
