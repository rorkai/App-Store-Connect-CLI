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
