package assets

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var focusedScreenshotDisplayTypes = []string{
	"APP_IPHONE_65",
	"APP_IPAD_PRO_3GEN_129",
}

var focusedScreenshotDisplayTypesByPlatform = map[string][]string{
	"IOS":       focusedScreenshotDisplayTypes,
	"MAC_OS":    {"APP_DESKTOP"},
	"TV_OS":     {"APP_APPLE_TV"},
	"VISION_OS": {"APP_APPLE_VISION_PRO"},
}

var screenshotFileChecksumFunc = computeFileChecksum

var knownAppStoreLocalizationLocales = func() map[string]struct{} {
	catalog := shared.AppStoreLocalizationCatalog()
	result := make(map[string]struct{}, len(catalog))
	for _, locale := range catalog {
		result[normalizeFanoutLocaleKey(locale.Code)] = struct{}{}
	}
	return result
}()

// ScreenshotSetListFunc fetches screenshot sets for a localization kind.
type ScreenshotSetListFunc func(context.Context, *asc.Client, string) (*asc.AppScreenshotSetsResponse, error)

// ScreenshotSetCreateFunc creates a screenshot set for a localization kind.
type ScreenshotSetCreateFunc func(context.Context, *asc.Client, string, string) (*asc.AppScreenshotSetResponse, error)

// ScreenshotSetAccess contains the list/create hooks for a screenshot-set owner.
type ScreenshotSetAccess struct {
	List   ScreenshotSetListFunc
	Create ScreenshotSetCreateFunc
}

// ScreenshotSetUploadOptions configures the shared screenshot upload path for
// custom product pages and PPO treatment localizations.
type ScreenshotSetUploadOptions[T any] struct {
	LocalizationID           string
	Path                     string
	DeviceType               string
	Replace                  bool
	InvalidDeviceTypeIsUsage bool

	ClientFactory  func() (*asc.Client, error)
	RequestContext func(context.Context) (context.Context, context.CancelFunc)
	UploadContext  func(context.Context) (context.Context, context.CancelFunc)

	Access      ScreenshotSetAccess
	BuildResult func(string, asc.Resource[asc.AppScreenshotSetAttributes], []asc.AssetUploadResultItem) T
}

type screenshotUploadConfig[T any] struct {
	Client         *asc.Client
	LocalizationID string
	DisplayType    string
	Files          []string
	SkipExisting   bool
	Replace        bool
	DryRun         bool
	RequestContext func(context.Context) (context.Context, context.CancelFunc)
	UploadContext  func(context.Context) (context.Context, context.CancelFunc)
	Access         ScreenshotSetAccess
	BuildResult    func(string, asc.Resource[asc.AppScreenshotSetAttributes], bool, []asc.AssetUploadResultItem) T
}

type screenshotUploadCommandOptions struct {
	VersionLocalizationID string
	AppID                 string
	Version               string
	VersionID             string
	Platform              string
	Path                  string
	DeviceType            string
	SkipExisting          bool
	Replace               bool
	DryRun                bool
}

type screenshotUploadDependencies struct {
	GetClient        func() (*asc.Client, error)
	RequestContext   func(context.Context) (context.Context, context.CancelFunc)
	UploadScreenshot func(context.Context, *asc.Client, string, string, []string, bool, bool, bool) (asc.AppScreenshotUploadResult, error)
}

type screenshotUploadFanoutConfig struct {
	Client       *asc.Client
	AppID        string
	Version      string
	VersionID    string
	Platform     string
	RootPath     string
	LocaleAssets []screenshotLocaleAssetFiles
	DisplayType  string
	SkipExisting bool
	Replace      bool
	DryRun       bool

	RequestContext   func(context.Context) (context.Context, context.CancelFunc)
	UploadScreenshot func(context.Context, *asc.Client, string, string, []string, bool, bool, bool) (asc.AppScreenshotUploadResult, error)
}

type screenshotLocaleAssetFiles struct {
	Locale string
	Files  []string
}

var appStoreVersionScreenshotSetAccess = ScreenshotSetAccess{
	List: func(ctx context.Context, client *asc.Client, localizationID string) (*asc.AppScreenshotSetsResponse, error) {
		return client.GetAppScreenshotSets(ctx, localizationID)
	},
	Create: func(ctx context.Context, client *asc.Client, localizationID, displayType string) (*asc.AppScreenshotSetResponse, error) {
		return client.CreateAppScreenshotSet(ctx, localizationID, displayType)
	},
}

func focusedScreenshotSizeCatalog() []asc.ScreenshotSizeEntry {
	focused := make([]asc.ScreenshotSizeEntry, 0, len(focusedScreenshotDisplayTypes))
	for _, displayType := range focusedScreenshotDisplayTypes {
		entry, ok := asc.ScreenshotSizeEntryForDisplayType(displayType)
		if !ok {
			continue
		}
		focused = append(focused, entry)
	}
	if len(focused) == 0 {
		return asc.ScreenshotSizeCatalog()
	}
	return focused
}

func focusedScreenshotDisplayTypesForPlatform(platform string) []string {
	normalized := strings.ToUpper(strings.TrimSpace(platform))
	if focused, ok := focusedScreenshotDisplayTypesByPlatform[normalized]; ok {
		return append([]string(nil), focused...)
	}
	return nil
}

// ExecuteScreenshotSetUpload validates flags/files and runs the shared
// screenshot upload/sync flow for a localization-backed screenshot set.
func ExecuteScreenshotSetUpload[T any](ctx context.Context, opts ScreenshotSetUploadOptions[T]) (T, error) {
	var zero T

	trimmedLocalizationID := strings.TrimSpace(opts.LocalizationID)
	if trimmedLocalizationID == "" {
		fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
		return zero, flag.ErrHelp
	}
	trimmedPath := strings.TrimSpace(opts.Path)
	if trimmedPath == "" {
		fmt.Fprintln(os.Stderr, "Error: --path is required")
		return zero, flag.ErrHelp
	}
	trimmedDeviceType := strings.TrimSpace(opts.DeviceType)
	if trimmedDeviceType == "" {
		fmt.Fprintln(os.Stderr, "Error: --device-type is required")
		return zero, flag.ErrHelp
	}
	if opts.ClientFactory == nil {
		return zero, fmt.Errorf("client factory is required")
	}
	if opts.BuildResult == nil {
		return zero, fmt.Errorf("build result function is required")
	}

	displayType, err := normalizeScreenshotDisplayType(trimmedDeviceType)
	if err != nil {
		if opts.InvalidDeviceTypeIsUsage {
			return zero, shared.UsageError(err.Error())
		}
		return zero, err
	}
	apiDisplayType := asc.CanonicalScreenshotDisplayTypeForAPI(displayType)
	files, err := CollectAssetFiles(trimmedPath)
	if err != nil {
		return zero, err
	}
	if err := ValidateScreenshotDimensions(files, apiDisplayType); err != nil {
		return zero, err
	}

	client, err := opts.ClientFactory()
	if err != nil {
		return zero, err
	}

	return uploadScreenshotsWithConfig(ctx, screenshotUploadConfig[T]{
		Client:         client,
		LocalizationID: trimmedLocalizationID,
		DisplayType:    apiDisplayType,
		Files:          files,
		Replace:        opts.Replace,
		RequestContext: opts.RequestContext,
		UploadContext:  opts.UploadContext,
		Access:         opts.Access,
		BuildResult: func(localizationID string, set asc.Resource[asc.AppScreenshotSetAttributes], _ bool, results []asc.AssetUploadResultItem) T {
			return opts.BuildResult(localizationID, set, results)
		},
	})
}

// AssetsScreenshotsListCommand returns the screenshots list subcommand.
func AssetsScreenshotsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	localizationID := fs.String("version-localization", "", "App Store version localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc screenshots list --version-localization \"LOC_ID\"",
		ShortHelp:  "List screenshots for a localization.",
		LongHelp: `List screenshots for a localization.

Examples:
  asc screenshots list --version-localization "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locID := strings.TrimSpace(*localizationID)
			if locID == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-localization is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("screenshots list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			setsResp, err := client.GetAppScreenshotSets(requestCtx, locID)
			if err != nil {
				return fmt.Errorf("screenshots list: failed to fetch sets: %w", err)
			}

			result := asc.AppScreenshotListResult{
				VersionLocalizationID: locID,
				Sets:                  make([]asc.AppScreenshotSetWithScreenshots, 0, len(setsResp.Data)),
			}

			for _, set := range setsResp.Data {
				screenshots, err := client.GetAppScreenshots(requestCtx, set.ID)
				if err != nil {
					return fmt.Errorf("screenshots list: failed to fetch screenshots for set %s: %w", set.ID, err)
				}
				result.Sets = append(result.Sets, asc.AppScreenshotSetWithScreenshots{
					Set:         set,
					Screenshots: screenshots.Data,
				})
			}

			return shared.PrintOutput(&result, *output.Output, *output.Pretty)
		},
	}
}

// AssetsScreenshotsSizesCommand returns the screenshots sizes subcommand.
func AssetsScreenshotsSizesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("sizes", flag.ExitOnError)

	displayType := fs.String("display-type", "", "Filter by screenshot display type (e.g., APP_IPHONE_65)")
	all := fs.Bool("all", false, "List all supported screenshot display types")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "sizes",
		ShortUsage: "asc screenshots sizes [--display-type \"APP_IPHONE_65\" | --all]",
		ShortHelp:  "List supported screenshot display sizes.",
		LongHelp: `List supported screenshot display sizes.

By default this command focuses on common iOS submission slots:
APP_IPHONE_65 and APP_IPAD_PRO_3GEN_129.

Examples:
  asc screenshots sizes
  asc screenshots sizes --all
  asc screenshots sizes --display-type "APP_IPHONE_65"
  asc screenshots sizes --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			filter := strings.TrimSpace(*displayType)
			if filter != "" && *all {
				return shared.UsageError("--display-type and --all are mutually exclusive")
			}

			result := asc.ScreenshotSizesResult{}

			if filter != "" {
				normalized, err := normalizeScreenshotDisplayType(filter)
				if err != nil {
					return shared.UsageError(err.Error())
				}
				entry, ok := asc.ScreenshotSizeEntryForDisplayType(normalized)
				if !ok {
					return fmt.Errorf("screenshots sizes: unsupported screenshot display type %q", normalized)
				}
				result.Sizes = []asc.ScreenshotSizeEntry{entry}
			} else if *all {
				result.Sizes = asc.ScreenshotSizeCatalog()
			} else {
				result.Sizes = focusedScreenshotSizeCatalog()
			}

			return shared.PrintOutput(&result, *output.Output, *output.Pretty)
		},
	}
}

// AssetsScreenshotsUploadCommand returns the screenshots upload subcommand.
func AssetsScreenshotsUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)

	localizationID := fs.String("version-localization", "", "App Store version localization ID")
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	version := fs.String("version", "", "App Store version string for app-scoped fan-out uploads")
	versionID := fs.String("version-id", "", "App Store version ID for app-scoped fan-out uploads")
	platform := fs.String("platform", "", "Platform for app-scoped fan-out uploads: IOS, MAC_OS, TV_OS, VISION_OS (default: IOS)")
	path := fs.String("path", "", "Path to screenshot file or directory")
	deviceType := fs.String("device-type", "", "Device type (e.g., IPHONE_65 or IPAD_PRO_3GEN_129)")
	skipExisting := fs.Bool("skip-existing", false, "Skip files whose MD5 checksum already exists in the target screenshot set")
	replace := fs.Bool("replace", false, "Delete all existing screenshots from the target set before uploading")
	dryRun := fs.Bool("dry-run", false, "Show what would be uploaded, skipped, or deleted without making changes")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc screenshots upload (--version-localization \"LOC_ID\" | --app \"APP_ID\" (--version \"1.2.3\" | --version-id \"VERSION_ID\")) --path \"./screenshots\" --device-type \"IPHONE_65\"",
		ShortHelp:  "Upload screenshots for one or more localizations.",
		LongHelp: `Upload screenshots for one or more localizations.

Use --version-localization for a single localization upload, or use --app with
--version/--version-id to fan out one run across locale directories under
--path. In fan-out mode, the immediate children of --path must be locale
directories. Each locale subtree is scanned recursively, and only files
matching --device-type are uploaded. This supports layouts like
./screenshots/en-US/iphone/*.png, or ./screenshots/iphone/en-US/*.png when
--path points to ./screenshots/iphone.

Examples:
  asc screenshots upload --version-localization "LOC_ID" --path "./screenshots" --device-type "IPHONE_65"
  asc screenshots upload --version-localization "LOC_ID" --path "./screenshots" --device-type "IPHONE_65" --skip-existing
  asc screenshots upload --version-localization "LOC_ID" --path "./screenshots" --device-type "IPHONE_65" --replace
  asc screenshots upload --version-localization "LOC_ID" --path "./screenshots" --device-type "IPHONE_65" --skip-existing --dry-run
  asc screenshots upload --version-localization "LOC_ID" --path "./screenshots" --device-type "IPAD_PRO_3GEN_129"
  asc screenshots upload --version-localization "LOC_ID" --path "./screenshots/en-US.png" --device-type "IPHONE_65"
  asc screenshots upload --app "123456789" --version "1.2.3" --path "./screenshots" --device-type "IPHONE_65"
  asc screenshots upload --app "123456789" --version-id "VERSION_ID" --path "./screenshots/ipad" --device-type "IPAD_PRO_3GEN_129" --dry-run`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			result, err := executeScreenshotUploadCommand(ctx, screenshotUploadCommandOptions{
				VersionLocalizationID: *localizationID,
				AppID:                 *appID,
				Version:               *version,
				VersionID:             *versionID,
				Platform:              *platform,
				Path:                  *path,
				DeviceType:            *deviceType,
				SkipExisting:          *skipExisting,
				Replace:               *replace,
				DryRun:                *dryRun,
			}, screenshotUploadDependencies{
				GetClient:        shared.GetASCClient,
				RequestContext:   shared.ContextWithTimeout,
				UploadScreenshot: uploadScreenshots,
			})
			if err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return err
				}
				return fmt.Errorf("screenshots upload: %w", err)
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func executeScreenshotUploadCommand(ctx context.Context, opts screenshotUploadCommandOptions, deps screenshotUploadDependencies) (any, error) {
	if deps.GetClient == nil {
		deps.GetClient = shared.GetASCClient
	}
	if deps.RequestContext == nil {
		deps.RequestContext = shared.ContextWithTimeout
	}
	if deps.UploadScreenshot == nil {
		deps.UploadScreenshot = uploadScreenshots
	}

	locID := strings.TrimSpace(opts.VersionLocalizationID)
	appFlagValue := strings.TrimSpace(opts.AppID)
	versionValue := strings.TrimSpace(opts.Version)
	versionIDValue := strings.TrimSpace(opts.VersionID)
	platformValue := strings.TrimSpace(opts.Platform)
	appModeRequested := appFlagValue != "" || versionValue != "" || versionIDValue != "" || platformValue != ""

	if locID == "" {
		if !appModeRequested {
			fmt.Fprintln(os.Stderr, "Error: --version-localization is required")
			return nil, flag.ErrHelp
		}
	} else if appModeRequested {
		fmt.Fprintln(os.Stderr, "Error: --version-localization cannot be combined with --app, --version, --version-id, or --platform")
		return nil, flag.ErrHelp
	}

	if locID == "" {
		resolvedAppValue := shared.ResolveAppID(appFlagValue)
		if resolvedAppValue == "" {
			fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
			return nil, flag.ErrHelp
		}
		if versionValue == "" && versionIDValue == "" {
			fmt.Fprintln(os.Stderr, "Error: --version or --version-id is required with --app")
			return nil, flag.ErrHelp
		}
		if versionValue != "" && versionIDValue != "" {
			fmt.Fprintln(os.Stderr, "Error: --version and --version-id are mutually exclusive")
			return nil, flag.ErrHelp
		}
		appFlagValue = resolvedAppValue
	}

	pathValue := strings.TrimSpace(opts.Path)
	if pathValue == "" {
		fmt.Fprintln(os.Stderr, "Error: --path is required")
		return nil, flag.ErrHelp
	}
	deviceValue := strings.TrimSpace(opts.DeviceType)
	if deviceValue == "" {
		fmt.Fprintln(os.Stderr, "Error: --device-type is required")
		return nil, flag.ErrHelp
	}
	if opts.SkipExisting && opts.Replace {
		fmt.Fprintln(os.Stderr, "Error: --skip-existing and --replace are mutually exclusive")
		return nil, flag.ErrHelp
	}

	displayType, err := normalizeScreenshotDisplayType(deviceValue)
	if err != nil {
		return nil, shared.UsageError(err.Error())
	}
	apiDisplayType := asc.CanonicalScreenshotDisplayTypeForAPI(displayType)

	if locID != "" {
		files, err := collectAssetFiles(pathValue)
		if err != nil {
			return nil, err
		}
		if err := validateScreenshotDimensions(files, apiDisplayType); err != nil {
			return nil, err
		}
		client, err := deps.GetClient()
		if err != nil {
			return nil, err
		}
		result, err := deps.UploadScreenshot(ctx, client, locID, apiDisplayType, files, opts.SkipExisting, opts.Replace, opts.DryRun)
		if err != nil {
			return nil, err
		}
		return &result, nil
	}

	normalizedPlatform := "IOS"
	if platformValue != "" {
		normalizedPlatform, err = shared.NormalizeAppStoreVersionPlatform(platformValue)
		if err != nil {
			return nil, shared.UsageError(err.Error())
		}
	}

	localeAssets, err := collectLocaleAssetFiles(pathValue, apiDisplayType)
	if err != nil {
		return nil, err
	}

	client, err := deps.GetClient()
	if err != nil {
		return nil, err
	}

	requestCtx, cancel := deps.RequestContext(ctx)
	defer cancel()

	resolvedAppID, err := shared.ResolveAppIDWithLookup(requestCtx, client, appFlagValue)
	if err != nil {
		return nil, err
	}
	resolvedVersionID, resolvedVersion, resolvedPlatform, err := resolveScreenshotPlanVersion(requestCtx, client, resolvedAppID, versionValue, versionIDValue, normalizedPlatform)
	if err != nil {
		return nil, err
	}

	result, err := uploadScreenshotsFanout(ctx, screenshotUploadFanoutConfig{
		Client:           client,
		AppID:            resolvedAppID,
		Version:          resolvedVersion,
		VersionID:        resolvedVersionID,
		Platform:         resolvedPlatform,
		RootPath:         pathValue,
		LocaleAssets:     localeAssets,
		DisplayType:      apiDisplayType,
		SkipExisting:     opts.SkipExisting,
		Replace:          opts.Replace,
		DryRun:           opts.DryRun,
		RequestContext:   deps.RequestContext,
		UploadScreenshot: deps.UploadScreenshot,
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func uploadScreenshotsFanout(ctx context.Context, cfg screenshotUploadFanoutConfig) (asc.AppScreenshotFanoutUploadResult, error) {
	var zero asc.AppScreenshotFanoutUploadResult

	if cfg.Client == nil {
		return zero, fmt.Errorf("client is required")
	}
	if cfg.RequestContext == nil {
		cfg.RequestContext = shared.ContextWithTimeout
	}
	if cfg.UploadScreenshot == nil {
		cfg.UploadScreenshot = uploadScreenshots
	}

	localeAssets := cfg.LocaleAssets
	if localeAssets == nil {
		var err error
		localeAssets, err = collectLocaleAssetFiles(cfg.RootPath, cfg.DisplayType)
		if err != nil {
			return zero, err
		}
	}
	localeAssets, err := canonicalizeUniqueScreenshotFanoutLocaleAssets(localeAssets)
	if err != nil {
		return zero, err
	}

	requestCtx, cancel := cfg.RequestContext(ctx)
	localizationsResp, err := cfg.Client.GetAppStoreVersionLocalizations(requestCtx, cfg.VersionID, asc.WithAppStoreVersionLocalizationsLimit(200))
	cancel()
	if err != nil {
		return zero, fmt.Errorf("fetch version localizations: %w", err)
	}

	localizationIDsByLocale := make(map[string]string, len(localizationsResp.Data))
	for _, item := range localizationsResp.Data {
		localeKey := normalizeFanoutLocaleKey(item.Attributes.Locale)
		if localeKey == "" {
			continue
		}
		localizationIDsByLocale[localeKey] = strings.TrimSpace(item.ID)
	}

	missingLocales := make([]string, 0)
	for _, item := range localeAssets {
		if localizationIDsByLocale[normalizeFanoutLocaleKey(item.Locale)] == "" {
			missingLocales = append(missingLocales, item.Locale)
		}
	}
	if len(missingLocales) > 0 {
		sort.Strings(missingLocales)
		return zero, fmt.Errorf("no matching App Store version localizations found for locales: %s", strings.Join(missingLocales, ", "))
	}

	result := asc.AppScreenshotFanoutUploadResult{
		AppID:         cfg.AppID,
		Version:       cfg.Version,
		VersionID:     cfg.VersionID,
		Platform:      cfg.Platform,
		DisplayType:   cfg.DisplayType,
		DryRun:        cfg.DryRun,
		Localizations: make([]asc.AppScreenshotLocalizationUploadResult, 0, len(localeAssets)),
	}

	for _, item := range localeAssets {
		localizationID := localizationIDsByLocale[normalizeFanoutLocaleKey(item.Locale)]
		uploadResult, err := cfg.UploadScreenshot(ctx, cfg.Client, localizationID, cfg.DisplayType, item.Files, cfg.SkipExisting, cfg.Replace, cfg.DryRun)
		if err != nil {
			return zero, fmt.Errorf("upload locale %s: %w", item.Locale, err)
		}
		result.Localizations = append(result.Localizations, asc.AppScreenshotLocalizationUploadResult{
			Locale:                item.Locale,
			VersionLocalizationID: uploadResult.VersionLocalizationID,
			SetID:                 uploadResult.SetID,
			DisplayType:           uploadResult.DisplayType,
			DryRun:                uploadResult.DryRun,
			Results:               uploadResult.Results,
		})
	}

	return result, nil
}

func collectLocaleAssetFiles(rootPath, displayType string) ([]screenshotLocaleAssetFiles, error) {
	info, err := os.Lstat(rootPath)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to read symlink %q", rootPath)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("fan-out upload path %q must be a directory containing locale subdirectories", rootPath)
	}

	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return nil, err
	}

	results := make([]screenshotLocaleAssetFiles, 0, len(entries))
	seenLocales := make(map[string]string, len(entries))
	for _, entry := range entries {
		if shouldIgnoreFanoutEntryName(entry.Name()) {
			continue
		}

		entryPath := filepath.Join(rootPath, entry.Name())
		info, err := os.Lstat(entryPath)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("refusing to read symlink %q", entryPath)
		}
		if !info.IsDir() {
			continue
		}

		locale, err := shared.CanonicalizeAppStoreLocalizationLocale(entry.Name())
		if err != nil {
			hasMatchingFiles, matchErr := directoryContainsMatchingScreenshotFiles(entryPath, displayType)
			if matchErr != nil {
				return nil, matchErr
			}
			if !hasMatchingFiles {
				continue
			}
			return nil, fmt.Errorf("invalid locale directory %q: %w", entry.Name(), err)
		}
		if !isKnownAppStoreLocalizationLocale(locale) {
			hasMatchingFiles, matchErr := directoryContainsMatchingScreenshotFiles(entryPath, displayType)
			if matchErr != nil {
				return nil, matchErr
			}
			if !hasMatchingFiles {
				continue
			}
		}
		files, err := collectLocaleAssetFilesRecursive(entryPath, displayType)
		if err != nil {
			return nil, fmt.Errorf("locale %s: %w", locale, err)
		}
		if err := registerUniqueCanonicalFanoutLocale(locale, entry.Name(), "fan-out path", "dirs", seenLocales); err != nil {
			return nil, err
		}
		results = append(results, screenshotLocaleAssetFiles{
			Locale: locale,
			Files:  files,
		})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no locale directories found in %q", rootPath)
	}

	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Locale) < strings.ToLower(results[j].Locale)
	})
	return results, nil
}

type screenshotMatchWalkOptions struct {
	ignoreInvalidFiles bool
	ignoreSymlinks     bool
	onMatch            func(path string) error
}

func walkMatchingScreenshotFiles(rootPath, displayType string, opts screenshotMatchWalkOptions) error {
	return filepath.WalkDir(rootPath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != rootPath && shouldIgnoreFanoutEntryName(entry.Name()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if opts.ignoreSymlinks {
				return nil
			}
			return fmt.Errorf("refusing to read symlink %q", path)
		}
		if entry.IsDir() {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || !isSupportedScreenshotUploadFile(path) {
			return nil
		}
		if err := asc.ValidateImageFile(path); err != nil {
			if opts.ignoreInvalidFiles {
				return nil
			}
			return err
		}
		matches, err := screenshotMatchesDisplayType(path, displayType)
		if err != nil {
			if opts.ignoreInvalidFiles {
				return nil
			}
			return err
		}
		if !matches || opts.onMatch == nil {
			return nil
		}
		return opts.onMatch(path)
	})
}

func collectLocaleAssetFilesRecursive(rootPath, displayType string) ([]string, error) {
	files := make([]string, 0)
	err := walkMatchingScreenshotFiles(rootPath, displayType, screenshotMatchWalkOptions{
		onMatch: func(path string) error {
			files = append(files, path)
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no screenshot files matching %s found in %q", displayType, rootPath)
	}
	sort.Strings(files)
	return files, nil
}

func directoryContainsMatchingScreenshotFiles(rootPath, displayType string) (bool, error) {
	found := false
	err := walkMatchingScreenshotFiles(rootPath, displayType, screenshotMatchWalkOptions{
		ignoreInvalidFiles: true,
		ignoreSymlinks:     true,
		onMatch: func(path string) error {
			found = true
			return filepath.SkipAll
		},
	})
	if err != nil && !errors.Is(err, filepath.SkipAll) {
		return false, err
	}
	return found, nil
}

func canonicalizeUniqueScreenshotFanoutLocaleAssets(localeAssets []screenshotLocaleAssetFiles) ([]screenshotLocaleAssetFiles, error) {
	result := make([]screenshotLocaleAssetFiles, 0, len(localeAssets))
	seen := make(map[string]string, len(localeAssets))
	for _, item := range localeAssets {
		canonicalLocale, err := shared.CanonicalizeAppStoreLocalizationLocale(item.Locale)
		if err != nil {
			return nil, fmt.Errorf("invalid locale %q in fan-out upload: %w", item.Locale, err)
		}
		if err := registerUniqueCanonicalFanoutLocale(canonicalLocale, item.Locale, "fan-out upload", "inputs", seen); err != nil {
			return nil, err
		}
		result = append(result, screenshotLocaleAssetFiles{
			Locale: canonicalLocale,
			Files:  item.Files,
		})
	}
	return result, nil
}

func registerUniqueCanonicalFanoutLocale(canonicalLocale, source, scope, itemLabel string, seen map[string]string) error {
	localeKey := normalizeFanoutLocaleKey(canonicalLocale)
	if previous, ok := seen[localeKey]; ok {
		return fmt.Errorf("duplicate locale %q in %s (%s: %q, %q)", canonicalLocale, scope, itemLabel, previous, source)
	}
	seen[localeKey] = source
	return nil
}

func normalizeFanoutLocaleKey(locale string) string {
	return strings.ToLower(shared.NormalizeLocaleCode(locale))
}

func isKnownAppStoreLocalizationLocale(locale string) bool {
	_, ok := knownAppStoreLocalizationLocales[normalizeFanoutLocaleKey(locale)]
	return ok
}

func screenshotMatchesDisplayType(path, displayType string) (bool, error) {
	allowed, ok := asc.ScreenshotDimensions(displayType)
	if !ok {
		return false, fmt.Errorf("unsupported screenshot display type %q", displayType)
	}

	dims, err := asc.ReadImageDimensions(path)
	if err != nil {
		return false, err
	}

	for _, dim := range allowed {
		if dim.Width == dims.Width && dim.Height == dims.Height {
			return true, nil
		}
	}
	return false, nil
}

func shouldIgnoreFanoutEntryName(name string) bool {
	return strings.HasPrefix(name, ".")
}

func isSupportedScreenshotUploadFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg":
		return true
	default:
		return false
	}
}

type screenshotDownloadItem struct {
	ID          string `json:"id"`
	DisplayType string `json:"displayType,omitempty"`
	FileName    string `json:"fileName,omitempty"`
	URL         string `json:"url,omitempty"`
	OutputPath  string `json:"outputPath"`

	ContentType  string `json:"contentType,omitempty"`
	BytesWritten int64  `json:"bytesWritten,omitempty"`
}

type screenshotDownloadFailure struct {
	ID          string `json:"id,omitempty"`
	DisplayType string `json:"displayType,omitempty"`
	URL         string `json:"url,omitempty"`
	OutputPath  string `json:"outputPath,omitempty"`
	Error       string `json:"error"`
}

type screenshotDownloadResult struct {
	VersionLocalizationID string `json:"versionLocalizationId,omitempty"`
	OutputDir             string `json:"outputDir,omitempty"`
	Overwrite             bool   `json:"overwrite"`

	Total      int `json:"total"`
	Downloaded int `json:"downloaded"`
	Failed     int `json:"failed"`

	Items    []screenshotDownloadItem    `json:"items,omitempty"`
	Failures []screenshotDownloadFailure `json:"failures,omitempty"`
}

// AssetsScreenshotsDownloadCommand returns the screenshots download subcommand.
func AssetsScreenshotsDownloadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("download", flag.ExitOnError)

	id := fs.String("id", "", "Screenshot ID to download")
	localizationID := fs.String("version-localization", "", "App Store version localization ID (download all screenshots)")
	outputPath := fs.String("output", "", "Output file path (required with --id)")
	outputDir := fs.String("output-dir", "", "Output directory (required with --version-localization)")
	overwrite := fs.Bool("overwrite", false, "Overwrite existing files")
	format := shared.BindOutputFlagsWith(fs, "format", "json", "Summary output format: json (default), table, markdown")

	return &ffcli.Command{
		Name:       "download",
		ShortUsage: "asc screenshots download (--id \"SCREENSHOT_ID\" --output \"./screenshot.png\") | (--version-localization \"LOC_ID\" --output-dir \"./screenshots\")",
		ShortHelp:  "Download App Store screenshots to disk.",
		LongHelp: `Download App Store screenshots to disk.

Examples:
  asc screenshots download --id "SCREENSHOT_ID" --output "./screenshot.png"
  asc screenshots download --version-localization "LOC_ID" --output-dir "./screenshots"
  asc screenshots download --version-localization "LOC_ID" --output-dir "./screenshots" --overwrite`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			locID := strings.TrimSpace(*localizationID)

			if idValue == "" && locID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id or --version-localization is required")
				return flag.ErrHelp
			}
			if idValue != "" && locID != "" {
				return shared.UsageError("--id and --version-localization are mutually exclusive")
			}

			outputFile := strings.TrimSpace(*outputPath)
			outputDirValue := strings.TrimSpace(*outputDir)
			if idValue != "" {
				if outputFile == "" {
					fmt.Fprintln(os.Stderr, "Error: --output is required with --id")
					return flag.ErrHelp
				}
				if strings.HasSuffix(outputFile, string(filepath.Separator)) {
					return shared.UsageError("--output must be a file path")
				}
			}
			if locID != "" {
				if outputDirValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --output-dir is required with --version-localization")
					return flag.ErrHelp
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("screenshots download: %w", err)
			}

			cleanOutputDir := ""
			if outputDirValue != "" {
				cleanOutputDir = filepath.Clean(outputDirValue)
			}
			result := &screenshotDownloadResult{
				VersionLocalizationID: locID,
				OutputDir:             cleanOutputDir,
				Overwrite:             *overwrite,
			}

			items := make([]screenshotDownloadItem, 0, 8)

			if idValue != "" {
				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				resp, err := client.GetAppScreenshot(requestCtx, idValue)
				cancel()
				if err != nil {
					return fmt.Errorf("screenshots download: failed to fetch screenshot: %w", err)
				}

				downloadURL, err := resolveImageAssetDownloadURL(resp.Data.Attributes.ImageAsset, resp.Data.Attributes.FileName)
				if err != nil {
					items = append(items, screenshotDownloadItem{
						ID:         idValue,
						FileName:   strings.TrimSpace(resp.Data.Attributes.FileName),
						OutputPath: outputFile,
					})
					result.Items = items
					result.Failures = append(result.Failures, screenshotDownloadFailure{
						ID:         idValue,
						OutputPath: outputFile,
						Error:      err.Error(),
					})
					result.Total = 1
					result.Failed = 1

					if err := shared.PrintOutputWithRenderers(
						result,
						*format.Output,
						*format.Pretty,
						func() error { return renderScreenshotDownloadResult(result, false) },
						func() error { return renderScreenshotDownloadResult(result, true) },
					); err != nil {
						return err
					}
					return shared.NewReportedError(fmt.Errorf("screenshots download: 1 file failed"))
				}

				items = append(items, screenshotDownloadItem{
					ID:         idValue,
					FileName:   strings.TrimSpace(resp.Data.Attributes.FileName),
					URL:        downloadURL,
					OutputPath: outputFile,
				})
			} else {
				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				setsResp, err := client.GetAppScreenshotSets(requestCtx, locID)
				cancel()
				if err != nil {
					return fmt.Errorf("screenshots download: failed to fetch sets: %w", err)
				}

				sets := make([]asc.Resource[asc.AppScreenshotSetAttributes], 0, len(setsResp.Data))
				sets = append(sets, setsResp.Data...)
				sort.Slice(sets, func(i, j int) bool {
					di := strings.ToUpper(strings.TrimSpace(sets[i].Attributes.ScreenshotDisplayType))
					dj := strings.ToUpper(strings.TrimSpace(sets[j].Attributes.ScreenshotDisplayType))
					if di == dj {
						return sets[i].ID < sets[j].ID
					}
					return di < dj
				})

				for _, set := range sets {
					displayType := strings.TrimSpace(set.Attributes.ScreenshotDisplayType)

					requestCtx, cancel := shared.ContextWithTimeout(ctx)
					shotsResp, err := client.GetAppScreenshots(requestCtx, set.ID)
					cancel()
					if err != nil {
						return fmt.Errorf("screenshots download: failed to fetch screenshots for set %s: %w", set.ID, err)
					}

					shots := make([]asc.Resource[asc.AppScreenshotAttributes], 0, len(shotsResp.Data))
					shots = append(shots, shotsResp.Data...)
					sort.Slice(shots, func(i, j int) bool {
						fi := strings.ToLower(strings.TrimSpace(shots[i].Attributes.FileName))
						fj := strings.ToLower(strings.TrimSpace(shots[j].Attributes.FileName))
						if fi == fj {
							return shots[i].ID < shots[j].ID
						}
						return fi < fj
					})

					for idx, shot := range shots {
						base := sanitizeBaseFileName(shot.Attributes.FileName)
						if base == "" {
							base = strings.TrimSpace(shot.ID)
						}
						if base == "" {
							base = fmt.Sprintf("screenshot-%d", idx+1)
						}

						destDir := filepath.Join(outputDirValue, displayType)
						destName := fmt.Sprintf("%02d_%s_%s", idx+1, strings.TrimSpace(shot.ID), base)
						destPath := filepath.Join(destDir, destName)

						imageAsset := shot.Attributes.ImageAsset
						if imageAsset == nil || strings.TrimSpace(imageAsset.TemplateURL) == "" {
							requestCtx, cancel := shared.ContextWithTimeout(ctx)
							full, err := client.GetAppScreenshot(requestCtx, shot.ID)
							cancel()
							if err == nil {
								imageAsset = full.Data.Attributes.ImageAsset
							}
						}

						downloadURL, err := resolveImageAssetDownloadURL(imageAsset, shot.Attributes.FileName)
						if err != nil {
							items = append(items, screenshotDownloadItem{
								ID:          strings.TrimSpace(shot.ID),
								DisplayType: displayType,
								FileName:    strings.TrimSpace(shot.Attributes.FileName),
								OutputPath:  destPath,
							})
							result.Failures = append(result.Failures, screenshotDownloadFailure{
								ID:          strings.TrimSpace(shot.ID),
								DisplayType: displayType,
								OutputPath:  destPath,
								Error:       err.Error(),
							})
							continue
						}

						items = append(items, screenshotDownloadItem{
							ID:          strings.TrimSpace(shot.ID),
							DisplayType: displayType,
							FileName:    strings.TrimSpace(shot.Attributes.FileName),
							URL:         downloadURL,
							OutputPath:  destPath,
						})
					}
				}
			}

			for i := range items {
				item := &items[i]
				if strings.TrimSpace(item.URL) == "" {
					continue
				}

				downloadCtx, cancel := shared.ContextWithTimeout(ctx)
				written, contentType, err := downloadURLToFile(downloadCtx, item.URL, item.OutputPath, *overwrite)
				cancel()
				if err != nil {
					result.Failures = append(result.Failures, screenshotDownloadFailure{
						ID:          item.ID,
						DisplayType: item.DisplayType,
						URL:         item.URL,
						OutputPath:  item.OutputPath,
						Error:       err.Error(),
					})
					continue
				}

				item.BytesWritten = written
				item.ContentType = contentType
				result.Downloaded++
			}

			result.Items = items
			result.Total = len(items)
			result.Failed = len(result.Failures)

			if err := shared.PrintOutputWithRenderers(
				result,
				*format.Output,
				*format.Pretty,
				func() error { return renderScreenshotDownloadResult(result, false) },
				func() error { return renderScreenshotDownloadResult(result, true) },
			); err != nil {
				return err
			}

			if result.Failed > 0 {
				return shared.NewReportedError(fmt.Errorf("screenshots download: %d file(s) failed", result.Failed))
			}
			return nil
		},
	}
}

func renderScreenshotDownloadResult(result *screenshotDownloadResult, markdown bool) error {
	if result == nil {
		return fmt.Errorf("result is nil")
	}

	render := asc.RenderTable
	if markdown {
		render = asc.RenderMarkdown
	}

	render(
		[]string{"Version Localization", "Output Dir", "Overwrite", "Total", "Downloaded", "Failed"},
		[][]string{{
			result.VersionLocalizationID,
			result.OutputDir,
			fmt.Sprintf("%t", result.Overwrite),
			fmt.Sprintf("%d", result.Total),
			fmt.Sprintf("%d", result.Downloaded),
			fmt.Sprintf("%d", result.Failed),
		}},
	)

	if len(result.Items) > 0 {
		rows := make([][]string, 0, len(result.Items))
		for _, item := range result.Items {
			rows = append(rows, []string{
				item.ID,
				item.DisplayType,
				item.FileName,
				item.OutputPath,
				fmt.Sprintf("%d", item.BytesWritten),
			})
		}
		render([]string{"ID", "Display Type", "File Name", "Output Path", "Bytes"}, rows)
	}

	if len(result.Failures) > 0 {
		rows := make([][]string, 0, len(result.Failures))
		for _, f := range result.Failures {
			rows = append(rows, []string{
				f.ID,
				f.DisplayType,
				f.OutputPath,
				f.Error,
			})
		}
		render([]string{"ID", "Display Type", "Output Path", "Error"}, rows)
	}

	return nil
}

// AssetsScreenshotsDeleteCommand returns the screenshot delete subcommand.
func AssetsScreenshotsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "Screenshot ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc screenshots delete --id \"SCREENSHOT_ID\" --confirm",
		ShortHelp:  "Delete a screenshot by ID.",
		LongHelp: `Delete a screenshot by ID.

Examples:
  asc screenshots delete --id "SCREENSHOT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			assetID := strings.TrimSpace(*id)
			if assetID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to delete")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("screenshots delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppScreenshot(requestCtx, assetID); err != nil {
				return fmt.Errorf("screenshots delete: %w", err)
			}

			result := asc.AssetDeleteResult{
				ID:      assetID,
				Deleted: true,
			}

			return shared.PrintOutput(&result, *output.Output, *output.Pretty)
		},
	}
}

func normalizeScreenshotDisplayType(input string) (string, error) {
	value := strings.ToUpper(strings.TrimSpace(input))
	if value == "" {
		return "", fmt.Errorf("device type is required")
	}
	if !strings.HasPrefix(value, "APP_") && !strings.HasPrefix(value, "IMESSAGE_") {
		value = "APP_" + value
	}
	value = normalizeScreenshotDisplayTypeAlias(value)
	if !asc.IsValidScreenshotDisplayType(value) {
		return "", fmt.Errorf("unsupported screenshot display type %q", value)
	}
	return value, nil
}

// NormalizeScreenshotDisplayType normalizes and validates a screenshot display type.
func NormalizeScreenshotDisplayType(input string) (string, error) {
	return normalizeScreenshotDisplayType(input)
}

func normalizeScreenshotDisplayTypeAlias(value string) string {
	return value
}

func validateScreenshotDimensions(files []string, displayType string) error {
	for _, filePath := range files {
		if err := asc.ValidateScreenshotDimensions(filePath, displayType); err != nil {
			return err
		}
	}
	return nil
}

// ValidateScreenshotDimensions validates screenshot dimensions for all files.
func ValidateScreenshotDimensions(files []string, displayType string) error {
	return validateScreenshotDimensions(files, displayType)
}

func uploadScreenshots(ctx context.Context, client *asc.Client, localizationID, displayType string, files []string, skipExisting, replace, dryRun bool) (asc.AppScreenshotUploadResult, error) {
	return uploadScreenshotsWithConfig(ctx, screenshotUploadConfig[asc.AppScreenshotUploadResult]{
		Client:         client,
		LocalizationID: localizationID,
		DisplayType:    displayType,
		Files:          files,
		SkipExisting:   skipExisting,
		Replace:        replace,
		DryRun:         dryRun,
		RequestContext: shared.ContextWithTimeout,
		UploadContext:  contextWithAssetUploadTimeout,
		Access:         appStoreVersionScreenshotSetAccess,
		BuildResult: func(localizationID string, set asc.Resource[asc.AppScreenshotSetAttributes], dryRun bool, results []asc.AssetUploadResultItem) asc.AppScreenshotUploadResult {
			return asc.AppScreenshotUploadResult{
				VersionLocalizationID: localizationID,
				SetID:                 set.ID,
				DisplayType:           set.Attributes.ScreenshotDisplayType,
				DryRun:                dryRun,
				Results:               results,
			}
		},
	})
}

func findScreenshotSetWithAccess(ctx context.Context, client *asc.Client, localizationID, displayType string, access ScreenshotSetAccess) (asc.Resource[asc.AppScreenshotSetAttributes], error) {
	if access.List == nil {
		return asc.Resource[asc.AppScreenshotSetAttributes]{}, fmt.Errorf("screenshot set list function is required")
	}

	resp, err := access.List(ctx, client, localizationID)
	if err != nil {
		return asc.Resource[asc.AppScreenshotSetAttributes]{}, err
	}
	for _, set := range resp.Data {
		if strings.EqualFold(set.Attributes.ScreenshotDisplayType, displayType) {
			return set, nil
		}
	}
	return asc.Resource[asc.AppScreenshotSetAttributes]{
		Attributes: asc.AppScreenshotSetAttributes{ScreenshotDisplayType: displayType},
	}, nil
}

func ensureScreenshotSetWithAccess(ctx context.Context, client *asc.Client, localizationID, displayType string, access ScreenshotSetAccess) (asc.Resource[asc.AppScreenshotSetAttributes], error) {
	if access.Create == nil {
		return asc.Resource[asc.AppScreenshotSetAttributes]{}, fmt.Errorf("screenshot set create function is required")
	}

	set, err := findScreenshotSetWithAccess(ctx, client, localizationID, displayType, access)
	if err != nil {
		return asc.Resource[asc.AppScreenshotSetAttributes]{}, err
	}
	if set.ID != "" {
		return set, nil
	}

	created, err := access.Create(ctx, client, localizationID, displayType)
	if err != nil {
		return asc.Resource[asc.AppScreenshotSetAttributes]{}, err
	}
	return created.Data, nil
}

func uploadScreenshotsWithConfig[T any](ctx context.Context, cfg screenshotUploadConfig[T]) (T, error) {
	var zero T

	if cfg.Client == nil {
		return zero, fmt.Errorf("client is required")
	}
	if cfg.BuildResult == nil {
		return zero, fmt.Errorf("build result function is required")
	}
	if cfg.RequestContext == nil {
		cfg.RequestContext = shared.ContextWithTimeout
	}
	if cfg.UploadContext == nil {
		cfg.UploadContext = contextWithAssetUploadTimeout
	}

	requestCtx, reqCancel := cfg.RequestContext(ctx)
	var (
		set asc.Resource[asc.AppScreenshotSetAttributes]
		err error
	)
	if cfg.DryRun {
		set, err = findScreenshotSetWithAccess(requestCtx, cfg.Client, cfg.LocalizationID, cfg.DisplayType, cfg.Access)
	} else {
		set, err = ensureScreenshotSetWithAccess(requestCtx, cfg.Client, cfg.LocalizationID, cfg.DisplayType, cfg.Access)
	}
	reqCancel()
	if err != nil {
		return zero, err
	}

	existingScreenshots := make([]asc.Resource[asc.AppScreenshotAttributes], 0)
	if (cfg.SkipExisting || cfg.Replace) && set.ID != "" {
		fetchCtx, fetchCancel := cfg.RequestContext(ctx)
		existingResp, err := cfg.Client.GetAppScreenshots(fetchCtx, set.ID)
		fetchCancel()
		if err != nil {
			return zero, err
		}
		existingScreenshots = existingResp.Data
	}

	skippedResults := make([]asc.AssetUploadResultItem, 0)
	files := cfg.Files
	if cfg.SkipExisting {
		var filterErr error
		files, skippedResults, filterErr = filterExistingScreenshotFiles(cfg.Files, existingScreenshots)
		if filterErr != nil {
			return zero, filterErr
		}
	}

	if cfg.DryRun {
		results := make([]asc.AssetUploadResultItem, 0, len(skippedResults)+len(files)+len(existingScreenshots))
		if cfg.Replace {
			for _, screenshot := range existingScreenshots {
				results = append(results, asc.AssetUploadResultItem{
					FileName: screenshot.Attributes.FileName,
					AssetID:  screenshot.ID,
					State:    "would-delete",
				})
			}
		}
		for _, filePath := range files {
			results = append(results, asc.AssetUploadResultItem{
				FileName: filepath.Base(filePath),
				FilePath: filePath,
				State:    "would-upload",
			})
		}
		results = append(results, skippedResults...)
		return cfg.BuildResult(cfg.LocalizationID, set, true, results), nil
	}

	uploadCtx, cancel := cfg.UploadContext(ctx)
	defer cancel()

	if cfg.Replace {
		if err := deleteExistingScreenshots(uploadCtx, cfg.Client, existingScreenshots); err != nil {
			return zero, err
		}
	}

	results := make([]asc.AssetUploadResultItem, 0, len(skippedResults)+len(files))
	if len(files) > 0 {
		uploadedResults, err := UploadScreenshotsToSet(uploadCtx, cfg.Client, set.ID, files, !cfg.Replace)
		if err != nil {
			return zero, err
		}
		results = append(results, uploadedResults...)
	}
	results = append(skippedResults, results...)

	return cfg.BuildResult(cfg.LocalizationID, set, false, results), nil
}

func deleteExistingScreenshots(ctx context.Context, client *asc.Client, screenshots []asc.Resource[asc.AppScreenshotAttributes]) error {
	for _, screenshot := range screenshots {
		if err := client.DeleteAppScreenshot(ctx, screenshot.ID); err != nil {
			return err
		}
	}
	return nil
}

func filterExistingScreenshotFiles(files []string, screenshots []asc.Resource[asc.AppScreenshotAttributes]) ([]string, []asc.AssetUploadResultItem, error) {
	existingChecksums := make(map[string]struct{}, len(screenshots))
	for _, screenshot := range screenshots {
		checksum := strings.TrimSpace(screenshot.Attributes.SourceFileChecksum)
		if checksum == "" {
			continue
		}
		existingChecksums[checksum] = struct{}{}
	}

	filtered := make([]string, 0, len(files))
	skipped := make([]asc.AssetUploadResultItem, 0)
	for _, filePath := range files {
		checksum, err := screenshotFileChecksumFunc(filePath)
		if err != nil {
			return nil, nil, err
		}
		if _, exists := existingChecksums[checksum]; exists {
			skipped = append(skipped, asc.AssetUploadResultItem{
				FileName: filepath.Base(filePath),
				FilePath: filePath,
				State:    "skipped",
				Skipped:  true,
			})
			continue
		}
		filtered = append(filtered, filePath)
	}

	return filtered, skipped, nil
}

func computeFileChecksum(filePath string) (string, error) {
	file, err := shared.OpenExistingNoFollow(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	checksum, err := asc.ComputeChecksumFromReader(file, asc.ChecksumAlgorithmMD5)
	if err != nil {
		return "", err
	}
	return checksum.Hash, nil
}

func uploadScreenshotAsset(ctx context.Context, client *asc.Client, setID, filePath string) (asc.AssetUploadResultItem, error) {
	if err := asc.ValidateImageFile(filePath); err != nil {
		return asc.AssetUploadResultItem{}, err
	}

	file, err := shared.OpenExistingNoFollow(filePath)
	if err != nil {
		return asc.AssetUploadResultItem{}, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return asc.AssetUploadResultItem{}, err
	}

	checksum, err := asc.ComputeChecksumFromReader(file, asc.ChecksumAlgorithmMD5)
	if err != nil {
		return asc.AssetUploadResultItem{}, err
	}

	created, err := client.CreateAppScreenshot(ctx, setID, info.Name(), info.Size())
	if err != nil {
		return asc.AssetUploadResultItem{}, err
	}
	if len(created.Data.Attributes.UploadOperations) == 0 {
		return asc.AssetUploadResultItem{}, fmt.Errorf("no upload operations returned for %q", info.Name())
	}

	if err := asc.UploadAssetFromFile(ctx, file, info.Size(), created.Data.Attributes.UploadOperations); err != nil {
		return asc.AssetUploadResultItem{}, err
	}

	if _, err := client.UpdateAppScreenshot(ctx, created.Data.ID, true, checksum.Hash); err != nil {
		return asc.AssetUploadResultItem{}, err
	}

	state, err := waitForScreenshotDelivery(ctx, client, created.Data.ID)
	if err != nil {
		return asc.AssetUploadResultItem{}, err
	}

	return asc.AssetUploadResultItem{
		FileName: info.Name(),
		FilePath: filePath,
		AssetID:  created.Data.ID,
		State:    state,
	}, nil
}

// UploadScreenshotAsset uploads a screenshot file to a set.
func UploadScreenshotAsset(ctx context.Context, client *asc.Client, setID, filePath string) (asc.AssetUploadResultItem, error) {
	return uploadScreenshotAsset(ctx, client, setID, filePath)
}

func waitForScreenshotDelivery(ctx context.Context, client *asc.Client, screenshotID string) (string, error) {
	return waitForAssetDeliveryState(ctx, screenshotID, func(ctx context.Context) (*asc.AssetDeliveryState, error) {
		resp, err := client.GetAppScreenshot(ctx, screenshotID)
		if err != nil {
			return nil, err
		}
		return resp.Data.Attributes.AssetDeliveryState, nil
	})
}
