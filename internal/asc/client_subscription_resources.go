package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GetSubscriptionLocalizations retrieves subscription localizations for a subscription.
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

// GetSubscriptionLocalization retrieves a subscription localization by ID.
func (c *Client) GetSubscriptionLocalization(ctx context.Context, localizationID string) (*SubscriptionLocalizationResponse, error) {
	path := fmt.Sprintf("/v1/subscriptionLocalizations/%s", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// CreateSubscriptionLocalization creates a subscription localization.
func (c *Client) CreateSubscriptionLocalization(ctx context.Context, subscriptionID string, attrs SubscriptionLocalizationCreateAttributes) (*SubscriptionLocalizationResponse, error) {
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}

	payload := SubscriptionLocalizationCreateRequest{
		Data: SubscriptionLocalizationCreateData{
			Type:       ResourceTypeSubscriptionLocalizations,
			Attributes: attrs,
			Relationships: &SubscriptionLocalizationRelationships{
				Subscription: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeSubscriptions,
						ID:   subscriptionID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/subscriptionLocalizations", body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// UpdateSubscriptionLocalization updates a subscription localization.
func (c *Client) UpdateSubscriptionLocalization(ctx context.Context, localizationID string, attrs SubscriptionLocalizationUpdateAttributes) (*SubscriptionLocalizationResponse, error) {
	payload := SubscriptionLocalizationUpdateRequest{
		Data: SubscriptionLocalizationUpdateData{
			Type:       ResourceTypeSubscriptionLocalizations,
			ID:         strings.TrimSpace(localizationID),
			Attributes: attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/subscriptionLocalizations/%s", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// DeleteSubscriptionLocalization deletes a subscription localization.
func (c *Client) DeleteSubscriptionLocalization(ctx context.Context, localizationID string) error {
	path := fmt.Sprintf("/v1/subscriptionLocalizations/%s", strings.TrimSpace(localizationID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetSubscriptionImages retrieves subscription images for a subscription.
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

// GetSubscriptionImage retrieves a subscription image by ID.
func (c *Client) GetSubscriptionImage(ctx context.Context, imageID string) (*SubscriptionImageResponse, error) {
	path := fmt.Sprintf("/v1/subscriptionImages/%s", strings.TrimSpace(imageID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// CreateSubscriptionImage creates a subscription image.
func (c *Client) CreateSubscriptionImage(ctx context.Context, subscriptionID, fileName string, fileSize int64) (*SubscriptionImageResponse, error) {
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return nil, fmt.Errorf("file name is required")
	}

	payload := SubscriptionImageCreateRequest{
		Data: SubscriptionImageCreateData{
			Type: ResourceTypeSubscriptionImages,
			Attributes: SubscriptionImageCreateAttributes{
				FileName: fileName,
				FileSize: fileSize,
			},
			Relationships: &SubscriptionImageRelationships{
				Subscription: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeSubscriptions,
						ID:   subscriptionID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/subscriptionImages", body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// UpdateSubscriptionImage updates a subscription image.
func (c *Client) UpdateSubscriptionImage(ctx context.Context, imageID string, attrs SubscriptionImageUpdateAttributes) (*SubscriptionImageResponse, error) {
	payload := SubscriptionImageUpdateRequest{
		Data: SubscriptionImageUpdateData{
			Type:       ResourceTypeSubscriptionImages,
			ID:         strings.TrimSpace(imageID),
			Attributes: attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/subscriptionImages/%s", strings.TrimSpace(imageID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// DeleteSubscriptionImage deletes a subscription image.
func (c *Client) DeleteSubscriptionImage(ctx context.Context, imageID string) error {
	path := fmt.Sprintf("/v1/subscriptionImages/%s", strings.TrimSpace(imageID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
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

// GetSubscriptionIntroductoryOffer retrieves an introductory offer by ID.
func (c *Client) GetSubscriptionIntroductoryOffer(ctx context.Context, offerID string) (*SubscriptionIntroductoryOfferResponse, error) {
	path := fmt.Sprintf("/v1/subscriptionIntroductoryOffers/%s", strings.TrimSpace(offerID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionIntroductoryOfferResponse
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/subscriptionIntroductoryOffers", body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionIntroductoryOfferResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/subscriptionIntroductoryOffers/%s", strings.TrimSpace(offerID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionIntroductoryOfferResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// DeleteSubscriptionIntroductoryOffer deletes an introductory offer.
func (c *Client) DeleteSubscriptionIntroductoryOffer(ctx context.Context, offerID string) error {
	path := fmt.Sprintf("/v1/subscriptionIntroductoryOffers/%s", strings.TrimSpace(offerID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
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
func (c *Client) GetSubscriptionPromotionalOffer(ctx context.Context, offerID string) (*SubscriptionPromotionalOfferResponse, error) {
	path := fmt.Sprintf("/v1/subscriptionPromotionalOffers/%s", strings.TrimSpace(offerID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionPromotionalOfferResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// CreateSubscriptionPromotionalOffer creates a promotional offer.
func (c *Client) CreateSubscriptionPromotionalOffer(ctx context.Context, subscriptionID string, attrs SubscriptionPromotionalOfferCreateAttributes, priceIDs []string) (*SubscriptionPromotionalOfferResponse, error) {
	subscriptionID = strings.TrimSpace(subscriptionID)
	priceIDs = normalizeList(priceIDs)
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}
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
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/subscriptionPromotionalOffers", body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionPromotionalOfferResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/subscriptionPromotionalOffers/%s", strings.TrimSpace(offerID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionPromotionalOfferResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// DeleteSubscriptionPromotionalOffer deletes a promotional offer.
func (c *Client) DeleteSubscriptionPromotionalOffer(ctx context.Context, offerID string) error {
	path := fmt.Sprintf("/v1/subscriptionPromotionalOffers/%s", strings.TrimSpace(offerID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
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
func (c *Client) GetSubscriptionOfferCode(ctx context.Context, offerCodeID string) (*SubscriptionOfferCodeResponse, error) {
	path := fmt.Sprintf("/v1/subscriptionOfferCodes/%s", strings.TrimSpace(offerCodeID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionOfferCodeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// CreateSubscriptionOfferCode creates a subscription offer code.
func (c *Client) CreateSubscriptionOfferCode(ctx context.Context, subscriptionID string, attrs SubscriptionOfferCodeCreateAttributes, priceIDs []string) (*SubscriptionOfferCodeResponse, error) {
	subscriptionID = strings.TrimSpace(subscriptionID)
	priceIDs = normalizeList(priceIDs)
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}
	if len(priceIDs) == 0 {
		return nil, fmt.Errorf("price IDs are required")
	}

	priceData := make([]ResourceData, 0, len(priceIDs))
	for _, priceID := range priceIDs {
		priceData = append(priceData, ResourceData{
			Type: ResourceTypeSubscriptionOfferCodePrices,
			ID:   priceID,
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
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/subscriptionOfferCodes", body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionOfferCodeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/subscriptionOfferCodes/%s", strings.TrimSpace(offerCodeID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionOfferCodeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionOfferCodeCustomCodes retrieves custom codes for an offer code.
func (c *Client) GetSubscriptionOfferCodeCustomCodes(ctx context.Context, offerCodeID string, opts ...SubscriptionOfferCodeCustomCodesOption) (*SubscriptionOfferCodeCustomCodesResponse, error) {
	query := &subscriptionOfferCodeCustomCodesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/subscriptionOfferCodes/%s/customCodes", strings.TrimSpace(offerCodeID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("subscriptionOfferCodeCustomCodes: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildSubscriptionOfferCodeCustomCodesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionOfferCodeCustomCodesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
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
		opt(query)
	}

	path := fmt.Sprintf("/v1/subscriptions/%s/pricePoints", strings.TrimSpace(subscriptionID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("subscriptionPricePoints: %w", err)
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

// GetSubscriptionPricePoint retrieves a subscription price point by ID.
func (c *Client) GetSubscriptionPricePoint(ctx context.Context, pricePointID string) (*SubscriptionPricePointResponse, error) {
	path := fmt.Sprintf("/v1/subscriptionPricePoints/%s", strings.TrimSpace(pricePointID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionPricePointResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionPricePointEqualizations retrieves equalizations for a price point.
func (c *Client) GetSubscriptionPricePointEqualizations(ctx context.Context, pricePointID string) (*SubscriptionPricePointsResponse, error) {
	path := fmt.Sprintf("/v1/subscriptionPricePoints/%s/equalizations", strings.TrimSpace(pricePointID))
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

// CreateSubscriptionSubmission creates a subscription submission.
func (c *Client) CreateSubscriptionSubmission(ctx context.Context, subscriptionID string) (*SubscriptionSubmissionResponse, error) {
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}

	payload := SubscriptionSubmissionCreateRequest{
		Data: SubscriptionSubmissionCreateData{
			Type: ResourceTypeSubscriptionSubmissions,
			Relationships: &SubscriptionSubmissionRelationships{
				Subscription: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeSubscriptions,
						ID:   subscriptionID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/subscriptionSubmissions", body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionSubmissionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// CreateSubscriptionGroupSubmission creates a subscription group submission.
func (c *Client) CreateSubscriptionGroupSubmission(ctx context.Context, groupID string) (*SubscriptionGroupSubmissionResponse, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, fmt.Errorf("group ID is required")
	}

	payload := SubscriptionGroupSubmissionCreateRequest{
		Data: SubscriptionGroupSubmissionCreateData{
			Type: ResourceTypeSubscriptionGroupSubmissions,
			Relationships: &SubscriptionGroupSubmissionRelationships{
				SubscriptionGroup: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeSubscriptionGroups,
						ID:   groupID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/subscriptionGroupSubmissions", body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionGroupSubmissionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionAppStoreReviewScreenshot retrieves a review screenshot by ID.
func (c *Client) GetSubscriptionAppStoreReviewScreenshot(ctx context.Context, screenshotID string) (*SubscriptionAppStoreReviewScreenshotResponse, error) {
	path := fmt.Sprintf("/v1/subscriptionAppStoreReviewScreenshots/%s", strings.TrimSpace(screenshotID))
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/subscriptionAppStoreReviewScreenshots", body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionAppStoreReviewScreenshotResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/subscriptionAppStoreReviewScreenshots/%s", strings.TrimSpace(screenshotID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionAppStoreReviewScreenshotResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// DeleteSubscriptionAppStoreReviewScreenshot deletes a review screenshot.
func (c *Client) DeleteSubscriptionAppStoreReviewScreenshot(ctx context.Context, screenshotID string) error {
	path := fmt.Sprintf("/v1/subscriptionAppStoreReviewScreenshots/%s", strings.TrimSpace(screenshotID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetSubscriptionGroupLocalizations retrieves subscription group localizations for a group.
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

// GetSubscriptionGroupLocalization retrieves a subscription group localization by ID.
func (c *Client) GetSubscriptionGroupLocalization(ctx context.Context, localizationID string) (*SubscriptionGroupLocalizationResponse, error) {
	path := fmt.Sprintf("/v1/subscriptionGroupLocalizations/%s", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionGroupLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// CreateSubscriptionGroupLocalization creates a subscription group localization.
func (c *Client) CreateSubscriptionGroupLocalization(ctx context.Context, groupID string, attrs SubscriptionGroupLocalizationCreateAttributes) (*SubscriptionGroupLocalizationResponse, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, fmt.Errorf("group ID is required")
	}

	payload := SubscriptionGroupLocalizationCreateRequest{
		Data: SubscriptionGroupLocalizationCreateData{
			Type:       ResourceTypeSubscriptionGroupLocalizations,
			Attributes: attrs,
			Relationships: &SubscriptionGroupLocalizationRelationships{
				SubscriptionGroup: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeSubscriptionGroups,
						ID:   groupID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/subscriptionGroupLocalizations", body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionGroupLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// UpdateSubscriptionGroupLocalization updates a subscription group localization.
func (c *Client) UpdateSubscriptionGroupLocalization(ctx context.Context, localizationID string, attrs SubscriptionGroupLocalizationUpdateAttributes) (*SubscriptionGroupLocalizationResponse, error) {
	payload := SubscriptionGroupLocalizationUpdateRequest{
		Data: SubscriptionGroupLocalizationUpdateData{
			Type:       ResourceTypeSubscriptionGroupLocalizations,
			ID:         strings.TrimSpace(localizationID),
			Attributes: attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/subscriptionGroupLocalizations/%s", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionGroupLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// DeleteSubscriptionGroupLocalization deletes a subscription group localization.
func (c *Client) DeleteSubscriptionGroupLocalization(ctx context.Context, localizationID string) error {
	path := fmt.Sprintf("/v1/subscriptionGroupLocalizations/%s", strings.TrimSpace(localizationID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}
