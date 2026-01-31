package iap

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// IAPOfferCodesCustomCodesCommand returns the custom codes command group.
func IAPOfferCodesCustomCodesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes custom-codes", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "custom-codes",
		ShortUsage: "asc iap offer-codes custom-codes <subcommand> [flags]",
		ShortHelp:  "Manage custom codes for in-app purchase offer codes.",
		LongHelp: `Manage custom codes for in-app purchase offer codes.

Examples:
  asc iap offer-codes custom-codes list --offer-code-id "OFFER_CODE_ID"
  asc iap offer-codes custom-codes get --custom-code-id "CUSTOM_CODE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPOfferCodesCustomCodesListCommand(),
			IAPOfferCodesCustomCodesGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// IAPOfferCodesCustomCodesListCommand returns the custom codes list subcommand.
func IAPOfferCodesCustomCodesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes custom-codes list", flag.ExitOnError)

	offerCodeID := fs.String("offer-code-id", "", "Offer code ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc iap offer-codes custom-codes list --offer-code-id \"OFFER_CODE_ID\" [flags]",
		ShortHelp:  "List custom codes for an offer code.",
		LongHelp: `List custom codes for an offer code.

Examples:
  asc iap offer-codes custom-codes list --offer-code-id "OFFER_CODE_ID"
  asc iap offer-codes custom-codes list --offer-code-id "OFFER_CODE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("iap offer-codes custom-codes list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("iap offer-codes custom-codes list: %w", err)
			}

			id := strings.TrimSpace(*offerCodeID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --offer-code-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("iap offer-codes custom-codes list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.IAPOfferCodeCustomCodesOption{
				asc.WithIAPOfferCodeCustomCodesLimit(*limit),
				asc.WithIAPOfferCodeCustomCodesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithIAPOfferCodeCustomCodesLimit(200))
				firstPage, err := client.GetInAppPurchaseOfferCodeCustomCodes(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("iap offer-codes custom-codes list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetInAppPurchaseOfferCodeCustomCodes(ctx, id, asc.WithIAPOfferCodeCustomCodesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap offer-codes custom-codes list: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetInAppPurchaseOfferCodeCustomCodes(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("iap offer-codes custom-codes list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// IAPOfferCodesCustomCodesGetCommand returns the custom codes get subcommand.
func IAPOfferCodesCustomCodesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes custom-codes get", flag.ExitOnError)

	customCodeID := fs.String("custom-code-id", "", "Custom code ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc iap offer-codes custom-codes get --custom-code-id \"CUSTOM_CODE_ID\"",
		ShortHelp:  "Get a custom code by ID.",
		LongHelp: `Get a custom code by ID.

Examples:
  asc iap offer-codes custom-codes get --custom-code-id "CUSTOM_CODE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*customCodeID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --custom-code-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("iap offer-codes custom-codes get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetInAppPurchaseOfferCodeCustomCode(requestCtx, id)
			if err != nil {
				return fmt.Errorf("iap offer-codes custom-codes get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// IAPOfferCodesOneTimeCodesCommand returns the one-time codes command group.
func IAPOfferCodesOneTimeCodesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes one-time-codes", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "one-time-codes",
		ShortUsage: "asc iap offer-codes one-time-codes <subcommand> [flags]",
		ShortHelp:  "Manage one-time use codes for in-app purchase offer codes.",
		LongHelp: `Manage one-time use codes for in-app purchase offer codes.

Examples:
  asc iap offer-codes one-time-codes list --offer-code-id "OFFER_CODE_ID"
  asc iap offer-codes one-time-codes get --one-time-code-id "ONE_TIME_USE_CODE_ID"
  asc iap offer-codes one-time-codes values --one-time-code-id "ONE_TIME_USE_CODE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPOfferCodesOneTimeCodesListCommand(),
			IAPOfferCodesOneTimeCodesGetCommand(),
			IAPOfferCodesOneTimeCodesValuesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// IAPOfferCodesOneTimeCodesListCommand returns the one-time codes list subcommand.
func IAPOfferCodesOneTimeCodesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes one-time-codes list", flag.ExitOnError)

	offerCodeID := fs.String("offer-code-id", "", "Offer code ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc iap offer-codes one-time-codes list --offer-code-id \"OFFER_CODE_ID\" [flags]",
		ShortHelp:  "List one-time use code batches for an offer code.",
		LongHelp: `List one-time use code batches for an offer code.

Examples:
  asc iap offer-codes one-time-codes list --offer-code-id "OFFER_CODE_ID"
  asc iap offer-codes one-time-codes list --offer-code-id "OFFER_CODE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("iap offer-codes one-time-codes list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("iap offer-codes one-time-codes list: %w", err)
			}

			id := strings.TrimSpace(*offerCodeID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --offer-code-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("iap offer-codes one-time-codes list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.IAPOfferCodeOneTimeUseCodesOption{
				asc.WithIAPOfferCodeOneTimeUseCodesLimit(*limit),
				asc.WithIAPOfferCodeOneTimeUseCodesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithIAPOfferCodeOneTimeUseCodesLimit(200))
				firstPage, err := client.GetInAppPurchaseOfferCodeOneTimeUseCodes(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("iap offer-codes one-time-codes list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetInAppPurchaseOfferCodeOneTimeUseCodes(ctx, id, asc.WithIAPOfferCodeOneTimeUseCodesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap offer-codes one-time-codes list: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetInAppPurchaseOfferCodeOneTimeUseCodes(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("iap offer-codes one-time-codes list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// IAPOfferCodesOneTimeCodesGetCommand returns the one-time codes get subcommand.
func IAPOfferCodesOneTimeCodesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes one-time-codes get", flag.ExitOnError)

	oneTimeCodeID := fs.String("one-time-code-id", "", "One-time use code batch ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc iap offer-codes one-time-codes get --one-time-code-id \"ONE_TIME_USE_CODE_ID\"",
		ShortHelp:  "Get a one-time use code batch by ID.",
		LongHelp: `Get a one-time use code batch by ID.

Examples:
  asc iap offer-codes one-time-codes get --one-time-code-id "ONE_TIME_USE_CODE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*oneTimeCodeID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --one-time-code-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("iap offer-codes one-time-codes get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetInAppPurchaseOfferCodeOneTimeUseCode(requestCtx, id)
			if err != nil {
				return fmt.Errorf("iap offer-codes one-time-codes get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// IAPOfferCodesOneTimeCodesValuesCommand returns the one-time code values subcommand.
func IAPOfferCodesOneTimeCodesValuesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes one-time-codes values", flag.ExitOnError)

	oneTimeCodeID := fs.String("one-time-code-id", "", "One-time use code batch ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "values",
		ShortUsage: "asc iap offer-codes one-time-codes values --one-time-code-id \"ONE_TIME_USE_CODE_ID\"",
		ShortHelp:  "Fetch one-time use offer code values for a batch.",
		LongHelp: `Fetch one-time use offer code values for a batch.

Examples:
  asc iap offer-codes one-time-codes values --one-time-code-id "ONE_TIME_USE_CODE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*oneTimeCodeID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --one-time-code-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("iap offer-codes one-time-codes values: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			values, err := client.GetInAppPurchaseOfferCodeOneTimeUseCodeValues(requestCtx, id)
			if err != nil {
				return fmt.Errorf("iap offer-codes one-time-codes values: failed to fetch: %w", err)
			}

			result := &asc.OfferCodeValuesResult{Codes: values}
			return printOutput(result, *output, *pretty)
		},
	}
}
