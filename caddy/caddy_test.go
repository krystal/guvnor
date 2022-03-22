package caddy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
