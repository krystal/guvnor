package caddy

import (
	"encoding/json"
	"fmt"
)

// These types map to caddy types, but allow us to use them more effectively as
// we've circumvented their heavy usage of json.RawMessage

type route struct {
	Group       string       `json:"group,omitempty"`
	MatcherSets []matcherSet `json:"match,omitempty"`
	Handlers    handlers     `json:"handle,omitempty"`
	Terminal    bool         `json:"terminal,omitempty"`
}

type matcherSet struct {
	Host []string `json:"host,omitempty"`
	Path []string `json:"path,omitempty"`
}

type handlers []handler

func (h handlers) MarshalJSON() ([]byte, error) {
	out := []map[string]interface{}{}

	for _, handler := range h {
		data, err := json.Marshal(handler)
		if err != nil {
			return nil, err
		}

		jsonMap := map[string]interface{}{}
		if err := json.Unmarshal(data, &jsonMap); err != nil {
			return nil, err
		}

		jsonMap["handler"] = handler.HandlerName()

		out = append(out, jsonMap)
	}

	return json.Marshal(out)
}

func (h *handlers) UnmarshalJSON(dataBytes []byte) error {
	out := handlers{}

	raw := []json.RawMessage{}
	if err := json.Unmarshal(dataBytes, &raw); err != nil {
		return err
	}

	for _, rawHandler := range raw {
		handlerIdentity := struct {
			Handler string `json:"handler"`
		}{}
		if err := json.Unmarshal(rawHandler, &handlerIdentity); err != nil {
			return err
		}

		switch handlerIdentity.Handler {
		case "reverse_proxy":
			value := reverseProxyHandler{}
			if err := json.Unmarshal(rawHandler, &value); err != nil {
				return err
			}
			out = append(out, value)
		case "static_response":
			value := staticResponseHandler{}
			if err := json.Unmarshal(rawHandler, &value); err != nil {
				return err
			}
			out = append(out, value)
		default:
			return fmt.Errorf(
				"unknown handler type '%s'", handlerIdentity.Handler,
			)
		}
	}

	*h = out
	return nil
}

type handler interface {
	HandlerName() string
}

type reverseProxyHandler struct {
	Upstreams []upstream `json:"upstreams,omitempty"`
}

type upstream struct {
	Dial string `json:"dial,omitempty"`
}

func (rph reverseProxyHandler) HandlerName() string {
	return "reverse_proxy"
}

type staticResponseHandler struct {
	Body       string `json:"body,omitempty"`
	StatusCode string `json:"status_code,omitempty"`
}

func (rph staticResponseHandler) HandlerName() string {
	return "static_response"
}
