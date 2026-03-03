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
	defer func() { _ = file.Close() }()
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
	if len(resolved) > 20 || !localePattern.MatchString(resolved) {
		return "", fmt.Errorf("invalid locale %q", resolved)
	}
	canonical, ok := supportedMetadataLocaleByFold[strings.ToLower(resolved)]
	if !ok {
		return "", fmt.Errorf("unsupported locale %q", resolved)
	}
	return canonical, nil
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
