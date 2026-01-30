package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// PreReleaseVersionAppLinkageResponse is the response for pre-release version app relationships.
type PreReleaseVersionAppLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links,omitempty"`
}

// GetPreReleaseVersionAppRelationship retrieves the app linkage for a pre-release version.
func (c *Client) GetPreReleaseVersionAppRelationship(ctx context.Context, versionID string) (*PreReleaseVersionAppLinkageResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	path := fmt.Sprintf("/v1/preReleaseVersions/%s/relationships/app", versionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response PreReleaseVersionAppLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetPreReleaseVersionBuildsRelationships retrieves build linkages for a pre-release version.
func (c *Client) GetPreReleaseVersionBuildsRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	versionID = strings.TrimSpace(versionID)
	if query.nextURL == "" && versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	path := fmt.Sprintf("/v1/preReleaseVersions/%s/relationships/builds", versionID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("preReleaseVersionBuilds: %w", err)
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
