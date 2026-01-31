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

var defaultOfferCodeEligibilities = []string{
	"NON_SPENDER",
	"ACTIVE_SPENDER",
	"CHURNED_SPENDER",
}

// IAPOfferCodesCommand returns the offer codes command group.
func IAPOfferCodesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "offer-codes",
		ShortUsage: "asc iap offer-codes <subcommand> [flags]",
		ShortHelp:  "Manage in-app purchase offer codes.",
		LongHelp: `Manage in-app purchase offer codes.

Examples:
  asc iap offer-codes list --iap-id "IAP_ID"
  asc iap offer-codes get --offer-code-id "CODE_ID"
  asc iap offer-codes create --iap-id "IAP_ID" --name "SPRING" --prices "USA:PRICE_POINT_ID"
  asc iap offer-codes update --offer-code-id "CODE_ID" --active true`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPOfferCodesListCommand(),
			IAPOfferCodesGetCommand(),
			IAPOfferCodesCreateCommand(),
			IAPOfferCodesUpdateCommand(),
			IAPOfferCodesCustomCodesCommand(),
			IAPOfferCodesOneTimeCodesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// IAPOfferCodesListCommand returns the offer codes list subcommand.
func IAPOfferCodesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes list", flag.ExitOnError)

	iapID := fs.String("iap-id", "", "In-app purchase ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc iap offer-codes list --iap-id \"IAP_ID\"",
		ShortHelp:  "List offer codes for an in-app purchase.",
		LongHelp: `List offer codes for an in-app purchase.

Examples:
  asc iap offer-codes list --iap-id "IAP_ID"
  asc iap offer-codes list --iap-id "IAP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("iap offer-codes list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("iap offer-codes list: %w", err)
			}

			iapValue := strings.TrimSpace(*iapID)
			if iapValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("iap offer-codes list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.IAPOfferCodesOption{
				asc.WithIAPOfferCodesLimit(*limit),
				asc.WithIAPOfferCodesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithIAPOfferCodesLimit(200))
				firstPage, err := client.GetInAppPurchaseOfferCodes(requestCtx, iapValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("iap offer-codes list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetInAppPurchaseOfferCodes(ctx, iapValue, asc.WithIAPOfferCodesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap offer-codes list: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetInAppPurchaseOfferCodes(requestCtx, iapValue, opts...)
			if err != nil {
				return fmt.Errorf("iap offer-codes list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// IAPOfferCodesGetCommand returns the offer codes get subcommand.
func IAPOfferCodesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes get", flag.ExitOnError)

	offerCodeID := fs.String("offer-code-id", "", "Offer code ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc iap offer-codes get --offer-code-id \"CODE_ID\"",
		ShortHelp:  "Get an offer code by ID.",
		LongHelp: `Get an offer code by ID.

Examples:
  asc iap offer-codes get --offer-code-id "CODE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			offerCodeValue := strings.TrimSpace(*offerCodeID)
			if offerCodeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --offer-code-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("iap offer-codes get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetInAppPurchaseOfferCode(requestCtx, offerCodeValue)
			if err != nil {
				return fmt.Errorf("iap offer-codes get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// IAPOfferCodesCreateCommand returns the offer codes create subcommand.
func IAPOfferCodesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes create", flag.ExitOnError)

	iapID := fs.String("iap-id", "", "In-app purchase ID")
	name := fs.String("name", "", "Offer code name")
	eligibilities := fs.String("eligibilities", "", "Customer eligibilities (comma-separated)")
	prices := fs.String("prices", "", "Prices: TERRITORY:PRICE_POINT_ID entries")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc iap offer-codes create --iap-id \"IAP_ID\" --name \"SPRING\" --prices \"USA:PRICE_POINT_ID\"",
		ShortHelp:  "Create an offer code.",
		LongHelp: `Create an offer code.

Examples:
  asc iap offer-codes create --iap-id "IAP_ID" --name "SPRING" --prices "USA:PRICE_POINT_ID"
  asc iap offer-codes create --iap-id "IAP_ID" --name "SPRING" --eligibilities "NON_SPENDER" --prices "USA:PRICE_POINT_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			iapValue := strings.TrimSpace(*iapID)
			if iapValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id is required")
				return flag.ErrHelp
			}

			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}

			parsedEligibilities, err := parseOfferCodeEligibilities(*eligibilities)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}
			if len(parsedEligibilities) == 0 {
				parsedEligibilities = defaultOfferCodeEligibilities
			}

			priceEntries, err := parseOfferCodePrices(*prices)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}
			if len(priceEntries) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --prices is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("iap offer-codes create: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateInAppPurchaseOfferCode(requestCtx, iapValue, asc.InAppPurchaseOfferCodeCreateAttributes{
				Name:                  nameValue,
				CustomerEligibilities: parsedEligibilities,
				Prices:                priceEntries,
			})
			if err != nil {
				return fmt.Errorf("iap offer-codes create: failed to create: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// IAPOfferCodesUpdateCommand returns the offer codes update subcommand.
func IAPOfferCodesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes update", flag.ExitOnError)

	offerCodeID := fs.String("offer-code-id", "", "Offer code ID")
	var active shared.OptionalBool
	fs.Var(&active, "active", "Set active status: true or false")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc iap offer-codes update --offer-code-id \"CODE_ID\" --active true",
		ShortHelp:  "Update an offer code.",
		LongHelp: `Update an offer code.

Examples:
  asc iap offer-codes update --offer-code-id "CODE_ID" --active true`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			offerCodeValue := strings.TrimSpace(*offerCodeID)
			if offerCodeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --offer-code-id is required")
				return flag.ErrHelp
			}
			if !active.IsSet() {
				fmt.Fprintln(os.Stderr, "Error: --active is required (true or false)")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("iap offer-codes update: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			value := active.Value()
			resp, err := client.UpdateInAppPurchaseOfferCode(requestCtx, offerCodeValue, asc.InAppPurchaseOfferCodeUpdateAttributes{
				Active: &value,
			})
			if err != nil {
				return fmt.Errorf("iap offer-codes update: failed to update: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
