package migrate

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
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
	versionID := fs.String("version-id", "", "App Store version ID (required)")
	fastlaneDir := fs.String("fastlane-dir", "", "Path to fastlane directory (required)")
	dryRun := fs.Bool("dry-run", false, "Preview changes without uploading")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "import",
		ShortUsage: "asc migrate import [flags]",
		ShortHelp:  "Import metadata from fastlane directory structure.",
		LongHelp: `Import metadata from fastlane directory structure.

Reads from the standard fastlane structure:
  fastlane/
  ├── metadata/
  │   ├── en-US/
  │   │   ├── name.txt           (App Info)
  │   │   ├── subtitle.txt       (App Info)
  │   │   ├── description.txt    (Version)
  │   │   ├── keywords.txt       (Version)
  │   │   ├── release_notes.txt  (Version)
  │   │   ├── promotional_text.txt (Version)
  │   │   ├── support_url.txt    (Version)
  │   │   └── marketing_url.txt  (Version)
  │   └── de-DE/
  │       └── ...

Note: privacy_url.txt is not supported (app-level, not localized).

Examples:
  asc migrate import --app "APP_ID" --version-id "VERSION_ID" --fastlane-dir ./fastlane
  asc migrate import --app "APP_ID" --version-id "VERSION_ID" --fastlane-dir ./fastlane --dry-run`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*versionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*fastlaneDir) == "" {
				fmt.Fprintln(os.Stderr, "Error: --fastlane-dir is required")
				return flag.ErrHelp
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			metadataDir := filepath.Join(*fastlaneDir, "metadata")

			// Read metadata from fastlane structure
			localizations, err := readFastlaneMetadata(metadataDir)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("migrate import: metadata directory not found: %s", metadataDir)
				}
				return fmt.Errorf("migrate import: %w", err)
			}

			// Read App Info metadata (name, subtitle)
			appInfoLocs, err := readFastlaneAppInfoMetadata(metadataDir)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("migrate import: metadata directory not found: %s", metadataDir)
				}
				return fmt.Errorf("migrate import: %w", err)
			}

			if *dryRun {
				result := &MigrateImportResult{
					DryRun:               true,
					VersionID:            strings.TrimSpace(*versionID),
					Localizations:        localizations,
					AppInfoLocalizations: appInfoLocs,
				}
				return printMigrateOutput(result, *output, *pretty)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("migrate import: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			// Fetch existing localizations to get their IDs
			existingLocs, err := client.GetAppStoreVersionLocalizations(requestCtx, strings.TrimSpace(*versionID))
			if err != nil {
				return fmt.Errorf("migrate import: failed to fetch existing localizations: %w", err)
			}

			// Build a map of locale -> localization ID
			localeToID := make(map[string]string)
			for _, loc := range existingLocs.Data {
				localeToID[loc.Attributes.Locale] = loc.ID
			}

			// Upload each localization
			uploaded := make([]LocalizationUploadItem, 0, len(localizations))
			for _, loc := range localizations {
				attrs := asc.AppStoreVersionLocalizationAttributes{
					Locale:          loc.Locale,
					Description:     loc.Description,
					Keywords:        loc.Keywords,
					WhatsNew:        loc.WhatsNew,
					PromotionalText: loc.PromotionalText,
					SupportURL:      loc.SupportURL,
					MarketingURL:    loc.MarketingURL,
				}

				// Check if localization already exists
				if existingID, exists := localeToID[loc.Locale]; exists {
					// Update existing localization
					_, err := client.UpdateAppStoreVersionLocalization(requestCtx, existingID, attrs)
					if err != nil {
						return fmt.Errorf("migrate import: failed to update %s: %w", loc.Locale, err)
					}
				} else {
					// Create new localization
					_, err := client.CreateAppStoreVersionLocalization(requestCtx, strings.TrimSpace(*versionID), attrs)
					if err != nil {
						return fmt.Errorf("migrate import: failed to create %s: %w", loc.Locale, err)
					}
				}

				uploaded = append(uploaded, LocalizationUploadItem{
					Locale: loc.Locale,
					Fields: countNonEmptyFields(loc),
				})
			}

			// Upload App Info localizations (name, subtitle)
			appInfoUploaded := make([]LocalizationUploadItem, 0, len(appInfoLocs))
			if len(appInfoLocs) > 0 {
				// Get AppInfo ID for the app
				appInfos, err := client.GetAppInfos(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("migrate import: failed to get app info: %w", err)
				}
				if len(appInfos.Data) == 0 {
					return fmt.Errorf("migrate import: no app info found for app")
				}
				appInfoID := selectBestAppInfoID(appInfos)
				if strings.TrimSpace(appInfoID) == "" {
					return fmt.Errorf("migrate import: failed to select app info for app")
				}

				// Get existing App Info localizations
				existingAppInfoLocs, err := client.GetAppInfoLocalizations(requestCtx, appInfoID)
				if err != nil {
					return fmt.Errorf("migrate import: failed to fetch app info localizations: %w", err)
				}

				// Build locale -> ID map
				appInfoLocaleToID := make(map[string]string)
				for _, loc := range existingAppInfoLocs.Data {
					appInfoLocaleToID[loc.Attributes.Locale] = loc.ID
				}

				// Upload each App Info localization
				for _, loc := range appInfoLocs {
					attrs := asc.AppInfoLocalizationAttributes{
						Locale:   loc.Locale,
						Name:     loc.Name,
						Subtitle: loc.Subtitle,
					}

					if existingID, exists := appInfoLocaleToID[loc.Locale]; exists {
						_, err := client.UpdateAppInfoLocalization(requestCtx, existingID, attrs)
						if err != nil {
							return fmt.Errorf("migrate import: failed to update app info %s: %w", loc.Locale, err)
						}
					} else {
						_, err := client.CreateAppInfoLocalization(requestCtx, appInfoID, attrs)
						if err != nil {
							return fmt.Errorf("migrate import: failed to create app info %s: %w", loc.Locale, err)
						}
					}

					fields := 0
					if loc.Name != "" {
						fields++
					}
					if loc.Subtitle != "" {
						fields++
					}
					appInfoUploaded = append(appInfoUploaded, LocalizationUploadItem{
						Locale: loc.Locale,
						Fields: fields,
					})
				}
			}

			result := &MigrateImportResult{
				DryRun:               false,
				VersionID:            strings.TrimSpace(*versionID),
				Localizations:        localizations,
				AppInfoLocalizations: appInfoLocs,
				Uploaded:             uploaded,
				AppInfoUploaded:      appInfoUploaded,
			}

			return printMigrateOutput(result, *output, *pretty)
		},
	}
}

// MigrateExportCommand returns the migrate export subcommand.
func MigrateExportCommand() *ffcli.Command {
	fs := flag.NewFlagSet("migrate export", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	versionID := fs.String("version-id", "", "App Store version ID (required)")
	outputDir := fs.String("output-dir", "", "Output directory for fastlane structure (required)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

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
				appInfoID := selectBestAppInfoID(appInfos)
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

			return printMigrateOutput(result, *output, *pretty)
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
	Locale   string `json:"locale"`
	Name     string `json:"name,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
}

// LocalizationUploadItem represents an uploaded localization.
type LocalizationUploadItem struct {
	Locale string `json:"locale"`
	Fields int    `json:"fields"`
}

// MigrateImportResult is the result of a migrate import operation.
type MigrateImportResult struct {
	DryRun               bool                          `json:"dryRun"`
	VersionID            string                        `json:"versionId"`
	Localizations        []FastlaneLocalization        `json:"localizations"`
	AppInfoLocalizations []AppInfoFastlaneLocalization `json:"appInfoLocalizations,omitempty"`
	Uploaded             []LocalizationUploadItem      `json:"uploaded,omitempty"`
	AppInfoUploaded      []LocalizationUploadItem      `json:"appInfoUploaded,omitempty"`
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

	var localizations []FastlaneLocalization
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		locale := entry.Name()
		if locale == "review_information" || locale == "default" {
			continue // Skip special directories
		}

		localeDir := filepath.Join(metadataDir, locale)
		loc := FastlaneLocalization{Locale: locale}

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

	var localizations []AppInfoFastlaneLocalization
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		locale := entry.Name()
		if locale == "review_information" || locale == "default" {
			continue
		}

		localeDir := filepath.Join(metadataDir, locale)
		name := readFileIfExists(filepath.Join(localeDir, "name.txt"))
		subtitle := readFileIfExists(filepath.Join(localeDir, "subtitle.txt"))

		// Only include if at least one field has content
		if name != "" || subtitle != "" {
			localizations = append(localizations, AppInfoFastlaneLocalization{
				Locale:   locale,
				Name:     name,
				Subtitle: subtitle,
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
func printMigrateOutput(data interface{}, format string, pretty bool) error {
	format = strings.ToLower(format)

	if format == "json" {
		if pretty {
			return asc.PrintPrettyJSON(data)
		}
		return asc.PrintJSON(data)
	}

	switch v := data.(type) {
	case *MigrateImportResult:
		if format == "markdown" || format == "md" {
			return printMigrateImportResultMarkdown(v)
		}
		if format == "table" {
			return printMigrateImportResultTable(v)
		}
	case *MigrateExportResult:
		if format == "markdown" || format == "md" {
			return printMigrateExportResultMarkdown(v)
		}
		if format == "table" {
			return printMigrateExportResultTable(v)
		}
	case *MigrateValidateResult:
		if format == "markdown" || format == "md" {
			return printMigrateValidateResultMarkdown(v)
		}
		if format == "table" {
			return printMigrateValidateResultTable(v)
		}
	default:
		return asc.PrintJSON(data)
	}

	return fmt.Errorf("unsupported format: %s", format)
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

// App Store metadata character limits
const (
	limitDescription     = 4000
	limitKeywords        = 100
	limitWhatsNew        = 4000
	limitPromotionalText = 170
	limitName            = 30
	limitSubtitle        = 30
)

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
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

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

			return printMigrateOutput(result, *output, *pretty)
		},
	}
}

// validateVersionLocalization checks version-level metadata for issues.
func validateVersionLocalization(loc FastlaneLocalization) []ValidationIssue {
	var issues []ValidationIssue

	if len(loc.Description) > limitDescription {
		issues = append(issues, ValidationIssue{
			Locale:   loc.Locale,
			Field:    "description",
			Severity: "error",
			Message:  fmt.Sprintf("exceeds %d character limit", limitDescription),
			Length:   len(loc.Description),
			Limit:    limitDescription,
		})
	}

	if len(loc.Keywords) > limitKeywords {
		issues = append(issues, ValidationIssue{
			Locale:   loc.Locale,
			Field:    "keywords",
			Severity: "error",
			Message:  fmt.Sprintf("exceeds %d character limit", limitKeywords),
			Length:   len(loc.Keywords),
			Limit:    limitKeywords,
		})
	}

	if len(loc.WhatsNew) > limitWhatsNew {
		issues = append(issues, ValidationIssue{
			Locale:   loc.Locale,
			Field:    "whatsNew",
			Severity: "error",
			Message:  fmt.Sprintf("exceeds %d character limit", limitWhatsNew),
			Length:   len(loc.WhatsNew),
			Limit:    limitWhatsNew,
		})
	}

	if len(loc.PromotionalText) > limitPromotionalText {
		issues = append(issues, ValidationIssue{
			Locale:   loc.Locale,
			Field:    "promotionalText",
			Severity: "error",
			Message:  fmt.Sprintf("exceeds %d character limit", limitPromotionalText),
			Length:   len(loc.PromotionalText),
			Limit:    limitPromotionalText,
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

func selectBestAppInfoID(appInfos *asc.AppInfosResponse) string {
	if appInfos == nil || len(appInfos.Data) == 0 {
		return ""
	}

	// Some apps have multiple appInfos (e.g. READY_FOR_SALE plus PREPARE_FOR_SUBMISSION).
	// Updating name/subtitle is only allowed in certain states, so prefer the one that is
	// actively editable for a submission.
	const target = "PREPARE_FOR_SUBMISSION"

	var firstNonLive string
	for _, info := range appInfos.Data {
		state := strings.ToUpper(appInfoAttrString(info.Attributes, "state"))
		appStoreState := strings.ToUpper(appInfoAttrString(info.Attributes, "appStoreState"))

		if state == target || appStoreState == target {
			return info.ID
		}
		if firstNonLive == "" && isNonLiveAppInfoState(state, appStoreState) {
			firstNonLive = info.ID
		}
	}
	if firstNonLive != "" {
		return firstNonLive
	}
	return appInfos.Data[0].ID
}

func isNonLiveAppInfoState(state, appStoreState string) bool {
	isLive := func(value string) bool {
		switch value {
		case "READY_FOR_DISTRIBUTION", "READY_FOR_SALE":
			return true
		default:
			return false
		}
	}

	if state != "" && !isLive(state) {
		return true
	}
	if appStoreState != "" && !isLive(appStoreState) {
		return true
	}
	return false
}

func appInfoAttrString(attrs asc.AppInfoAttributes, key string) string {
	if attrs == nil {
		return ""
	}
	v, ok := attrs[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

// validateAppInfoLocalization checks app-level metadata for issues.
func validateAppInfoLocalization(loc AppInfoFastlaneLocalization) []ValidationIssue {
	var issues []ValidationIssue

	if len(loc.Name) > limitName {
		issues = append(issues, ValidationIssue{
			Locale:   loc.Locale,
			Field:    "name",
			Severity: "error",
			Message:  fmt.Sprintf("exceeds %d character limit", limitName),
			Length:   len(loc.Name),
			Limit:    limitName,
		})
	}

	if len(loc.Subtitle) > limitSubtitle {
		issues = append(issues, ValidationIssue{
			Locale:   loc.Locale,
			Field:    "subtitle",
			Severity: "error",
			Message:  fmt.Sprintf("exceeds %d character limit", limitSubtitle),
			Length:   len(loc.Subtitle),
			Limit:    limitSubtitle,
		})
	}

	return issues
}
