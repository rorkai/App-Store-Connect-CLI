package metadata

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const includeLocalizations = "localizations"

// PullResult is the structured output artifact for metadata pull.
type PullResult struct {
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

// MetadataPullCommand returns the metadata pull subcommand.
func MetadataPullCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata pull", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	appInfoID := fs.String("app-info", "", "App Info ID (optional override for apps with multiple app-infos)")
	version := fs.String("version", "", "App version string (for example 1.2.3)")
	platform := fs.String("platform", "", "Optional platform: IOS, MAC_OS, TV_OS, or VISION_OS")
	dir := fs.String("dir", "", "Output root directory (required)")
	force := fs.Bool("force", false, "Overwrite existing metadata files in --dir")
	include := fs.String("include", includeLocalizations, "Included metadata scopes (comma-separated)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "pull",
		ShortUsage: "asc metadata pull --app \"APP_ID\" --version \"1.2.3\" --dir \"./metadata\" [--app-info \"APP_INFO_ID\"] [flags]",
		ShortHelp:  "Pull metadata from App Store Connect into canonical files.",
		LongHelp: `Pull metadata from App Store Connect into canonical files.

Phase 1 supports localization metadata for app-info and app-store versions.

Examples:
  asc metadata pull --app "APP_ID" --version "1.2.3" --dir "./metadata"
  asc metadata pull --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata"
  asc metadata pull --app "APP_ID" --app-info "APP_INFO_ID" --version "1.2.3" --dir "./metadata"
  asc metadata pull --app "APP_ID" --version "1.2.3" --dir "./metadata" --force`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("metadata pull does not accept positional arguments")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}

			versionValue := strings.TrimSpace(*version)
			if versionValue == "" {
				return shared.UsageError("--version is required")
			}

			dirValue := strings.TrimSpace(*dir)
			if dirValue == "" {
				return shared.UsageError("--dir is required")
			}

			platformValue := strings.TrimSpace(*platform)
			if platformValue != "" {
				normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(platformValue)
				if err != nil {
					return shared.UsageError(err.Error())
				}
				platformValue = normalizedPlatform
			}

			includes, err := parseIncludes(*include)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("metadata pull: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			versionIDValue, versionStateValue, err := resolveVersionID(requestCtx, client, resolvedAppID, versionValue, platformValue)
			if err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return err
				}
				return fmt.Errorf("metadata pull: %w", err)
			}

			appInfoIDValue, err := resolveMetadataPullAppInfoID(
				requestCtx,
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

			appInfoItems, err := fetchAppInfoLocalizations(requestCtx, client, appInfoIDValue)
			if err != nil {
				return fmt.Errorf("metadata pull: %w", err)
			}
			versionItems, err := fetchVersionLocalizations(requestCtx, client, versionIDValue)
			if err != nil {
				return fmt.Errorf("metadata pull: %w", err)
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
			if !*force {
				if err := ensureNoExistingPullTargets(plans); err != nil {
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

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return printPullResultTable(result) },
				func() error { return printPullResultMarkdown(result) },
			)
		},
	}
}

func ensureNoExistingPullTargets(plans []WritePlan) error {
	for _, plan := range plans {
		if _, err := os.Lstat(plan.Path); err == nil {
			return shared.UsageErrorf("refusing to overwrite existing file %s (use --force)", plan.Path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("metadata pull: failed to inspect %s: %w", plan.Path, err)
		}
	}
	return nil
}

func parseIncludes(value string) ([]string, error) {
	includes := shared.SplitCSV(value)
	if len(includes) == 0 {
		return []string{includeLocalizations}, nil
	}

	unique := make(map[string]struct{})
	for _, item := range includes {
		normalized := strings.ToLower(strings.TrimSpace(item))
		if normalized != includeLocalizations {
			return nil, fmt.Errorf("--include supports only %q", includeLocalizations)
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

func resolveVersionID(ctx context.Context, client *asc.Client, appID, version, platform string) (string, string, error) {
	if platform != "" {
		return shared.ResolveAppStoreVersionIDAndState(ctx, client, appID, version, platform)
	}

	resp, err := client.GetAppStoreVersions(
		ctx,
		appID,
		asc.WithAppStoreVersionsVersionStrings([]string{version}),
		asc.WithAppStoreVersionsLimit(200),
	)
	if err != nil {
		return "", "", err
	}
	if resp == nil || len(resp.Data) == 0 {
		return "", "", fmt.Errorf("app store version not found for version %q", version)
	}
	if len(resp.Data) > 1 {
		return "", "", shared.UsageErrorf("--platform is required when multiple app store versions match --version %q", version)
	}
	return resp.Data[0].ID, asc.ResolveAppStoreVersionState(resp.Data[0].Attributes), nil
}

func resolveMetadataPullAppInfoID(
	ctx context.Context,
	client *asc.Client,
	appID string,
	appInfoID string,
	version string,
	platform string,
	dir string,
	versionState string,
) (string, error) {
	return resolveMetadataAppInfoID(ctx, client, appID, appInfoID, version, platform, dir, versionState, func(aid, v, p, d, infoID string) string {
		return buildMetadataAppInfoExample("pull", aid, v, p, d, infoID)
	})
}

func fetchAppInfoLocalizations(ctx context.Context, client *asc.Client, appInfoID string) ([]asc.Resource[asc.AppInfoLocalizationAttributes], error) {
	firstPage, err := client.GetAppInfoLocalizations(ctx, appInfoID, asc.WithAppInfoLocalizationsLimit(200))
	if err != nil {
		return nil, err
	}
	if firstPage == nil || firstPage.Links.Next == "" {
		if firstPage == nil {
			return nil, nil
		}
		return firstPage.Data, nil
	}

	paginated, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetAppInfoLocalizations(ctx, appInfoID, asc.WithAppInfoLocalizationsNextURL(nextURL))
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

func fetchVersionLocalizations(ctx context.Context, client *asc.Client, versionID string) ([]asc.Resource[asc.AppStoreVersionLocalizationAttributes], error) {
	firstPage, err := client.GetAppStoreVersionLocalizations(ctx, versionID, asc.WithAppStoreVersionLocalizationsLimit(200))
	if err != nil {
		return nil, err
	}
	if firstPage == nil || firstPage.Links.Next == "" {
		if firstPage == nil {
			return nil, nil
		}
		return firstPage.Data, nil
	}

	paginated, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetAppStoreVersionLocalizations(ctx, versionID, asc.WithAppStoreVersionLocalizationsNextURL(nextURL))
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

func printPullResultTable(result PullResult) error {
	fmt.Printf("App ID: %s\n", result.AppID)
	fmt.Printf("Version: %s\n", result.Version)
	fmt.Printf("Dir: %s\n", result.Dir)
	fmt.Printf("Includes: %s\n", strings.Join(result.Includes, ","))
	fmt.Printf("File Count: %d\n\n", result.FileCount)

	rows := make([][]string, 0, len(result.Files))
	for _, file := range result.Files {
		rows = append(rows, []string{file})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"(none)"})
	}
	asc.RenderTable([]string{"file"}, rows)
	return nil
}

func printPullResultMarkdown(result PullResult) error {
	fmt.Printf("**App ID:** %s\n\n", result.AppID)
	fmt.Printf("**Version:** %s\n\n", result.Version)
	fmt.Printf("**Dir:** %s\n\n", result.Dir)
	fmt.Printf("**Includes:** %s\n\n", strings.Join(result.Includes, ","))
	fmt.Printf("**File Count:** %d\n\n", result.FileCount)

	rows := make([][]string, 0, len(result.Files))
	for _, file := range result.Files {
		rows = append(rows, []string{file})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"(none)"})
	}
	asc.RenderMarkdown([]string{"file"}, rows)
	return nil
}
