package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// ResourceTypeAppStoreVersionReleaseRequests is the resource type for release requests.
const ResourceTypeAppStoreVersionReleaseRequests ResourceType = "appStoreVersionReleaseRequests"

// AppStoreVersionReleaseRequest represents a release request resource.
type AppStoreVersionReleaseRequest struct {
	Type ResourceType `json:"type"`
	ID   string       `json:"id"`
}

// AppStoreVersionReleaseRequestResponse represents a release request API response.
type AppStoreVersionReleaseRequestResponse struct {
	Data  AppStoreVersionReleaseRequest `json:"data"`
	Links Links                         `json:"links"`
}

// AppStoreVersionReleaseRequestCreateRequest represents a release request create payload.
type AppStoreVersionReleaseRequestCreateRequest struct {
	Data AppStoreVersionReleaseRequestCreateData `json:"data"`
}

// AppStoreVersionReleaseRequestCreateData represents data for creating a release request.
type AppStoreVersionReleaseRequestCreateData struct {
	Type          ResourceType                               `json:"type"`
	Relationships AppStoreVersionReleaseRequestRelationships `json:"relationships"`
}

// AppStoreVersionReleaseRequestRelationships describes relationships for a release request.
type AppStoreVersionReleaseRequestRelationships struct {
	AppStoreVersion Relationship `json:"appStoreVersion"`
}

// CreateAppStoreVersionReleaseRequest creates a release request for an app store version.
func (c *Client) CreateAppStoreVersionReleaseRequest(ctx context.Context, versionID string) (*AppStoreVersionReleaseRequestResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("version ID is required")
	}

	payload := AppStoreVersionReleaseRequestCreateRequest{
		Data: AppStoreVersionReleaseRequestCreateData{
			Type: ResourceTypeAppStoreVersionReleaseRequests,
			Relationships: AppStoreVersionReleaseRequestRelationships{
				AppStoreVersion: Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppStoreVersions,
						ID:   versionID,
					},
				},
			},
		},
	}

	return sendJSON[AppStoreVersionReleaseRequestResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/appStoreVersionReleaseRequests", Payload: payload})
}
