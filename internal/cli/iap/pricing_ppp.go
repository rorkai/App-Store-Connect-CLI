package iap

import (
	"context"
	"errors"
	"flag"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// IAPPricingPPPCommand returns the iap pricing ppp subcommand.
func IAPPricingPPPCommand() *ffcli.Command {
	return shared.NewPriceSchedulePPPCommand(shared.PriceSchedulePPPCommandConfig{
		FlagSetName:  "iap pricing ppp",
		ShortUsage:   "asc iap pricing ppp --iap-id IAP_ID --base-price PRICE [flags]",
		ResourceNoun: "in-app purchase",
		ErrorPrefix:  "iap pricing ppp",
		Examples: `  asc iap pricing ppp --iap-id "IAP_ID" --base-price "4.99" --dry-run
  asc iap pricing ppp --iap-id "IAP_ID" --base-price "4.99" --round down --dry-run --output table
  asc iap pricing ppp --iap-id "IAP_ID" --base-price "4.99" --territory "IN,BR,MX" --confirm
  asc iap pricing ppp --iap-id "IAP_ID" --base-territory "GBR" --base-price "3.99" --start-date "YYYY-MM-DD" --confirm`,
		BindResource: func(fs *flag.FlagSet) shared.PriceSchedulePPPResource {
			iapID := shared.BindResourceIDFlag(fs, "iap-id", "inAppPurchases", "In-app purchase ID, product ID, or exact current name (required)")
			appID := addIAPLookupAppFlag(fs)
			return shared.PriceSchedulePPPResource{
				Validate: func() error {
					if strings.TrimSpace(*iapID) == "" {
						return shared.RequiredFlag("--iap-id")
					}
					return shared.RequireAppForStableSelector(shared.ResolveAppID(*appID), strings.TrimSpace(*iapID), "--iap-id")
				},
				Resolve: func(ctx context.Context, client *asc.Client) (shared.PriceSchedulePPPSurface, error) {
					id, err := resolveIAPLookupIDWithTimeout(ctx, client, *appID, strings.TrimSpace(*iapID))
					if err != nil {
						return shared.PriceSchedulePPPSurface{}, err
					}
					return iapPPPSurface(client, id), nil
				},
			}
		},
	})
}

func iapPPPSurface(client *asc.Client, iapID string) shared.PriceSchedulePPPSurface {
	return shared.PriceSchedulePPPSurface{
		IAPID:           iapID,
		LadderBatchSize: 10,
		ReadSchedule: func(ctx context.Context) (shared.PPPScheduleSnapshot, error) {
			return readIAPPPPSchedule(ctx, client, iapID)
		},
		CurrentPrices: func(ctx context.Context, at time.Time) ([]shared.ResolvedPriceRow, error) {
			schedule, err := shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (*asc.InAppPurchasePriceScheduleResponse, error) {
				return client.GetInAppPurchasePriceSchedule(requestCtx, iapID)
			})
			if errors.Is(err, asc.ErrNotFound) {
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resolved, err := fetchResolvedIAPSchedulePrices(requestCtx, client, schedule.Data.ID, "manual", 200, "", at, true)
			if err != nil {
				return nil, err
			}
			return resolved.Prices, nil
		},
		LadderBatch: func(ctx context.Context, territories []string) (map[string][]shared.PriceLadderCandidate, error) {
			return shared.CollectPricePointsByTerritory(ctx, territories, func(requestCtx context.Context, next string) (*asc.InAppPurchasePricePointsResponse, error) {
				if next != "" {
					return client.GetInAppPurchasePricePoints(requestCtx, iapID, asc.WithIAPPricePointsNextURL(next))
				}
				return client.GetInAppPurchasePricePoints(
					requestCtx,
					iapID,
					asc.WithIAPPricePointsTerritory(strings.Join(territories, ",")),
					asc.WithIAPPricePointsFields([]string{"customerPrice", "territory"}),
					asc.WithIAPPricePointsInclude([]string{"territory"}),
					asc.WithIAPPricePointsLimit(8000),
				)
			}, iapCustomerPrice)
		},
		Equalizations: func(ctx context.Context, pricePointID string) (map[string][]shared.PriceLadderCandidate, error) {
			return shared.CollectPricePointsByTerritory(ctx, nil, func(requestCtx context.Context, next string) (*asc.InAppPurchasePricePointsResponse, error) {
				if next != "" {
					return client.GetInAppPurchasePricePointEqualizations(requestCtx, pricePointID, asc.WithIAPPricePointsNextURL(next))
				}
				return client.GetInAppPurchasePricePointEqualizations(
					requestCtx,
					pricePointID,
					asc.WithIAPPricePointsFields([]string{"customerPrice", "territory"}),
					asc.WithIAPPricePointsInclude([]string{"territory"}),
					asc.WithIAPPricePointsLimit(8000),
				)
			}, iapCustomerPrice)
		},
		ReplaceSchedule: func(ctx context.Context, baseTerritory string, prices []shared.ScheduledPricePoint) error {
			attrs := asc.InAppPurchasePriceScheduleCreateAttributes{BaseTerritoryID: baseTerritory}
			for _, price := range prices {
				attrs.Prices = append(attrs.Prices, asc.InAppPurchasePriceSchedulePrice(price))
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			_, err := client.CreateInAppPurchasePriceSchedule(requestCtx, iapID, attrs)
			return err
		},
		PricePointKey: func(id string) string {
			if meta, ok := decodeIAPPriceResourceMetadata(id); ok && meta.PricePointID != "" {
				return meta.PricePointID
			}
			return strings.TrimSpace(id)
		},
	}
}

func iapCustomerPrice(attributes asc.InAppPurchasePricePointAttributes) string {
	return attributes.CustomerPrice
}
