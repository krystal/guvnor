package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/jimeh/go-golden"
	"github.com/krystal/guvnor"
	"github.com/stretchr/testify/assert"
)

func Test_newDeployCmd(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantArgs  *guvnor.DeployArgs
		engineRes *guvnor.DeployResult
		engineErr error
		wantErr   string
	}{
		{
			name: "success",
			args: []string{"fizzler"},
			wantArgs: &guvnor.DeployArgs{
				ServiceName: "fizzler",
			},
			engineRes: &guvnor.DeployResult{
				ServiceName:  "fizzler",
				DeploymentID: 100,
			},
		},
		{
			name: "default service",
			args: []string{},
			wantArgs: &guvnor.DeployArgs{
				ServiceName: "boris",
			},
			engineRes: &guvnor.DeployResult{
				ServiceName:  "boris",
				DeploymentID: 200,
			},
		},
		{
			name:      "error",
			args:      []string{"oops"},
			engineErr: errors.New("rats"),
			wantArgs: &guvnor.DeployArgs{
				ServiceName: "oops",
			},
			wantErr: "rats",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mEngine := NewMockengine(ctrl)

			ctx := context.Background()
			provider := func() (engine, *guvnor.EngineConfig, error) {
				return mEngine, nil, nil
			}

			if tt.wantArgs != nil {
				mEngine.EXPECT().
					Deploy(
						ctx,
						*tt.wantArgs).
					Return(tt.engineRes, tt.engineErr)
				mEngine.EXPECT().
					GetDefaultService().
					Return(&guvnor.GetDefaultServiceResult{Name: "boris"}, nil).
					AnyTimes()
			}

			cmd := newDeployCmd(provider)
			stdout := bytes.NewBufferString("")
			stderr := bytes.NewBufferString("")
			cmd.SetOut(stdout)
			cmd.SetErr(stderr)
			cmd.SetArgs(tt.args)

			err := cmd.ExecuteContext(ctx)
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
		})
	}
}
