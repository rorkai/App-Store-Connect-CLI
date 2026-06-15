package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

const subscriptionEqualizationFailedCode = "STATE_ERROR.EQUALIZATION_FAILED"

// SubscriptionAdjustedEqualization is one territory-specific price point.
type SubscriptionAdjustedEqualization struct {
	ID            string `json:"id"`
	Territory     string `json:"territory,omitempty"`
	CustomerPrice string `json:"customerPrice,omitempty"`
	Currency      string `json:"currency,omitempty"`
}

// SubscriptionAdjustedEqualizationsResult is a sanitized adjusted-equalizations response.
type SubscriptionAdjustedEqualizationsResult struct {
	PricePointID          string                             `json:"pricePointId"`
	PlanType              string                             `json:"planType"`
	Status                int                                `json:"status"`
	Available             bool                               `json:"available"`
	Code                  string                             `json:"code,omitempty"`
	Detail                string                             `json:"detail,omitempty"`
	MissingTerritoryCount int                                `json:"missingTerritoryCount,omitempty"`
	MissingTerritories    []string                           `json:"missingTerritories,omitempty"`
	Equalizations         []SubscriptionAdjustedEqualization `json:"equalizations,omitempty"`
}

type adjustedEqualizationErrorPayload struct {
	Errors []struct {
		Code   string `json:"code"`
		Detail string `json:"detail"`
		Meta   struct {
			AssociatedErrors map[string][]struct {
				Code   string `json:"code"`
				Detail string `json:"detail"`
			} `json:"associatedErrors"`
		} `json:"meta"`
	} `json:"errors"`
}

// GetSubscriptionAdjustedEqualizations retrieves Apple's private adjusted price matrix.
func (c *Client) GetSubscriptionAdjustedEqualizations(ctx context.Context, pricePointID, planType string) (*SubscriptionAdjustedEqualizationsResult, error) {
	pricePointID = strings.TrimSpace(pricePointID)
	if pricePointID == "" {
		return nil, fmt.Errorf("subscription price point id is required")
	}
	planType = strings.ToUpper(strings.TrimSpace(planType))
	if planType != "MONTHLY" {
		return nil, fmt.Errorf(`plan type must be "MONTHLY"`)
	}

	query := url.Values{}
	query.Set("include", "territory")
	query.Set("limit", "200")
	query.Set("filter[planType]", planType)
	path := queryPath("/subscriptionPricePoints/"+url.PathEscape(pricePointID)+"/adjustedEqualizations", query)
	responseBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict {
			return nil, err
		}
		return decodeAdjustedEqualizationConflict(pricePointID, planType, apiErr)
	}

	var payload jsonAPIListPayload
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse subscription adjusted equalizations response: %w", err)
	}
	result := &SubscriptionAdjustedEqualizationsResult{
		PricePointID: pricePointID,
		PlanType:     planType,
		Status:       http.StatusOK,
		Available:    true,
	}
	for _, resource := range payload.Data {
		territory := ""
		if refs := parseRelationshipRefs(resource.Relationships["territory"].Data); len(refs) > 0 {
			territory = strings.ToUpper(strings.TrimSpace(refs[0].ID))
		}
		result.Equalizations = append(result.Equalizations, SubscriptionAdjustedEqualization{
			ID:            strings.TrimSpace(resource.ID),
			Territory:     territory,
			CustomerPrice: stringAttr(resource.Attributes, "customerPrice"),
			Currency:      stringAttr(resource.Attributes, "currency"),
		})
	}
	return result, nil
}

func decodeAdjustedEqualizationConflict(pricePointID, planType string, apiErr *APIError) (*SubscriptionAdjustedEqualizationsResult, error) {
	var payload adjustedEqualizationErrorPayload
	if err := json.Unmarshal(apiErr.rawResponseBody(), &payload); err != nil || len(payload.Errors) == 0 {
		return nil, apiErr
	}
	primary := payload.Errors[0]
	if strings.TrimSpace(primary.Code) != subscriptionEqualizationFailedCode {
		return nil, apiErr
	}
	missingCount := 0
	territorySet := map[string]struct{}{}
	for _, associatedErrors := range primary.Meta.AssociatedErrors {
		for _, associated := range associatedErrors {
			if !strings.EqualFold(strings.TrimSpace(associated.Code), "STATE_ERROR.NO_TIER_IN_TERRITORY") {
				continue
			}
			missingCount++
			if territory := strings.ToUpper(strings.TrimSpace(associated.Detail)); territory != "" {
				territorySet[territory] = struct{}{}
			}
		}
	}
	territories := make([]string, 0, len(territorySet))
	for territory := range territorySet {
		territories = append(territories, territory)
	}
	sort.Strings(territories)
	return &SubscriptionAdjustedEqualizationsResult{
		PricePointID:          pricePointID,
		PlanType:              planType,
		Status:                apiErr.Status,
		Available:             false,
		Code:                  strings.TrimSpace(primary.Code),
		Detail:                strings.TrimSpace(primary.Detail),
		MissingTerritoryCount: missingCount,
		MissingTerritories:    territories,
	}, nil
}
