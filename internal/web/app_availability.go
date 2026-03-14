package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// AppAvailability models the internal web API app availability resource.
type AppAvailability struct {
	ID                        string   `json:"id"`
	Type                      string   `json:"type,omitempty"`
	AvailableInNewTerritories bool     `json:"availableInNewTerritories"`
	AvailableTerritories      []string `json:"availableTerritories,omitempty"`
}

// AppAvailabilityCreateAttributes defines inputs for creating initial app availability.
type AppAvailabilityCreateAttributes struct {
	AppID                     string   `json:"-"`
	AvailableInNewTerritories bool     `json:"-"`
	AvailableTerritories      []string `json:"-"`
}

// IsNotFound reports whether the internal web API returned a not-found response.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound
}

func normalizeAppAvailabilityCreateAttributes(attrs AppAvailabilityCreateAttributes) (AppAvailabilityCreateAttributes, error) {
	attrs.AppID = strings.TrimSpace(attrs.AppID)
	if attrs.AppID == "" {
		return attrs, fmt.Errorf("app id is required")
	}

	normalizedTerritories := make([]string, 0, len(attrs.AvailableTerritories))
	seen := make(map[string]struct{}, len(attrs.AvailableTerritories))
	for _, territoryID := range attrs.AvailableTerritories {
		normalized := strings.ToUpper(strings.TrimSpace(territoryID))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		normalizedTerritories = append(normalizedTerritories, normalized)
	}
	if len(normalizedTerritories) == 0 {
		return attrs, fmt.Errorf("at least one available territory is required")
	}
	slices.Sort(normalizedTerritories)
	attrs.AvailableTerritories = normalizedTerritories
	return attrs, nil
}

func decodeAppAvailabilityResource(resource jsonAPIResource) AppAvailability {
	availability := AppAvailability{
		ID:                        strings.TrimSpace(resource.ID),
		Type:                      strings.TrimSpace(resource.Type),
		AvailableInNewTerritories: boolAttr(resource.Attributes, "availableInNewTerritories"),
	}

	refs := relationshipRefs(resource, "availableTerritories")
	if len(refs) == 0 {
		return availability
	}

	territories := make([]string, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		territoryID := strings.ToUpper(strings.TrimSpace(ref.ID))
		if territoryID == "" {
			continue
		}
		if _, ok := seen[territoryID]; ok {
			continue
		}
		seen[territoryID] = struct{}{}
		territories = append(territories, territoryID)
	}
	slices.Sort(territories)
	availability.AvailableTerritories = territories
	return availability
}

// GetAppAvailability retrieves the internal web app availability resource for an app.
func (c *Client) GetAppAvailability(ctx context.Context, appID string) (*AppAvailability, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app id is required")
	}

	path := "/apps/" + url.PathEscape(appID) + "/appAvailabilityV2"
	responseBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Data jsonAPIResource `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse app availability response: %w", err)
	}

	availability := decodeAppAvailabilityResource(payload.Data)
	return &availability, nil
}

// CreateAppAvailability creates the initial app availability via the internal web API.
func (c *Client) CreateAppAvailability(ctx context.Context, attrs AppAvailabilityCreateAttributes) (*AppAvailability, error) {
	normalized, err := normalizeAppAvailabilityCreateAttributes(attrs)
	if err != nil {
		return nil, err
	}

	territories := make([]map[string]string, 0, len(normalized.AvailableTerritories))
	for _, territoryID := range normalized.AvailableTerritories {
		territories = append(territories, map[string]string{
			"type": "territories",
			"id":   territoryID,
		})
	}

	requestBody := map[string]any{
		"data": map[string]any{
			"type": "appAvailabilities",
			"attributes": map[string]bool{
				"availableInNewTerritories": normalized.AvailableInNewTerritories,
			},
			"relationships": map[string]any{
				"app": map[string]any{
					"data": map[string]string{
						"type": "apps",
						"id":   normalized.AppID,
					},
				},
				"availableTerritories": map[string]any{
					"data": territories,
				},
			},
		},
	}

	responseBody, err := c.doRequest(ctx, http.MethodPost, "/appAvailabilities", requestBody)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Data jsonAPIResource `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse app availability create response: %w", err)
	}

	availability := decodeAppAvailabilityResource(payload.Data)
	return &availability, nil
}
