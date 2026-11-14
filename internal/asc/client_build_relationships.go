package asc

import (
	"context"
)

// BuildAppLinkageResponse is the response for build app relationships.
type BuildAppLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// BuildAppStoreVersionLinkageResponse is the response for build app store version relationships.
type BuildAppStoreVersionLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// BuildBuildBetaDetailLinkageResponse is the response for build beta detail relationships.
type BuildBuildBetaDetailLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// BuildPreReleaseVersionLinkageResponse is the response for build pre-release version relationships.
type BuildPreReleaseVersionLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// GetBuildBetaBuildLocalizationsRelationships retrieves beta build localization linkages for a build.
func (c *Client) GetBuildBetaBuildLocalizationsRelationships(ctx context.Context, buildID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBuildLinkages(ctx, buildID, "betaBuildLocalizations", opts...)
}

// GetBuildDiagnosticSignaturesRelationships retrieves diagnostic signature linkages for a build.
func (c *Client) GetBuildDiagnosticSignaturesRelationships(ctx context.Context, buildID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBuildLinkages(ctx, buildID, "diagnosticSignatures", opts...)
}

// GetBuildIndividualTestersRelationships retrieves individual tester linkages for a build.
func (c *Client) GetBuildIndividualTestersRelationships(ctx context.Context, buildID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBuildLinkages(ctx, buildID, "individualTesters", opts...)
}

// GetBuildIconsRelationships retrieves build icon linkages for a build.
func (c *Client) GetBuildIconsRelationships(ctx context.Context, buildID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBuildLinkages(ctx, buildID, "icons", opts...)
}

func (c *Client) getBuildLinkages(ctx context.Context, buildID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(
		ctx,
		buildID,
		relationship,
		"buildID",
		"/v1/builds/%s/relationships/%s",
		"buildRelationships",
		opts...,
	)
}
