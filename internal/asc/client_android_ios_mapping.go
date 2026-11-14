package asc

import (
	"context"
	"fmt"
	"strings"
)

// CreateAndroidToIosAppMappingDetail creates a new mapping.
func (c *Client) CreateAndroidToIosAppMappingDetail(ctx context.Context, appID string, attrs AndroidToIosAppMappingDetailCreateAttributes) (*AndroidToIosAppMappingDetailResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app ID is required")
	}

	request := AndroidToIosAppMappingDetailCreateRequest{
		Data: AndroidToIosAppMappingDetailCreateData{
			Type:       ResourceTypeAndroidToIosAppMappingDetails,
			Attributes: attrs,
			Relationships: AndroidToIosAppMappingDetailCreateRelationships{
				App: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   appID,
					},
				},
			},
		},
	}

	return sendJSON[AndroidToIosAppMappingDetailResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/androidToIosAppMappingDetails", Payload: request})
}

// UpdateAndroidToIosAppMappingDetail updates an existing mapping.
func (c *Client) UpdateAndroidToIosAppMappingDetail(ctx context.Context, id string, attrs AndroidToIosAppMappingDetailUpdateAttributes) (*AndroidToIosAppMappingDetailResponse, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("mapping ID is required")
	}

	request := AndroidToIosAppMappingDetailUpdateRequest{
		Data: AndroidToIosAppMappingDetailUpdateData{
			Type:       ResourceTypeAndroidToIosAppMappingDetails,
			ID:         id,
			Attributes: &attrs,
		},
	}

	return sendJSON[AndroidToIosAppMappingDetailResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/androidToIosAppMappingDetails/%s", id), Payload: request})
}
