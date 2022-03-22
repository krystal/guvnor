package caddy

import (
	"encoding/json"
	"testing"

	"github.com/jimeh/go-golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_route_MarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		route route
	}{
		{
			name: "success",
			route: route{
				Group:    "fizz",
				Terminal: true,
				MatcherSets: []matcherSet{
					{
						Host: []string{"guvnor.k.io"},
					},
					{
						Host: []string{"guvnor.k.io"},
						Path: []string{"/help"},
					},
				},
				Handlers: handlers{
					reverseProxyHandler{
						Upstreams: []upstream{
							{
								Dial: "google.com",
							},
						},
					},
					reverseProxyHandler{
						Upstreams: []upstream{
							{
								Dial: "facebook.com",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.route)
			require.NoError(t, err)

			if golden.Update() {
				golden.Set(t, got)
			}
			want := golden.Get(t)
			assert.Equal(t, want, got)
		})
	}
}

func Test_route_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		want    route
		data    []byte
		wantErr string
	}{
		{
			name: "success",
			want: route{
				Group:    "fizz",
				Terminal: true,
				MatcherSets: []matcherSet{
					{
						Host: []string{"guvnor.k.io"},
					},
					{
						Host: []string{"guvnor.k.io"},
						Path: []string{"/help"},
					},
				},
				Handlers: handlers{
					reverseProxyHandler{
						Upstreams: []upstream{
							{
								Dial: "google.com",
							},
						},
					},
					reverseProxyHandler{
						Upstreams: []upstream{
							{
								Dial: "facebook.com",
							},
						},
					},
				},
			},
			data: []byte(`{"group":"fizz","match":[{"host":["guvnor.k.io"]},{"host":["guvnor.k.io"],"path":["/help"]}],"handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"google.com"}]},{"handler":"reverse_proxy","upstreams":[{"dial":"facebook.com"}]}],"terminal":true}`),
		},
		{
			name:    "invalid handler",
			wantErr: "unknown handler type 'i_dont_exist'",
			data:    []byte(`{"handle":[{"handler":"i_dont_exist"}]}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := route{}
			err := json.Unmarshal(tt.data, &got)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.want, got)
		})
	}
}
