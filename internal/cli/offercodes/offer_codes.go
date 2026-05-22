package offercodes

import (
	"context"
	"encoding/csv"
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

// OfferCodesGenerateCommand returns the offer codes generate subcommand.
func OfferCodesGenerateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)

	offerCodeID := fs.String("offer-code-id", "", "Subscription offer code ID (required)")
	quantity := fs.Int("quantity", 0, "Number of one-time use codes to generate (required)")
	expirationDate := fs.String("expiration-date", "", "Expiration date (YYYY-MM-DD) (required)")
	outputPath := fs.String("output", "", "Output file path for offer codes")
	output := shared.BindMetadataOutputFlags(fs)

	return &ffcli.Command{
		Name:       "generate",
		ShortUsage: "asc offer-codes generate [flags]",
		ShortHelp:  "Generate one-time use offer codes for a subscription offer.",
		LongHelp: `Generate one-time use offer codes for a subscription offer.

Examples:
  asc offer-codes generate --offer-code-id "OFFER_CODE_ID" --quantity 10 --expiration-date "2026-02-01"
  asc offer-codes generate --offer-code-id "OFFER_CODE_ID" --quantity 10 --expiration-date "2026-02-01" --output "./offer-codes.txt"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedOfferCodeID := strings.TrimSpace(*offerCodeID)
			if trimmedOfferCodeID == "" {
				fmt.Fprintf(os.Stderr, "Error: --offer-code-id is required\n\n")
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
					} else if err := writeOfferCodesFile(*outputPath, codes, "text"); err != nil {
						writeErr = fmt.Errorf("offer-codes generate: %w", err)
					}
				}
			}

			if err := shared.PrintOutput(resp, *output.OutputFormat, *output.Pretty); err != nil {
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

	id := fs.String("batch-id", "", "One-time use offer code batch ID (required)")
	outputPath := fs.String("output", "", "Output file path for offer codes")
	outputFormat := fs.String("format", "text", "Output file format: text, csv")

	return &ffcli.Command{
		Name:       "values",
		ShortUsage: "asc offer-codes values [flags]",
		ShortHelp:  "Fetch one-time use offer code values for a batch.",
		LongHelp: `Fetch one-time use offer code values for a batch.

Examples:
  asc offer-codes values --batch-id "ONE_TIME_USE_CODE_ID"
  asc offer-codes values --batch-id "ONE_TIME_USE_CODE_ID" --output "./offer-codes.txt"
  asc offer-codes values --batch-id "ONE_TIME_USE_CODE_ID" --output "./offer-codes.csv" --format csv`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*id)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --batch-id is required")
				return flag.ErrHelp
			}
			format, err := normalizeOfferCodeValuesFormat(*outputFormat)
			if err != nil {
				return err
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
				if err := writeOfferCodesFile(*outputPath, codes, format); err != nil {
					return fmt.Errorf("offer-codes values: %w", err)
				}
				return nil
			}

			return writeOfferCodes(os.Stdout, codes, format)
		},
	}
}

func normalizeOfferCodeExpirationDate(value string) (string, error) {
	return shared.NormalizeDate(value, "--expiration-date")
}

func normalizeOfferCodeValuesFormat(value string) (string, error) {
	format := strings.ToLower(strings.TrimSpace(value))
	if format == "" {
		return "text", nil
	}
	switch format {
	case "text", "csv":
		return format, nil
	default:
		return "", shared.UsageError("--format must be text or csv")
	}
}

func writeOfferCodesFile(path string, codes []string, format string) error {
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

	if err := writeOfferCodes(file, codes, format); err != nil {
		return err
	}
	return file.Sync()
}

func writeOfferCodes(file *os.File, codes []string, format string) error {
	if format == "csv" {
		writer := csv.NewWriter(file)
		if err := writer.Write([]string{"code"}); err != nil {
			return err
		}
		for _, code := range codes {
			trimmed := strings.TrimSpace(code)
			if trimmed == "" {
				continue
			}
			if err := writer.Write([]string{trimmed}); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	}

	for _, code := range codes {
		trimmed := strings.TrimSpace(code)
		if trimmed == "" {
			continue
		}
		if _, err := fmt.Fprintln(file, trimmed); err != nil {
			return err
		}
	}
	return nil
}
