package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GetBetaAppLocalizations retrieves beta app localizations with optional filters.
func (c *Client) GetBetaAppLocalizations(ctx context.Context, opts ...BetaAppLocalizationsOption) (*BetaAppLocalizationsResponse, error) {
	query := &betaAppLocalizationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/betaAppLocalizations"
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("betaAppLocalizations: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildBetaAppLocalizationsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaAppLocalizationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBetaAppLocalization retrieves a beta app localization by ID.
func (c *Client) GetBetaAppLocalization(ctx context.Context, localizationID string) (*BetaAppLocalizationResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	path := fmt.Sprintf("/v1/betaAppLocalizations/%s", localizationID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaAppLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateBetaAppLocalization creates a beta app localization for an app.
func (c *Client) CreateBetaAppLocalization(ctx context.Context, appID string, attrs BetaAppLocalizationAttributes) (*BetaAppLocalizationResponse, error) {
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/betaAppLocalizations", body)
	if err != nil {
		return nil, err
	}

	var response BetaAppLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/betaAppLocalizations/%s", localizationID), body)
	if err != nil {
		return nil, err
	}

	var response BetaAppLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteBetaAppLocalization deletes a beta app localization by ID.
func (c *Client) DeleteBetaAppLocalization(ctx context.Context, localizationID string) error {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return fmt.Errorf("localizationID is required")
	}

	path := fmt.Sprintf("/v1/betaAppLocalizations/%s", localizationID)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}
