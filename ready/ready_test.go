package ready

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPCheck_Test(t *testing.T) {
	tests := []struct {
		name      string
		wantError string

		hc         *HTTPCheck
		sendStatus int
	}{
		{
			name: "success",
			hc: &HTTPCheck{
				ExpectedStatus: 200,
				Path:           "/fizz/buzz",
				Headers: []HTTPHeader{
					{
						Name:  "Fizz",
						Value: "Buzz",
					},
					{
						Name:  "Foo",
						Value: "Bar",
					},
					{
						Name:  "Host",
						Value: "google.com",
					},
				},
			},
			sendStatus: 200,
		},
		{
			name: "failure",
			hc: &HTTPCheck{
				ExpectedStatus: 200,
				Path:           "/an/error",
				Headers:        []HTTPHeader{},
			},
			wantError:  "unexpected status code (wanted 200, got 500)",
			sendStatus: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			srv := httptest.NewServer(http.HandlerFunc(
				func(rw http.ResponseWriter, r *http.Request) {
					assert.Equal(t, tt.hc.Path, r.URL.Path)

					for _, hdr := range tt.hc.Headers {
						if hdr.Name == "Host" {
							assert.Equal(t, hdr.Value, r.Host)
						} else {
							assert.Equal(t, hdr.Value, r.Header.Get(hdr.Name))
						}
					}

					rw.WriteHeader(tt.sendStatus)
				},
			))
			defer srv.Close()

			srvUrl, err := url.Parse(srv.URL)
			require.NoError(t, err)
			tt.hc.Host = srvUrl.Host

			err = tt.hc.Test(ctx)
			if tt.wantError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.wantError)
			}
		})
	}
}
