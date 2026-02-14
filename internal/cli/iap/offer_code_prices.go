package iap

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// IAPOfferCodesPricesCommand returns the offer code prices subcommand.
func IAPOfferCodesPricesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes prices", flag.ExitOnError)

	offerCodeID := fs.String("offer-code-id", "", "Offer code ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "prices",
		ShortUsage: "asc iap offer-codes prices --offer-code-id \"OFFER_CODE_ID\" [flags]",
		ShortHelp:  "List prices for an offer code.",
		LongHelp: `List prices for an offer code.

Examples:
  asc iap offer-codes prices --offer-code-id "OFFER_CODE_ID"
  asc iap offer-codes prices --offer-code-id "OFFER_CODE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("iap offer-codes prices: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("iap offer-codes prices: %w", err)
			}

			id := strings.TrimSpace(*offerCodeID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --offer-code-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap offer-codes prices: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.IAPOfferCodePricesOption{
				asc.WithIAPOfferCodePricesLimit(*limit),
				asc.WithIAPOfferCodePricesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithIAPOfferCodePricesLimit(200))
				firstPage, err := client.GetInAppPurchaseOfferCodePrices(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("iap offer-codes prices: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetInAppPurchaseOfferCodePrices(ctx, id, asc.WithIAPOfferCodePricesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap offer-codes prices: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetInAppPurchaseOfferCodePrices(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("iap offer-codes prices: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
