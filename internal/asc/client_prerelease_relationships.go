package asc

import (
	"context"
)

// PreReleaseVersionAppLinkageResponse is the response for pre-release version app relationships.
type PreReleaseVersionAppLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
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
