package asc

import (
	"context"
	"fmt"
	"strings"
)

// CreateBetaAppLocalization creates a beta app localization for an app.
func (c *Client) CreateBetaAppLocalization(ctx context.Context, appID string, attrs BetaAppLocalizationAttributes) (*BetaAppLocalizationResponse, error) {
	attrs.api = apiAttributes{}
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

	return sendJSON[BetaAppLocalizationResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/betaAppLocalizations", Payload: payload})
}
