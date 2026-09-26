package metadata

import (
	"context"
	"encoding/json"
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
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/metadataurl"
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
	checkURLs := fs.Bool("check-urls", false, "Fetch support and privacy policy URLs to detect redirects and root pages")
	subscriptionApp := fs.Bool("subscription-app", false, "Enable subscription-specific Terms of Use / EULA link checks")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "validate",
		ShortUsage: "asc metadata validate --dir \"./metadata\" [--check-urls] [--subscription-app]",
		ShortHelp:  "Validate canonical metadata files (offline by default).",
		LongHelp: `Validate canonical metadata files. Validation is offline by default;
--check-urls opts into bounded HTTP checks.

Checks:
  - strict JSON schema decode (unknown keys rejected)
  - required fields
  - metadata character limits
  - URL syntax and length for marketing, support, privacy policy, and privacy choices URLs
  - optional redirect, final-host, status, and site-root checks for support and privacy policy URLs
  - implausibly short app name and description values
  - optional subscription-app Terms of Use / EULA description link heuristic

Examples:
  asc metadata validate --dir "./metadata"
  asc metadata validate --dir "./metadata" --check-urls
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
				return metadataRequiredInputError("--dir", "--dir is required")
			}

			result, err := validateDirWithOptions(ctx, dirValue, validateDirOptions{
				checkURLs:       *checkURLs,
				subscriptionApp: *subscriptionApp,
			})
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
				return shared.NewValidationReportedError(fmt.Errorf("metadata validate: found %d error(s)", result.ErrorCount))
			}
			return nil
		},
	}
}

func validateDir(dir string) (ValidateResult, error) {
	return validateDirWithOptions(context.Background(), dir, validateDirOptions{})
}

type validateDirOptions struct {
	checkURLs       bool
	subscriptionApp bool
	urlChecker      metadataurl.Checker
}

func validateDirWithOptions(ctx context.Context, dir string, options validateDirOptions) (ValidateResult, error) {
	result := ValidateResult{
		Dir:    dir,
		Issues: make([]ValidateIssue, 0),
	}
	urlTargets := make([]metadataURLTarget, 0)

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

			data, readErr := readFileNoFollow(filePath)
			if readErr != nil {
				return ValidateResult{}, shared.UsageErrorf("invalid metadata schema in %s: %v", filePath, readErr)
			}
			fieldIntentIssues, hasClear, clearOnly, fieldIntentErr := metadataFieldIntentIssues(data, appInfoPlanFields, resolvedLocale != DefaultLocale)
			if fieldIntentErr != nil {
				return ValidateResult{}, shared.UsageErrorf("invalid metadata schema in %s: %v", filePath, fieldIntentErr)
			}
			loc, readErr := DecodeAppInfoLocalization(data)
			if readErr != nil {
				return ValidateResult{}, shared.UsageErrorf("invalid metadata schema in %s: %v", filePath, readErr)
			}
			result.FilesScanned++
			result.Issues = append(result.Issues, metadataIntentValidateIssues(appInfoDirName, filePath, resolvedLocale, "", fieldIntentIssues)...)

			issues := ValidateAppInfoLocalization(loc, ValidationOptions{
				RequireName: resolvedLocale != DefaultLocale && !hasClear,
				AllowEmpty:  clearOnly,
			})
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
			result.Issues = append(result.Issues, appInfoURLIssues(filePath, resolvedLocale, loc)...)
			result.Issues = append(result.Issues, appInfoMinimumLengthIssues(filePath, resolvedLocale, loc)...)
			if options.checkURLs {
				urlTargets = append(urlTargets, metadataURLTargets(appInfoDirName, filePath, resolvedLocale, "", []metadataURLField{
					{field: "privacyPolicyUrl", label: "privacy policy URL", value: loc.PrivacyPolicyURL},
				})...)
			}
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

				data, readErr := readFileNoFollow(filePath)
				if readErr != nil {
					return ValidateResult{}, shared.UsageErrorf("invalid metadata schema in %s: %v", filePath, readErr)
				}
				fieldIntentIssues, _, clearOnly, fieldIntentErr := metadataFieldIntentIssues(data, versionPlanFields, resolvedLocale != DefaultLocale)
				if fieldIntentErr != nil {
					return ValidateResult{}, shared.UsageErrorf("invalid metadata schema in %s: %v", filePath, fieldIntentErr)
				}
				loc, readErr := DecodeVersionLocalization(data)
				if readErr != nil {
					return ValidateResult{}, shared.UsageErrorf("invalid metadata schema in %s: %v", filePath, readErr)
				}
				result.FilesScanned++
				result.Issues = append(result.Issues, metadataIntentValidateIssues(versionDirName, filePath, resolvedLocale, version, fieldIntentIssues)...)

				var issues []ValidationIssue
				if !clearOnly {
					issues = ValidateVersionLocalization(loc)
				}
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
				result.Issues = append(result.Issues, versionURLIssues(filePath, version, resolvedLocale, loc)...)
				result.Issues = append(result.Issues, versionMinimumLengthIssues(filePath, version, resolvedLocale, loc)...)
				if options.subscriptionApp {
					result.Issues = append(result.Issues, versionTermsIssues(filePath, version, resolvedLocale, loc)...)
				}
				if options.checkURLs {
					urlTargets = append(urlTargets, metadataURLTargets(versionDirName, filePath, resolvedLocale, version, []metadataURLField{
						{field: "supportUrl", label: "support URL", value: loc.SupportURL},
					})...)
				}
			}
		}
	}

	if options.checkURLs && len(urlTargets) > 0 {
		checker := options.urlChecker
		if checker == nil {
			checker = newMetadataURLChecker()
		}
		urlIssues, urlErr := metadataURLCheckIssues(ctx, checker, urlTargets)
		if urlErr != nil {
			return ValidateResult{}, urlErr
		}
		result.Issues = append(result.Issues, urlIssues...)
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

type metadataFieldIntentIssue struct {
	Field   string
	Message string
}

func metadataFieldIntentIssues(data []byte, allowed []string, allowClear bool) ([]metadataFieldIntentIssue, bool, bool, error) {
	var raw map[string]json.RawMessage
	if err := decodeStrictJSON(data, &raw); err != nil {
		return nil, false, false, err
	}

	hasSetContent := false
	hasClear := false
	hasExplicitIntent := false
	issues := make([]metadataFieldIntentIssue, 0)
	for _, key := range sortedKeys(raw) {
		rawValue := raw[key]
		canonicalKey, err := canonicalStringFieldPatchKey(key, allowed)
		if err != nil {
			return nil, false, false, err
		}
		if isJSONNull(rawValue) {
			hasExplicitIntent = true
			if _, clearable := clearableMetadataFields[canonicalKey]; !clearable {
				issues = append(issues, metadataFieldIntentIssue{
					Field:   canonicalKey,
					Message: fmt.Sprintf("field %q cannot be null; omit the key to leave the remote value unchanged", canonicalKey),
				})
				continue
			}
			if !allowClear {
				issues = append(issues, metadataFieldIntentIssue{
					Field:   canonicalKey,
					Message: fmt.Sprintf("field %q cannot be cleared in default.json; move the null value to an explicit locale file", canonicalKey),
				})
				continue
			}
			hasClear = true
			continue
		}
		var value string
		if err := json.Unmarshal(rawValue, &value); err != nil {
			return nil, false, false, err
		}
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			hasExplicitIntent = true
		}
		if trimmed == "__ASC_DELETE__" {
			issues = append(issues, metadataFieldIntentIssue{
				Field:   canonicalKey,
				Message: fmt.Sprintf("field %q uses unsupported clear token __ASC_DELETE__; omit the key to keep the remote value", canonicalKey),
			})
			continue
		}
		if trimmed == "" {
			issues = append(issues, metadataFieldIntentIssue{
				Field:   canonicalKey,
				Message: fmt.Sprintf("field %q cannot be empty; omit the key to leave the remote value unchanged", canonicalKey),
			})
			continue
		}
		hasSetContent = true
	}
	if !hasExplicitIntent {
		return nil, false, false, nil
	}
	return issues, hasClear, hasClear && !hasSetContent, nil
}

func metadataIntentValidateIssues(scope, filePath, locale, version string, issues []metadataFieldIntentIssue) []ValidateIssue {
	result := make([]ValidateIssue, 0, len(issues))
	for _, issue := range issues {
		result = append(result, ValidateIssue{
			Scope:    scope,
			File:     filePath,
			Locale:   locale,
			Version:  version,
			Field:    issue.Field,
			Severity: issueSeverityError,
			Message:  issue.Message,
		})
	}
	return result
}

func versionLengthIssues(filePath, version, locale string, loc VersionLocalization) []ValidateIssue {
	issues := make([]ValidateIssue, 0, 6)
	for _, issue := range validation.VersionLocalizationLengthIssues(validation.VersionLocalization{
		Description:     loc.Description,
		Keywords:        loc.Keywords,
		WhatsNew:        loc.WhatsNew,
		PromotionalText: loc.PromotionalText,
		MarketingURL:    loc.MarketingURL,
		SupportURL:      loc.SupportURL,
	}) {
		issues = append(issues, metadataLengthValidateIssue(versionDirName, filePath, locale, version, issue))
	}
	return issues
}

func appInfoLengthIssues(filePath, locale string, loc AppInfoLocalization) []ValidateIssue {
	issues := make([]ValidateIssue, 0, 4)
	for _, issue := range validation.AppInfoLocalizationLengthIssues(validation.AppInfoLocalization{
		Name:              loc.Name,
		Subtitle:          loc.Subtitle,
		PrivacyPolicyURL:  loc.PrivacyPolicyURL,
		PrivacyChoicesURL: loc.PrivacyChoicesURL,
	}) {
		issues = append(issues, metadataLengthValidateIssue(appInfoDirName, filePath, locale, "", issue))
	}
	return issues
}

// versionMinimumLengthIssues reports version localization values that are too
// short to be real content, such as a placeholder description.
func versionMinimumLengthIssues(filePath, version, locale string, loc VersionLocalization) []ValidateIssue {
	issues := make([]ValidateIssue, 0, 1)
	for _, issue := range validation.VersionLocalizationMinimumLengthIssues(validation.VersionLocalization{
		Description: loc.Description,
	}) {
		issues = append(issues, metadataMinimumLengthValidateIssue(versionDirName, filePath, locale, version, issue))
	}
	return issues
}

// appInfoMinimumLengthIssues reports app-info localization values that are too
// short to be real content, such as a single-character app name.
func appInfoMinimumLengthIssues(filePath, locale string, loc AppInfoLocalization) []ValidateIssue {
	issues := make([]ValidateIssue, 0, 1)
	for _, issue := range validation.AppInfoLocalizationMinimumLengthIssues(validation.AppInfoLocalization{
		Name: loc.Name,
	}) {
		issues = append(issues, metadataMinimumLengthValidateIssue(appInfoDirName, filePath, locale, "", issue))
	}
	return issues
}

func metadataMinimumLengthValidateIssue(scope, filePath, locale, version string, issue validation.MetadataMinimumLengthIssue) ValidateIssue {
	return ValidateIssue{
		Scope:   scope,
		File:    filePath,
		Locale:  locale,
		Version: version,
		Field:   issue.Field,
		// Apple publishes no exact minimum, so short values are advisory.
		Severity: issueSeverityWarning,
		Message: fmt.Sprintf(
			"%s is shorter than %d characters",
			validation.MetadataFieldLabel(issue.Field),
			issue.Minimum,
		),
		Length: issue.Length,
	}
}

func metadataLengthValidateIssue(scope, filePath, locale, version string, issue validation.MetadataLengthIssue) ValidateIssue {
	verb := "exceeds"
	if issue.Field == "keywords" {
		verb = "exceed"
	}
	severity := issueSeverityError
	if validation.MetadataLengthSeverity(issue.Field) == validation.SeverityWarning {
		severity = issueSeverityWarning
	}

	return ValidateIssue{
		Scope:    scope,
		File:     filePath,
		Locale:   locale,
		Version:  version,
		Field:    issue.Field,
		Severity: severity,
		Message:  fmt.Sprintf("%s %s %d %s", validation.MetadataFieldLabel(issue.Field), verb, issue.Limit, issue.Unit),
		Length:   issue.Length,
		Limit:    issue.Limit,
	}
}

type metadataURLField struct {
	field string
	label string
	value string
}

// versionURLIssues reports version localization URL fields App Store Connect
// rejects at push time because they are not absolute HTTP/HTTPS URLs.
func versionURLIssues(filePath, version, locale string, loc VersionLocalization) []ValidateIssue {
	return metadataURLIssues(versionDirName, filePath, locale, version, []metadataURLField{
		{field: "marketingUrl", label: "marketing URL", value: loc.MarketingURL},
		{field: "supportUrl", label: "support URL", value: loc.SupportURL},
	})
}

// appInfoURLIssues reports app-info localization URL fields App Store Connect
// rejects at push time because they are not absolute HTTP/HTTPS URLs.
func appInfoURLIssues(filePath, locale string, loc AppInfoLocalization) []ValidateIssue {
	return metadataURLIssues(appInfoDirName, filePath, locale, "", []metadataURLField{
		{field: "privacyChoicesUrl", label: "privacy choices URL", value: loc.PrivacyChoicesURL},
		{field: "privacyPolicyUrl", label: "privacy policy URL", value: loc.PrivacyPolicyURL},
	})
}

func metadataURLIssues(scope, filePath, locale, version string, fields []metadataURLField) []ValidateIssue {
	issues := make([]ValidateIssue, 0, len(fields))
	for _, field := range fields {
		value := strings.TrimSpace(field.value)
		if value == "" || validation.IsValidHTTPURL(value) {
			continue
		}
		issues = append(issues, ValidateIssue{
			Scope:    scope,
			File:     filePath,
			Locale:   locale,
			Version:  version,
			Field:    field.field,
			Severity: issueSeverityWarning,
			Message:  fmt.Sprintf("%s is not a valid HTTP/HTTPS URL", field.label),
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
