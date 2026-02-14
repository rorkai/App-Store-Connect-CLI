package offercodes

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

// OfferCodePricesCommand returns the prices command group.
func OfferCodePricesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("prices", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "prices",
		ShortUsage: "asc offer-codes prices <subcommand> [flags]",
		ShortHelp:  "Manage offer code prices.",
		LongHelp: `Manage offer code prices.

Examples:
  asc offer-codes prices list --offer-code-id "OFFER_CODE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			OfferCodePricesListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// OfferCodePricesListCommand returns the prices list subcommand.
func OfferCodePricesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	offerCodeID := fs.String("offer-code-id", "", "Subscription offer code ID (required)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc offer-codes prices list [flags]",
		ShortHelp:  "List prices for a subscription offer code.",
		LongHelp: `List prices for a subscription offer code.

Examples:
  asc offer-codes prices list --offer-code-id "OFFER_CODE_ID"
  asc offer-codes prices list --offer-code-id "OFFER_CODE_ID" --limit 50
  asc offer-codes prices list --offer-code-id "OFFER_CODE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > offerCodesMaxLimit) {
				return fmt.Errorf("offer-codes prices list: --limit must be between 1 and %d", offerCodesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("offer-codes prices list: %w", err)
			}

			trimmedOfferCodeID := strings.TrimSpace(*offerCodeID)
			if trimmedOfferCodeID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --offer-code-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("offer-codes prices list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionOfferCodePricesOption{
				asc.WithSubscriptionOfferCodePricesLimit(*limit),
				asc.WithSubscriptionOfferCodePricesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionOfferCodePricesLimit(offerCodesMaxLimit))
				firstPage, err := client.GetSubscriptionOfferCodePrices(requestCtx, trimmedOfferCodeID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("offer-codes prices list: failed to fetch: %w", err)
				}

				pages, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionOfferCodePrices(ctx, trimmedOfferCodeID, asc.WithSubscriptionOfferCodePricesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("offer-codes prices list: %w", err)
				}

				return shared.PrintOutput(pages, *output.Output, *output.Pretty)
			}

			resp, err := client.GetSubscriptionOfferCodePrices(requestCtx, trimmedOfferCodeID, opts...)
			if err != nil {
				return fmt.Errorf("offer-codes prices list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
