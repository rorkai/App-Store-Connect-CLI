package validate

import (
	"context"
	"encoding/json"
	"math/big"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/pricing"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type currentAppPriceCandidate struct {
	startDate string
	paid      bool
	known     bool
}

func fetchCurrentAppPaidPricingEvidence(ctx context.Context, client *asc.Client, scheduleID string) (bool, bool) {
	const limit = 200
	query := currentAppPricingQuery(limit)
	fetchPage := func(nextURL string) (*asc.AppPricesResponse, error) {
		opts := []asc.AppPriceSchedulePricesOption{
			asc.WithAppPriceSchedulePricesInclude([]string{"appPricePoint"}),
			asc.WithAppPriceSchedulePricesFields([]string{"manual", "startDate", "endDate", "appPricePoint"}),
			asc.WithAppPriceSchedulePricesPricePointFields([]string{"customerPrice"}),
			asc.Limit[asc.AppPriceSchedulePricesOption](limit),
		}
		if strings.TrimSpace(nextURL) != "" {
			merged, err := shared.MergeNextURLQuery(nextURL, query)
			if err != nil {
				return nil, err
			}
			opts = []asc.AppPriceSchedulePricesOption{asc.NextURL[asc.AppPriceSchedulePricesOption](merged)}
		}
		return doReadinessRequest(ctx, func(requestCtx context.Context) (*asc.AppPricesResponse, error) {
			return client.GetAppPriceScheduleManualPrices(requestCtx, scheduleID, opts...)
		})
	}

	firstPage, err := fetchPage("")
	if err != nil {
		return false, false
	}
	pages := make([]*asc.AppPricesResponse, 0, 1)
	if err := asc.PaginateEach(ctx, firstPage, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return fetchPage(nextURL)
	}, func(page asc.PaginatedResponse) error {
		typed, ok := page.(*asc.AppPricesResponse)
		if !ok {
			return nil
		}
		pages = append(pages, typed)
		return nil
	}); err != nil {
		return false, false
	}
	return currentAppPaidPricingEvidence(pages, shared.PricingNow())
}

func currentAppPricingQuery(limit int) url.Values {
	query := url.Values{}
	query.Set("include", "appPricePoint")
	query.Set("fields[appPrices]", "manual,startDate,endDate,appPricePoint")
	query.Set("fields[appPricePoints]", "customerPrice")
	query.Set("limit", strconv.Itoa(limit))
	return query
}

func currentAppPaidPricingEvidence(pages []*asc.AppPricesResponse, now time.Time) (bool, bool) {
	candidates := make([]currentAppPriceCandidate, 0)
	for _, page := range pages {
		if page == nil {
			continue
		}
		pricePoints := includedAppPricePoints(page.Included)
		for _, resource := range page.Data {
			active, datesKnown := pricing.AppPriceActiveOn(resource.Attributes.StartDate, resource.Attributes.EndDate, now)
			if !datesKnown {
				return false, false
			}
			if !active {
				continue
			}
			candidate := currentAppPriceCandidate{startDate: strings.TrimSpace(resource.Attributes.StartDate)}
			pricePointID := appPriceRelationshipID(resource.Relationships)
			price, ok := pricePoints[pricePointID]
			if ok {
				value, parsed := new(big.Rat).SetString(strings.TrimSpace(price))
				if parsed {
					candidate.known = true
					candidate.paid = value.Sign() > 0
				}
			}
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 0 {
		return false, false
	}

	latestStart := candidates[0].startDate
	for _, candidate := range candidates[1:] {
		if candidate.startDate > latestStart {
			latestStart = candidate.startDate
		}
	}
	wantPaid := false
	wantPaidSet := false
	for _, candidate := range candidates {
		if candidate.startDate != latestStart {
			continue
		}
		if !candidate.known {
			return false, false
		}
		if wantPaidSet && candidate.paid != wantPaid {
			return false, false
		}
		wantPaid = candidate.paid
		wantPaidSet = true
	}
	return wantPaid, wantPaidSet
}

func includedAppPricePoints(raw json.RawMessage) map[string]string {
	result := make(map[string]string)
	var included []struct {
		Type       string `json:"type"`
		ID         string `json:"id"`
		Attributes struct {
			CustomerPrice string `json:"customerPrice"`
		} `json:"attributes"`
	}
	if json.Unmarshal(raw, &included) != nil {
		return result
	}
	for _, item := range included {
		if item.Type == string(asc.ResourceTypeAppPricePoints) && strings.TrimSpace(item.ID) != "" {
			result[strings.TrimSpace(item.ID)] = strings.TrimSpace(item.Attributes.CustomerPrice)
		}
	}
	return result
}

func appPriceRelationshipID(raw json.RawMessage) string {
	var relationships struct {
		AppPricePoint struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		} `json:"appPricePoint"`
	}
	if json.Unmarshal(raw, &relationships) != nil {
		return ""
	}
	return strings.TrimSpace(relationships.AppPricePoint.Data.ID)
}
