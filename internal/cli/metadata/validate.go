package metadata

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
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

const (
	issueSeverityError   = "error"
	issueSeverityWarning = "warning"
)

// ValidateIssue represents one metadata validation issue.
type ValidateIssue struct {
	Scope    string `json:"scope"`
	File     string `json:"file"`
	Locale   string `json:"locale,omitempty"`
	Version  string `json:"version,omitempty"`
	Field    string `json:"field"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Length   int    `json:"length,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// ValidateResult is the structured result for metadata validate.
type ValidateResult struct {
	Dir          string          `json:"dir"`
	FilesScanned int             `json:"filesScanned"`
	Issues       []ValidateIssue `json:"issues"`
	ErrorCount   int             `json:"errorCount"`
	WarningCount int             `json:"warningCount"`
	Valid        bool            `json:"valid"`
}

// MetadataValidateCommand returns the metadata validate subcommand.
func MetadataValidateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata validate", flag.ExitOnError)

	dir := fs.String("dir", "", "Metadata root directory (required)")
	subscriptionApp := fs.Bool("subscription-app", false, "Enable subscription-specific Terms of Use / EULA link checks")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "validate",
		ShortUsage: "asc metadata validate --dir \"./metadata\" [--subscription-app]",
		ShortHelp:  "Validate canonical metadata files offline.",
		LongHelp: `Validate canonical metadata files offline.

Checks:
  - strict JSON schema decode (unknown keys rejected)
  - required fields
  - metadata character limits
  - optional subscription-app Terms of Use / EULA description link heuristic

Examples:
  asc metadata validate --dir "./metadata"
  asc metadata validate --dir "./metadata" --subscription-app
  asc metadata validate --dir "./metadata" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("metadata validate does not accept positional arguments")
			}

			dirValue := strings.TrimSpace(*dir)
			if dirValue == "" {
				return shared.UsageError("--dir is required")
			}

			result, err := validateDir(dirValue, *subscriptionApp)
			if err != nil {
				return err
			}

			if err := shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return printValidateResultTable(result) },
				func() error { return printValidateResultMarkdown(result) },
			); err != nil {
				return err
			}

			if result.ErrorCount > 0 {
				return shared.NewReportedError(fmt.Errorf("metadata validate: found %d error(s)", result.ErrorCount))
			}
			return nil
		},
	}
}

func validateDir(dir string, subscriptionApp bool) (ValidateResult, error) {
	result := ValidateResult{
		Dir:    dir,
		Issues: make([]ValidateIssue, 0),
	}

	appInfoDir := filepath.Join(dir, appInfoDirName)
	appInfoEntries, err := os.ReadDir(appInfoDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return ValidateResult{}, fmt.Errorf("metadata validate: failed to read %s: %w", appInfoDir, err)
	}
	if err == nil {
		seenAppInfoLocales := make(map[string]string)
		for _, entry := range appInfoEntries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			locale := strings.TrimSuffix(entry.Name(), ".json")
			resolvedLocale, localeErr := validateLocale(locale)
			if localeErr != nil {
				return ValidateResult{}, shared.UsageErrorf("invalid app-info localization file %q: %v", entry.Name(), localeErr)
			}
			if err := recordCanonicalLocaleFile(seenAppInfoLocales, resolvedLocale, entry.Name()); err != nil {
				return ValidateResult{}, shared.UsageError(err.Error())
			}
			filePath := filepath.Join(appInfoDir, entry.Name())

			loc, readErr := ReadAppInfoLocalizationFile(filePath)
			if readErr != nil {
				return ValidateResult{}, shared.UsageErrorf("invalid metadata schema in %s: %v", filePath, readErr)
			}
			result.FilesScanned++

			issues := ValidateAppInfoLocalization(loc, ValidationOptions{RequireName: resolvedLocale != DefaultLocale})
			for _, issue := range issues {
				result.Issues = append(result.Issues, ValidateIssue{
					Scope:    appInfoDirName,
					File:     filePath,
					Locale:   resolvedLocale,
					Field:    issue.Field,
					Severity: issueSeverityError,
					Message:  issue.Message,
				})
			}
			result.Issues = append(result.Issues, appInfoLengthIssues(filePath, resolvedLocale, loc)...)
		}
	}

	versionDir := filepath.Join(dir, versionDirName)
	versionEntries, err := os.ReadDir(versionDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return ValidateResult{}, fmt.Errorf("metadata validate: failed to read %s: %w", versionDir, err)
	}
	if err == nil {
		for _, versionEntry := range versionEntries {
			if !versionEntry.IsDir() {
				continue
			}
			version := versionEntry.Name()
			versionPath := filepath.Join(versionDir, version)
			seenVersionLocales := make(map[string]string)

			localeEntries, localeErr := os.ReadDir(versionPath)
			if localeErr != nil {
				return ValidateResult{}, fmt.Errorf("metadata validate: failed to read %s: %w", versionPath, localeErr)
			}
			for _, localeEntry := range localeEntries {
				if localeEntry.IsDir() || filepath.Ext(localeEntry.Name()) != ".json" {
					continue
				}

				locale := strings.TrimSuffix(localeEntry.Name(), ".json")
				resolvedLocale, localeErr := validateLocale(locale)
				if localeErr != nil {
					return ValidateResult{}, shared.UsageErrorf("invalid version localization file %q: %v", localeEntry.Name(), localeErr)
				}
				if err := recordCanonicalLocaleFile(seenVersionLocales, resolvedLocale, localeEntry.Name()); err != nil {
					return ValidateResult{}, shared.UsageError(err.Error())
				}
				filePath := filepath.Join(versionPath, localeEntry.Name())

				loc, readErr := ReadVersionLocalizationFile(filePath)
				if readErr != nil {
					return ValidateResult{}, shared.UsageErrorf("invalid metadata schema in %s: %v", filePath, readErr)
				}
				result.FilesScanned++

				issues := ValidateVersionLocalization(loc)
				for _, issue := range issues {
					result.Issues = append(result.Issues, ValidateIssue{
						Scope:    versionDirName,
						File:     filePath,
						Locale:   resolvedLocale,
						Version:  version,
						Field:    issue.Field,
						Severity: issueSeverityError,
						Message:  issue.Message,
					})
				}
				result.Issues = append(result.Issues, versionLengthIssues(filePath, version, resolvedLocale, loc)...)
				if subscriptionApp {
					result.Issues = append(result.Issues, versionTermsIssues(filePath, version, resolvedLocale, loc)...)
				}
			}
		}
	}

	if result.FilesScanned == 0 {
		result.Issues = append(result.Issues, ValidateIssue{
			Scope:    "metadata",
			File:     dir,
			Field:    "metadata",
			Severity: issueSeverityError,
			Message:  "no metadata .json files found",
		})
	}

	sort.Slice(result.Issues, func(i, j int) bool {
		if result.Issues[i].File == result.Issues[j].File {
			if result.Issues[i].Field == result.Issues[j].Field {
				return result.Issues[i].Message < result.Issues[j].Message
			}
			return result.Issues[i].Field < result.Issues[j].Field
		}
		return result.Issues[i].File < result.Issues[j].File
	})

	for _, issue := range result.Issues {
		if issue.Severity == issueSeverityError {
			result.ErrorCount++
			continue
		}
		result.WarningCount++
	}
	result.Valid = result.ErrorCount == 0

	return result, nil
}

func versionLengthIssues(filePath, version, locale string, loc VersionLocalization) []ValidateIssue {
	issues := make([]ValidateIssue, 0, 4)
	for _, issue := range validation.VersionLocalizationLengthIssues(validation.VersionLocalization{
		Description:     loc.Description,
		Keywords:        loc.Keywords,
		WhatsNew:        loc.WhatsNew,
		PromotionalText: loc.PromotionalText,
	}) {
		issues = append(issues, ValidateIssue{
			Scope:    versionDirName,
			File:     filePath,
			Locale:   locale,
			Version:  version,
			Field:    issue.Field,
			Severity: issueSeverityError,
			Message:  fmt.Sprintf("%s exceeds %d characters", issue.Field, issue.Limit),
			Length:   issue.Length,
			Limit:    issue.Limit,
		})
	}
	return issues
}

func appInfoLengthIssues(filePath, locale string, loc AppInfoLocalization) []ValidateIssue {
	issues := make([]ValidateIssue, 0, 2)
	for _, issue := range validation.AppInfoLocalizationLengthIssues(validation.AppInfoLocalization{
		Name:     loc.Name,
		Subtitle: loc.Subtitle,
	}) {
		issues = append(issues, ValidateIssue{
			Scope:    appInfoDirName,
			File:     filePath,
			Locale:   locale,
			Field:    issue.Field,
			Severity: issueSeverityError,
			Message:  fmt.Sprintf("%s exceeds %d characters", issue.Field, issue.Limit),
			Length:   issue.Length,
			Limit:    issue.Limit,
		})
	}
	return issues
}

func versionTermsIssues(filePath, version, locale string, loc VersionLocalization) []ValidateIssue {
	description := strings.TrimSpace(loc.Description)
	if description == "" || validation.HasTermsOfUseLink(description) {
		return nil
	}

	return []ValidateIssue{{
		Scope:    versionDirName,
		File:     filePath,
		Locale:   locale,
		Version:  version,
		Field:    "description",
		Severity: issueSeverityWarning,
		Message:  "description is missing a Terms of Use / EULA link for subscription apps",
	}}
}

func printValidateResultTable(result ValidateResult) error {
	fmt.Printf("Dir: %s\n", result.Dir)
	fmt.Printf("Files Scanned: %d\n", result.FilesScanned)
	fmt.Printf("Errors: %d  Warnings: %d\n\n", result.ErrorCount, result.WarningCount)

	rows := make([][]string, 0, len(result.Issues))
	for _, issue := range result.Issues {
		length := "-"
		limit := "-"
		if issue.Length > 0 {
			length = fmt.Sprintf("%d", issue.Length)
		}
		if issue.Limit > 0 {
			limit = fmt.Sprintf("%d", issue.Limit)
		}
		rows = append(rows, []string{
			issue.Scope,
			issue.File,
			issue.Locale,
			issue.Version,
			issue.Field,
			issue.Severity,
			issue.Message,
			length,
			limit,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"metadata", result.Dir, "", "", "", "info", "no issues", "-", "-"})
	}
	asc.RenderTable(
		[]string{"scope", "file", "locale", "version", "field", "severity", "message", "length", "limit"},
		rows,
	)
	return nil
}

func printValidateResultMarkdown(result ValidateResult) error {
	fmt.Printf("**Dir:** %s\n\n", result.Dir)
	fmt.Printf("**Files Scanned:** %d\n\n", result.FilesScanned)
	fmt.Printf("**Errors:** %d\n\n", result.ErrorCount)
	fmt.Printf("**Warnings:** %d\n\n", result.WarningCount)

	rows := make([][]string, 0, len(result.Issues))
	for _, issue := range result.Issues {
		length := "-"
		limit := "-"
		if issue.Length > 0 {
			length = fmt.Sprintf("%d", issue.Length)
		}
		if issue.Limit > 0 {
			limit = fmt.Sprintf("%d", issue.Limit)
		}
		rows = append(rows, []string{
			issue.Scope,
			issue.File,
			issue.Locale,
			issue.Version,
			issue.Field,
			issue.Severity,
			issue.Message,
			length,
			limit,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"metadata", result.Dir, "", "", "", "info", "no issues", "-", "-"})
	}
	asc.RenderMarkdown(
		[]string{"scope", "file", "locale", "version", "field", "severity", "message", "length", "limit"},
		rows,
	)
	return nil
}
