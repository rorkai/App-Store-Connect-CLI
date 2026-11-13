package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// readRequest describes a GET of one App Store Connect resource or page.
type readRequest struct {
	// Path is the formatted endpoint path without a query string.
	Path string
	// Query is the encoded query string; it is not sent when Next is set.
	Query string
	// Next is a links.next URL that replaces Path and Query after validation.
	Next string
	// NextContext prefixes a rejected Next error, as in "devices: ...".
	NextContext string
	// ParseErr prefixes a decode error; empty means "failed to parse response".
	ParseErr string
}

// writeRequest describes a request with a JSON body whose JSON response is decoded.
type writeRequest struct {
	Method  string
	Path    string
	Payload any
	// ParseErr prefixes a decode error; empty means "failed to parse response".
	ParseErr string
}

// applyOptions applies functional options to a zero query value.
func applyOptions[Q any, O ~func(*Q)](opts []O) *Q {
	query := new(Q)
	for _, opt := range opts {
		opt(query)
	}
	return query
}

// getJSON sends a GET and decodes the JSON response into T.
func getJSON[T any](ctx context.Context, c *Client, r readRequest) (*T, error) {
	path := r.Path
	if r.Next != "" {
		if err := validateNextURL(r.Next); err != nil {
			return nil, fmt.Errorf("%s: %w", r.NextContext, err)
		}
		path = r.Next
	} else if r.Query != "" {
		path += "?" + r.Query
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return decodeJSON[T](data, r.ParseErr)
}

// sendJSON sends the payload as a JSON body and decodes the JSON response into T.
func sendJSON[T any](ctx context.Context, c *Client, r writeRequest) (*T, error) {
	body, err := BuildRequestBody(r.Payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, r.Method, r.Path, body)
	if err != nil {
		return nil, err
	}
	return decodeJSON[T](data, r.ParseErr)
}

// sendNoContent sends a request whose response body is not read. A nil
// payload sends no body.
func sendNoContent(ctx context.Context, c *Client, method, path string, payload any) error {
	if payload == nil {
		_, err := c.do(ctx, method, path, nil)
		return err
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}
	_, err = c.do(ctx, method, path, body)
	return err
}

func decodeJSON[T any](data []byte, parseErr string) (*T, error) {
	if parseErr == "" {
		parseErr = "failed to parse response"
	}
	var response T
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("%s: %w", parseErr, err)
	}
	return &response, nil
}
