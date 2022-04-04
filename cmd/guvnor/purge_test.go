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

func Test_newPurgeCmd(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantArgs   *guvnor.CleanupArgs
		engineErr  error
		wantCalled bool
		wantErr    string
	}{
		{
			name:       "success",
			args:       []string{"--confirm"},
			wantCalled: true,
			wantArgs: &guvnor.CleanupArgs{
				ServiceName: "fizzler",
			},
		},
		{
			name:       "error",
			args:       []string{"--confirm"},
			engineErr:  errors.New("rats"),
			wantCalled: true,
			wantErr:    "rats",
		},
		{
			name:    "confirm not specififed",
			args:    []string{},
			wantErr: "confirm flag must be specified to trigger purge",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mEngine := &mockEngine{}
			provider := func() (engine, *guvnor.EngineConfig, error) {
				return mEngine, nil, nil
			}

			if tt.wantCalled {
				mEngine.
					On(
						"Purge",
						mock.MatchedBy(func(_ context.Context) bool {
							return true
						})).
					Return(tt.engineErr)
			}

			cmd := newPurgeCmd(provider)
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
