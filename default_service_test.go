package guvnor

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Engine_GetDefaultService(t *testing.T) {
	tests := []struct {
		name    string
		want    *GetDefaultServiceResult
		wantErr string
	}{
		{
			name: "valid",
			want: &GetDefaultServiceResult{
				Name: "one",
			},
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
			e := Engine{
				config: EngineConfig{
					Paths: PathsConfig{
						Config: path.Join("./testdata/", t.Name()),
					},
				},
			}
			got, gotErr := e.GetDefaultService()
			if tt.wantErr != "" {
				assert.EqualError(t, gotErr, tt.wantErr)
			} else {
				assert.NoError(t, gotErr)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
