package guvnor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
