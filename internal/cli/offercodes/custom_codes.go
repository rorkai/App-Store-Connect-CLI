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
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "list",
		Name:        "list",
		ShortUsage:  "asc offer-codes custom-codes list [flags]",
		ShortHelp:   "List custom codes for a subscription offer.",
		LongHelp: `List custom codes for a subscription offer.

Examples:
  asc offer-codes custom-codes list --offer-code-id "OFFER_CODE_ID"
  asc offer-codes custom-codes list --offer-code-id "OFFER_CODE_ID" --limit 50
  asc offer-codes custom-codes list --offer-code-id "OFFER_CODE_ID" --paginate`,
		ParentFlag:  "offer-code-id",
		ParentUsage: "Subscription offer code ID (required)",
		LimitMax:    offerCodesMaxLimit,
		ErrorPrefix: "offer-codes custom-codes list",
		FetchPage: func(ctx context.Context, client *asc.Client, offerCodeID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.SubscriptionOfferCodeCustomCodesOption{
				asc.WithSubscriptionOfferCodeCustomCodesLimit(limit),
				asc.WithSubscriptionOfferCodeCustomCodesNextURL(next),
			}
			resp, err := client.GetSubscriptionOfferCodeCustomCodes(ctx, offerCodeID, opts...)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch: %w", err)
			}
			return resp, nil
		},
	})
}

// OfferCodeCustomCodesGetCommand returns the custom codes get subcommand.
func OfferCodeCustomCodesGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "get",
		Name:        "get",
		ShortUsage:  "asc offer-codes custom-codes get --custom-code-id ID",
		ShortHelp:   "Get a custom code by ID.",
		LongHelp: `Get a custom code by ID.

Examples:
  asc offer-codes custom-codes get --custom-code-id "CUSTOM_CODE_ID"`,
		IDFlag:      "custom-code-id",
		IDUsage:     "Custom code ID (required)",
		ErrorPrefix: "offer-codes custom-codes get",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetSubscriptionOfferCodeCustomCode(ctx, id)
		},
	})
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
				return shared.MissingRequiredUsageError()
			}

			trimmedCode := strings.TrimSpace(*code)
			if trimmedCode == "" {
				fmt.Fprintln(os.Stderr, "Error: --code is required")
				return shared.MissingRequiredUsageError()
			}

			if *quantity <= 0 {
				fmt.Fprintln(os.Stderr, "Error: --quantity is required")
				return shared.MissingRequiredUsageError()
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
	return newActiveUpdateCommand(activeUpdateCommandConfig{
		FlagSetName: "update",
		Name:        "update",
		ShortUsage:  "asc offer-codes custom-codes update [flags]",
		ShortHelp:   "Update a custom code.",
		LongHelp: `Update a custom code.

Examples:
  asc offer-codes custom-codes update --custom-code-id "CUSTOM_CODE_ID" --active false`,
		IDFlag:      "custom-code-id",
		IDUsage:     "Custom code ID (required)",
		ErrorPrefix: "offer-codes custom-codes update",
		Update: func(ctx context.Context, client *asc.Client, id string, active *bool) (any, error) {
			return client.UpdateSubscriptionOfferCodeCustomCode(ctx, id, asc.SubscriptionOfferCodeCustomCodeUpdateAttributes{Active: active})
		},
	})
}
