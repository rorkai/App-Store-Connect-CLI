package asc

import (
	"context"
)

// GetBetaGroupBetaTestersRelationships retrieves beta tester linkages for a beta group.
func (c *Client) GetBetaGroupBetaTestersRelationships(ctx context.Context, groupID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBetaGroupLinkages(ctx, groupID, "betaTesters", opts...)
}

// GetBetaGroupBuildsRelationships retrieves build linkages for a beta group.
func (c *Client) GetBetaGroupBuildsRelationships(ctx context.Context, groupID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBetaGroupLinkages(ctx, groupID, "builds", opts...)
}

// GetBetaTesterAppsRelationships retrieves app linkages for a beta tester.
func (c *Client) GetBetaTesterAppsRelationships(ctx context.Context, testerID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBetaTesterLinkages(ctx, testerID, "apps", opts...)
}

// GetBetaTesterBetaGroupsRelationships retrieves beta group linkages for a beta tester.
func (c *Client) GetBetaTesterBetaGroupsRelationships(ctx context.Context, testerID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBetaTesterLinkages(ctx, testerID, "betaGroups", opts...)
}

// GetBetaTesterBuildsRelationships retrieves build linkages for a beta tester.
func (c *Client) GetBetaTesterBuildsRelationships(ctx context.Context, testerID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBetaTesterLinkages(ctx, testerID, "builds", opts...)
}

func (c *Client) getBetaGroupLinkages(ctx context.Context, groupID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(
		ctx,
		groupID,
		relationship,
		"groupID",
		"/v1/betaGroups/%s/relationships/%s",
		"betaGroupRelationships",
		opts...,
	)
}

func (c *Client) getBetaTesterLinkages(ctx context.Context, testerID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(
		ctx,
		testerID,
		relationship,
		"testerID",
		"/v1/betaTesters/%s/relationships/%s",
		"betaTesterRelationships",
		opts...,
	)
}
