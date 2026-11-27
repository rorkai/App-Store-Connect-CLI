package productpages

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/assets"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newCustomPageScreenshotsCommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{
		Name:        "screenshots",
		ShortUsage:  "asc product-pages custom-pages screenshots <subcommand>",
		ShortHelp:   "Plan screenshot targets for a custom product page.",
		FlagSet:     d.NewFlagSet("custom-page screenshots", flag.ExitOnError),
		UsageFunc:   d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{newCustomPageScreenshotsPlanCommand(d)},
		Exec:        func(context.Context, []string) error { return flag.ErrHelp },
	}
}

func newCustomPageScreenshotsPlanCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-page screenshots plan", flag.ExitOnError)
	app := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	page := shared.BindResourceIDFlag(fs, "custom-page-id", "appCustomProductPages", "Custom product page ID")
	path := fs.String("path", "", "Screenshot tree containing locale-first or device-first folders")
	devices := fs.String("device-type", "", "Device types, comma-separated (e.g., IPHONE_65,IPAD_PRO_3GEN_129)")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name:       "plan",
		ShortUsage: "asc product-pages custom-pages screenshots plan --app APP_ID --custom-page-id PAGE_ID --path ./screenshots --device-type IPHONE_65,IPAD_PRO_3GEN_129",
		ShortHelp:  "Preview locale and device screenshot targets without changes.",
		LongHelp: `Resolve a screenshot target plan for one custom product page.

The page must belong to --app and have exactly one PREPARE_FOR_SUBMISSION or
REJECTED version. Ownership and version selection are checked before local files
are read. All remote collection pages are inspected.

Use existing locale-first folders such as screenshots/en-US/iphone/*.png, or
device-first folders such as screenshots/iphone/en-US/*.png. Device types are
explicit; image dimensions select among them, and ambiguous matches are refused.
Unselected images and non-image files are listed in skippedFiles. Image format,
alpha, dimensions, duplicate content, filename order and set limits are checked.
Symlinks and duplicate locale aliases are refused.

The plan resolves existing localization IDs and reports missing localizations.
ready means every planned localization exists. This read-only command does not
create localizations, upload files, or confirm provider write permission.

Examples:
  asc product-pages custom-pages screenshots plan --app APP_ID --custom-page-id PAGE_ID --path ./screenshots --device-type IPHONE_65,IPAD_PRO_3GEN_129`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			appID, pageID, rootPath := d.ResolveAppID(*app), strings.TrimSpace(*page), strings.TrimSpace(*path)
			for _, required := range []struct{ name, value string }{{"--app", appID}, {"--custom-page-id", pageID}, {"--path", rootPath}, {"--device-type", strings.TrimSpace(*devices)}} {
				if required.value == "" {
					return d.RequiredFlag(required.name)
				}
			}
			if !shared.IsNumericAppID(appID) || strings.HasPrefix(appID, "0") {
				return d.UsageError("--app requires a canonical numeric app ID")
			}
			deviceTypes, err := assets.NormalizeScreenshotPlanDeviceTypes(shared.SplitCSV(*devices))
			if err != nil {
				return d.UsageError(err.Error())
			}
			ctx = asc.WithNoRedirectReads(ctx)
			client, err := d.Client()
			if err != nil {
				return err
			}
			pages, err := collectCustomPagePlanResources(ctx, d, client, "/v1/apps/"+appID+"/appCustomProductPages", func(ctx context.Context, next string) (*asc.AppCustomProductPagesResponse, error) {
				return client.GetAppCustomProductPages(ctx, appID, asc.NextURL[asc.AppCustomProductPagesOption](next))
			})
			if err != nil {
				return err
			}
			owned := false
			for _, item := range pages.Data {
				if item.ID == pageID && item.Type == asc.ResourceTypeAppCustomProductPages {
					owned = true
				}
			}
			if !owned {
				return fmt.Errorf("custom page %s does not belong to app %s", pageID, appID)
			}
			versions, err := collectCustomPagePlanResources(ctx, d, client, "/v1/appCustomProductPages/"+url.PathEscape(pageID)+"/appCustomProductPageVersions", func(ctx context.Context, next string) (*asc.AppCustomProductPageVersionsResponse, error) {
				return client.GetAppCustomProductPageVersions(ctx, pageID, asc.NextURL[asc.AppCustomProductPageVersionsOption](next))
			})
			if err != nil {
				return err
			}
			editable := []asc.Resource[asc.AppCustomProductPageVersionAttributes]{}
			for _, version := range versions.Data {
				if version.Type == asc.ResourceTypeAppCustomProductPageVersions && (version.Attributes.State == "PREPARE_FOR_SUBMISSION" || version.Attributes.State == "REJECTED") {
					editable = append(editable, version)
				}
			}
			if len(editable) != 1 {
				return fmt.Errorf("expected exactly one editable custom page version (PREPARE_FOR_SUBMISSION or REJECTED), found %d", len(editable))
			}
			version := editable[0]
			if _, err := asc.ValidateResourcePathSegment(version.ID); err != nil {
				return fmt.Errorf("invalid custom page version ID: %w", err)
			}
			localizations, err := collectCustomPagePlanResources(ctx, d, client, "/v1/appCustomProductPageVersions/"+url.PathEscape(version.ID)+"/appCustomProductPageLocalizations", func(ctx context.Context, next string) (*asc.AppCustomProductPageLocalizationsResponse, error) {
				return client.GetAppCustomProductPageLocalizations(ctx, version.ID, asc.NextURL[asc.AppCustomProductPageLocalizationsOption](next))
			})
			if err != nil {
				return err
			}
			targets := map[string]string{}
			for _, item := range localizations.Data {
				locale, err := shared.CanonicalizeAppStoreLocalizationLocale(item.Attributes.Locale)
				if err != nil {
					return err
				}
				if item.ID == "" || item.Type != asc.ResourceTypeAppCustomProductPageLocalizations || targets[locale] != "" {
					return fmt.Errorf("ambiguous custom page localization for %s", locale)
				}
				targets[locale] = item.ID
			}
			groups, skipped, err := assets.DiscoverScreenshotPlanGroups(d.OSPath(rootPath), deviceTypes)
			if err != nil {
				return assets.ClassifyMediaInputError(err, "--path")
			}
			if len(skipped) > 0 {
				fmt.Fprintf(d.IO.ErrOut, "Warning: %d file(s) were not selected for the plan: %s\n", len(skipped), shared.SanitizeTerminal(strings.Join(skipped, ", ")))
			}
			result := &asc.CustomPageScreenshotPlan{AppID: appID, CustomPageID: pageID, VersionID: version.ID, VersionState: version.Attributes.State, Path: rootPath, ReadOnly: true, Ready: true, Groups: []asc.CustomPageScreenshotPlanGroup{}, MissingLocalizations: []string{}, SkippedFiles: skipped}
			missing := map[string]bool{}
			for _, group := range groups {
				id := targets[group.Locale]
				status := "resolved"
				if id == "" {
					status = "missingLocalization"
					result.Ready = false
					missing[group.Locale] = true
				}
				result.Groups = append(result.Groups, asc.CustomPageScreenshotPlanGroup{Locale: group.Locale, DisplayType: group.DisplayType, LocalizationID: id, Status: status, Files: group.Files})
			}
			for locale := range missing {
				result.MissingLocalizations = append(result.MissingLocalizations, locale)
			}
			sort.Strings(result.MissingLocalizations)
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func collectCustomPagePlanResources[T any](ctx context.Context, d *shared.Deps, client *asc.Client, expectedPath string, fetch func(context.Context, string) (*asc.Response[T], error)) (*asc.Response[T], error) {
	page := func(ctx context.Context, next string) (*asc.Response[T], error) {
		requestCtx, cancel := d.Timeout(ctx)
		defer cancel()
		if next != "" {
			var err error
			next, err = client.ValidateNoRedirectReadContinuation(requestCtx, next, expectedPath)
			if err != nil {
				return nil, fmt.Errorf("custom page pagination: %w", err)
			}
		}
		return fetch(requestCtx, next)
	}
	first, err := page(ctx, "")
	if err != nil {
		return nil, err
	}
	all, err := asc.PaginateAll(ctx, first, func(ctx context.Context, next string) (asc.PaginatedResponse, error) { return page(ctx, next) })
	if err != nil {
		return nil, err
	}
	return all.(*asc.Response[T]), nil
}
