package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// SubscriptionsPromotionalOffersCommand returns the promotional offers command group.
func SubscriptionsPromotionalOffersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("promotional-offers", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "promotional-offers",
		ShortUsage: "asc subscriptions promotional-offers <subcommand> [flags]",
		ShortHelp:  "Manage subscription promotional offers.",
		LongHelp: `Manage subscription promotional offers.

Examples:
  asc subscriptions promotional-offers list --subscription-id "SUB_ID"
  asc subscriptions promotional-offers create --subscription-id "SUB_ID" --offer-code "SPRING" --name "Spring" --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1 --prices "PRICE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsPromotionalOffersListCommand(),
			SubscriptionsPromotionalOffersGetCommand(),
			SubscriptionsPromotionalOffersCreateCommand(),
			SubscriptionsPromotionalOffersUpdateCommand(),
			SubscriptionsPromotionalOffersDeleteCommand(),
			SubscriptionsPromotionalOfferPricesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsPromotionalOffersListCommand returns the promotional offers list subcommand.
func SubscriptionsPromotionalOffersListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("promotional-offers list", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions promotional-offers list [flags]",
		ShortHelp:  "List promotional offers for a subscription.",
		LongHelp: `List promotional offers for a subscription.

Examples:
  asc subscriptions promotional-offers list --subscription-id "SUB_ID"
  asc subscriptions promotional-offers list --subscription-id "SUB_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions promotional-offers list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions promotional-offers list: %w", err)
			}

			id := strings.TrimSpace(*subscriptionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions promotional-offers list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionPromotionalOffersOption{
				asc.WithSubscriptionPromotionalOffersLimit(*limit),
				asc.WithSubscriptionPromotionalOffersNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionPromotionalOffersLimit(200))
				firstPage, err := client.GetSubscriptionPromotionalOffers(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions promotional-offers list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionPromotionalOffers(ctx, id, asc.WithSubscriptionPromotionalOffersNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions promotional-offers list: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetSubscriptionPromotionalOffers(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions promotional-offers list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsPromotionalOffersGetCommand returns the promotional offers get subcommand.
func SubscriptionsPromotionalOffersGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("promotional-offers get", flag.ExitOnError)

	offerID := fs.String("id", "", "Promotional offer ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc subscriptions promotional-offers get --id \"OFFER_ID\"",
		ShortHelp:  "Get a promotional offer by ID.",
		LongHelp: `Get a promotional offer by ID.

Examples:
  asc subscriptions promotional-offers get --id "OFFER_ID"`,
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
				return fmt.Errorf("subscriptions promotional-offers get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionPromotionalOffer(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions promotional-offers get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsPromotionalOffersCreateCommand returns the promotional offers create subcommand.
func SubscriptionsPromotionalOffersCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("promotional-offers create", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID")
	offerCode := fs.String("offer-code", "", "Offer code")
	name := fs.String("name", "", "Offer name")
	offerDuration := fs.String("offer-duration", "", "Offer duration: "+strings.Join(subscriptionOfferDurationValues, ", "))
	offerMode := fs.String("offer-mode", "", "Offer mode: "+strings.Join(subscriptionOfferModeValues, ", "))
	numberOfPeriods := fs.Int("number-of-periods", 0, "Number of periods (required)")
	prices := fs.String("prices", "", "Promotional offer price ID(s), comma-separated")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc subscriptions promotional-offers create [flags]",
		ShortHelp:  "Create a promotional offer.",
		LongHelp: `Create a promotional offer.

Examples:
  asc subscriptions promotional-offers create --subscription-id "SUB_ID" --offer-code "SPRING" --name "Spring" --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1 --prices "PRICE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return flag.ErrHelp
			}

			offerCodeValue := strings.TrimSpace(*offerCode)
			if offerCodeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --offer-code is required")
				return flag.ErrHelp
			}

			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
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

			priceIDs := parseCommaSeparatedIDs(*prices)
			if len(priceIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --prices is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions promotional-offers create: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionPromotionalOfferCreateAttributes{
				Duration:        duration,
				Name:            nameValue,
				NumberOfPeriods: *numberOfPeriods,
				OfferCode:       offerCodeValue,
				OfferMode:       mode,
			}

			resp, err := client.CreateSubscriptionPromotionalOffer(requestCtx, id, attrs, priceIDs)
			if err != nil {
				return fmt.Errorf("subscriptions promotional-offers create: failed to create: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsPromotionalOffersUpdateCommand returns the promotional offers update subcommand.
func SubscriptionsPromotionalOffersUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("promotional-offers update", flag.ExitOnError)

	offerID := fs.String("id", "", "Promotional offer ID")
	prices := fs.String("prices", "", "Promotional offer price ID(s), comma-separated")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc subscriptions promotional-offers update [flags]",
		ShortHelp:  "Update a promotional offer's prices.",
		LongHelp: `Update a promotional offer's prices.

Examples:
  asc subscriptions promotional-offers update --id "OFFER_ID" --prices "PRICE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*offerID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			priceIDs := parseCommaSeparatedIDs(*prices)
			if len(priceIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --prices is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions promotional-offers update: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateSubscriptionPromotionalOffer(requestCtx, id, priceIDs)
			if err != nil {
				return fmt.Errorf("subscriptions promotional-offers update: failed to update: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsPromotionalOffersDeleteCommand returns the promotional offers delete subcommand.
func SubscriptionsPromotionalOffersDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("promotional-offers delete", flag.ExitOnError)

	offerID := fs.String("id", "", "Promotional offer ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc subscriptions promotional-offers delete --id \"OFFER_ID\" --confirm",
		ShortHelp:  "Delete a promotional offer.",
		LongHelp: `Delete a promotional offer.

Examples:
  asc subscriptions promotional-offers delete --id "OFFER_ID" --confirm`,
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
				return fmt.Errorf("subscriptions promotional-offers delete: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteSubscriptionPromotionalOffer(requestCtx, id); err != nil {
				return fmt.Errorf("subscriptions promotional-offers delete: failed to delete: %w", err)
			}

			result := &deleteResult{ID: id, Deleted: true}
			return printOutput(result, *output, *pretty)
		},
	}
}

// SubscriptionsPromotionalOfferPricesCommand returns the promotional offer prices subcommand.
func SubscriptionsPromotionalOfferPricesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("promotional-offers prices", flag.ExitOnError)

	offerID := fs.String("id", "", "Promotional offer ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "prices",
		ShortUsage: "asc subscriptions promotional-offers prices --id \"OFFER_ID\" [flags]",
		ShortHelp:  "List prices for a promotional offer.",
		LongHelp: `List prices for a promotional offer.

Examples:
  asc subscriptions promotional-offers prices --id "OFFER_ID"
  asc subscriptions promotional-offers prices --id "OFFER_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions promotional-offers prices: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions promotional-offers prices: %w", err)
			}

			id := strings.TrimSpace(*offerID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions promotional-offers prices: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionPromotionalOfferPricesOption{
				asc.WithSubscriptionPromotionalOfferPricesLimit(*limit),
				asc.WithSubscriptionPromotionalOfferPricesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionPromotionalOfferPricesLimit(200))
				firstPage, err := client.GetSubscriptionPromotionalOfferPrices(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions promotional-offers prices: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionPromotionalOfferPrices(ctx, id, asc.WithSubscriptionPromotionalOfferPricesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions promotional-offers prices: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetSubscriptionPromotionalOfferPrices(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions promotional-offers prices: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
