package shared

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const (
	LocalizationTypeVersion = "version"
	LocalizationTypeAppInfo = "app-info"
)

var (
	versionLocalizationKeys = []string{
		"description",
		"keywords",
		"marketingUrl",
		"promotionalText",
		"supportUrl",
		"whatsNew",
	}
	appInfoLocalizationKeys = []string{
		"name",
		"subtitle",
		"privacyPolicyUrl",
		"privacyChoicesUrl",
		"privacyPolicyText",
	}
	versionLocalizationAllowedKeys = buildAllowedKeys(versionLocalizationKeys)
	appInfoLocalizationAllowedKeys = buildAllowedKeys(appInfoLocalizationKeys)
)

// VersionLocalizationKeys returns the supported app store version localization keys.
func VersionLocalizationKeys() []string {
	return append([]string(nil), versionLocalizationKeys...)
}

// ValidateVersionLocalizationKeys validates .strings keys for a version localization locale.
func ValidateVersionLocalizationKeys(locale string, values map[string]string) error {
	return validateLocalizationKeys(locale, values, versionLocalizationAllowedKeys)
}

// ValidateAppInfoLocalizationKeys validates .strings keys for an app-info localization locale.
func ValidateAppInfoLocalizationKeys(locale string, values map[string]string) error {
	return validateLocalizationKeys(locale, values, appInfoLocalizationAllowedKeys)
}

type versionLocalizationClient interface {
	GetAppStoreVersionLocalizations(context.Context, string, ...asc.AppStoreVersionLocalizationsOption) (*asc.AppStoreVersionLocalizationsResponse, error)
	CreateAppStoreVersionLocalization(context.Context, string, asc.AppStoreVersionLocalizationAttributes) (*asc.AppStoreVersionLocalizationResponse, error)
	UpdateAppStoreVersionLocalization(context.Context, string, asc.AppStoreVersionLocalizationAttributes) (*asc.AppStoreVersionLocalizationResponse, error)
}

type appInfoLocalizationClient interface {
	GetAppInfoLocalizations(context.Context, string, ...asc.AppInfoLocalizationsOption) (*asc.AppInfoLocalizationsResponse, error)
	CreateAppInfoLocalization(context.Context, string, asc.AppInfoLocalizationAttributes) (*asc.AppInfoLocalizationResponse, error)
	UpdateAppInfoLocalization(context.Context, string, asc.AppInfoLocalizationAttributes) (*asc.AppInfoLocalizationResponse, error)
}

func NormalizeLocalizationType(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case LocalizationTypeVersion, LocalizationTypeAppInfo:
		return normalized, nil
	default:
		return "", fmt.Errorf("--type must be %q or %q", LocalizationTypeVersion, LocalizationTypeAppInfo)
	}
}

func WriteVersionLocalizationStrings(outputPath string, items []asc.Resource[asc.AppStoreVersionLocalizationAttributes]) ([]asc.LocalizationFileResult, error) {
	byLocale := make(map[string]map[string]string, len(items))
	for _, item := range items {
		locale := strings.TrimSpace(item.Attributes.Locale)
		if locale == "" {
			continue
		}
		byLocale[locale] = mapVersionLocalizationStrings(item.Attributes)
	}
	return writeLocalizationStrings(outputPath, byLocale, versionLocalizationKeys)
}

func WriteAppInfoLocalizationStrings(outputPath string, items []asc.Resource[asc.AppInfoLocalizationAttributes]) ([]asc.LocalizationFileResult, error) {
	byLocale := make(map[string]map[string]string, len(items))
	for _, item := range items {
		locale := strings.TrimSpace(item.Attributes.Locale)
		if locale == "" {
			continue
		}
		byLocale[locale] = mapAppInfoLocalizationStrings(item.Attributes)
	}
	return writeLocalizationStrings(outputPath, byLocale, appInfoLocalizationKeys)
}

func writeLocalizationStrings(outputPath string, valuesByLocale map[string]map[string]string, order []string) ([]asc.LocalizationFileResult, error) {
	if len(valuesByLocale) == 0 {
		return nil, fmt.Errorf("no localizations returned")
	}

	locales := make([]string, 0, len(valuesByLocale))
	for locale := range valuesByLocale {
		locales = append(locales, locale)
	}
	sort.Strings(locales)

	paths, err := resolveLocalizationOutputPaths(outputPath, locales)
	if err != nil {
		return nil, err
	}

	results := make([]asc.LocalizationFileResult, 0, len(locales))
	for _, locale := range locales {
		path, ok := paths[locale]
		if !ok {
			continue
		}
		if err := writeStringsFile(path, valuesByLocale[locale], order); err != nil {
			return nil, err
		}
		results = append(results, asc.LocalizationFileResult{
			Locale: locale,
			Path:   path,
		})
	}
	return results, nil
}

// localeValidationRegex matches valid Apple locale codes (e.g., "en", "en-US", "zh-Hans", "zh-Hant")
// This prevents path traversal attacks via malicious locale values.
// Allows 2-3 letter language codes, optionally followed by BCP-47 subtags (case-insensitive).
var localeValidationRegex = regexp.MustCompile(`^[a-zA-Z]{2,3}(-[a-zA-Z0-9]+)*$`)

// isValidLocale checks if a locale string is safe to use in file paths.
// Valid locales follow the pattern: 2-3 lowercase letters, optionally followed by
// a hyphen and uppercase letters/numbers (e.g., "en", "en-US", "zh-Hans").
func isValidLocale(locale string) bool {
	if locale == "" || len(locale) > 20 {
		return false
	}
	return localeValidationRegex.MatchString(locale)
}

func resolveLocalizationOutputPaths(outputPath string, locales []string) (map[string]string, error) {
	if strings.TrimSpace(outputPath) == "" {
		outputPath = "localizations"
	}

	result := make(map[string]string, len(locales))
	if strings.HasSuffix(outputPath, ".strings") {
		if len(locales) != 1 {
			return nil, fmt.Errorf("output path %q requires exactly one locale", outputPath)
		}
		path := outputPath
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		result[locales[0]] = path
		return result, nil
	}

	if err := os.MkdirAll(outputPath, 0o755); err != nil {
		return nil, err
	}
	for _, locale := range locales {
		// Validate locale to prevent path traversal attacks
		if !isValidLocale(locale) {
			return nil, fmt.Errorf("invalid locale code %q: must match pattern like 'en', 'en-US', or 'zh-Hans'", locale)
		}
		result[locale] = filepath.Join(outputPath, locale+".strings")
	}
	return result, nil
}

func mapVersionLocalizationStrings(attrs asc.AppStoreVersionLocalizationAttributes) map[string]string {
	values := make(map[string]string)
	setIfNotEmpty(values, "description", attrs.Description)
	setIfNotEmpty(values, "keywords", attrs.Keywords)
	setIfNotEmpty(values, "marketingUrl", attrs.MarketingURL)
	setIfNotEmpty(values, "promotionalText", attrs.PromotionalText)
	setIfNotEmpty(values, "supportUrl", attrs.SupportURL)
	setIfNotEmpty(values, "whatsNew", attrs.WhatsNew)
	return values
}

// MapVersionLocalizationStrings converts version localization attributes into .strings keys.
func MapVersionLocalizationStrings(attrs asc.AppStoreVersionLocalizationAttributes) map[string]string {
	return mapVersionLocalizationStrings(attrs)
}

func mapAppInfoLocalizationStrings(attrs asc.AppInfoLocalizationAttributes) map[string]string {
	values := make(map[string]string)
	setIfNotEmpty(values, "name", attrs.Name)
	setIfNotEmpty(values, "subtitle", attrs.Subtitle)
	setIfNotEmpty(values, "privacyPolicyUrl", attrs.PrivacyPolicyURL)
	setIfNotEmpty(values, "privacyChoicesUrl", attrs.PrivacyChoicesURL)
	setIfNotEmpty(values, "privacyPolicyText", attrs.PrivacyPolicyText)
	return values
}

func setIfNotEmpty(values map[string]string, key, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	values[key] = value
}

func ReadLocalizationStrings(inputPath string, locales []string) (map[string]map[string]string, error) {
	info, err := os.Stat(inputPath)
	if err != nil {
		return nil, err
	}

	filter := make(map[string]bool)
	for _, locale := range locales {
		filter[locale] = true
	}

	if !info.IsDir() {
		if len(locales) > 1 {
			return nil, fmt.Errorf("single file input only supports one locale")
		}
		locale := ""
		if len(locales) == 1 {
			locale = locales[0]
		} else {
			locale = strings.TrimSuffix(filepath.Base(inputPath), ".strings")
			if locale == "" || locale == filepath.Base(inputPath) {
				return nil, fmt.Errorf("cannot infer locale from %q (use --locale)", inputPath)
			}
		}

		entries, err := readStringsFile(inputPath)
		if err != nil {
			return nil, err
		}
		return map[string]map[string]string{locale: entries}, nil
	}

	entries, err := os.ReadDir(inputPath)
	if err != nil {
		return nil, err
	}

	values := make(map[string]map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".strings" {
			continue
		}
		locale := strings.TrimSuffix(entry.Name(), ".strings")
		if locale == "" {
			continue
		}
		if len(filter) > 0 && !filter[locale] {
			continue
		}
		path := filepath.Join(inputPath, entry.Name())
		parsed, err := readStringsFile(path)
		if err != nil {
			return nil, err
		}
		if _, exists := values[locale]; exists {
			return nil, fmt.Errorf("duplicate locale %q in %s", locale, inputPath)
		}
		values[locale] = parsed
	}

	if len(values) == 0 {
		return nil, fmt.Errorf("no .strings files found in %q", inputPath)
	}
	return values, nil
}

func UploadVersionLocalizations(ctx context.Context, client versionLocalizationClient, versionID string, valuesByLocale map[string]map[string]string, dryRun bool) ([]asc.LocalizationUploadLocaleResult, error) {
	for locale, values := range valuesByLocale {
		if err := ValidateVersionLocalizationKeys(locale, values); err != nil {
			return nil, err
		}
	}

	existing, err := client.GetAppStoreVersionLocalizations(ctx, versionID, asc.WithAppStoreVersionLocalizationsLimit(200))
	if err != nil {
		return nil, err
	}
	existingByLocale := make(map[string]string, len(existing.Data))
	for _, item := range existing.Data {
		if strings.TrimSpace(item.Attributes.Locale) == "" {
			continue
		}
		existingByLocale[item.Attributes.Locale] = item.ID
	}

	return uploadLocalizationValues(valuesByLocale, existingByLocale, func(locale string, values map[string]string, existingID string) (asc.LocalizationUploadLocaleResult, error) {
		attributes := buildVersionLocalizationAttributes(locale, values, existingID == "")
		if existingID == "" {
			if dryRun {
				return asc.LocalizationUploadLocaleResult{Locale: locale, Action: "create"}, nil
			}
			resp, err := client.CreateAppStoreVersionLocalization(ctx, versionID, attributes)
			if err != nil {
				return asc.LocalizationUploadLocaleResult{}, err
			}
			return asc.LocalizationUploadLocaleResult{Locale: locale, Action: "create", LocalizationID: resp.Data.ID}, nil
		}
		if dryRun {
			return asc.LocalizationUploadLocaleResult{Locale: locale, Action: "update", LocalizationID: existingID}, nil
		}
		resp, err := client.UpdateAppStoreVersionLocalization(ctx, existingID, attributes)
		// If the API rejects whatsNew (e.g. on an initial v1.0 release where
		// there is no previous version), retry without it and warn the user.
		if err != nil && strings.TrimSpace(attributes.WhatsNew) != "" && isWhatsNewUnsupportedError(err) {
			fmt.Fprintln(os.Stderr, "Warning: 'whatsNew' cannot be set for this version (initial releases have no What's New section). Retrying without it.")
			attributes.WhatsNew = ""
			resp, err = client.UpdateAppStoreVersionLocalization(ctx, existingID, attributes)
		}
		if err != nil {
			return asc.LocalizationUploadLocaleResult{}, err
		}
		return asc.LocalizationUploadLocaleResult{Locale: locale, Action: "update", LocalizationID: resp.Data.ID}, nil
	})
}

func UploadAppInfoLocalizations(ctx context.Context, client appInfoLocalizationClient, appInfoID string, valuesByLocale map[string]map[string]string, dryRun bool) ([]asc.LocalizationUploadLocaleResult, error) {
	for locale, values := range valuesByLocale {
		if err := ValidateAppInfoLocalizationKeys(locale, values); err != nil {
			return nil, err
		}
	}

	existing, err := client.GetAppInfoLocalizations(ctx, appInfoID, asc.WithAppInfoLocalizationsLimit(200))
	if err != nil {
		return nil, err
	}
	existingByLocale := make(map[string]string, len(existing.Data))
	for _, item := range existing.Data {
		if strings.TrimSpace(item.Attributes.Locale) == "" {
			continue
		}
		existingByLocale[item.Attributes.Locale] = item.ID
	}

	return uploadLocalizationValues(valuesByLocale, existingByLocale, func(locale string, values map[string]string, existingID string) (asc.LocalizationUploadLocaleResult, error) {
		attributes := buildAppInfoLocalizationAttributes(locale, values, existingID == "")
		if existingID == "" {
			if dryRun {
				return asc.LocalizationUploadLocaleResult{Locale: locale, Action: "create"}, nil
			}
			resp, err := client.CreateAppInfoLocalization(ctx, appInfoID, attributes)
			if err != nil {
				return asc.LocalizationUploadLocaleResult{}, err
			}
			return asc.LocalizationUploadLocaleResult{Locale: locale, Action: "create", LocalizationID: resp.Data.ID}, nil
		}
		if dryRun {
			return asc.LocalizationUploadLocaleResult{Locale: locale, Action: "update", LocalizationID: existingID}, nil
		}
		resp, err := client.UpdateAppInfoLocalization(ctx, existingID, attributes)
		if err != nil {
			return asc.LocalizationUploadLocaleResult{}, err
		}
		return asc.LocalizationUploadLocaleResult{Locale: locale, Action: "update", LocalizationID: resp.Data.ID}, nil
	})
}

func isWhatsNewUnsupportedError(err error) bool {
	if err == nil {
		return false
	}
	if apiErr, ok := errors.AsType[*asc.APIError](err); ok {
		if containsWhatsNewToken(apiErr.Code) || containsWhatsNewToken(apiErr.Title) || containsWhatsNewToken(apiErr.Detail) {
			return true
		}
	}
	return containsWhatsNewToken(err.Error())
}

func containsWhatsNewToken(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return false
	}
	normalized = strings.ReplaceAll(normalized, "'", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, " ", "")
	return strings.Contains(normalized, "whatsnew")
}

func uploadLocalizationValues(valuesByLocale map[string]map[string]string, existing map[string]string, handler func(locale string, values map[string]string, existingID string) (asc.LocalizationUploadLocaleResult, error)) ([]asc.LocalizationUploadLocaleResult, error) {
	locales := make([]string, 0, len(valuesByLocale))
	for locale := range valuesByLocale {
		locales = append(locales, locale)
	}
	sort.Strings(locales)

	results := make([]asc.LocalizationUploadLocaleResult, 0, len(locales))
	for _, locale := range locales {
		values := valuesByLocale[locale]
		if len(values) == 0 {
			return nil, fmt.Errorf("no localization values for locale %q", locale)
		}
		if !hasNonEmptyLocalizationValues(values) {
			return nil, fmt.Errorf("localization values for locale %q are empty", locale)
		}
		result, err := handler(locale, values, existing[locale])
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func hasNonEmptyLocalizationValues(values map[string]string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func buildAllowedKeys(keys []string) map[string]bool {
	allowed := make(map[string]bool, len(keys))
	for _, key := range keys {
		allowed[key] = true
	}
	return allowed
}

func validateLocalizationKeys(locale string, values map[string]string, allowed map[string]bool) error {
	unknown := make([]string, 0)
	for key := range values {
		if !allowed[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("unsupported keys for locale %q: %s", locale, strings.Join(unknown, ", "))
	}
	return nil
}

func buildVersionLocalizationAttributes(locale string, values map[string]string, includeLocale bool) asc.AppStoreVersionLocalizationAttributes {
	attrs := asc.AppStoreVersionLocalizationAttributes{}
	if includeLocale {
		attrs.Locale = locale
	}
	if value, ok := values["description"]; ok {
		attrs.Description = value
	}
	if value, ok := values["keywords"]; ok {
		attrs.Keywords = value
	}
	if value, ok := values["marketingUrl"]; ok {
		attrs.MarketingURL = value
	}
	if value, ok := values["promotionalText"]; ok {
		attrs.PromotionalText = value
	}
	if value, ok := values["supportUrl"]; ok {
		attrs.SupportURL = value
	}
	if value, ok := values["whatsNew"]; ok {
		attrs.WhatsNew = value
	}
	return attrs
}

func buildAppInfoLocalizationAttributes(locale string, values map[string]string, includeLocale bool) asc.AppInfoLocalizationAttributes {
	attrs := asc.AppInfoLocalizationAttributes{}
	if includeLocale {
		attrs.Locale = locale
	}
	if value, ok := values["name"]; ok {
		attrs.Name = value
	}
	if value, ok := values["subtitle"]; ok {
		attrs.Subtitle = value
	}
	if value, ok := values["privacyPolicyUrl"]; ok {
		attrs.PrivacyPolicyURL = value
	}
	if value, ok := values["privacyChoicesUrl"]; ok {
		attrs.PrivacyChoicesURL = value
	}
	if value, ok := values["privacyPolicyText"]; ok {
		attrs.PrivacyPolicyText = value
	}
	return attrs
}

type stringsParser struct {
	runes []rune
	pos   int
	line  int
}

func readStringsFile(path string) (map[string]string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to read symlink %q", path)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("expected regular file: %q", path)
	}

	file, err := OpenExistingNoFollow(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return parseStringsContent(string(data))
}

func parseStringsContent(content string) (map[string]string, error) {
	parser := &stringsParser{runes: []rune(content), line: 1}
	values := make(map[string]string)
	for {
		if err := parser.skipWhitespaceAndComments(); err != nil {
			return nil, err
		}
		if parser.eof() {
			break
		}
		key, err := parser.readQuoted()
		if err != nil {
			return nil, err
		}
		if err := parser.skipWhitespaceAndComments(); err != nil {
			return nil, err
		}
		if !parser.consume('=') {
			return nil, parser.errorf("expected '=' after key")
		}
		if err := parser.skipWhitespaceAndComments(); err != nil {
			return nil, err
		}
		value, err := parser.readQuoted()
		if err != nil {
			return nil, err
		}
		if err := parser.skipWhitespaceAndComments(); err != nil {
			return nil, err
		}
		if !parser.consume(';') {
			return nil, parser.errorf("expected ';' after value")
		}
		values[key] = value
	}
	return values, nil
}

func (p *stringsParser) eof() bool {
	return p.pos >= len(p.runes)
}

func (p *stringsParser) peek() rune {
	if p.eof() {
		return 0
	}
	return p.runes[p.pos]
}

func (p *stringsParser) peekNext() rune {
	if p.pos+1 >= len(p.runes) {
		return 0
	}
	return p.runes[p.pos+1]
}

func (p *stringsParser) next() rune {
	if p.eof() {
		return 0
	}
	ch := p.runes[p.pos]
	p.pos++
	if ch == '\n' {
		p.line++
	}
	return ch
}

func (p *stringsParser) consume(expected rune) bool {
	if p.peek() != expected {
		return false
	}
	p.next()
	return true
}

func (p *stringsParser) skipWhitespaceAndComments() error {
	for {
		for unicode.IsSpace(p.peek()) {
			p.next()
		}
		if p.peek() == '/' && p.peekNext() == '/' {
			for !p.eof() && p.next() != '\n' {
			}
			continue
		}
		if p.peek() == '/' && p.peekNext() == '*' {
			p.next()
			p.next()
			for !p.eof() {
				if p.peek() == '*' && p.peekNext() == '/' {
					p.next()
					p.next()
					break
				}
				p.next()
			}
			if p.eof() {
				return p.errorf("unterminated block comment")
			}
			continue
		}
		break
	}
	return nil
}

func (p *stringsParser) readQuoted() (string, error) {
	if !p.consume('"') {
		return "", p.errorf("expected '\"'")
	}
	var b strings.Builder
	for !p.eof() {
		ch := p.next()
		if ch == '"' {
			return b.String(), nil
		}
		if ch == '\\' {
			if p.eof() {
				return "", p.errorf("unterminated escape sequence")
			}
			escaped := p.next()
			switch escaped {
			case '"', '\\':
				b.WriteRune(escaped)
			case 'n':
				b.WriteRune('\n')
			case 'r':
				b.WriteRune('\r')
			case 't':
				b.WriteRune('\t')
			case 'u':
				r, err := p.readHexRune(4)
				if err != nil {
					return "", err
				}
				b.WriteRune(r)
			case 'U':
				r, err := p.readHexRune(8)
				if err != nil {
					return "", err
				}
				b.WriteRune(r)
			default:
				b.WriteRune(escaped)
			}
			continue
		}
		b.WriteRune(ch)
	}
	return "", p.errorf("unterminated string")
}

func (p *stringsParser) readHexRune(length int) (rune, error) {
	if p.pos+length > len(p.runes) {
		return 0, p.errorf("invalid unicode escape")
	}
	hex := string(p.runes[p.pos : p.pos+length])
	p.pos += length
	value, err := strconv.ParseInt(hex, 16, 32)
	if err != nil {
		return 0, p.errorf("invalid unicode escape")
	}
	return rune(value), nil
}

func (p *stringsParser) errorf(message string) error {
	return fmt.Errorf("strings parse error on line %d: %s", p.line, message)
}

func writeStringsFile(path string, values map[string]string, order []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	var b strings.Builder
	for _, key := range order {
		value, ok := values[key]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "\"%s\" = \"%s\";\n", key, escapeStringsValue(value))
	}

	// Create file securely to prevent symlink attacks and TOCTOU vulnerabilities
	// O_EXCL ensures atomic creation, O_NOFOLLOW prevents symlink traversal
	file, err := OpenNewFileNoFollow(path, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("output file already exists: %w", err)
		}
		return err
	}
	defer func() { _ = file.Close() }()

	if _, err := file.WriteString(b.String()); err != nil {
		return err
	}
	return file.Sync()
}

func escapeStringsValue(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"\n", "\\n",
		"\r", "\\r",
		"\t", "\\t",
	)
	return replacer.Replace(value)
}
