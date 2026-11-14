package asc

import (
	"context"
)

// AppTagsResponse is the response from app tags endpoints.
type AppTagsResponse = Response[AppTagAttributes]

// AppTagTerritoriesLinkagesResponse is the response from app tag territory linkages endpoints.
type AppTagTerritoriesLinkagesResponse = LinkagesResponse

// AppAppTagsLinkagesResponse is the response from app app tag linkages endpoints.
type AppAppTagsLinkagesResponse = LinkagesResponse

// GetAppTagTerritoriesRelationships retrieves territory linkages for an app tag.
func (c *Client) GetAppTagTerritoriesRelationships(ctx context.Context, tagID string, opts ...LinkagesOption) (*AppTagTerritoriesLinkagesResponse, error) {
	return getTypedResourceLinkages[AppTagTerritoriesLinkagesResponse](
		c,
		ctx,
		tagID,
		"territories",
		"app tag ID",
		"/v1/appTags/%s/relationships/%s",
		"appTagTerritoriesRelationships",
		opts...,
	)
}

// GetAppTagsRelationshipsForApp retrieves app tag linkages for an app.
func (c *Client) GetAppTagsRelationshipsForApp(ctx context.Context, appID string, opts ...LinkagesOption) (*AppAppTagsLinkagesResponse, error) {
	return getTypedResourceLinkages[AppAppTagsLinkagesResponse](
		c,
		ctx,
		appID,
		"appTags",
		"app ID",
		"/v1/apps/%s/relationships/%s",
		"appTagsRelationships",
		opts...,
	)
}
