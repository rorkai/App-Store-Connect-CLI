package iap

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

type resolvedIAPPriceCandidate struct {
	entry iapPriceEntry
	row   shared.ResolvedPriceRow
	err   error
}

type iapSchedulePriceFetcher func(context.Context, string, ...asc.IAPPriceSchedulePricesOption) (*asc.InAppPurchasePricesResponse, error)

// requireComplete rejects unresolved active rows before a full schedule replacement.
func fetchResolvedIAPSchedulePrices(
	ctx context.Context,
	client *asc.Client,
	scheduleID string,
	surface string,
	limit int,
	nextURL string,
	now time.Time,
	requireComplete bool,
) (*shared.ResolvedPricesResult, error) {
	if limit <= 0 {
		limit = 200
	}

	candidates := make(map[string]resolvedIAPPriceCandidate)
	surface = strings.TrimSpace(surface)
	includeSibling := strings.TrimSpace(nextURL) == ""

	switch surface {
	case "manual":
		if err := consumeResolvedIAPSchedulePrices(
			ctx,
			scheduleID,
			nextURL,
			limit,
			now,
			candidates,
			client.GetInAppPurchasePriceScheduleManualPrices,
			requireComplete,
		); err != nil {
			return nil, fmt.Errorf("fetch manual prices: %w", err)
		}
		if includeSibling {
			if err := consumeResolvedIAPSchedulePrices(
				ctx,
				scheduleID,
				"",
				limit,
				now,
				candidates,
				client.GetInAppPurchasePriceScheduleAutomaticPrices,
				requireComplete,
			); err != nil {
				return nil, fmt.Errorf("fetch automatic prices: %w", err)
			}
		}
	case "automatic":
		if err := consumeResolvedIAPSchedulePrices(
			ctx,
			scheduleID,
			nextURL,
			limit,
			now,
			candidates,
			client.GetInAppPurchasePriceScheduleAutomaticPrices,
			requireComplete,
		); err != nil {
			return nil, fmt.Errorf("fetch automatic prices: %w", err)
		}
		if includeSibling {
			if err := consumeResolvedIAPSchedulePrices(
				ctx,
				scheduleID,
				"",
				limit,
				now,
				candidates,
				client.GetInAppPurchasePriceScheduleManualPrices,
				requireComplete,
			); err != nil {
				return nil, fmt.Errorf("fetch manual prices: %w", err)
			}
		}
	default:
		return nil, fmt.Errorf("unknown IAP price surface %q", surface)
	}

	rows := make([]shared.ResolvedPriceRow, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.err != nil {
			return nil, candidate.err
		}
		rows = append(rows, candidate.row)
	}
	shared.SortResolvedPrices(rows)
	return &shared.ResolvedPricesResult{Prices: rows}, nil
}

func consumeResolvedIAPSchedulePrices(
	ctx context.Context,
	scheduleID string,
	nextURL string,
	limit int,
	now time.Time,
	candidates map[string]resolvedIAPPriceCandidate,
	fetch iapSchedulePriceFetcher,
	requireComplete bool,
) error {
	opts := []asc.IAPPriceSchedulePricesOption{
		asc.Limit[asc.IAPPriceSchedulePricesOption](limit),
		asc.NextURL[asc.IAPPriceSchedulePricesOption](nextURL),
		asc.WithIAPPriceSchedulePricesInclude([]string{"inAppPurchasePricePoint", "territory"}),
		asc.WithIAPPriceSchedulePricesFields([]string{"manual", "startDate", "endDate", "inAppPurchasePricePoint", "territory"}),
		asc.WithIAPPriceSchedulePricesPricePointFields([]string{"customerPrice", "proceeds", "territory"}),
		asc.WithIAPPriceSchedulePricesTerritoryFields([]string{"currency"}),
	}

	firstPage, err := fetch(ctx, scheduleID, opts...)
	if err != nil {
		return err
	}

	return asc.PaginateEach(ctx, firstPage, func(ctx context.Context, next string) (asc.PaginatedResponse, error) {
		nextURL, err := shared.MergeNextURLQuery(next, resolvedIAPSchedulePricesQuery(limit))
		if err != nil {
			return nil, err
		}
		return fetch(ctx, scheduleID, asc.NextURL[asc.IAPPriceSchedulePricesOption](nextURL))
	}, func(page asc.PaginatedResponse) error {
		resp, ok := page.(*asc.InAppPurchasePricesResponse)
		if !ok {
			return fmt.Errorf("unexpected in-app purchase prices response type %T", page)
		}
		return consumeResolvedIAPPricePage(candidates, resp, now, requireComplete)
	})
}

func resolvedIAPSchedulePricesQuery(limit int) url.Values {
	values := url.Values{}
	values.Set("include", "inAppPurchasePricePoint,territory")
	values.Set("fields[inAppPurchasePrices]", "manual,startDate,endDate,inAppPurchasePricePoint,territory")
	values.Set("fields[inAppPurchasePricePoints]", "customerPrice,proceeds,territory")
	values.Set("fields[territories]", "currency")
	if limit > 0 {
		values.Set("limit", fmt.Sprintf("%d", limit))
	}
	return values
}

func consumeResolvedIAPPricePage(
	candidates map[string]resolvedIAPPriceCandidate,
	page *asc.InAppPurchasePricesResponse,
	now time.Time,
	requireComplete bool,
) error {
	if page == nil {
		return nil
	}

	values, currencies, err := parseResolvedIAPPricePageIncluded(page.Included)
	if err != nil {
		return err
	}

	asOf := shared.PricingDate(now)
	for _, item := range page.Data {
		entry, row, ok := resolvedIAPPriceCandidateFromResource(item, values, currencies)
		if !entryActiveOn(entry, asOf) {
			continue
		}
		candidate := resolvedIAPPriceCandidate{entry: entry, row: row}
		if !ok {
			if !requireComplete {
				continue
			}
			candidate.err = fmt.Errorf("cannot resolve active price %q (territory %q, price point %q)", item.ID, entry.TerritoryID, entry.PricePointID)
			if entry.TerritoryID == "" || entry.PricePointID == "" {
				return candidate.err
			}
		}

		existing, exists := candidates[entry.TerritoryID]
		if !exists || iapPriceEntryIsNewer(entry, existing.entry) {
			candidates[entry.TerritoryID] = candidate
		}
	}

	return nil
}

func parseResolvedIAPPricePageIncluded(raw json.RawMessage) (map[string]iapPricePointValue, map[string]string, error) {
	values := make(map[string]iapPricePointValue)
	currencies := make(map[string]string)
	if len(raw) == 0 {
		return values, currencies, nil
	}

	var included []struct {
		Type          string          `json:"type"`
		ID            string          `json:"id"`
		Attributes    json.RawMessage `json:"attributes"`
		Relationships json.RawMessage `json:"relationships"`
	}
	if err := json.Unmarshal(raw, &included); err != nil {
		return nil, nil, fmt.Errorf("parse schedule included resources: %w", err)
	}

	for _, item := range included {
		switch item.Type {
		case string(asc.ResourceTypeTerritories):
			var attrs struct {
				Currency string `json:"currency"`
			}
			if err := json.Unmarshal(item.Attributes, &attrs); err != nil {
				continue
			}
			if currency := strings.TrimSpace(attrs.Currency); currency != "" {
				currencies[strings.ToUpper(strings.TrimSpace(item.ID))] = currency
			}
		case string(asc.ResourceTypeInAppPurchasePricePoints):
			var attrs struct {
				CustomerPrice string `json:"customerPrice"`
				Proceeds      string `json:"proceeds"`
			}
			if err := json.Unmarshal(item.Attributes, &attrs); err != nil {
				continue
			}

			value := iapPricePointValue{
				CustomerPrice: strings.TrimSpace(attrs.CustomerPrice),
				Proceeds:      strings.TrimSpace(attrs.Proceeds),
			}
			if value.CustomerPrice == "" && value.Proceeds == "" {
				continue
			}

			values[strings.TrimSpace(item.ID)] = value
			_, decodedPricePointID, ok := decodeIAPPriceResourceID(item.ID)
			if ok && strings.TrimSpace(decodedPricePointID) != "" {
				values[strings.TrimSpace(decodedPricePointID)] = value
			}
		}
	}

	return values, currencies, nil
}

func resolvedIAPPriceCandidateFromResource(
	item asc.Resource[asc.InAppPurchasePriceAttributes],
	values map[string]iapPricePointValue,
	currencies map[string]string,
) (iapPriceEntry, shared.ResolvedPriceRow, bool) {
	decodedMeta, decodedOK := decodeIAPPriceResourceMetadata(item.ID)

	territoryID, err := relationshipID(item.Relationships, "territory")
	if err != nil || strings.TrimSpace(territoryID) == "" {
		territoryID = decodedMeta.TerritoryID
	}
	pricePointID, err := relationshipID(item.Relationships, "inAppPurchasePricePoint")
	if err != nil || strings.TrimSpace(pricePointID) == "" {
		pricePointID = decodedMeta.PricePointID
	} else if strings.TrimSpace(decodedMeta.PricePointID) != "" {
		pricePointID = decodedMeta.PricePointID
	}

	territoryID = strings.ToUpper(strings.TrimSpace(territoryID))
	pricePointID = strings.TrimSpace(pricePointID)
	startDate := strings.TrimSpace(item.Attributes.StartDate)
	endDate := strings.TrimSpace(item.Attributes.EndDate)
	if decodedOK {
		if startDate == "" {
			startDate = decodedMeta.StartDate
		}
		if endDate == "" {
			endDate = decodedMeta.EndDate
		}
	}

	entry := newIAPPriceEntry(territoryID, pricePointID, startDate, endDate, item.Attributes.Manual)
	if territoryID == "" || pricePointID == "" {
		return entry, shared.ResolvedPriceRow{}, false
	}

	value, ok := values[pricePointID]
	if !ok {
		return entry, shared.ResolvedPriceRow{}, false
	}

	currency := currencies[territoryID]
	if currency == "" {
		currency = territoryID
	}

	return entry, shared.ResolvedPriceRow{
		Territory:     territoryID,
		PriceID:       strings.TrimSpace(item.ID),
		PricePointID:  pricePointID,
		CustomerPrice: value.CustomerPrice,
		Currency:      currency,
		Proceeds:      value.Proceeds,
		StartDate:     entry.StartDate,
		EndDate:       entry.EndDate,
		Manual:        iapResolvedBoolPtr(item.Attributes.Manual),
	}, true
}

func iapResolvedBoolPtr(value bool) *bool {
	v := value
	return &v
}
