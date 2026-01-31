package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const appPriceScheduleManualPriceID = "${local-manual-price-1}"

// GetTerritories retrieves available territories.
func (c *Client) GetTerritories(ctx context.Context, opts ...TerritoriesOption) (*TerritoriesResponse, error) {
	query := &territoriesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/territories"
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("territories: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildTerritoriesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response TerritoriesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse territories response: %w", err)
	}

	return &response, nil
}

// GetAppPricePoints retrieves app price points for an app.
func (c *Client) GetAppPricePoints(ctx context.Context, appID string, opts ...PricePointsOption) (*AppPricePointsV3Response, error) {
	query := &pricePointsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	appID = strings.TrimSpace(appID)
	path := fmt.Sprintf("/v1/apps/%s/appPricePoints", appID)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appPricePoints: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildPricePointsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppPricePointsV3Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse app price points response: %w", err)
	}

	return &response, nil
}

// GetAppPricePoint retrieves a single app price point by ID.
func (c *Client) GetAppPricePoint(ctx context.Context, pricePointID string) (*AppPricePointsV3Response, error) {
	pricePointID = strings.TrimSpace(pricePointID)
	path := fmt.Sprintf("/v3/appPricePoints/%s", pricePointID)

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var single SingleResponse[AppPricePointV3Attributes]
	if err := json.Unmarshal(data, &single); err != nil {
		return nil, fmt.Errorf("failed to parse app price point response: %w", err)
	}

	response := AppPricePointsV3Response{
		Data:  []Resource[AppPricePointV3Attributes]{single.Data},
		Links: single.Links,
	}

	return &response, nil
}

// GetAppPricePointEqualizations retrieves equalized price points for a price point.
func (c *Client) GetAppPricePointEqualizations(ctx context.Context, pricePointID string) (*AppPricePointsV3Response, error) {
	pricePointID = strings.TrimSpace(pricePointID)
	path := fmt.Sprintf("/v3/appPricePoints/%s/equalizations", pricePointID)

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppPricePointsV3Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse app price point equalizations response: %w", err)
	}

	return &response, nil
}

// GetAppPriceSchedule retrieves the app price schedule for an app.
func (c *Client) GetAppPriceSchedule(ctx context.Context, appID string) (*AppPriceScheduleResponse, error) {
	appID = strings.TrimSpace(appID)
	path := fmt.Sprintf("/v1/apps/%s/appPriceSchedule", appID)

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppPriceScheduleResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse app price schedule response: %w", err)
	}

	return &response, nil
}

// GetAppPriceScheduleByID retrieves an app price schedule by ID.
func (c *Client) GetAppPriceScheduleByID(ctx context.Context, scheduleID string) (*AppPriceScheduleResponse, error) {
	scheduleID = strings.TrimSpace(scheduleID)
	if scheduleID == "" {
		return nil, fmt.Errorf("scheduleID is required")
	}
	path := fmt.Sprintf("/v1/appPriceSchedules/%s", scheduleID)

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppPriceScheduleResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse app price schedule response: %w", err)
	}

	return &response, nil
}

// CreateAppPriceSchedule creates an app price schedule with a manual price.
func (c *Client) CreateAppPriceSchedule(ctx context.Context, appID string, attrs AppPriceScheduleCreateAttributes) (*AppPriceScheduleResponse, error) {
	appID = strings.TrimSpace(appID)
	pricePointID := strings.TrimSpace(attrs.PricePointID)
	startDate := strings.TrimSpace(attrs.StartDate)
	baseTerritoryID := strings.ToUpper(strings.TrimSpace(attrs.BaseTerritoryID))
	if appID == "" {
		return nil, fmt.Errorf("app ID is required")
	}
	if pricePointID == "" {
		return nil, fmt.Errorf("price point ID is required")
	}
	if startDate == "" {
		return nil, fmt.Errorf("start date is required")
	}
	if baseTerritoryID == "" {
		return nil, fmt.Errorf("base territory ID is required")
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
				ManualPrices: RelationshipList{
					Data: []ResourceData{
						{
							Type: ResourceTypeAppPrices,
							ID:   appPriceScheduleManualPriceID,
						},
					},
				},
			},
		},
		Included: []AppPriceCreateResource{
			{
				Type:       ResourceTypeAppPrices,
				ID:         appPriceScheduleManualPriceID,
				Attributes: AppPriceAttributes{StartDate: startDate},
				Relationships: AppPriceRelationships{
					AppPricePoint: Relationship{
						Data: ResourceData{
							Type: ResourceTypeAppPricePoints,
							ID:   pricePointID,
						},
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/appPriceSchedules", body)
	if err != nil {
		return nil, err
	}

	var response AppPriceScheduleResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse app price schedule response: %w", err)
	}

	return &response, nil
}

// GetAppPriceScheduleBaseTerritory retrieves the base territory for a schedule.
func (c *Client) GetAppPriceScheduleBaseTerritory(ctx context.Context, scheduleID string) (*TerritoryResponse, error) {
	scheduleID = strings.TrimSpace(scheduleID)
	path := fmt.Sprintf("/v1/appPriceSchedules/%s/baseTerritory", scheduleID)

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response TerritoryResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse base territory response: %w", err)
	}

	return &response, nil
}

// GetAppPriceScheduleManualPrices retrieves manual prices for a schedule.
func (c *Client) GetAppPriceScheduleManualPrices(ctx context.Context, scheduleID string) (*AppPricesResponse, error) {
	scheduleID = strings.TrimSpace(scheduleID)
	path := fmt.Sprintf("/v1/appPriceSchedules/%s/manualPrices", scheduleID)

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppPricesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse manual prices response: %w", err)
	}

	return &response, nil
}

// GetAppPriceScheduleAutomaticPrices retrieves automatic prices for a schedule.
func (c *Client) GetAppPriceScheduleAutomaticPrices(ctx context.Context, scheduleID string) (*AppPricesResponse, error) {
	scheduleID = strings.TrimSpace(scheduleID)
	path := fmt.Sprintf("/v1/appPriceSchedules/%s/automaticPrices", scheduleID)

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppPricesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse automatic prices response: %w", err)
	}

	return &response, nil
}

// GetAppAvailabilityV2 retrieves app availability for an app.
func (c *Client) GetAppAvailabilityV2(ctx context.Context, appID string) (*AppAvailabilityV2Response, error) {
	appID = strings.TrimSpace(appID)
	path := fmt.Sprintf("/v1/apps/%s/appAvailabilityV2", appID)

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppAvailabilityV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse app availability response: %w", err)
	}

	return &response, nil
}

// GetAppAvailabilityV2ByID retrieves app availability by ID.
func (c *Client) GetAppAvailabilityV2ByID(ctx context.Context, availabilityID string) (*AppAvailabilityV2Response, error) {
	availabilityID = strings.TrimSpace(availabilityID)
	if availabilityID == "" {
		return nil, fmt.Errorf("availabilityID is required")
	}
	path := fmt.Sprintf("/v2/appAvailabilities/%s", availabilityID)

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppAvailabilityV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse app availability response: %w", err)
	}

	return &response, nil
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
		values.Set("fields[territoryAvailabilities]", "available,releaseDate,preOrderEnabled,territory")
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

// CreateAppAvailabilityV2 creates or updates app availability.
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

	if len(attrs.TerritoryAvailabilities) > 0 {
		payload.Included = make([]TerritoryAvailabilityCreateResource, 0, len(attrs.TerritoryAvailabilities))
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
			payload.Included = append(payload.Included, TerritoryAvailabilityCreateResource{
				Type: ResourceTypeTerritoryAvailabilities,
				ID:   resourceID,
				Attributes: TerritoryAvailabilityCreateAttributes{
					Available:       availability.Available,
					ReleaseDate:     strings.TrimSpace(availability.ReleaseDate),
					PreOrderEnabled: availability.PreOrderEnabled,
				},
				Relationships: TerritoryAvailabilityRelationships{
					Territory: Relationship{
						Data: ResourceData{
							Type: ResourceTypeTerritories,
							ID:   territoryID,
						},
					},
				},
			})
		}
		payload.Data.Relationships.TerritoryAvailabilities = RelationshipList{
			Data: relationshipData,
		}
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v2/appAvailabilities", body)
	if err != nil {
		return nil, err
	}

	var response AppAvailabilityV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse app availability response: %w", err)
	}

	return &response, nil
}

// UpdateTerritoryAvailability updates a territory availability.
func (c *Client) UpdateTerritoryAvailability(ctx context.Context, territoryAvailabilityID string, attrs TerritoryAvailabilityUpdateAttributes) (*TerritoryAvailabilityResponse, error) {
	territoryAvailabilityID = strings.TrimSpace(territoryAvailabilityID)
	if territoryAvailabilityID == "" {
		return nil, fmt.Errorf("territory availability ID is required")
	}

	attributes := TerritoryAvailabilityUpdateAttributes{
		Available:       attrs.Available,
		PreOrderEnabled: attrs.PreOrderEnabled,
	}
	if attrs.ReleaseDate != nil {
		trimmed := strings.TrimSpace(*attrs.ReleaseDate)
		if trimmed == "" {
			return nil, fmt.Errorf("release date is required")
		}
		attributes.ReleaseDate = &trimmed
	}

	if attributes.Available == nil && attributes.ReleaseDate == nil && attributes.PreOrderEnabled == nil {
		return nil, fmt.Errorf("at least one attribute is required")
	}

	payload := TerritoryAvailabilityUpdateRequest{
		Data: TerritoryAvailabilityUpdateData{
			Type:       ResourceTypeTerritoryAvailabilities,
			ID:         territoryAvailabilityID,
			Attributes: &attributes,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/territoryAvailabilities/%s", territoryAvailabilityID)
	data, err := c.do(ctx, "PATCH", path, body)
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/endAppAvailabilityPreOrders", body)
	if err != nil {
		return nil, err
	}

	var response EndAppAvailabilityPreOrderResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse end app availability pre-order response: %w", err)
	}

	return &response, nil
}
