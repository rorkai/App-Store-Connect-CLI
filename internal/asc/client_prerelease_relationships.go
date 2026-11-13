package asc

import (
	"context"
	"fmt"
	"strings"
)

// PreReleaseVersionAppLinkageResponse is the response for pre-release version app relationships.
type PreReleaseVersionAppLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// GetPreReleaseVersionAppRelationship retrieves the app linkage for a pre-release version.
func (c *Client) GetPreReleaseVersionAppRelationship(ctx context.Context, versionID string) (*PreReleaseVersionAppLinkageResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	return getJSON[PreReleaseVersionAppLinkageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/preReleaseVersions/%s/relationships/app", versionID), ParseErr: "failed to parse app relationship response"})
}

// GetPreReleaseVersionBuildsRelationships retrieves build linkages for a pre-release version.
func (c *Client) GetPreReleaseVersionBuildsRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(
		ctx,
		versionID,
		"builds",
		"versionID",
		"/v1/preReleaseVersions/%s/relationships/%s",
		"preReleaseVersionBuilds",
		opts...,
	)
}
