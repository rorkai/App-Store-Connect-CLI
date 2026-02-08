package reviews

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

// ReviewDetailsGetCommand returns the review details get subcommand.
func ReviewDetailsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("details-get", flag.ExitOnError)

	detailID := fs.String("id", "", "App Store review detail ID (required)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "details-get",
		ShortUsage: "asc review details-get --id \"DETAIL_ID\"",
		ShortHelp:  "Get an App Store review detail by ID.",
		LongHelp: `Get an App Store review detail by ID.

Examples:
  asc review details-get --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			detailValue := strings.TrimSpace(*detailID)
			if detailValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("review details-get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppStoreReviewDetail(requestCtx, detailValue)
			if err != nil {
				return fmt.Errorf("review details-get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// ReviewDetailsForVersionCommand returns the review details for-version subcommand.
func ReviewDetailsForVersionCommand() *ffcli.Command {
	fs := flag.NewFlagSet("details-for-version", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (required)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "details-for-version",
		ShortUsage: "asc review details-for-version --version-id \"VERSION_ID\"",
		ShortHelp:  "Get the review detail for a version.",
		LongHelp: `Get the review detail for a specific App Store version.

Examples:
  asc review details-for-version --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			versionValue := strings.TrimSpace(*versionID)
			if versionValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("review details-for-version: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppStoreReviewDetailForVersion(requestCtx, versionValue)
			if err != nil {
				return fmt.Errorf("review details-for-version: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// ReviewDetailsCreateCommand returns the review details create subcommand.
func ReviewDetailsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("details-create", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (required)")
	contactFirstName := fs.String("contact-first-name", "", "Contact first name")
	contactLastName := fs.String("contact-last-name", "", "Contact last name")
	contactEmail := fs.String("contact-email", "", "Contact email")
	contactPhone := fs.String("contact-phone", "", "Contact phone")
	demoAccountName := fs.String("demo-account-name", "", "Demo account name")
	demoAccountPassword := fs.String("demo-account-password", "", "Demo account password")
	demoAccountRequired := fs.Bool("demo-account-required", false, "Demo account required")
	notes := fs.String("notes", "", "Review notes")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "details-create",
		ShortUsage: "asc review details-create --version-id \"VERSION_ID\" [flags]",
		ShortHelp:  "Create App Store review details for a version.",
		LongHelp: `Create App Store review details for a version.

Examples:
  asc review details-create --version-id "VERSION_ID" --contact-email "dev@example.com"
  asc review details-create --version-id "VERSION_ID" --notes "Review notes"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			versionValue := strings.TrimSpace(*versionID)
			if versionValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			var attrsPtr *asc.AppStoreReviewDetailCreateAttributes
			if hasReviewDetailUpdates(visited) {
				attrs := asc.AppStoreReviewDetailCreateAttributes{}
				if visited["contact-first-name"] {
					value := strings.TrimSpace(*contactFirstName)
					attrs.ContactFirstName = &value
				}
				if visited["contact-last-name"] {
					value := strings.TrimSpace(*contactLastName)
					attrs.ContactLastName = &value
				}
				if visited["contact-email"] {
					value := strings.TrimSpace(*contactEmail)
					attrs.ContactEmail = &value
				}
				if visited["contact-phone"] {
					value := strings.TrimSpace(*contactPhone)
					attrs.ContactPhone = &value
				}
				if visited["demo-account-name"] {
					value := strings.TrimSpace(*demoAccountName)
					attrs.DemoAccountName = &value
				}
				if visited["demo-account-password"] {
					value := strings.TrimSpace(*demoAccountPassword)
					attrs.DemoAccountPassword = &value
				}
				if visited["demo-account-required"] {
					value := *demoAccountRequired
					attrs.DemoAccountRequired = &value
				}
				if visited["notes"] {
					value := strings.TrimSpace(*notes)
					attrs.Notes = &value
				}
				attrsPtr = &attrs
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("review details-create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAppStoreReviewDetail(requestCtx, versionValue, attrsPtr)
			if err != nil {
				return fmt.Errorf("review details-create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// ReviewDetailsUpdateCommand returns the review details update subcommand.
func ReviewDetailsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("details-update", flag.ExitOnError)

	detailID := fs.String("id", "", "App Store review detail ID (required)")
	contactFirstName := fs.String("contact-first-name", "", "Contact first name")
	contactLastName := fs.String("contact-last-name", "", "Contact last name")
	contactEmail := fs.String("contact-email", "", "Contact email")
	contactPhone := fs.String("contact-phone", "", "Contact phone")
	demoAccountName := fs.String("demo-account-name", "", "Demo account name")
	demoAccountPassword := fs.String("demo-account-password", "", "Demo account password")
	demoAccountRequired := fs.Bool("demo-account-required", false, "Demo account required")
	notes := fs.String("notes", "", "Review notes")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "details-update",
		ShortUsage: "asc review details-update --id \"DETAIL_ID\" [flags]",
		ShortHelp:  "Update App Store review details.",
		LongHelp: `Update App Store review details.

Examples:
  asc review details-update --id "DETAIL_ID" --contact-email "dev@example.com"
  asc review details-update --id "DETAIL_ID" --notes "Updated review notes"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			detailValue := strings.TrimSpace(*detailID)
			if detailValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})

			if !hasReviewDetailUpdates(visited) {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			attrs := asc.AppStoreReviewDetailUpdateAttributes{}
			if visited["contact-first-name"] {
				value := strings.TrimSpace(*contactFirstName)
				attrs.ContactFirstName = &value
			}
			if visited["contact-last-name"] {
				value := strings.TrimSpace(*contactLastName)
				attrs.ContactLastName = &value
			}
			if visited["contact-email"] {
				value := strings.TrimSpace(*contactEmail)
				attrs.ContactEmail = &value
			}
			if visited["contact-phone"] {
				value := strings.TrimSpace(*contactPhone)
				attrs.ContactPhone = &value
			}
			if visited["demo-account-name"] {
				value := strings.TrimSpace(*demoAccountName)
				attrs.DemoAccountName = &value
			}
			if visited["demo-account-password"] {
				value := strings.TrimSpace(*demoAccountPassword)
				attrs.DemoAccountPassword = &value
			}
			if visited["demo-account-required"] {
				value := *demoAccountRequired
				attrs.DemoAccountRequired = &value
			}
			if visited["notes"] {
				value := strings.TrimSpace(*notes)
				attrs.Notes = &value
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("review details-update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppStoreReviewDetail(requestCtx, detailValue, attrs)
			if err != nil {
				return fmt.Errorf("review details-update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

func hasReviewDetailUpdates(visited map[string]bool) bool {
	return visited["contact-first-name"] ||
		visited["contact-last-name"] ||
		visited["contact-email"] ||
		visited["contact-phone"] ||
		visited["demo-account-name"] ||
		visited["demo-account-password"] ||
		visited["demo-account-required"] ||
		visited["notes"]
}
