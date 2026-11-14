package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// GetAppPricePoint retrieves a single app price point by ID.
func (c *Client) GetAppPricePoint(ctx context.Context, pricePointID string) (*AppPricePointsV3Response, error) {
	pricePointID = strings.TrimSpace(pricePointID)
	single, err := getJSON[SingleResponse[AppPricePointV3Attributes]](ctx, c, readRequest{
		Path:     fmt.Sprintf("/v3/appPricePoints/%s", pricePointID),
		ParseErr: "failed to parse app price point response",
	})
	if err != nil {
		return nil, err
	}

	response := AppPricePointsV3Response{
		Data:     []Resource[AppPricePointV3Attributes]{single.Data},
		Links:    single.Links,
		Included: single.Included,
	}

	return &response, nil
}

// CreateAppPriceSchedule creates an app price schedule with manual prices.
func (c *Client) CreateAppPriceSchedule(ctx context.Context, appID string, attrs AppPriceScheduleCreateAttributes) (*AppPriceScheduleResponse, error) {
	appID = strings.TrimSpace(appID)
	baseTerritoryID := strings.ToUpper(strings.TrimSpace(attrs.BaseTerritoryID))
	if appID == "" {
		return nil, fmt.Errorf("app ID is required")
	}
	prices := attrs.Prices
	if len(prices) == 0 {
		pricePointID := strings.TrimSpace(attrs.PricePointID)
		startDate := strings.TrimSpace(attrs.StartDate)
		if pricePointID == "" {
			return nil, fmt.Errorf("price point ID is required")
		}
		if startDate == "" {
			return nil, fmt.Errorf("start date is required")
		}
		prices = []AppPriceSchedulePrice{{PricePointID: pricePointID, StartDate: startDate}}
	}
	if baseTerritoryID == "" {
		return nil, fmt.Errorf("base territory ID is required")
	}

	manualPrices := make([]ResourceData, 0, len(prices))
	included := make([]AppPriceCreateResource, 0, len(prices))
	for index, price := range prices {
		pricePointID := strings.TrimSpace(price.PricePointID)
		if pricePointID == "" {
			return nil, fmt.Errorf("price point ID is required")
		}
		localID := fmt.Sprintf("${local-manual-price-%d}", index+1)
		manualPrices = append(manualPrices, ResourceData{Type: ResourceTypeAppPrices, ID: localID})
		included = append(included, AppPriceCreateResource{
			Type: ResourceTypeAppPrices,
			ID:   localID,
			Attributes: AppPriceAttributes{
				StartDate: strings.TrimSpace(price.StartDate),
				EndDate:   strings.TrimSpace(price.EndDate),
			},
			Relationships: AppPriceRelationships{
				AppPricePoint: Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppPricePoints,
						ID:   pricePointID,
					},
				},
			},
		})
	}

	payload := AppPriceScheduleCreateRequest{
		Data: AppPriceScheduleCreateData{
			Type: ResourceTypeAppPriceSchedules,
			Relationships: AppPriceScheduleCreateRelationships{
				App: Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   appID,
					},
				},
				BaseTerritory: Relationship{
					Data: ResourceData{
						Type: ResourceTypeTerritories,
						ID:   baseTerritoryID,
					},
				},
				ManualPrices: RelationshipList{Data: manualPrices},
			},
		},
		Included: included,
	}

	return sendJSON[AppPriceScheduleResponse](ctx, c, writeRequest{
		Method:   "POST",
		Path:     "/v1/appPriceSchedules",
		Payload:  payload,
		ParseErr: "failed to parse app price schedule response",
	})
}

// GetTerritoryAvailabilities retrieves territory availabilities for an availability ID.
func (c *Client) GetTerritoryAvailabilities(ctx context.Context, availabilityID string, opts ...TerritoryAvailabilitiesOption) (*TerritoryAvailabilitiesResponse, error) {
	availabilityID = strings.TrimSpace(availabilityID)
	query := &territoryAvailabilitiesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v2/appAvailabilities/%s/territoryAvailabilities", availabilityID)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("territoryAvailabilities: %w", err)
		}
		path = query.nextURL
	} else {
		values := url.Values{}
		values.Set("fields[territoryAvailabilities]", "available,releaseDate,preOrderEnabled,preOrderPublishDate,contentStatuses,territory")
		values.Set("include", "territory")
		addLimit(values, query.limit)
		path += "?" + values.Encode()
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response TerritoryAvailabilitiesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse territory availabilities response: %w", err)
	}

	return &response, nil
}

// CreateAppAvailabilityV2 creates an app availability and its inline territory availabilities.
// It calls POST /v2/appAvailabilities.
func (c *Client) CreateAppAvailabilityV2(ctx context.Context, appID string, attrs AppAvailabilityV2CreateAttributes) (*AppAvailabilityV2Response, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app ID is required")
	}

	var attributes *AppAvailabilityV2CreateAttributes
	if attrs.AvailableInNewTerritories != nil {
		attributes = &AppAvailabilityV2CreateAttributes{
			AvailableInNewTerritories: attrs.AvailableInNewTerritories,
		}
	}

	payload := AppAvailabilityV2CreateRequest{
		Data: AppAvailabilityV2CreateData{
			Type:       ResourceTypeAppAvailabilities,
			Attributes: attributes,
			Relationships: AppAvailabilityV2CreateRelationships{
				App: Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   appID,
					},
				},
			},
		},
	}

	if len(attrs.TerritoryAvailabilityIDs) > 0 {
		relationshipData := make([]ResourceData, 0, len(attrs.TerritoryAvailabilityIDs))
		payload.Included = make([]AppAvailabilityV2IncludedResource, 0, len(attrs.TerritoryAvailabilityIDs))
		for _, territoryAvailabilityID := range attrs.TerritoryAvailabilityIDs {
			trimmedID := strings.TrimSpace(territoryAvailabilityID)
			if trimmedID == "" {
				return nil, fmt.Errorf("territory availability ID is required")
			}
			relationshipData = append(relationshipData, ResourceData{
				Type: ResourceTypeTerritoryAvailabilities,
				ID:   trimmedID,
			})
			payload.Included = append(payload.Included, AppAvailabilityV2IncludedResource{
				Type: ResourceTypeTerritoryAvailabilities,
				ID:   trimmedID,
			})
		}
		payload.Data.Relationships.TerritoryAvailabilities = &RelationshipList{
			Data: relationshipData,
		}
	} else if len(attrs.TerritoryAvailabilities) > 0 {
		payload.Included = make([]AppAvailabilityV2IncludedResource, 0, len(attrs.TerritoryAvailabilities))
		relationshipData := make([]ResourceData, 0, len(attrs.TerritoryAvailabilities))
		for _, availability := range attrs.TerritoryAvailabilities {
			territoryID := strings.ToUpper(strings.TrimSpace(availability.TerritoryID))
			if territoryID == "" {
				return nil, fmt.Errorf("territory ID is required")
			}
			resourceID := fmt.Sprintf("${local-%s}", strings.ToLower(territoryID))
			relationshipData = append(relationshipData, ResourceData{
				Type: ResourceTypeTerritoryAvailabilities,
				ID:   resourceID,
			})
			attributes := &TerritoryAvailabilityCreateAttributes{
				Available:       availability.Available,
				ReleaseDate:     strings.TrimSpace(availability.ReleaseDate),
				PreOrderEnabled: availability.PreOrderEnabled,
			}
			relationships := &TerritoryAvailabilityRelationships{
				Territory: Relationship{
					Data: ResourceData{
						Type: ResourceTypeTerritories,
						ID:   territoryID,
					},
				},
			}
			payload.Included = append(payload.Included, AppAvailabilityV2IncludedResource{
				Type:          ResourceTypeTerritoryAvailabilities,
				ID:            resourceID,
				Attributes:    attributes,
				Relationships: relationships,
			})
		}
		payload.Data.Relationships.TerritoryAvailabilities = &RelationshipList{
			Data: relationshipData,
		}
	}

	return sendJSON[AppAvailabilityV2Response](ctx, c, writeRequest{
		Method:   "POST",
		Path:     "/v2/appAvailabilities",
		Payload:  payload,
		ParseErr: "failed to parse app availability response",
	})
}

// UpdateTerritoryAvailability updates a territory availability.
func (c *Client) UpdateTerritoryAvailability(ctx context.Context, territoryAvailabilityID string, attrs TerritoryAvailabilityUpdateAttributes) (*TerritoryAvailabilityResponse, error) {
	territoryAvailabilityID = strings.TrimSpace(territoryAvailabilityID)
	if territoryAvailabilityID == "" {
		return nil, fmt.Errorf("territory availability ID is required")
	}

	if attrs.ReleaseDate != nil {
		trimmed := strings.TrimSpace(*attrs.ReleaseDate)
		if trimmed == "" {
			return nil, fmt.Errorf("release date is required")
		}
		attrs.ReleaseDate = &trimmed
	}

	if attrs.Available == nil && attrs.ReleaseDate == nil && attrs.PreOrderEnabled == nil && !attrs.ClearReleaseDate {
		return nil, fmt.Errorf("at least one attribute is required")
	}

	attrMap := make(map[string]interface{})
	if attrs.Available != nil {
		attrMap["available"] = *attrs.Available
	}
	if attrs.PreOrderEnabled != nil {
		attrMap["preOrderEnabled"] = *attrs.PreOrderEnabled
	}
	if attrs.ReleaseDate != nil {
		attrMap["releaseDate"] = *attrs.ReleaseDate
	} else if attrs.ClearReleaseDate {
		attrMap["releaseDate"] = nil
	}

	attrJSON, err := json.Marshal(attrMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal attributes: %w", err)
	}

	type patchData struct {
		Type       ResourceType    `json:"type"`
		ID         string          `json:"id"`
		Attributes json.RawMessage `json:"attributes"`
	}

	payload := struct {
		Data patchData `json:"data"`
	}{
		Data: patchData{
			Type:       ResourceTypeTerritoryAvailabilities,
			ID:         territoryAvailabilityID,
			Attributes: attrJSON,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/territoryAvailabilities/%s", territoryAvailabilityID)
	// This PATCH sets exact values rather than applying a transition, so replaying
	// the same payload after a transient failure is safe.
	data, err := c.doIdempotentMutation(ctx, "PATCH", path, body)
	if err != nil {
		return nil, err
	}

	var response TerritoryAvailabilityResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse territory availability response: %w", err)
	}

	return &response, nil
}

// EndAppAvailabilityPreOrders ends pre-orders for territory availabilities.
func (c *Client) EndAppAvailabilityPreOrders(ctx context.Context, territoryAvailabilityIDs []string) (*EndAppAvailabilityPreOrderResponse, error) {
	if len(territoryAvailabilityIDs) == 0 {
		return nil, fmt.Errorf("territory availability IDs are required")
	}

	relationshipData := make([]ResourceData, 0, len(territoryAvailabilityIDs))
	for _, id := range territoryAvailabilityIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			return nil, fmt.Errorf("territory availability ID is required")
		}
		relationshipData = append(relationshipData, ResourceData{
			Type: ResourceTypeTerritoryAvailabilities,
			ID:   trimmed,
		})
	}

	payload := EndAppAvailabilityPreOrderCreateRequest{
		Data: EndAppAvailabilityPreOrderCreateData{
			Type: ResourceTypeEndAppAvailabilityPreOrders,
			Relationships: EndAppAvailabilityPreOrderRelationships{
				TerritoryAvailabilities: RelationshipList{
					Data: relationshipData,
				},
			},
		},
	}

	return sendJSON[EndAppAvailabilityPreOrderResponse](ctx, c, writeRequest{
		Method:   "POST",
		Path:     "/v1/endAppAvailabilityPreOrders",
		Payload:  payload,
		ParseErr: "failed to parse end app availability pre-order response",
	})
}
