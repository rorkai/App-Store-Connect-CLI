package migrate

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

// MigrateCommand returns the migrate command with subcommands.
func MigrateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "migrate",
		ShortUsage: "asc migrate <subcommand> [flags]",
		ShortHelp:  "Migrate metadata from/to fastlane format.",
		LongHelp: `Migrate metadata from/to fastlane directory structure.

This enables transitioning from fastlane's deliver tool to asc.

Examples:
  asc migrate import --app "APP_ID" --version-id "VERSION_ID" --fastlane-dir ./fastlane --confirm
  asc migrate export --app "APP_ID" --version-id "VERSION_ID" --output-dir ./fastlane
  asc migrate metadata pull --app "APP_ID" --version "1.2.3" --dir "./metadata"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			MigrateImportCommand(),
			MigrateExportCommand(),
			MigrateValidateCommand(),
			MigrateMetadataCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// MigrateImportCommand returns the migrate import subcommand.
func MigrateImportCommand() *ffcli.Command {
	fs := flag.NewFlagSet("migrate import", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	versionID := fs.String("version-id", "", "App Store version ID (required unless Deliverfile app_version + platform)")
	fastlaneDir := fs.String("fastlane-dir", "", "Path to fastlane directory (optional)")
	dryRun := fs.Bool("dry-run", false, "Preview changes without uploading")
	confirm := fs.Bool("confirm", false, "Confirm uploading the imported metadata and screenshots (required unless --dry-run)")
	skipScreenshots := fs.Bool("skip-screenshots", false, "Skip screenshot discovery and upload")
	skipAppClip := fs.Bool("skip-app-clip", false, "Skip App Clip metadata and header images")
	skipPreviews := fs.Bool("skip-previews", false, "Skip App Preview videos")
	allowExternalMetadata := fs.Bool("allow-external-metadata", false, "Trust Deliverfile metadata paths and symlinks outside the selected Fastlane directory")
	allowExternalScreenshots := fs.Bool("allow-external-screenshots", false, "Trust Deliverfile screenshot paths and symlinks outside the selected Fastlane directory")
	allowSymlinkedDeliverfile := fs.Bool("allow-symlinked-deliverfile", false, "Trust and follow a symlinked Deliverfile")
	includeSensitive := shared.BindIncludeSensitiveFlag(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "import",
		ShortUsage: "asc migrate import [flags]",
		ShortHelp:  "Import metadata from fastlane directory structure.",
		LongHelp: `Import metadata from fastlane directory structure.

Reads from Deliver-style structure using --fastlane-dir, Deliverfile values,
or conventional metadata/ and screenshots/ directories:
  fastlane/
  ├── Deliverfile
  ├── metadata/
  │   ├── en-US/
  │   │   ├── name.txt            (App Info)
  │   │   ├── subtitle.txt        (App Info)
  │   │   ├── privacy_url.txt     (App Info)
  │   │   ├── description.txt     (Version)
  │   │   ├── keywords.txt        (Version)
  │   │   ├── release_notes.txt   (Version)
  │   │   ├── promotional_text.txt (Version)
  │   │   ├── support_url.txt     (Version)
  │   │   └── marketing_url.txt   (Version)
  │   ├── review_information/
  │   │   ├── first_name.txt
  │   │   ├── last_name.txt
  │   │   ├── email_address.txt
  │   │   ├── phone_number.txt
  │   │   ├── demo_user.txt
  │   │   ├── demo_password.txt
  │   │   ├── demo_required.txt
  │   │   └── notes.txt
  │   │   └── app_clip/
  │   │       ├── subtitle.txt
  │   │       └── header_image.png
  │   └── app_clip/action.txt
  ├── app_previews/
  │   └── en-US/
  │       └── iphone_65/
  │           ├── preview.mp4
  │           └── preview.poster_frame.txt
  ├── screenshots/
  │   ├── en-US/
  │   │   ├── iphone_65_1.png
  │   │   └── ...

Examples:
  asc migrate import --app "APP_ID" --version-id "VERSION_ID" --fastlane-dir ./fastlane --confirm
  asc migrate import --app "APP_ID" --version-id "VERSION_ID" --fastlane-dir ./fastlane --dry-run
  asc migrate import --app "APP_ID" --version-id "VERSION_ID" --fastlane-dir ./fastlane --skip-screenshots --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			// A fastlane directory turns into localization updates, review
			// information changes, and screenshot uploads, so the same apply
			// decision the other file-driven importers require is enforced
			// here before the directory is even read.
			if err := shared.RequireConfirmUnlessDryRun(*dryRun, *confirm); err != nil {
				return err
			}

			workDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("migrate import: %w", err)
			}

			inputs, skipped, err := resolveImportInputs(importInputOptions{
				WorkDir:                   workDir,
				FastlaneDir:               *fastlaneDir,
				SkipScreenshots:           *skipScreenshots,
				AllowExternalMetadata:     *allowExternalMetadata,
				AllowExternalScreenshots:  *allowExternalScreenshots,
				AllowSymlinkedDeliverfile: *allowSymlinkedDeliverfile,
			})
			if err != nil {
				return fmt.Errorf("migrate import: %w", err)
			}

			metadataDir := inputs.MetadataDir
			screenshotsDir := inputs.ScreenshotsDir
			if inputs.DeliverfileConfig.SkipMetadata && metadataDir != "" {
				skipped = append(skipped, SkippedItem{
					Path:   metadataDir,
					Reason: "skip_metadata in Deliverfile",
				})
				metadataDir = ""
			}
			if inputs.DeliverfileConfig.SkipScreenshots && screenshotsDir != "" {
				skipped = append(skipped, SkippedItem{
					Path:   screenshotsDir,
					Reason: "skip_screenshots in Deliverfile",
				})
				screenshotsDir = ""
			}
			if *skipScreenshots && screenshotsDir != "" {
				skipped = append(skipped, SkippedItem{
					Path:   screenshotsDir,
					Reason: "skipped by --skip-screenshots",
				})
				screenshotsDir = ""
			}

			var localizations []FastlaneLocalization
			var appInfoLocs []AppInfoFastlaneLocalization
			var reviewInfo *ReviewInformation
			if metadataDir != "" {
				localeDirs, metadataSkipped, err := scanFastlaneMetadataLocaleDirs(metadataDir)
				if err != nil {
					return fmt.Errorf("migrate import: %w", err)
				}
				skipped = append(skipped, metadataSkipped...)

				localizations, err = readFastlaneMetadataFromLocaleDirs(metadataDir, localeDirs)
				if err != nil {
					return fmt.Errorf("migrate import: %w", err)
				}
				appInfoLocs, err = readFastlaneAppInfoMetadataFromLocaleDirs(metadataDir, localeDirs)
				if err != nil {
					return fmt.Errorf("migrate import: %w", err)
				}

				reviewInfo, err = readFastlaneReviewInformation(metadataDir)
				if err != nil {
					return fmt.Errorf("migrate import: %w", err)
				}
			}
			var appClip *AppClipLayout
			if metadataDir != "" && !*skipAppClip {
				layout, present, err := readAppClipLayout(metadataDir)
				if err != nil {
					return fmt.Errorf("migrate import: %w", err)
				}
				if present {
					appClip = &layout
				}
			}
			var previews []PreviewLayout
			if fastlaneRoot := strings.TrimSpace(*fastlaneDir); fastlaneRoot != "" && !*skipPreviews {
				previews, err = readPreviewLayout(fastlaneRoot)
				if err != nil {
					return fmt.Errorf("migrate import: %w", err)
				}
			}

			var screenshotPlan []ScreenshotPlan
			var skippedScreenshots []SkippedItem
			if screenshotsDir != "" {
				screenshotPlan, skippedScreenshots, err = discoverScreenshotPlanForUpload(screenshotsDir)
				if err != nil {
					return fmt.Errorf("migrate import: %w", err)
				}
				defer closeScreenshotPlans(screenshotPlan)
				skipped = append(skipped, skippedScreenshots...)
			}

			locales := collectLocales(localizations, appInfoLocs, screenshotPlan)
			metadataFiles := buildMetadataFilePlans(localizations)
			appInfoFiles := buildAppInfoFilePlans(appInfoLocs)

			if strings.TrimSpace(*versionID) == "" && (strings.TrimSpace(inputs.DeliverfileConfig.AppVersion) == "" || strings.TrimSpace(inputs.DeliverfileConfig.Platform) == "") {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required (or set Deliverfile app_version and platform)")
				return shared.MissingRequiredUsageError("--version-id")
			}
			if strings.TrimSpace(*appID) == "" && strings.TrimSpace(inputs.DeliverfileConfig.AppIdentifier) == "" && shared.ResolveAppID("") == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID or Deliverfile app_identifier)")
				return shared.MissingRequiredUsageError("--app")
			}
			preparedLocalizations, err := prepareVersionLocalizations(localizations)
			if err != nil {
				return err
			}
			preparedAppInfoLocalizations, err := prepareAppInfoLocalizationAttributes(appInfoLocs)
			if err != nil {
				return err
			}
			if *dryRun {
				// A preview has no remote localization list that could exempt an
				// already existing locale, so run the locale half of the apply
				// preflight here. Without it a clean plan is followed by a hard
				// failure on the --confirm run for a purely local reason.
				if err := validateCreateTargetLocales(preparedLocalizations, preparedAppInfoLocalizations, screenshotPlan); err != nil {
					return err
				}
			}

			var client *asc.Client
			// Resolution reads share one request budget. Every later call
			// derives its own from ctx, so a long import is not capped by this
			// deadline.
			requestCtx := ctx
			needsClient := !*dryRun ||
				(strings.TrimSpace(*appID) == "" && strings.TrimSpace(inputs.DeliverfileConfig.AppIdentifier) != "") ||
				(strings.TrimSpace(*versionID) == "" && strings.TrimSpace(inputs.DeliverfileConfig.AppVersion) != "" && strings.TrimSpace(inputs.DeliverfileConfig.Platform) != "")
			if needsClient {
				client, err = shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("migrate import: %w", err)
				}
				resolveCtx, cancelResolve := migrateRequestContext(ctx)
				defer cancelResolve()
				requestCtx = resolveCtx
			}

			resolvedAppID, err := resolveAppID(requestCtx, client, *appID, inputs.DeliverfileConfig)
			if err != nil {
				return fmt.Errorf("migrate import: %w", err)
			}
			resolvedVersionID, err := resolveVersionID(requestCtx, client, *versionID, resolvedAppID, inputs.DeliverfileConfig)
			if err != nil {
				return fmt.Errorf("migrate import: %w", err)
			}

			result := &MigrateImportResult{
				DryRun:               *dryRun,
				VersionID:            resolvedVersionID,
				AppID:                resolvedAppID,
				DeliverfilePath:      inputs.DeliverfilePath,
				MetadataDir:          metadataDir,
				ScreenshotsDir:       screenshotsDir,
				Locales:              locales,
				Localizations:        localizations,
				AppInfoLocalizations: appInfoLocs,
				MetadataFiles:        metadataFiles,
				AppInfoFiles:         appInfoFiles,
				ReviewInformation:    reviewInfo,
				ScreenshotPlan:       screenshotPlan,
				AppClip:              appClip,
				Previews:             previews,
				Skipped:              skipped,
			}

			if *dryRun {
				shared.WarnIncludeSensitive(os.Stderr, *includeSensitive)
				return printMigrateOutput(presentableImportResult(result, *includeSensitive), *output.Output, *output.Pretty)
			}

			ownershipCtx, cancelOwnership := migrateRequestContext(ctx)
			err = verifyExplicitVersionOwnership(ownershipCtx, client, *versionID, resolvedAppID, resolvedVersionID)
			cancelOwnership()
			if err != nil {
				return fmt.Errorf("migrate import: %w", err)
			}

			localeToID := make(map[string]string)
			if len(localizations) > 0 || len(screenshotPlan) > 0 {
				existingCtx, cancelExisting := migrateRequestContext(ctx)
				existingLocs, err := fetchVersionLocalizationsForPlan(existingCtx, client, strings.TrimSpace(resolvedVersionID))
				cancelExisting()
				if err != nil {
					return fmt.Errorf("migrate import: failed to fetch existing localizations: %w", err)
				}
				for _, loc := range existingLocs {
					localeToID[loc.Attributes.Locale] = loc.ID
				}
			}
			if err := validateVersionLocalizationCreateLocales(preparedLocalizations, localeToID); err != nil {
				return err
			}
			if err := validateScreenshotLocalizationCreateLocales(screenshotPlan, localeToID); err != nil {
				return err
			}
			appInfoCtx, cancelAppInfo := migrateRequestContext(ctx)
			appInfoPlan, err := prepareAppInfoLocalizations(appInfoCtx, client, resolvedAppID, preparedAppInfoLocalizations)
			cancelAppInfo()
			if err != nil {
				return err
			}

			submitOpts := shared.SubmitReadinessOptions{}
			if migrateVersionLocalizationsNeedUpdateContext(localizations, localeToID) {
				readinessCtx, cancelReadiness := migrateRequestContext(ctx)
				submitOpts = shared.ResolveSubmitReadinessOptionsForVersionBestEffort(readinessCtx, client, resolvedVersionID, resolvedAppID, "")
				cancelReadiness()
			}
			// Each stage records what it applied before the failure so an
			// interrupted import still reports the App Store Connect state it
			// left behind instead of printing nothing.
			completedStages := make([]string, 0, 4)
			var createWarnings []shared.SubmitReadinessCreateWarning
			reportPartialFailure := func(stage string, failure error) error {
				if !migrateImportAppliedAnything(result) {
					return failure
				}
				result.Status = migratePartialStatus
				result.FailureStage = stage
				result.Failure = shared.SanitizeTerminal(failure.Error())
				result.CompletedStages = append([]string(nil), completedStages...)
				shared.WarnIncludeSensitive(os.Stderr, *includeSensitive)
				if printErr := printMigrateOutput(presentableImportResult(result, *includeSensitive), *output.Output, *output.Pretty); printErr != nil {
					return errors.Join(failure, fmt.Errorf("print partial migrate import result: %w", printErr))
				}
				// Locales created before the failure still need the submission
				// fields the warning names, so report them here too.
				if warnErr := shared.PrintSubmitReadinessCreateWarnings(os.Stderr, createWarnings); warnErr != nil {
					return errors.Join(failure, warnErr)
				}
				return failure
			}

			uploaded, warnings, err := uploadVersionLocalizations(ctx, client, resolvedVersionID, preparedLocalizations, localeToID, submitOpts)
			result.Uploaded = uploaded
			createWarnings = warnings
			if err != nil {
				return reportPartialFailure(migrateStageVersionLocalizations, err)
			}
			if len(uploaded) > 0 {
				completedStages = append(completedStages, migrateStageVersionLocalizations)
			}

			appInfoUploaded, err := uploadAppInfoLocalizations(ctx, client, appInfoPlan)
			result.AppInfoUploaded = appInfoUploaded
			if err != nil {
				return reportPartialFailure(migrateStageAppInfoLocalizations, err)
			}
			if len(appInfoUploaded) > 0 {
				completedStages = append(completedStages, migrateStageAppInfoLocalizations)
			}

			reviewResult, err := uploadReviewInformation(ctx, client, resolvedVersionID, reviewInfo)
			result.ReviewInfoResult = reviewResult
			if err != nil {
				return reportPartialFailure(migrateStageReviewInformation, err)
			}
			if migrateReviewInfoApplied(reviewResult) {
				completedStages = append(completedStages, migrateStageReviewInformation)
			}

			screenshotResults, err := uploadScreenshots(ctx, client, resolvedVersionID, localeToID, screenshotPlan)
			result.ScreenshotResults = screenshotResults
			if err != nil {
				return reportPartialFailure(migrateStageScreenshots, err)
			}
			if migrateScreenshotsApplied(screenshotResults) {
				completedStages = append(completedStages, migrateStageScreenshots)
			}

			shared.WarnIncludeSensitive(os.Stderr, *includeSensitive)
			if err := printMigrateOutput(presentableImportResult(result, *includeSensitive), *output.Output, *output.Pretty); err != nil {
				return err
			}
			return shared.PrintSubmitReadinessCreateWarnings(os.Stderr, createWarnings)
		},
	}
}

// migrateImportAppliedAnything reports whether the run already changed App
// Store Connect. A failure before the first mutation keeps the plain error and
// an empty stdout.
func migrateImportAppliedAnything(result *MigrateImportResult) bool {
	if result == nil {
		return false
	}
	return len(result.Uploaded) > 0 ||
		len(result.AppInfoUploaded) > 0 ||
		migrateReviewInfoApplied(result.ReviewInfoResult) ||
		migrateScreenshotsApplied(result.ScreenshotResults)
}

// migrateReviewInfoApplied reports whether the review information stage changed
// the remote detail. A skip means App Store Connect already carried the
// imported values, so nothing was written.
func migrateReviewInfoApplied(result *ReviewInfoResult) bool {
	return result != nil && result.Action != migrateReviewInfoActionSkip
}

// migrateScreenshotsApplied reports whether the screenshot stage changed App
// Store Connect. A result that only lists assets which already existed left the
// version untouched, while a created set counts even when no asset finished
// uploading into it.
func migrateScreenshotsApplied(results []ScreenshotUploadResult) bool {
	for _, result := range results {
		if len(result.Uploaded) > 0 || result.createdSet {
			return true
		}
	}
	return false
}

func migrateVersionLocalizationsNeedUpdateContext(localizations []FastlaneLocalization, localeToID map[string]string) bool {
	for _, loc := range localizations {
		if strings.TrimSpace(localeToID[loc.Locale]) != "" {
			continue
		}
		if strings.TrimSpace(loc.WhatsNew) == "" {
			return true
		}
	}
	return false
}

// MigrateExportCommand returns the migrate export subcommand.
func MigrateExportCommand() *ffcli.Command {
	fs := flag.NewFlagSet("migrate export", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	versionID := fs.String("version-id", "", "App Store version ID (required)")
	outputDir := fs.String("output-dir", "", "Output directory for fastlane structure (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "export",
		ShortUsage: "asc migrate export [flags]",
		ShortHelp:  "Export metadata to fastlane directory structure.",
		LongHelp: `Export current App Store metadata to fastlane directory structure.

Creates the standard fastlane structure with all localizations.

Examples:
  asc migrate export --app "APP_ID" --version-id "VERSION_ID" --output-dir ./fastlane`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*versionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError("--version-id")
			}
			if strings.TrimSpace(*outputDir) == "" {
				fmt.Fprintln(os.Stderr, "Error: --output-dir is required")
				return shared.MissingRequiredUsageError("--output-dir")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("migrate export: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if _, err := shared.ResolveOwnedAppStoreVersionByID(requestCtx, client, resolvedAppID, strings.TrimSpace(*versionID), ""); err != nil {
				return fmt.Errorf("migrate export: %w", err)
			}

			// Fetch all localizations
			resp, err := client.GetAppStoreVersionLocalizations(requestCtx, strings.TrimSpace(*versionID))
			if err != nil {
				return fmt.Errorf("migrate export: %w", err)
			}

			// Create output directory structure beneath the operator-selected root
			root, err := newMigrateExportRoot(*outputDir)
			if err != nil {
				return fmt.Errorf("migrate export: %w", err)
			}
			if err := root.MkdirAll("metadata", 0o755); err != nil {
				return fmt.Errorf("migrate export: failed to create directory: %w", err)
			}

			// Write each localization
			exported := make([]string, 0, len(resp.Data))
			totalFiles := 0
			for _, loc := range resp.Data {
				locale := loc.Attributes.Locale
				localeDir, err := migrateExportLocaleDir(locale)
				if err != nil {
					return fmt.Errorf("migrate export: %w", err)
				}
				if err := root.MkdirAll(localeDir, 0o755); err != nil {
					return fmt.Errorf("migrate export: failed to create locale directory: %w", err)
				}

				// Write files (only non-empty content creates files)
				files := []struct {
					name    string
					content string
				}{
					{"description.txt", loc.Attributes.Description},
					{"keywords.txt", loc.Attributes.Keywords},
					{"release_notes.txt", loc.Attributes.WhatsNew},
					{"promotional_text.txt", loc.Attributes.PromotionalText},
					{"support_url.txt", loc.Attributes.SupportURL},
					{"marketing_url.txt", loc.Attributes.MarketingURL},
				}
				for _, file := range files {
					written, err := writeAndCount(root, filepath.Join(localeDir, file.name), file.content)
					if err != nil {
						return fmt.Errorf("migrate export: %w", err)
					}
					totalFiles += written
				}

				exported = append(exported, locale)
			}

			// Export App Info localizations (name, subtitle)
			appInfos, err := client.GetAppInfos(requestCtx, resolvedAppID)
			if err == nil && len(appInfos.Data) > 0 {
				appInfoID := shared.SelectBestAppInfoID(appInfos)
				if strings.TrimSpace(appInfoID) == "" {
					return fmt.Errorf("migrate export: failed to select app info for app")
				}
				appInfoLocs, err := client.GetAppInfoLocalizations(requestCtx, appInfoID)
				if err == nil {
					for _, loc := range appInfoLocs.Data {
						localeDir, err := migrateExportLocaleDir(loc.Attributes.Locale)
						if err != nil {
							return fmt.Errorf("migrate export: %w", err)
						}
						// Create locale dir if it doesn't exist (may have App Info but no version localizations)
						if err := root.MkdirAll(localeDir, 0o755); err != nil {
							return fmt.Errorf("migrate export: failed to create locale directory: %w", err)
						}
						files := []struct {
							name    string
							content string
						}{
							{"name.txt", loc.Attributes.Name},
							{"subtitle.txt", loc.Attributes.Subtitle},
							{"privacy_url.txt", loc.Attributes.PrivacyPolicyURL},
						}
						for _, file := range files {
							written, err := writeAndCount(root, filepath.Join(localeDir, file.name), file.content)
							if err != nil {
								return fmt.Errorf("migrate export: %w", err)
							}
							totalFiles += written
						}
					}
				}
			}

			result := &MigrateExportResult{
				VersionID:  strings.TrimSpace(*versionID),
				OutputDir:  *outputDir,
				Locales:    exported,
				TotalFiles: totalFiles,
			}

			return printMigrateOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// FastlaneLocalization holds version-level metadata read from fastlane structure.
type FastlaneLocalization struct {
	Locale          string `json:"locale"`
	Description     string `json:"description,omitempty"`
	Keywords        string `json:"keywords,omitempty"`
	WhatsNew        string `json:"whatsNew,omitempty"`
	PromotionalText string `json:"promotionalText,omitempty"`
	SupportURL      string `json:"supportUrl,omitempty"`
	MarketingURL    string `json:"marketingUrl,omitempty"`
}

// AppInfoFastlaneLocalization holds app-level metadata (name, subtitle) from fastlane.
type AppInfoFastlaneLocalization struct {
	Locale     string `json:"locale"`
	Name       string `json:"name,omitempty"`
	Subtitle   string `json:"subtitle,omitempty"`
	PrivacyURL string `json:"privacyUrl,omitempty"`
}

// LocalizationUploadItem represents an uploaded localization.
type LocalizationUploadItem struct {
	Locale         string `json:"locale"`
	Fields         int    `json:"fields"`
	Action         string `json:"action,omitempty"`
	LocalizationID string `json:"localizationId,omitempty"`
}

type LocalizationFilePlan struct {
	Locale string   `json:"locale"`
	Files  []string `json:"files"`
}

type ReviewInfoResult struct {
	Action   string `json:"action,omitempty"`
	DetailID string `json:"detailId,omitempty"`
}

type SkippedItem struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

const (
	migratePartialStatus             = "partial"
	migrateStageVersionLocalizations = "version_localizations"
	migrateStageAppInfoLocalizations = "app_info_localizations"
	migrateStageReviewInformation    = "review_information"
	migrateStageScreenshots          = "screenshots"
)

// Review information outcomes. Only create and update write to App Store
// Connect; skip means the remote detail already matched the imported values.
const (
	migrateReviewInfoActionCreate = "create"
	migrateReviewInfoActionUpdate = "update"
	migrateReviewInfoActionSkip   = "skip"
)

// MigrateImportResult is the result of a migrate import operation.
type MigrateImportResult struct {
	DryRun               bool                          `json:"dryRun"`
	Status               string                        `json:"status,omitempty"`
	FailureStage         string                        `json:"failureStage,omitempty"`
	Failure              string                        `json:"failure,omitempty"`
	CompletedStages      []string                      `json:"completedStages,omitempty"`
	VersionID            string                        `json:"versionId"`
	AppID                string                        `json:"appId,omitempty"`
	DeliverfilePath      string                        `json:"deliverfilePath,omitempty"`
	MetadataDir          string                        `json:"metadataDir,omitempty"`
	ScreenshotsDir       string                        `json:"screenshotsDir,omitempty"`
	Locales              []string                      `json:"locales,omitempty"`
	Localizations        []FastlaneLocalization        `json:"localizations,omitempty"`
	AppInfoLocalizations []AppInfoFastlaneLocalization `json:"appInfoLocalizations,omitempty"`
	MetadataFiles        []LocalizationFilePlan        `json:"metadataFiles,omitempty"`
	AppInfoFiles         []LocalizationFilePlan        `json:"appInfoFiles,omitempty"`
	ReviewInformation    *ReviewInformation            `json:"reviewInformation,omitempty"`
	ScreenshotPlan       []ScreenshotPlan              `json:"screenshotPlan,omitempty"`
	AppClip              *AppClipLayout                `json:"appClip,omitempty"`
	Previews             []PreviewLayout               `json:"previews,omitempty"`
	Skipped              []SkippedItem                 `json:"skipped,omitempty"`
	Uploaded             []LocalizationUploadItem      `json:"uploaded,omitempty"`
	AppInfoUploaded      []LocalizationUploadItem      `json:"appInfoUploaded,omitempty"`
	ReviewInfoResult     *ReviewInfoResult             `json:"reviewInfoResult,omitempty"`
	ScreenshotResults    []ScreenshotUploadResult      `json:"screenshotResults,omitempty"`
}

// MigrateExportResult is the result of a migrate export operation.
type MigrateExportResult struct {
	VersionID  string   `json:"versionId"`
	OutputDir  string   `json:"outputDir"`
	Locales    []string `json:"locales"`
	TotalFiles int      `json:"totalFiles"`
}

// readFastlaneMetadata reads metadata from a fastlane metadata directory.
func readFastlaneMetadata(metadataDir string) ([]FastlaneLocalization, error) {
	localeDirs, _, err := scanFastlaneMetadataLocaleDirs(metadataDir)
	if err != nil {
		return nil, err
	}
	return readFastlaneMetadataFromLocaleDirs(metadataDir, localeDirs)
}

// readFastlaneAppInfoMetadata reads app-level metadata (name, subtitle) from fastlane structure.
func readFastlaneAppInfoMetadata(metadataDir string) ([]AppInfoFastlaneLocalization, error) {
	localeDirs, _, err := scanFastlaneMetadataLocaleDirs(metadataDir)
	if err != nil {
		return nil, err
	}
	return readFastlaneAppInfoMetadataFromLocaleDirs(metadataDir, localeDirs)
}

// newMigrateContentRoot anchors metadata and screenshot reads for dir so
// repository-controlled directories and files cannot redirect reads to local
// secrets before they are published upstream. A directory inside the working
// directory (including the default fastlane/metadata and fastlane/screenshots
// layouts, whose components ship with the checkout) is anchored at the working
// directory so every component below it is validated; an operator-selected
// directory outside the working directory is its own trusted root. The returned
// prefix is the root-relative content directory.
func newMigrateContentRoot(dir string) (rootfs.Root, string, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return rootfs.Root{}, "", err
	}
	if cwd, cwdErr := os.Getwd(); cwdErr == nil {
		if root, rootErr := rootfs.New(cwd); rootErr == nil {
			if relative, relErr := filepath.Rel(root.Path(), absolute); relErr == nil {
				if relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
					return root, relative, nil
				}
			}
		}
	}
	root, err := rootfs.New(absolute)
	if err != nil {
		return rootfs.Root{}, "", err
	}
	return root, ".", nil
}

// checkContentRootContained refuses a repository-controlled content directory
// whose components below the trusted root traverse a symlink. The prefix "."
// marks an operator-selected external directory, which is trusted as given.
func checkContentRootContained(root rootfs.Root, prefix string) error {
	if prefix == "." {
		return nil
	}
	return root.CheckContained(prefix)
}

// newMigrateExportRoot anchors export writes to the operator-selected output
// directory, which may legitimately live outside the current repository.
func newMigrateExportRoot(outputDir string) (rootfs.Root, error) {
	return rootfs.New(outputDir)
}

// migrateExportLocaleDir builds the export-relative locale directory for a
// locale returned by App Store Connect.
func migrateExportLocaleDir(locale string) (string, error) {
	trimmed := strings.TrimSpace(locale)
	if trimmed == "" {
		return "", fmt.Errorf("app store connect returned an empty locale")
	}
	if err := rootfs.ValidateRelative(trimmed); err != nil {
		return "", err
	}
	return filepath.Join("metadata", trimmed), nil
}

// readMetadataFile reads an optional metadata file beneath the metadata root.
// A missing file yields an empty value; a symlinked file is an error.
func readMetadataFile(root rootfs.Root, name string) (string, error) {
	data, found, err := root.ReadFileOptional(name)
	if err != nil {
		return "", err
	}
	if !found {
		return "", nil
	}
	return strings.TrimSpace(string(data)), nil
}

// writeAndCount writes content beneath root and returns 1 when a file was
// written and 0 when the content was empty. An existing destination keeps its
// permissions, matching the previous in-place write.
func writeAndCount(root rootfs.Root, name, content string) (int, error) {
	if content == "" {
		return 0, nil
	}
	if err := root.WriteFilePreservingMode(name, []byte(content+"\n"), 0o644); err != nil {
		return 0, err
	}
	return 1, nil
}

// printMigrateOutput handles output for migrate-specific result types.
func printMigrateOutput(data any, format string, pretty bool) error {
	normalizedFormat := shared.NormalizeOutputFormat(format)
	return shared.PrintOutputWithRenderers(
		data,
		normalizedFormat,
		pretty,
		func() error {
			switch v := data.(type) {
			case *MigrateImportResult:
				return printMigrateImportResultTable(v)
			case *MigrateExportResult:
				return printMigrateExportResultTable(v)
			case *MigrateValidateResult:
				return printMigrateValidateResultTable(v)
			default:
				return fmt.Errorf("unsupported format: %s", normalizedFormat)
			}
		},
		func() error {
			switch v := data.(type) {
			case *MigrateImportResult:
				return printMigrateImportResultMarkdown(v)
			case *MigrateExportResult:
				return printMigrateExportResultMarkdown(v)
			case *MigrateValidateResult:
				return printMigrateValidateResultMarkdown(v)
			default:
				return fmt.Errorf("unsupported format: %s", normalizedFormat)
			}
		},
	)
}

// countNonEmptyFields counts the number of non-empty fields in a localization.
func countNonEmptyFields(loc FastlaneLocalization) int {
	count := 0
	fields := []string{
		loc.Description,
		loc.Keywords,
		loc.WhatsNew,
		loc.PromotionalText,
		loc.SupportURL,
		loc.MarketingURL,
	}
	for _, f := range fields {
		if f != "" {
			count++
		}
	}
	return count
}

func countAppInfoFields(loc AppInfoFastlaneLocalization) int {
	count := 0
	if loc.Name != "" {
		count++
	}
	if loc.Subtitle != "" {
		count++
	}
	if loc.PrivacyURL != "" {
		count++
	}
	return count
}

func versionLocalizationFiles(loc FastlaneLocalization) []string {
	files := []string{}
	if loc.Description != "" {
		files = append(files, "description.txt")
	}
	if loc.Keywords != "" {
		files = append(files, "keywords.txt")
	}
	if loc.WhatsNew != "" {
		files = append(files, "release_notes.txt")
	}
	if loc.PromotionalText != "" {
		files = append(files, "promotional_text.txt")
	}
	if loc.SupportURL != "" {
		files = append(files, "support_url.txt")
	}
	if loc.MarketingURL != "" {
		files = append(files, "marketing_url.txt")
	}
	return files
}

func appInfoLocalizationFiles(loc AppInfoFastlaneLocalization) []string {
	files := []string{}
	if loc.Name != "" {
		files = append(files, "name.txt")
	}
	if loc.Subtitle != "" {
		files = append(files, "subtitle.txt")
	}
	if loc.PrivacyURL != "" {
		files = append(files, "privacy_url.txt")
	}
	return files
}

// ValidationIssue represents a validation error or warning.
type ValidationIssue struct {
	Locale   string `json:"locale"`
	Field    string `json:"field"`
	Severity string `json:"severity"` // "error" or "warning"
	Message  string `json:"message"`
	Length   int    `json:"length,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// MigrateValidateResult is the result of a migrate validate operation.
type MigrateValidateResult struct {
	FastlaneDir string            `json:"fastlaneDir"`
	Locales     []string          `json:"locales"`
	Issues      []ValidationIssue `json:"issues"`
	Skipped     []SkippedItem     `json:"skipped,omitempty"`
	ErrorCount  int               `json:"errorCount"`
	WarnCount   int               `json:"warnCount"`
	Valid       bool              `json:"valid"`
}

type fastlaneLocaleDir struct {
	DirName string
	Locale  string
}

func scanFastlaneMetadataLocaleDirs(metadataDir string) ([]fastlaneLocaleDir, []SkippedItem, error) {
	root, prefix, err := newMigrateContentRoot(metadataDir)
	if err != nil {
		return nil, nil, err
	}
	if err := checkContentRootContained(root, prefix); err != nil {
		return nil, nil, err
	}
	entries, err := os.ReadDir(metadataDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read metadata directory: %w", err)
	}

	seen := make(map[string]string)
	dirs := make([]fastlaneLocaleDir, 0, len(entries))
	skipped := []SkippedItem{}
	for _, entry := range entries {
		dirName := entry.Name()
		if entry.Type()&os.ModeSymlink != 0 {
			// A symlinked locale directory would read metadata from outside the
			// metadata root, so report it instead of silently following it.
			skipped = append(skipped, SkippedItem{
				Path:   filepath.Join(metadataDir, dirName),
				Reason: fmt.Sprintf("skipped symlinked metadata entry %q", dirName),
			})
			continue
		}
		if !entry.IsDir() {
			continue
		}

		if dirName == "review_information" || dirName == "default" {
			continue // Skip special directories
		}

		if err := rootfs.ValidateRelative(dirName); err != nil {
			return nil, nil, err
		}

		normalized, err := normalizeLocale(dirName)
		if err != nil {
			// Keep import/validate running, but surface a warning via structured output.
			skipped = append(skipped, SkippedItem{
				Path:   filepath.Join(metadataDir, dirName),
				Reason: fmt.Sprintf("skipped non-locale directory %q: %v", dirName, err),
			})
			continue
		}

		if other, ok := seen[normalized]; ok {
			return nil, nil, fmt.Errorf("duplicate locale %q in metadata (dirs: %q, %q)", normalized, other, dirName)
		}
		seen[normalized] = dirName

		dirs = append(dirs, fastlaneLocaleDir{
			DirName: dirName,
			Locale:  normalized,
		})
	}

	return dirs, skipped, nil
}

func readFastlaneMetadataFromLocaleDirs(metadataDir string, localeDirs []fastlaneLocaleDir) ([]FastlaneLocalization, error) {
	root, prefix, err := newMigrateContentRoot(metadataDir)
	if err != nil {
		return nil, err
	}
	if err := checkContentRootContained(root, prefix); err != nil {
		return nil, err
	}

	localizations := make([]FastlaneLocalization, 0, len(localeDirs))
	for _, ld := range localeDirs {
		loc := FastlaneLocalization{Locale: ld.Locale}

		// Read each metadata file (version-level localization fields only)
		fields := []struct {
			file  string
			field *string
		}{
			{"description.txt", &loc.Description},
			{"keywords.txt", &loc.Keywords},
			{"release_notes.txt", &loc.WhatsNew},
			{"promotional_text.txt", &loc.PromotionalText},
			{"support_url.txt", &loc.SupportURL},
			{"marketing_url.txt", &loc.MarketingURL},
		}
		for _, field := range fields {
			value, err := readMetadataFile(root, filepath.Join(prefix, ld.DirName, field.file))
			if err != nil {
				return nil, err
			}
			*field.field = value
		}

		localizations = append(localizations, loc)
	}
	return localizations, nil
}

func readFastlaneAppInfoMetadataFromLocaleDirs(metadataDir string, localeDirs []fastlaneLocaleDir) ([]AppInfoFastlaneLocalization, error) {
	root, prefix, err := newMigrateContentRoot(metadataDir)
	if err != nil {
		return nil, err
	}
	if err := checkContentRootContained(root, prefix); err != nil {
		return nil, err
	}

	localizations := make([]AppInfoFastlaneLocalization, 0, len(localeDirs))
	for _, ld := range localeDirs {
		name, err := readMetadataFile(root, filepath.Join(prefix, ld.DirName, "name.txt"))
		if err != nil {
			return nil, err
		}
		subtitle, err := readMetadataFile(root, filepath.Join(prefix, ld.DirName, "subtitle.txt"))
		if err != nil {
			return nil, err
		}
		privacyURL, err := readMetadataFile(root, filepath.Join(prefix, ld.DirName, "privacy_url.txt"))
		if err != nil {
			return nil, err
		}

		// Only include if at least one field has content
		if name != "" || subtitle != "" || privacyURL != "" {
			localizations = append(localizations, AppInfoFastlaneLocalization{
				Locale:     ld.Locale,
				Name:       name,
				Subtitle:   subtitle,
				PrivacyURL: privacyURL,
			})
		}
	}
	return localizations, nil
}

// MigrateValidateCommand returns the migrate validate subcommand.
func MigrateValidateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("migrate validate", flag.ExitOnError)

	fastlaneDir := fs.String("fastlane-dir", "", "Path to fastlane directory (required)")
	allowExternalMetadata := fs.Bool("allow-external-metadata", false, "Trust a metadata symlink outside the selected Fastlane directory")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "validate",
		ShortUsage: "asc migrate validate [flags]",
		ShortHelp:  "Validate fastlane metadata without uploading.",
		LongHelp: `Validate fastlane metadata without making any API calls.

Checks character limits for App Store Connect metadata:
  - Description: 4000 characters
  - Keywords: 100 characters
  - What's New (release notes): 4000 characters
  - Promotional Text: 170 characters
  - Name: 30 characters
  - Subtitle: 30 characters

Examples:
  asc migrate validate --fastlane-dir ./fastlane
  asc migrate validate --fastlane-dir ./fastlane --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*fastlaneDir) == "" {
				fmt.Fprintln(os.Stderr, "Error: --fastlane-dir is required")
				return shared.MissingRequiredUsageError("--fastlane-dir")
			}

			workDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("migrate validate: %w", err)
			}

			// Resolve the metadata directory exactly the way migrate import
			// does, so a Deliverfile metadata_path cannot leave validate
			// checking a directory the import step never reads.
			inputs, skipped, err := resolveImportInputs(importInputOptions{
				WorkDir:               workDir,
				FastlaneDir:           *fastlaneDir,
				MetadataOnly:          true,
				AllowExternalMetadata: *allowExternalMetadata,
			})
			if err != nil {
				return fmt.Errorf("migrate validate: %w", err)
			}

			metadataDir := inputs.MetadataDir
			if inputs.DeliverfileConfig.SkipMetadata && metadataDir != "" {
				skipped = append(skipped, SkippedItem{
					Path:   metadataDir,
					Reason: "skip_metadata in Deliverfile",
				})
				metadataDir = ""
			}

			// Read metadata from fastlane structure
			var localizations []FastlaneLocalization
			var appInfoLocs []AppInfoFastlaneLocalization
			if metadataDir != "" {
				localeDirs, metadataSkipped, err := scanFastlaneMetadataLocaleDirs(metadataDir)
				if err != nil {
					if os.IsNotExist(err) {
						return fmt.Errorf("migrate validate: metadata directory not found: %s", metadataDir)
					}
					return fmt.Errorf("migrate validate: %w", err)
				}
				skipped = append(skipped, metadataSkipped...)

				localizations, err = readFastlaneMetadataFromLocaleDirs(metadataDir, localeDirs)
				if err != nil {
					return fmt.Errorf("migrate validate: %w", err)
				}

				// Read App Info metadata (name, subtitle)
				appInfoLocs, err = readFastlaneAppInfoMetadataFromLocaleDirs(metadataDir, localeDirs)
				if err != nil {
					return fmt.Errorf("migrate validate: %w", err)
				}
				if _, _, err := readAppClipLayout(metadataDir); err != nil {
					return fmt.Errorf("migrate validate: %w", err)
				}
			}
			if _, err := readPreviewLayout(*fastlaneDir); err != nil {
				return fmt.Errorf("migrate validate: %w", err)
			}

			// Validate and collect issues
			var issues []ValidationIssue
			var locales []string

			for _, loc := range localizations {
				locales = append(locales, loc.Locale)
				// migrate import rejects these locales when it has to create a
				// localization, so report them here instead of passing a tree
				// the import step refuses.
				if issue := localeCreateIssue(loc.Locale); issue != nil {
					issues = append(issues, *issue)
				}
				issues = append(issues, validateVersionLocalization(loc)...)
			}

			for _, loc := range appInfoLocs {
				issues = append(issues, validateAppInfoLocalization(loc)...)
			}

			// Count errors and warnings
			errorCount := 0
			warnCount := 0
			for _, issue := range issues {
				if issue.Severity == "error" {
					errorCount++
				} else {
					warnCount++
				}
			}

			result := &MigrateValidateResult{
				FastlaneDir: *fastlaneDir,
				Locales:     locales,
				Issues:      issues,
				Skipped:     skipped,
				ErrorCount:  errorCount,
				WarnCount:   warnCount,
				Valid:       errorCount == 0,
			}

			return printMigrateOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// validateVersionLocalization checks version-level metadata for issues.
func validateVersionLocalization(loc FastlaneLocalization) []ValidationIssue {
	var issues []ValidationIssue

	descriptionLength := utf8.RuneCountInString(loc.Description)
	if descriptionLength > validation.LimitDescription {
		issues = append(issues, ValidationIssue{
			Locale:   loc.Locale,
			Field:    "description",
			Severity: "error",
			Message:  fmt.Sprintf("exceeds %d character limit", validation.LimitDescription),
			Length:   descriptionLength,
			Limit:    validation.LimitDescription,
		})
	}

	if issue := validation.KeywordFieldLengthIssue(loc.Keywords); issue != nil {
		limitUnit := strings.TrimSuffix(issue.Unit, "s")
		issues = append(issues, ValidationIssue{
			Locale:   loc.Locale,
			Field:    "keywords",
			Severity: "error",
			Message:  fmt.Sprintf("exceeds %d-%s limit", issue.Limit, limitUnit),
			Length:   issue.Length,
			Limit:    issue.Limit,
		})
	}

	whatsNewLength := utf8.RuneCountInString(loc.WhatsNew)
	if whatsNewLength > validation.LimitWhatsNew {
		issues = append(issues, ValidationIssue{
			Locale:   loc.Locale,
			Field:    "whatsNew",
			Severity: "error",
			Message:  fmt.Sprintf("exceeds %d character limit", validation.LimitWhatsNew),
			Length:   whatsNewLength,
			Limit:    validation.LimitWhatsNew,
		})
	}

	promotionalTextLength := utf8.RuneCountInString(loc.PromotionalText)
	if promotionalTextLength > validation.LimitPromotionalText {
		issues = append(issues, ValidationIssue{
			Locale:   loc.Locale,
			Field:    "promotionalText",
			Severity: "error",
			Message:  fmt.Sprintf("exceeds %d character limit", validation.LimitPromotionalText),
			Length:   promotionalTextLength,
			Limit:    validation.LimitPromotionalText,
		})
	}

	// Warn if description is empty (usually required)
	if loc.Description == "" {
		issues = append(issues, ValidationIssue{
			Locale:   loc.Locale,
			Field:    "description",
			Severity: "warning",
			Message:  "description is empty (usually required)",
		})
	}

	return issues
}

// validateAppInfoLocalization checks app-level metadata for issues.
func validateAppInfoLocalization(loc AppInfoFastlaneLocalization) []ValidationIssue {
	var issues []ValidationIssue

	nameLength := utf8.RuneCountInString(loc.Name)
	if nameLength > validation.LimitName {
		issues = append(issues, ValidationIssue{
			Locale:   loc.Locale,
			Field:    "name",
			Severity: "error",
			Message:  fmt.Sprintf("exceeds %d character limit", validation.LimitName),
			Length:   nameLength,
			Limit:    validation.LimitName,
		})
	}

	subtitleLength := utf8.RuneCountInString(loc.Subtitle)
	if subtitleLength > validation.LimitSubtitle {
		issues = append(issues, ValidationIssue{
			Locale:   loc.Locale,
			Field:    "subtitle",
			Severity: "error",
			Message:  fmt.Sprintf("exceeds %d character limit", validation.LimitSubtitle),
			Length:   subtitleLength,
			Limit:    validation.LimitSubtitle,
		})
	}

	return issues
}
