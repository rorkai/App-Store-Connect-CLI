package localizations

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

// LocalizationsPreviewSetsCommand returns the preview sets command group.
func LocalizationsPreviewSetsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("preview-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "preview-sets",
		ShortUsage: "asc localizations preview-sets <subcommand> [flags]",
		ShortHelp:  "Manage preview sets for an App Store localization.",
		LongHelp: `Manage preview sets for an App Store localization.

Examples:
  asc localizations preview-sets list --localization-id "LOCALIZATION_ID"
  asc localizations preview-sets get --id "PREVIEW_SET_ID"
  asc localizations preview-sets relationships --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			LocalizationsPreviewSetsListCommand(),
			LocalizationsPreviewSetsGetCommand(),
			LocalizationsPreviewSetsRelationshipsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// LocalizationsPreviewSetsListCommand returns the preview sets list subcommand.
func LocalizationsPreviewSetsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations preview-sets list", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "App Store version localization ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc localizations preview-sets list --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "List preview sets for an App Store localization.",
		LongHelp: `List preview sets for an App Store localization.

Examples:
  asc localizations preview-sets list --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			trimmedNext := strings.TrimSpace(*next)
			if trimmedID == "" && trimmedNext == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("localizations preview-sets list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("localizations preview-sets list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("localizations preview-sets list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppStoreVersionLocalizationPreviewSetsOption{
				asc.WithAppStoreVersionLocalizationPreviewSetsLimit(*limit),
				asc.WithAppStoreVersionLocalizationPreviewSetsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppStoreVersionLocalizationPreviewSetsLimit(200))
				firstPage, err := client.GetAppStoreVersionLocalizationPreviewSets(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("localizations preview-sets list: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppStoreVersionLocalizationPreviewSets(ctx, trimmedID, asc.WithAppStoreVersionLocalizationPreviewSetsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("localizations preview-sets list: %w", err)
				}
				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppStoreVersionLocalizationPreviewSets(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("localizations preview-sets list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// LocalizationsPreviewSetsGetCommand returns the preview sets get subcommand.
func LocalizationsPreviewSetsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations preview-sets get", flag.ExitOnError)

	setID := fs.String("id", "", "App preview set ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc localizations preview-sets get --id \"PREVIEW_SET_ID\"",
		ShortHelp:  "Get an app preview set by ID.",
		LongHelp: `Get an app preview set by ID.

Examples:
  asc localizations preview-sets get --id "PREVIEW_SET_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*setID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("localizations preview-sets get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppPreviewSet(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("localizations preview-sets get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// LocalizationsPreviewSetsRelationshipsCommand returns the preview sets relationships subcommand.
func LocalizationsPreviewSetsRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations preview-sets relationships", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "App Store version localization ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc localizations preview-sets relationships --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "List preview set relationships for an App Store localization.",
		LongHelp: `List preview set relationships for an App Store localization.

Examples:
  asc localizations preview-sets relationships --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			trimmedNext := strings.TrimSpace(*next)
			if trimmedID == "" && trimmedNext == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("localizations preview-sets relationships: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("localizations preview-sets relationships: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("localizations preview-sets relationships: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetAppStoreVersionLocalizationPreviewSetsRelationships(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("localizations preview-sets relationships: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppStoreVersionLocalizationPreviewSetsRelationships(ctx, trimmedID, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("localizations preview-sets relationships: %w", err)
				}
				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppStoreVersionLocalizationPreviewSetsRelationships(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("localizations preview-sets relationships: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// LocalizationsScreenshotSetsCommand returns the screenshot sets command group.
func LocalizationsScreenshotSetsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshot-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "screenshot-sets",
		ShortUsage: "asc localizations screenshot-sets <subcommand> [flags]",
		ShortHelp:  "Manage screenshot sets for an App Store localization.",
		LongHelp: `Manage screenshot sets for an App Store localization.

Examples:
  asc localizations screenshot-sets list --localization-id "LOCALIZATION_ID"
  asc localizations screenshot-sets get --id "SCREENSHOT_SET_ID"
  asc localizations screenshot-sets relationships --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			LocalizationsScreenshotSetsListCommand(),
			LocalizationsScreenshotSetsGetCommand(),
			LocalizationsScreenshotSetsRelationshipsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// LocalizationsScreenshotSetsGetCommand returns the screenshot sets get subcommand.
func LocalizationsScreenshotSetsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations screenshot-sets get", flag.ExitOnError)

	setID := fs.String("id", "", "App screenshot set ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc localizations screenshot-sets get --id \"SCREENSHOT_SET_ID\"",
		ShortHelp:  "Get an app screenshot set by ID.",
		LongHelp: `Get an app screenshot set by ID.

Examples:
  asc localizations screenshot-sets get --id "SCREENSHOT_SET_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*setID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("localizations screenshot-sets get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppScreenshotSet(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("localizations screenshot-sets get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// LocalizationsScreenshotSetsListCommand returns the screenshot sets list subcommand.
func LocalizationsScreenshotSetsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations screenshot-sets list", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "App Store version localization ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc localizations screenshot-sets list --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "List screenshot sets for an App Store localization.",
		LongHelp: `List screenshot sets for an App Store localization.

Examples:
  asc localizations screenshot-sets list --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			trimmedNext := strings.TrimSpace(*next)
			if trimmedID == "" && trimmedNext == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("localizations screenshot-sets list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("localizations screenshot-sets list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("localizations screenshot-sets list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppStoreVersionLocalizationScreenshotSetsOption{
				asc.WithAppStoreVersionLocalizationScreenshotSetsLimit(*limit),
				asc.WithAppStoreVersionLocalizationScreenshotSetsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppStoreVersionLocalizationScreenshotSetsLimit(200))
				firstPage, err := client.GetAppStoreVersionLocalizationScreenshotSets(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("localizations screenshot-sets list: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppStoreVersionLocalizationScreenshotSets(ctx, trimmedID, asc.WithAppStoreVersionLocalizationScreenshotSetsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("localizations screenshot-sets list: %w", err)
				}
				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppStoreVersionLocalizationScreenshotSets(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("localizations screenshot-sets list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// LocalizationsScreenshotSetsRelationshipsCommand returns the screenshot sets relationships subcommand.
func LocalizationsScreenshotSetsRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations screenshot-sets relationships", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "App Store version localization ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc localizations screenshot-sets relationships --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "List screenshot set relationships for an App Store localization.",
		LongHelp: `List screenshot set relationships for an App Store localization.

Examples:
  asc localizations screenshot-sets relationships --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			trimmedNext := strings.TrimSpace(*next)
			if trimmedID == "" && trimmedNext == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("localizations screenshot-sets relationships: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("localizations screenshot-sets relationships: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("localizations screenshot-sets relationships: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetAppStoreVersionLocalizationScreenshotSetsRelationships(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("localizations screenshot-sets relationships: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppStoreVersionLocalizationScreenshotSetsRelationships(ctx, trimmedID, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("localizations screenshot-sets relationships: %w", err)
				}
				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppStoreVersionLocalizationScreenshotSetsRelationships(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("localizations screenshot-sets relationships: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}
