package assets

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var focusedScreenshotDisplayTypes = []string{
	"APP_IPHONE_65",
	"APP_IPAD_PRO_3GEN_129",
}

const appScreenshotSetMaxScreenshots = 10

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
type ScreenshotSetListFunc func(context.Context, *asc.Client, string, asc.RequestContextFunc) (*asc.AppScreenshotSetsResponse, error)

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
	LocalizationID string
	Path           string
	DeviceType     string
	Replace        bool
	// InspectCommand is the owner-specific read-only command shown when the
	// target screenshot set is full or has ambiguous checksum matches.
	InspectCommand string
	// ReplaceCommand is the owner-specific replacement command shown when the
	// target screenshot set is full.
	ReplaceCommand           string
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
	RootPath       string
	Files          []string
	SkipExisting   bool
	Replace        bool
	DryRun         bool
	MaxScreenshots int
	InspectCommand string
	ReplaceCommand string
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
	Confirm               bool
	DryRun                bool
	MaxScreenshots        int
}

type screenshotUploadDependencies struct {
	GetClient        func() (*asc.Client, error)
	RequestContext   func(context.Context) (context.Context, context.CancelFunc)
	UploadScreenshot func(context.Context, *asc.Client, string, string, []string, bool, bool, bool) (asc.AppScreenshotUploadResult, error)
	ExecuteUpload    func(context.Context, screenshotUploadConfig[asc.AppScreenshotUploadResult], string) (asc.AppScreenshotUploadResult, error)
}

type screenshotUploadFanoutConfig struct {
	Client       *asc.Client
	AppID        string
	Version      string
	VersionID    string
	Platform     string
	RootPath     string
	LocaleAssets []screenshotLocaleAssetFiles
	// LocaleAssetsCanonical marks LocaleAssets as already canonicalized and
	// duplicate-checked by fan-out discovery.
	LocaleAssetsCanonical bool
	DisplayType           string
	SkipExisting          bool
	Replace               bool
	DryRun                bool
	MaxScreenshots        int

	RequestContext   func(context.Context) (context.Context, context.CancelFunc)
	UploadScreenshot func(context.Context, *asc.Client, string, string, []string, bool, bool, bool) (asc.AppScreenshotUploadResult, error)
	ExecuteUpload    func(context.Context, screenshotUploadConfig[asc.AppScreenshotUploadResult], string) (asc.AppScreenshotUploadResult, error)
}

type screenshotLocaleAssetFiles struct {
	Locale string
	Files  []string
}

var appStoreVersionScreenshotSetAccess = ScreenshotSetAccess{
	List: func(ctx context.Context, client *asc.Client, localizationID string, requestContext asc.RequestContextFunc) (*asc.AppScreenshotSetsResponse, error) {
		return client.GetAllAppScreenshotSets(ctx, localizationID, asc.WithAppScreenshotSetsRequestContext(requestContext))
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

func resolveScreenshotUploadExecutor(
	exec func(context.Context, screenshotUploadConfig[asc.AppScreenshotUploadResult], string) (asc.AppScreenshotUploadResult, error),
	upload func(context.Context, *asc.Client, string, string, []string, bool, bool, bool) (asc.AppScreenshotUploadResult, error),
) func(context.Context, screenshotUploadConfig[asc.AppScreenshotUploadResult], string) (asc.AppScreenshotUploadResult, error) {
	if exec != nil {
		return exec
	}
	if upload != nil {
		return func(ctx context.Context, cfg screenshotUploadConfig[asc.AppScreenshotUploadResult], _ string) (asc.AppScreenshotUploadResult, error) {
			return upload(ctx, cfg.Client, cfg.LocalizationID, cfg.DisplayType, cfg.Files, cfg.SkipExisting, cfg.Replace, cfg.DryRun)
		}
	}
	return executeAppScreenshotUpload
}

func resolveAppScopedScreenshotPlatform(version, platformValue string) (string, error) {
	trimmedPlatform := strings.TrimSpace(platformValue)
	if trimmedPlatform != "" {
		return shared.NormalizeAppStoreVersionPlatform(trimmedPlatform)
	}
	if strings.TrimSpace(version) != "" {
		return "IOS", nil
	}
	return "", nil
}

func buildFanoutLocalizationUploadResult(locale string, uploadResult asc.AppScreenshotUploadResult) asc.AppScreenshotLocalizationUploadResult {
	return asc.AppScreenshotLocalizationUploadResult{
		Locale:                locale,
		VersionLocalizationID: uploadResult.VersionLocalizationID,
		SetID:                 uploadResult.SetID,
		DisplayType:           uploadResult.DisplayType,
		DryRun:                uploadResult.DryRun,
		Total:                 uploadResult.Total,
		Uploaded:              uploadResult.Uploaded,
		Skipped:               uploadResult.Skipped,
		Pending:               uploadResult.Pending,
		Failed:                uploadResult.Failed,
		FailureArtifactPath:   uploadResult.FailureArtifactPath,
		Results:               append([]asc.AssetUploadResultItem(nil), uploadResult.Results...),
		Failures:              append([]asc.AssetUploadFailureItem(nil), uploadResult.Failures...),
	}
}

func hasAppScreenshotFanoutUploadResultOutput(result asc.AppScreenshotFanoutUploadResult) bool {
	return strings.TrimSpace(result.AppID) != "" ||
		strings.TrimSpace(result.Version) != "" ||
		strings.TrimSpace(result.VersionID) != "" ||
		strings.TrimSpace(result.Platform) != "" ||
		strings.TrimSpace(result.DisplayType) != "" ||
		len(result.Localizations) > 0
}

// ExecuteScreenshotSetUpload validates flags/files and runs the shared
// screenshot upload/sync flow for a localization-backed screenshot set.
func ExecuteScreenshotSetUpload[T any](ctx context.Context, opts ScreenshotSetUploadOptions[T]) (T, error) {
	var zero T

	trimmedLocalizationID := strings.TrimSpace(opts.LocalizationID)
	if trimmedLocalizationID == "" {
		fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
		return zero, shared.MissingRequiredUsageError("--localization-id")
	}
	trimmedPath := strings.TrimSpace(opts.Path)
	if trimmedPath == "" {
		fmt.Fprintln(os.Stderr, "Error: --path is required")
		return zero, shared.MissingRequiredUsageError("--path")
	}
	trimmedDeviceType := strings.TrimSpace(opts.DeviceType)
	if trimmedDeviceType == "" {
		fmt.Fprintln(os.Stderr, "Error: --device-type is required")
		return zero, shared.MissingRequiredUsageError("--device-type")
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
	if err := validateScreenshotDimensions(files, apiDisplayType); err != nil {
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
		InspectCommand: opts.InspectCommand,
		ReplaceCommand: opts.ReplaceCommand,
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
	appID := fs.String("app", "", "App Store Connect app ID, bundle ID, or exact app name (or ASC_APP_ID env)")
	version := fs.String("version", "", "App Store version string (requires --app)")
	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "App Store version ID")
	platform := fs.String("platform", "", "Platform: IOS, MAC_OS, TV_OS, VISION_OS (defaults to IOS with --version; with --version-id requires --app or ASC_APP_ID)")
	locale := fs.String("locale", "", "Localization locale (required with --version or --version-id)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc screenshots list (--version-localization \"VERSION_LOCALIZATION_ID\" | --version-id \"VERSION_ID\" --locale \"LOCALE\" | --app \"APP_ID\" --version \"VERSION\" --locale \"LOCALE\")",
		ShortHelp:  "List screenshots for a localization.",
		LongHelp: `List screenshots for a localization.

--version-localization is the App Store version localization resource ID
returned as data[].id by:
  asc localizations list --version "VERSION_ID" --output json --locale "en-US"
It is not the locale code such as en-US.

Examples:
  asc screenshots list --version-localization "VERSION_LOCALIZATION_ID"
  asc screenshots list --version-id "VERSION_ID" --locale "en-US"
  asc screenshots list --app "123456789" --version "1.2.3" --locale "en-US"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.RejectPositionalArgs(args); err != nil {
				return err
			}

			result, err := executeScreenshotListCommand(ctx, screenshotListCommandOptions{
				VersionLocalizationID: *localizationID,
				AppID:                 *appID,
				Version:               *version,
				VersionID:             *versionID,
				Platform:              *platform,
				Locale:                *locale,
			}, screenshotListDependencies{})
			if err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return err
				}
				return fmt.Errorf("screenshots list: %w", err)
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

type screenshotListCommandOptions struct {
	VersionLocalizationID string
	AppID                 string
	Version               string
	VersionID             string
	Platform              string
	Locale                string
}

type screenshotListDependencies struct {
	GetClient      func() (*asc.Client, error)
	RequestContext func(context.Context) (context.Context, context.CancelFunc)
}

func executeScreenshotListCommand(ctx context.Context, opts screenshotListCommandOptions, deps screenshotListDependencies) (*asc.AppScreenshotListResult, error) {
	if deps.GetClient == nil {
		deps.GetClient = shared.GetASCClient
	}
	if deps.RequestContext == nil {
		deps.RequestContext = shared.ContextWithTimeout
	}

	locID := strings.TrimSpace(opts.VersionLocalizationID)
	appValue := strings.TrimSpace(opts.AppID)
	versionValue := strings.TrimSpace(opts.Version)
	versionIDValue := strings.TrimSpace(opts.VersionID)
	platformValue := strings.TrimSpace(opts.Platform)
	localeValue := strings.TrimSpace(opts.Locale)
	normalizedPlatform := ""
	versionModeRequested := appValue != "" || versionValue != "" || versionIDValue != "" || platformValue != "" || localeValue != ""

	if locID == "" && !versionModeRequested {
		fmt.Fprintln(os.Stderr, "Error: choose a localization selector: --version-localization VERSION_LOCALIZATION_ID; --version-id VERSION_ID with --locale LOCALE; or (--app APP_ID or ASC_APP_ID) with --version VERSION and --locale LOCALE")
		return nil, shared.MissingRequiredUsageError("")
	}
	if locID != "" && versionModeRequested {
		fmt.Fprintln(os.Stderr, "Error: --version-localization cannot be combined with --app, --version, --version-id, --platform, or --locale")
		return nil, flag.ErrHelp
	}
	if versionValue != "" && versionIDValue != "" {
		fmt.Fprintln(os.Stderr, "Error: --version and --version-id are mutually exclusive")
		return nil, flag.ErrHelp
	}

	if locID == "" {
		if versionValue == "" && versionIDValue == "" {
			fmt.Fprintln(os.Stderr, "Error: --version or --version-id is required")
			return nil, shared.MissingRequiredUsageError("")
		}
		if localeValue == "" {
			fmt.Fprintln(os.Stderr, "Error: --locale is required with --version or --version-id")
			return nil, shared.MissingRequiredUsageError("--locale")
		}
		if versionValue != "" || (versionIDValue != "" && platformValue != "") {
			appValue = shared.ResolveAppID(appValue)
		}
		if versionValue != "" {
			if appValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required with --version (or set ASC_APP_ID)")
				return nil, shared.MissingRequiredUsageError("--app")
			}
		}
		if platformValue != "" && versionValue == "" && appValue == "" {
			return nil, shared.UsageError("--platform with --version-id requires --app or ASC_APP_ID so ownership and platform can be verified")
		}
		if platformValue != "" {
			parsedPlatform, platformErr := shared.NormalizeAppStoreVersionPlatform(platformValue)
			if platformErr != nil {
				return nil, shared.UsageError(platformErr.Error())
			}
			normalizedPlatform = parsedPlatform
		} else if versionValue != "" {
			normalizedPlatform = "IOS"
		}

		canonicalLocale, err := shared.CanonicalizeAppStoreLocalizationLocale(localeValue)
		if err != nil {
			return nil, shared.UsageError(err.Error())
		}
		localeValue = canonicalLocale
	}

	client, err := deps.GetClient()
	if err != nil {
		return nil, err
	}

	if locID == "" {
		resolvedVersionID := versionIDValue
		if versionValue != "" || appValue != "" {
			requestCtx, cancel := deps.RequestContext(ctx)
			resolvedAppID, resolveErr := shared.ResolveAppIDWithLookup(requestCtx, client, appValue)
			if resolveErr == nil {
				if versionValue != "" {
					resolvedVersionID, resolveErr = shared.ResolveAppStoreVersionID(requestCtx, client, resolvedAppID, versionValue, normalizedPlatform)
				} else {
					versionData, ownedErr := shared.ResolveOwnedAppStoreVersionByID(requestCtx, client, resolvedAppID, versionIDValue, normalizedPlatform)
					resolveErr = ownedErr
					resolvedVersionID = strings.TrimSpace(versionData.ID)
				}
			}
			cancel()
			if resolveErr != nil {
				return nil, resolveErr
			}
		}

		localizations, listErr := fetchAllScreenshotVersionLocalizations(ctx, client, resolvedVersionID, deps.RequestContext)
		if listErr != nil {
			return nil, fmt.Errorf("failed to fetch version localizations: %w", listErr)
		}
		for _, item := range localizations {
			if strings.EqualFold(strings.TrimSpace(item.Attributes.Locale), localeValue) {
				locID = strings.TrimSpace(item.ID)
				break
			}
		}
		if locID == "" {
			return nil, fmt.Errorf("no App Store version localization found for locale %q", localeValue)
		}
	}

	return fetchScreenshotList(ctx, client, locID, deps.RequestContext)
}

func fetchAllScreenshotVersionLocalizations(
	ctx context.Context,
	client *asc.Client,
	versionID string,
	requestContext func(context.Context) (context.Context, context.CancelFunc),
) ([]asc.Resource[asc.AppStoreVersionLocalizationAttributes], error) {
	requestCtx, cancel := requestContext(ctx)
	firstPage, err := client.GetAppStoreVersionLocalizations(requestCtx, versionID, asc.WithAppStoreVersionLocalizationsLimit(200))
	cancel()
	if err != nil {
		return nil, err
	}
	if firstPage == nil {
		return nil, fmt.Errorf("empty version localization response")
	}

	paginated, err := asc.PaginateAll(ctx, firstPage, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		nextCtx, nextCancel := requestContext(pageCtx)
		nextPage, nextErr := client.GetAppStoreVersionLocalizations(nextCtx, versionID, asc.WithAppStoreVersionLocalizationsNextURL(nextURL))
		nextCancel()
		if nextErr != nil {
			return nil, nextErr
		}
		if nextPage == nil {
			return nil, fmt.Errorf("empty version localization response")
		}
		return nextPage, nil
	})
	if err != nil {
		return nil, err
	}
	allPages, ok := paginated.(*asc.AppStoreVersionLocalizationsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected version localization pagination response type")
	}
	return allPages.Data, nil
}

func fetchScreenshotList(
	ctx context.Context,
	client *asc.Client,
	localizationID string,
	requestContext func(context.Context) (context.Context, context.CancelFunc),
) (*asc.AppScreenshotListResult, error) {
	setsResp, err := client.GetAllAppScreenshotSets(ctx, localizationID, asc.WithAppScreenshotSetsRequestContext(requestContext))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sets: %w", err)
	}

	result := &asc.AppScreenshotListResult{
		VersionLocalizationID: localizationID,
		Sets:                  make([]asc.AppScreenshotSetWithScreenshots, 0, len(setsResp.Data)),
	}

	for _, set := range setsResp.Data {
		screenshots, err := client.GetAllAppScreenshots(ctx, set.ID, asc.WithAppScreenshotsRequestContext(requestContext))
		if err != nil {
			return nil, fmt.Errorf("failed to fetch screenshots for set %s: %w", set.ID, err)
		}
		result.Sets = append(result.Sets, asc.AppScreenshotSetWithScreenshots{
			Set:         set,
			Screenshots: screenshots.Data,
		})
	}

	return result, nil
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
	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "App Store version ID for app-scoped fan-out uploads")
	platform := fs.String("platform", "", "Platform for app-scoped fan-out uploads: IOS, MAC_OS, TV_OS, VISION_OS (default: IOS)")
	path := fs.String("path", "", "Path to screenshot file or directory")
	deviceType := fs.String("device-type", "", "Device type (e.g., IPHONE_65 or IPAD_PRO_3GEN_129)")
	resume := fs.String("resume", "", "Resume a previous upload from a failure artifact")
	skipExisting := fs.Bool("skip-existing", false, "Skip files whose MD5 checksum already exists in the target screenshot set")
	replace := fs.Bool("replace", false, "Delete all existing screenshots from the target set before uploading (requires --confirm)")
	confirm := fs.Bool("confirm", false, "Confirm the deletions performed by --replace (required with --replace)")
	dryRun := fs.Bool("dry-run", false, "Show what would be uploaded, skipped, or deleted without making changes")
	maxScreenshots := fs.Int("max-screenshots", 0, "Upload only the first N sorted screenshots per set; must be 10 or less")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc screenshots upload (--version-localization \"VERSION_LOCALIZATION_ID\" | --app \"APP_ID\" (--version \"1.2.3\" | --version-id \"VERSION_ID\")) --path \"./screenshots\" --device-type \"IPHONE_65\"",
		ShortHelp:  "Upload screenshots for one or more localizations.",
		LongHelp: `Upload screenshots for one or more localizations.

Use --version-localization for a single localization upload, or use --app with
--version/--version-id to fan out one run across locale directories under
--path. In fan-out mode, the immediate children of --path must be locale
directories. Each locale subtree is scanned recursively, and only files
matching --device-type are uploaded. This supports layouts like
./screenshots/en-US/iphone/*.png, or ./screenshots/iphone/en-US/*.png when
--path points to ./screenshots/iphone.

--version-localization is the App Store version localization resource ID
returned as data[].id by:
  asc localizations list --version "VERSION_ID" --output json --locale "en-US"
It is not the locale code such as en-US.

--replace deletes every existing screenshot in each target set before uploading
and therefore requires --confirm. Use --replace --dry-run to preview the
deletions without --confirm.

Examples:
  asc screenshots upload --version-localization "VERSION_LOCALIZATION_ID" --path "./screenshots" --device-type "IPHONE_65"
  asc screenshots upload --version-localization "VERSION_LOCALIZATION_ID" --path "./screenshots" --device-type "IPHONE_65" --skip-existing
  asc screenshots upload --version-localization "VERSION_LOCALIZATION_ID" --path "./screenshots" --device-type "IPHONE_65" --replace --confirm
  asc screenshots upload --version-localization "VERSION_LOCALIZATION_ID" --path "./screenshots" --device-type "IPHONE_65" --replace --dry-run
  asc screenshots upload --version-localization "VERSION_LOCALIZATION_ID" --path "./screenshots" --device-type "IPHONE_65" --max-screenshots 10
  asc screenshots upload --version-localization "VERSION_LOCALIZATION_ID" --path "./screenshots" --device-type "IPHONE_65" --skip-existing --dry-run
  asc screenshots upload --version-localization "VERSION_LOCALIZATION_ID" --path "./screenshots" --device-type "IPAD_PRO_3GEN_129"
  asc screenshots upload --version-localization "VERSION_LOCALIZATION_ID" --path "./screenshots/en-US.png" --device-type "IPHONE_65"
  asc screenshots upload --app "123456789" --version "1.2.3" --path "./screenshots" --device-type "IPHONE_65"
  asc screenshots upload --app "123456789" --version-id "VERSION_ID" --path "./screenshots/ipad" --device-type "IPAD_PRO_3GEN_129" --dry-run
  asc screenshots upload --resume ".asc/reports/screenshots-upload/failures-123.json"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resumePath := strings.TrimSpace(*resume)
			if resumePath != "" {
				if strings.TrimSpace(*localizationID) != "" ||
					strings.TrimSpace(*appID) != "" ||
					strings.TrimSpace(*version) != "" ||
					strings.TrimSpace(*versionID) != "" ||
					strings.TrimSpace(*platform) != "" ||
					strings.TrimSpace(*path) != "" ||
					strings.TrimSpace(*deviceType) != "" {
					return shared.UsageError("--resume cannot be combined with --version-localization, --app, --version, --version-id, --platform, --path, or --device-type")
				}
				if *skipExisting || *replace || *confirm || *dryRun || *maxScreenshots != 0 {
					return shared.UsageError("--resume cannot be combined with --skip-existing, --replace, --confirm, --dry-run, or --max-screenshots")
				}

				client, err := shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("screenshots upload: %w", err)
				}

				result, err := resumeAppScreenshotUpload(ctx, client, resumePath)
				if hasAppScreenshotUploadResultOutput(result) {
					if printErr := shared.PrintOutput(&result, *output.Output, *output.Pretty); printErr != nil {
						return printErr
					}
				}
				return err
			}

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
				Confirm:               *confirm,
				DryRun:                *dryRun,
				MaxScreenshots:        *maxScreenshots,
			}, screenshotUploadDependencies{
				GetClient:        shared.GetASCClient,
				RequestContext:   shared.ContextWithTimeout,
				UploadScreenshot: uploadScreenshots,
				ExecuteUpload:    executeAppScreenshotUpload,
			})
			if result != nil {
				shouldPrint := err == nil
				switch typed := result.(type) {
				case *asc.AppScreenshotUploadResult:
					if hasAppScreenshotUploadResultOutput(*typed) {
						shouldPrint = true
					}
				case *asc.AppScreenshotFanoutUploadResult:
					if hasAppScreenshotFanoutUploadResultOutput(*typed) {
						shouldPrint = true
					}
				}
				if shouldPrint {
					if printErr := shared.PrintOutput(result, *output.Output, *output.Pretty); printErr != nil {
						return printErr
					}
				}
			}
			if err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return err
				}
				return fmt.Errorf("screenshots upload: %w", err)
			}
			return nil
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
	deps.ExecuteUpload = resolveScreenshotUploadExecutor(deps.ExecuteUpload, deps.UploadScreenshot)

	locID := strings.TrimSpace(opts.VersionLocalizationID)
	appFlagValue := strings.TrimSpace(opts.AppID)
	versionValue := strings.TrimSpace(opts.Version)
	versionIDValue := strings.TrimSpace(opts.VersionID)
	platformValue := strings.TrimSpace(opts.Platform)
	appModeRequested := appFlagValue != "" || versionValue != "" || versionIDValue != "" || platformValue != ""

	if locID == "" {
		if !appModeRequested {
			fmt.Fprintln(os.Stderr, "Error: choose an upload mode: --version-localization VERSION_LOCALIZATION_ID; (--app APP_ID or ASC_APP_ID) with --version VERSION or --version-id VERSION_ID; or --resume ARTIFACT_PATH")
			return nil, shared.MissingRequiredUsageError("")
		}
	} else if appModeRequested {
		fmt.Fprintln(os.Stderr, "Error: --version-localization cannot be combined with --app, --version, --version-id, or --platform")
		return nil, flag.ErrHelp
	}

	if locID == "" {
		resolvedAppValue := shared.ResolveAppID(appFlagValue)
		if resolvedAppValue == "" {
			fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
			return nil, shared.MissingRequiredUsageError("--app")
		}
		if versionValue == "" && versionIDValue == "" {
			fmt.Fprintln(os.Stderr, "Error: --version or --version-id is required with --app")
			return nil, shared.MissingRequiredUsageError("")
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
		return nil, shared.MissingRequiredUsageError("--path")
	}
	deviceValue := strings.TrimSpace(opts.DeviceType)
	if deviceValue == "" {
		fmt.Fprintln(os.Stderr, "Error: --device-type is required")
		return nil, shared.MissingRequiredUsageError("--device-type")
	}
	if opts.SkipExisting && opts.Replace {
		fmt.Fprintln(os.Stderr, "Error: --skip-existing and --replace are mutually exclusive")
		return nil, flag.ErrHelp
	}
	if opts.Replace && !opts.DryRun && !opts.Confirm {
		fmt.Fprintln(os.Stderr, "Error: --confirm is required to delete existing screenshots with --replace")
		return nil, shared.MissingRequiredUsageError("--confirm")
	}
	if opts.Confirm && !opts.Replace {
		return nil, shared.UsageError("--confirm only applies to --replace")
	}
	if opts.MaxScreenshots < 0 {
		return nil, shared.UsageError("--max-screenshots must be zero or greater")
	}
	if opts.MaxScreenshots > appScreenshotSetMaxScreenshots {
		return nil, shared.UsageError(fmt.Sprintf("--max-screenshots cannot exceed %d; App Store screenshot sets allow at most %d images", appScreenshotSetMaxScreenshots, appScreenshotSetMaxScreenshots))
	}

	displayType, err := normalizeScreenshotDisplayType(deviceValue)
	if err != nil {
		return nil, shared.UsageError(err.Error())
	}
	apiDisplayType := asc.CanonicalScreenshotDisplayTypeForAPI(displayType)

	if locID != "" {
		files, err := collectScreenshotUploadFiles(pathValue, opts.MaxScreenshots)
		if err != nil {
			return nil, shared.NewValidationError(err)
		}
		if err := validateScreenshotDimensions(files, apiDisplayType); err != nil {
			return nil, shared.NewValidationError(err)
		}
		client, err := deps.GetClient()
		if err != nil {
			return nil, err
		}
		result, err := deps.ExecuteUpload(ctx, screenshotUploadConfig[asc.AppScreenshotUploadResult]{
			Client:         client,
			LocalizationID: locID,
			DisplayType:    apiDisplayType,
			RootPath:       pathValue,
			Files:          files,
			SkipExisting:   opts.SkipExisting,
			Replace:        opts.Replace,
			DryRun:         opts.DryRun,
			MaxScreenshots: opts.MaxScreenshots,
			RequestContext: deps.RequestContext,
			UploadContext:  contextWithAssetUploadTimeout,
			Access:         appStoreVersionScreenshotSetAccess,
		}, "")
		return &result, err
	}

	normalizedPlatform, err := resolveAppScopedScreenshotPlatform(versionValue, platformValue)
	if err != nil {
		return nil, shared.UsageError(err.Error())
	}

	localeAssets, err := collectLocaleAssetFilesWithLimit(pathValue, apiDisplayType, opts.MaxScreenshots)
	if err != nil {
		return nil, shared.NewValidationError(err)
	}
	localeAssets, err = limitScreenshotFanoutUploadFiles(localeAssets, opts.MaxScreenshots)
	if err != nil {
		return nil, shared.NewValidationError(err)
	}
	if err := validateScreenshotFanoutAssets(localeAssets, apiDisplayType); err != nil {
		return nil, shared.NewValidationError(err)
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
		Client:                client,
		AppID:                 resolvedAppID,
		Version:               resolvedVersion,
		VersionID:             resolvedVersionID,
		Platform:              resolvedPlatform,
		RootPath:              pathValue,
		LocaleAssets:          localeAssets,
		LocaleAssetsCanonical: true,
		DisplayType:           apiDisplayType,
		SkipExisting:          opts.SkipExisting,
		Replace:               opts.Replace,
		DryRun:                opts.DryRun,
		MaxScreenshots:        opts.MaxScreenshots,
		RequestContext:        deps.RequestContext,
		UploadScreenshot:      deps.UploadScreenshot,
		ExecuteUpload:         deps.ExecuteUpload,
	})
	if err != nil {
		return &result, err
	}
	return &result, nil
}
