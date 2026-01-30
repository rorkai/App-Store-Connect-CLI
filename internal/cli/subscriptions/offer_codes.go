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

// SubscriptionsOfferCodesCommand returns the offer codes command group.
func SubscriptionsOfferCodesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "offer-codes",
		ShortUsage: "asc subscriptions offer-codes <subcommand> [flags]",
		ShortHelp:  "Manage subscription offer codes.",
		LongHelp: `Manage subscription offer codes.

Examples:
  asc subscriptions offer-codes list --subscription-id "SUB_ID"
  asc subscriptions offer-codes create --subscription-id "SUB_ID" --name "SPRING" --offer-eligibility STACK_WITH_INTRO_OFFERS --customer-eligibilities NEW --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1 --prices "PRICE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsOfferCodesListCommand(),
			SubscriptionsOfferCodesGetCommand(),
			SubscriptionsOfferCodesCreateCommand(),
			SubscriptionsOfferCodesUpdateCommand(),
			SubscriptionsOfferCodesCustomCodesCommand(),
			SubscriptionsOfferCodesOneTimeCodesCommand(),
			SubscriptionsOfferCodesPricesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsOfferCodesListCommand returns the offer codes list subcommand.
func SubscriptionsOfferCodesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes list", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions offer-codes list [flags]",
		ShortHelp:  "List offer codes for a subscription.",
		LongHelp: `List offer codes for a subscription.

Examples:
  asc subscriptions offer-codes list --subscription-id "SUB_ID"
  asc subscriptions offer-codes list --subscription-id "SUB_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions offer-codes list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions offer-codes list: %w", err)
			}

			id := strings.TrimSpace(*subscriptionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionOfferCodesOption{
				asc.WithSubscriptionOfferCodesLimit(*limit),
				asc.WithSubscriptionOfferCodesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionOfferCodesLimit(200))
				firstPage, err := client.GetSubscriptionOfferCodes(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions offer-codes list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionOfferCodes(ctx, id, asc.WithSubscriptionOfferCodesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions offer-codes list: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetSubscriptionOfferCodes(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsOfferCodesGetCommand returns the offer codes get subcommand.
func SubscriptionsOfferCodesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes get", flag.ExitOnError)

	offerCodeID := fs.String("id", "", "Offer code ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc subscriptions offer-codes get --id \"OFFER_CODE_ID\"",
		ShortHelp:  "Get an offer code by ID.",
		LongHelp: `Get an offer code by ID.

Examples:
  asc subscriptions offer-codes get --id "OFFER_CODE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*offerCodeID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionOfferCode(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsOfferCodesCreateCommand returns the offer codes create subcommand.
func SubscriptionsOfferCodesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes create", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID")
	name := fs.String("name", "", "Offer code name")
	offerEligibility := fs.String("offer-eligibility", "", "Offer eligibility: "+strings.Join(subscriptionOfferEligibilityValues, ", "))
	customerEligibilities := fs.String("customer-eligibilities", "", "Customer eligibilities: "+strings.Join(subscriptionCustomerEligibilityValues, ", "))
	offerDuration := fs.String("offer-duration", "", "Offer duration: "+strings.Join(subscriptionOfferDurationValues, ", "))
	offerMode := fs.String("offer-mode", "", "Offer mode: "+strings.Join(subscriptionOfferModeValues, ", "))
	numberOfPeriods := fs.Int("number-of-periods", 0, "Number of periods (required)")
	prices := fs.String("prices", "", "Offer code price ID(s), comma-separated")
	var autoRenewEnabled shared.OptionalBool
	fs.Var(&autoRenewEnabled, "auto-renew-enabled", "Enable auto-renew: true or false")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc subscriptions offer-codes create [flags]",
		ShortHelp:  "Create an offer code.",
		LongHelp: `Create an offer code.

Examples:
  asc subscriptions offer-codes create --subscription-id "SUB_ID" --name "SPRING" --offer-eligibility STACK_WITH_INTRO_OFFERS --customer-eligibilities NEW --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1 --prices "PRICE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return flag.ErrHelp
			}

			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}

			eligibility, err := normalizeSubscriptionOfferEligibility(*offerEligibility, true)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			customerEligibilityValues, err := normalizeSubscriptionCustomerEligibilities(*customerEligibilities, true)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
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
				return fmt.Errorf("subscriptions offer-codes create: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			attrs := asc.SubscriptionOfferCodeCreateAttributes{
				Name:                  nameValue,
				OfferEligibility:      eligibility,
				CustomerEligibilities: customerEligibilityValues,
				Duration:              duration,
				OfferMode:             mode,
				NumberOfPeriods:       *numberOfPeriods,
			}
			if autoRenewEnabled.IsSet() {
				value := autoRenewEnabled.Value()
				attrs.AutoRenewEnabled = &value
			}

			resp, err := client.CreateSubscriptionOfferCode(requestCtx, id, attrs, priceIDs)
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes create: failed to create: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsOfferCodesUpdateCommand returns the offer codes update subcommand.
func SubscriptionsOfferCodesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes update", flag.ExitOnError)

	offerCodeID := fs.String("id", "", "Offer code ID")
	var active shared.OptionalBool
	fs.Var(&active, "active", "Enable or disable the offer code: true or false")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc subscriptions offer-codes update [flags]",
		ShortHelp:  "Update an offer code.",
		LongHelp: `Update an offer code.

Examples:
  asc subscriptions offer-codes update --id "OFFER_CODE_ID" --active false`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*offerCodeID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !active.IsSet() {
				fmt.Fprintln(os.Stderr, "Error: --active is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes update: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			value := active.Value()
			attrs := asc.SubscriptionOfferCodeUpdateAttributes{
				Active: &value,
			}

			resp, err := client.UpdateSubscriptionOfferCode(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes update: failed to update: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsOfferCodesCustomCodesCommand returns the offer code custom codes subcommand.
func SubscriptionsOfferCodesCustomCodesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes custom-codes", flag.ExitOnError)

	offerCodeID := fs.String("offer-code-id", "", "Offer code ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "custom-codes",
		ShortUsage: "asc subscriptions offer-codes custom-codes --offer-code-id \"OFFER_CODE_ID\" [flags]",
		ShortHelp:  "List custom codes for an offer code.",
		LongHelp: `List custom codes for an offer code.

Examples:
  asc subscriptions offer-codes custom-codes --offer-code-id "OFFER_CODE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions offer-codes custom-codes: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions offer-codes custom-codes: %w", err)
			}

			id := strings.TrimSpace(*offerCodeID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --offer-code-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes custom-codes: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionOfferCodeCustomCodesOption{
				asc.WithSubscriptionOfferCodeCustomCodesLimit(*limit),
				asc.WithSubscriptionOfferCodeCustomCodesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionOfferCodeCustomCodesLimit(200))
				firstPage, err := client.GetSubscriptionOfferCodeCustomCodes(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions offer-codes custom-codes: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionOfferCodeCustomCodes(ctx, id, asc.WithSubscriptionOfferCodeCustomCodesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions offer-codes custom-codes: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetSubscriptionOfferCodeCustomCodes(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes custom-codes: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsOfferCodesOneTimeCodesCommand returns the offer code one-time use codes subcommand.
func SubscriptionsOfferCodesOneTimeCodesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes one-time-codes", flag.ExitOnError)

	offerCodeID := fs.String("offer-code-id", "", "Offer code ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "one-time-codes",
		ShortUsage: "asc subscriptions offer-codes one-time-codes --offer-code-id \"OFFER_CODE_ID\" [flags]",
		ShortHelp:  "List one-time use code batches for an offer code.",
		LongHelp: `List one-time use code batches for an offer code.

Examples:
  asc subscriptions offer-codes one-time-codes --offer-code-id "OFFER_CODE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions offer-codes one-time-codes: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions offer-codes one-time-codes: %w", err)
			}

			id := strings.TrimSpace(*offerCodeID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --offer-code-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes one-time-codes: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionOfferCodeOneTimeUseCodesOption{
				asc.WithSubscriptionOfferCodeOneTimeUseCodesLimit(*limit),
				asc.WithSubscriptionOfferCodeOneTimeUseCodesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionOfferCodeOneTimeUseCodesLimit(200))
				firstPage, err := client.GetSubscriptionOfferCodeOneTimeUseCodes(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions offer-codes one-time-codes: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionOfferCodeOneTimeUseCodes(ctx, id, asc.WithSubscriptionOfferCodeOneTimeUseCodesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions offer-codes one-time-codes: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetSubscriptionOfferCodeOneTimeUseCodes(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes one-time-codes: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsOfferCodesPricesCommand returns the offer code prices subcommand.
func SubscriptionsOfferCodesPricesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes prices", flag.ExitOnError)

	offerCodeID := fs.String("offer-code-id", "", "Offer code ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "prices",
		ShortUsage: "asc subscriptions offer-codes prices --offer-code-id \"OFFER_CODE_ID\" [flags]",
		ShortHelp:  "List prices for an offer code.",
		LongHelp: `List prices for an offer code.

Examples:
  asc subscriptions offer-codes prices --offer-code-id "OFFER_CODE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions offer-codes prices: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions offer-codes prices: %w", err)
			}

			id := strings.TrimSpace(*offerCodeID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --offer-code-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes prices: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionOfferCodePricesOption{
				asc.WithSubscriptionOfferCodePricesLimit(*limit),
				asc.WithSubscriptionOfferCodePricesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionOfferCodePricesLimit(200))
				firstPage, err := client.GetSubscriptionOfferCodePrices(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions offer-codes prices: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionOfferCodePrices(ctx, id, asc.WithSubscriptionOfferCodePricesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions offer-codes prices: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetSubscriptionOfferCodePrices(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions offer-codes prices: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
