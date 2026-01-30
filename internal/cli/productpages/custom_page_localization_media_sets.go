package productpages

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// CustomPageLocalizationsPreviewSetsCommand returns the preview sets command group.
func CustomPageLocalizationsPreviewSetsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("preview-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "preview-sets",
		ShortUsage: "asc product-pages custom-pages localizations preview-sets <subcommand> [flags]",
		ShortHelp:  "Manage preview sets for a custom product page localization.",
		LongHelp: `Manage preview sets for a custom product page localization.

Examples:
  asc product-pages custom-pages localizations preview-sets list --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			CustomPageLocalizationsPreviewSetsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// CustomPageLocalizationsPreviewSetsListCommand returns the preview sets list subcommand.
func CustomPageLocalizationsPreviewSetsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-page-localizations preview-sets list", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Custom product page localization ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc product-pages custom-pages localizations preview-sets list --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "List preview sets for a custom product page localization.",
		LongHelp: `List preview sets for a custom product page localization.

Examples:
  asc product-pages custom-pages localizations preview-sets list --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages localizations preview-sets list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppCustomProductPageLocalizationPreviewSets(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("custom-pages localizations preview-sets list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// CustomPageLocalizationsScreenshotSetsCommand returns the screenshot sets command group.
func CustomPageLocalizationsScreenshotSetsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshot-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "screenshot-sets",
		ShortUsage: "asc product-pages custom-pages localizations screenshot-sets <subcommand> [flags]",
		ShortHelp:  "Manage screenshot sets for a custom product page localization.",
		LongHelp: `Manage screenshot sets for a custom product page localization.

Examples:
  asc product-pages custom-pages localizations screenshot-sets list --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			CustomPageLocalizationsScreenshotSetsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// CustomPageLocalizationsScreenshotSetsListCommand returns the screenshot sets list subcommand.
func CustomPageLocalizationsScreenshotSetsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-page-localizations screenshot-sets list", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Custom product page localization ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc product-pages custom-pages localizations screenshot-sets list --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "List screenshot sets for a custom product page localization.",
		LongHelp: `List screenshot sets for a custom product page localization.

Examples:
  asc product-pages custom-pages localizations screenshot-sets list --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages localizations screenshot-sets list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppCustomProductPageLocalizationScreenshotSets(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("custom-pages localizations screenshot-sets list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
