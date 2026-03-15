package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/suggest"
)

const (
	appInfoDirName = "app-info"
	versionDirName = "version"
	// DefaultLocale is the fastlane-compatible fallback locale token.
	DefaultLocale = "default"
)

var localePattern = regexp.MustCompile(`^[a-zA-Z]{2,3}(-[a-zA-Z0-9]+)*$`)

var supportedMetadataLocales = []string{
	"ar-SA",
	"ca",
	"cs",
	"da",
	"de-DE",
	"el",
	"en-AU",
	"en-CA",
	"en-GB",
	"en-US",
	"es-ES",
	"es-MX",
	"fi",
	"fr-CA",
	"fr-FR",
	"he",
	"hi",
	"hr",
	"hu",
	"id",
	"it",
	"ja",
	"ko",
	"ms",
	"nl-NL",
	"no",
	"pl",
	"pt-BR",
	"pt-PT",
	"ro",
	"ru",
	"sk",
	"sv",
	"th",
	"tr",
	"uk",
	"vi",
	"zh-Hans",
	"zh-Hant",
}

var supportedMetadataLocaleByFold = func() map[string]string {
	result := make(map[string]string, len(supportedMetadataLocales))
	for _, locale := range supportedMetadataLocales {
		result[strings.ToLower(locale)] = locale
	}
	return result
}()

var metadataLocaleCandidatesByRoot = func() map[string][]string {
	result := make(map[string][]string)
	for _, locale := range supportedMetadataLocales {
		root := metadataLocaleRoot(locale)
		result[root] = append(result[root], locale)
	}
	for root := range result {
		sort.Strings(result[root])
	}
	return result
}()

var metadataLocaleCandidatesByLanguageName = map[string][]string{
	"arabic":     {"ar-SA"},
	"catalan":    {"ca"},
	"chinese":    {"zh-Hans", "zh-Hant"},
	"croatian":   {"hr"},
	"czech":      {"cs"},
	"danish":     {"da"},
	"dutch":      {"nl-NL"},
	"english":    {"en-AU", "en-CA", "en-GB", "en-US"},
	"finnish":    {"fi"},
	"french":     {"fr-CA", "fr-FR"},
	"german":     {"de-DE"},
	"greek":      {"el"},
	"hebrew":     {"he"},
	"hindi":      {"hi"},
	"hungarian":  {"hu"},
	"indonesian": {"id"},
	"italian":    {"it"},
	"japanese":   {"ja"},
	"korean":     {"ko"},
	"malay":      {"ms"},
	"norwegian":  {"no"},
	"polish":     {"pl"},
	"portuguese": {"pt-BR", "pt-PT"},
	"romanian":   {"ro"},
	"russian":    {"ru"},
	"slovak":     {"sk"},
	"spanish":    {"es-ES", "es-MX"},
	"swedish":    {"sv"},
	"thai":       {"th"},
	"turkish":    {"tr"},
	"ukrainian":  {"uk"},
	"vietnamese": {"vi"},
}

var metadataLocaleAliases = func() map[string]string {
	result := make(map[string]string)
	add := func(alias string, canonical string) {
		result[normalizeMetadataLocaleAliasKey(alias)] = canonical
	}

	for _, locale := range supportedMetadataLocales {
		add(locale, locale)
		add(strings.ReplaceAll(locale, "-", "_"), locale)
	}

	add("ar", "ar-SA")
	add("arabic-ar", "ar-SA")
	add("catalan-ca", "ca")
	add("croatian-hr", "hr")
	add("czech-cs", "cs")
	add("danish-da", "da")
	add("de", "de-DE")
	add("dutch-nl", "nl-NL")
	add("english-au", "en-AU")
	add("english-australia", "en-AU")
	add("english-ca", "en-CA")
	add("english-canada", "en-CA")
	add("english-gb", "en-GB")
	add("english-uk", "en-GB")
	add("en-uk", "en-GB")
	add("english-us", "en-US")
	add("english-u-s", "en-US")
	add("french-ca", "fr-CA")
	add("french-canada", "fr-CA")
	add("french-fr", "fr-FR")
	add("french-france", "fr-FR")
	add("german-de", "de-DE")
	add("hebrew-he", "he")
	add("hindi-hi", "hi")
	add("hungarian-hu", "hu")
	add("japanese-ja", "ja")
	add("korean-ko", "ko")
	add("norwegian-no", "no")
	add("polish-pl", "pl")
	add("portuguese-br", "pt-BR")
	add("portuguese-brazil", "pt-BR")
	add("portuguese-pt", "pt-PT")
	add("portuguese-portugal", "pt-PT")
	add("romanian-ro", "ro")
	add("russian-ru", "ru")
	add("slovak-sk", "sk")
	add("spanish-es", "es-ES")
	add("spanish-spain", "es-ES")
	add("spanish-mexico", "es-MX")
	add("spanish-mx", "es-MX")
	add("swedish-sv", "sv")
	add("thai-th", "th")
	add("turkish-tr", "tr")
	add("ukrainian-uk", "uk")
	add("vietnamese-vi", "vi")
	add("zh-cn", "zh-Hans")
	add("zh-sg", "zh-Hans")
	add("chinese-simplified", "zh-Hans")
	add("chinese-simplified-cn", "zh-Hans")
	add("zh-tw", "zh-Hant")
	add("zh-hk", "zh-Hant")
	add("zh-mo", "zh-Hant")
	add("chinese-traditional", "zh-Hant")
	add("chinese-traditional-tw", "zh-Hant")

	return result
}()

// AppInfoLocalization is the canonical app-info localization schema.
type AppInfoLocalization struct {
	Name              string `json:"name,omitempty"`
	Subtitle          string `json:"subtitle,omitempty"`
	PrivacyPolicyURL  string `json:"privacyPolicyUrl,omitempty"`
	PrivacyChoicesURL string `json:"privacyChoicesUrl,omitempty"`
	PrivacyPolicyText string `json:"privacyPolicyText,omitempty"`
}

// VersionLocalization is the canonical version localization schema.
type VersionLocalization struct {
	Description     string `json:"description,omitempty"`
	Keywords        string `json:"keywords,omitempty"`
	MarketingURL    string `json:"marketingUrl,omitempty"`
	PromotionalText string `json:"promotionalText,omitempty"`
	SupportURL      string `json:"supportUrl,omitempty"`
	WhatsNew        string `json:"whatsNew,omitempty"`
}

// ValidationOptions controls required-field validation.
type ValidationOptions struct {
	RequireName bool
	AllowEmpty  bool
}

// ValidationIssue describes a schema or content validation issue.
type ValidationIssue struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// WritePlan represents one deterministic file write operation.
type WritePlan struct {
	Path     string
	Contents []byte
}

// NormalizeAppInfoLocalization trims all field values.
func NormalizeAppInfoLocalization(loc AppInfoLocalization) AppInfoLocalization {
	return AppInfoLocalization{
		Name:              strings.TrimSpace(loc.Name),
		Subtitle:          strings.TrimSpace(loc.Subtitle),
		PrivacyPolicyURL:  strings.TrimSpace(loc.PrivacyPolicyURL),
		PrivacyChoicesURL: strings.TrimSpace(loc.PrivacyChoicesURL),
		PrivacyPolicyText: strings.TrimSpace(loc.PrivacyPolicyText),
	}
}

// NormalizeVersionLocalization trims all field values.
func NormalizeVersionLocalization(loc VersionLocalization) VersionLocalization {
	return VersionLocalization{
		Description:     strings.TrimSpace(loc.Description),
		Keywords:        strings.TrimSpace(loc.Keywords),
		MarketingURL:    strings.TrimSpace(loc.MarketingURL),
		PromotionalText: strings.TrimSpace(loc.PromotionalText),
		SupportURL:      strings.TrimSpace(loc.SupportURL),
		WhatsNew:        strings.TrimSpace(loc.WhatsNew),
	}
}

// ValidateAppInfoLocalization validates required app-info localization fields.
func ValidateAppInfoLocalization(loc AppInfoLocalization, opts ValidationOptions) []ValidationIssue {
	normalized := NormalizeAppInfoLocalization(loc)
	issues := make([]ValidationIssue, 0, 2)

	if opts.RequireName && normalized.Name == "" {
		issues = append(issues, ValidationIssue{
			Field:   "name",
			Message: "name is required",
		})
	}
	if !opts.AllowEmpty && !hasAppInfoContent(normalized) {
		issues = append(issues, ValidationIssue{
			Field:   "metadata",
			Message: "at least one app-info field is required",
		})
	}

	return issues
}

// ValidateVersionLocalization validates required version localization fields.
func ValidateVersionLocalization(loc VersionLocalization) []ValidationIssue {
	normalized := NormalizeVersionLocalization(loc)
	if hasVersionContent(normalized) {
		return nil
	}
	return []ValidationIssue{
		{
			Field:   "metadata",
			Message: "at least one version metadata field is required",
		},
	}
}

// DecodeAppInfoLocalization strictly decodes canonical app-info JSON.
func DecodeAppInfoLocalization(data []byte) (AppInfoLocalization, error) {
	var loc AppInfoLocalization
	if err := decodeStrictJSON(data, &loc); err != nil {
		return AppInfoLocalization{}, fmt.Errorf("decode app-info localization: %w", err)
	}
	return NormalizeAppInfoLocalization(loc), nil
}

// DecodeVersionLocalization strictly decodes canonical version JSON.
func DecodeVersionLocalization(data []byte) (VersionLocalization, error) {
	var loc VersionLocalization
	if err := decodeStrictJSON(data, &loc); err != nil {
		return VersionLocalization{}, fmt.Errorf("decode version localization: %w", err)
	}
	return NormalizeVersionLocalization(loc), nil
}

// EncodeAppInfoLocalization returns deterministic canonical JSON.
func EncodeAppInfoLocalization(loc AppInfoLocalization) ([]byte, error) {
	normalized := NormalizeAppInfoLocalization(loc)
	return encodeCanonicalJSON(normalized)
}

// EncodeVersionLocalization returns deterministic canonical JSON.
func EncodeVersionLocalization(loc VersionLocalization) ([]byte, error) {
	normalized := NormalizeVersionLocalization(loc)
	return encodeCanonicalJSON(normalized)
}

// ReadAppInfoLocalizationFile reads and decodes canonical app-info JSON.
func ReadAppInfoLocalizationFile(path string) (AppInfoLocalization, error) {
	data, err := readFileNoFollow(path)
	if err != nil {
		return AppInfoLocalization{}, err
	}
	return DecodeAppInfoLocalization(data)
}

// ReadVersionLocalizationFile reads and decodes canonical version JSON.
func ReadVersionLocalizationFile(path string) (VersionLocalization, error) {
	data, err := readFileNoFollow(path)
	if err != nil {
		return VersionLocalization{}, err
	}
	return DecodeVersionLocalization(data)
}

// AppInfoLocalizationFilePath resolves canonical app-info file path.
func AppInfoLocalizationFilePath(rootDir, locale string) (string, error) {
	base, err := validateRootDir(rootDir)
	if err != nil {
		return "", err
	}
	resolvedLocale, err := validateLocale(locale)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appInfoDirName, resolvedLocale+".json"), nil
}

// VersionLocalizationFilePath resolves canonical version file path.
func VersionLocalizationFilePath(rootDir, version, locale string) (string, error) {
	base, err := validateRootDir(rootDir)
	if err != nil {
		return "", err
	}
	resolvedVersion, err := validatePathSegment("version", version)
	if err != nil {
		return "", err
	}
	resolvedLocale, err := validateLocale(locale)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, versionDirName, resolvedVersion, resolvedLocale+".json"), nil
}

// BuildWritePlans creates deterministic write plans for canonical metadata files.
func BuildWritePlans(
	rootDir string,
	appInfoLocalizations map[string]AppInfoLocalization,
	versionLocalizations map[string]map[string]VersionLocalization,
) ([]WritePlan, error) {
	plans := make([]WritePlan, 0)

	appInfoLocales := sortedKeys(appInfoLocalizations)
	for _, locale := range appInfoLocales {
		loc := NormalizeAppInfoLocalization(appInfoLocalizations[locale])
		if !hasAppInfoContent(loc) {
			continue
		}
		path, err := AppInfoLocalizationFilePath(rootDir, locale)
		if err != nil {
			return nil, err
		}
		data, err := EncodeAppInfoLocalization(loc)
		if err != nil {
			return nil, err
		}
		plans = append(plans, WritePlan{Path: path, Contents: data})
	}

	versions := sortedKeys(versionLocalizations)
	for _, version := range versions {
		locales := sortedKeys(versionLocalizations[version])
		for _, locale := range locales {
			loc := NormalizeVersionLocalization(versionLocalizations[version][locale])
			if !hasVersionContent(loc) {
				continue
			}
			path, err := VersionLocalizationFilePath(rootDir, version, locale)
			if err != nil {
				return nil, err
			}
			data, err := EncodeVersionLocalization(loc)
			if err != nil {
				return nil, err
			}
			plans = append(plans, WritePlan{Path: path, Contents: data})
		}
	}

	sort.Slice(plans, func(i, j int) bool {
		return plans[i].Path < plans[j].Path
	})
	return plans, nil
}

// ApplyWritePlans writes plans in deterministic order.
func ApplyWritePlans(plans []WritePlan) error {
	sortedPlans := append([]WritePlan(nil), plans...)
	sort.Slice(sortedPlans, func(i, j int) bool {
		return sortedPlans[i].Path < sortedPlans[j].Path
	})
	for _, plan := range sortedPlans {
		if err := writeFileNoFollow(plan.Path, plan.Contents); err != nil {
			return err
		}
	}
	return nil
}

func decodeStrictJSON(data []byte, target any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("trailing data")
	}
	return nil
}

func encodeCanonicalJSON(value any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

func readFileNoFollow(path string) ([]byte, error) {
	file, err := shared.OpenExistingNoFollow(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

func writeFileNoFollow(path string, data []byte) error {
	_, err := shared.WriteFileNoSymlinkOverwrite(
		path,
		bytes.NewReader(data),
		0o644,
		".asc-metadata-*.tmp",
		".asc-metadata-*.bak",
	)
	return err
}

func validateRootDir(rootDir string) (string, error) {
	trimmed := strings.TrimSpace(rootDir)
	if trimmed == "" {
		return "", fmt.Errorf("metadata root directory is required")
	}
	return trimmed, nil
}

func validateLocale(locale string) (string, error) {
	resolved, err := validatePathSegment("locale", locale)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(resolved, DefaultLocale) {
		return DefaultLocale, nil
	}

	normalizedCode := normalizeMetadataLocaleCode(resolved)
	if canonical, ok := supportedMetadataLocaleByFold[strings.ToLower(normalizedCode)]; ok {
		return canonical, nil
	}

	aliasKey := normalizeMetadataLocaleAliasKey(resolved)
	if canonical, ok := metadataLocaleAliases[aliasKey]; ok {
		return canonical, nil
	}

	if candidates, ok := metadataLocaleCandidatesByLanguageName[aliasKey]; ok {
		if len(candidates) == 1 {
			return candidates[0], nil
		}
		return "", fmt.Errorf("ambiguous locale %q; use one of: %s", resolved, strings.Join(candidates, ", "))
	}

	root := metadataLocaleRoot(normalizedCode)
	if candidates, ok := metadataLocaleCandidatesByRoot[root]; ok && root != "" {
		if strings.EqualFold(normalizedCode, root) {
			if len(candidates) == 1 {
				return candidates[0], nil
			}
			return "", fmt.Errorf("ambiguous locale %q; use one of: %s", resolved, strings.Join(candidates, ", "))
		}
		if len(candidates) > 0 {
			return "", fmt.Errorf("unsupported locale %q; use one of: %s", resolved, strings.Join(candidates, ", "))
		}
	}

	if len(normalizedCode) > 20 || !localePattern.MatchString(normalizedCode) {
		if suggestions := metadataLocaleSuggestions(aliasKey); len(suggestions) > 0 {
			return "", fmt.Errorf("invalid locale %q; did you mean: %s", resolved, strings.Join(suggestions, ", "))
		}
		return "", fmt.Errorf("invalid locale %q", resolved)
	}

	if suggestions := metadataLocaleSuggestions(normalizedCode); len(suggestions) > 0 {
		return "", fmt.Errorf("unsupported locale %q; did you mean: %s", resolved, strings.Join(suggestions, ", "))
	}
	return "", fmt.Errorf("unsupported locale %q", resolved)
}

func normalizeMetadataLocaleCode(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), "_", "-")
}

func normalizeMetadataLocaleAliasKey(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastHyphen := false
	for _, r := range normalized {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastHyphen = false
		case r == '-' || r == '_' || r == ' ' || r == '(' || r == ')' || r == '.':
			if !lastHyphen && b.Len() > 0 {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func metadataLocaleRoot(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	parts := strings.SplitN(strings.ToLower(trimmed), "-", 2)
	return parts[0]
}

func metadataLocaleSuggestions(value string) []string {
	suggestions := suggest.Commands(strings.ToLower(strings.TrimSpace(value)), supportedMetadataLocales)
	if len(suggestions) == 0 {
		return nil
	}
	result := make([]string, 0, len(suggestions))
	seen := make(map[string]struct{}, len(suggestions))
	for _, item := range suggestions {
		canonical, ok := supportedMetadataLocaleByFold[strings.ToLower(item)]
		if !ok {
			continue
		}
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		result = append(result, canonical)
	}
	return result
}

func validatePathSegment(label, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	if trimmed == "." || trimmed == ".." {
		return "", fmt.Errorf("invalid %s %q", label, trimmed)
	}
	if strings.Contains(trimmed, "/") || strings.Contains(trimmed, `\`) {
		return "", fmt.Errorf("invalid %s %q", label, trimmed)
	}
	return trimmed, nil
}

func recordCanonicalLocaleFile(seen map[string]string, canonicalLocale string, fileName string) error {
	if seen == nil {
		return nil
	}
	if prior, exists := seen[canonicalLocale]; exists && prior != fileName {
		return fmt.Errorf(
			"duplicate canonical locale %q from files %q and %q",
			canonicalLocale,
			prior,
			fileName,
		)
	}
	seen[canonicalLocale] = fileName
	return nil
}

func hasAppInfoContent(loc AppInfoLocalization) bool {
	return loc.Name != "" ||
		loc.Subtitle != "" ||
		loc.PrivacyPolicyURL != "" ||
		loc.PrivacyChoicesURL != "" ||
		loc.PrivacyPolicyText != ""
}

func hasVersionContent(loc VersionLocalization) bool {
	return loc.Description != "" ||
		loc.Keywords != "" ||
		loc.MarketingURL != "" ||
		loc.PromotionalText != "" ||
		loc.SupportURL != "" ||
		loc.WhatsNew != ""
}

func sortedKeys[T any](items map[string]T) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
