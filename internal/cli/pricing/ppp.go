package pricing

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// PricingPPPCommand returns the pricing ppp subcommand.
func PricingPPPCommand() *ffcli.Command {
	return shared.NewPriceSchedulePPPCommand(shared.PriceSchedulePPPCommandConfig{
		FlagSetName:  "pricing ppp",
		ShortUsage:   "asc pricing ppp --app APP_ID --base-price PRICE [flags]",
		ResourceNoun: "app",
		ErrorPrefix:  "pricing ppp",
		Examples: `  asc pricing ppp --app "123456789" --base-price "4.99" --dry-run
  asc pricing ppp --app "123456789" --base-price "4.99" --round down --dry-run --output table
  asc pricing ppp --app "123456789" --base-price "4.99" --territory "IN,BR,MX" --confirm
  asc pricing ppp --app "123456789" --base-territory "GBR" --base-price "3.99" --start-date "YYYY-MM-DD" --confirm`,
		BindResource: func(fs *flag.FlagSet) shared.PriceSchedulePPPResource {
			appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
			return shared.PriceSchedulePPPResource{
				Validate: func() error {
					if shared.ResolveAppID(*appID) == "" {
						fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
						return shared.MissingRequiredUsageError("--app")
					}
					return nil
				},
				Resolve: func(ctx context.Context, client *asc.Client) (shared.PriceSchedulePPPSurface, error) {
					return appPPPSurface(client, shared.ResolveAppID(*appID)), nil
				},
			}
		},
	})
}

func appPPPSurface(client *asc.Client, appID string) shared.PriceSchedulePPPSurface {
	return shared.PriceSchedulePPPSurface{
		AppID:           appID,
		LadderBatchSize: 10,
		ReadSchedule: func(ctx context.Context) (shared.PPPScheduleSnapshot, error) {
			return readAppPPPSchedule(ctx, client, appID)
		},
		CurrentPrices: func(ctx context.Context, at time.Time) ([]shared.ResolvedPriceRow, error) {
			schedule, err := shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (*asc.AppPriceScheduleResponse, error) {
				return client.GetAppPriceSchedule(requestCtx, appID)
			})
			if err != nil {
				return nil, err
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			resolved, err := fetchResolvedAppSchedulePrices(requestCtx, client, schedule.Data.ID, "manual", 200, "", at, true)
			if isAppPriceScheduleNotConfigured(err) {
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			return resolved.Prices, nil
		},
		LadderBatch: func(ctx context.Context, territories []string) (map[string][]shared.PriceLadderCandidate, error) {
			return shared.CollectPricePointsByTerritory(ctx, territories, func(requestCtx context.Context, next string) (*asc.AppPricePointsV3Response, error) {
				if next != "" {
					return client.GetAppPricePoints(requestCtx, appID, asc.NextURL[asc.PricePointsOption](next))
				}
				return client.GetAppPricePoints(
					requestCtx,
					appID,
					asc.WithPricePointsTerritory(strings.Join(territories, ",")),
					asc.WithPricePointsInclude([]string{"territory"}),
					asc.Limit[asc.PricePointsOption](200),
				)
			}, appCustomerPrice)
		},
		Equalizations: func(ctx context.Context, pricePointID string) (map[string][]shared.PriceLadderCandidate, error) {
			return shared.CollectPricePointsByTerritory(ctx, nil, func(requestCtx context.Context, next string) (*asc.AppPricePointsV3Response, error) {
				if next != "" {
					return client.GetAppPricePointEqualizations(requestCtx, pricePointID, asc.NextURL[asc.PricePointsOption](next))
				}
				return client.GetAppPricePointEqualizations(
					requestCtx,
					pricePointID,
					asc.WithPricePointsInclude([]string{"territory"}),
					asc.Limit[asc.PricePointsOption](200),
				)
			}, appCustomerPrice)
		},
		ReplaceSchedule: func(ctx context.Context, baseTerritory string, prices []shared.ScheduledPricePoint) error {
			attrs := asc.AppPriceScheduleCreateAttributes{BaseTerritoryID: baseTerritory}
			for _, price := range prices {
				attrs.Prices = append(attrs.Prices, asc.AppPriceSchedulePrice(price))
			}
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			_, err := client.CreateAppPriceSchedule(requestCtx, appID, attrs)
			return err
		},
	}
}

func appCustomerPrice(attributes asc.AppPricePointV3Attributes) string {
	return attributes.CustomerPrice
}
