package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GetSubscriptionGroups retrieves the list of subscription groups for an app.
func (c *Client) GetSubscriptionGroups(ctx context.Context, appID string, opts ...SubscriptionGroupsOption) (*SubscriptionGroupsResponse, error) {
	query := &subscriptionGroupsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/apps/%s/subscriptionGroups", strings.TrimSpace(appID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("subscriptionGroups: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildSubscriptionGroupsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionGroupsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateSubscriptionGroup creates a subscription group for an app.
func (c *Client) CreateSubscriptionGroup(ctx context.Context, appID string, attrs SubscriptionGroupCreateAttributes) (*SubscriptionGroupResponse, error) {
	payload := SubscriptionGroupCreateRequest{
		Data: SubscriptionGroupCreateData{
			Type:       ResourceTypeSubscriptionGroups,
			Attributes: attrs,
			Relationships: &SubscriptionGroupRelationships{
				App: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   strings.TrimSpace(appID),
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/subscriptionGroups", body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionGroupResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetSubscriptionGroup retrieves a subscription group by ID.
func (c *Client) GetSubscriptionGroup(ctx context.Context, groupID string) (*SubscriptionGroupResponse, error) {
	path := fmt.Sprintf("/v1/subscriptionGroups/%s", strings.TrimSpace(groupID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionGroupResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateSubscriptionGroup updates a subscription group.
func (c *Client) UpdateSubscriptionGroup(ctx context.Context, groupID string, attrs SubscriptionGroupUpdateAttributes) (*SubscriptionGroupResponse, error) {
	payload := SubscriptionGroupUpdateRequest{
		Data: SubscriptionGroupUpdateData{
			Type:       ResourceTypeSubscriptionGroups,
			ID:         strings.TrimSpace(groupID),
			Attributes: attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/subscriptionGroups/%s", strings.TrimSpace(groupID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionGroupResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteSubscriptionGroup deletes a subscription group.
func (c *Client) DeleteSubscriptionGroup(ctx context.Context, groupID string) error {
	path := fmt.Sprintf("/v1/subscriptionGroups/%s", strings.TrimSpace(groupID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetSubscriptions retrieves the list of subscriptions for a group.
func (c *Client) GetSubscriptions(ctx context.Context, groupID string, opts ...SubscriptionsOption) (*SubscriptionsResponse, error) {
	query := &subscriptionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/subscriptionGroups/%s/subscriptions", strings.TrimSpace(groupID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("subscriptions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildSubscriptionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateSubscription creates a subscription within a group.
func (c *Client) CreateSubscription(ctx context.Context, groupID string, attrs SubscriptionCreateAttributes) (*SubscriptionResponse, error) {
	payload := SubscriptionCreateRequest{
		Data: SubscriptionCreateData{
			Type:       ResourceTypeSubscriptions,
			Attributes: attrs,
			Relationships: &SubscriptionRelationships{
				Group: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeSubscriptionGroups,
						ID:   strings.TrimSpace(groupID),
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/subscriptions", body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetSubscription retrieves a subscription by ID.
func (c *Client) GetSubscription(ctx context.Context, subID string) (*SubscriptionResponse, error) {
	path := fmt.Sprintf("/v1/subscriptions/%s", strings.TrimSpace(subID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateSubscription updates a subscription.
func (c *Client) UpdateSubscription(ctx context.Context, subID string, attrs SubscriptionUpdateAttributes) (*SubscriptionResponse, error) {
	payload := SubscriptionUpdateRequest{
		Data: SubscriptionUpdateData{
			Type:       ResourceTypeSubscriptions,
			ID:         strings.TrimSpace(subID),
			Attributes: attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/subscriptions/%s", strings.TrimSpace(subID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteSubscription deletes a subscription.
func (c *Client) DeleteSubscription(ctx context.Context, subID string) error {
	path := fmt.Sprintf("/v1/subscriptions/%s", strings.TrimSpace(subID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// CreateSubscriptionPrice adds a price to a subscription.
func (c *Client) CreateSubscriptionPrice(ctx context.Context, subID, pricePointID, territoryID string, attrs SubscriptionPriceCreateAttributes) (*SubscriptionPriceResponse, error) {
	subID = strings.TrimSpace(subID)
	pricePointID = strings.TrimSpace(pricePointID)
	territoryID = strings.ToUpper(strings.TrimSpace(territoryID))
	if subID == "" || pricePointID == "" {
		return nil, fmt.Errorf("subscription ID and price point ID are required")
	}

	var attributes *SubscriptionPriceCreateAttributes
	if attrs.StartDate != "" || attrs.Preserved != nil {
		attributes = &attrs
	}

	relationships := &SubscriptionPriceRelationships{
		Subscription: &Relationship{
			Data: ResourceData{
				Type: ResourceTypeSubscriptions,
				ID:   subID,
			},
		},
		SubscriptionPricePoint: &Relationship{
			Data: ResourceData{
				Type: ResourceTypeSubscriptionPricePoints,
				ID:   pricePointID,
			},
		},
	}
	if territoryID != "" {
		relationships.Territory = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeTerritories,
				ID:   territoryID,
			},
		}
	}

	payload := SubscriptionPriceCreateRequest{
		Data: SubscriptionPriceCreateData{
			Type:          ResourceTypeSubscriptionPrices,
			Attributes:    attributes,
			Relationships: relationships,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/subscriptionPrices", body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionPriceResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteSubscriptionPrice deletes a subscription price.
func (c *Client) DeleteSubscriptionPrice(ctx context.Context, priceID string) error {
	path := fmt.Sprintf("/v1/subscriptionPrices/%s", strings.TrimSpace(priceID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// CreateSubscriptionAvailability sets subscription availability in territories.
func (c *Client) CreateSubscriptionAvailability(ctx context.Context, subID string, territoryIDs []string, attrs SubscriptionAvailabilityAttributes) (*SubscriptionAvailabilityResponse, error) {
	subID = strings.TrimSpace(subID)
	territoryIDs = normalizeList(territoryIDs)
	if subID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}
	if len(territoryIDs) == 0 {
		return nil, fmt.Errorf("territory IDs are required")
	}

	relData := make([]ResourceData, 0, len(territoryIDs))
	for _, territoryID := range territoryIDs {
		relData = append(relData, ResourceData{
			Type: ResourceTypeTerritories,
			ID:   territoryID,
		})
	}

	payload := SubscriptionAvailabilityCreateRequest{
		Data: SubscriptionAvailabilityCreateData{
			Type:       ResourceTypeSubscriptionAvailabilities,
			Attributes: attrs,
			Relationships: &SubscriptionAvailabilityRelationships{
				Subscription: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeSubscriptions,
						ID:   subID,
					},
				},
				AvailableTerritories: &RelationshipList{Data: relData},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/subscriptionAvailabilities", body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionAvailabilityResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetSubscriptionAvailability retrieves a subscription availability by ID.
func (c *Client) GetSubscriptionAvailability(ctx context.Context, availabilityID string) (*SubscriptionAvailabilityResponse, error) {
	availabilityID = strings.TrimSpace(availabilityID)
	if availabilityID == "" {
		return nil, fmt.Errorf("availabilityID is required")
	}

	path := fmt.Sprintf("/v1/subscriptionAvailabilities/%s", availabilityID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionAvailabilityResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetSubscriptionAvailabilityForSubscription retrieves availability for a subscription.
func (c *Client) GetSubscriptionAvailabilityForSubscription(ctx context.Context, subID string) (*SubscriptionAvailabilityResponse, error) {
	subID = strings.TrimSpace(subID)
	if subID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}

	path := fmt.Sprintf("/v1/subscriptions/%s/subscriptionAvailability", subID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionAvailabilityResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetSubscriptionAvailabilityAvailableTerritories lists available territories for an availability.
func (c *Client) GetSubscriptionAvailabilityAvailableTerritories(ctx context.Context, availabilityID string, opts ...SubscriptionAvailabilityTerritoriesOption) (*TerritoriesResponse, error) {
	query := &subscriptionAvailabilityTerritoriesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	availabilityID = strings.TrimSpace(availabilityID)
	if query.nextURL == "" && availabilityID == "" {
		return nil, fmt.Errorf("availabilityID is required")
	}

	path := fmt.Sprintf("/v1/subscriptionAvailabilities/%s/availableTerritories", availabilityID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("subscription-availability-territories: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildSubscriptionAvailabilityTerritoriesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response TerritoriesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetSubscriptionAppStoreReviewScreenshotForSubscription retrieves the review screenshot for a subscription.
func (c *Client) GetSubscriptionAppStoreReviewScreenshotForSubscription(ctx context.Context, subID string) (*SubscriptionAppStoreReviewScreenshotResponse, error) {
	subID = strings.TrimSpace(subID)
	if subID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}

	path := fmt.Sprintf("/v1/subscriptions/%s/appStoreReviewScreenshot", subID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionAppStoreReviewScreenshotResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetSubscriptionPromotedPurchase retrieves the promoted purchase for a subscription.
func (c *Client) GetSubscriptionPromotedPurchase(ctx context.Context, subID string) (*PromotedPurchaseResponse, error) {
	subID = strings.TrimSpace(subID)
	if subID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}

	path := fmt.Sprintf("/v1/subscriptions/%s/promotedPurchase", subID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response PromotedPurchaseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
