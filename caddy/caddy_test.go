package caddy

import (
	"context"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestManager_calculateConfigChanges(t *testing.T) {
	defaultConfig := Config{
		Ports: PortsConfig{
			HTTP:  80,
			HTTPS: 443,
		},
	}

	tests := []struct {
		name     string
		config   Config
		existing *caddy.Config

		want        *caddy.Config
		wantErr     string
		wantChanged bool
	}{
		{
			name:        "blank to full",
			config:      defaultConfig,
			wantChanged: true,
			existing:    &caddy.Config{},
			want: &caddy.Config{
				AppsRaw: caddy.ModuleMap{
					"http": []byte("{\"http_port\":80,\"https_port\":443,\"servers\":{\"guvnor\":{\"listen\":[\":443\"],\"routes\":[{\"handle\":[{\"body\":\"Welcome to Guvnor. We found no backend matching your request.\",\"handler\":\"static_response\",\"status_code\":\"404\"}]}]}}}"),
				},
			},
		},
		{
			name:        "no change",
			config:      defaultConfig,
			wantChanged: false,
			existing: &caddy.Config{
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
			config:      defaultConfig,
			wantChanged: true,
			existing: &caddy.Config{
				AppsRaw: caddy.ModuleMap{
					"http": []byte("{\"http_port\":8080,\"https_port\":1443,\"servers\":{\"guvnor\":{\"listen\":[\":1443\"],\"routes\":[{\"handle\":[{\"body\":\"Welcome to Guvnor. We found no backend matching your request.\",\"handler\":\"static_response\",\"status_code\":\"404\"}]}]}}}"),
				},
			},
			want: &caddy.Config{
				AppsRaw: caddy.ModuleMap{
					"http": []byte("{\"http_port\":80,\"https_port\":443,\"servers\":{\"guvnor\":{\"listen\":[\":443\"],\"routes\":[{\"handle\":[{\"body\":\"Welcome to Guvnor. We found no backend matching your request.\",\"handler\":\"static_response\",\"status_code\":\"404\"}]}]}}}"),
				},
			},
		},
		{
			name: "update listen ip",
			config: Config{
				Ports: PortsConfig{
					HTTP:  80,
					HTTPS: 443,
				},
				ListenIP: "127.0.0.1",
			},
			wantChanged: true,
			existing: &caddy.Config{
				AppsRaw: caddy.ModuleMap{
					"http": []byte("{\"http_port\":8080,\"https_port\":443,\"servers\":{\"guvnor\":{\"listen\":[\":443\"],\"routes\":[{\"handle\":[{\"body\":\"Welcome to Guvnor. We found no backend matching your request.\",\"handler\":\"static_response\",\"status_code\":\"404\"}]}]}}}"),
				},
			},
			want: &caddy.Config{
				AppsRaw: caddy.ModuleMap{
					"http": []byte("{\"http_port\":80,\"https_port\":443,\"servers\":{\"guvnor\":{\"listen\":[\"127.0.0.1:443\"],\"routes\":[{\"handle\":[{\"body\":\"Welcome to Guvnor. We found no backend matching your request.\",\"handler\":\"static_response\",\"status_code\":\"404\"}]}]}}}"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := Manager{
				Config: tt.config,
			}
			gotChanged, err := cm.calculateConfigChanges(tt.existing)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.wantChanged, gotChanged)
			assert.Equal(t, tt.want, tt.existing)
		})
	}
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

type mockCaddyConfigurator struct {
	t *testing.T

	routes       []route
	getRoutesErr error
	setRoutesErr error
}

func (m *mockCaddyConfigurator) getRoutes(ctx context.Context) ([]route, error) {
	assert.NotNil(m.t, ctx)
	return m.routes, m.getRoutesErr
}

func (m *mockCaddyConfigurator) updateRoutes(ctx context.Context, routes []route) error {
	assert.NotNil(m.t, ctx)
	m.routes = routes
	return m.setRoutesErr
}

func (m *mockCaddyConfigurator) getConfig(ctx context.Context) (*caddy.Config, error) {
	assert.NotNil(m.t, ctx)
	panic("not implemented")
}

func (m *mockCaddyConfigurator) updateConfig(ctx context.Context, cfg *caddy.Config) error {
	assert.NotNil(m.t, ctx)
	panic("not implemented")
}

func TestManager_ConfigureBackend(t *testing.T) {
	defaultRoute := route{
		Handlers: handlers{
			staticResponseHandler{
				Body:       "default route",
				StatusCode: "404",
			},
		},
	}
	tests := []struct {
		name string

		routes []route

		backendName string
		hostNames   []string
		upstreams   []string
		path        string

		wantRoutes []route
		wantErr    string
	}{
		{
			name: "new backend",

			routes: []route{
				// Just the default route, we want to ensure this is preserved
				defaultRoute,
			},

			backendName: "fizz",
			hostNames:   []string{"fizz.example.com", "fizz2.example.com"},
			upstreams:   []string{"localhost:1337", "localhost:8080"},
			path:        "/boo",

			wantRoutes: []route{
				{
					Group: "fizz",
					MatcherSets: []matcherSet{
						{
							Host: []string{"fizz.example.com", "fizz2.example.com"},
							Path: []string{"/boo"},
						},
					},
					Handlers: handlers{
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
					Terminal: true,
				},
				defaultRoute,
			},
		},
		{
			name: "update existing backend",

			routes: []route{
				// Include a route we want to ensure isnt edited
				{
					Group: "fubar",
					MatcherSets: []matcherSet{
						{
							Host: []string{"fubar.example.com"},
							Path: []string{"/fubar/boofar"},
						},
					},
					Handlers: handlers{
						reverseProxyHandler{
							Upstreams: []upstream{
								{
									Dial: "localhost:1337",
								},
							},
						},
					},
					Terminal: true,
				},
				// Include a route we want to modify
				{
					Group: "fizz",
					MatcherSets: []matcherSet{
						{
							Host: []string{"fizz.example.com"},
							Path: []string{"/boo"},
						},
					},
					Handlers: handlers{
						reverseProxyHandler{
							Upstreams: []upstream{
								{
									Dial: "localhost:8080",
								},
							},
						},
					},
					Terminal: true,
				},

				defaultRoute,
			},

			backendName: "fizz",
			hostNames:   []string{"fizz.example.net"},
			upstreams:   []string{"localhost:9090"},
			path:        "/fizz",

			wantRoutes: []route{
				{
					Group: "fubar",
					MatcherSets: []matcherSet{
						{
							Host: []string{"fubar.example.com"},
							Path: []string{"/fubar/boofar"},
						},
					},
					Handlers: handlers{
						reverseProxyHandler{
							Upstreams: []upstream{
								{
									Dial: "localhost:1337",
								},
							},
						},
					},
					Terminal: true,
				},
				{
					Group: "fizz",
					MatcherSets: []matcherSet{
						{
							Host: []string{"fizz.example.net"},
							Path: []string{"/fizz"},
						},
					},
					Handlers: handlers{
						reverseProxyHandler{
							Upstreams: []upstream{
								{
									Dial: "localhost:9090",
								},
							},
						},
					},
					Terminal: true,
				},
				defaultRoute,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAdmin := &mockCaddyConfigurator{
				t:      t,
				routes: append([]route{}, tt.routes...),
			}
			cm := Manager{
				CaddyConfigurator: mockAdmin,
				Log:               zaptest.NewLogger(t),
			}

			ctx := context.Background()
			err := cm.ConfigureBackend(ctx, tt.backendName, tt.hostNames, tt.upstreams, tt.path)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.wantRoutes, mockAdmin.routes)
		})
	}
}
