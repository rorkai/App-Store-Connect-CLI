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

const (
	reviewDetailDemoAccountNameUsage     = "Demo account name when demo credentials are required"
	reviewDetailDemoAccountPasswordUsage = "Demo account password when demo credentials are required"
	reviewDetailDemoAccountRequiredUsage = "Set true only when App Review needs demo credentials; leave false when reviewer guidance in --notes is enough"
	reviewDetailNotesUsage               = "Review notes for reviewer instructions or context; supplemental when demo credentials are required"
	reviewDetailDemoCredentialsError     = "Error: --demo-account-required=true requires both --demo-account-name and --demo-account-password"
)

// ReviewDetailsGetCommand returns the review details get subcommand.
func ReviewDetailsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("details-get", flag.ExitOnError)

	detailID := fs.String("id", "", "App Store review detail ID (required)")
	output := shared.BindOutputFlags(fs)

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

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ReviewDetailsForVersionCommand returns the review details for-version subcommand.
func ReviewDetailsForVersionCommand() *ffcli.Command {
	fs := flag.NewFlagSet("details-for-version", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (required)")
	output := shared.BindOutputFlags(fs)

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

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
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
	demoAccountName := fs.String("demo-account-name", "", reviewDetailDemoAccountNameUsage)
	demoAccountPassword := fs.String("demo-account-password", "", reviewDetailDemoAccountPasswordUsage)
	demoAccountRequired := fs.Bool("demo-account-required", false, reviewDetailDemoAccountRequiredUsage)
	notes := fs.String("notes", "", reviewDetailNotesUsage)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "details-create",
		ShortUsage: "asc review details-create --version-id \"VERSION_ID\" [flags]",
		ShortHelp:  "Create App Store review details for a version.",
		LongHelp: `Create App Store review details for a version.

Leave ` + "`--demo-account-required`" + ` false when ` + "`--notes`" + ` are enough for reviewer instructions.
Use ` + "`--demo-account-required=true`" + ` only when App Review needs demo credentials.
Do not use placeholder demo credentials just to satisfy the field shape.

Examples:
  asc review details-create --version-id "VERSION_ID" --contact-first-name "Dev" --contact-last-name "Support" --contact-email "dev@example.com" --contact-phone "+1 555 0100" --notes "Reviewer can use the guest flow from the welcome screen."
  asc review details-create --version-id "VERSION_ID" --contact-first-name "Dev" --contact-last-name "Support" --contact-email "dev@example.com" --contact-phone "+1 555 0100" --demo-account-required=true --demo-account-name "reviewer@example.com" --demo-account-password "app-specific-password" --notes "2FA is disabled for this review account."`,
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

			if visited["demo-account-required"] && *demoAccountRequired {
				if err := validateReviewDetailDemoCredentialValues(strings.TrimSpace(*demoAccountName), strings.TrimSpace(*demoAccountPassword)); err != nil {
					return err
				}
			}

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

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
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
	demoAccountName := fs.String("demo-account-name", "", reviewDetailDemoAccountNameUsage)
	demoAccountPassword := fs.String("demo-account-password", "", reviewDetailDemoAccountPasswordUsage)
	demoAccountRequired := fs.Bool("demo-account-required", false, reviewDetailDemoAccountRequiredUsage)
	notes := fs.String("notes", "", reviewDetailNotesUsage)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "details-update",
		ShortUsage: "asc review details-update --id \"DETAIL_ID\" [flags]",
		ShortHelp:  "Update App Store review details.",
		LongHelp: `Update App Store review details.

Leave ` + "`--demo-account-required`" + ` false when ` + "`--notes`" + ` are enough for reviewer instructions.
Use ` + "`--demo-account-required=true`" + ` only when App Review needs demo credentials.
Do not use placeholder demo credentials just to satisfy the field shape.

Examples:
  asc review details-update --id "DETAIL_ID" --notes "Reviewer can use the guest flow from the welcome screen."
  asc review details-update --id "DETAIL_ID" --demo-account-required=true --demo-account-name "reviewer@example.com" --demo-account-password "rotated-password" --notes "This account has full reviewer access."`,
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

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("review details-update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := validateReviewDetailUpdateDemoCredentials(
				requestCtx,
				client,
				detailValue,
				visited,
				*demoAccountRequired,
				strings.TrimSpace(*demoAccountName),
				strings.TrimSpace(*demoAccountPassword),
			); err != nil {
				return err
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

			resp, err := client.UpdateAppStoreReviewDetail(requestCtx, detailValue, attrs)
			if err != nil {
				return fmt.Errorf("review details-update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
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

func validateReviewDetailUpdateDemoCredentials(
	ctx context.Context,
	client *asc.Client,
	detailID string,
	visited map[string]bool,
	demoAccountRequired bool,
	demoAccountName string,
	demoAccountPassword string,
) error {
	if !visited["demo-account-required"] || !demoAccountRequired {
		return nil
	}

	effectiveName := demoAccountName
	effectivePassword := demoAccountPassword
	if visited["demo-account-name"] && visited["demo-account-password"] {
		return validateReviewDetailDemoCredentialValues(effectiveName, effectivePassword)
	}

	resp, err := client.GetAppStoreReviewDetail(ctx, detailID)
	if err != nil {
		return fmt.Errorf("review details-update: failed to fetch existing review details for demo credential validation: %w", err)
	}

	if !visited["demo-account-name"] {
		effectiveName = strings.TrimSpace(resp.Data.Attributes.DemoAccountName)
	}
	if !visited["demo-account-password"] {
		effectivePassword = strings.TrimSpace(resp.Data.Attributes.DemoAccountPassword)
	}

	return validateReviewDetailDemoCredentialValues(effectiveName, effectivePassword)
}

func validateReviewDetailDemoCredentialValues(demoAccountName, demoAccountPassword string) error {
	if strings.TrimSpace(demoAccountName) != "" && strings.TrimSpace(demoAccountPassword) != "" {
		return nil
	}

	fmt.Fprintln(os.Stderr, reviewDetailDemoCredentialsError)
	return flag.ErrHelp
}
