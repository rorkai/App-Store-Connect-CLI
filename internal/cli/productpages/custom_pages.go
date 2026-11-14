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

// newCustomPagesCommand returns the custom pages command group.
func newCustomPagesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-pages", flag.ExitOnError)

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
			newCustomPagesListCommand(d),
			newCustomPagesGetCommand(d),
			newCustomPagesCreateCommand(d),
			newCustomPagesUpdateCommand(d),
			newCustomPagesDeleteCommand(d),
			newCustomPageVersionsCommand(d),
			newCustomPageLocalizationsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newCustomPagesListCommand returns the custom pages list subcommand.
func newCustomPagesListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-pages list", flag.ExitOnError)

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
	output := d.BindOutputFlags(fs)

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
				return d.UsageErrorf("custom-pages list: %v", err)
			}
			if err := d.RejectNextFlagConflicts(
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
					return d.UsageError(fmt.Sprintf("custom-pages list: --%s must not be empty", queryFlag.name))
				}
			}
			if *limit != 0 && (*limit < 1 || *limit > productPagesMaxLimit) {
				return d.UsageErrorf("custom-pages list: --limit must be between 1 and %d", productPagesMaxLimit)
			}
			if providedQueryFlags["versions-limit"] && (*versionsLimit < 1 || *versionsLimit > customPagesMaxVersionsLimit) {
				return d.UsageError(fmt.Sprintf("custom-pages list: --versions-limit must be between 1 and %d", customPagesMaxVersionsLimit))
			}

			visibleValues, err := normalizeCustomPagesVisible(*visible)
			if err != nil {
				return d.UsageError("custom-pages list: " + err.Error())
			}
			pageFields, err := shared.NormalizeSelection(*fields, customPagesFields, "--fields")
			if err != nil {
				return d.UsageError("custom-pages list: " + err.Error())
			}
			appFieldValues, err := shared.NormalizeSelection(*appFields, shared.AppFieldsList(), "--app-fields")
			if err != nil {
				return d.UsageError("custom-pages list: " + err.Error())
			}
			versionFieldValues, err := shared.NormalizeSelection(*versionFields, customPagesVersionFields, "--version-fields")
			if err != nil {
				return d.UsageError("custom-pages list: " + err.Error())
			}
			includeValues, err := shared.NormalizeSelection(*include, customPagesIncludes, "--include")
			if err != nil {
				return d.UsageError("custom-pages list: " + err.Error())
			}
			if len(appFieldValues) > 0 && !shared.HasInclude(includeValues, "app") {
				return customPagesListIncludeRequirementUsageError(d, "--app-fields", "custom-pages list: --app-fields requires --include app")
			}
			if len(versionFieldValues) > 0 && !shared.HasInclude(includeValues, "appCustomProductPageVersions") {
				return customPagesListIncludeRequirementUsageError(d, "--version-fields", "custom-pages list: --version-fields requires --include appCustomProductPageVersions")
			}
			if *versionsLimit > 0 && !shared.HasInclude(includeValues, "appCustomProductPageVersions") {
				return customPagesListIncludeRequirementUsageError(d, "--versions-limit", "custom-pages list: --versions-limit requires --include appCustomProductPageVersions")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("custom-pages list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.AppCustomProductPagesOption{
				asc.WithAppCustomProductPagesVisible(visibleValues),
				asc.Limit[asc.AppCustomProductPagesOption](*limit),
				asc.NextURL[asc.AppCustomProductPagesOption](*next),
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
				paginateOpts = append(paginateOpts, asc.Limit[asc.AppCustomProductPagesOption](productPagesMaxLimit))
				firstPage, err := client.GetAppCustomProductPages(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("custom-pages list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					continuationOpts := append([]asc.AppCustomProductPagesOption{}, opts...)
					continuationOpts = append(continuationOpts, asc.NextURL[asc.AppCustomProductPagesOption](nextURL))
					return client.GetAppCustomProductPages(ctx, resolvedAppID, continuationOpts...)
				})
				if err != nil {
					return fmt.Errorf("custom-pages list: %w", err)
				}

				return d.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppCustomProductPages(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("custom-pages list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func customPagesListIncludeRequirementUsageError(d *shared.Deps, parameter, message string) error {
	fmt.Fprintln(d.IO.ErrOut, "Error: "+message)
	return shared.WithDiagnostic(
		shared.NewReportedUsageError(shared.UsageErrorInvalidValue, message),
		shared.DiagnosticInvalidInput,
		parameter,
	)
}

const customPagesMaxVersionsLimit = 50

var (
	customPagesFields        = []string{"name", "url", "visible", "app", "appCustomProductPageVersions"}
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

// newCustomPagesGetCommand returns the custom pages get subcommand.
func newCustomPagesGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-pages view", flag.ExitOnError)

	customPageID := shared.BindResourceIDFlag(fs, "custom-page-id", "appCustomProductPages", "Custom product page ID")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--custom-page-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("custom-pages view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAppCustomProductPage(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("custom-pages view: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newCustomPagesCreateCommand returns the custom pages create subcommand.
func newCustomPagesCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-pages create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	name := fs.String("name", "", "Custom product page name")
	fromVersion := shared.BindResourceIDFlag(fs, "from-version", "appStoreVersions", "App Store version ID to use as the page template")
	fromCustomPage := shared.BindResourceIDFlag(fs, "from-custom-page", "appCustomProductPages", "Custom product page ID to use as the page template")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc product-pages custom-pages create --app \"APP_ID\" --name \"NAME\" [--from-version VERSION_ID | --from-custom-page PAGE_ID]",
		ShortHelp:  "Create a custom product page.",
		LongHelp: `Create a custom product page.

The command also creates an initial version and a localization using the app's primary locale.
Pass --from-version or --from-custom-page to start the page from a template.
Apple accepts --from-custom-page only for a page with an approved version.

Examples:
  asc product-pages custom-pages create --app "APP_ID" --name "Summer Campaign"
  asc product-pages custom-pages create --app "APP_ID" --name "Summer Campaign" --from-version "VERSION_ID"
  asc product-pages custom-pages create --app "APP_ID" --name "Summer Campaign" --from-custom-page "PAGE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				return d.RequiredFlag("--name")
			}

			template := asc.AppCustomProductPageTemplate{AppStoreVersionID: strings.TrimSpace(*fromVersion), CustomPageID: strings.TrimSpace(*fromCustomPage)}
			if template.AppStoreVersionID != "" && template.CustomPageID != "" {
				return d.UsageError("--from-version and --from-custom-page are mutually exclusive")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("custom-pages create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateAppCustomProductPage(requestCtx, resolvedAppID, nameValue, template)
			if err != nil {
				return fmt.Errorf("custom-pages create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newCustomPagesUpdateCommand returns the custom pages update subcommand.
func newCustomPagesUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-pages update", flag.ExitOnError)

	customPageID := shared.BindResourceIDFlag(fs, "custom-page-id", "appCustomProductPages", "Custom product page ID")
	name := fs.String("name", "", "Update page name")
	var visible shared.OptionalBool
	fs.Var(&visible, "visible", "Set visibility: true or false")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--custom-page-id")
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
				fmt.Fprintln(d.IO.ErrOut, "Error: --name or --visible is required")
				return shared.MissingRequiredUsageError("")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("custom-pages update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppCustomProductPage(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("custom-pages update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newCustomPagesDeleteCommand returns the custom pages delete subcommand.
func newCustomPagesDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:        d,
		FlagSetName: "custom-pages delete",
		Name:        "delete",
		ShortUsage:  "asc product-pages custom-pages delete --custom-page-id \"PAGE_ID\" --confirm",
		ShortHelp:   "Delete a custom product page.",
		LongHelp: `Delete a custom product page.

Examples:
  asc product-pages custom-pages delete --custom-page-id "PAGE_ID" --confirm`,
		IDFlag:             "custom-page-id",
		IDUsage:            "Custom product page ID",
		IDType:             "appCustomProductPages",
		RequiredDiagnostic: true,
		ErrorPrefix:        "custom-pages delete",
		DeleteErrorPrefix:  "custom-pages delete: failed to delete",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppCustomProductPage(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AppCustomProductPageDeleteResult{ID: id, Deleted: true}
		},
	})
}
