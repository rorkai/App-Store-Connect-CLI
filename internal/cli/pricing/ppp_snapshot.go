package pricing

import (
	"context"
	"fmt"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func readAppPPPSchedule(ctx context.Context, client *asc.Client, appID string) (shared.PPPScheduleSnapshot, error) {
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()
	schedule, err := client.GetAppPriceSchedule(requestCtx, appID)
	if err != nil {
		return shared.PPPScheduleSnapshot{}, err
	}
	base, err := client.GetAppPriceScheduleBaseTerritory(requestCtx, schedule.Data.ID)
	if err != nil {
		return shared.PPPScheduleSnapshot{}, err
	}
	snapshot := shared.PPPScheduleSnapshot{BaseTerritory: base.Data.ID}
	for index, fetch := range []appSchedulePriceFetcher{client.GetAppPriceScheduleManualPrices, client.GetAppPriceScheduleAutomaticPrices} {
		first, err := fetch(requestCtx, schedule.Data.ID,
			asc.Limit[asc.AppPriceSchedulePricesOption](200),
			asc.WithAppPriceSchedulePricesInclude([]string{"appPricePoint", "territory"}),
			asc.WithAppPriceSchedulePricesFields([]string{"manual", "startDate", "endDate", "appPricePoint", "territory"}),
			asc.WithAppPriceSchedulePricesPricePointFields([]string{"customerPrice", "proceeds", "territory"}),
			asc.WithAppPriceSchedulePricesTerritoryFields([]string{"currency"}))
		if err != nil {
			return snapshot, err
		}
		err = asc.PaginateEach(requestCtx, first, func(ctx context.Context, next string) (asc.PaginatedResponse, error) {
			next, err := shared.MergeNextURLQuery(next, appSchedulePricesQuery(200))
			if err != nil {
				return nil, err
			}
			return fetch(ctx, schedule.Data.ID, asc.NextURL[asc.AppPriceSchedulePricesOption](next))
		}, func(page asc.PaginatedResponse) error {
			typed, ok := page.(*asc.AppPricesResponse)
			if !ok {
				return fmt.Errorf("unexpected schedule response %T", page)
			}
			values, currencies, err := parseResolvedAppPricePageIncluded(typed.Included)
			if err != nil {
				return err
			}
			for _, item := range typed.Data {
				_, row, ok := resolvedAppPriceCandidateFromResource(item, values, currencies)
				if !ok {
					return fmt.Errorf("cannot resolve complete schedule price %q", item.ID)
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
