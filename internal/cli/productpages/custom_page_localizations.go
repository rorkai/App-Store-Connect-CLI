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
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
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
			if err := d.ValidatePaginationFlags(ctx, fs, "product-pages custom-pages localizations list", productPagesMaxLimit, "include", "placements-limit"); err != nil {
				return err
			}

			trimmedID := strings.TrimSpace(*versionID)
			if trimmedID == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--custom-page-version-id")
			}

			includeValues, placementsLimit, err := include.Resolve()
			if err != nil {
				return err
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
	var (
		includeValues   []string
		placementsLimit int
	)

	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "custom-page-localizations view",
		Name:        "view",
		ShortUsage:  "asc product-pages custom-pages localizations view --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:   "View a custom product page localization by ID.",
		LongHelp: `View a custom product page localization by ID.

Examples:
  asc product-pages custom-pages localizations view --localization-id "LOCALIZATION_ID"
  asc product-pages custom-pages localizations view --localization-id "LOCALIZATION_ID" --include placements --placements-limit 50`,
		IDFlag:             "localization-id",
		IDUsage:            "Custom product page localization ID",
		IDType:             "appCustomProductPageLocalizations",
		RequiredDiagnostic: true,
		ErrorPrefix:        "custom-pages localizations view",
		Deps:               d,
		Flags: func(fs *flag.FlagSet) func() error {
			include := shared.BindLocalizationIncludeFlags(fs, "Include related resources", customPageLocalizationIncludeList())
			return func() (err error) {
				includeValues, placementsLimit, err = include.Resolve()
				return err
			}
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppCustomProductPageLocalization(ctx, id, asc.WithLocalizationReadInclude(includeValues), asc.WithLocalizationReadPlacementsLimit(placementsLimit))
		},
	})
}

// newCustomPageLocalizationsCreateCommand returns the custom page localizations create subcommand.
func newCustomPageLocalizationsCreateCommand(d *shared.Deps) *ffcli.Command {
	var locale, promotionalText string
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		FlagSetName: "custom-page-localizations create",
		Name:        "create",
		ShortUsage:  "asc product-pages custom-pages localizations create --custom-page-version-id \"VERSION_ID\" --locale \"en-US\"",
		ShortHelp:   "Create a custom product page localization.",
		LongHelp: `Create a custom product page localization.

Examples:
  asc product-pages custom-pages localizations create --custom-page-version-id "VERSION_ID" --locale "en-US"
  asc product-pages custom-pages localizations create --custom-page-version-id "VERSION_ID" --locale "en-US" --promotional-text "Promo copy"`,
		ErrorPrefix: "custom-pages localizations create",
		IDFlag:      "custom-page-version-id",
		IDUsage:     "Custom product page version ID",
		IDType:      "appCustomProductPageVersions",
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&locale, "locale", "Localization locale (e.g., en-US)"),
			shared.StringFlag(&promotionalText, "promotional-text", "Promotional text"),
		},
		Deps: d,
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateAppCustomProductPageLocalization(ctx, id, locale, promotionalText)
		},
	})
}

// newCustomPageLocalizationsUpdateCommand returns the custom page localizations update subcommand.
func newCustomPageLocalizationsUpdateCommand(d *shared.Deps) *ffcli.Command {
	var promotionalText string
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		FlagSetName: "custom-page-localizations update",
		Name:        "update",
		ShortUsage:  "asc product-pages custom-pages localizations update --localization-id \"LOCALIZATION_ID\" --promotional-text \"TEXT\"",
		ShortHelp:   "Update a custom product page localization.",
		LongHelp: `Update a custom product page localization.

Examples:
  asc product-pages custom-pages localizations update --localization-id "LOCALIZATION_ID" --promotional-text "Updated copy"`,
		ErrorPrefix: "custom-pages localizations update",
		IDFlag:      "localization-id",
		IDUsage:     "Custom product page localization ID",
		IDType:      "appCustomProductPageLocalizations",
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&promotionalText, "promotional-text", "Update promotional text"),
		},
		Deps: d,
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.UpdateAppCustomProductPageLocalization(ctx, id, asc.AppCustomProductPageLocalizationUpdateAttributes{PromotionalText: &promotionalText})
		},
	})
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
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppCustomProductPageLocalization(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}
