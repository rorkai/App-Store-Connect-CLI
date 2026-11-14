package asc

import (
	"context"
	"fmt"
	"strings"
)

// AppTagsResponse is the response from app tags endpoints.
type AppTagsResponse = Response[AppTagAttributes]

// AppTagTerritoriesLinkagesResponse is the response from app tag territory linkages endpoints.
type AppTagTerritoriesLinkagesResponse = LinkagesResponse

// AppAppTagsLinkagesResponse is the response from app app tag linkages endpoints.
type AppAppTagsLinkagesResponse = LinkagesResponse

// UpdateAppTag updates an app tag by ID.
func (c *Client) UpdateAppTag(ctx context.Context, tagID string, attrs AppTagUpdateAttributes) (*AppTagResponse, error) {
	tagID = strings.TrimSpace(tagID)
	payload := AppTagUpdateRequest{
		Data: AppTagUpdateData{
			Type: ResourceTypeAppTags,
			ID:   tagID,
		},
	}
	if attrs.VisibleInAppStore != nil {
		payload.Data.Attributes = &attrs
	}

	return sendJSON[AppTagResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/appTags/%s", tagID), Payload: payload})
}

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
