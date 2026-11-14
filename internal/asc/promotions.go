package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// AppStoreVersionPromotionAttributes describes promotion metadata.
type AppStoreVersionPromotionAttributes struct {
	CreatedDate string `json:"createdDate,omitempty"`
}

// AppStoreVersionPromotionRelationships describes promotion relationships.
type AppStoreVersionPromotionRelationships struct {
	AppStoreVersion                    *Relationship `json:"appStoreVersion,omitempty"`
	AppStoreVersionExperimentTreatment *Relationship `json:"appStoreVersionExperimentTreatment,omitempty"`
}

// AppStoreVersionPromotionResponse is the response from promotion create endpoints.
type AppStoreVersionPromotionResponse struct {
	Data  Resource[AppStoreVersionPromotionAttributes] `json:"data"`
	Links Links                                        `json:"links"`
}

// CreateAppStoreVersionPromotion creates a version promotion.
func (c *Client) CreateAppStoreVersionPromotion(ctx context.Context, versionID, treatmentID string) (*AppStoreVersionPromotionResponse, error) {
	versionID = strings.TrimSpace(versionID)
	treatmentID = strings.TrimSpace(treatmentID)
	if versionID == "" {
		return nil, fmt.Errorf("version ID is required")
	}
	if treatmentID == "" {
		return nil, fmt.Errorf("treatment ID is required")
	}

	payload := AppStoreVersionPromotionCreateRequest{
		Data: AppStoreVersionPromotionCreateData{
			Type: ResourceTypeAppStoreVersionPromotions,
			Relationships: AppStoreVersionPromotionRelationships{
				AppStoreVersion: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppStoreVersions,
						ID:   versionID,
					},
				},
				AppStoreVersionExperimentTreatment: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppStoreVersionExperimentTreatments,
						ID:   treatmentID,
					},
				},
			},
		},
	}

	return sendJSON[AppStoreVersionPromotionResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/appStoreVersionPromotions", Payload: payload})
}
