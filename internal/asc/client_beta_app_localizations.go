package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// CreateBetaAppLocalization creates a beta app localization for an app.
func (c *Client) CreateBetaAppLocalization(ctx context.Context, appID string, attrs BetaAppLocalizationCreateAttributes) (*BetaAppLocalizationResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}
	payload := BetaAppLocalizationCreateRequest{
		Data: BetaAppLocalizationCreateData{
			Type:       ResourceTypeBetaAppLocalizations,
			Attributes: attrs,
			Relationships: &BetaAppLocalizationRelationships{
				App: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   appID,
					},
				},
			},
		},
	}
	return sendJSON[BetaAppLocalizationResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v1/betaAppLocalizations",
		Payload: payload,
	})
}
