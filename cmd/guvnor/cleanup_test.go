package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/jimeh/go-golden"
	"github.com/krystal/guvnor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_newCleanupCmd(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantArgs  *guvnor.CleanupArgs
		engineErr error
		wantErr   string
	}{
		{
			name: "success",
			args: []string{"fizzler"},
			wantArgs: &guvnor.CleanupArgs{
				ServiceName: "fizzler",
			},
		},
		{
			name: "default service",
			args: []string{},
			wantArgs: &guvnor.CleanupArgs{
				ServiceName: "boris",
			},
		},
		{
			name:      "error",
			args:      []string{"oops"},
			engineErr: errors.New("rats"),
			wantArgs: &guvnor.CleanupArgs{
				ServiceName: "oops",
			},
			wantErr: "rats",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mEngine := &mockEngine{}
			provider := func() (engine, *guvnor.EngineConfig, error) {
				return mEngine, nil, nil
			}

			if tt.wantArgs != nil {
				mEngine.
					On(
						"Cleanup",
						mock.MatchedBy(func(_ context.Context) bool {
							return true
						}),
						*tt.wantArgs).
					Return(tt.engineErr)
				mEngine.
					On("GetDefaultService").
					Return(&guvnor.GetDefaultServiceResult{Name: "boris"}, nil).
					Maybe()
			}

			cmd := newCleanupCommand(provider)
			stdout := bytes.NewBufferString("")
			stderr := bytes.NewBufferString("")
			cmd.SetOut(stdout)
			cmd.SetErr(stderr)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}

			if golden.Update() {
				golden.SetP(t, "stdout", stdout.Bytes())
				golden.SetP(t, "stderr", stderr.Bytes())
			}
			assert.Equal(t, golden.GetP(t, "stdout"), stdout.Bytes())
			assert.Equal(t, golden.GetP(t, "stderr"), stderr.Bytes())

			mEngine.AssertExpectations(t)
		})
	}
}
