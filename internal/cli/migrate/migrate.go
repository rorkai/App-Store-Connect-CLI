package migrate

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
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
  asc migrate import --app "APP_ID" --version "VERSION_ID" --fastlane-dir ./fastlane
  asc migrate export --app "APP_ID" --version "VERSION_ID" --output-dir ./fastlane`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			MigrateImportCommand(),
			MigrateExportCommand(),
			MigrateValidateCommand(),
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
  ├── screenshots/
  │   ├── en-US/
  │   │   ├── iphone_65_1.png
  │   │   └── ...

Examples:
  asc migrate import --app "APP_ID" --version-id "VERSION_ID" --fastlane-dir ./fastlane
  asc migrate import --app "APP_ID" --version-id "VERSION_ID" --fastlane-dir ./fastlane --dry-run`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			workDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("migrate import: %w", err)
			}

			inputs, skipped, err := resolveImportInputs(importInputOptions{
				WorkDir:     workDir,
				FastlaneDir: strings.TrimSpace(*fastlaneDir),
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

			var localizations []FastlaneLocalization
			var appInfoLocs []AppInfoFastlaneLocalization
			var reviewInfo *ReviewInformation
			if metadataDir != "" {
				localizations, err = readFastlaneMetadata(metadataDir)
				if err != nil {
					return fmt.Errorf("migrate import: %w", err)
				}
				appInfoLocs, err = readFastlaneAppInfoMetadata(metadataDir)
				if err != nil {
					return fmt.Errorf("migrate import: %w", err)
				}
				reviewInfo, err = readFastlaneReviewInformation(metadataDir)
				if err != nil {
					return fmt.Errorf("migrate import: %w", err)
				}
			}

			var screenshotPlan []ScreenshotPlan
			var skippedScreenshots []SkippedItem
			if screenshotsDir != "" {
				screenshotPlan, skippedScreenshots, err = discoverScreenshotPlan(screenshotsDir)
				if err != nil {
					return fmt.Errorf("migrate import: %w", err)
				}
				skipped = append(skipped, skippedScreenshots...)
			}

			locales := collectLocales(localizations, appInfoLocs, screenshotPlan)
			metadataFiles := buildMetadataFilePlans(localizations)
			appInfoFiles := buildAppInfoFilePlans(appInfoLocs)

			if strings.TrimSpace(*versionID) == "" && (strings.TrimSpace(inputs.DeliverfileConfig.AppVersion) == "" || strings.TrimSpace(inputs.DeliverfileConfig.Platform) == "") {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required (or set Deliverfile app_version and platform)")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*appID) == "" && strings.TrimSpace(inputs.DeliverfileConfig.AppIdentifier) == "" && shared.ResolveAppID("") == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID or Deliverfile app_identifier)")
				return flag.ErrHelp
			}

			var client *asc.Client
			var requestCtx context.Context
			var cancel context.CancelFunc
			needsClient := !*dryRun ||
				(strings.TrimSpace(*appID) == "" && strings.TrimSpace(inputs.DeliverfileConfig.AppIdentifier) != "") ||
				(strings.TrimSpace(*versionID) == "" && strings.TrimSpace(inputs.DeliverfileConfig.AppVersion) != "" && strings.TrimSpace(inputs.DeliverfileConfig.Platform) != "")
			if needsClient {
				client, err = shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("migrate import: %w", err)
				}
				requestCtx, cancel = shared.ContextWithTimeout(ctx)
				defer cancel()
			} else {
				requestCtx = ctx
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
				Skipped:              skipped,
			}

			if *dryRun {
				return printMigrateOutput(result, *output.Output, *output.Pretty)
			}

			if client == nil {
				client, err = shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("migrate import: %w", err)
				}
			}
			if requestCtx == nil {
				requestCtx, cancel = shared.ContextWithTimeout(ctx)
				defer cancel()
			}

			localeToID := make(map[string]string)
			if len(localizations) > 0 || len(screenshotPlan) > 0 {
				existingLocs, err := client.GetAppStoreVersionLocalizations(requestCtx, strings.TrimSpace(resolvedVersionID), asc.WithAppStoreVersionLocalizationsLimit(200))
				if err != nil {
					return fmt.Errorf("migrate import: failed to fetch existing localizations: %w", err)
				}
				for _, loc := range existingLocs.Data {
					localeToID[loc.Attributes.Locale] = loc.ID
				}
			}

			uploaded, err := uploadVersionLocalizations(requestCtx, client, resolvedVersionID, localizations, localeToID)
			if err != nil {
				return err
			}
			appInfoUploaded, err := uploadAppInfoLocalizations(requestCtx, client, resolvedAppID, appInfoLocs)
			if err != nil {
				return err
			}
			reviewResult, err := uploadReviewInformation(requestCtx, client, resolvedVersionID, reviewInfo)
			if err != nil {
				return err
			}
			screenshotResults, err := uploadScreenshots(requestCtx, client, resolvedVersionID, localeToID, screenshotPlan)
			if err != nil {
				return err
			}

			result.Uploaded = uploaded
			result.AppInfoUploaded = appInfoUploaded
			result.ReviewInfoResult = reviewResult
			result.ScreenshotResults = screenshotResults

			return printMigrateOutput(result, *output.Output, *output.Pretty)
		},
	}
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
				return flag.ErrHelp
			}
			if strings.TrimSpace(*outputDir) == "" {
				fmt.Fprintln(os.Stderr, "Error: --output-dir is required")
				return flag.ErrHelp
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("migrate export: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			// Fetch all localizations
			resp, err := client.GetAppStoreVersionLocalizations(requestCtx, strings.TrimSpace(*versionID))
			if err != nil {
				return fmt.Errorf("migrate export: %w", err)
			}

			// Create output directory structure
			metadataDir := filepath.Join(*outputDir, "metadata")
			if err := os.MkdirAll(metadataDir, 0o755); err != nil {
				return fmt.Errorf("migrate export: failed to create directory: %w", err)
			}

			// Write each localization
			exported := make([]string, 0, len(resp.Data))
			totalFiles := 0
			for _, loc := range resp.Data {
				locale := loc.Attributes.Locale
				localeDir := filepath.Join(metadataDir, locale)
				if err := os.MkdirAll(localeDir, 0o755); err != nil {
					return fmt.Errorf("migrate export: failed to create locale directory: %w", err)
				}

				// Write files (only non-empty content creates files)
				totalFiles += writeAndCount(filepath.Join(localeDir, "description.txt"), loc.Attributes.Description)
				totalFiles += writeAndCount(filepath.Join(localeDir, "keywords.txt"), loc.Attributes.Keywords)
				totalFiles += writeAndCount(filepath.Join(localeDir, "release_notes.txt"), loc.Attributes.WhatsNew)
				totalFiles += writeAndCount(filepath.Join(localeDir, "promotional_text.txt"), loc.Attributes.PromotionalText)
				totalFiles += writeAndCount(filepath.Join(localeDir, "support_url.txt"), loc.Attributes.SupportURL)
				totalFiles += writeAndCount(filepath.Join(localeDir, "marketing_url.txt"), loc.Attributes.MarketingURL)

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
						locale := loc.Attributes.Locale
						localeDir := filepath.Join(metadataDir, locale)
						// Create locale dir if it doesn't exist (may have App Info but no version localizations)
						if err := os.MkdirAll(localeDir, 0o755); err == nil {
							totalFiles += writeAndCount(filepath.Join(localeDir, "name.txt"), loc.Attributes.Name)
							totalFiles += writeAndCount(filepath.Join(localeDir, "subtitle.txt"), loc.Attributes.Subtitle)
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

// MigrateImportResult is the result of a migrate import operation.
type MigrateImportResult struct {
	DryRun               bool                          `json:"dryRun"`
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
	entries, err := os.ReadDir(metadataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata directory: %w", err)
	}

	seen := make(map[string]bool)
	var localizations []FastlaneLocalization
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		locale := entry.Name()
		if locale == "review_information" || locale == "default" {
			continue // Skip special directories
		}

		normalized, err := normalizeLocale(locale)
		if err != nil {
			return nil, fmt.Errorf("invalid locale %q in metadata: %w", locale, err)
		}
		if seen[normalized] {
			return nil, fmt.Errorf("duplicate locale %q in metadata", normalized)
		}
		seen[normalized] = true

		localeDir := filepath.Join(metadataDir, locale)
		loc := FastlaneLocalization{Locale: normalized}

		// Read each metadata file (version-level localization fields only)
		loc.Description = readFileIfExists(filepath.Join(localeDir, "description.txt"))
		loc.Keywords = readFileIfExists(filepath.Join(localeDir, "keywords.txt"))
		loc.WhatsNew = readFileIfExists(filepath.Join(localeDir, "release_notes.txt"))
		loc.PromotionalText = readFileIfExists(filepath.Join(localeDir, "promotional_text.txt"))
		loc.SupportURL = readFileIfExists(filepath.Join(localeDir, "support_url.txt"))
		loc.MarketingURL = readFileIfExists(filepath.Join(localeDir, "marketing_url.txt"))

		localizations = append(localizations, loc)
	}

	return localizations, nil
}

// readFastlaneAppInfoMetadata reads app-level metadata (name, subtitle) from fastlane structure.
func readFastlaneAppInfoMetadata(metadataDir string) ([]AppInfoFastlaneLocalization, error) {
	entries, err := os.ReadDir(metadataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata directory: %w", err)
	}

	seen := make(map[string]bool)
	var localizations []AppInfoFastlaneLocalization
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		locale := entry.Name()
		if locale == "review_information" || locale == "default" {
			continue
		}

		normalized, err := normalizeLocale(locale)
		if err != nil {
			return nil, fmt.Errorf("invalid locale %q in metadata: %w", locale, err)
		}
		if seen[normalized] {
			return nil, fmt.Errorf("duplicate locale %q in metadata", normalized)
		}
		seen[normalized] = true

		localeDir := filepath.Join(metadataDir, locale)
		name := readFileIfExists(filepath.Join(localeDir, "name.txt"))
		subtitle := readFileIfExists(filepath.Join(localeDir, "subtitle.txt"))
		privacyURL := readFileIfExists(filepath.Join(localeDir, "privacy_url.txt"))

		// Only include if at least one field has content
		if name != "" || subtitle != "" || privacyURL != "" {
			localizations = append(localizations, AppInfoFastlaneLocalization{
				Locale:     normalized,
				Name:       name,
				Subtitle:   subtitle,
				PrivacyURL: privacyURL,
			})
		}
	}

	return localizations, nil
}

// readFileIfExists reads a file's contents if it exists, returning empty string otherwise.
func readFileIfExists(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// writeAndCount writes content to a file and returns 1 if written, 0 if skipped.
func writeAndCount(path, content string) int {
	if content == "" {
		return 0
	}
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		return 0
	}
	return 1
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
	ErrorCount  int               `json:"errorCount"`
	WarnCount   int               `json:"warnCount"`
	Valid       bool              `json:"valid"`
}

// MigrateValidateCommand returns the migrate validate subcommand.
func MigrateValidateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("migrate validate", flag.ExitOnError)

	fastlaneDir := fs.String("fastlane-dir", "", "Path to fastlane directory (required)")
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
				return flag.ErrHelp
			}

			metadataDir := filepath.Join(*fastlaneDir, "metadata")

			// Read metadata from fastlane structure
			localizations, err := readFastlaneMetadata(metadataDir)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("migrate validate: metadata directory not found: %s", metadataDir)
				}
				return fmt.Errorf("migrate validate: %w", err)
			}

			// Read App Info metadata (name, subtitle)
			appInfoLocs, err := readFastlaneAppInfoMetadata(metadataDir)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("migrate validate: metadata directory not found: %s", metadataDir)
				}
				return fmt.Errorf("migrate validate: %w", err)
			}

			// Validate and collect issues
			var issues []ValidationIssue
			var locales []string

			for _, loc := range localizations {
				locales = append(locales, loc.Locale)
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

	keywordsLength := utf8.RuneCountInString(loc.Keywords)
	if keywordsLength > validation.LimitKeywords {
		issues = append(issues, ValidationIssue{
			Locale:   loc.Locale,
			Field:    "keywords",
			Severity: "error",
			Message:  fmt.Sprintf("exceeds %d character limit", validation.LimitKeywords),
			Length:   keywordsLength,
			Limit:    validation.LimitKeywords,
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
