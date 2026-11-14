package shared

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

const (
	LocalizationTypeVersion = "version"
	LocalizationTypeAppInfo = "app-info"
)

type localizationInputError struct {
	err error
}

func (e localizationInputError) Error() string { return e.err.Error() }
func (e localizationInputError) Unwrap() error { return e.err }

func newLocalizationInputError(err error) error {
	if err == nil {
		return nil
	}
	return localizationInputError{err: err}
}

// IsLocalizationInputError reports malformed locale/file input that should use
// CLI usage exit semantics rather than a runtime failure.
func IsLocalizationInputError(err error) bool {
	_, ok := errors.AsType[localizationInputError](err)
	return ok
}

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

// ValidateVersionLocalizationAttributes validates version localization attributes
// that the App Store Connect request contract makes locally knowable.
func ValidateVersionLocalizationAttributes(attrs asc.AppStoreVersionLocalizationAttributes) error {
	issues := validation.VersionLocalizationLengthIssues(validation.VersionLocalization{
		Description:     attrs.Description,
		Keywords:        attrs.Keywords,
		WhatsNew:        attrs.WhatsNew,
		PromotionalText: attrs.PromotionalText,
	})
	if len(issues) > 0 {
		issue := issues[0]
		if issue.Field == "keywords" {
			return fmt.Errorf("keywords exceed %d %s", issue.Limit, issue.Unit)
		}
		return fmt.Errorf("%s exceeds %d %s", issue.Field, issue.Limit, issue.Unit)
	}

	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "marketingUrl", value: attrs.MarketingURL},
		{name: "supportUrl", value: attrs.SupportURL},
	} {
		if field.value == "" {
			continue
		}
		// The request schema declares these attributes as URI-formatted strings.
		// Scheme-specific suitability remains a non-blocking validation concern.
		parsed, err := url.Parse(field.value)
		if err != nil || !parsed.IsAbs() || strings.ContainsAny(field.value, " \t\r\n") {
			return fmt.Errorf("%s must be a valid URI", field.name)
		}
	}
	return nil
}

// ValidateVersionLocalizationValues validates .strings keys and value limits for one locale.
func ValidateVersionLocalizationValues(locale string, values map[string]string) error {
	if err := ValidateVersionLocalizationKeys(locale, values); err != nil {
		return err
	}
	if err := ValidateVersionLocalizationAttributes(buildVersionLocalizationAttributes(locale, values, false)); err != nil {
		return fmt.Errorf("locale %q: %w", locale, err)
	}
	return nil
}

// ValidateVersionLocalizationValueSet validates value maps for all locales.
func ValidateVersionLocalizationValueSet(valuesByLocale map[string]map[string]string) error {
	locales := make([]string, 0, len(valuesByLocale))
	for locale := range valuesByLocale {
		locales = append(locales, locale)
	}
	sort.Strings(locales)

	for _, locale := range locales {
		if err := validateLocalizationValuesForBatch(locale, valuesByLocale[locale]); err != nil {
			return err
		}
		if err := ValidateVersionLocalizationValues(locale, valuesByLocale[locale]); err != nil {
			return err
		}
	}
	return nil
}

// ValidateVersionLocalizationAttributesByLocale validates attribute values for all locales.
func ValidateVersionLocalizationAttributesByLocale(valuesByLocale map[string]asc.AppStoreVersionLocalizationAttributes) error {
	locales := make([]string, 0, len(valuesByLocale))
	for locale := range valuesByLocale {
		locales = append(locales, locale)
	}
	sort.Strings(locales)

	for _, locale := range locales {
		if err := ValidateVersionLocalizationAttributes(valuesByLocale[locale]); err != nil {
			return fmt.Errorf("locale %q: %w", locale, err)
		}
	}
	return nil
}

// ValidateAppInfoLocalizationKeys validates .strings keys for an app-info localization locale.
func ValidateAppInfoLocalizationKeys(locale string, values map[string]string) error {
	return validateLocalizationKeys(locale, values, appInfoLocalizationAllowedKeys)
}

// ValidateAppInfoLocalizationValueSet validates every locale before auth or
// remote state is read.
func ValidateAppInfoLocalizationValueSet(valuesByLocale map[string]map[string]string) error {
	locales := make([]string, 0, len(valuesByLocale))
	for locale := range valuesByLocale {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	for _, locale := range locales {
		values := valuesByLocale[locale]
		if err := validateLocalizationValuesForBatch(locale, values); err != nil {
			return err
		}
		if err := ValidateAppInfoLocalizationKeys(locale, values); err != nil {
			return err
		}
	}
	return nil
}

type versionLocalizationClient interface {
	GetAppStoreVersionLocalizations(context.Context, string, ...asc.AppStoreVersionLocalizationsOption) (*asc.AppStoreVersionLocalizationsResponse, error)
	CreateAppStoreVersionLocalization(context.Context, string, asc.AppStoreVersionLocalizationAttributes) (*asc.AppStoreVersionLocalizationResponse, error)
	UpdateAppStoreVersionLocalizationFields(context.Context, string, map[string]string) (*asc.AppStoreVersionLocalizationResponse, error)
}

type appInfoLocalizationClient interface {
	GetAppInfoLocalizations(context.Context, string, ...asc.AppInfoLocalizationsOption) (*asc.AppInfoLocalizationsResponse, error)
	CreateAppInfoLocalization(context.Context, string, asc.AppInfoLocalizationAttributes) (*asc.AppInfoLocalizationResponse, error)
	UpdateAppInfoLocalizationFields(context.Context, string, map[string]string) (*asc.AppInfoLocalizationResponse, error)
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
		canonical, err := CanonicalizeAppStoreLocalizationLocale(locale)
		if err != nil {
			return nil, newLocalizationInputError(err)
		}
		if filter[canonical] {
			return nil, newLocalizationInputError(fmt.Errorf("duplicate canonical locale %q in --locale", canonical))
		}
		filter[canonical] = true
	}

	if !info.IsDir() {
		if len(locales) > 1 {
			return nil, newLocalizationInputError(fmt.Errorf("single file input only supports one locale"))
		}
		locale := ""
		if len(locales) == 1 {
			locale = locales[0]
		} else {
			locale = strings.TrimSuffix(filepath.Base(inputPath), ".strings")
			if locale == "" || locale == filepath.Base(inputPath) {
				return nil, newLocalizationInputError(fmt.Errorf("cannot infer locale from %q (use --locale)", inputPath))
			}
		}
		locale, err = CanonicalizeAppStoreLocalizationLocale(locale)
		if err != nil {
			return nil, newLocalizationInputError(err)
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
	sources := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".strings" {
			continue
		}
		rawLocale := strings.TrimSuffix(entry.Name(), ".strings")
		if rawLocale == "" {
			continue
		}
		locale, err := CanonicalizeAppStoreLocalizationLocale(rawLocale)
		if err != nil {
			return nil, newLocalizationInputError(fmt.Errorf("invalid localization file %q: %w", entry.Name(), err))
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
			return nil, newLocalizationInputError(fmt.Errorf("duplicate canonical locale %q from files %q and %q", locale, sources[locale], entry.Name()))
		}
		values[locale] = parsed
		sources[locale] = entry.Name()
	}

	if len(values) == 0 {
		return nil, newLocalizationInputError(fmt.Errorf("no .strings files found in %q", inputPath))
	}
	return values, nil
}

func UploadVersionLocalizations(ctx context.Context, client versionLocalizationClient, versionID string, valuesByLocale map[string]map[string]string, dryRun bool) ([]asc.LocalizationUploadLocaleResult, error) {
	results, _, err := UploadVersionLocalizationsWithWarnings(ctx, client, versionID, valuesByLocale, dryRun, SubmitReadinessOptions{})
	return results, err
}

func UploadVersionLocalizationsWithWarnings(ctx context.Context, client versionLocalizationClient, versionID string, valuesByLocale map[string]map[string]string, dryRun bool, submitOpts SubmitReadinessOptions) ([]asc.LocalizationUploadLocaleResult, []SubmitReadinessCreateWarning, error) {
	if err := ValidateVersionLocalizationValueSet(valuesByLocale); err != nil {
		return nil, nil, err
	}
	return UploadPrevalidatedVersionLocalizationsWithWarnings(ctx, client, versionID, valuesByLocale, dryRun, submitOpts)
}

// UploadPrevalidatedVersionLocalizationsWithWarnings uploads version localizations
// after the caller has already validated the input value set.
func UploadPrevalidatedVersionLocalizationsWithWarnings(ctx context.Context, client versionLocalizationClient, versionID string, valuesByLocale map[string]map[string]string, dryRun bool, submitOpts SubmitReadinessOptions) ([]asc.LocalizationUploadLocaleResult, []SubmitReadinessCreateWarning, error) {
	existing, err := fetchAllVersionLocalizations(ctx, client, versionID)
	if err != nil {
		return nil, nil, err
	}
	existingByLocale := make(map[string]string, len(existing))
	existingItems := make(map[string]asc.Resource[asc.AppStoreVersionLocalizationAttributes], len(existing))
	for _, item := range existing {
		if strings.TrimSpace(item.Attributes.Locale) == "" {
			continue
		}
		existingByLocale[item.Attributes.Locale] = item.ID
		existingItems[item.Attributes.Locale] = item
	}
	if err := validateVersionLocalizationCreates(valuesByLocale, existingByLocale); err != nil {
		return nil, nil, newLocalizationInputError(err)
	}

	mode := SubmitReadinessCreateModeApplied
	if dryRun {
		mode = SubmitReadinessCreateModePlanned
	}
	warnings := make([]SubmitReadinessCreateWarning, 0, len(valuesByLocale))

	results, err := uploadLocalizationValues(valuesByLocale, existingByLocale, dryRun, func(locale string, values map[string]string, existingID string) (asc.LocalizationUploadLocaleResult, error) {
		attributes := buildVersionLocalizationAttributes(locale, values, existingID == "")
		if existing, ok := existingItems[locale]; ok && versionLocalizationMatchesValues(existing.Attributes, values) {
			return asc.LocalizationUploadLocaleResult{Locale: locale, Action: "skip", LocalizationID: existing.ID}, nil
		}
		if existingID == "" {
			warning, hasWarning := SubmitReadinessCreateWarningForLocaleWithOptions(locale, attributes, mode, submitOpts)
			if dryRun {
				if hasWarning {
					warnings = append(warnings, warning)
				}
				return asc.LocalizationUploadLocaleResult{Locale: locale, Action: "create"}, nil
			}
			id, reconciled, err := runLocalizationMutationWithReadback(
				ctx,
				func(requestCtx context.Context) (string, error) {
					resp, err := client.CreateAppStoreVersionLocalization(requestCtx, versionID, attributes)
					if err != nil {
						return "", err
					}
					return resp.Data.ID, nil
				},
				func(requestCtx context.Context) (string, bool, error) {
					return findMatchingVersionLocalization(requestCtx, client, versionID, locale, values)
				},
			)
			if err != nil {
				return asc.LocalizationUploadLocaleResult{}, err
			}
			if hasWarning {
				warnings = append(warnings, warning)
			}
			action := "create"
			if reconciled {
				action = "reconcile"
			}
			return asc.LocalizationUploadLocaleResult{Locale: locale, Action: action, LocalizationID: id}, nil
		}
		if dryRun {
			return asc.LocalizationUploadLocaleResult{Locale: locale, Action: "update", LocalizationID: existingID}, nil
		}
		probeValues := cloneLocalizationValues(values)
		updateFields := cloneLocalizationValues(values)
		id, reconciled, err := runLocalizationMutationWithReadback(
			ctx,
			func(requestCtx context.Context) (string, error) {
				resp, err := client.UpdateAppStoreVersionLocalizationFields(requestCtx, existingID, updateFields)
				// A rejected whatsNew mutation is known not to have applied, so the
				// documented initial-release fallback remains safe to send immediately.
				if err != nil && strings.TrimSpace(updateFields["whatsNew"]) != "" && isWhatsNewUnsupportedError(err) {
					fmt.Fprintln(os.Stderr, "Warning: 'whatsNew' cannot be set for this version (initial releases have no What's New section). Retrying without it.")
					delete(updateFields, "whatsNew")
					delete(probeValues, "whatsNew")
					resp, err = client.UpdateAppStoreVersionLocalizationFields(requestCtx, existingID, updateFields)
				}
				if err != nil {
					return "", err
				}
				return resp.Data.ID, nil
			},
			func(requestCtx context.Context) (string, bool, error) {
				return findMatchingVersionLocalization(requestCtx, client, versionID, locale, probeValues)
			},
		)
		if err != nil {
			return asc.LocalizationUploadLocaleResult{}, err
		}
		action := "update"
		if reconciled {
			action = "reconcile"
		}
		return asc.LocalizationUploadLocaleResult{Locale: locale, Action: action, LocalizationID: id}, nil
	})
	return results, NormalizeSubmitReadinessCreateWarnings(warnings), err
}

func UploadAppInfoLocalizations(ctx context.Context, client appInfoLocalizationClient, appInfoID string, valuesByLocale map[string]map[string]string, dryRun bool) ([]asc.LocalizationUploadLocaleResult, error) {
	if err := ValidateAppInfoLocalizationValueSet(valuesByLocale); err != nil {
		return nil, err
	}

	existing, err := fetchAllAppInfoLocalizations(ctx, client, appInfoID)
	if err != nil {
		return nil, err
	}
	existingByLocale := make(map[string]string, len(existing))
	existingItems := make(map[string]asc.Resource[asc.AppInfoLocalizationAttributes], len(existing))
	for _, item := range existing {
		if strings.TrimSpace(item.Attributes.Locale) == "" {
			continue
		}
		existingByLocale[item.Attributes.Locale] = item.ID
		existingItems[item.Attributes.Locale] = item
	}
	if err := validateAppInfoLocalizationCreates(valuesByLocale, existingByLocale); err != nil {
		return nil, newLocalizationInputError(err)
	}

	return uploadLocalizationValues(valuesByLocale, existingByLocale, dryRun, func(locale string, values map[string]string, existingID string) (asc.LocalizationUploadLocaleResult, error) {
		attributes := buildAppInfoLocalizationAttributes(locale, values, existingID == "")
		if existing, ok := existingItems[locale]; ok && appInfoLocalizationMatchesValues(existing.Attributes, values) {
			return asc.LocalizationUploadLocaleResult{Locale: locale, Action: "skip", LocalizationID: existing.ID}, nil
		}
		if existingID == "" {
			if dryRun {
				return asc.LocalizationUploadLocaleResult{Locale: locale, Action: "create"}, nil
			}
			id, reconciled, err := runLocalizationMutationWithReadback(
				ctx,
				func(requestCtx context.Context) (string, error) {
					resp, err := client.CreateAppInfoLocalization(requestCtx, appInfoID, attributes)
					if err != nil {
						return "", err
					}
					return resp.Data.ID, nil
				},
				func(requestCtx context.Context) (string, bool, error) {
					return findMatchingAppInfoLocalization(requestCtx, client, appInfoID, locale, values)
				},
			)
			if err != nil {
				return asc.LocalizationUploadLocaleResult{}, err
			}
			action := "create"
			if reconciled {
				action = "reconcile"
			}
			return asc.LocalizationUploadLocaleResult{Locale: locale, Action: action, LocalizationID: id}, nil
		}
		if dryRun {
			return asc.LocalizationUploadLocaleResult{Locale: locale, Action: "update", LocalizationID: existingID}, nil
		}
		id, reconciled, err := runLocalizationMutationWithReadback(
			ctx,
			func(requestCtx context.Context) (string, error) {
				resp, err := client.UpdateAppInfoLocalizationFields(requestCtx, existingID, cloneLocalizationValues(values))
				if err != nil {
					return "", err
				}
				return resp.Data.ID, nil
			},
			func(requestCtx context.Context) (string, bool, error) {
				return findMatchingAppInfoLocalization(requestCtx, client, appInfoID, locale, values)
			},
		)
		if err != nil {
			return asc.LocalizationUploadLocaleResult{}, err
		}
		action := "update"
		if reconciled {
			action = "reconcile"
		}
		return asc.LocalizationUploadLocaleResult{Locale: locale, Action: action, LocalizationID: id}, nil
	})
}

func runLocalizationMutationWithReadback(
	ctx context.Context,
	mutate func(context.Context) (string, error),
	probe func(context.Context) (string, bool, error),
) (string, bool, error) {
	id, status, err := RunReconciledMutation(ctx, mutate, probe)
	return id, status == ReconciledMutationRecovered, err
}

func findMatchingVersionLocalization(ctx context.Context, client versionLocalizationClient, versionID, locale string, values map[string]string) (string, bool, error) {
	items, err := fetchAllVersionLocalizations(ctx, client, versionID)
	if err != nil {
		return "", false, err
	}
	for _, item := range items {
		if item.Attributes.Locale == locale && versionLocalizationMatchesValues(item.Attributes, values) {
			return item.ID, true, nil
		}
	}
	return "", false, nil
}

func findMatchingAppInfoLocalization(ctx context.Context, client appInfoLocalizationClient, appInfoID, locale string, values map[string]string) (string, bool, error) {
	items, err := fetchAllAppInfoLocalizations(ctx, client, appInfoID)
	if err != nil {
		return "", false, err
	}
	for _, item := range items {
		if item.Attributes.Locale == locale && appInfoLocalizationMatchesValues(item.Attributes, values) {
			return item.ID, true, nil
		}
	}
	return "", false, nil
}

func fetchAllVersionLocalizations(ctx context.Context, client versionLocalizationClient, versionID string) ([]asc.Resource[asc.AppStoreVersionLocalizationAttributes], error) {
	firstPage, err := RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (*asc.AppStoreVersionLocalizationsResponse, error) {
		return client.GetAppStoreVersionLocalizations(requestCtx, versionID, asc.Limit[asc.AppStoreVersionLocalizationsOption](200))
	})
	if err != nil {
		return nil, err
	}
	if firstPage == nil {
		return nil, fmt.Errorf("empty version localization response")
	}
	if strings.TrimSpace(firstPage.Links.Next) == "" {
		return firstPage.Data, nil
	}

	paginated, err := asc.PaginateAll(ctx, firstPage, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		nextPage, err := RetryReadWithFreshTimeout(pageCtx, func(requestCtx context.Context) (*asc.AppStoreVersionLocalizationsResponse, error) {
			return client.GetAppStoreVersionLocalizations(requestCtx, versionID, asc.NextURL[asc.AppStoreVersionLocalizationsOption](nextURL))
		})
		if err != nil {
			return nil, err
		}
		if nextPage == nil {
			return nil, fmt.Errorf("empty version localization response")
		}
		return nextPage, nil
	})
	if err != nil {
		return nil, err
	}
	allPages, ok := paginated.(*asc.AppStoreVersionLocalizationsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected version localization pagination response type")
	}
	return allPages.Data, nil
}

func fetchAllAppInfoLocalizations(ctx context.Context, client appInfoLocalizationClient, appInfoID string) ([]asc.Resource[asc.AppInfoLocalizationAttributes], error) {
	firstPage, err := RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (*asc.AppInfoLocalizationsResponse, error) {
		return client.GetAppInfoLocalizations(requestCtx, appInfoID, asc.Limit[asc.AppInfoLocalizationsOption](200))
	})
	if err != nil {
		return nil, err
	}
	if firstPage == nil {
		return nil, fmt.Errorf("empty app-info localization response")
	}
	if strings.TrimSpace(firstPage.Links.Next) == "" {
		return firstPage.Data, nil
	}

	paginated, err := asc.PaginateAll(ctx, firstPage, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		nextPage, err := RetryReadWithFreshTimeout(pageCtx, func(requestCtx context.Context) (*asc.AppInfoLocalizationsResponse, error) {
			return client.GetAppInfoLocalizations(requestCtx, appInfoID, asc.NextURL[asc.AppInfoLocalizationsOption](nextURL))
		})
		if err != nil {
			return nil, err
		}
		if nextPage == nil {
			return nil, fmt.Errorf("empty app-info localization response")
		}
		return nextPage, nil
	})
	if err != nil {
		return nil, err
	}
	allPages, ok := paginated.(*asc.AppInfoLocalizationsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected app-info localization pagination response type")
	}
	return allPages.Data, nil
}

func versionLocalizationMatchesValues(attrs asc.AppStoreVersionLocalizationAttributes, values map[string]string) bool {
	remote := mapVersionLocalizationStrings(attrs)
	return localizationValuesMatch(remote, values)
}

func appInfoLocalizationMatchesValues(attrs asc.AppInfoLocalizationAttributes, values map[string]string) bool {
	remote := mapAppInfoLocalizationStrings(attrs)
	return localizationValuesMatch(remote, values)
}

func localizationValuesMatch(remote, desired map[string]string) bool {
	for key, value := range desired {
		if remote[key] != value {
			return false
		}
	}
	return true
}

func cloneLocalizationValues(values map[string]string) map[string]string {
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
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

func uploadLocalizationValues(valuesByLocale map[string]map[string]string, existing map[string]string, dryRun bool, handler func(locale string, values map[string]string, existingID string) (asc.LocalizationUploadLocaleResult, error)) ([]asc.LocalizationUploadLocaleResult, error) {
	locales := make([]string, 0, len(valuesByLocale))
	for locale := range valuesByLocale {
		locales = append(locales, locale)
	}
	sort.Strings(locales)

	results := make([]asc.LocalizationUploadLocaleResult, 0, len(locales))
	batchErrors := make([]error, 0)
	for _, locale := range locales {
		values := valuesByLocale[locale]
		result, err := handler(locale, values, existing[locale])
		if err != nil {
			if result.Locale == "" {
				result.Locale = locale
			}
			if result.Action == "" {
				result.Action = localizationUploadMutationAction(existing[locale])
			}
			if result.LocalizationID == "" {
				result.LocalizationID = existing[locale]
			}
			result.Status = "failed"
			result.Error = err.Error()
			result.DesiredValues = cloneLocalizationValues(values)
			results = append(results, result)
			batchErrors = append(batchErrors, fmt.Errorf("locale %q: %w", locale, err))
			continue
		}
		if result.Status == "" {
			if dryRun {
				result.Status = "planned"
			} else {
				result.Status = "succeeded"
			}
		}
		results = append(results, result)
	}
	return results, errors.Join(batchErrors...)
}

func localizationUploadMutationAction(existingID string) string {
	if existingID == "" {
		return "create"
	}
	return "update"
}

func validateLocalizationValuesForBatch(locale string, values map[string]string) error {
	if len(values) == 0 {
		return fmt.Errorf("no localization values for locale %q", locale)
	}
	return nil
}

func validateVersionLocalizationCreates(valuesByLocale map[string]map[string]string, existing map[string]string) error {
	locales := make([]string, 0, len(valuesByLocale))
	for locale := range valuesByLocale {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	for _, locale := range locales {
		if existing[locale] == "" && !hasNonEmptyLocalizationValues(valuesByLocale[locale]) {
			return fmt.Errorf("cannot create version localization %q without a non-empty value", locale)
		}
	}
	return nil
}

func validateAppInfoLocalizationCreates(valuesByLocale map[string]map[string]string, existing map[string]string) error {
	locales := make([]string, 0, len(valuesByLocale))
	for locale := range valuesByLocale {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	for _, locale := range locales {
		if existing[locale] == "" && strings.TrimSpace(valuesByLocale[locale]["name"]) == "" {
			return fmt.Errorf("cannot create app-info localization %q without a non-empty name", locale)
		}
	}
	return nil
}

type localizationUploadFailureArtifact struct {
	SchemaVersion int                                  `json:"schemaVersion"`
	Command       string                               `json:"command"`
	Type          string                               `json:"type"`
	VersionID     string                               `json:"versionId,omitempty"`
	AppID         string                               `json:"appId,omitempty"`
	AppInfoID     string                               `json:"appInfoId,omitempty"`
	InputPath     string                               `json:"inputPath,omitempty"`
	Failed        int                                  `json:"failed"`
	GeneratedAt   string                               `json:"generatedAt"`
	Results       []asc.LocalizationUploadLocaleResult `json:"results"`
}

// FinalizeLocalizationUploadResult computes batch counts and writes a
// versioned retry artifact for failed locales. Artifact failures are retained
// in the result so callers can print the batch before returning an error.
func FinalizeLocalizationUploadResult(result *asc.LocalizationUploadResult, command string) {
	if result == nil {
		return
	}
	result.Total = len(result.Results)
	result.Succeeded = 0
	result.Failed = 0
	for _, item := range result.Results {
		if item.Status == "failed" || item.Error != "" {
			result.Failed++
			continue
		}
		result.Succeeded++
	}
	if result.Failed == 0 {
		return
	}
	path, err := writeLocalizationUploadFailureArtifact(result, command)
	if err != nil {
		result.FailureArtifactError = err.Error()
		return
	}
	result.FailureArtifactPath = path
}

// RenderLocalizationUploadResult renders both the batch summary and per-locale
// details so interactive output includes retry artifact information.
func RenderLocalizationUploadResult(result *asc.LocalizationUploadResult, markdown bool) error {
	if result == nil {
		return fmt.Errorf("localization upload result is nil")
	}
	render := asc.RenderTable
	if markdown {
		render = asc.RenderMarkdown
	}
	render(
		[]string{"Type", "Input Path", "Dry Run", "Total", "Succeeded", "Failed", "Failure Artifact", "Failure Artifact Error"},
		[][]string{{
			result.Type,
			result.InputPath,
			fmt.Sprintf("%t", result.DryRun),
			strconv.Itoa(result.Total),
			strconv.Itoa(result.Succeeded),
			strconv.Itoa(result.Failed),
			result.FailureArtifactPath,
			result.FailureArtifactError,
		}},
	)
	rows := make([][]string, 0, len(result.Results))
	for _, item := range result.Results {
		rows = append(rows, []string{item.Locale, item.Action, item.Status, item.LocalizationID, item.Error})
	}
	render([]string{"Locale", "Action", "Status", "Localization ID", "Error"}, rows)
	return nil
}

func writeLocalizationUploadFailureArtifact(result *asc.LocalizationUploadResult, command string) (string, error) {
	failures := make([]asc.LocalizationUploadLocaleResult, 0, result.Failed)
	for _, item := range result.Results {
		if item.Status == "failed" || item.Error != "" {
			failures = append(failures, item)
		}
	}
	artifact := localizationUploadFailureArtifact{
		SchemaVersion: 1,
		Command:       strings.TrimSpace(command),
		Type:          result.Type,
		VersionID:     result.VersionID,
		AppID:         result.AppID,
		AppInfoID:     result.AppInfoID,
		InputPath:     result.InputPath,
		Failed:        result.Failed,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Results:       failures,
	}
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(".asc", "reports", "localizations-upload", fmt.Sprintf("failures-%d.json", time.Now().UTC().UnixNano()))
	if _, err := WriteStreamToFile(path, bytes.NewReader(data)); err != nil {
		return "", err
	}
	return path, nil
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
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	values, err := parseStringsContent(string(data))
	if err != nil {
		return nil, newLocalizationInputError(err)
	}
	return values, nil
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
	defer file.Close()

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
