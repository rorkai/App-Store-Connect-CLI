package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GetBundleIDApp retrieves the app for a bundle ID.
func (c *Client) GetBundleIDApp(ctx context.Context, bundleID string) (*AppResponse, error) {
	bundleID = strings.TrimSpace(bundleID)
	path := fmt.Sprintf("/v1/bundleIds/%s/app", bundleID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBundleIDProfiles retrieves profiles for a bundle ID.
func (c *Client) GetBundleIDProfiles(ctx context.Context, bundleID string, opts ...BundleIDProfilesOption) (*ProfilesResponse, error) {
	query := &bundleIDProfilesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/bundleIds/%s/profiles", strings.TrimSpace(bundleID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("bundleIdProfiles: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildBundleIDProfilesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response ProfilesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
