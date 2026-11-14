package productpages

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newCustomPageLocalizationsCommand returns the custom page localizations command group.
func newCustomPageLocalizationsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("localizations", flag.ExitOnError)

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
			newCustomPageLocalizationsListCommand(d),
			newCustomPageLocalizationsGetCommand(d),
			newCustomPageLocalizationsCreateCommand(d),
			newCustomPageLocalizationsUpdateCommand(d),
			newCustomPageLocalizationsDeleteCommand(d),
			newCustomPageLocalizationsSearchKeywordsCommand(d),
			newCustomPageLocalizationsPreviewSetsCommand(d),
			newCustomPageLocalizationsScreenshotSetsCommand(d),
			shared.CreativePlacementsCommand("appCustomProductPageLocalizations", "product-pages custom-pages localizations", "a custom product page localization"),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func customPageLocalizationIncludeList() []string {
	return []string{"appCustomProductPageVersion", "appScreenshotSets", "appPreviewSets", "searchKeywords", "placements"}
}

// newCustomPageLocalizationsListCommand returns the custom page localizations list subcommand.
func newCustomPageLocalizationsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-page-localizations list", flag.ExitOnError)

	versionID := shared.BindResourceIDFlag(fs, "custom-page-version-id", "appCustomProductPageVersions", "Custom product page version ID")
	include := shared.BindLocalizationIncludeFlags(fs, "Include related resources", customPageLocalizationIncludeList())
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc product-pages custom-pages localizations list --custom-page-version-id \"VERSION_ID\" [flags]",
		ShortHelp:  "List custom product page localizations.",
		LongHelp: `List custom product page localizations.

Examples:
  asc product-pages custom-pages localizations list --custom-page-version-id "VERSION_ID"
  asc product-pages custom-pages localizations list --custom-page-version-id "VERSION_ID" --include placements --placements-limit 50
  asc product-pages custom-pages localizations list --custom-page-version-id "VERSION_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > productPagesMaxLimit) {
				return d.UsageErrorf("custom-pages localizations list: --limit must be between 1 and %d", productPagesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return d.UsageErrorf("custom-pages localizations list: %v", err)
			}
			if err := d.RejectNextFlagConflicts(fs, *next, "custom-pages localizations list", "include", "placements-limit"); err != nil {
				return err
			}
			includeValues, placementsLimit, err := include.Resolve()
			if err != nil {
				return err
			}

			trimmedID := strings.TrimSpace(*versionID)
			if trimmedID == "" {
				return d.RequiredFlag("--custom-page-version-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("custom-pages localizations list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.AppCustomProductPageLocalizationsOption{
				asc.Limit[asc.AppCustomProductPageLocalizationsOption](*limit),
				asc.NextURL[asc.AppCustomProductPageLocalizationsOption](*next),
				asc.WithAppCustomProductPageLocalizationsInclude(includeValues),
				asc.WithAppCustomProductPageLocalizationsPlacementsLimit(placementsLimit),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.AppCustomProductPageLocalizationsOption](productPagesMaxLimit))
				firstPage, err := client.GetAppCustomProductPageLocalizations(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("custom-pages localizations list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppCustomProductPageLocalizations(ctx, trimmedID, asc.NextURL[asc.AppCustomProductPageLocalizationsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("custom-pages localizations list: %w", err)
				}

				return d.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppCustomProductPageLocalizations(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("custom-pages localizations list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newCustomPageLocalizationsGetCommand returns the custom page localizations get subcommand.
func newCustomPageLocalizationsGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-page-localizations view", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appCustomProductPageLocalizations", "Custom product page localization ID")
	include := shared.BindLocalizationIncludeFlags(fs, "Include related resources", customPageLocalizationIncludeList())
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc product-pages custom-pages localizations view --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "View a custom product page localization by ID.",
		LongHelp: `View a custom product page localization by ID.

Examples:
  asc product-pages custom-pages localizations view --localization-id "LOCALIZATION_ID"
  asc product-pages custom-pages localizations view --localization-id "LOCALIZATION_ID" --include placements --placements-limit 50`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			if trimmedID == "" {
				return d.RequiredFlag("--localization-id")
			}
			includeValues, placementsLimit, err := include.Resolve()
			if err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("custom-pages localizations view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAppCustomProductPageLocalization(requestCtx, trimmedID, asc.WithLocalizationReadInclude(includeValues), asc.WithLocalizationReadPlacementsLimit(placementsLimit))
			if err != nil {
				return fmt.Errorf("custom-pages localizations view: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newCustomPageLocalizationsCreateCommand returns the custom page localizations create subcommand.
func newCustomPageLocalizationsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-page-localizations create", flag.ExitOnError)

	versionID := shared.BindResourceIDFlag(fs, "custom-page-version-id", "appCustomProductPageVersions", "Custom product page version ID")
	locale := fs.String("locale", "", "Localization locale (e.g., en-US)")
	promotionalText := fs.String("promotional-text", "", "Promotional text")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--custom-page-version-id")
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				return d.RequiredFlag("--locale")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("custom-pages localizations create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateAppCustomProductPageLocalization(requestCtx, trimmedID, localeValue, strings.TrimSpace(*promotionalText))
			if err != nil {
				return fmt.Errorf("custom-pages localizations create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newCustomPageLocalizationsUpdateCommand returns the custom page localizations update subcommand.
func newCustomPageLocalizationsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-page-localizations update", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appCustomProductPageLocalizations", "Custom product page localization ID")
	promotionalText := fs.String("promotional-text", "", "Update promotional text")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--localization-id")
			}

			promoValue := strings.TrimSpace(*promotionalText)
			if promoValue == "" {
				return d.RequiredFlag("--promotional-text")
			}

			attrs := asc.AppCustomProductPageLocalizationUpdateAttributes{
				PromotionalText: &promoValue,
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("custom-pages localizations update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppCustomProductPageLocalization(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("custom-pages localizations update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newCustomPageLocalizationsDeleteCommand returns the custom page localizations delete subcommand.
func newCustomPageLocalizationsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:        d,
		FlagSetName: "custom-page-localizations delete",
		Name:        "delete",
		ShortUsage:  "asc product-pages custom-pages localizations delete --localization-id \"LOCALIZATION_ID\" --confirm",
		ShortHelp:   "Delete a custom product page localization.",
		LongHelp: `Delete a custom product page localization.

Examples:
  asc product-pages custom-pages localizations delete --localization-id "LOCALIZATION_ID" --confirm`,
		IDFlag:             "localization-id",
		IDUsage:            "Custom product page localization ID",
		IDType:             "appCustomProductPageLocalizations",
		RequiredDiagnostic: true,
		ErrorPrefix:        "custom-pages localizations delete",
		DeleteErrorPrefix:  "custom-pages localizations delete: failed to delete",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppCustomProductPageLocalization(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AppCustomProductPageLocalizationDeleteResult{ID: id, Deleted: true}
		},
	})
}
