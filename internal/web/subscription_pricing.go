package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// SubscriptionPlanPricesResult identifies the paired billing-plan prices created.
type SubscriptionPlanPricesResult struct {
	SubscriptionID      string `json:"subscriptionId"`
	UpfrontPricePointID string `json:"upfrontPricePointId"`
	MonthlyPricePointID string `json:"monthlyPricePointId"`
}

// SubscriptionPlanPrice identifies one billing plan's price and scheduling attributes.
type SubscriptionPlanPrice struct {
	PlanType             string
	PricePointID         string
	StartDate            string
	PreserveCurrentPrice bool
}

// CreateSubscriptionPlanPrices creates paired upfront and monthly prices through
// the inline subscription PATCH used by App Store Connect.
func (c *Client) CreateSubscriptionPlanPrices(ctx context.Context, subscriptionID, upfrontPricePointID, monthlyPricePointID string) (*SubscriptionPlanPricesResult, error) {
	return c.SetSubscriptionPlanPrices(ctx, subscriptionID, []SubscriptionPlanPrice{
		{PlanType: "UPFRONT", PricePointID: upfrontPricePointID},
		{PlanType: "MONTHLY", PricePointID: monthlyPricePointID},
	})
}

// SetSubscriptionPlanPrices creates or schedules paired plan prices through the inline PATCH.
func (c *Client) SetSubscriptionPlanPrices(ctx context.Context, subscriptionID string, prices []SubscriptionPlanPrice) (*SubscriptionPlanPricesResult, error) {
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription id is required")
	}
	if len(prices) != 2 {
		return nil, fmt.Errorf("exactly two subscription plan prices are required")
	}

	upfrontID := "${current-upfront}"
	monthlyID := "${current-monthly}"
	priceRef := func(id string) map[string]string {
		return map[string]string{"type": "subscriptionPrices", "id": id}
	}
	includedPrice := func(id string, price SubscriptionPlanPrice) map[string]any {
		attributes := map[string]any{"planType": price.PlanType}
		if strings.TrimSpace(price.StartDate) != "" {
			attributes["startDate"] = strings.TrimSpace(price.StartDate)
			attributes["preserveCurrentPrice"] = price.PreserveCurrentPrice
		}
		return map[string]any{
			"type":       "subscriptionPrices",
			"id":         id,
			"attributes": attributes,
			"relationships": map[string]any{
				"subscriptionPricePoint": map[string]any{
					"data": map[string]string{
						"type": "subscriptionPricePoints",
						"id":   price.PricePointID,
					},
				},
			},
		}
	}
	byType := map[string]SubscriptionPlanPrice{}
	for _, price := range prices {
		price.PlanType = strings.ToUpper(strings.TrimSpace(price.PlanType))
		price.PricePointID = strings.TrimSpace(price.PricePointID)
		if price.PlanType != "UPFRONT" && price.PlanType != "MONTHLY" {
			return nil, fmt.Errorf(`plan type must be "UPFRONT" or "MONTHLY"`)
		}
		if price.PricePointID == "" {
			return nil, fmt.Errorf("%s price point id is required", strings.ToLower(price.PlanType))
		}
		byType[price.PlanType] = price
	}
	upfront, upfrontOK := byType["UPFRONT"]
	monthly, monthlyOK := byType["MONTHLY"]
	if !upfrontOK || !monthlyOK {
		return nil, fmt.Errorf("both UPFRONT and MONTHLY prices are required")
	}
	requestBody := map[string]any{
		"data": map[string]any{
			"type": "subscriptions",
			"id":   subscriptionID,
			"relationships": map[string]any{
				"prices": map[string]any{
					"data": []map[string]string{
						priceRef(upfrontID),
						priceRef(monthlyID),
					},
				},
			},
		},
		"included": []map[string]any{
			includedPrice(upfrontID, upfront),
			includedPrice(monthlyID, monthly),
		},
	}

	responseBody, err := c.doRequest(ctx, http.MethodPatch, "/subscriptions/"+url.PathEscape(subscriptionID), requestBody)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data jsonAPIResource `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse subscription pricing response: %w", err)
	}
	if returnedID := strings.TrimSpace(payload.Data.ID); returnedID != "" && returnedID != subscriptionID {
		return nil, fmt.Errorf("apple returned subscription %q after patching %q", returnedID, subscriptionID)
	}
	return &SubscriptionPlanPricesResult{
		SubscriptionID:      subscriptionID,
		UpfrontPricePointID: upfront.PricePointID,
		MonthlyPricePointID: monthly.PricePointID,
	}, nil
}
