package ready

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"go.uber.org/zap"
)

type HTTPHeader struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type HTTPCheck struct {
	Host           string       `yaml:"host"`
	ExpectedStatus int          `yaml:"expectedStatus"`
	Path           string       `yaml:"path"`
	Headers        []HTTPHeader `yaml:"headers"`
	// Timeout        int          `yaml:"timeout"`
}

func (hc *HTTPCheck) Test(ctx context.Context) error {
	url := url.URL{
		Scheme: "http",
		Host:   hc.Host,
		Path:   hc.Path,
	}
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

	if res.StatusCode != hc.ExpectedStatus {
		return fmt.Errorf(
			"unexpected status code (wanted %d, got %d)",
			hc.ExpectedStatus,
			res.StatusCode,
		)
	}

	return nil
}

type Check struct {
	Frequency int        `yaml:"frequency"`
	Maximum   int        `yaml:"maximum"`
	HTTP      *HTTPCheck `yaml:"http"`
}

// Test runs a check
func (c *Check) Test(ctx context.Context) error {
	if c.HTTP == nil {
		return errors.New("http check must be configured")
	}

	return c.HTTP.Test(ctx)
}

// Wait will provide a way to run a check continously until it passes.
func (c *Check) Wait(ctx context.Context, log *zap.Logger) error {
	for attempt := 1; attempt <= c.Maximum; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := c.Test(ctx)
		if err == nil {
			return nil
		} else {
			log.Debug("check attempt failed",
				zap.Int("attempt", attempt),
				zap.Int("maxAttempts", c.Maximum),
				zap.Error(err),
			)
		}
	}

	return errors.New("exhausted check limit, service has not come online")
}
