package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetBetaAppLocalizations retrieves beta app localizations with optional filters.
func (c *Client) GetBetaAppLocalizations(ctx context.Context, opts ...BetaAppLocalizationsOption) (*BetaAppLocalizationsResponse, error) {
	query := applyOptions(opts)
	return getJSON[BetaAppLocalizationsResponse](ctx, c, readRequest{
		Path:        "/v1/betaAppLocalizations",
		Query:       buildBetaAppLocalizationsQuery(query),
		Next:        query.nextURL,
		NextContext: "betaAppLocalizations",
	})
}

// GetBetaAppLocalization retrieves a beta app localization by ID.
func (c *Client) GetBetaAppLocalization(ctx context.Context, localizationID string) (*BetaAppLocalizationResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	return getJSON[BetaAppLocalizationResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/betaAppLocalizations/%s", localizationID)})
}

// GetBetaAppLocalizationApp retrieves the app for a beta app localization.
func (c *Client) GetBetaAppLocalizationApp(ctx context.Context, localizationID string) (*AppResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	return getJSON[AppResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/betaAppLocalizations/%s/app", localizationID)})
}

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

// UpdateBetaAppLocalization updates a beta app localization by ID.
func (c *Client) UpdateBetaAppLocalization(ctx context.Context, localizationID string, attrs BetaAppLocalizationUpdateAttributes) (*BetaAppLocalizationResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	payload := BetaAppLocalizationUpdateRequest{
		Data: BetaAppLocalizationUpdateData{
			Type:       ResourceTypeBetaAppLocalizations,
			ID:         localizationID,
			Attributes: &attrs,
		},
	}

	return sendJSON[BetaAppLocalizationResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/betaAppLocalizations/%s", localizationID), Payload: payload})
}

// DeleteBetaAppLocalization deletes a beta app localization by ID.
func (c *Client) DeleteBetaAppLocalization(ctx context.Context, localizationID string) error {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return fmt.Errorf("localizationID is required")
	}

	return sendNoContent(ctx, c, "DELETE", fmt.Sprintf("/v1/betaAppLocalizations/%s", localizationID), nil)
}
