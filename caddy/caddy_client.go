package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/caddyserver/caddy/v2"
	"go.uber.org/zap"
)

// Client wraps access to the Caddy Admin API.
type Client struct {
	basePath string
	log      *zap.Logger
}

func NewClient(log *zap.Logger) *Client {
	if log == nil {
		log = zap.NewNop()
	}

	return &Client{
		basePath: "http://localhost:2019",
		log:      log,
	}
}

// getRoutes returns an slice of routes configured on the caddy server
func (c *Client) getRoutes(ctx context.Context) ([]route, error) {
	currentRoutes := []route{}
	routesConfigPath := fmt.Sprintf(
		"config/apps/http/servers/%s/routes",
		guvnorServerName,
	)
	err := c.doRequest(ctx, http.MethodGet, &url.URL{Path: routesConfigPath}, nil, &currentRoutes)
	if err != nil {
		return nil, err
	}

	return currentRoutes, nil
}

// prependRoute adds a new route to the start of the route array in the server
func (c *Client) patchRoutes(ctx context.Context, route []route) error {
	prependRoutePath := fmt.Sprintf(
		"config/apps/http/servers/%s/routes",
		guvnorServerName,
	)
	return c.doRequest(
		ctx,
		http.MethodPatch,
		&url.URL{Path: prependRoutePath},
		route,
		nil,
	)
}

func (c *Client) getConfig(ctx context.Context) (*caddy.Config, error) {
	cfg := &caddy.Config{}
	err := c.doRequest(
		ctx,
		http.MethodGet,
		&url.URL{Path: "config/"},
		nil,
		cfg,
	)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Client) postConfig(ctx context.Context, cfg *caddy.Config) error {
	err := c.doRequest(
		ctx, http.MethodPost, &url.URL{Path: "config/"}, cfg, nil,
	)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) doRequest(ctx context.Context, method string, path *url.URL, body interface{}, out interface{}) error {
	var bodyToSend io.Reader
	if body != nil {
		if v, ok := body.(string); ok {
			// Send string directly
			bodyToSend = bytes.NewBufferString(v)
		} else if v, ok := body.([]byte); ok {
			bodyToSend = bytes.NewBuffer(v)
		} else {
			// If not a string, JSONify it and send it
			data, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("marshalling body: %w", err)
			}
			bodyToSend = bytes.NewBuffer(data)
		}
	}

	// TODO: Pull this into the config for Manager
	rootPath, err := url.Parse(c.basePath)
	if err != nil {
		return err
	}

	fullPath := rootPath.ResolveReference(path).String()

	req, err := http.NewRequestWithContext(ctx, method, fullPath, bodyToSend)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")

	c.log.Debug("making request to caddy",
		zap.String("url", req.URL.String()),
		zap.String("method", req.Method),
	)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer res.Body.Close()

	// TODO: Check status codes
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}
	c.log.Debug("response from caddy",
		zap.String("body", string(data)),
		zap.Int("status", res.StatusCode),
	)
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("unmarshalling response: %w", err)
		}
	}

	return nil
}
