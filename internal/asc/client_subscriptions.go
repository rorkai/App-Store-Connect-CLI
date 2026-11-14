package asc

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

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

	return sendJSON[SubscriptionGroupResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v1/subscriptionGroups",
		Payload: payload,
	})
}

// GetSubscriptionGroup retrieves a subscription group by ID.
func (c *Client) GetSubscriptionGroup(ctx context.Context, groupID string, opts ...SubscriptionGroupsOption) (*SubscriptionGroupResponse, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, fmt.Errorf("groupID is required")
	}
	query := applyOptions(opts)
	if query.limit > 0 {
		return nil, fmt.Errorf("limit is only supported when listing subscription groups")
	}
	if query.nextURL != "" {
		return nil, fmt.Errorf("next URL is only supported when listing subscription groups")
	}

	return getJSON[SubscriptionGroupResponse](ctx, c, readRequest{
		Path:  fmt.Sprintf("/v1/subscriptionGroups/%s", groupID),
		Query: buildSubscriptionGroupsQuery(query),
	})
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

	return sendJSON[SubscriptionGroupResponse](ctx, c, writeRequest{
		Method:  http.MethodPatch,
		Path:    fmt.Sprintf("/v1/subscriptionGroups/%s", strings.TrimSpace(groupID)),
		Payload: payload,
	})
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

	return sendJSON[SubscriptionResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v1/subscriptions",
		Payload: payload,
	})
}

// GetSubscription retrieves a subscription by ID.
func (c *Client) GetSubscription(ctx context.Context, subID string, opts ...SubscriptionOption) (*SubscriptionResponse, error) {
	subID = strings.TrimSpace(subID)
	if subID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}
	query := applyOptions(opts)
	return getJSON[SubscriptionResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/subscriptions/%s", subID), Query: buildSubscriptionQuery(query)})
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

	return sendJSON[SubscriptionResponse](ctx, c, writeRequest{
		Method:  http.MethodPatch,
		Path:    fmt.Sprintf("/v1/subscriptions/%s", strings.TrimSpace(subID)),
		Payload: payload,
	})
}

// SetSubscriptionPriceMatrix atomically sets an initial subscription price
// matrix using inline SubscriptionPriceInlineCreate resources.
func (c *Client) SetSubscriptionPriceMatrix(ctx context.Context, subID string, entries []SubscriptionInlinePrice) (*SubscriptionResponse, error) {
	subID = strings.TrimSpace(subID)
	if subID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("at least one subscription price is required")
	}

	normalized := make([]SubscriptionInlinePrice, 0, len(entries))
	byTerritory := make(map[string]SubscriptionInlinePrice, len(entries))
	for i, entry := range entries {
		entry.PricePointID = strings.TrimSpace(entry.PricePointID)
		entry.TerritoryID = strings.ToUpper(strings.TrimSpace(entry.TerritoryID))
		entry.Attributes.StartDate = strings.TrimSpace(entry.Attributes.StartDate)
		entry.Attributes.PlanType = SubscriptionPlanType(strings.TrimSpace(string(entry.Attributes.PlanType)))
		if entry.PricePointID == "" {
			return nil, fmt.Errorf("subscription price row %d: price point ID is required", i+1)
		}
		if previous, ok := byTerritory[entry.TerritoryID]; ok {
			if subscriptionPriceMatrixEntriesEqual(previous, entry) {
				continue
			}
			return nil, fmt.Errorf("subscription price matrix has duplicate territory %q with conflicting values", entry.TerritoryID)
		}
		byTerritory[entry.TerritoryID] = entry
		normalized = append(normalized, entry)
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].TerritoryID < normalized[j].TerritoryID
	})

	relationshipData := make([]ResourceData, 0, len(normalized))
	included := make([]SubscriptionPriceInlineCreate, 0, len(normalized))
	for i, entry := range normalized {
		inlinePriceID := fmt.Sprintf("${price-%d}", i+1)
		relationshipData = append(relationshipData, ResourceData{
			Type: ResourceTypeSubscriptionPrices,
			ID:   inlinePriceID,
		})

		var attributes *SubscriptionPriceCreateAttributes
		if entry.Attributes.StartDate != "" || entry.Attributes.Preserved != nil || entry.Attributes.PlanType != "" {
			attrs := entry.Attributes
			attributes = &attrs
		}
		relationships := SubscriptionPriceInlineRelationships{
			Subscription:           Relationship{Data: ResourceData{Type: ResourceTypeSubscriptions, ID: subID}},
			SubscriptionPricePoint: Relationship{Data: ResourceData{Type: ResourceTypeSubscriptionPricePoints, ID: entry.PricePointID}},
		}
		if entry.TerritoryID != "" {
			relationships.Territory = &Relationship{
				Data: ResourceData{
					Type: ResourceTypeTerritories,
					ID:   entry.TerritoryID,
				},
			}
		}
		included = append(included, SubscriptionPriceInlineCreate{
			Type:          ResourceTypeSubscriptionPrices,
			ID:            inlinePriceID,
			Attributes:    attributes,
			Relationships: relationships,
		})
	}

	payload := SubscriptionUpdateRequest{
		Data: SubscriptionUpdateData{
			Type: ResourceTypeSubscriptions,
			ID:   subID,
			Relationships: &SubscriptionUpdateRelationships{
				Prices: &RelationshipList{
					Data: relationshipData,
				},
			},
		},
		Included: included,
	}

	return sendJSON[SubscriptionResponse](ctx, c, writeRequest{
		Method:  http.MethodPatch,
		Path:    fmt.Sprintf("/v1/subscriptions/%s", subID),
		Payload: payload,
	})
}

func subscriptionPriceMatrixEntriesEqual(a, b SubscriptionInlinePrice) bool {
	if a.PricePointID != b.PricePointID || a.TerritoryID != b.TerritoryID || a.Attributes.StartDate != b.Attributes.StartDate || a.Attributes.PlanType != b.Attributes.PlanType {
		return false
	}
	if a.Attributes.Preserved == nil || b.Attributes.Preserved == nil {
		return a.Attributes.Preserved == nil && b.Attributes.Preserved == nil
	}
	return *a.Attributes.Preserved == *b.Attributes.Preserved
}

// SetSubscriptionInitialPrice sets the initial base price on a subscription that
// has no existing prices. This uses PATCH /v1/subscriptions/{id} with inline
// SubscriptionPriceInlineCreate resources, which is the only supported method
// for setting the first price on a new subscription. POST /v1/subscriptionPrices
// only works for price *changes* on subscriptions that already have a price.
func (c *Client) SetSubscriptionInitialPrice(ctx context.Context, subID, pricePointID, territoryID string, attrs SubscriptionPriceCreateAttributes) (*SubscriptionResponse, error) {
	return c.SetSubscriptionPriceMatrix(ctx, subID, []SubscriptionInlinePrice{{
		PricePointID: pricePointID,
		TerritoryID:  territoryID,
		Attributes:   attrs,
	}})
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
	if attrs.StartDate != "" || attrs.Preserved != nil || attrs.PlanType != "" {
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

	return sendJSON[SubscriptionPriceResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v1/subscriptionPrices",
		Payload: payload,
	})
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

	return sendJSON[SubscriptionAvailabilityResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v1/subscriptionAvailabilities",
		Payload: payload,
	})
}

// GetSubscriptionAppStoreReviewScreenshotForSubscription retrieves the review screenshot for a subscription.
func (c *Client) GetSubscriptionAppStoreReviewScreenshotForSubscription(ctx context.Context, subID string, opts ...SubscriptionAppStoreReviewScreenshotOption) (*SubscriptionAppStoreReviewScreenshotResponse, error) {
	subID = strings.TrimSpace(subID)
	if subID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}

	query := applyOptions(opts)
	return getJSON[SubscriptionAppStoreReviewScreenshotResponse](ctx, c, readRequest{
		Path:  fmt.Sprintf("/v1/subscriptions/%s/appStoreReviewScreenshot", subID),
		Query: buildSubscriptionAppStoreReviewScreenshotQuery(query),
	})
}

// GetSubscriptionPromotedPurchase retrieves the promoted purchase for a subscription.
func (c *Client) GetSubscriptionPromotedPurchase(ctx context.Context, subID string, opts ...PromotedPurchaseGetOption) (*PromotedPurchaseResponse, error) {
	subID = strings.TrimSpace(subID)
	if subID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}

	query := applyOptions(opts)
	return getJSON[PromotedPurchaseResponse](ctx, c, readRequest{
		Path:  fmt.Sprintf("/v1/subscriptions/%s/promotedPurchase", subID),
		Query: buildPromotedPurchaseGetQuery(query),
	})
}
