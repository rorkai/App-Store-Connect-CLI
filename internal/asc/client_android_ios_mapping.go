package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// CreateAndroidToIosAppMappingDetail creates a new mapping.
func (c *Client) CreateAndroidToIosAppMappingDetail(ctx context.Context, appID string, attrs AndroidToIosAppMappingDetailCreateAttributes) (*AndroidToIosAppMappingDetailResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app ID is required")
	}

	payload := AndroidToIosAppMappingDetailCreateRequest{
		Data: AndroidToIosAppMappingDetailCreateData{
			Type:       ResourceTypeAndroidToIosAppMappingDetails,
			Attributes: attrs,
			Relationships: AndroidToIosAppMappingDetailCreateRelationships{
				App: Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   appID,
					},
				},
			},
		},
	}

	return sendJSON[AndroidToIosAppMappingDetailResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/androidToIosAppMappingDetails", Payload: payload})
}
