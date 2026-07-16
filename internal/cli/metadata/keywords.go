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

// MetadataKeywordFileResult describes one local keyword file change.
type MetadataKeywordFileResult struct {
	Locale            string   `json:"locale"`
	File              string   `json:"file"`
	Action            string   `json:"action"`
	Reason            string   `json:"reason,omitempty"`
	KeywordField      string   `json:"keywordField,omitempty"`
	KeywordCount      int      `json:"keywordCount,omitempty"`
	DuplicateCount    int      `json:"duplicateCount,omitempty"`
	SkippedDuplicates []string `json:"skippedDuplicates,omitempty"`
}

// MetadataKeywordIssue describes one preview-time validation issue.
type MetadataKeywordIssue struct {
	Locale       string `json:"locale,omitempty"`
	File         string `json:"file,omitempty"`
	Severity     string `json:"severity"`
	Message      string `json:"message"`
	KeywordField string `json:"keywordField,omitempty"`
	Length       int    `json:"length,omitempty"`
	Limit        int    `json:"limit,omitempty"`
}

type keywordLocalState struct {
	locale string
	file   string
	full   VersionLocalization
	patch  versionLocalPatch
}

type metadataKeywordFieldDetails struct {
	field      string
	count      int
	duplicates []string
}

// MetadataKeywordsCommand returns the canonical metadata keywords subtree.
func MetadataKeywordsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("keywords", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "keywords",
		ShortUsage: "asc metadata keywords <subcommand> [flags]",
		ShortHelp:  "Manage canonical version-localization keyword metadata.",
		LongHelp: `Manage canonical version-localization keyword metadata.

This workflow manages the canonical version-localization ` + "`keywords`" + ` field.

Most commands operate on local ` + "`./metadata/version/<version>/<locale>.json`" + `
files. The direct-remote ` + "`push`" + ` helper writes locale-keyed keyword input
straight to App Store Connect by App Store version ID.

It does not front the raw App Store Connect ` + "`searchKeywords`" + `
relationship APIs. Those low-level surfaces remain available under:
  - ` + "`asc apps search-keywords ...`" + `
  - ` + "`asc localizations search-keywords ...`" + `

Examples:
  asc metadata keywords import --dir "./metadata" --version "1.2.3" --locale "en-US" --input "./keywords.csv"
  asc metadata keywords audit --app "APP_ID" --version "1.2.3"
  asc metadata keywords plan --app "APP_ID" --version "1.2.3" --dir "./metadata"
  asc metadata keywords localize --dir "./metadata" --version "1.2.3" --from-locale "en-US" --to-locales "fr-FR,de-DE"
  asc metadata keywords apply --app "APP_ID" --version "1.2.3" --dir "./metadata" --confirm
  asc metadata keywords push --version-id "VERSION_ID" --input "./keywords.json"
  asc metadata keywords sync --app "APP_ID" --version "1.2.3" --dir "./metadata" --input "./keywords.json" --format json --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			MetadataKeywordsImportCommand(),
			MetadataKeywordsAuditCommand(),
			MetadataKeywordsPlanCommand(),
			MetadataKeywordsDiffCommand(),
			MetadataKeywordsLocalizeCommand(),
			MetadataKeywordsApplyCommand(),
			MetadataKeywordsPushCommand(),
			MetadataKeywordsSyncCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func validateMetadataKeywordDirVersion(dir string, version string) (string, string, error) {
	dirValue := strings.TrimSpace(dir)
	if dirValue == "" {
		return "", "", shared.UsageError("--dir is required")
	}
	versionValue := strings.TrimSpace(version)
	if versionValue == "" {
		return "", "", shared.UsageError("--version is required")
	}
	return dirValue, versionValue, nil
}

func validateMetadataKeywordLocale(locale string) (string, error) {
	resolvedLocale, err := validateLocale(locale)
	if err != nil {
		return "", err
	}
	if resolvedLocale == DefaultLocale {
		return "", fmt.Errorf("default locale is not supported for metadata keywords")
	}
	return resolvedLocale, nil
}

func buildMetadataKeywordWriteResults(dir, version string, valuesByLocale map[string][]string, overwrite bool) (map[string]keywordLocalState, []MetadataKeywordFileResult, []WritePlan, []MetadataKeywordIssue, error) {
	locales := make([]string, 0, len(valuesByLocale))
	for locale := range valuesByLocale {
		locales = append(locales, locale)
	}
	sort.Strings(locales)

	states := make(map[string]keywordLocalState, len(locales))
	results := make([]MetadataKeywordFileResult, 0, len(locales))
	plans := make([]WritePlan, 0, len(locales))
	issues := make([]MetadataKeywordIssue, 0)

	for _, locale := range locales {
		path, err := resolveExistingVersionLocalizationPath(dir, version, locale)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		keywordField, keywordCount, duplicateCount, skippedDuplicates, keywordIssues, err := buildMetadataKeywordPreview(valuesByLocale[locale])
		if err != nil {
			return nil, nil, nil, nil, shared.UsageErrorf("locale %q: %v", locale, err)
		}

		existing, exists, err := readExistingVersionLocalization(path)
		if err != nil {
			return nil, nil, nil, nil, err
		}

		result := MetadataKeywordFileResult{
			Locale:            locale,
			File:              path,
			KeywordField:      keywordField,
			KeywordCount:      keywordCount,
			DuplicateCount:    duplicateCount,
			SkippedDuplicates: skippedDuplicates,
		}
		for _, issue := range keywordIssues {
			issue.Locale = locale
			issue.File = path
			if issue.KeywordField == "" {
				issue.KeywordField = keywordField
			}
			issues = append(issues, issue)
		}
		if len(keywordIssues) > 0 {
			result.Action = "invalid"
			result.Reason = keywordIssues[0].Message
			results = append(results, result)
			continue
		}

		switch {
		case exists && strings.TrimSpace(existing.Keywords) == keywordField:
			result.Action = "noop"
			result.Reason = "keywords already match canonical file"
			states[locale] = buildMetadataKeywordLocalState(locale, path, existing)
		case exists && strings.TrimSpace(existing.Keywords) != "" && !overwrite:
			result.Action = "skip"
			result.Reason = "existing keywords preserved (use --overwrite to replace)"
			states[locale] = buildMetadataKeywordLocalState(locale, path, existing)
		default:
			next := existing
			next.Keywords = keywordField
			result.Action = "update"
			if !exists {
				result.Action = "create"
			}
			result.Reason = "canonical keyword field updated"
			states[locale] = buildMetadataKeywordLocalState(locale, path, next)
			data, err := EncodeVersionLocalization(next)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			plans = append(plans, WritePlan{Path: path, Contents: data})
		}
		results = append(results, result)
	}
	return states, results, plans, issues, nil
}

func buildMetadataKeywordLocalState(locale, path string, localization VersionLocalization) keywordLocalState {
	normalized := NormalizeVersionLocalization(localization)
	return keywordLocalState{
		locale: locale,
		file:   path,
		full:   normalized,
		patch: versionLocalPatch{
			localization:       NormalizeVersionLocalization(VersionLocalization{Keywords: normalized.Keywords}),
			createLocalization: normalized,
			setFields:          map[string]string{"keywords": normalized.Keywords},
		},
	}
}

func readExistingVersionLocalization(path string) (VersionLocalization, bool, error) {
	localization, err := ReadVersionLocalizationFile(path)
	if err == nil {
		return localization, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return VersionLocalization{}, false, nil
	}
	return VersionLocalization{}, false, err
}

func resolveExistingVersionLocalizationPath(dir string, version string, canonicalLocale string) (string, error) {
	canonicalPath, err := VersionLocalizationFilePath(dir, version, canonicalLocale)
	if err != nil {
		return "", err
	}

	resolvedVersion, err := validatePathSegment("version", version)
	if err != nil {
		return "", err
	}
	versionPath := filepath.Join(strings.TrimSpace(dir), versionDirName, resolvedVersion)
	entries, err := os.ReadDir(versionPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return canonicalPath, nil
		}
		return "", err
	}

	seenLocales := make(map[string]string)
	existingPath := ""
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		rawLocale := strings.TrimSuffix(entry.Name(), ".json")
		if strings.EqualFold(rawLocale, DefaultLocale) {
			continue
		}
		resolvedLocale, localeErr := validateMetadataKeywordLocale(rawLocale)
		if localeErr != nil {
			return "", shared.UsageErrorf("invalid metadata keywords file %q: %v", entry.Name(), localeErr)
		}
		if err := recordCanonicalLocaleFile(seenLocales, resolvedLocale, entry.Name()); err != nil {
			return "", shared.UsageError(err.Error())
		}
		if resolvedLocale == canonicalLocale {
			existingPath = filepath.Join(versionPath, entry.Name())
		}
	}
	if existingPath != "" {
		return existingPath, nil
	}
	return canonicalPath, nil
}

func splitMetadataKeywordTokens(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '，' || r == '、' || r == ';' || r == '；' || r == '\n' || r == '\r'
	})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.Join(strings.Fields(strings.TrimSpace(part)), " ")
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func normalizeMetadataKeywordTokensPreserveDuplicates(keywords []string) ([]string, error) {
	normalized := make([]string, 0, len(keywords))
	for _, keyword := range keywords {
		trimmed := strings.Join(strings.Fields(strings.TrimSpace(keyword)), " ")
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil, fmt.Errorf("no non-empty keywords were found")
	}
	return normalized, nil
}

func normalizeMetadataKeywordListDetailed(keywords []string) ([]string, []string, error) {
	normalizedInputs, err := normalizeMetadataKeywordTokensPreserveDuplicates(keywords)
	if err != nil {
		return nil, nil, err
	}
	normalized := make([]string, 0, len(normalizedInputs))
	duplicates := make([]string, 0)
	seen := make(map[string]struct{}, len(normalizedInputs))
	for _, trimmed := range normalizedInputs {
		folded := strings.ToLower(trimmed)
		if _, ok := seen[folded]; ok {
			duplicates = append(duplicates, trimmed)
			continue
		}
		seen[folded] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized, duplicates, nil
}

func buildMetadataKeywordField(keywords []string) (string, int, error) {
	details, err := buildMetadataKeywordFieldDetails(keywords)
	if err != nil {
		return "", 0, err
	}
	if err := validation.ValidateKeywordField(details.field); err != nil {
		return "", 0, err
	}
	return details.field, details.count, nil
}

func buildMetadataKeywordFieldDetails(keywords []string) (metadataKeywordFieldDetails, error) {
	normalized, duplicates, err := normalizeMetadataKeywordListDetailed(keywords)
	if err != nil {
		return metadataKeywordFieldDetails{}, err
	}
	field := strings.Join(normalized, ",")
	return metadataKeywordFieldDetails{
		field:      field,
		count:      len(normalized),
		duplicates: duplicates,
	}, nil
}

func buildMetadataKeywordPreview(keywords []string) (string, int, int, []string, []MetadataKeywordIssue, error) {
	details, err := buildMetadataKeywordFieldDetails(keywords)
	if err != nil {
		return "", 0, 0, nil, nil, err
	}
	issues := make([]MetadataKeywordIssue, 0, 1)
	if issue := validation.KeywordFieldLengthIssue(details.field); issue != nil {
		issues = append(issues, MetadataKeywordIssue{
			Severity:     "error",
			Message:      fmt.Sprintf("keywords exceed %d %s", issue.Limit, issue.Unit),
			KeywordField: details.field,
			Length:       issue.Length,
			Limit:        issue.Limit,
		})
	}
	return details.field, details.count, len(details.duplicates), details.duplicates, issues, nil
}

func parseMetadataKeywordField(field string) ([]string, error) {
	normalized, _, err := normalizeMetadataKeywordListDetailed(splitMetadataKeywordTokens(field))
	if err != nil {
		return nil, err
	}
	joined := strings.Join(normalized, ",")
	if err := validation.ValidateKeywordField(joined); err != nil {
		return nil, err
	}
	return normalized, nil
}

func printMetadataKeywordFileResultTable(title string, results []MetadataKeywordFileResult, detectedLocales []string, issues []MetadataKeywordIssue, dir string, version string, dryRun bool, sideDataCount int, sideDataReportPath string) error {
	fmt.Printf("%s\n", title)
	fmt.Printf("Dir: %s\n", dir)
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("Dry Run: %t\n\n", dryRun)
	if len(detectedLocales) > 0 {
		fmt.Printf("Detected Locales: %s\n\n", strings.Join(detectedLocales, ","))
	}
	if sideDataCount > 0 {
		fmt.Printf("Side Data Records: %d\n", sideDataCount)
		fmt.Printf("Side Data Report: %s\n\n", sideDataReportPath)
	}

	rows := make([][]string, 0, len(results))
	for _, result := range results {
		rows = append(rows, []string{
			result.Locale,
			result.Action,
			result.Reason,
			fmt.Sprintf("%d", result.KeywordCount),
			fmt.Sprintf("%d", result.DuplicateCount),
			strings.Join(result.SkippedDuplicates, ","),
			sanitizePlanCell(result.KeywordField),
			result.File,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"", "none", "no changes", "0", "0", "", "", ""})
	}
	asc.RenderTable([]string{"locale", "action", "reason", "count", "duplicates", "skipped", "keywords", "file"}, rows)
	if len(issues) > 0 {
		fmt.Println()
		asc.RenderTable([]string{"locale", "severity", "message", "keywords", "length", "limit"}, buildMetadataKeywordIssueRows(issues))
	}
	return nil
}

func printMetadataKeywordFileResultMarkdown(title string, results []MetadataKeywordFileResult, detectedLocales []string, issues []MetadataKeywordIssue, dir string, version string, dryRun bool, sideDataCount int, sideDataReportPath string) error {
	fmt.Printf("## %s\n\n", title)
	fmt.Printf("**Dir:** %s\n\n", dir)
	fmt.Printf("**Version:** %s\n\n", version)
	fmt.Printf("**Dry Run:** %t\n\n", dryRun)
	if len(detectedLocales) > 0 {
		fmt.Printf("**Detected Locales:** %s\n\n", strings.Join(detectedLocales, ","))
	}
	if sideDataCount > 0 {
		fmt.Printf("**Side Data Records:** %d\n\n", sideDataCount)
		fmt.Printf("**Side Data Report:** %s\n\n", sideDataReportPath)
	}

	rows := make([][]string, 0, len(results))
	for _, result := range results {
		rows = append(rows, []string{
			result.Locale,
			result.Action,
			result.Reason,
			fmt.Sprintf("%d", result.KeywordCount),
			fmt.Sprintf("%d", result.DuplicateCount),
			strings.Join(result.SkippedDuplicates, ","),
			sanitizePlanCell(result.KeywordField),
			result.File,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"", "none", "no changes", "0", "0", "", "", ""})
	}
	asc.RenderMarkdown([]string{"locale", "action", "reason", "count", "duplicates", "skipped", "keywords", "file"}, rows)
	if len(issues) > 0 {
		fmt.Println()
		asc.RenderMarkdown([]string{"locale", "severity", "message", "keywords", "length", "limit"}, buildMetadataKeywordIssueRows(issues))
	}
	return nil
}

func buildMetadataKeywordIssueRows(issues []MetadataKeywordIssue) [][]string {
	rows := make([][]string, 0, len(issues))
	for _, issue := range issues {
		length := "-"
		limit := "-"
		if issue.Length > 0 {
			length = fmt.Sprintf("%d", issue.Length)
		}
		if issue.Limit > 0 {
			limit = fmt.Sprintf("%d", issue.Limit)
		}
		rows = append(rows, []string{
			issue.Locale,
			issue.Severity,
			issue.Message,
			sanitizePlanCell(issue.KeywordField),
			length,
			limit,
		})
	}
	return rows
}
