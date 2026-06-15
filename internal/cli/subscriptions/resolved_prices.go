package subscriptions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type resolvedSubscriptionPriceCandidate struct {
	row       shared.ResolvedPriceRow
	startAt   *time.Time
	preserved bool
}

func fetchResolvedSubscriptionPrices(
	ctx context.Context,
	client *asc.Client,
	subscriptionID string,
	limit int,
	nextURL string,
	now time.Time,
	planType asc.SubscriptionPlanType,
) (*shared.ResolvedPricesResult, error) {
	if limit <= 0 {
		limit = 200
	}

	opts := []asc.SubscriptionPricesOption{
		asc.WithSubscriptionPricesLimit(limit),
		asc.WithSubscriptionPricesNextURL(nextURL),
		asc.WithSubscriptionPricesInclude([]string{"subscriptionPricePoint", "territory"}),
		asc.WithSubscriptionPricesPricePointFields([]string{"customerPrice", "proceeds", "proceedsYear2"}),
		asc.WithSubscriptionPricesTerritoryFields([]string{"currency"}),
	}
	if planType != "" {
		opts = append(opts, asc.WithSubscriptionPricesPlanType(planType))
	}

	firstPage, err := client.GetSubscriptionPrices(ctx, subscriptionID, opts...)
	if err != nil {
		return nil, err
	}

	candidates := make(map[string]resolvedSubscriptionPriceCandidate)
	if err := asc.PaginateEach(ctx, firstPage, func(ctx context.Context, next string) (asc.PaginatedResponse, error) {
		nextURL, err := mergeSubscriptionPricesNextQuery(next, resolvedSubscriptionPricesQuery(limit, planType))
		if err != nil {
			return nil, err
		}
		return client.GetSubscriptionPrices(
			ctx,
			subscriptionID,
			asc.WithSubscriptionPricesNextURL(nextURL),
			asc.WithSubscriptionPricesInclude([]string{"subscriptionPricePoint", "territory"}),
			asc.WithSubscriptionPricesPricePointFields([]string{"customerPrice", "proceeds", "proceedsYear2"}),
			asc.WithSubscriptionPricesTerritoryFields([]string{"currency"}),
			asc.WithSubscriptionPricesPlanType(planType),
		)
	}, func(page asc.PaginatedResponse) error {
		resp, ok := page.(*asc.SubscriptionPricesResponse)
		if !ok {
			return fmt.Errorf("unexpected subscription prices response type %T", page)
		}
		return consumeResolvedSubscriptionPricePageForPlanType(candidates, resp, now, planType)
	}); err != nil {
		return nil, err
	}

	rows := make([]shared.ResolvedPriceRow, 0, len(candidates))
	for _, candidate := range candidates {
		rows = append(rows, candidate.row)
	}
	shared.SortResolvedPrices(rows)
	return &shared.ResolvedPricesResult{Prices: rows}, nil
}

func resolvedSubscriptionPricesQuery(limit int, planType asc.SubscriptionPlanType) url.Values {
	values := url.Values{}
	values.Set("include", "subscriptionPricePoint,territory")
	values.Set("fields[subscriptionPricePoints]", "customerPrice,proceeds,proceedsYear2")
	values.Set("fields[territories]", "currency")
	if limit > 0 {
		values.Set("limit", fmt.Sprintf("%d", limit))
	}
	if planType != "" {
		values.Set("filter[planType]", string(planType))
	}
	return values
}

func consumeResolvedSubscriptionPricePage(
	candidates map[string]resolvedSubscriptionPriceCandidate,
	page *asc.SubscriptionPricesResponse,
	now time.Time,
) error {
	return consumeResolvedSubscriptionPricePageForPlanType(candidates, page, now, "")
}

func consumeResolvedSubscriptionPricePageForPlanType(
	candidates map[string]resolvedSubscriptionPriceCandidate,
	page *asc.SubscriptionPricesResponse,
	now time.Time,
	planType asc.SubscriptionPlanType,
) error {
	if page == nil {
		return nil
	}

	values, currencies := parseSubscriptionPricesIncluded(page.Included)
	asOf := dateOnlyUTC(now)

	for _, price := range page.Data {
		territoryID := extractSubscriptionPriceRelationshipID(price, "territory")
		if territoryID == "" {
			continue
		}

		pricePointID := extractSubscriptionPriceRelationshipID(price, "subscriptionPricePoint")
		if pricePointID == "" {
			continue
		}

		value, ok := values[pricePointID]
		if !ok {
			continue
		}

		startAt := parseSubscriptionPricingDate(price.Attributes.StartDate)
		// Preserve the legacy undated-price skip unless a plan-specific view was requested.
		if startAt == nil {
			if planType == "" {
				continue
			}
		}
		if startAt != nil && startAt.After(asOf) {
			continue
		}

		territoryID = strings.ToUpper(strings.TrimSpace(territoryID))
		currency := currencies[territoryID]
		if currency == "" {
			currency = territoryToCurrency(territoryID)
		}

		candidate := resolvedSubscriptionPriceCandidate{
			row: shared.ResolvedPriceRow{
				Territory:     territoryID,
				PriceID:       strings.TrimSpace(price.ID),
				PricePointID:  strings.TrimSpace(pricePointID),
				CustomerPrice: value.CustomerPrice,
				Currency:      currency,
				Proceeds:      value.Proceeds,
				ProceedsYear2: value.ProceedsYear2,
				StartDate:     strings.TrimSpace(price.Attributes.StartDate),
				Preserved:     boolPtr(price.Attributes.Preserved),
			},
			startAt:   startAt,
			preserved: price.Attributes.Preserved,
		}

		existing, ok := candidates[territoryID]
		if !ok || subscriptionResolvedCandidateIsNewer(candidate, existing) {
			candidates[territoryID] = candidate
		}
	}

	return nil
}

func subscriptionResolvedCandidateIsNewer(candidate, existing resolvedSubscriptionPriceCandidate) bool {
	if candidate.startAt == nil || existing.startAt == nil {
		if candidate.startAt != nil {
			return true
		}
		if existing.startAt != nil {
			return false
		}
	} else if candidate.startAt.After(*existing.startAt) {
		return true
	} else if candidate.startAt.Before(*existing.startAt) {
		return false
	}
	if candidate.preserved != existing.preserved {
		return !candidate.preserved && existing.preserved
	}
	return candidate.row.PriceID < existing.row.PriceID
}

func extractSubscriptionPriceRelationshipID(price asc.Resource[asc.SubscriptionPriceAttributes], key string) string {
	if len(price.Relationships) == 0 {
		return ""
	}

	var rels map[string]json.RawMessage
	if err := json.Unmarshal(price.Relationships, &rels); err != nil {
		return ""
	}

	rawRelationship, ok := rels[key]
	if !ok {
		return ""
	}

	var relationship struct {
		Data asc.ResourceData `json:"data"`
	}
	if err := json.Unmarshal(rawRelationship, &relationship); err != nil {
		return ""
	}

	return strings.TrimSpace(relationship.Data.ID)
}

func boolPtr(value bool) *bool {
	v := value
	return &v
}
