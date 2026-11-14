package iap

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func readIAPPPPSchedule(ctx context.Context, client *asc.Client, iapID string) (shared.PPPScheduleSnapshot, error) {
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()
	schedule, err := client.GetInAppPurchasePriceSchedule(requestCtx, iapID)
	if err != nil {
		return shared.PPPScheduleSnapshot{}, err
	}
	base, err := client.GetInAppPurchasePriceScheduleBaseTerritory(requestCtx, schedule.Data.ID)
	if err != nil {
		return shared.PPPScheduleSnapshot{}, err
	}
	snapshot := shared.PPPScheduleSnapshot{BaseTerritory: base.Data.ID}
	for index, fetch := range []iapSchedulePriceFetcher{client.GetInAppPurchasePriceScheduleManualPrices, client.GetInAppPurchasePriceScheduleAutomaticPrices} {
		first, err := fetch(requestCtx, schedule.Data.ID,
			asc.Limit[asc.IAPPriceSchedulePricesOption](200),
			asc.WithIAPPriceSchedulePricesInclude([]string{"inAppPurchasePricePoint", "territory"}),
			asc.WithIAPPriceSchedulePricesFields([]string{"manual", "startDate", "endDate", "inAppPurchasePricePoint", "territory"}),
			asc.WithIAPPriceSchedulePricesPricePointFields([]string{"customerPrice", "proceeds", "territory"}),
			asc.WithIAPPriceSchedulePricesTerritoryFields([]string{"currency"}))
		if err != nil {
			return snapshot, err
		}
		err = asc.PaginateEach(requestCtx, first, func(ctx context.Context, next string) (asc.PaginatedResponse, error) {
			next, err := shared.MergeNextURLQuery(next, resolvedIAPSchedulePricesQuery(200))
			if err != nil {
				return nil, err
			}
			return fetch(ctx, schedule.Data.ID, asc.NextURL[asc.IAPPriceSchedulePricesOption](next))
		}, func(page asc.PaginatedResponse) error {
			typed, ok := page.(*asc.InAppPurchasePricesResponse)
			if !ok {
				return fmt.Errorf("unexpected schedule response %T", page)
			}
			_, currencies, err := parseResolvedIAPPricePageIncluded(typed.Included)
			if err != nil {
				return err
			}
			var included []struct {
				Type, ID   string
				Attributes asc.InAppPurchasePricePointAttributes
			}
			if len(typed.Included) > 0 {
				if err := json.Unmarshal(typed.Included, &included); err != nil {
					return fmt.Errorf("parse schedule included identifiers: %w", err)
				}
			}
			canonicalPoints := make(map[string]asc.InAppPurchasePricePointAttributes)
			for _, item := range included {
				if item.Type == string(asc.ResourceTypeInAppPurchasePricePoints) {
					canonicalPoints[strings.TrimSpace(item.ID)] = item.Attributes
				}
			}

			for _, item := range typed.Data {
				entry, _, _ := resolvedIAPPriceCandidateFromResource(item, nil, currencies)
				pointID, relationshipErr := relationshipID(item.Relationships, "inAppPurchasePricePoint")
				pointID = strings.TrimSpace(pointID)
				value, exists := canonicalPoints[pointID]
				value.CustomerPrice = strings.TrimSpace(value.CustomerPrice)
				value.Proceeds = strings.TrimSpace(value.Proceeds)
				if relationshipErr != nil || !exists || value.CustomerPrice == "" || entry.TerritoryID == "" {
					return fmt.Errorf("cannot resolve complete schedule price %q", item.ID)
				}
				currency := currencies[entry.TerritoryID]
				if currency == "" {
					currency = entry.TerritoryID
				}
				row := shared.ResolvedPriceRow{
					Territory: entry.TerritoryID, PriceID: strings.TrimSpace(item.ID),
					PricePointID: pointID, CustomerPrice: value.CustomerPrice,
					Currency: currency, Proceeds: value.Proceeds,
					StartDate: entry.StartDate, EndDate: entry.EndDate,
					Manual: iapResolvedBoolPtr(entry.Manual),
				}
				if *row.Manual != (index == 0) {
					return fmt.Errorf("price %q has inconsistent manual pricing mode", item.ID)
				}
				snapshot.Prices = append(snapshot.Prices, row)
			}
			return nil
		})
		if err != nil {
			return snapshot, err
		}
	}
	return snapshot, shared.ValidatePPPScheduleSnapshot(snapshot)
}
