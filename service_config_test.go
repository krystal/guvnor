package guvnor

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_findDefaultService(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr string
	}{
		{
			name: "valid",
			want: "one",
		},
		{
			name:    "multiple",
			wantErr: ErrMultipleServices.Error(),
		},
		{
			name:    "none",
			wantErr: ErrNoService.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDataPath := path.Join("./testdata/", t.Name())
			got, gotErr := findDefaultService(testDataPath)
			if tt.wantErr != "" {
				assert.EqualError(t, gotErr, tt.wantErr)
			} else {
				assert.NoError(t, gotErr)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
