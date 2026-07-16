package metadata

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	keywordImportFormatAuto     = "auto"
	keywordImportFormatCSV      = "csv"
	keywordImportFormatJSON     = "json"
	keywordImportFormatText     = "text"
	keywordImportFormatAstroCSV = "astro-csv"
)

type metadataKeywordImportParser func([]byte, string) (metadataKeywordImportedData, error)

var metadataKeywordImportFormats = map[string]metadataKeywordImportParser{
	keywordImportFormatCSV:      parseMetadataKeywordCSV,
	keywordImportFormatJSON:     parseMetadataKeywordJSON,
	keywordImportFormatText:     parseMetadataKeywordText,
	keywordImportFormatAstroCSV: parseMetadataKeywordAstroCSV,
}

// MetadataKeywordsImportResult describes one import run.
type MetadataKeywordsImportResult struct {
	Dir                 string                      `json:"dir"`
	Version             string                      `json:"version"`
	Input               string                      `json:"input"`
	Format              string                      `json:"format"`
	DryRun              bool                        `json:"dryRun"`
	Valid               bool                        `json:"valid"`
	DetectedLocales     []string                    `json:"detectedLocales"`
	Results             []MetadataKeywordFileResult `json:"results"`
	Issues              []MetadataKeywordIssue      `json:"issues,omitempty"`
	SideDataRecordCount int                         `json:"sideDataRecordCount,omitempty"`
	SideDataReportPath  string                      `json:"sideDataReportPath,omitempty"`
}

type metadataKeywordsImportOptions struct {
	Dir                string
	Version            string
	Input              string
	Format             string
	DefaultLocale      string
	DryRun             bool
	Overwrite          bool
	SideDataReportFile string
}

type keywordImportPayload struct {
	states map[string]keywordLocalState
	result MetadataKeywordsImportResult
}

type metadataKeywordImportedData struct {
	locales  map[string][]string
	sideData []MetadataKeywordSideDataRecord
}

// MetadataKeywordSideDataRecord captures non-publishable research fields from imports.
type MetadataKeywordSideDataRecord struct {
	Locale   string         `json:"locale,omitempty"`
	Keywords []string       `json:"keywords,omitempty"`
	Fields   map[string]any `json:"fields"`
}

// MetadataKeywordSideDataArtifact is the persisted side-data report format.
type MetadataKeywordSideDataArtifact struct {
	Dir     string                          `json:"dir"`
	Version string                          `json:"version"`
	Input   string                          `json:"input"`
	Format  string                          `json:"format"`
	Records []MetadataKeywordSideDataRecord `json:"records"`
}

// MetadataKeywordsImportCommand returns the keywords import subcommand.
func MetadataKeywordsImportCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata keywords import", flag.ExitOnError)

	dir := fs.String("dir", "", "Metadata root directory (required)")
	version := fs.String("version", "", "App version string (for example 1.2.3)")
	input := fs.String("input", "", "Import file path or - for stdin (required)")
	format := fs.String("format", keywordImportFormatAuto, "Input format: auto, csv, json, text, or astro-csv")
	locale := fs.String("locale", "", "Default locale for inputs without a locale column/field")
	sideDataReportFile := fs.String("side-data-report-file", "", "Optional path to write side-data report JSON when research fields are present")
	dryRun := fs.Bool("dry-run", false, "Preview local file changes without writing files")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "import",
		ShortUsage: "asc metadata keywords import --dir \"./metadata\" --version \"1.2.3\" --input \"./keywords.csv\" [flags]",
		ShortHelp:  "Import provider keyword exports into canonical metadata files.",
		LongHelp: `Import provider keyword exports into canonical metadata files.

Supported input formats:
  - csv: header-based rows with locale + keywords/keyword/term columns
  - json: locale-keyed maps, arrays of localization objects, or a single localization object
  - text: a plain comma/newline-separated keyword list (requires --locale)
  - astro-csv: Astro keyword export CSV using the documented Keyword column (requires --locale unless locale data is present)

Examples:
  asc metadata keywords import --dir "./metadata" --version "1.2.3" --locale "en-US" --input "./keywords.csv"
  asc metadata keywords import --dir "./metadata" --version "1.2.3" --format json --input "./keywords.json"
  asc metadata keywords import --dir "./metadata" --version "1.2.3" --format text --locale "fr-FR" --input "./keywords.txt" --dry-run`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("metadata keywords import does not accept positional arguments")
			}
			result, err := executeMetadataKeywordsImport(metadataKeywordsImportOptions{
				Dir:                *dir,
				Version:            *version,
				Input:              *input,
				Format:             *format,
				DefaultLocale:      *locale,
				DryRun:             *dryRun,
				Overwrite:          true,
				SideDataReportFile: *sideDataReportFile,
			})
			if err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return err
				}
				return fmt.Errorf("metadata keywords import: %w", err)
			}
			if err := shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error {
					return printMetadataKeywordFileResultTable("Keyword Import", result.Results, result.DetectedLocales, result.Issues, result.Dir, result.Version, result.DryRun, result.SideDataRecordCount, result.SideDataReportPath)
				},
				func() error {
					return printMetadataKeywordFileResultMarkdown("Keyword Import", result.Results, result.DetectedLocales, result.Issues, result.Dir, result.Version, result.DryRun, result.SideDataRecordCount, result.SideDataReportPath)
				},
			); err != nil {
				return err
			}
			if !result.Valid {
				return shared.NewReportedError(fmt.Errorf("metadata keywords import: found %d issue(s)", len(result.Issues)))
			}
			return nil
		},
	}
}

func executeMetadataKeywordsImport(opts metadataKeywordsImportOptions) (MetadataKeywordsImportResult, error) {
	payload, err := executeMetadataKeywordsImportWithState(opts)
	if err != nil {
		return MetadataKeywordsImportResult{}, err
	}
	return payload.result, nil
}

func executeMetadataKeywordsImportWithState(opts metadataKeywordsImportOptions) (keywordImportPayload, error) {
	dirValue, versionValue, err := validateMetadataKeywordDirVersion(opts.Dir, opts.Version)
	if err != nil {
		return keywordImportPayload{}, err
	}
	inputValue := strings.TrimSpace(opts.Input)
	if inputValue == "" {
		return keywordImportPayload{}, shared.UsageError("--input is required")
	}
	formatValue, err := resolveMetadataKeywordImportFormat(inputValue, opts.Format)
	if err != nil {
		return keywordImportPayload{}, shared.UsageError(err.Error())
	}

	imported, err := readMetadataKeywordImportInput(inputValue, formatValue, strings.TrimSpace(opts.DefaultLocale))
	if err != nil {
		return keywordImportPayload{}, err
	}

	states, results, plans, issues, err := buildMetadataKeywordWriteResults(dirValue, versionValue, imported.locales, opts.Overwrite)
	if err != nil {
		return keywordImportPayload{}, err
	}
	if !opts.DryRun && len(issues) == 0 {
		if err := ApplyWritePlans(plans); err != nil {
			return keywordImportPayload{}, err
		}
	}
	sideDataReportPath, sideDataRecordCount, err := maybeWriteMetadataKeywordSideDataReport(
		dirValue,
		versionValue,
		inputValue,
		formatValue,
		opts.SideDataReportFile,
		opts.DryRun || len(issues) > 0,
		imported.sideData,
	)
	if err != nil {
		return keywordImportPayload{}, err
	}
	detectedLocales := sortedKeys(imported.locales)

	return keywordImportPayload{
		states: states,
		result: MetadataKeywordsImportResult{
			Dir:                 dirValue,
			Version:             versionValue,
			Input:               inputValue,
			Format:              formatValue,
			DryRun:              opts.DryRun,
			Valid:               len(issues) == 0,
			DetectedLocales:     detectedLocales,
			Results:             results,
			Issues:              issues,
			SideDataRecordCount: sideDataRecordCount,
			SideDataReportPath:  sideDataReportPath,
		},
	}, nil
}

func maybeWriteMetadataKeywordSideDataReport(
	dir string,
	version string,
	input string,
	format string,
	reportFile string,
	dryRun bool,
	records []MetadataKeywordSideDataRecord,
) (string, int, error) {
	if len(records) == 0 {
		return "", 0, nil
	}

	path, err := resolveMetadataKeywordSideDataReportPath(dir, version, reportFile)
	if err != nil {
		return "", 0, err
	}
	if dryRun {
		return path, len(records), nil
	}

	artifact := MetadataKeywordSideDataArtifact{
		Dir:     dir,
		Version: version,
		Input:   input,
		Format:  format,
		Records: records,
	}
	data, err := encodeCanonicalJSON(artifact)
	if err != nil {
		return "", 0, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", 0, err
	}
	if err := writeFileNoFollow(path, data); err != nil {
		return "", 0, err
	}
	return path, len(records), nil
}

func resolveMetadataKeywordSideDataReportPath(dir string, version string, reportFile string) (string, error) {
	if strings.TrimSpace(reportFile) != "" {
		return strings.TrimSpace(reportFile), nil
	}
	resolvedVersion, err := validatePathSegment("version", version)
	if err != nil {
		return "", shared.UsageError(err.Error())
	}
	return filepath.Join(strings.TrimSpace(dir), "reports", "metadata-keywords-side-data", resolvedVersion+".json"), nil
}

func resolveMetadataKeywordImportFormat(inputPath string, format string) (string, error) {
	formatValue := strings.ToLower(strings.TrimSpace(format))
	if formatValue == "" {
		formatValue = keywordImportFormatAuto
	}
	switch formatValue {
	case keywordImportFormatAuto:
		if strings.TrimSpace(inputPath) == "-" {
			return "", fmt.Errorf("--format is required when --input is -")
		}
		switch strings.ToLower(filepath.Ext(inputPath)) {
		case ".csv":
			return keywordImportFormatCSV, nil
		case ".json":
			return keywordImportFormatJSON, nil
		case ".txt", ".text":
			return keywordImportFormatText, nil
		default:
			return "", fmt.Errorf("could not infer input format from %q; use --format", inputPath)
		}
	default:
		if _, ok := metadataKeywordImportFormats[formatValue]; ok {
			return formatValue, nil
		}
		return "", fmt.Errorf("--format must be one of %s", strings.Join(metadataKeywordImportFormatList(), ", "))
	}
}

func metadataKeywordImportFormatList() []string {
	formats := []string{keywordImportFormatAuto}
	for name := range metadataKeywordImportFormats {
		formats = append(formats, name)
	}
	sort.Strings(formats[1:])
	return formats
}

func readMetadataKeywordImportInput(inputPath, format, defaultLocale string) (metadataKeywordImportedData, error) {
	data, err := readMetadataKeywordInputBytes(inputPath)
	if err != nil {
		return metadataKeywordImportedData{}, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return metadataKeywordImportedData{}, shared.UsageError("import input is empty")
	}

	parser, ok := metadataKeywordImportFormats[format]
	if !ok {
		return metadataKeywordImportedData{}, shared.UsageErrorf("unsupported import format %q", format)
	}
	raw, err := parser(data, defaultLocale)
	if err != nil {
		return metadataKeywordImportedData{}, err
	}
	return normalizeImportedMetadataKeywords(raw, defaultLocale)
}

func readMetadataKeywordInputBytes(inputPath string) ([]byte, error) {
	inputValue := strings.TrimSpace(inputPath)
	if inputValue == "-" {
		return io.ReadAll(os.Stdin)
	}
	return readFileNoFollow(inputValue)
}

func parseMetadataKeywordText(data []byte, defaultLocale string) (metadataKeywordImportedData, error) {
	if strings.TrimSpace(defaultLocale) == "" {
		return metadataKeywordImportedData{}, shared.UsageError("--locale is required for text input")
	}
	return metadataKeywordImportedData{
		locales: map[string][]string{
			defaultLocale: splitMetadataKeywordTokens(string(data)),
		},
	}, nil
}

func parseMetadataKeywordCSV(data []byte, defaultLocale string) (metadataKeywordImportedData, error) {
	return parseMetadataKeywordCSVWithHeaders(
		data,
		defaultLocale,
		[]string{"locale", "lang", "language"},
		[]string{"keywords", "keywordfield", "keywordlist"},
		[]string{"keyword", "term", "searchterm", "searchkeyword"},
	)
}

func parseMetadataKeywordAstroCSV(data []byte, defaultLocale string) (metadataKeywordImportedData, error) {
	return parseMetadataKeywordCSVWithHeaders(
		data,
		defaultLocale,
		[]string{"locale", "lang", "language"},
		nil,
		[]string{"keyword"},
	)
}

func parseMetadataKeywordCSVWithHeaders(
	data []byte,
	defaultLocale string,
	localeHeaders []string,
	keywordsHeaders []string,
	keywordHeaders []string,
) (metadataKeywordImportedData, error) {
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	rows, err := reader.ReadAll()
	if err != nil {
		return metadataKeywordImportedData{}, shared.UsageErrorf("invalid csv input: %v", err)
	}
	if len(rows) == 0 {
		return metadataKeywordImportedData{}, shared.UsageError("csv input is empty")
	}

	headerIndex := make(map[string]int, len(rows[0]))
	for idx, header := range rows[0] {
		headerIndex[normalizeMetadataKeywordHeader(header)] = idx
	}

	localeIdx, hasLocale := metadataKeywordHeaderIndex(headerIndex, localeHeaders...)
	keywordsIdx, hasKeywords := metadataKeywordHeaderIndex(headerIndex, keywordsHeaders...)
	keywordIdx, hasKeyword := metadataKeywordHeaderIndex(headerIndex, keywordHeaders...)
	if !hasKeywords && !hasKeyword {
		return metadataKeywordImportedData{}, shared.UsageError(`csv input requires a "keywords" or "keyword" column`)
	}

	result := make(map[string][]string)
	sideData := make([]MetadataKeywordSideDataRecord, 0)
	for rowIndex, row := range rows[1:] {
		if metadataKeywordCSVRowEmpty(row) {
			continue
		}

		rowLocale := strings.TrimSpace(defaultLocale)
		if hasLocale && localeIdx < len(row) {
			rowLocale = strings.TrimSpace(row[localeIdx])
			if rowLocale == "" {
				rowLocale = strings.TrimSpace(defaultLocale)
			}
		}
		if rowLocale == "" {
			return metadataKeywordImportedData{}, shared.UsageErrorf("csv row %d is missing a locale (set --locale or add a locale column)", rowIndex+2)
		}

		var tokens []string
		if hasKeywords && keywordsIdx < len(row) {
			tokens = append(tokens, splitMetadataKeywordTokens(row[keywordsIdx])...)
		}
		if len(tokens) == 0 && hasKeyword && keywordIdx < len(row) {
			tokens = append(tokens, splitMetadataKeywordTokens(row[keywordIdx])...)
		}
		if len(tokens) == 0 {
			continue
		}
		result[rowLocale] = append(result[rowLocale], tokens...)
		fields := make(map[string]any)
		for idx, rawHeader := range rows[0] {
			if idx >= len(row) {
				continue
			}
			value := strings.TrimSpace(row[idx])
			if value == "" {
				continue
			}
			if (hasLocale && idx == localeIdx) || (hasKeywords && idx == keywordsIdx) || (hasKeyword && idx == keywordIdx) {
				continue
			}
			fields[normalizeMetadataKeywordHeader(rawHeader)] = value
		}
		if len(fields) > 0 {
			sideData = append(sideData, MetadataKeywordSideDataRecord{
				Locale:   rowLocale,
				Keywords: append([]string(nil), tokens...),
				Fields:   fields,
			})
		}
	}
	return metadataKeywordImportedData{
		locales:  result,
		sideData: sideData,
	}, nil
}

func parseMetadataKeywordJSON(data []byte, defaultLocale string) (metadataKeywordImportedData, error) {
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return metadataKeywordImportedData{}, shared.UsageErrorf("invalid json input: %v", err)
	}
	result, err := collectMetadataKeywordJSON(payload, strings.TrimSpace(defaultLocale))
	if err != nil {
		return metadataKeywordImportedData{}, err
	}
	return result, nil
}

func collectMetadataKeywordJSON(payload any, defaultLocale string) (metadataKeywordImportedData, error) {
	switch value := payload.(type) {
	case []any:
		result := make(map[string][]string)
		sideData := make([]MetadataKeywordSideDataRecord, 0)
		for idx, item := range value {
			locale, keywords, fields, err := parseMetadataKeywordJSONObject(item, defaultLocale)
			if err != nil {
				return metadataKeywordImportedData{}, shared.UsageErrorf("json item %d: %v", idx, err)
			}
			result[locale] = append(result[locale], keywords...)
			if len(fields) > 0 {
				sideData = append(sideData, MetadataKeywordSideDataRecord{
					Locale:   locale,
					Keywords: append([]string(nil), keywords...),
					Fields:   fields,
				})
			}
		}
		return metadataKeywordImportedData{locales: result, sideData: sideData}, nil
	case map[string]any:
		if nested, ok := value["localizations"]; ok {
			return collectMetadataKeywordJSON(nested, defaultLocale)
		}
		if looksLikeMetadataKeywordLocalizationObject(value) {
			locale, keywords, fields, err := parseMetadataKeywordJSONObject(value, defaultLocale)
			if err != nil {
				return metadataKeywordImportedData{}, err
			}
			result := metadataKeywordImportedData{
				locales: map[string][]string{locale: keywords},
			}
			if len(fields) > 0 {
				result.sideData = []MetadataKeywordSideDataRecord{{
					Locale:   locale,
					Keywords: append([]string(nil), keywords...),
					Fields:   fields,
				}}
			}
			return result, nil
		}

		result := make(map[string][]string)
		sideData := make([]MetadataKeywordSideDataRecord, 0)
		rawLocales := make([]string, 0, len(value))
		for rawLocale := range value {
			rawLocales = append(rawLocales, rawLocale)
		}
		sort.Strings(rawLocales)
		for _, rawLocale := range rawLocales {
			rawKeywords := value[rawLocale]
			if object, ok := rawKeywords.(map[string]any); ok && looksLikeMetadataKeywordLocalizationObject(object) {
				locale, keywords, fields, err := parseMetadataKeywordJSONObject(object, rawLocale)
				if err != nil {
					return metadataKeywordImportedData{}, shared.UsageErrorf("json locale %q: %v", rawLocale, err)
				}
				result[locale] = append(result[locale], keywords...)
				if len(fields) > 0 {
					sideData = append(sideData, MetadataKeywordSideDataRecord{
						Locale:   locale,
						Keywords: append([]string(nil), keywords...),
						Fields:   fields,
					})
				}
				continue
			}
			keywords, err := decodeMetadataKeywordValue(rawKeywords)
			if err != nil {
				return metadataKeywordImportedData{}, shared.UsageErrorf("json locale %q: %v", rawLocale, err)
			}
			result[rawLocale] = append(result[rawLocale], keywords...)
		}
		return metadataKeywordImportedData{locales: result, sideData: sideData}, nil
	default:
		return metadataKeywordImportedData{}, shared.UsageError("json input must be an object or array")
	}
}

func looksLikeMetadataKeywordLocalizationObject(value map[string]any) bool {
	if _, ok := value["locale"]; ok {
		return true
	}
	for key := range value {
		switch normalizeMetadataKeywordHeader(key) {
		case "keywords", "keywordfield", "keywordlist", "keyword", "term", "searchterm", "searchkeyword":
			return true
		}
	}
	return false
}

func metadataKeywordJSONSideFields(value map[string]any) map[string]any {
	fields := make(map[string]any)
	for rawKey, rawValue := range value {
		switch normalizeMetadataKeywordHeader(rawKey) {
		case "locale", "keywords", "keywordfield", "keywordlist", "keyword", "term", "searchterm", "searchkeyword":
			continue
		default:
			fields[rawKey] = rawValue
		}
	}
	return fields
}

func parseMetadataKeywordJSONObject(raw any, defaultLocale string) (string, []string, map[string]any, error) {
	object, ok := raw.(map[string]any)
	if !ok {
		return "", nil, nil, fmt.Errorf("expected object")
	}

	localeValue := strings.TrimSpace(defaultLocale)
	if localeRaw, ok := object["locale"]; ok {
		if localeString, ok := localeRaw.(string); ok {
			localeValue = strings.TrimSpace(localeString)
		} else {
			return "", nil, nil, fmt.Errorf(`field "locale" must be a string`)
		}
	}
	if localeValue == "" {
		return "", nil, nil, fmt.Errorf("locale is required")
	}
	sideFields := metadataKeywordJSONSideFields(object)

	for _, key := range []string{"keywords", "keywordField", "keywordList", "keyword", "term", "searchTerm", "searchKeyword"} {
		if rawValue, ok := object[key]; ok {
			keywords, err := decodeMetadataKeywordValue(rawValue)
			if err != nil {
				return "", nil, nil, err
			}
			return localeValue, keywords, sideFields, nil
		}
	}
	for rawKey, rawValue := range object {
		switch normalizeMetadataKeywordHeader(rawKey) {
		case "keywords", "keywordfield", "keywordlist", "keyword", "term", "searchterm", "searchkeyword":
			keywords, err := decodeMetadataKeywordValue(rawValue)
			if err != nil {
				return "", nil, nil, err
			}
			return localeValue, keywords, sideFields, nil
		}
	}
	return "", nil, nil, fmt.Errorf("keywords field is required")
}

func decodeMetadataKeywordValue(raw any) ([]string, error) {
	switch value := raw.(type) {
	case string:
		return splitMetadataKeywordTokens(value), nil
	case []any:
		tokens := make([]string, 0, len(value))
		for idx, item := range value {
			itemString, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("keyword item %d must be a string", idx)
			}
			tokens = append(tokens, splitMetadataKeywordTokens(itemString)...)
		}
		return tokens, nil
	default:
		return nil, fmt.Errorf("keywords must be a string or string array")
	}
}

func normalizeImportedMetadataKeywords(raw metadataKeywordImportedData, defaultLocale string) (metadataKeywordImportedData, error) {
	result := make(map[string][]string, len(raw.locales))
	locales := make([]string, 0, len(raw.locales))
	for locale := range raw.locales {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	for _, locale := range locales {
		keywords := raw.locales[locale]
		resolvedLocale := strings.TrimSpace(locale)
		if resolvedLocale == "" {
			resolvedLocale = strings.TrimSpace(defaultLocale)
		}
		if resolvedLocale == "" {
			return metadataKeywordImportedData{}, shared.UsageError("locale is required for imported keywords")
		}

		canonicalLocale, err := validateMetadataKeywordLocale(resolvedLocale)
		if err != nil {
			return metadataKeywordImportedData{}, shared.UsageError(err.Error())
		}

		normalizedKeywords, err := normalizeMetadataKeywordTokensPreserveDuplicates(keywords)
		if err != nil {
			return metadataKeywordImportedData{}, shared.UsageErrorf("locale %q: %v", canonicalLocale, err)
		}
		result[canonicalLocale] = append(result[canonicalLocale], normalizedKeywords...)
	}

	for locale, keywords := range result {
		normalizedKeywords, err := normalizeMetadataKeywordTokensPreserveDuplicates(keywords)
		if err != nil {
			return metadataKeywordImportedData{}, shared.UsageErrorf("locale %q: %v", locale, err)
		}
		result[locale] = normalizedKeywords
	}
	if len(result) == 0 {
		return metadataKeywordImportedData{}, shared.UsageError("no keywords were found in the import input")
	}
	sideData := make([]MetadataKeywordSideDataRecord, 0, len(raw.sideData))
	for _, record := range raw.sideData {
		resolvedLocale := strings.TrimSpace(record.Locale)
		if resolvedLocale == "" {
			resolvedLocale = strings.TrimSpace(defaultLocale)
		}
		if resolvedLocale != "" {
			canonicalLocale, err := validateMetadataKeywordLocale(resolvedLocale)
			if err != nil {
				return metadataKeywordImportedData{}, shared.UsageError(err.Error())
			}
			record.Locale = canonicalLocale
		}
		if len(record.Keywords) > 0 {
			normalizedKeywords, err := normalizeMetadataKeywordTokensPreserveDuplicates(record.Keywords)
			if err != nil {
				return metadataKeywordImportedData{}, shared.UsageErrorf("locale %q: %v", record.Locale, err)
			}
			record.Keywords = normalizedKeywords
		}
		sideData = append(sideData, record)
	}
	return metadataKeywordImportedData{
		locales:  result,
		sideData: sideData,
	}, nil
}

func metadataKeywordHeaderIndex(headers map[string]int, names ...string) (int, bool) {
	for _, name := range names {
		if idx, ok := headers[name]; ok {
			return idx, true
		}
	}
	return 0, false
}

func normalizeMetadataKeywordHeader(value string) string {
	normalized := strings.TrimSpace(strings.TrimPrefix(value, "\ufeff"))
	normalized = strings.ToLower(normalized)
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	return normalized
}

func metadataKeywordCSVRowEmpty(row []string) bool {
	for _, item := range row {
		if strings.TrimSpace(item) != "" {
			return false
		}
	}
	return true
}
