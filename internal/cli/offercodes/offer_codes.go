package offercodes

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const offerCodesMaxLimit = 200

// OfferCodesCommand returns the offer codes command with subcommands.
func OfferCodesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offer-codes", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "offer-codes",
		ShortUsage: "asc offer-codes <subcommand> [flags]",
		ShortHelp:  "Manage subscription offer codes.",
		LongHelp: `Manage subscription offer codes and related custom or one-time use codes.

Examples:
  asc offer-codes get --offer-code-id "OFFER_CODE_ID"
  asc offer-codes create --subscription-id "SUB_ID" --name "SPRING" --customer-eligibilities NEW --offer-eligibility STACK_WITH_INTRO_OFFERS --duration ONE_MONTH --offer-mode PAY_AS_YOU_GO --number-of-periods 1 --prices "USA:PRICE_POINT_ID"
  asc offer-codes update --offer-code-id "OFFER_CODE_ID" --active true
  asc offer-codes custom-codes list --offer-code-id "OFFER_CODE_ID"
  asc offer-codes prices list --offer-code-id "OFFER_CODE_ID"
  asc offer-codes list --offer-code "OFFER_CODE_ID"
  asc offer-codes generate --offer-code "OFFER_CODE_ID" --quantity 10 --expiration-date "2026-02-01"
  asc offer-codes values --id "ONE_TIME_USE_CODE_ID" --output "./offer-codes.txt"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			OfferCodesGetCommand(),
			OfferCodesCreateCommand(),
			OfferCodesUpdateCommand(),
			OfferCodeCustomCodesCommand(),
			OfferCodePricesCommand(),
			OfferCodesListCommand(),
			OfferCodesGenerateCommand(),
			OfferCodesValuesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// OfferCodesListCommand returns the offer codes list subcommand.
func OfferCodesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	offerCodeID := fs.String("offer-code", "", "Subscription offer code ID (required)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc offer-codes list [flags]",
		ShortHelp:  "List one-time use offer code batches for a subscription offer.",
		LongHelp: `List one-time use offer code batches for a subscription offer.

Examples:
  asc offer-codes list --offer-code "OFFER_CODE_ID"
  asc offer-codes list --offer-code "OFFER_CODE_ID" --limit 10
  asc offer-codes list --offer-code "OFFER_CODE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > offerCodesMaxLimit) {
				return fmt.Errorf("offer-codes list: --limit must be between 1 and %d", offerCodesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("offer-codes list: %w", err)
			}

			trimmedOfferCodeID := strings.TrimSpace(*offerCodeID)
			if trimmedOfferCodeID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: --offer-code is required\n\n")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("offer-codes list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionOfferCodeOneTimeUseCodesOption{
				asc.WithSubscriptionOfferCodeOneTimeUseCodesLimit(*limit),
				asc.WithSubscriptionOfferCodeOneTimeUseCodesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionOfferCodeOneTimeUseCodesLimit(offerCodesMaxLimit))
				firstPage, err := client.GetSubscriptionOfferCodeOneTimeUseCodes(requestCtx, trimmedOfferCodeID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("offer-codes list: failed to fetch: %w", err)
				}

				pages, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionOfferCodeOneTimeUseCodes(ctx, trimmedOfferCodeID, asc.WithSubscriptionOfferCodeOneTimeUseCodesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("offer-codes list: %w", err)
				}

				return shared.PrintOutput(pages, *output, *pretty)
			}

			resp, err := client.GetSubscriptionOfferCodeOneTimeUseCodes(requestCtx, trimmedOfferCodeID, opts...)
			if err != nil {
				return fmt.Errorf("offer-codes list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// OfferCodesGenerateCommand returns the offer codes generate subcommand.
func OfferCodesGenerateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)

	offerCodeID := fs.String("offer-code", "", "Subscription offer code ID (required)")
	quantity := fs.Int("quantity", 0, "Number of one-time use codes to generate (required)")
	expirationDate := fs.String("expiration-date", "", "Expiration date (YYYY-MM-DD) (required)")
	outputPath := fs.String("output", "", "Output file path for offer codes (one per line)")
	outputFormat := fs.String("output-format", "json", "Output format for metadata: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "generate",
		ShortUsage: "asc offer-codes generate [flags]",
		ShortHelp:  "Generate one-time use offer codes for a subscription offer.",
		LongHelp: `Generate one-time use offer codes for a subscription offer.

Examples:
  asc offer-codes generate --offer-code "OFFER_CODE_ID" --quantity 10 --expiration-date "2026-02-01"
  asc offer-codes generate --offer-code "OFFER_CODE_ID" --quantity 10 --expiration-date "2026-02-01" --output "./offer-codes.txt"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedOfferCodeID := strings.TrimSpace(*offerCodeID)
			if trimmedOfferCodeID == "" {
				fmt.Fprintf(os.Stderr, "Error: --offer-code is required\n\n")
				return flag.ErrHelp
			}
			if *quantity <= 0 {
				fmt.Fprintln(os.Stderr, "Error: --quantity is required")
				return flag.ErrHelp
			}
			normalizedExpirationDate, err := normalizeOfferCodeExpirationDate(*expirationDate)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("offer-codes generate: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			req := asc.SubscriptionOfferCodeOneTimeUseCodeCreateRequest{
				Data: asc.SubscriptionOfferCodeOneTimeUseCodeCreateData{
					Type: asc.ResourceTypeSubscriptionOfferCodeOneTimeUseCodes,
					Attributes: asc.SubscriptionOfferCodeOneTimeUseCodeCreateAttributes{
						NumberOfCodes:  *quantity,
						ExpirationDate: normalizedExpirationDate,
					},
					Relationships: asc.SubscriptionOfferCodeOneTimeUseCodeCreateRelationships{
						OfferCode: asc.Relationship{
							Data: asc.ResourceData{
								Type: asc.ResourceTypeSubscriptionOfferCodes,
								ID:   trimmedOfferCodeID,
							},
						},
					},
				},
			}

			resp, err := client.CreateSubscriptionOfferCodeOneTimeUseCode(requestCtx, req)
			if err != nil {
				return fmt.Errorf("offer-codes generate: failed to generate: %w", err)
			}

			var writeErr error
			if strings.TrimSpace(*outputPath) != "" {
				batchID := strings.TrimSpace(resp.Data.ID)
				if batchID == "" {
					writeErr = fmt.Errorf("offer-codes generate: missing one-time use code batch ID")
				} else {
					codes, err := client.GetSubscriptionOfferCodeOneTimeUseCodeValues(requestCtx, batchID)
					if err != nil {
						writeErr = fmt.Errorf("offer-codes generate: failed to fetch values: %w", err)
					} else if len(codes) == 0 {
						writeErr = fmt.Errorf("offer-codes generate: no codes returned to write")
					} else if err := writeOfferCodesFile(*outputPath, codes); err != nil {
						writeErr = fmt.Errorf("offer-codes generate: %w", err)
					}
				}
			}

			if err := shared.PrintOutput(resp, *outputFormat, *pretty); err != nil {
				return err
			}
			if writeErr != nil {
				return writeErr
			}
			return nil
		},
	}
}

// OfferCodesValuesCommand returns the offer codes values subcommand.
func OfferCodesValuesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("values", flag.ExitOnError)

	id := fs.String("id", "", "One-time use offer code batch ID (required)")
	outputPath := fs.String("output", "", "Output file path for offer codes (one per line)")

	return &ffcli.Command{
		Name:       "values",
		ShortUsage: "asc offer-codes values [flags]",
		ShortHelp:  "Fetch one-time use offer code values for a batch.",
		LongHelp: `Fetch one-time use offer code values for a batch.

Examples:
  asc offer-codes values --id "ONE_TIME_USE_CODE_ID"
  asc offer-codes values --id "ONE_TIME_USE_CODE_ID" --output "./offer-codes.txt"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*id)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("offer-codes values: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			codes, err := client.GetSubscriptionOfferCodeOneTimeUseCodeValues(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("offer-codes values: failed to fetch: %w", err)
			}
			if len(codes) == 0 {
				return fmt.Errorf("offer-codes values: no codes returned")
			}

			if strings.TrimSpace(*outputPath) != "" {
				if err := writeOfferCodesFile(*outputPath, codes); err != nil {
					return fmt.Errorf("offer-codes values: %w", err)
				}
				return nil
			}

			for _, code := range codes {
				trimmed := strings.TrimSpace(code)
				if trimmed == "" {
					continue
				}
				if _, err := fmt.Fprintln(os.Stdout, trimmed); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func normalizeOfferCodeExpirationDate(value string) (string, error) {
	return shared.NormalizeDate(value, "--expiration-date")
}

func writeOfferCodesFile(path string, codes []string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := shared.OpenNewFileNoFollow(path, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("output file already exists: %w", err)
		}
		return err
	}
	defer file.Close()

	for _, code := range codes {
		trimmed := strings.TrimSpace(code)
		if trimmed == "" {
			continue
		}
		if _, err := fmt.Fprintln(file, trimmed); err != nil {
			return err
		}
	}
	return file.Sync()
}
