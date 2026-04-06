package shared

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/suggest"
)

// AppStoreLocalizationLocale describes one known App Store localization locale.
type AppStoreLocalizationLocale struct {
	Code             string `json:"code"`
	Name             string `json:"name"`
	SupportsMetadata bool   `json:"supportsMetadata"`
}

var appStoreLocalizationLocalePattern = regexp.MustCompile(`^[a-zA-Z]{2,3}(-([a-zA-Z]{2}|[a-zA-Z]{4}|[0-9]{3}))?$`)

// Source notes:
//   - The 39 metadata-capable locales come from Apple's locale shortcode documentation.
//   - The additional version-localization locales are present in current App Store Connect
//     Help and have been observed in live App Store version localizations.
var appStoreLocalizationCatalog = []AppStoreLocalizationLocale{
	{Code: "ar-SA", Name: "Arabic", SupportsMetadata: true},
	{Code: "bn-BD", Name: "Bangla", SupportsMetadata: false},
	{Code: "ca", Name: "Catalan", SupportsMetadata: true},
	{Code: "cs", Name: "Czech", SupportsMetadata: true},
	{Code: "da", Name: "Danish", SupportsMetadata: true},
	{Code: "de-DE", Name: "German", SupportsMetadata: true},
	{Code: "el", Name: "Greek", SupportsMetadata: true},
	{Code: "en-AU", Name: "English (Australia)", SupportsMetadata: true},
	{Code: "en-CA", Name: "English (Canada)", SupportsMetadata: true},
	{Code: "en-GB", Name: "English (U.K.)", SupportsMetadata: true},
	{Code: "en-US", Name: "English (U.S.)", SupportsMetadata: true},
	{Code: "es-ES", Name: "Spanish (Spain)", SupportsMetadata: true},
	{Code: "es-MX", Name: "Spanish (Mexico)", SupportsMetadata: true},
	{Code: "fi", Name: "Finnish", SupportsMetadata: true},
	{Code: "fr-CA", Name: "French (Canada)", SupportsMetadata: true},
	{Code: "fr-FR", Name: "French", SupportsMetadata: true},
	{Code: "gu-IN", Name: "Gujarati", SupportsMetadata: false},
	{Code: "he", Name: "Hebrew", SupportsMetadata: true},
	{Code: "hi", Name: "Hindi", SupportsMetadata: true},
	{Code: "hr", Name: "Croatian", SupportsMetadata: true},
	{Code: "hu", Name: "Hungarian", SupportsMetadata: true},
	{Code: "id", Name: "Indonesian", SupportsMetadata: true},
	{Code: "it", Name: "Italian", SupportsMetadata: true},
	{Code: "ja", Name: "Japanese", SupportsMetadata: true},
	{Code: "kn-IN", Name: "Kannada", SupportsMetadata: false},
	{Code: "ko", Name: "Korean", SupportsMetadata: true},
	{Code: "ml-IN", Name: "Malayalam", SupportsMetadata: false},
	{Code: "mr-IN", Name: "Marathi", SupportsMetadata: false},
	{Code: "ms", Name: "Malay", SupportsMetadata: true},
	{Code: "nl-NL", Name: "Dutch", SupportsMetadata: true},
	{Code: "no", Name: "Norwegian", SupportsMetadata: true},
	{Code: "or-IN", Name: "Odia", SupportsMetadata: false},
	{Code: "pa-IN", Name: "Punjabi", SupportsMetadata: false},
	{Code: "pl", Name: "Polish", SupportsMetadata: true},
	{Code: "pt-BR", Name: "Portuguese (Brazil)", SupportsMetadata: true},
	{Code: "pt-PT", Name: "Portuguese (Portugal)", SupportsMetadata: true},
	{Code: "ro", Name: "Romanian", SupportsMetadata: true},
	{Code: "ru", Name: "Russian", SupportsMetadata: true},
	{Code: "sk", Name: "Slovak", SupportsMetadata: true},
	{Code: "sl-SI", Name: "Slovenian", SupportsMetadata: false},
	{Code: "sv", Name: "Swedish", SupportsMetadata: true},
	{Code: "ta-IN", Name: "Tamil", SupportsMetadata: false},
	{Code: "te-IN", Name: "Telugu", SupportsMetadata: false},
	{Code: "th", Name: "Thai", SupportsMetadata: true},
	{Code: "tr", Name: "Turkish", SupportsMetadata: true},
	{Code: "uk", Name: "Ukrainian", SupportsMetadata: true},
	{Code: "ur-PK", Name: "Urdu", SupportsMetadata: false},
	{Code: "vi", Name: "Vietnamese", SupportsMetadata: true},
	{Code: "zh-Hans", Name: "Chinese (Simplified)", SupportsMetadata: true},
	{Code: "zh-Hant", Name: "Chinese (Traditional)", SupportsMetadata: true},
}

var appStoreLocalizationByFold = func() map[string]AppStoreLocalizationLocale {
	result := make(map[string]AppStoreLocalizationLocale, len(appStoreLocalizationCatalog))
	for _, locale := range appStoreLocalizationCatalog {
		result[strings.ToLower(locale.Code)] = locale
	}
	return result
}()

var appStoreLocalizationCodeByFold = func() map[string]string {
	result := make(map[string]string, len(appStoreLocalizationCatalog))
	for _, locale := range appStoreLocalizationCatalog {
		result[strings.ToLower(locale.Code)] = locale.Code
	}
	return result
}()

var supportedAppStoreLocalizationLocales = func() []string {
	result := make([]string, 0, len(appStoreLocalizationCatalog))
	for _, locale := range appStoreLocalizationCatalog {
		result = append(result, locale.Code)
	}
	return result
}()

var supportedMetadataLocales = func() []string {
	result := make([]string, 0, len(appStoreLocalizationCatalog))
	for _, locale := range appStoreLocalizationCatalog {
		if locale.SupportsMetadata {
			result = append(result, locale.Code)
		}
	}
	return result
}()

var appStoreLocalizationCandidatesByRoot = func() map[string][]string {
	result := make(map[string][]string)
	for _, locale := range supportedAppStoreLocalizationLocales {
		root := LocaleRoot(locale)
		result[root] = append(result[root], locale)
	}
	for root := range result {
		sort.Strings(result[root])
	}
	return result
}()

// AppStoreLocalizationCatalog returns the known App Store localization catalog.
func AppStoreLocalizationCatalog() []AppStoreLocalizationLocale {
	return slices.Clone(appStoreLocalizationCatalog)
}

// SupportedMetadataLocales returns the metadata-compatible subset of the shared App Store localization catalog.
func SupportedMetadataLocales() []string {
	return slices.Clone(supportedMetadataLocales)
}

// NormalizeAppStoreLocalizationLocale validates locale syntax and canonicalizes known codes.
// Unknown but well-formed locale codes are preserved for forward compatibility.
func NormalizeAppStoreLocalizationLocale(value string) (string, error) {
	normalized, _, err := resolveAppStoreLocalizationLocale(value)
	return normalized, err
}

func resolveAppStoreLocalizationLocale(value string) (string, bool, error) {
	normalized := NormalizeLocaleCode(value)
	if normalized == "" || !appStoreLocalizationLocalePattern.MatchString(normalized) {
		return "", false, fmt.Errorf("invalid locale %q: must match pattern like en or en-US", value)
	}
	if locale, ok := appStoreLocalizationByFold[strings.ToLower(normalized)]; ok {
		return locale.Code, true, nil
	}
	return normalized, false, nil
}

// CanonicalizeAppStoreLocalizationLocale validates a locale against the known catalog
// and returns a helpful suggestion when the input is well-formed but unsupported.
func CanonicalizeAppStoreLocalizationLocale(value string) (string, error) {
	normalized, known, err := resolveAppStoreLocalizationLocale(value)
	if err != nil {
		return "", err
	}
	if known {
		return normalized, nil
	}

	rootCandidates := appStoreLocalizationCandidatesByRoot[LocaleRoot(normalized)]
	switch len(rootCandidates) {
	case 0:
		if suggestions := SuggestCanonicalLocaleCodes(normalized, supportedAppStoreLocalizationLocales, appStoreLocalizationCodeByFold); len(suggestions) > 0 {
			return "", fmt.Errorf("unsupported locale %q; did you mean: %s", normalized, strings.Join(suggestions, ", "))
		}
		return "", fmt.Errorf("unsupported locale %q", normalized)
	case 1:
		return "", fmt.Errorf("unsupported locale %q; did you mean: %s", normalized, rootCandidates[0])
	default:
		return "", fmt.Errorf("unsupported locale %q; use one of: %s", normalized, strings.Join(rootCandidates, ", "))
	}
}

// NormalizeLocaleCode trims whitespace and canonicalizes separators for locale codes.
func NormalizeLocaleCode(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), "_", "-")
}

// LocaleRoot returns the lowercased language root for a locale code.
func LocaleRoot(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	parts := strings.SplitN(strings.ToLower(trimmed), "-", 2)
	return parts[0]
}

// SuggestCanonicalLocaleCodes returns deduplicated canonical locale suggestions for fuzzy input.
func SuggestCanonicalLocaleCodes(value string, supported []string, canonicalByFold map[string]string) []string {
	suggestions := suggest.Commands(strings.ToLower(strings.TrimSpace(value)), supported)
	if len(suggestions) == 0 {
		return nil
	}

	result := make([]string, 0, len(suggestions))
	seen := make(map[string]struct{}, len(suggestions))
	for _, item := range suggestions {
		canonical, ok := canonicalByFold[strings.ToLower(item)]
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
