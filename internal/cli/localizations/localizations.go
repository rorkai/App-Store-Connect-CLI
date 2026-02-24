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

// LocalizationsCommand returns the localizations command with subcommands.
func LocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc localizations <subcommand> [flags]",
		ShortHelp:  "Manage App Store localization metadata.",
		LongHelp: `Manage App Store localization metadata.

Examples:
  asc localizations list --version "VERSION_ID"
  asc localizations search-keywords list --localization-id "LOCALIZATION_ID"
  asc localizations preview-sets list --localization-id "LOCALIZATION_ID"
  asc localizations preview-sets get --id "PREVIEW_SET_ID"
  asc localizations screenshot-sets get --id "SCREENSHOT_SET_ID"
  asc localizations download --version "VERSION_ID" --path "./localizations"
  asc localizations upload --version "VERSION_ID" --path "./localizations"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			LocalizationsListCommand(),
			LocalizationsUpdateCommand(),
			LocalizationsSearchKeywordsCommand(),
			LocalizationsPreviewSetsCommand(),
			LocalizationsScreenshotSetsCommand(),
			LocalizationsDownloadCommand(),
			LocalizationsUploadCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// LocalizationsListCommand returns the list localizations subcommand.
func LocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	versionID := fs.String("version", "", "App Store version ID")
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	appInfoID := fs.String("app-info", "", "App Info ID (optional override)")
	locType := fs.String("type", shared.LocalizationTypeVersion, "Localization type: version (default) or app-info")
	locale := fs.String("locale", "", "Filter by locale(s), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc localizations list [flags]",
		ShortHelp:  "List localization metadata for an app or version.",
		LongHelp: `List localization metadata for an app or version.

Examples:
  asc localizations list --version "VERSION_ID"
  asc localizations list --app "APP_ID" --type app-info
  asc localizations list --version "VERSION_ID" --locale "en-US,ja"
  asc localizations list --version "VERSION_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("localizations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("localizations list: %w", err)
			}

			normalizedType, err := shared.NormalizeLocalizationType(*locType)
			if err != nil {
				return fmt.Errorf("localizations list: %w", err)
			}

			locales := shared.SplitCSV(*locale)

			switch normalizedType {
			case shared.LocalizationTypeVersion:
				if strings.TrimSpace(*versionID) == "" {
					fmt.Fprintln(os.Stderr, "Error: --version is required for version localizations")
					return flag.ErrHelp
				}

				client, err := shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("localizations list: %w", err)
				}

				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				defer cancel()

				opts := []asc.AppStoreVersionLocalizationsOption{
					asc.WithAppStoreVersionLocalizationsLimit(*limit),
					asc.WithAppStoreVersionLocalizationsNextURL(*next),
				}
				if len(locales) > 0 {
					opts = append(opts, asc.WithAppStoreVersionLocalizationLocales(locales))
				}

				if *paginate {
					// Fetch first page with limit set for consistent pagination
					paginateOpts := append(opts, asc.WithAppStoreVersionLocalizationsLimit(200))
					firstPage, err := client.GetAppStoreVersionLocalizations(requestCtx, strings.TrimSpace(*versionID), paginateOpts...)
					if err != nil {
						return fmt.Errorf("localizations list: failed to fetch: %w", err)
					}

					// Fetch all remaining pages
					resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetAppStoreVersionLocalizations(ctx, strings.TrimSpace(*versionID), asc.WithAppStoreVersionLocalizationsNextURL(nextURL))
					})
					if err != nil {
						return fmt.Errorf("localizations list: %w", err)
					}
					return shared.PrintOutput(resp, *output.Output, *output.Pretty)
				}

				resp, err := client.GetAppStoreVersionLocalizations(requestCtx, strings.TrimSpace(*versionID), opts...)
				if err != nil {
					return fmt.Errorf("localizations list: failed to fetch: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			case shared.LocalizationTypeAppInfo:
				resolvedAppID := shared.ResolveAppID(*appID)
				if resolvedAppID == "" {
					fmt.Fprintln(os.Stderr, "Error: --app is required for app-info localizations")
					return flag.ErrHelp
				}

				client, err := shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("localizations list: %w", err)
				}

				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				defer cancel()

				appInfo, err := shared.ResolveAppInfoID(requestCtx, client, resolvedAppID, strings.TrimSpace(*appInfoID))
				if err != nil {
					return fmt.Errorf("localizations list: %w", err)
				}

				opts := []asc.AppInfoLocalizationsOption{
					asc.WithAppInfoLocalizationsLimit(*limit),
					asc.WithAppInfoLocalizationsNextURL(*next),
				}
				if len(locales) > 0 {
					opts = append(opts, asc.WithAppInfoLocalizationLocales(locales))
				}

				if *paginate {
					// Fetch first page with limit set for consistent pagination
					paginateOpts := append(opts, asc.WithAppInfoLocalizationsLimit(200))
					firstPage, err := client.GetAppInfoLocalizations(requestCtx, appInfo, paginateOpts...)
					if err != nil {
						return fmt.Errorf("localizations list: failed to fetch: %w", err)
					}

					// Fetch all remaining pages
					resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetAppInfoLocalizations(ctx, appInfo, asc.WithAppInfoLocalizationsNextURL(nextURL))
					})
					if err != nil {
						return fmt.Errorf("localizations list: %w", err)
					}
					return shared.PrintOutput(resp, *output.Output, *output.Pretty)
				}

				resp, err := client.GetAppInfoLocalizations(requestCtx, appInfo, opts...)
				if err != nil {
					return fmt.Errorf("localizations list: failed to fetch: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			default:
				return fmt.Errorf("localizations list: unsupported type %q", normalizedType)
			}
		},
	}
}

// LocalizationsDownloadCommand returns the download localizations subcommand.
func LocalizationsDownloadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("download", flag.ExitOnError)

	versionID := fs.String("version", "", "App Store version ID")
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	appInfoID := fs.String("app-info", "", "App Info ID (optional override)")
	locType := fs.String("type", shared.LocalizationTypeVersion, "Localization type: version (default) or app-info")
	locale := fs.String("locale", "", "Filter by locale(s), comma-separated")
	path := fs.String("path", "localizations", "Output path (directory or .strings file)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "download",
		ShortUsage: "asc localizations download [flags]",
		ShortHelp:  "Download localizations to .strings files.",
		LongHelp: `Download localizations to .strings files.

Examples:
  asc localizations download --version "VERSION_ID" --path "./localizations"
  asc localizations download --app "APP_ID" --type app-info --path "./localizations"
  asc localizations download --version "VERSION_ID" --locale "en-US" --path "en-US.strings"
  asc localizations download --version "VERSION_ID" --paginate --path "./localizations"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("localizations download: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("localizations download: %w", err)
			}

			normalizedType, err := shared.NormalizeLocalizationType(*locType)
			if err != nil {
				return fmt.Errorf("localizations download: %w", err)
			}

			locales := shared.SplitCSV(*locale)

			switch normalizedType {
			case shared.LocalizationTypeVersion:
				if strings.TrimSpace(*versionID) == "" {
					fmt.Fprintln(os.Stderr, "Error: --version is required for version localizations")
					return flag.ErrHelp
				}

				client, err := shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("localizations download: %w", err)
				}

				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				defer cancel()

				opts := []asc.AppStoreVersionLocalizationsOption{
					asc.WithAppStoreVersionLocalizationsLimit(*limit),
					asc.WithAppStoreVersionLocalizationsNextURL(*next),
				}
				if len(locales) > 0 {
					opts = append(opts, asc.WithAppStoreVersionLocalizationLocales(locales))
				}

				if *paginate {
					paginateOpts := append(opts, asc.WithAppStoreVersionLocalizationsLimit(200))
					firstPage, err := client.GetAppStoreVersionLocalizations(requestCtx, strings.TrimSpace(*versionID), paginateOpts...)
					if err != nil {
						return fmt.Errorf("localizations download: failed to fetch: %w", err)
					}

					resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetAppStoreVersionLocalizations(ctx, strings.TrimSpace(*versionID), asc.WithAppStoreVersionLocalizationsNextURL(nextURL))
					})
					if err != nil {
						return fmt.Errorf("localizations download: %w", err)
					}

					aggregated, ok := resp.(*asc.AppStoreVersionLocalizationsResponse)
					if !ok {
						return fmt.Errorf("localizations download: unexpected pagination response type")
					}

					files, err := shared.WriteVersionLocalizationStrings(*path, aggregated.Data)
					if err != nil {
						return fmt.Errorf("localizations download: %w", err)
					}

					result := asc.LocalizationDownloadResult{
						Type:       normalizedType,
						VersionID:  strings.TrimSpace(*versionID),
						OutputPath: *path,
						Files:      files,
					}

					return shared.PrintOutput(&result, *output.Output, *output.Pretty)
				}

				resp, err := client.GetAppStoreVersionLocalizations(requestCtx, strings.TrimSpace(*versionID), opts...)
				if err != nil {
					return fmt.Errorf("localizations download: failed to fetch: %w", err)
				}

				files, err := shared.WriteVersionLocalizationStrings(*path, resp.Data)
				if err != nil {
					return fmt.Errorf("localizations download: %w", err)
				}

				result := asc.LocalizationDownloadResult{
					Type:       normalizedType,
					VersionID:  strings.TrimSpace(*versionID),
					OutputPath: *path,
					Files:      files,
				}

				return shared.PrintOutput(&result, *output.Output, *output.Pretty)
			case shared.LocalizationTypeAppInfo:
				resolvedAppID := shared.ResolveAppID(*appID)
				if resolvedAppID == "" {
					fmt.Fprintln(os.Stderr, "Error: --app is required for app-info localizations")
					return flag.ErrHelp
				}

				client, err := shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("localizations download: %w", err)
				}

				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				defer cancel()

				appInfo, err := shared.ResolveAppInfoID(requestCtx, client, resolvedAppID, strings.TrimSpace(*appInfoID))
				if err != nil {
					return fmt.Errorf("localizations download: %w", err)
				}

				opts := []asc.AppInfoLocalizationsOption{
					asc.WithAppInfoLocalizationsLimit(*limit),
					asc.WithAppInfoLocalizationsNextURL(*next),
				}
				if len(locales) > 0 {
					opts = append(opts, asc.WithAppInfoLocalizationLocales(locales))
				}

				if *paginate {
					paginateOpts := append(opts, asc.WithAppInfoLocalizationsLimit(200))
					firstPage, err := client.GetAppInfoLocalizations(requestCtx, appInfo, paginateOpts...)
					if err != nil {
						return fmt.Errorf("localizations download: failed to fetch: %w", err)
					}

					resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetAppInfoLocalizations(ctx, appInfo, asc.WithAppInfoLocalizationsNextURL(nextURL))
					})
					if err != nil {
						return fmt.Errorf("localizations download: %w", err)
					}

					aggregated, ok := resp.(*asc.AppInfoLocalizationsResponse)
					if !ok {
						return fmt.Errorf("localizations download: unexpected pagination response type")
					}

					files, err := shared.WriteAppInfoLocalizationStrings(*path, aggregated.Data)
					if err != nil {
						return fmt.Errorf("localizations download: %w", err)
					}

					result := asc.LocalizationDownloadResult{
						Type:       normalizedType,
						AppID:      resolvedAppID,
						AppInfoID:  appInfo,
						OutputPath: *path,
						Files:      files,
					}

					return shared.PrintOutput(&result, *output.Output, *output.Pretty)
				}

				resp, err := client.GetAppInfoLocalizations(requestCtx, appInfo, opts...)
				if err != nil {
					return fmt.Errorf("localizations download: failed to fetch: %w", err)
				}

				files, err := shared.WriteAppInfoLocalizationStrings(*path, resp.Data)
				if err != nil {
					return fmt.Errorf("localizations download: %w", err)
				}

				result := asc.LocalizationDownloadResult{
					Type:       normalizedType,
					AppID:      resolvedAppID,
					AppInfoID:  appInfo,
					OutputPath: *path,
					Files:      files,
				}

				return shared.PrintOutput(&result, *output.Output, *output.Pretty)
			default:
				return fmt.Errorf("localizations download: unsupported type %q", normalizedType)
			}
		},
	}
}

// LocalizationsUploadCommand returns the upload localizations subcommand.
func LocalizationsUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)

	versionID := fs.String("version", "", "App Store version ID")
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	appInfoID := fs.String("app-info", "", "App Info ID (optional override)")
	locType := fs.String("type", shared.LocalizationTypeVersion, "Localization type: version (default) or app-info")
	locale := fs.String("locale", "", "Filter by locale(s), comma-separated")
	path := fs.String("path", "", "Input path (directory or .strings file)")
	dryRun := fs.Bool("dry-run", false, "Validate file without uploading")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc localizations upload [flags]",
		ShortHelp:  "Upload localizations from .strings files.",
		LongHelp: `Upload localizations from .strings files.

Examples:
  asc localizations upload --version "VERSION_ID" --path "./localizations"
  asc localizations upload --app "APP_ID" --type app-info --path "./localizations"
  asc localizations upload --version "VERSION_ID" --locale "en-US" --path "en-US.strings"
  asc localizations upload --version "VERSION_ID" --path "./localizations" --dry-run`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*path) == "" {
				fmt.Fprintln(os.Stderr, "Error: --path is required")
				return flag.ErrHelp
			}

			normalizedType, err := shared.NormalizeLocalizationType(*locType)
			if err != nil {
				return fmt.Errorf("localizations upload: %w", err)
			}

			locales := shared.SplitCSV(*locale)

			switch normalizedType {
			case shared.LocalizationTypeVersion:
				if strings.TrimSpace(*versionID) == "" {
					fmt.Fprintln(os.Stderr, "Error: --version is required for version localizations")
					return flag.ErrHelp
				}

				client, err := shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("localizations upload: %w", err)
				}

				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				defer cancel()

				valuesByLocale, err := shared.ReadLocalizationStrings(*path, locales)
				if err != nil {
					return fmt.Errorf("localizations upload: %w", err)
				}

				results, err := shared.UploadVersionLocalizations(requestCtx, client, strings.TrimSpace(*versionID), valuesByLocale, *dryRun)
				if err != nil {
					return fmt.Errorf("localizations upload: %w", err)
				}

				result := asc.LocalizationUploadResult{
					Type:      normalizedType,
					VersionID: strings.TrimSpace(*versionID),
					DryRun:    *dryRun,
					Results:   results,
				}

				return shared.PrintOutput(&result, *output.Output, *output.Pretty)
			case shared.LocalizationTypeAppInfo:
				resolvedAppID := shared.ResolveAppID(*appID)
				if resolvedAppID == "" {
					fmt.Fprintln(os.Stderr, "Error: --app is required for app-info localizations")
					return flag.ErrHelp
				}

				client, err := shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("localizations upload: %w", err)
				}

				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				defer cancel()

				appInfo, err := shared.ResolveAppInfoID(requestCtx, client, resolvedAppID, strings.TrimSpace(*appInfoID))
				if err != nil {
					return fmt.Errorf("localizations upload: %w", err)
				}

				valuesByLocale, err := shared.ReadLocalizationStrings(*path, locales)
				if err != nil {
					return fmt.Errorf("localizations upload: %w", err)
				}

				results, err := shared.UploadAppInfoLocalizations(requestCtx, client, appInfo, valuesByLocale, *dryRun)
				if err != nil {
					return fmt.Errorf("localizations upload: %w", err)
				}

				result := asc.LocalizationUploadResult{
					Type:      normalizedType,
					AppID:     resolvedAppID,
					AppInfoID: appInfo,
					DryRun:    *dryRun,
					Results:   results,
				}

				return shared.PrintOutput(&result, *output.Output, *output.Pretty)
			default:
				return fmt.Errorf("localizations upload: unsupported type %q", normalizedType)
			}
		},
	}
}
