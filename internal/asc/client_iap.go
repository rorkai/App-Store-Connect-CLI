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
	return c.getInAppPurchasesV2(ctx, appID, false, opts...)
}

// GetInAppPurchasesV2Strict validates collection envelopes before using them as evidence.
func (c *Client) GetInAppPurchasesV2Strict(ctx context.Context, appID string, opts ...IAPOption) (*InAppPurchasesV2Response, error) {
	return c.getInAppPurchasesV2(ctx, appID, true, opts...)
}

func (c *Client) getInAppPurchasesV2(ctx context.Context, appID string, strict bool, opts ...IAPOption) (*InAppPurchasesV2Response, error) {
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

	if strict {
		if err := validateReviewSubmissionCollectionEnvelope(data, "in-app purchases", reviewCollectionResourceSpec{resourceType: ResourceTypeInAppPurchases}); err != nil {
			return nil, err
		}
	}
	var response InAppPurchasesV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchaseV2 retrieves an in-app purchase by ID.
func (c *Client) GetInAppPurchaseV2(ctx context.Context, iapID string, opts ...IAPGetOption) (*InAppPurchaseV2Response, error) {
	iapID = strings.TrimSpace(iapID)
	if iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}

	query := applyOptions(opts)
	return getJSON[InAppPurchaseV2Response](ctx, c, readRequest{Path: fmt.Sprintf("/v2/inAppPurchases/%s", iapID), Query: buildInAppPurchaseGetQuery(query)})
}

// GetInAppPurchase retrieves a legacy in-app purchase by ID.
func (c *Client) GetInAppPurchase(ctx context.Context, iapID string) (*InAppPurchaseResponse, error) {
	return getJSON[InAppPurchaseResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/inAppPurchases/%s", strings.TrimSpace(iapID))})
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

	return sendJSON[InAppPurchaseV2Response](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v2/inAppPurchases", Payload: payload})
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

	return sendJSON[InAppPurchaseV2Response](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v2/inAppPurchases/%s", strings.TrimSpace(iapID)), Payload: payload})
}

// DeleteInAppPurchaseV2 deletes an in-app purchase.
func (c *Client) DeleteInAppPurchaseV2(ctx context.Context, iapID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v2/inAppPurchases/%s", strings.TrimSpace(iapID)), nil)
}

// GetInAppPurchaseLocalizations fetches localizations for an IAP.
//
// Deprecated: Use GetInAppPurchaseVersionLocalizations with an in-app purchase version ID.
func (c *Client) GetInAppPurchaseLocalizations(ctx context.Context, iapID string, opts ...IAPLocalizationsOption) (*InAppPurchaseLocalizationsResponse, error) {
	query := applyOptions(opts)
	return getJSON[InAppPurchaseLocalizationsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v2/inAppPurchases/%s/inAppPurchaseLocalizations", strings.TrimSpace(iapID)),
		Query:       buildIAPLocalizationsQuery(query),
		Next:        query.nextURL,
		NextContext: "in-app-purchase-localizations",
	})
}
