package productpages

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

// CustomPageLocalizationsCommand returns the custom page localizations command group.
func CustomPageLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc product-pages custom-pages localizations <subcommand> [flags]",
		ShortHelp:  "Manage custom product page localizations.",
		LongHelp: `Manage custom product page localizations.

Examples:
  asc product-pages custom-pages localizations list --custom-page-version-id "VERSION_ID"
  asc product-pages custom-pages localizations create --custom-page-version-id "VERSION_ID" --locale "en-US"
  asc product-pages custom-pages localizations delete --localization-id "LOCALIZATION_ID" --confirm
  asc product-pages custom-pages localizations search-keywords list --localization-id "LOCALIZATION_ID"
  asc product-pages custom-pages localizations preview-sets list --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			CustomPageLocalizationsListCommand(),
			CustomPageLocalizationsGetCommand(),
			CustomPageLocalizationsCreateCommand(),
			CustomPageLocalizationsUpdateCommand(),
			CustomPageLocalizationsDeleteCommand(),
			CustomPageLocalizationsSearchKeywordsCommand(),
			CustomPageLocalizationsPreviewSetsCommand(),
			CustomPageLocalizationsScreenshotSetsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// CustomPageLocalizationsListCommand returns the custom page localizations list subcommand.
func CustomPageLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-page-localizations list", flag.ExitOnError)

	versionID := fs.String("custom-page-version-id", "", "Custom product page version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc product-pages custom-pages localizations list --custom-page-version-id \"VERSION_ID\" [flags]",
		ShortHelp:  "List custom product page localizations.",
		LongHelp: `List custom product page localizations.

Examples:
  asc product-pages custom-pages localizations list --custom-page-version-id "VERSION_ID"
  asc product-pages custom-pages localizations list --custom-page-version-id "VERSION_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > productPagesMaxLimit) {
				return fmt.Errorf("custom-pages localizations list: --limit must be between 1 and %d", productPagesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("custom-pages localizations list: %w", err)
			}

			trimmedID := strings.TrimSpace(*versionID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --custom-page-version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages localizations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppCustomProductPageLocalizationsOption{
				asc.WithAppCustomProductPageLocalizationsLimit(*limit),
				asc.WithAppCustomProductPageLocalizationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppCustomProductPageLocalizationsLimit(productPagesMaxLimit))
				firstPage, err := client.GetAppCustomProductPageLocalizations(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("custom-pages localizations list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppCustomProductPageLocalizations(ctx, trimmedID, asc.WithAppCustomProductPageLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("custom-pages localizations list: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppCustomProductPageLocalizations(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("custom-pages localizations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// CustomPageLocalizationsGetCommand returns the custom page localizations get subcommand.
func CustomPageLocalizationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-page-localizations get", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Custom product page localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc product-pages custom-pages localizations get --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "Get a custom product page localization by ID.",
		LongHelp: `Get a custom product page localization by ID.

Examples:
  asc product-pages custom-pages localizations get --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages localizations get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppCustomProductPageLocalization(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("custom-pages localizations get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// CustomPageLocalizationsCreateCommand returns the custom page localizations create subcommand.
func CustomPageLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-page-localizations create", flag.ExitOnError)

	versionID := fs.String("custom-page-version-id", "", "Custom product page version ID")
	locale := fs.String("locale", "", "Localization locale (e.g., en-US)")
	promotionalText := fs.String("promotional-text", "", "Promotional text")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc product-pages custom-pages localizations create --custom-page-version-id \"VERSION_ID\" --locale \"en-US\"",
		ShortHelp:  "Create a custom product page localization.",
		LongHelp: `Create a custom product page localization.

Examples:
  asc product-pages custom-pages localizations create --custom-page-version-id "VERSION_ID" --locale "en-US"
  asc product-pages custom-pages localizations create --custom-page-version-id "VERSION_ID" --locale "en-US" --promotional-text "Promo copy"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*versionID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --custom-page-version-id is required")
				return flag.ErrHelp
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages localizations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAppCustomProductPageLocalization(requestCtx, trimmedID, localeValue, strings.TrimSpace(*promotionalText))
			if err != nil {
				return fmt.Errorf("custom-pages localizations create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// CustomPageLocalizationsUpdateCommand returns the custom page localizations update subcommand.
func CustomPageLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-page-localizations update", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Custom product page localization ID")
	promotionalText := fs.String("promotional-text", "", "Update promotional text")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc product-pages custom-pages localizations update --localization-id \"LOCALIZATION_ID\" --promotional-text \"TEXT\"",
		ShortHelp:  "Update a custom product page localization.",
		LongHelp: `Update a custom product page localization.

Examples:
  asc product-pages custom-pages localizations update --localization-id "LOCALIZATION_ID" --promotional-text "Updated copy"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			promoValue := strings.TrimSpace(*promotionalText)
			if promoValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --promotional-text is required")
				return flag.ErrHelp
			}

			attrs := asc.AppCustomProductPageLocalizationUpdateAttributes{
				PromotionalText: &promoValue,
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages localizations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppCustomProductPageLocalization(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("custom-pages localizations update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// CustomPageLocalizationsDeleteCommand returns the custom page localizations delete subcommand.
func CustomPageLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-page-localizations delete", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Custom product page localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc product-pages custom-pages localizations delete --localization-id \"LOCALIZATION_ID\" --confirm",
		ShortHelp:  "Delete a custom product page localization.",
		LongHelp: `Delete a custom product page localization.

Examples:
  asc product-pages custom-pages localizations delete --localization-id "LOCALIZATION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages localizations delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppCustomProductPageLocalization(requestCtx, trimmedID); err != nil {
				return fmt.Errorf("custom-pages localizations delete: failed to delete: %w", err)
			}

			result := &asc.AppCustomProductPageLocalizationDeleteResult{ID: trimmedID, Deleted: true}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
