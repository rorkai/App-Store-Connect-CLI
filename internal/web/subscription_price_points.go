package web

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
)

// SubscriptionPricePoint is a web subscription price point.
type SubscriptionPricePoint struct {
	ID            string `json:"id"`
	Territory     string `json:"territory"`
	CustomerPrice string `json:"customerPrice"`
	Currency      string `json:"currency,omitempty"`
}

// ListSubscriptionPricePoints lists price points for one subscription territory.
func (c *Client) ListSubscriptionPricePoints(ctx context.Context, subscriptionID, territory string) ([]SubscriptionPricePoint, error) {
	subscriptionID = strings.TrimSpace(subscriptionID)
	territory = strings.ToUpper(strings.TrimSpace(territory))
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription id is required")
	}
	if len(territory) != 3 {
		return nil, fmt.Errorf("territory must be a three-letter territory id")
	}
	query := url.Values{}
	query.Set("filter[territory]", territory)
	query.Set("include", "territory")
	query.Set("limit", "1000")
	path := queryPath("/subscriptions/"+url.PathEscape(subscriptionID)+"/pricePoints", query)
	responseBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var payload jsonAPIListPayload
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse subscription price points response: %w", err)
	}
	result := make([]SubscriptionPricePoint, 0, len(payload.Data))
	for _, resource := range payload.Data {
		pointTerritory := territory
		if refs := parseRelationshipRefs(resource.Relationships["territory"].Data); len(refs) > 0 {
			pointTerritory = strings.ToUpper(strings.TrimSpace(refs[0].ID))
		}
		result = append(result, SubscriptionPricePoint{
			ID:            strings.TrimSpace(resource.ID),
			Territory:     pointTerritory,
			CustomerPrice: stringAttr(resource.Attributes, "customerPrice"),
			Currency:      stringAttr(resource.Attributes, "currency"),
		})
	}
	return result, nil
}

// ResolveSubscriptionPricePoint resolves an exact decimal customer price.
func (c *Client) ResolveSubscriptionPricePoint(ctx context.Context, subscriptionID, territory, customerPrice string) (*SubscriptionPricePoint, error) {
	requested, ok := new(big.Rat).SetString(strings.TrimSpace(customerPrice))
	if !ok || requested.Sign() <= 0 {
		return nil, fmt.Errorf("customer price must be a positive decimal")
	}
	points, err := c.ListSubscriptionPricePoints(ctx, subscriptionID, territory)
	if err != nil {
		return nil, err
	}
	for i := range points {
		candidate, ok := new(big.Rat).SetString(strings.TrimSpace(points[i].CustomerPrice))
		if ok && requested.Cmp(candidate) == 0 {
			return &points[i], nil
		}
	}
	return nil, fmt.Errorf("no subscription price point matches customer price %s in %s", strings.TrimSpace(customerPrice), strings.ToUpper(strings.TrimSpace(territory)))
}
