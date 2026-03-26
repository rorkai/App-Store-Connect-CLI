package itunes

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const defaultBaseURL = "https://itunes.apple.com"

// Client is an iTunes public API client.
type Client struct {
	HTTPClient *http.Client
	BaseURL    string
}

// NewClient creates a new iTunes API client.
func NewClient() *Client {
	return &Client{
		HTTPClient: http.DefaultClient,
		BaseURL:    defaultBaseURL,
	}
}

func (c *Client) httpClient() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *Client) baseURL() string {
	if c == nil {
		return defaultBaseURL
	}
	base := strings.TrimSpace(c.BaseURL)
	if base == "" {
		return defaultBaseURL
	}
	return strings.TrimRight(base, "/")
}

func (c *Client) newRequest(ctx context.Context, method, path string, query url.Values) (*http.Request, error) {
	base, err := url.Parse(c.baseURL())
	if err != nil {
		return nil, fmt.Errorf("invalid iTunes base URL: %w", err)
	}

	reqURL := *base
	reqURL.Path = strings.TrimRight(reqURL.Path, "/") + path
	if len(query) > 0 {
		reqURL.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	return req, nil
}
