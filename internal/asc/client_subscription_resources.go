package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GetSubscriptionLocalizations retrieves subscription localizations for a subscription.
//
// Deprecated: Use GetSubscriptionVersionLocalizations with a subscription version ID.
func (c *Client) GetSubscriptionLocalizations(ctx context.Context, subscriptionID string, opts ...SubscriptionLocalizationsOption) (*SubscriptionLocalizationsResponse, error) {
	query := &subscriptionLocalizationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/subscriptions/%s/subscriptionLocalizations", strings.TrimSpace(subscriptionID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("subscriptionLocalizations: %w", err)
		}
		if subscriptionLocalizationsQueryHasModifiers(query) {
			return nil, fmt.Errorf("subscriptionLocalizations: next URL cannot be combined with query options")
		}
		path = query.nextURL
	} else if queryString := buildSubscriptionLocalizationsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionLocalizationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetSubscriptionImages retrieves subscription images for a subscription.
//
// Deprecated: Use GetSubscriptionVersionImages with a subscription version ID.
func (c *Client) GetSubscriptionImages(ctx context.Context, subscriptionID string, opts ...SubscriptionImagesOption) (*SubscriptionImagesResponse, error) {
	query := &subscriptionImagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/subscriptions/%s/images", strings.TrimSpace(subscriptionID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("subscriptionImages: %w", err)
		}
		if subscriptionImagesQueryHasModifiers(query) {
			return nil, fmt.Errorf("subscriptionImages: next URL cannot be combined with query options")
		}
		path = query.nextURL
	} else if queryString := buildSubscriptionImagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionImagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionIntroductoryOffers retrieves introductory offers for a subscription.
func (c *Client) GetSubscriptionIntroductoryOffers(ctx context.Context, subscriptionID string, opts ...SubscriptionIntroductoryOffersOption) (*SubscriptionIntroductoryOffersResponse, error) {
	query := &subscriptionIntroductoryOffersQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/subscriptions/%s/introductoryOffers", strings.TrimSpace(subscriptionID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("subscriptionIntroductoryOffers: %w", err)
		}
		if subscriptionIntroductoryOffersQueryHasModifiers(query) {
			return nil, fmt.Errorf("subscriptionIntroductoryOffers: next URL cannot be combined with query options")
		}
		path = query.nextURL
	} else if queryString := buildSubscriptionIntroductoryOffersQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionIntroductoryOffersResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// CreateSubscriptionIntroductoryOffer creates an introductory offer.
func (c *Client) CreateSubscriptionIntroductoryOffer(ctx context.Context, subscriptionID string, attrs SubscriptionIntroductoryOfferCreateAttributes, territoryID, pricePointID string) (*SubscriptionIntroductoryOfferResponse, error) {
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}

	relationships := &SubscriptionIntroductoryOfferRelationships{
		Subscription: &Relationship{
			Data: ResourceData{
				Type: ResourceTypeSubscriptions,
				ID:   subscriptionID,
			},
		},
	}
	if strings.TrimSpace(territoryID) != "" {
		relationships.Territory = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeTerritories,
				ID:   strings.TrimSpace(territoryID),
			},
		}
	}
	if strings.TrimSpace(pricePointID) != "" {
		relationships.SubscriptionPricePoint = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeSubscriptionPricePoints,
				ID:   strings.TrimSpace(pricePointID),
			},
		}
	}

	payload := SubscriptionIntroductoryOfferCreateRequest{
		Data: SubscriptionIntroductoryOfferCreateData{
			Type:          ResourceTypeSubscriptionIntroductoryOffers,
			Attributes:    attrs,
			Relationships: relationships,
		},
	}

	return sendJSON[SubscriptionIntroductoryOfferResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v1/subscriptionIntroductoryOffers",
		Payload: payload,
	})
}

// UpdateSubscriptionIntroductoryOffer updates an introductory offer.
func (c *Client) UpdateSubscriptionIntroductoryOffer(ctx context.Context, offerID string, attrs SubscriptionIntroductoryOfferUpdateAttributes) (*SubscriptionIntroductoryOfferResponse, error) {
	payload := SubscriptionIntroductoryOfferUpdateRequest{
		Data: SubscriptionIntroductoryOfferUpdateData{
			Type:       ResourceTypeSubscriptionIntroductoryOffers,
			ID:         strings.TrimSpace(offerID),
			Attributes: attrs,
		},
	}

	return sendJSON[SubscriptionIntroductoryOfferResponse](ctx, c, writeRequest{
		Method:  http.MethodPatch,
		Path:    fmt.Sprintf("/v1/subscriptionIntroductoryOffers/%s", strings.TrimSpace(offerID)),
		Payload: payload,
	})
}

// GetSubscriptionPromotionalOffers retrieves promotional offers for a subscription.
func (c *Client) GetSubscriptionPromotionalOffers(ctx context.Context, subscriptionID string, opts ...SubscriptionPromotionalOffersOption) (*SubscriptionPromotionalOffersResponse, error) {
	query := &subscriptionPromotionalOffersQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/subscriptions/%s/promotionalOffers", strings.TrimSpace(subscriptionID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("subscriptionPromotionalOffers: %w", err)
		}
		if subscriptionPromotionalOffersQueryHasModifiers(query) {
			return nil, fmt.Errorf("subscriptionPromotionalOffers: next URL cannot be combined with query options")
		}
		path = query.nextURL
	} else if queryString := buildSubscriptionPromotionalOffersQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionPromotionalOffersResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionPromotionalOffer retrieves a promotional offer by ID.
func (c *Client) GetSubscriptionPromotionalOffer(ctx context.Context, offerID string, opts ...SubscriptionPromotionalOfferOption) (*SubscriptionPromotionalOfferResponse, error) {
	query := applyOptions(opts)
	return getJSON[SubscriptionPromotionalOfferResponse](ctx, c, readRequest{
		Path:  fmt.Sprintf("/v1/subscriptionPromotionalOffers/%s", strings.TrimSpace(offerID)),
		Query: buildSubscriptionRelatedFieldsQuery("subscriptions", query),
	})
}

// CreateSubscriptionPromotionalOffer creates a promotional offer.
func (c *Client) CreateSubscriptionPromotionalOffer(ctx context.Context, subscriptionID string, attrs SubscriptionPromotionalOfferCreateAttributes, prices []SubscriptionPromotionalOfferPrice) (*SubscriptionPromotionalOfferResponse, error) {
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}
	if len(prices) == 0 {
		return nil, fmt.Errorf("at least one price is required")
	}

	priceData := make([]ResourceData, 0, len(prices))
	included := make([]SubscriptionPromotionalOfferPriceInlineCreate, 0, len(prices))
	usesReferences := false
	usesInlinePrices := false
	for idx, price := range prices {
		priceID := strings.TrimSpace(price.ID)
		territoryID := strings.ToUpper(strings.TrimSpace(price.TerritoryID))
		pricePointID := strings.TrimSpace(price.PricePointID)
		if priceID != "" {
			if territoryID != "" || pricePointID != "" {
				return nil, fmt.Errorf("price reference ID must not be combined with inline relationships")
			}
			usesReferences = true
			priceData = append(priceData, ResourceData{
				Type: ResourceTypeSubscriptionPromotionalOfferPrices,
				ID:   priceID,
			})
			continue
		}
		if territoryID == "" && pricePointID == "" {
			return nil, fmt.Errorf("price reference ID or inline relationship is required")
		}
		usesInlinePrices = true

		priceRelationships := SubscriptionPromotionalOfferPriceRelationships{}
		if territoryID != "" {
			priceRelationships.Territory = &Relationship{
				Data: ResourceData{
					Type: ResourceTypeTerritories,
					ID:   territoryID,
				},
			}
		}
		if pricePointID != "" {
			priceRelationships.SubscriptionPricePoint = &Relationship{
				Data: ResourceData{
					Type: ResourceTypeSubscriptionPricePoints,
					ID:   pricePointID,
				},
			}
		}

		resourceID := fmt.Sprintf("${local-price-%d}", idx+1)
		priceData = append(priceData, ResourceData{
			Type: ResourceTypeSubscriptionPromotionalOfferPrices,
			ID:   resourceID,
		})
		included = append(included, SubscriptionPromotionalOfferPriceInlineCreate{
			Type:          ResourceTypeSubscriptionPromotionalOfferPrices,
			ID:            resourceID,
			Relationships: priceRelationships,
		})
	}
	if usesReferences && usesInlinePrices {
		return nil, fmt.Errorf("price inputs must not mix existing IDs with inline relationships")
	}

	payload := SubscriptionPromotionalOfferCreateRequest{
		Data: SubscriptionPromotionalOfferCreateData{
			Type:       ResourceTypeSubscriptionPromotionalOffers,
			Attributes: attrs,
			Relationships: SubscriptionPromotionalOfferRelationships{
				Subscription: Relationship{
					Data: ResourceData{
						Type: ResourceTypeSubscriptions,
						ID:   subscriptionID,
					},
				},
				Prices: RelationshipList{Data: priceData},
			},
		},
		Included: included,
	}

	return sendJSON[SubscriptionPromotionalOfferResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v1/subscriptionPromotionalOffers",
		Payload: payload,
	})
}

// UpdateSubscriptionPromotionalOffer updates a promotional offer.
func (c *Client) UpdateSubscriptionPromotionalOffer(ctx context.Context, offerID string, priceIDs []string) (*SubscriptionPromotionalOfferResponse, error) {
	priceIDs = normalizeList(priceIDs)
	if len(priceIDs) == 0 {
		return nil, fmt.Errorf("price IDs are required")
	}

	priceData := make([]ResourceData, 0, len(priceIDs))
	for _, priceID := range priceIDs {
		priceData = append(priceData, ResourceData{
			Type: ResourceTypeSubscriptionPromotionalOfferPrices,
			ID:   priceID,
		})
	}

	payload := SubscriptionPromotionalOfferUpdateRequest{
		Data: SubscriptionPromotionalOfferUpdateData{
			Type: ResourceTypeSubscriptionPromotionalOffers,
			ID:   strings.TrimSpace(offerID),
			Relationships: &SubscriptionPromotionalOfferUpdateRelationships{
				Prices: RelationshipList{Data: priceData},
			},
		},
	}

	return sendJSON[SubscriptionPromotionalOfferResponse](ctx, c, writeRequest{
		Method:  http.MethodPatch,
		Path:    fmt.Sprintf("/v1/subscriptionPromotionalOffers/%s", strings.TrimSpace(offerID)),
		Payload: payload,
	})
}

// GetSubscriptionPromotionalOfferPrices retrieves prices for a promotional offer.
func (c *Client) GetSubscriptionPromotionalOfferPrices(ctx context.Context, offerID string, opts ...SubscriptionPromotionalOfferPricesOption) (*SubscriptionPromotionalOfferPricesResponse, error) {
	query := &subscriptionPromotionalOfferPricesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/subscriptionPromotionalOffers/%s/prices", strings.TrimSpace(offerID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("subscriptionPromotionalOfferPrices: %w", err)
		}
		if subscriptionPromotionalOfferPricesQueryHasModifiers(query) {
			return nil, fmt.Errorf("subscriptionPromotionalOfferPrices: next URL cannot be combined with query options")
		}
		path = query.nextURL
	} else if queryString := buildSubscriptionPromotionalOfferPricesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionPromotionalOfferPricesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionOfferCodes retrieves offer codes for a subscription.
func (c *Client) GetSubscriptionOfferCodes(ctx context.Context, subscriptionID string, opts ...SubscriptionOfferCodesOption) (*SubscriptionOfferCodesResponse, error) {
	query := &subscriptionOfferCodesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/subscriptions/%s/offerCodes", strings.TrimSpace(subscriptionID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("subscriptionOfferCodes: %w", err)
		}
		if subscriptionOfferCodesQueryHasModifiers(query) {
			return nil, fmt.Errorf("subscriptionOfferCodes: next URL cannot be combined with query options")
		}
		path = query.nextURL
	} else if queryString := buildSubscriptionOfferCodesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionOfferCodesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionOfferCode retrieves an offer code by ID.
func (c *Client) GetSubscriptionOfferCode(ctx context.Context, offerCodeID string, opts ...SubscriptionOfferCodeOption) (*SubscriptionOfferCodeResponse, error) {
	query := applyOptions(opts)
	return getJSON[SubscriptionOfferCodeResponse](ctx, c, readRequest{
		Path:  fmt.Sprintf("/v1/subscriptionOfferCodes/%s", strings.TrimSpace(offerCodeID)),
		Query: buildSubscriptionRelatedFieldsQuery("subscriptions", query),
	})
}

// CreateSubscriptionOfferCode creates a subscription offer code.
func (c *Client) CreateSubscriptionOfferCode(ctx context.Context, subscriptionID string, attrs SubscriptionOfferCodeCreateAttributes, prices []SubscriptionOfferCodePrice) (*SubscriptionOfferCodeResponse, error) {
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}
	if len(prices) == 0 {
		return nil, fmt.Errorf("at least one price is required")
	}

	isFreeTrial := attrs.OfferMode == SubscriptionOfferModeFreeTrial
	priceData := make([]ResourceData, 0, len(prices))
	included := make([]SubscriptionOfferCodePriceInlineCreate, 0, len(prices))
	for idx, price := range prices {
		territoryID := strings.ToUpper(strings.TrimSpace(price.TerritoryID))
		pricePointID := strings.TrimSpace(price.PricePointID)
		if territoryID == "" {
			return nil, fmt.Errorf("territory ID is required")
		}
		if isFreeTrial && pricePointID != "" {
			return nil, fmt.Errorf("price point must not be set for FREE_TRIAL offer mode")
		}
		if !isFreeTrial && pricePointID == "" {
			return nil, fmt.Errorf("price point ID is required")
		}

		priceRelationships := SubscriptionOfferCodePriceRelationships{
			Territory: Relationship{
				Data: ResourceData{
					Type: ResourceTypeTerritories,
					ID:   territoryID,
				},
			},
		}
		if pricePointID != "" {
			priceRelationships.SubscriptionPricePoint = &Relationship{
				Data: ResourceData{
					Type: ResourceTypeSubscriptionPricePoints,
					ID:   pricePointID,
				},
			}
		}

		resourceID := fmt.Sprintf("${local-price-%d}", idx+1)
		priceData = append(priceData, ResourceData{
			Type: ResourceTypeSubscriptionOfferCodePrices,
			ID:   resourceID,
		})
		included = append(included, SubscriptionOfferCodePriceInlineCreate{
			Type:          ResourceTypeSubscriptionOfferCodePrices,
			ID:            resourceID,
			Relationships: priceRelationships,
		})
	}

	payload := SubscriptionOfferCodeCreateRequest{
		Data: SubscriptionOfferCodeCreateData{
			Type:       ResourceTypeSubscriptionOfferCodes,
			Attributes: attrs,
			Relationships: SubscriptionOfferCodeRelationships{
				Subscription: Relationship{
					Data: ResourceData{
						Type: ResourceTypeSubscriptions,
						ID:   subscriptionID,
					},
				},
				Prices: RelationshipList{Data: priceData},
			},
		},
		Included: included,
	}

	return sendJSON[SubscriptionOfferCodeResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v1/subscriptionOfferCodes",
		Payload: payload,
	})
}

// UpdateSubscriptionOfferCode updates an offer code.
func (c *Client) UpdateSubscriptionOfferCode(ctx context.Context, offerCodeID string, attrs SubscriptionOfferCodeUpdateAttributes) (*SubscriptionOfferCodeResponse, error) {
	payload := SubscriptionOfferCodeUpdateRequest{
		Data: SubscriptionOfferCodeUpdateData{
			Type:       ResourceTypeSubscriptionOfferCodes,
			ID:         strings.TrimSpace(offerCodeID),
			Attributes: attrs,
		},
	}

	return sendJSON[SubscriptionOfferCodeResponse](ctx, c, writeRequest{
		Method:  http.MethodPatch,
		Path:    fmt.Sprintf("/v1/subscriptionOfferCodes/%s", strings.TrimSpace(offerCodeID)),
		Payload: payload,
	})
}

// GetSubscriptionOfferCodePrices retrieves prices for an offer code.
func (c *Client) GetSubscriptionOfferCodePrices(ctx context.Context, offerCodeID string, opts ...SubscriptionOfferCodePricesOption) (*SubscriptionOfferCodePricesResponse, error) {
	query := &subscriptionOfferCodePricesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/subscriptionOfferCodes/%s/prices", strings.TrimSpace(offerCodeID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("subscriptionOfferCodePrices: %w", err)
		}
		if subscriptionOfferCodePricesQueryHasModifiers(query) {
			return nil, fmt.Errorf("subscriptionOfferCodePrices: next URL cannot be combined with query options")
		}
		path = query.nextURL
	} else if queryString := buildSubscriptionOfferCodePricesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionOfferCodePricesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionPricePoints retrieves price points for a subscription.
func (c *Client) GetSubscriptionPricePoints(ctx context.Context, subscriptionID string, opts ...SubscriptionPricePointsOption) (*SubscriptionPricePointsResponse, error) {
	query := &subscriptionPricePointsQuery{}
	for _, opt := range opts {
		opt.applySubscriptionPricePoints(query)
	}

	subscriptionID = strings.TrimSpace(subscriptionID)
	if query.nextURL == "" && subscriptionID == "" {
		return nil, fmt.Errorf("subscriptionID is required")
	}

	path := fmt.Sprintf("/v1/subscriptions/%s/pricePoints", subscriptionID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("subscriptionPricePoints: %w", err)
		}
		if subscriptionPricePointsQueryHasModifiers(query) {
			return nil, fmt.Errorf("subscriptionPricePoints: next URL cannot be combined with query options")
		}
		path = query.nextURL
	} else if queryString := buildSubscriptionPricePointsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionPricePointsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionPricePointEqualizations retrieves equalizations for a price point.
func (c *Client) GetSubscriptionPricePointEqualizations(ctx context.Context, pricePointID string, opts ...SubscriptionPricePointEqualizationsOption) (*SubscriptionPricePointsResponse, error) {
	query := &subscriptionPricePointsQuery{}
	for _, opt := range opts {
		opt.applySubscriptionPricePoints(query)
	}

	pricePointID = strings.TrimSpace(pricePointID)
	if query.nextURL == "" && pricePointID == "" {
		return nil, fmt.Errorf("pricePointID is required")
	}

	path := fmt.Sprintf("/v1/subscriptionPricePoints/%s/equalizations", pricePointID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("subscriptionPricePointEqualizations: %w", err)
		}
		if subscriptionPricePointsQueryHasModifiers(query) {
			return nil, fmt.Errorf("subscriptionPricePointEqualizations: next URL cannot be combined with query options")
		}
		path = query.nextURL
	} else if queryString := buildSubscriptionPricePointsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionPricePointsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionPricePointAdjustedEqualizations retrieves adjusted equalizations for a price point.
func (c *Client) GetSubscriptionPricePointAdjustedEqualizations(ctx context.Context, pricePointID string, opts ...SubscriptionPricePointEqualizationsOption) (*SubscriptionPricePointsResponse, error) {
	query := &subscriptionPricePointsQuery{}
	for _, opt := range opts {
		opt.applySubscriptionPricePoints(query)
	}

	pricePointID = strings.TrimSpace(pricePointID)
	if query.nextURL == "" && pricePointID == "" {
		return nil, fmt.Errorf("pricePointID is required")
	}

	path := fmt.Sprintf("/v1/subscriptionPricePoints/%s/adjustedEqualizations", pricePointID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("subscriptionPricePointAdjustedEqualizations: %w", err)
		}
		if subscriptionPricePointsQueryHasModifiers(query) {
			return nil, fmt.Errorf("subscriptionPricePointAdjustedEqualizations: next URL cannot be combined with query options")
		}
		path = query.nextURL
	} else if queryString := buildSubscriptionPricePointsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionPricePointsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// CreateSubscriptionAppStoreReviewScreenshot creates a review screenshot.
func (c *Client) CreateSubscriptionAppStoreReviewScreenshot(ctx context.Context, subscriptionID, fileName string, fileSize int64) (*SubscriptionAppStoreReviewScreenshotResponse, error) {
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return nil, fmt.Errorf("file name is required")
	}

	payload := SubscriptionAppStoreReviewScreenshotCreateRequest{
		Data: SubscriptionAppStoreReviewScreenshotCreateData{
			Type: ResourceTypeSubscriptionAppStoreReviewScreenshots,
			Attributes: SubscriptionAppStoreReviewScreenshotCreateAttributes{
				FileName: fileName,
				FileSize: fileSize,
			},
			Relationships: &SubscriptionAppStoreReviewScreenshotRelationships{
				Subscription: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeSubscriptions,
						ID:   subscriptionID,
					},
				},
			},
		},
	}

	return sendJSON[SubscriptionAppStoreReviewScreenshotResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v1/subscriptionAppStoreReviewScreenshots",
		Payload: payload,
	})
}

// UpdateSubscriptionAppStoreReviewScreenshot updates a review screenshot.
func (c *Client) UpdateSubscriptionAppStoreReviewScreenshot(ctx context.Context, screenshotID string, attrs SubscriptionAppStoreReviewScreenshotUpdateAttributes) (*SubscriptionAppStoreReviewScreenshotResponse, error) {
	payload := SubscriptionAppStoreReviewScreenshotUpdateRequest{
		Data: SubscriptionAppStoreReviewScreenshotUpdateData{
			Type:       ResourceTypeSubscriptionAppStoreReviewScreenshots,
			ID:         strings.TrimSpace(screenshotID),
			Attributes: attrs,
		},
	}

	return sendJSON[SubscriptionAppStoreReviewScreenshotResponse](ctx, c, writeRequest{
		Method:  http.MethodPatch,
		Path:    fmt.Sprintf("/v1/subscriptionAppStoreReviewScreenshots/%s", strings.TrimSpace(screenshotID)),
		Payload: payload,
	})
}

// GetSubscriptionGroupLocalizations retrieves subscription group localizations for a group.
//
// Deprecated: Use GetSubscriptionGroupVersionLocalizations with a subscription group version ID.
func (c *Client) GetSubscriptionGroupLocalizations(ctx context.Context, groupID string, opts ...SubscriptionGroupLocalizationsOption) (*SubscriptionGroupLocalizationsResponse, error) {
	query := &subscriptionGroupLocalizationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/subscriptionGroups/%s/subscriptionGroupLocalizations", strings.TrimSpace(groupID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("subscriptionGroupLocalizations: %w", err)
		}
		if subscriptionGroupLocalizationsQueryHasModifiers(query) {
			return nil, fmt.Errorf("subscriptionGroupLocalizations: next URL cannot be combined with query options")
		}
		path = query.nextURL
	} else if queryString := buildSubscriptionGroupLocalizationsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionGroupLocalizationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}
