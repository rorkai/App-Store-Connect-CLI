package iap

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/offercodes"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
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
  asc iap offer-codes custom-codes view --custom-code-id "CUSTOM_CODE_ID"
  asc iap offer-codes custom-codes create --offer-code-id "OFFER_CODE_ID" --custom-code "SUMMER26" --quantity 100
  asc iap offer-codes custom-codes update --custom-code-id "CUSTOM_CODE_ID" --active false`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPOfferCodesCustomCodesListCommand(),
			IAPOfferCodesCustomCodesGetCommand(),
			IAPOfferCodesCustomCodesCreateCommand(),
			IAPOfferCodesCustomCodesUpdateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// IAPOfferCodesCustomCodesListCommand returns the custom codes list subcommand.
func IAPOfferCodesCustomCodesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes custom-codes list", flag.ExitOnError)

	offerCodeID := shared.BindResourceIDFlag(fs, "offer-code-id", "inAppPurchaseOfferCodes", "Offer code ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc iap offer-codes custom-codes list --offer-code-id \"OFFER_CODE_ID\" [flags]",
		ShortHelp:  "List custom codes for an offer code.",
		LongHelp: `List custom codes for an offer code.

Examples:
  asc iap offer-codes custom-codes list --offer-code-id "OFFER_CODE_ID"
  asc iap offer-codes custom-codes list --offer-code-id "OFFER_CODE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("iap offer-codes custom-codes list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("iap offer-codes custom-codes list: %v", err)
			}

			id := strings.TrimSpace(*offerCodeID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return shared.RequiredFlag("--offer-code-id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap offer-codes custom-codes list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.IAPOfferCodeCustomCodesOption{
				asc.Limit[asc.IAPOfferCodeCustomCodesOption](*limit),
				asc.NextURL[asc.IAPOfferCodeCustomCodesOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.IAPOfferCodeCustomCodesOption](200))
				firstPage, err := client.GetInAppPurchaseOfferCodeCustomCodes(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("iap offer-codes custom-codes list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetInAppPurchaseOfferCodeCustomCodes(ctx, id, asc.NextURL[asc.IAPOfferCodeCustomCodesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap offer-codes custom-codes list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetInAppPurchaseOfferCodeCustomCodes(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("iap offer-codes custom-codes list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// IAPOfferCodesCustomCodesGetCommand returns the custom codes get subcommand.
func IAPOfferCodesCustomCodesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes custom-codes view", flag.ExitOnError)

	customCodeID := shared.BindResourceIDFlag(fs, "custom-code-id", "inAppPurchaseOfferCodeCustomCodes", "Custom code ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc iap offer-codes custom-codes view --custom-code-id \"CUSTOM_CODE_ID\"",
		ShortHelp:  "View a custom code by ID.",
		LongHelp: `View a custom code by ID.

Examples:
  asc iap offer-codes custom-codes view --custom-code-id "CUSTOM_CODE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*customCodeID)
			if id == "" {
				return shared.RequiredFlag("--custom-code-id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap offer-codes custom-codes view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetInAppPurchaseOfferCodeCustomCode(requestCtx, id)
			if err != nil {
				return fmt.Errorf("iap offer-codes custom-codes view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// IAPOfferCodesCustomCodesCreateCommand returns the custom codes create subcommand.
func IAPOfferCodesCustomCodesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes custom-codes create", flag.ExitOnError)

	offerCodeID := shared.BindResourceIDFlag(fs, "offer-code-id", "inAppPurchaseOfferCodes", "Offer code ID (required)")
	customCode := fs.String("custom-code", "", "Custom code value (required)")
	quantity := fs.Int("quantity", 0, "Number of codes to create (required, positive integer)")
	expirationDate := fs.String("expiration-date", "", "Expiration date (YYYY-MM-DD)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc iap offer-codes custom-codes create --offer-code-id \"OFFER_CODE_ID\" --custom-code \"CODE\" --quantity N [flags]",
		ShortHelp:  "Create custom codes for an in-app purchase offer code.",
		LongHelp: `Create custom codes for an in-app purchase offer code.

Examples:
  asc iap offer-codes custom-codes create --offer-code-id "OFFER_CODE_ID" --custom-code "SUMMER26" --quantity 100
  asc iap offer-codes custom-codes create --offer-code-id "OFFER_CODE_ID" --custom-code "HOLIDAY2026" --quantity 50 --expiration-date "2026-12-31"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*offerCodeID)
			if id == "" {
				return shared.RequiredFlag("--offer-code-id")
			}

			code := strings.TrimSpace(*customCode)
			if code == "" {
				return shared.RequiredFlag("--custom-code")
			}

			if *quantity <= 0 {
				fmt.Fprintln(os.Stderr, "Error: --quantity must be a positive integer")
				return flag.ErrHelp
			}

			var normalizedExpiration *string
			if strings.TrimSpace(*expirationDate) != "" {
				normalized, err := shared.NormalizeDate(*expirationDate, "--expiration-date")
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err)
					return flag.ErrHelp
				}
				normalizedExpiration = &normalized
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap offer-codes custom-codes create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			req := asc.InAppPurchaseOfferCodeCustomCodeCreateRequest{
				Data: asc.InAppPurchaseOfferCodeCustomCodeCreateData{
					Type: asc.ResourceTypeInAppPurchaseOfferCodeCustomCodes,
					Attributes: asc.InAppPurchaseOfferCodeCustomCodeCreateAttributes{
						CustomCode:     code,
						NumberOfCodes:  *quantity,
						ExpirationDate: normalizedExpiration,
					},
					Relationships: asc.InAppPurchaseOfferCodeCustomCodeCreateRelationships{
						OfferCode: asc.Relationship{
							Data: asc.ResourceData{
								Type: asc.ResourceTypeInAppPurchaseOfferCodes,
								ID:   id,
							},
						},
					},
				},
			}

			resp, err := client.CreateInAppPurchaseOfferCodeCustomCode(requestCtx, req)
			if err != nil {
				return fmt.Errorf("iap offer-codes custom-codes create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// IAPOfferCodesCustomCodesUpdateCommand returns the custom codes update subcommand.
func IAPOfferCodesCustomCodesUpdateCommand() *ffcli.Command {
	return offercodes.NewActiveUpdateCommand(offercodes.ActiveUpdateCommandConfig{
		FlagSetName: "offer-codes custom-codes update",
		Name:        "update",
		ShortUsage:  "asc iap offer-codes custom-codes update --custom-code-id \"CUSTOM_CODE_ID\" --active false",
		ShortHelp:   "Turn a custom code on or off.",
		LongHelp: `Turn a custom code on or off.

Examples:
  asc iap offer-codes custom-codes update --custom-code-id "CUSTOM_CODE_ID" --active false`,
		IDFlag:      "custom-code-id",
		IDUsage:     "Custom code ID (required)",
		IDType:      "inAppPurchaseOfferCodeCustomCodes",
		ErrorPrefix: "iap offer-codes custom-codes update",
		Update: func(ctx context.Context, client *asc.Client, id string, active *bool) (any, error) {
			return client.UpdateInAppPurchaseOfferCodeCustomCode(ctx, id, asc.InAppPurchaseOfferCodeCustomCodeUpdateAttributes{Active: active})
		},
	})
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
  asc iap offer-codes one-time-codes view --one-time-code-id "ONE_TIME_USE_CODE_ID"
  asc iap offer-codes one-time-codes create --offer-code-id "OFFER_CODE_ID" --quantity 100 --expiration-date "2026-12-31"
  asc iap offer-codes one-time-codes update --one-time-code-id "ONE_TIME_USE_CODE_ID" --active false --confirm
  asc iap offer-codes one-time-codes values --one-time-code-id "ONE_TIME_USE_CODE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPOfferCodesOneTimeCodesListCommand(),
			IAPOfferCodesOneTimeCodesGetCommand(),
			IAPOfferCodesOneTimeCodesCreateCommand(),
			IAPOfferCodesOneTimeCodesUpdateCommand(),
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

	offerCodeID := shared.BindResourceIDFlag(fs, "offer-code-id", "inAppPurchaseOfferCodes", "Offer code ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc iap offer-codes one-time-codes list --offer-code-id \"OFFER_CODE_ID\" [flags]",
		ShortHelp:  "List one-time use code batches for an offer code.",
		LongHelp: `List one-time use code batches for an offer code.

Examples:
  asc iap offer-codes one-time-codes list --offer-code-id "OFFER_CODE_ID"
  asc iap offer-codes one-time-codes list --offer-code-id "OFFER_CODE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("iap offer-codes one-time-codes list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("iap offer-codes one-time-codes list: %v", err)
			}

			id := strings.TrimSpace(*offerCodeID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return shared.RequiredFlag("--offer-code-id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap offer-codes one-time-codes list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.IAPOfferCodeOneTimeUseCodesOption{
				asc.Limit[asc.IAPOfferCodeOneTimeUseCodesOption](*limit),
				asc.NextURL[asc.IAPOfferCodeOneTimeUseCodesOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.IAPOfferCodeOneTimeUseCodesOption](200))
				firstPage, err := client.GetInAppPurchaseOfferCodeOneTimeUseCodes(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("iap offer-codes one-time-codes list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetInAppPurchaseOfferCodeOneTimeUseCodes(ctx, id, asc.NextURL[asc.IAPOfferCodeOneTimeUseCodesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap offer-codes one-time-codes list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetInAppPurchaseOfferCodeOneTimeUseCodes(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("iap offer-codes one-time-codes list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// IAPOfferCodesOneTimeCodesGetCommand returns the one-time codes get subcommand.
func IAPOfferCodesOneTimeCodesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes one-time-codes view", flag.ExitOnError)

	oneTimeCodeID := shared.BindResourceIDFlag(fs, "one-time-code-id", "inAppPurchaseOfferCodeOneTimeUseCodes", "One-time use code batch ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc iap offer-codes one-time-codes view --one-time-code-id \"ONE_TIME_USE_CODE_ID\"",
		ShortHelp:  "View a one-time use code batch by ID.",
		LongHelp: `View a one-time use code batch by ID.

Examples:
  asc iap offer-codes one-time-codes view --one-time-code-id "ONE_TIME_USE_CODE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*oneTimeCodeID)
			if id == "" {
				return shared.RequiredFlag("--one-time-code-id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap offer-codes one-time-codes view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetInAppPurchaseOfferCodeOneTimeUseCode(requestCtx, id)
			if err != nil {
				return fmt.Errorf("iap offer-codes one-time-codes view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// IAPOfferCodesOneTimeCodesCreateCommand returns the one-time codes create subcommand.
func IAPOfferCodesOneTimeCodesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes one-time-codes create", flag.ExitOnError)

	offerCodeID := shared.BindResourceIDFlag(fs, "offer-code-id", "inAppPurchaseOfferCodes", "Offer code ID (required)")
	quantity := fs.Int("quantity", 0, "Number of codes to generate (required, positive integer)")
	expirationDate := fs.String("expiration-date", "", "Expiration date (YYYY-MM-DD) (required)")
	environment := fs.String("environment", "", "Offer code environment: PRODUCTION or SANDBOX")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc iap offer-codes one-time-codes create --offer-code-id \"OFFER_CODE_ID\" --quantity N --expiration-date \"YYYY-MM-DD\" [--environment PRODUCTION|SANDBOX] [flags]",
		ShortHelp:  "Generate one-time use codes for an in-app purchase offer code.",
		LongHelp: `Generate one-time use codes for an in-app purchase offer code.

Examples:
  asc iap offer-codes one-time-codes create --offer-code-id "OFFER_CODE_ID" --quantity 100 --expiration-date "2026-12-31"
  asc iap offer-codes one-time-codes create --offer-code-id "OFFER_CODE_ID" --quantity 500 --expiration-date "2026-09-30"
  asc iap offer-codes one-time-codes create --offer-code-id "OFFER_CODE_ID" --quantity 100 --expiration-date "2026-12-31" --environment SANDBOX`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*offerCodeID)
			if id == "" {
				return shared.RequiredFlag("--offer-code-id")
			}

			if *quantity <= 0 {
				fmt.Fprintln(os.Stderr, "Error: --quantity must be a positive integer")
				return flag.ErrHelp
			}

			normalizedExpiration, err := shared.NormalizeDate(*expirationDate, "--expiration-date")
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				return flag.ErrHelp
			}
			normalizedEnvironment, err := shared.NormalizeOfferCodeEnvironment(*environment)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap offer-codes one-time-codes create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			req := asc.InAppPurchaseOfferCodeOneTimeUseCodeCreateRequest{
				Data: asc.InAppPurchaseOfferCodeOneTimeUseCodeCreateData{
					Type: asc.ResourceTypeInAppPurchaseOfferCodeOneTimeUseCodes,
					Attributes: asc.InAppPurchaseOfferCodeOneTimeUseCodeCreateAttributes{
						NumberOfCodes:  *quantity,
						ExpirationDate: normalizedExpiration,
						Environment:    normalizedEnvironment,
					},
					Relationships: asc.InAppPurchaseOfferCodeOneTimeUseCodeCreateRelationships{
						OfferCode: asc.Relationship{
							Data: asc.ResourceData{
								Type: asc.ResourceTypeInAppPurchaseOfferCodes,
								ID:   id,
							},
						},
					},
				},
			}

			resp, err := client.CreateInAppPurchaseOfferCodeOneTimeUseCode(requestCtx, req)
			if err != nil {
				return fmt.Errorf("iap offer-codes one-time-codes create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// IAPOfferCodesOneTimeCodesUpdateCommand returns the one-time codes update subcommand.
func IAPOfferCodesOneTimeCodesUpdateCommand() *ffcli.Command {
	return offercodes.NewActiveUpdateCommand(offercodes.ActiveUpdateCommandConfig{
		FlagSetName: "offer-codes one-time-codes update",
		Name:        "update",
		ShortUsage:  "asc iap offer-codes one-time-codes update --one-time-code-id \"ONE_TIME_USE_CODE_ID\" --active false --confirm",
		ShortHelp:   "Turn off a one-time use code batch.",
		LongHelp: `Turn off a one-time use code batch.

Turning a batch off is permanent: Apple rejects --active true on an inactive
batch, so --active false requires --confirm.

Examples:
  asc iap offer-codes one-time-codes update --one-time-code-id "ONE_TIME_USE_CODE_ID" --active false --confirm`,
		IDFlag:      "one-time-code-id",
		IDUsage:     "One-time use code batch ID (required)",
		IDType:      "inAppPurchaseOfferCodeOneTimeUseCodes",
		ErrorPrefix: "iap offer-codes one-time-codes update",
		ConfirmOff:  "a one-time use code batch",
		Update: func(ctx context.Context, client *asc.Client, id string, active *bool) (any, error) {
			return client.UpdateInAppPurchaseOfferCodeOneTimeUseCode(ctx, id, asc.InAppPurchaseOfferCodeOneTimeUseCodeUpdateAttributes{Active: active})
		},
	})
}

// IAPOfferCodesOneTimeCodesValuesCommand returns the one-time code values subcommand.
func IAPOfferCodesOneTimeCodesValuesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes one-time-codes values", flag.ExitOnError)

	oneTimeCodeID := shared.BindResourceIDFlag(fs, "one-time-code-id", "inAppPurchaseOfferCodeOneTimeUseCodes", "One-time use code batch ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "values",
		ShortUsage: "asc iap offer-codes one-time-codes values --one-time-code-id \"ONE_TIME_USE_CODE_ID\"",
		ShortHelp:  "Fetch one-time use offer code values for a batch.",
		LongHelp: `Fetch one-time use offer code values for a batch.

Examples:
  asc iap offer-codes one-time-codes values --one-time-code-id "ONE_TIME_USE_CODE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*oneTimeCodeID)
			if id == "" {
				return shared.RequiredFlag("--one-time-code-id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap offer-codes one-time-codes values: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			values, err := client.GetInAppPurchaseOfferCodeOneTimeUseCodeValues(requestCtx, id)
			if err != nil {
				return fmt.Errorf("iap offer-codes one-time-codes values: failed to fetch: %w", err)
			}

			result := &asc.OfferCodeValuesResult{Codes: values}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
