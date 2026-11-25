package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/storeassets"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const includeLocalizations = "localizations"

// PullResult is the structured output artifact for metadata pull.
type PullResult struct {
	Failure   string   `json:"failure,omitempty"`
	AppID     string   `json:"appId"`
	AppInfoID string   `json:"appInfoId"`
	Version   string   `json:"version"`
	VersionID string   `json:"versionId"`
	Dir       string   `json:"dir"`
	Includes  []string `json:"includes"`
	Locales   []string `json:"locales,omitempty"`
	FileCount int      `json:"fileCount"`
	Files     []string `json:"files"`
}

// newMetadataPullCommand returns the metadata pull subcommand.
func newMetadataPullCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("metadata pull", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	appInfoID := shared.BindResourceIDFlag(fs, "app-info", "appInfos", "App Info ID (optional override for apps with multiple app-infos)")
	version := fs.String("version", "", "App version string (for example 1.2.3) or App Store version ID; defaults to the app's active editable version, then a developer-removed version, else the live version")
	platform := fs.String("platform", "", "Optional platform: IOS, MAC_OS, TV_OS, or VISION_OS")
	dir := fs.String("dir", "", "Output root directory (required)")
	force := fs.Bool("force", false, "Replace existing files in --dir whose values differ from App Store Connect (files that already match are refreshed without it)")
	include := fs.String("include", includeLocalizations, metadataIncludeFlagUsage())
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "pull",
		ShortUsage: "asc metadata pull --app \"APP_ID\" --dir \"./metadata\" [--version \"1.2.3\"] [--app-info \"APP_INFO_ID\"] [flags]",
		ShortHelp:  "Pull metadata from App Store Connect into canonical files.",
		LongHelp: `Pull metadata from App Store Connect into canonical files.

The default scope is localizations. Add --include localizations,app-clip,previews to transfer
App Clip metadata, header images, and App Preview videos. Media uses locale/app_clip,
app_clip/action.txt, and app_previews below --dir.

Opt-in scopes write one file each below --dir:
` + metadataSettingsHelp("  ", false) + `

When --version is omitted, the app's newest active editable App Store version is used
(PREPARE_FOR_SUBMISSION, DEVELOPER_REJECTED, REJECTED, METADATA_REJECTED,
READY_FOR_REVIEW, WAITING_FOR_REVIEW, or INVALID_BINARY), then a
DEVELOPER_REMOVED_FROM_SALE version, and finally the live version. The selected version is reported on stderr. Pass --platform when
the app has candidate versions on more than one platform. --version also takes an
App Store version ID, such as the IDs a version-not-found error lists; the
version's own string and platform are then used.

Pulling again into the same --dir refreshes files that already hold the App Store
Connect values, and App Clip or preview media left unchanged since the last pull.
A file with different values (a local edit not yet pushed) is kept, and pull
fails before writing anything; push it first, or pass --force.

Examples:
  asc metadata pull --app "APP_ID" --dir "./metadata"
  asc metadata pull --app "APP_ID" --version "1.2.3" --dir "./metadata"
  asc metadata pull --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata"
  asc metadata pull --app "APP_ID" --version "VERSION_ID" --dir "./metadata"
  asc metadata pull --app "APP_ID" --app-info "APP_INFO_ID" --version "1.2.3" --dir "./metadata"
  asc metadata pull --app "APP_ID" --version "1.2.3" --dir "./metadata" --force`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			appSelector := d.ResolveAppSelector(*appID)
			resolvedAppID := appSelector.Value
			if resolvedAppID == "" {
				return metadataRequiredInputError(d, "--app", "--app is required (or set ASC_APP_ID)")
			}

			versionValue := strings.TrimSpace(*version)

			dirValue := strings.TrimSpace(*dir)
			if dirValue == "" {
				return metadataRequiredInputError(d, "--dir", "--dir is required")
			}

			platformValue := strings.TrimSpace(*platform)
			if platformValue != "" {
				normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(platformValue)
				if err != nil {
					return d.UsageError(err.Error())
				}
				platformValue = normalizedPlatform
			}

			if err := rejectPlatformInVersion(d, versionValue); err != nil {
				return err
			}

			includes, err := parseIncludes(*include)
			if err != nil {
				return d.UsageError(err.Error())
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("metadata pull: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resolvedAppID, err = shared.ResolveScopedAppID(requestCtx, client, appSelector)
			if err != nil {
				return fmt.Errorf("metadata pull: resolve app: %w", err)
			}

			var versionIDValue, versionStateValue string
			if versionValue == "" {
				resolved, err := d.ResolveAndAnnounceDefaultAppStoreVersion(requestCtx, client, resolvedAppID, platformValue, "--version")
				if err != nil {
					if errors.Is(err, flag.ErrHelp) {
						return err
					}
					return fmt.Errorf("metadata pull: %w", err)
				}
				versionValue = resolved.VersionString
				versionIDValue = resolved.ID
				versionStateValue = resolved.State
				platformValue = resolved.Platform
			} else if appStoreVersionIDPattern.MatchString(versionValue) {
				// The version-not-found error lists each version's ID, so
				// callers retry with one; read its version string and platform.
				version, err := shared.ResolveOwnedAppStoreVersionByID(requestCtx, client, resolvedAppID, versionValue, platformValue)
				if err != nil {
					if diagnostic, ok := shared.DiagnosticFromError(err); ok && diagnostic.Parameter == "--version-id" {
						err = shared.WithDiagnostic(err, diagnostic.Code, "--version")
					}
					return fmt.Errorf("metadata pull: %w", err)
				}
				versionIDValue = version.ID
				versionValue = version.Attributes.VersionString
				versionStateValue = asc.ResolveAppStoreVersionState(version.Attributes)
				platformValue = string(version.Attributes.Platform)
			} else {
				versionIDValue, versionStateValue, err = resolveVersionID(requestCtx, d, client, resolvedAppID, versionValue, platformValue)
				if err != nil {
					if errors.Is(err, flag.ErrHelp) {
						return err
					}
					return fmt.Errorf("metadata pull: %w", err)
				}
			}

			var appInfoIDValue string
			var appInfoItems []asc.Resource[asc.AppInfoLocalizationAttributes]
			var versionItems []asc.Resource[asc.AppStoreVersionLocalizationAttributes]
			settingScopes := selectedMetadataSettingScopes(includes)
			if includesScope(includes, includeLocalizations) || metadataSettingsNeedAppInfo(settingScopes) {
				appInfoIDValue, err = resolveMetadataPullAppInfoID(
					requestCtx,
					d,
					client,
					resolvedAppID,
					strings.TrimSpace(*appInfoID),
					versionValue,
					platformValue,
					dirValue,
					versionStateValue,
				)
				if err != nil {
					return fmt.Errorf("metadata pull: %w", err)
				}
			}
			if includesScope(includes, includeLocalizations) {
				appInfoItems, versionItems, err = fetchMetadataLocalizations(ctx, d, client, appInfoIDValue, versionIDValue, true, true)
				if err != nil {
					return fmt.Errorf("metadata pull: %w", err)
				}
			}

			appInfoByLocale := make(map[string]AppInfoLocalization, len(appInfoItems))
			localeSet := make(map[string]struct{})
			for _, item := range appInfoItems {
				locale := strings.TrimSpace(item.Attributes.Locale)
				if locale == "" {
					continue
				}
				appInfoByLocale[locale] = NormalizeAppInfoLocalization(AppInfoLocalization{
					Name:              item.Attributes.Name,
					Subtitle:          item.Attributes.Subtitle,
					PrivacyPolicyURL:  item.Attributes.PrivacyPolicyURL,
					PrivacyChoicesURL: item.Attributes.PrivacyChoicesURL,
					PrivacyPolicyText: item.Attributes.PrivacyPolicyText,
				})
				localeSet[locale] = struct{}{}
			}

			versionByLocale := make(map[string]VersionLocalization, len(versionItems))
			for _, item := range versionItems {
				locale := strings.TrimSpace(item.Attributes.Locale)
				if locale == "" {
					continue
				}
				versionByLocale[locale] = NormalizeVersionLocalization(VersionLocalization{
					Description:     item.Attributes.Description,
					Keywords:        item.Attributes.Keywords,
					MarketingURL:    item.Attributes.MarketingURL,
					PromotionalText: item.Attributes.PromotionalText,
					SupportURL:      item.Attributes.SupportURL,
					WhatsNew:        item.Attributes.WhatsNew,
				})
				localeSet[locale] = struct{}{}
			}

			plans, err := BuildWritePlans(
				dirValue,
				appInfoByLocale,
				map[string]map[string]VersionLocalization{
					versionValue: versionByLocale,
				},
			)
			if err != nil {
				return fmt.Errorf("metadata pull: %w", err)
			}
			settingTarget := metadataSettingTarget{appInfoID: appInfoIDValue, versionID: versionIDValue}
			for _, scope := range settingScopes {
				values, err := scope.fetch(ctx, d, client, settingTarget)
				if err != nil {
					return fmt.Errorf("metadata pull: read %s: %w", scope.name, err)
				}
				data, err := encodeMetadataSettingFile(scope, values)
				if err != nil {
					return fmt.Errorf("metadata pull: %w", err)
				}
				plans = append(plans, WritePlan{Path: filepath.Join(dirValue, scope.fileName), Contents: data})
			}
			var assetFiles []storeassets.ExportFile
			var assetWarnings []string
			if includesScope(includes, includeLocalizations) {
				assetFiles, assetWarnings, err = storeassets.ExportPlanWithVersionLocalizations(ctx, d, client, versionIDValue, ".", includesScope(includes, "app-clip"), includesScope(includes, "previews"), versionItems)
			} else {
				assetFiles, assetWarnings, err = storeassets.ExportPlan(ctx, d, client, versionIDValue, ".", includesScope(includes, "app-clip"), includesScope(includes, "previews"))
			}
			if err != nil {
				return fmt.Errorf("metadata pull: %w", err)
			}
			for _, warning := range assetWarnings {
				fmt.Fprintln(d.IO.ErrOut, "Warning:", warning)
			}
			assetRoot, err := rootfs.New(dirValue)
			if err != nil {
				return err
			}
			defer assetRoot.Close()
			assetExport, err := storeassets.PrepareExport(d, assetRoot, assetFiles, storeassets.ExportOptions{AppID: resolvedAppID, VersionID: versionIDValue, MetadataPrefix: ".", Clip: includesScope(includes, "app-clip"), Previews: includesScope(includes, "previews"), Overwrite: *force})
			var differingAssets []string
			var differingTargets *storeassets.DifferingTargetsError
			if errors.As(err, &differingTargets) {
				for _, path := range differingTargets.Paths {
					differingAssets = append(differingAssets, filepath.Join(dirValue, filepath.FromSlash(path)))
				}
			} else if err != nil {
				return fmt.Errorf("metadata pull: %w", err)
			}

			if !*force {
				if err := ensureNoExistingPullTargets(d, plans, differingAssets); err != nil {
					return err
				}
			}
			if err := ApplyWritePlans(plans); err != nil {
				return fmt.Errorf("metadata pull: %w", err)
			}

			files := make([]string, 0, len(plans))
			for _, plan := range plans {
				files = append(files, plan.Path)
			}

			assetWritten, cleanupWarnings, assetErr := assetExport.Write(ctx)
			for _, warning := range cleanupWarnings {
				fmt.Fprintln(d.IO.ErrOut, "Warning:", warning)
			}
			for _, path := range assetWritten {
				parts := strings.Split(filepath.ToSlash(path), "/")
				if len(parts) > 1 {
					if parts[0] == "app_previews" && len(parts) > 2 {
						localeSet[parts[1]] = struct{}{}
					} else if parts[0] != "app_clip" {
						localeSet[parts[0]] = struct{}{}
					}
				}
				files = append(files, filepath.Join(dirValue, path))
			}

			locales := make([]string, 0, len(localeSet))
			for locale := range localeSet {
				locales = append(locales, locale)
			}
			sort.Strings(locales)

			result := PullResult{
				AppID:     resolvedAppID,
				AppInfoID: appInfoIDValue,
				Version:   versionValue,
				VersionID: versionIDValue,
				Dir:       dirValue,
				Includes:  includes,
				Locales:   locales,
				FileCount: len(files),
				Files:     files,
			}

			if assetErr != nil {
				result.Failure = shared.SanitizeTerminal(assetErr.Error())
			}
			printErr := d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return printPullResultTable(d, result) },
				func() error { return printPullResultMarkdown(d, result) },
			)
			if printErr != nil {
				return printErr
			}
			if assetErr != nil {
				return fmt.Errorf("metadata pull: partial export: %w", assetErr)
			}
			return nil
		},
	}
}

// ensureNoExistingPullTargets refuses to replace a file whose JSON values
// differ from what pull would write. A file that already holds the same values
// is an earlier pull's output, or an edit pushed since, so refreshing it loses
// nothing; a re-pull into the same --dir then needs no --force. differingAssets
// are the App Clip and preview files the asset export already refused.
func ensureNoExistingPullTargets(d *shared.Deps, plans []WritePlan, differingAssets []string) error {
	var differing []string
	for _, plan := range plans {
		info, err := os.Lstat(plan.Path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("metadata pull: failed to inspect %s: %w", plan.Path, err)
		}
		if info.Mode().IsRegular() {
			existing, err := readFileNoFollow(plan.Path)
			if err != nil {
				return fmt.Errorf("metadata pull: failed to inspect %s: %w", plan.Path, err)
			}
			if sameJSONValues(existing, plan.Contents) {
				continue
			}
		}
		differing = append(differing, plan.Path)
	}
	differing = append(differing, differingAssets...)
	if len(differing) == 0 {
		return nil
	}
	noun := "1 file that differs"
	if len(differing) > 1 {
		noun = fmt.Sprintf("%d files that differ", len(differing))
	}
	message := fmt.Sprintf(
		"refusing to overwrite %s from App Store Connect: %s\nPush local edits first with `asc metadata push`, or rerun with --force to replace them with the App Store Connect values.",
		noun, strings.Join(differing, ", "),
	)
	fmt.Fprintf(d.IO.ErrOut, "Error: %s\n", message)
	return shared.NewReportedUsageError(shared.UsageErrorOther, message)
}

// sameJSONValues reports whether a and b decode to the same JSON value,
// ignoring formatting and key order.
func sameJSONValues(a, b []byte) bool {
	var left, right any
	if json.Unmarshal(a, &left) != nil || json.Unmarshal(b, &right) != nil {
		return false
	}
	return reflect.DeepEqual(left, right)
}

func parseIncludes(value string) ([]string, error) {
	includes := shared.SplitCSV(value)
	if len(includes) == 0 {
		return []string{includeLocalizations}, nil
	}

	unique := make(map[string]struct{})
	for _, item := range includes {
		normalized := strings.ToLower(strings.TrimSpace(item))
		if normalized != includeLocalizations && normalized != "app-clip" && normalized != "previews" && !isMetadataSettingScope(normalized) {
			return nil, fmt.Errorf("--include supports %s", strings.Join(append([]string{includeLocalizations, "app-clip", "previews"}, metadataSettingScopeNames()...), ", "))
		}
		unique[normalized] = struct{}{}
	}

	result := make([]string, 0, len(unique))
	for item := range unique {
		result = append(result, item)
	}
	sort.Strings(result)
	return result, nil
}

// appStoreVersionIDPattern matches an App Store version ID, which App Store
// Connect issues as a UUID; version strings are numeric and dotted.
var appStoreVersionIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// rejectPlatformInVersion answers a --version such as "1.0.0/IOS", which
// joins the version string and platform, with the flags that take them.
func rejectPlatformInVersion(d *shared.Deps, version string) error {
	versionString, suffix, ok := strings.Cut(version, "/")
	if !ok || !shared.IsAppStoreVersionString(strings.TrimSpace(versionString)) {
		return nil
	}
	platform, err := shared.NormalizeAppStoreVersionPlatform(strings.TrimSpace(suffix))
	if err != nil {
		return nil
	}
	return shared.WithDiagnostic(
		d.UsageErrorf("--version takes the version string alone; pass --version %q --platform %s", strings.TrimSpace(versionString), platform),
		shared.DiagnosticInvalidInput,
		"--version",
	)
}

func resolveVersionID(ctx context.Context, d *shared.Deps, client *asc.Client, appID, version, platform string) (string, string, error) {
	id, state, _, err := resolveVersion(ctx, d, client, appID, version, platform)
	return id, state, err
}

// resolveVersion is resolveVersionID that also returns the version's platform.
func resolveVersion(ctx context.Context, d *shared.Deps, client *asc.Client, appID, version, platform string) (string, string, string, error) {
	ctx = asc.WithNoRedirectReads(ctx)
	if platform != "" {
		id, state, err := shared.ResolveAppStoreVersionIDAndState(ctx, client, appID, version, platform)
		return id, state, platform, err
	}

	resp, err := client.GetAppStoreVersions(
		ctx,
		appID,
		asc.WithAppStoreVersionsVersionStrings([]string{version}),
		asc.Limit[asc.AppStoreVersionsOption](200),
	)
	if err != nil {
		return "", "", "", err
	}
	if resp == nil {
		return "", "", "", fmt.Errorf("empty app store versions response")
	}
	pageHasNext := strings.TrimSpace(resp.Links.Next) != ""
	if len(resp.Data) == 0 && !pageHasNext {
		return "", "", "", shared.WithAppStoreVersionNotFoundDiagnostics(ctx, client, appID, version, "", shared.NewErrorWithCause(fmt.Errorf("app store version not found for version %q", version), asc.ErrNotFound))
	}
	if len(resp.Data) > 1 || pageHasNext {
		ambiguous := shared.AmbiguousAppStoreVersionError(version, "", resp.Data, "--platform", "")
		if pageHasNext {
			ambiguous = shared.MarkAmbiguousSelectionSample(ambiguous)
		}
		usageKind := shared.UsageErrorOther
		var selection *shared.AmbiguousSelectionError
		if errors.As(ambiguous, &selection) && selection.Flag == "--platform" {
			usageKind = shared.UsageErrorMissingRequired
		}
		return "", "", "", d.AmbiguousUsageErrorWithKind(ambiguous, usageKind)
	}
	return resp.Data[0].ID, asc.ResolveAppStoreVersionState(resp.Data[0].Attributes), string(resp.Data[0].Attributes.Platform), nil
}

func resolveMetadataPullAppInfoID(
	ctx context.Context,
	d *shared.Deps,
	client *asc.Client,
	appID string,
	appInfoID string,
	version string,
	platform string,
	dir string,
	versionState string,
) (string, error) {
	return resolveMetadataAppInfoID(ctx, d, client, appID, appInfoID, version, platform, dir, versionState, func(aid, v, p, d, infoID string) string {
		return buildMetadataAppInfoExample("pull", aid, v, p, d, infoID)
	})
}

func fetchAppInfoLocalizations(ctx context.Context, d *shared.Deps, client *asc.Client, appInfoID string) ([]asc.Resource[asc.AppInfoLocalizationAttributes], error) {
	ctx = asc.WithNoRedirectReads(ctx)
	firstPage, err := shared.RetryReadWithFreshTimeoutWith(ctx, d, func(requestCtx context.Context) (*asc.AppInfoLocalizationsResponse, error) {
		return client.GetAppInfoLocalizations(requestCtx, appInfoID, asc.Limit[asc.AppInfoLocalizationsOption](200))
	})
	if err != nil {
		return nil, err
	}
	if firstPage == nil || firstPage.Links.Next == "" {
		if firstPage == nil {
			return nil, nil
		}
		return firstPage.Data, nil
	}

	paginated, err := asc.PaginateAll(ctx, firstPage, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		scopedURL, err := client.ValidateNoRedirectReadContinuation(pageCtx, nextURL, fmt.Sprintf("/v1/appInfos/%s/appInfoLocalizations", appInfoID))
		if err != nil {
			return nil, err
		}
		return shared.RetryReadWithFreshTimeoutWith(ctx, d, func(requestCtx context.Context) (*asc.AppInfoLocalizationsResponse, error) {
			return client.GetAppInfoLocalizations(requestCtx, appInfoID, asc.NextURL[asc.AppInfoLocalizationsOption](scopedURL))
		})
	})
	if err != nil {
		return nil, err
	}
	typed, ok := paginated.(*asc.AppInfoLocalizationsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected pagination response type")
	}
	return typed.Data, nil
}

func fetchVersionLocalizations(ctx context.Context, d *shared.Deps, client *asc.Client, versionID string) ([]asc.Resource[asc.AppStoreVersionLocalizationAttributes], error) {
	ctx = asc.WithNoRedirectReads(ctx)
	firstPage, err := shared.RetryReadWithFreshTimeoutWith(ctx, d, func(requestCtx context.Context) (*asc.AppStoreVersionLocalizationsResponse, error) {
		return client.GetAppStoreVersionLocalizations(requestCtx, versionID, asc.Limit[asc.AppStoreVersionLocalizationsOption](200))
	})
	if err != nil {
		return nil, err
	}
	if firstPage == nil || firstPage.Links.Next == "" {
		if firstPage == nil {
			return nil, nil
		}
		return firstPage.Data, nil
	}

	paginated, err := asc.PaginateAll(ctx, firstPage, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		scopedURL, err := client.ValidateNoRedirectReadContinuation(pageCtx, nextURL, fmt.Sprintf("/v1/appStoreVersions/%s/appStoreVersionLocalizations", versionID))
		if err != nil {
			return nil, err
		}
		return shared.RetryReadWithFreshTimeoutWith(ctx, d, func(requestCtx context.Context) (*asc.AppStoreVersionLocalizationsResponse, error) {
			return client.GetAppStoreVersionLocalizations(requestCtx, versionID, asc.NextURL[asc.AppStoreVersionLocalizationsOption](scopedURL))
		})
	})
	if err != nil {
		return nil, err
	}
	typed, ok := paginated.(*asc.AppStoreVersionLocalizationsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected pagination response type")
	}
	return typed.Data, nil
}

func printPullResultTable(d *shared.Deps, result PullResult) error {
	if result.Failure != "" {
		fmt.Fprintf(d.IO.Out, "Failure: %s\n", result.Failure)
	}
	fmt.Fprintf(d.IO.Out, "App ID: %s\n", result.AppID)
	fmt.Fprintf(d.IO.Out, "Version: %s\n", result.Version)
	fmt.Fprintf(d.IO.Out, "Dir: %s\n", result.Dir)
	fmt.Fprintf(d.IO.Out, "Includes: %s\n", strings.Join(result.Includes, ","))
	fmt.Fprintf(d.IO.Out, "File Count: %d\n\n", result.FileCount)

	rows := make([][]string, 0, len(result.Files))
	for _, file := range result.Files {
		rows = append(rows, []string{file})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"(none)"})
	}
	asc.RenderTableTo(d.IO.Out, []string{"file"}, rows)
	return nil
}

func printPullResultMarkdown(d *shared.Deps, result PullResult) error {
	if result.Failure != "" {
		fmt.Fprintf(d.IO.Out, "**Failure:** %s\n\n", result.Failure)
	}
	fmt.Fprintf(d.IO.Out, "**App ID:** %s\n\n", result.AppID)
	fmt.Fprintf(d.IO.Out, "**Version:** %s\n\n", result.Version)
	fmt.Fprintf(d.IO.Out, "**Dir:** %s\n\n", result.Dir)
	fmt.Fprintf(d.IO.Out, "**Includes:** %s\n\n", strings.Join(result.Includes, ","))
	fmt.Fprintf(d.IO.Out, "**File Count:** %d\n\n", result.FileCount)

	rows := make([][]string, 0, len(result.Files))
	for _, file := range result.Files {
		rows = append(rows, []string{file})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"(none)"})
	}
	asc.RenderMarkdownTo(d.IO.Out, []string{"file"}, rows)
	return nil
}
