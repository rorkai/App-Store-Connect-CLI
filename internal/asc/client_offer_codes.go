package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GetSubscriptionOfferCode retrieves a subscription offer code by ID.
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
func (c *Client) CreateSubscriptionOfferCode(ctx context.Context, req SubscriptionOfferCodeCreateRequest) (*SubscriptionOfferCodeResponse, error) {
	body, err := BuildRequestBody(req)
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

// UpdateSubscriptionOfferCode updates a subscription offer code.
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
			return nil, fmt.Errorf("offerCode customCodes: %w", err)
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

// GetSubscriptionOfferCodeCustomCode retrieves a custom code by ID.
func (c *Client) GetSubscriptionOfferCodeCustomCode(ctx context.Context, customCodeID string) (*SubscriptionOfferCodeCustomCodeResponse, error) {
	path := fmt.Sprintf("/v1/subscriptionOfferCodeCustomCodes/%s", strings.TrimSpace(customCodeID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionOfferCodeCustomCodeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateSubscriptionOfferCodeCustomCode creates a custom code.
func (c *Client) CreateSubscriptionOfferCodeCustomCode(ctx context.Context, req SubscriptionOfferCodeCustomCodeCreateRequest) (*SubscriptionOfferCodeCustomCodeResponse, error) {
	body, err := BuildRequestBody(req)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/subscriptionOfferCodeCustomCodes", body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionOfferCodeCustomCodeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateSubscriptionOfferCodeCustomCode updates a custom code.
func (c *Client) UpdateSubscriptionOfferCodeCustomCode(ctx context.Context, customCodeID string, attrs SubscriptionOfferCodeCustomCodeUpdateAttributes) (*SubscriptionOfferCodeCustomCodeResponse, error) {
	payload := SubscriptionOfferCodeCustomCodeUpdateRequest{
		Data: SubscriptionOfferCodeCustomCodeUpdateData{
			Type:       ResourceTypeSubscriptionOfferCodeCustomCodes,
			ID:         strings.TrimSpace(customCodeID),
			Attributes: attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/subscriptionOfferCodeCustomCodes/%s", strings.TrimSpace(customCodeID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionOfferCodeCustomCodeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateSubscriptionOfferCodeOneTimeUseCode updates a one-time use code batch.
func (c *Client) UpdateSubscriptionOfferCodeOneTimeUseCode(ctx context.Context, oneTimeUseCodeID string, attrs SubscriptionOfferCodeOneTimeUseCodeUpdateAttributes) (*SubscriptionOfferCodeOneTimeUseCodeResponse, error) {
	payload := SubscriptionOfferCodeOneTimeUseCodeUpdateRequest{
		Data: SubscriptionOfferCodeOneTimeUseCodeUpdateData{
			Type:       ResourceTypeSubscriptionOfferCodeOneTimeUseCodes,
			ID:         strings.TrimSpace(oneTimeUseCodeID),
			Attributes: attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/subscriptionOfferCodeOneTimeUseCodes/%s", strings.TrimSpace(oneTimeUseCodeID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionOfferCodeOneTimeUseCodeResponse
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
			return nil, fmt.Errorf("offerCode prices: %w", err)
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
