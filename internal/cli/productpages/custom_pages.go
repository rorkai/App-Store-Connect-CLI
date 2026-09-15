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

// CustomPagesCommand returns the custom pages command group.
func CustomPagesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-pages", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "custom-pages",
		ShortUsage: "asc product-pages custom-pages <subcommand> [flags]",
		ShortHelp:  "Manage custom product pages.",
		LongHelp: `Manage custom product pages.

Examples:
  asc product-pages custom-pages list --app "APP_ID"
  asc product-pages custom-pages view --custom-page-id "PAGE_ID"
  asc product-pages custom-pages create --app "APP_ID" --name "Summer Campaign"
  asc product-pages custom-pages update --custom-page-id "PAGE_ID" --name "Updated"
  asc product-pages custom-pages delete --custom-page-id "PAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			CustomPagesListCommand(),
			CustomPagesGetCommand(),
			CustomPagesCreateCommand(),
			CustomPagesUpdateCommand(),
			CustomPagesDeleteCommand(),
			CustomPageVersionsCommand(),
			CustomPageLocalizationsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// CustomPagesListCommand returns the custom pages list subcommand.
func CustomPagesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-pages list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	visible := fs.String("visible", "", "Filter by visibility (true/false), comma-separated")
	fields := fs.String("fields", "", "Custom product page fields: name,url,visible,app,appCustomProductPageVersions")
	appFields := fs.String("app-fields", "", "Included app fields (comma-separated)")
	versionFields := fs.String("version-fields", "", "Included version fields: version,state,deepLink,appCustomProductPage,appCustomProductPageLocalizations")
	include := fs.String("include", "", "Include related resources: app,appCustomProductPageVersions")
	versionsLimit := fs.Int("versions-limit", 0, "Maximum included versions (1-50)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc product-pages custom-pages list [flags]",
		ShortHelp:  "List custom product pages.",
		LongHelp: `List custom product pages.

Examples:
  asc product-pages custom-pages list --app "APP_ID"
  asc product-pages custom-pages list --app "APP_ID" --visible true
  asc product-pages custom-pages list --app "APP_ID" --include app,appCustomProductPageVersions --versions-limit 10
  asc product-pages custom-pages list --app "APP_ID" --limit 50
  asc product-pages custom-pages list --app "APP_ID" --paginate
  asc product-pages custom-pages list --next "<links.next>"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("custom-pages list: %v", err)
			}
			if err := shared.RejectNextFlagConflicts(
				fs,
				*next,
				"custom-pages list",
				"visible", "fields", "app-fields", "version-fields", "include", "versions-limit", "limit",
			); err != nil {
				return err
			}
			providedQueryFlags := make(map[string]bool)
			fs.Visit(func(f *flag.Flag) {
				providedQueryFlags[f.Name] = true
			})
			for _, queryFlag := range []struct {
				name  string
				value string
			}{
				{name: "visible", value: *visible},
				{name: "fields", value: *fields},
				{name: "app-fields", value: *appFields},
				{name: "version-fields", value: *versionFields},
				{name: "include", value: *include},
			} {
				if providedQueryFlags[queryFlag.name] && len(shared.SplitCSV(queryFlag.value)) == 0 {
					return shared.UsageError(fmt.Sprintf("custom-pages list: --%s must not be empty", queryFlag.name))
				}
			}
			if *limit != 0 && (*limit < 1 || *limit > productPagesMaxLimit) {
				return shared.UsageErrorf("custom-pages list: --limit must be between 1 and %d", productPagesMaxLimit)
			}
			if providedQueryFlags["versions-limit"] && (*versionsLimit < 1 || *versionsLimit > customPagesMaxVersionsLimit) {
				return shared.UsageError(fmt.Sprintf("custom-pages list: --versions-limit must be between 1 and %d", customPagesMaxVersionsLimit))
			}

			visibleValues, err := normalizeCustomPagesVisible(*visible)
			if err != nil {
				return shared.UsageError("custom-pages list: " + err.Error())
			}
			pageFields, err := shared.NormalizeSelection(*fields, customPagesFields, "--fields")
			if err != nil {
				return shared.UsageError("custom-pages list: " + err.Error())
			}
			appFieldValues, err := shared.NormalizeSelection(*appFields, customPagesAppFields, "--app-fields")
			if err != nil {
				return shared.UsageError("custom-pages list: " + err.Error())
			}
			versionFieldValues, err := shared.NormalizeSelection(*versionFields, customPagesVersionFields, "--version-fields")
			if err != nil {
				return shared.UsageError("custom-pages list: " + err.Error())
			}
			includeValues, err := shared.NormalizeSelection(*include, customPagesIncludes, "--include")
			if err != nil {
				return shared.UsageError("custom-pages list: " + err.Error())
			}
			if len(appFieldValues) > 0 && !shared.HasInclude(includeValues, "app") {
				return customPagesListIncludeRequirementUsageError("--app-fields", "custom-pages list: --app-fields requires --include app")
			}
			if len(versionFieldValues) > 0 && !shared.HasInclude(includeValues, "appCustomProductPageVersions") {
				return customPagesListIncludeRequirementUsageError("--version-fields", "custom-pages list: --version-fields requires --include appCustomProductPageVersions")
			}
			if *versionsLimit > 0 && !shared.HasInclude(includeValues, "appCustomProductPageVersions") {
				return customPagesListIncludeRequirementUsageError("--versions-limit", "custom-pages list: --versions-limit requires --include appCustomProductPageVersions")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppCustomProductPagesOption{
				asc.WithAppCustomProductPagesVisible(visibleValues),
				asc.WithAppCustomProductPagesLimit(*limit),
				asc.WithAppCustomProductPagesNextURL(*next),
			}
			if len(pageFields) > 0 {
				opts = append(opts, asc.WithAppCustomProductPagesFields(pageFields))
			}
			if len(appFieldValues) > 0 {
				opts = append(opts, asc.WithAppCustomProductPagesAppFields(appFieldValues))
			}
			if len(versionFieldValues) > 0 {
				opts = append(opts, asc.WithAppCustomProductPagesVersionFields(versionFieldValues))
			}
			if len(includeValues) > 0 {
				opts = append(opts, asc.WithAppCustomProductPagesInclude(includeValues))
			}
			if *versionsLimit > 0 {
				opts = append(opts, asc.WithAppCustomProductPagesVersionsLimit(*versionsLimit))
			}

			if *paginate {
				paginateOpts := append([]asc.AppCustomProductPagesOption{}, opts...)
				paginateOpts = append(paginateOpts, asc.WithAppCustomProductPagesLimit(productPagesMaxLimit))
				firstPage, err := client.GetAppCustomProductPages(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("custom-pages list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					continuationOpts := append([]asc.AppCustomProductPagesOption{}, opts...)
					continuationOpts = append(continuationOpts, asc.WithAppCustomProductPagesNextURL(nextURL))
					return client.GetAppCustomProductPages(ctx, resolvedAppID, continuationOpts...)
				})
				if err != nil {
					return fmt.Errorf("custom-pages list: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppCustomProductPages(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("custom-pages list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func customPagesListIncludeRequirementUsageError(parameter, message string) error {
	fmt.Fprintln(os.Stderr, "Error: "+message)
	return shared.WithDiagnostic(
		shared.NewReportedUsageError(shared.UsageErrorInvalidValue, message),
		shared.DiagnosticInvalidInput,
		parameter,
	)
}

const customPagesMaxVersionsLimit = 50

var (
	customPagesFields        = []string{"name", "url", "visible", "app", "appCustomProductPageVersions"}
	customPagesAppFields     = []string{"accessibilityUrl", "name", "bundleId", "sku", "primaryLocale", "isOrEverWasMadeForKids", "subscriptionStatusUrl", "subscriptionStatusUrlVersion", "subscriptionStatusUrlForSandbox", "subscriptionStatusUrlVersionForSandbox", "contentRightsDeclaration", "streamlinedPurchasingEnabled", "accessibilityDeclarations", "appEncryptionDeclarations", "appStoreIcon", "ciProduct", "betaTesters", "betaGroups", "appStoreVersions", "appTags", "preReleaseVersions", "betaAppLocalizations", "builds", "betaLicenseAgreement", "betaAppReviewDetail", "appInfos", "appClips", "appPricePoints", "endUserLicenseAgreement", "appPriceSchedule", "appAvailabilityV2", "inAppPurchases", "subscriptionGroups", "gameCenterEnabledVersions", "perfPowerMetrics", "appCustomProductPages", "inAppPurchasesV2", "promotedPurchases", "appEvents", "reviewSubmissions", "subscriptionGracePeriod", "customerReviews", "customerReviewSummarizations", "gameCenterDetail", "appStoreVersionExperimentsV2", "alternativeDistributionKey", "analyticsReportRequests", "marketplaceSearchDetail", "buildUploads", "backgroundAssets", "betaFeedbackScreenshotSubmissions", "betaFeedbackCrashSubmissions", "searchKeywords", "webhooks", "androidToIosAppMappingDetails"}
	customPagesVersionFields = []string{"version", "state", "deepLink", "appCustomProductPage", "appCustomProductPageLocalizations"}
	customPagesIncludes      = []string{"app", "appCustomProductPageVersions"}
)

func normalizeCustomPagesVisible(value string) ([]string, error) {
	values := shared.SplitCSV(value)
	if len(values) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(values))
	for _, item := range values {
		switch strings.ToLower(strings.TrimSpace(item)) {
		case "true":
			normalized = append(normalized, "true")
		case "false":
			normalized = append(normalized, "false")
		default:
			return nil, fmt.Errorf("--visible must be true or false")
		}
	}
	return normalized, nil
}

// CustomPagesGetCommand returns the custom pages get subcommand.
func CustomPagesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-pages view", flag.ExitOnError)

	customPageID := shared.BindResourceIDFlag(fs, "custom-page-id", "appCustomProductPages", "Custom product page ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc product-pages custom-pages view --custom-page-id \"PAGE_ID\"",
		ShortHelp:  "View a custom product page by ID.",
		LongHelp: `View a custom product page by ID.

Examples:
  asc product-pages custom-pages view --custom-page-id "PAGE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*customPageID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --custom-page-id is required")
				return shared.MissingRequiredUsageError("--custom-page-id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppCustomProductPage(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("custom-pages view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// CustomPagesCreateCommand returns the custom pages create subcommand.
func CustomPagesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-pages create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	name := fs.String("name", "", "Custom product page name")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc product-pages custom-pages create --app \"APP_ID\" --name \"NAME\"",
		ShortHelp:  "Create a custom product page.",
		LongHelp: `Create a custom product page.

Examples:
  asc product-pages custom-pages create --app "APP_ID" --name "Summer Campaign"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return shared.MissingRequiredUsageError("--name")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAppCustomProductPage(requestCtx, resolvedAppID, nameValue)
			if err != nil {
				return fmt.Errorf("custom-pages create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// CustomPagesUpdateCommand returns the custom pages update subcommand.
func CustomPagesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-pages update", flag.ExitOnError)

	customPageID := shared.BindResourceIDFlag(fs, "custom-page-id", "appCustomProductPages", "Custom product page ID")
	name := fs.String("name", "", "Update page name")
	var visible shared.OptionalBool
	fs.Var(&visible, "visible", "Set visibility: true or false")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc product-pages custom-pages update --custom-page-id \"PAGE_ID\" [--name \"NAME\"] [--visible true|false]",
		ShortHelp:  "Update a custom product page.",
		LongHelp: `Update a custom product page.

Examples:
  asc product-pages custom-pages update --custom-page-id "PAGE_ID" --name "Updated"
  asc product-pages custom-pages update --custom-page-id "PAGE_ID" --visible true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*customPageID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --custom-page-id is required")
				return shared.MissingRequiredUsageError("--custom-page-id")
			}

			attrs := asc.AppCustomProductPageUpdateAttributes{}
			if nameValue := strings.TrimSpace(*name); nameValue != "" {
				attrs.Name = &nameValue
			}
			if visible.IsSet() {
				value := visible.Value()
				attrs.Visible = &value
			}
			if attrs.Name == nil && attrs.Visible == nil {
				fmt.Fprintln(os.Stderr, "Error: --name or --visible is required")
				return shared.MissingRequiredUsageError("")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppCustomProductPage(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("custom-pages update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// CustomPagesDeleteCommand returns the custom pages delete subcommand.
func CustomPagesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-pages delete", flag.ExitOnError)

	customPageID := shared.BindResourceIDFlag(fs, "custom-page-id", "appCustomProductPages", "Custom product page ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc product-pages custom-pages delete --custom-page-id \"PAGE_ID\" --confirm",
		ShortHelp:  "Delete a custom product page.",
		LongHelp: `Delete a custom product page.

Examples:
  asc product-pages custom-pages delete --custom-page-id "PAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*customPageID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --custom-page-id is required")
				return shared.MissingRequiredUsageError("--custom-page-id")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppCustomProductPage(requestCtx, trimmedID); err != nil {
				return fmt.Errorf("custom-pages delete: failed to delete: %w", err)
			}

			result := &asc.AppCustomProductPageDeleteResult{ID: trimmedID, Deleted: true}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
