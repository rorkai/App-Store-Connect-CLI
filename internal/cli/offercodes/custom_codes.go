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

// OfferCodeCustomCodesCommand returns the custom codes command group.
func OfferCodeCustomCodesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-codes", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "custom-codes",
		ShortUsage: "asc offer-codes custom-codes <subcommand> [flags]",
		ShortHelp:  "Manage custom offer codes.",
		LongHelp: `Manage custom offer codes.

Examples:
  asc offer-codes custom-codes list --offer-code-id "OFFER_CODE_ID"
  asc offer-codes custom-codes get --custom-code-id "CUSTOM_CODE_ID"
  asc offer-codes custom-codes create --offer-code-id "OFFER_CODE_ID" --code "SPRING2026" --quantity 10
  asc offer-codes custom-codes update --custom-code-id "CUSTOM_CODE_ID" --active false`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			OfferCodeCustomCodesListCommand(),
			OfferCodeCustomCodesGetCommand(),
			OfferCodeCustomCodesCreateCommand(),
			OfferCodeCustomCodesUpdateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// OfferCodeCustomCodesListCommand returns the custom codes list subcommand.
func OfferCodeCustomCodesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	offerCodeID := fs.String("offer-code-id", "", "Subscription offer code ID (required)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc offer-codes custom-codes list [flags]",
		ShortHelp:  "List custom codes for a subscription offer.",
		LongHelp: `List custom codes for a subscription offer.

Examples:
  asc offer-codes custom-codes list --offer-code-id "OFFER_CODE_ID"
  asc offer-codes custom-codes list --offer-code-id "OFFER_CODE_ID" --limit 50
  asc offer-codes custom-codes list --offer-code-id "OFFER_CODE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > offerCodesMaxLimit) {
				return fmt.Errorf("offer-codes custom-codes list: --limit must be between 1 and %d", offerCodesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("offer-codes custom-codes list: %w", err)
			}

			trimmedOfferCodeID := strings.TrimSpace(*offerCodeID)
			if trimmedOfferCodeID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --offer-code-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("offer-codes custom-codes list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionOfferCodeCustomCodesOption{
				asc.WithSubscriptionOfferCodeCustomCodesLimit(*limit),
				asc.WithSubscriptionOfferCodeCustomCodesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionOfferCodeCustomCodesLimit(offerCodesMaxLimit))
				firstPage, err := client.GetSubscriptionOfferCodeCustomCodes(requestCtx, trimmedOfferCodeID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("offer-codes custom-codes list: failed to fetch: %w", err)
				}

				pages, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionOfferCodeCustomCodes(ctx, trimmedOfferCodeID, asc.WithSubscriptionOfferCodeCustomCodesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("offer-codes custom-codes list: %w", err)
				}

				return shared.PrintOutput(pages, *output.Output, *output.Pretty)
			}

			resp, err := client.GetSubscriptionOfferCodeCustomCodes(requestCtx, trimmedOfferCodeID, opts...)
			if err != nil {
				return fmt.Errorf("offer-codes custom-codes list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// OfferCodeCustomCodesGetCommand returns the custom codes get subcommand.
func OfferCodeCustomCodesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	customCodeID := fs.String("custom-code-id", "", "Custom code ID (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc offer-codes custom-codes get --custom-code-id ID",
		ShortHelp:  "Get a custom code by ID.",
		LongHelp: `Get a custom code by ID.

Examples:
  asc offer-codes custom-codes get --custom-code-id "CUSTOM_CODE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*customCodeID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --custom-code-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("offer-codes custom-codes get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionOfferCodeCustomCode(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("offer-codes custom-codes get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// OfferCodeCustomCodesCreateCommand returns the custom codes create subcommand.
func OfferCodeCustomCodesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	offerCodeID := fs.String("offer-code-id", "", "Subscription offer code ID (required)")
	code := fs.String("code", "", "Custom code value (required)")
	quantity := fs.Int("quantity", 0, "Number of codes to create (required)")
	expirationDate := fs.String("expiration-date", "", "Expiration date (YYYY-MM-DD)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc offer-codes custom-codes create [flags]",
		ShortHelp:  "Create custom codes for a subscription offer.",
		LongHelp: `Create custom codes for a subscription offer.

Examples:
  asc offer-codes custom-codes create --offer-code-id "OFFER_CODE_ID" --code "SPRING2026" --quantity 10
  asc offer-codes custom-codes create --offer-code-id "OFFER_CODE_ID" --code "SPRING2026" --quantity 10 --expiration-date "2026-02-01"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedOfferCodeID := strings.TrimSpace(*offerCodeID)
			if trimmedOfferCodeID == "" {
				fmt.Fprintln(os.Stderr, "Error: --offer-code-id is required")
				return flag.ErrHelp
			}

			trimmedCode := strings.TrimSpace(*code)
			if trimmedCode == "" {
				fmt.Fprintln(os.Stderr, "Error: --code is required")
				return flag.ErrHelp
			}

			if *quantity <= 0 {
				fmt.Fprintln(os.Stderr, "Error: --quantity is required")
				return flag.ErrHelp
			}

			var normalizedExpiration *string
			if strings.TrimSpace(*expirationDate) != "" {
				normalized, err := normalizeOfferCodeExpirationDate(*expirationDate)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err)
					return flag.ErrHelp
				}
				normalizedExpiration = &normalized
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("offer-codes custom-codes create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			req := asc.SubscriptionOfferCodeCustomCodeCreateRequest{
				Data: asc.SubscriptionOfferCodeCustomCodeCreateData{
					Type: asc.ResourceTypeSubscriptionOfferCodeCustomCodes,
					Attributes: asc.SubscriptionOfferCodeCustomCodeCreateAttributes{
						CustomCode:     trimmedCode,
						NumberOfCodes:  *quantity,
						ExpirationDate: normalizedExpiration,
					},
					Relationships: asc.SubscriptionOfferCodeCustomCodeCreateRelationships{
						OfferCode: asc.Relationship{
							Data: asc.ResourceData{
								Type: asc.ResourceTypeSubscriptionOfferCodes,
								ID:   trimmedOfferCodeID,
							},
						},
					},
				},
			}

			resp, err := client.CreateSubscriptionOfferCodeCustomCode(requestCtx, req)
			if err != nil {
				return fmt.Errorf("offer-codes custom-codes create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// OfferCodeCustomCodesUpdateCommand returns the custom codes update subcommand.
func OfferCodeCustomCodesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	customCodeID := fs.String("custom-code-id", "", "Custom code ID (required)")
	active := fs.String("active", "", "Set active (true/false)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc offer-codes custom-codes update [flags]",
		ShortHelp:  "Update a custom code.",
		LongHelp: `Update a custom code.

Examples:
  asc offer-codes custom-codes update --custom-code-id "CUSTOM_CODE_ID" --active false`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*customCodeID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --custom-code-id is required")
				return flag.ErrHelp
			}

			activeValue, err := shared.ParseOptionalBoolFlag("--active", *active)
			if err != nil {
				return fmt.Errorf("offer-codes custom-codes update: %w", err)
			}
			if activeValue == nil {
				fmt.Fprintln(os.Stderr, "Error: --active is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("offer-codes custom-codes update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateSubscriptionOfferCodeCustomCode(requestCtx, trimmedID, asc.SubscriptionOfferCodeCustomCodeUpdateAttributes{Active: activeValue})
			if err != nil {
				return fmt.Errorf("offer-codes custom-codes update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
