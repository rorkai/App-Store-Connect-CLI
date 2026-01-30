package subscriptions

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

// SubscriptionsIntroductoryOffersCommand returns the introductory offers command group.
func SubscriptionsIntroductoryOffersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("introductory-offers", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "introductory-offers",
		ShortUsage: "asc subscriptions introductory-offers <subcommand> [flags]",
		ShortHelp:  "Manage subscription introductory offers.",
		LongHelp: `Manage subscription introductory offers.

Examples:
  asc subscriptions introductory-offers list --subscription-id "SUB_ID"
  asc subscriptions introductory-offers create --subscription-id "SUB_ID" --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsIntroductoryOffersListCommand(),
			SubscriptionsIntroductoryOffersGetCommand(),
			SubscriptionsIntroductoryOffersCreateCommand(),
			SubscriptionsIntroductoryOffersUpdateCommand(),
			SubscriptionsIntroductoryOffersDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsIntroductoryOffersListCommand returns the introductory offers list subcommand.
func SubscriptionsIntroductoryOffersListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("introductory-offers list", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions introductory-offers list [flags]",
		ShortHelp:  "List introductory offers for a subscription.",
		LongHelp: `List introductory offers for a subscription.

Examples:
  asc subscriptions introductory-offers list --subscription-id "SUB_ID"
  asc subscriptions introductory-offers list --subscription-id "SUB_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions introductory-offers list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions introductory-offers list: %w", err)
			}

			id := strings.TrimSpace(*subscriptionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionIntroductoryOffersOption{
				asc.WithSubscriptionIntroductoryOffersLimit(*limit),
				asc.WithSubscriptionIntroductoryOffersNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionIntroductoryOffersLimit(200))
				firstPage, err := client.GetSubscriptionIntroductoryOffers(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions introductory-offers list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionIntroductoryOffers(ctx, id, asc.WithSubscriptionIntroductoryOffersNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions introductory-offers list: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetSubscriptionIntroductoryOffers(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsIntroductoryOffersGetCommand returns the introductory offers get subcommand.
func SubscriptionsIntroductoryOffersGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("introductory-offers get", flag.ExitOnError)

	offerID := fs.String("id", "", "Introductory offer ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc subscriptions introductory-offers get --id \"OFFER_ID\"",
		ShortHelp:  "Get an introductory offer by ID.",
		LongHelp: `Get an introductory offer by ID.

Examples:
  asc subscriptions introductory-offers get --id "OFFER_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*offerID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionIntroductoryOffer(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsIntroductoryOffersCreateCommand returns the introductory offers create subcommand.
func SubscriptionsIntroductoryOffersCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("introductory-offers create", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID")
	offerDuration := fs.String("offer-duration", "", "Offer duration: "+strings.Join(subscriptionOfferDurationValues, ", "))
	offerMode := fs.String("offer-mode", "", "Offer mode: "+strings.Join(subscriptionOfferModeValues, ", "))
	numberOfPeriods := fs.Int("number-of-periods", 0, "Number of periods (required)")
	startDate := fs.String("start-date", "", "Start date (YYYY-MM-DD)")
	endDate := fs.String("end-date", "", "End date (YYYY-MM-DD)")
	territory := fs.String("territory", "", "Territory ID for price override")
	pricePoint := fs.String("price-point", "", "Subscription price point ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc subscriptions introductory-offers create [flags]",
		ShortHelp:  "Create an introductory offer.",
		LongHelp: `Create an introductory offer.

Examples:
  asc subscriptions introductory-offers create --subscription-id "SUB_ID" --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return flag.ErrHelp
			}

			duration, err := normalizeSubscriptionOfferDuration(*offerDuration, true)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			mode, err := normalizeSubscriptionOfferMode(*offerMode, true)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			if *numberOfPeriods <= 0 {
				fmt.Fprintln(os.Stderr, "Error: --number-of-periods is required")
				return flag.ErrHelp
			}

			var normalizedStartDate string
			if strings.TrimSpace(*startDate) != "" {
				normalizedStartDate, err = shared.NormalizeDate(*startDate, "--start-date")
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
			}

			var normalizedEndDate string
			if strings.TrimSpace(*endDate) != "" {
				normalizedEndDate, err = shared.NormalizeDate(*endDate, "--end-date")
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers create: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionIntroductoryOfferCreateAttributes{
				Duration:        duration,
				OfferMode:       mode,
				NumberOfPeriods: *numberOfPeriods,
			}
			if normalizedStartDate != "" {
				attrs.StartDate = normalizedStartDate
			}
			if normalizedEndDate != "" {
				attrs.EndDate = normalizedEndDate
			}

			resp, err := client.CreateSubscriptionIntroductoryOffer(
				requestCtx,
				id,
				attrs,
				strings.TrimSpace(*territory),
				strings.TrimSpace(*pricePoint),
			)
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers create: failed to create: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsIntroductoryOffersUpdateCommand returns the introductory offers update subcommand.
func SubscriptionsIntroductoryOffersUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("introductory-offers update", flag.ExitOnError)

	offerID := fs.String("id", "", "Introductory offer ID")
	endDate := fs.String("end-date", "", "End date (YYYY-MM-DD)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc subscriptions introductory-offers update [flags]",
		ShortHelp:  "Update an introductory offer.",
		LongHelp: `Update an introductory offer.

Examples:
  asc subscriptions introductory-offers update --id "OFFER_ID" --end-date "2026-02-01"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*offerID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*endDate) == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			normalizedEndDate, err := shared.NormalizeDate(*endDate, "--end-date")
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers update: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionIntroductoryOfferUpdateAttributes{
				EndDate: &normalizedEndDate,
			}

			resp, err := client.UpdateSubscriptionIntroductoryOffer(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers update: failed to update: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsIntroductoryOffersDeleteCommand returns the introductory offers delete subcommand.
func SubscriptionsIntroductoryOffersDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("introductory-offers delete", flag.ExitOnError)

	offerID := fs.String("id", "", "Introductory offer ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc subscriptions introductory-offers delete --id \"OFFER_ID\" --confirm",
		ShortHelp:  "Delete an introductory offer.",
		LongHelp: `Delete an introductory offer.

Examples:
  asc subscriptions introductory-offers delete --id "OFFER_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*offerID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions introductory-offers delete: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteSubscriptionIntroductoryOffer(requestCtx, id); err != nil {
				return fmt.Errorf("subscriptions introductory-offers delete: failed to delete: %w", err)
			}

			result := &deleteResult{ID: id, Deleted: true}
			return printOutput(result, *output, *pretty)
		},
	}
}
