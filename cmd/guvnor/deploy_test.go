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

func Test_newDeployCmd(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantArgs  *guvnor.DeployArgs
		engineRes *guvnor.DeployRes
		engineErr error
		wantErr   string
	}{
		{
			name: "success",
			args: []string{"fizzler"},
			wantArgs: &guvnor.DeployArgs{
				ServiceName: "fizzler",
			},
			engineRes: &guvnor.DeployRes{
				ServiceName:  "fizzler",
				DeploymentID: 100,
			},
		},
		{
			name: "default service",
			args: []string{},
			wantArgs: &guvnor.DeployArgs{
				ServiceName: "",
			},
			engineRes: &guvnor.DeployRes{
				ServiceName:  "boris",
				DeploymentID: 200,
			},
		},
		{
			name:      "error",
			args:      []string{},
			engineErr: errors.New("rats"),
			wantArgs: &guvnor.DeployArgs{
				ServiceName: "",
			},
			wantErr: "rats",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mEngine := &mockEngine{}
			provider := func() (engine, error) {
				return mEngine, nil
			}

			if tt.wantArgs != nil {
				mEngine.
					On(
						"Deploy",
						mock.MatchedBy(func(_ context.Context) bool {
							return true
						}),
						*tt.wantArgs).
					Return(tt.engineRes, tt.engineErr)
			}

			cmd := newDeployCmd(provider)
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
