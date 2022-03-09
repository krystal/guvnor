package ready

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"
)

type HTTPHeader struct {
	// Name is the key of the headers to set. This will be canonicalized.
	Name string `yaml:"name"`
	// Value is the value to set in the header specified by Name
	Value string `yaml:"value"`
}

type HTTPCheck struct {
	// Host is the IP/Hostname + Port combination to connect to when making the
	// request.
	Host string `yaml:"-"`
	// ExpectedStatus is the status code we expect the HTTP response to have.
	// It defaults to 200.
	ExpectedStatus int `yaml:"expectedStatus"`
	// Path is the path that the HTTP request should be made to.
	Path string `yaml:"path"`
	// Headers is a slice of headers to attach to the HTTP request.
	Headers []HTTPHeader `yaml:"headers"`
	// Timeout is the amount of time to allow for the request, failing if the
	// request takes longer. This defaults to 5 seconds.
	Timeout time.Duration `yaml:"timeout"`
}

func (hc *HTTPCheck) Test(ctx context.Context) error {
	url := url.URL{
		Scheme: "http",
		Host:   hc.Host,
		Path:   hc.Path,
	}

	timeout := time.Second * 5
	if hc.Timeout != 0 {
		timeout = hc.Timeout
	}

	expectedStatus := 200
	if hc.ExpectedStatus != 0 {
		expectedStatus = hc.ExpectedStatus
	}

	var cancel func()
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
	if err != nil {
		return err
	}

	for _, hdr := range hc.Headers {
		req.Header.Set(hdr.Name, hdr.Value)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode != expectedStatus {
		return fmt.Errorf(
			"unexpected status code (wanted %d, got %d)",
			expectedStatus,
			res.StatusCode,
		)
	}

	return nil
}

type Check struct {
	// Frequency is how often the check should be retried when we are trying to
	// detect if the service comes online.
	Frequency time.Duration `yaml:"frequency" validate:"required"`
	// Maximum is the maximum number of attempts to make before giving up on
	// the service coming online.
	Maximum int        `yaml:"maximum" validate:"required"`
	HTTP    *HTTPCheck `yaml:"http" validate:"required"`
}

// Test runs a check
func (c *Check) Test(ctx context.Context) error {
	if c.HTTP == nil {
		return errors.New("http check must be configured")
	}

	return c.HTTP.Test(ctx)
}

// Wait will provide a way to run a check continously until it passes or the
// maximum try threshold is passed.
func (c *Check) Wait(ctx context.Context, log *zap.Logger) error {
	t := time.NewTicker(c.Frequency)
	defer t.Stop()

	log.Debug("waiting for ready check to pass")
	var err error
	for attempt := 1; attempt <= c.Maximum; attempt++ {
		err = c.Test(ctx)
		if err == nil {
			log.Debug("attempt passed", zap.Int("attempt", attempt))
			return nil
		} else {
			log.Debug("attempt failed",
				zap.Int("attempt", attempt),
				zap.Int("maxAttempts", c.Maximum),
				zap.Error(err),
			)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}

	return fmt.Errorf("exhausted retry count: %w", err)
}
