package guvnor

import (
	"testing"

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
