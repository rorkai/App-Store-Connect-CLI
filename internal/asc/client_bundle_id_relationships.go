package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GetBundleIDApp retrieves the app for a bundle ID.
func (c *Client) GetBundleIDApp(ctx context.Context, bundleID string) (*AppResponse, error) {
	bundleID = strings.TrimSpace(bundleID)
	path := fmt.Sprintf("/v1/bundleIds/%s/app", bundleID)
	data, err := c.do(ctx, "GET", path, nil)
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

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ProfilesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBundleIDCapabilitiesRelationships retrieves capability linkages for a bundle ID.
func (c *Client) GetBundleIDCapabilitiesRelationships(ctx context.Context, bundleID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	bundleID = strings.TrimSpace(bundleID)
	if query.nextURL == "" && bundleID == "" {
		return nil, fmt.Errorf("bundleID is required")
	}
	// Production rejects `limit` for this relationship endpoint.
	if query.nextURL == "" && query.limit > 0 {
		return nil, fmt.Errorf("bundleIdCapabilities relationship does not support limit")
	}

	path := fmt.Sprintf("/v1/bundleIds/%s/relationships/bundleIdCapabilities", bundleID)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration.
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("bundleIDCapabilitiesRelationships: %w", err)
		}
		path = query.nextURL
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBundleIDProfilesRelationships retrieves profile linkages for a bundle ID.
func (c *Client) GetBundleIDProfilesRelationships(ctx context.Context, bundleID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBundleIDLinkages(ctx, bundleID, "profiles", opts...)
}

func (c *Client) getBundleIDLinkages(ctx context.Context, bundleID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/bundleIds/%s/relationships/%s", strings.TrimSpace(bundleID), relationship)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("bundleIDRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
