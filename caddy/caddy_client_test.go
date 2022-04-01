package caddy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestClient_doRequest(t *testing.T) {
	type testObj struct {
		Field string `json:"field"`
	}

	tests := []struct {
		name string

		method string
		path   *url.URL
		body   interface{}
		out    interface{}

		responseBody   []byte
		responseStatus int

		wantOut         interface{}
		wantRequestBody []byte
		wantErr         string
	}{
		{
			name:   "successful JSON post",
			method: http.MethodPost,
			path:   &url.URL{Path: "/succ_json_post/"},
			body: &testObj{
				Field: "dog",
			},
			out: &testObj{},

			wantRequestBody: []byte(`{"field":"dog"}`),
			responseBody:    []byte(`{"field":"fish"}`),
			wantOut: &testObj{
				Field: "fish",
			},
		},
		{
			name:   "successful JSON get",
			method: http.MethodGet,
			out:    &testObj{},

			path:         &url.URL{Path: "/succ_json_gett/"},
			responseBody: []byte(`{"field":"dog"}`),
			wantOut: &testObj{
				Field: "dog",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, err := io.ReadAll(r.Body)
					require.NoError(t, err)

					if tt.wantRequestBody != nil {
						assert.Equal(t, tt.wantRequestBody, body)
					} else {
						assert.Empty(t, body)
					}

					assert.Equal(
						t,
						"application/json",
						r.Header.Get("Content-Type"),
					)
					assert.Equal(t, tt.method, r.Method)

					if tt.responseStatus != 0 {
						w.WriteHeader(tt.responseStatus)
					} else {
						w.WriteHeader(200)
					}

					if tt.responseBody != nil {
						w.Write(tt.responseBody)
					}

					assert.Equal(t, tt.path.Path, r.URL.Path)
				}),
			)
			t.Cleanup(srv.Close)

			c := &Client{
				basePath: srv.URL,
				log:      zaptest.NewLogger(t),
			}

			err := c.doRequest(context.Background(), tt.method, tt.path, tt.body, tt.out)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
