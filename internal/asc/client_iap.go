package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GetInAppPurchasesV2 retrieves the list of in-app purchases for an app.
func (c *Client) GetInAppPurchasesV2(ctx context.Context, appID string, opts ...IAPOption) (*InAppPurchasesV2Response, error) {
	query := &inAppPurchasesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/apps/%s/inAppPurchasesV2", strings.TrimSpace(appID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("in-app-purchases: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildInAppPurchasesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchasesV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchases retrieves the legacy list of in-app purchases for an app.
func (c *Client) GetInAppPurchases(ctx context.Context, appID string, opts ...IAPOption) (*InAppPurchasesResponse, error) {
	query := &inAppPurchasesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/apps/%s/inAppPurchases", strings.TrimSpace(appID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("in-app-purchases: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildInAppPurchasesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchasesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchaseV2 retrieves an in-app purchase by ID.
func (c *Client) GetInAppPurchaseV2(ctx context.Context, iapID string) (*InAppPurchaseV2Response, error) {
	path := fmt.Sprintf("/v2/inAppPurchases/%s", strings.TrimSpace(iapID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchase retrieves a legacy in-app purchase by ID.
func (c *Client) GetInAppPurchase(ctx context.Context, iapID string) (*InAppPurchaseResponse, error) {
	path := fmt.Sprintf("/v1/inAppPurchases/%s", strings.TrimSpace(iapID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateInAppPurchaseV2 creates a new in-app purchase.
func (c *Client) CreateInAppPurchaseV2(ctx context.Context, appID string, attrs InAppPurchaseV2CreateAttributes) (*InAppPurchaseV2Response, error) {
	payload := InAppPurchaseV2CreateRequest{
		Data: InAppPurchaseV2CreateData{
			Type:       ResourceTypeInAppPurchases,
			Attributes: attrs,
			Relationships: &InAppPurchaseV2Relationships{
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

	data, err := c.do(ctx, http.MethodPost, "/v2/inAppPurchases", body)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateInAppPurchaseV2 updates an existing in-app purchase.
func (c *Client) UpdateInAppPurchaseV2(ctx context.Context, iapID string, attrs InAppPurchaseV2UpdateAttributes) (*InAppPurchaseV2Response, error) {
	payload := InAppPurchaseV2UpdateRequest{
		Data: InAppPurchaseV2UpdateData{
			Type:       ResourceTypeInAppPurchases,
			ID:         strings.TrimSpace(iapID),
			Attributes: &attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v2/inAppPurchases/%s", strings.TrimSpace(iapID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteInAppPurchaseV2 deletes an in-app purchase.
func (c *Client) DeleteInAppPurchaseV2(ctx context.Context, iapID string) error {
	path := fmt.Sprintf("/v2/inAppPurchases/%s", strings.TrimSpace(iapID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetInAppPurchaseLocalizations fetches localizations for an IAP.
func (c *Client) GetInAppPurchaseLocalizations(ctx context.Context, iapID string, opts ...IAPLocalizationsOption) (*InAppPurchaseLocalizationsResponse, error) {
	query := &iapLocalizationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v2/inAppPurchases/%s/inAppPurchaseLocalizations", strings.TrimSpace(iapID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("in-app-purchase-localizations: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildIAPLocalizationsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseLocalizationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
