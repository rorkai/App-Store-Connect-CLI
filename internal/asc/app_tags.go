package asc

import (
	"context"
	"fmt"
	"strings"
)

// AppTagAttributes describes an app tag resource.
type AppTagAttributes struct {
	Name              string `json:"name,omitempty"`
	VisibleInAppStore bool   `json:"visibleInAppStore,omitempty"`
}

// AppTagsResponse is the response from app tags endpoints.
type AppTagsResponse = Response[AppTagAttributes]

// AppTagResponse is the response from app tag detail/updates.
type AppTagResponse = SingleResponse[AppTagAttributes]

// AppTagTerritoriesLinkagesResponse is the response from app tag territory linkages endpoints.
type AppTagTerritoriesLinkagesResponse = LinkagesResponse

// AppAppTagsLinkagesResponse is the response from app app tag linkages endpoints.
type AppAppTagsLinkagesResponse = LinkagesResponse

// AppTagUpdateAttributes describes fields for updating an app tag.
type AppTagUpdateAttributes struct {
	VisibleInAppStore *bool `json:"visibleInAppStore,omitempty"`
}

// AppTagUpdateData is the data portion of an app tag update request.
type AppTagUpdateData struct {
	Type       ResourceType            `json:"type"`
	ID         string                  `json:"id"`
	Attributes *AppTagUpdateAttributes `json:"attributes,omitempty"`
}

// AppTagUpdateRequest is a request to update an app tag.
type AppTagUpdateRequest struct {
	Data AppTagUpdateData `json:"data"`
}

// GetAppTags retrieves the list of app tags for an app.
func (c *Client) GetAppTags(ctx context.Context, appID string, opts ...AppTagsOption) (*AppTagsResponse, error) {
	query := applyOptions(opts)
	appID = strings.TrimSpace(appID)
	return getJSON[AppTagsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/apps/%s/appTags", appID),
		Query:       buildAppTagsQuery(query),
		Next:        query.nextURL,
		NextContext: "appTags",
	})
}

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

// GetAppTagTerritories retrieves territories for a specific app tag.
func (c *Client) GetAppTagTerritories(ctx context.Context, tagID string, opts ...TerritoriesOption) (*TerritoriesResponse, error) {
	query := applyOptions(opts)
	tagID = strings.TrimSpace(tagID)
	return getJSON[TerritoriesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appTags/%s/territories", tagID),
		Query:       buildTerritoriesQuery(query),
		Next:        query.nextURL,
		NextContext: "appTagTerritories",
	})
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
