package validation

import (
	"fmt"
	"strconv"
	"strings"
)

func requiredFieldChecks(primaryLocale string, versionString string, versionState string, versionLocs []VersionLocalization, appInfoLocs []AppInfoLocalization) []CheckResult {
	var checks []CheckResult

	normalizedState := strings.ToUpper(strings.TrimSpace(versionState))
	if normalizedState != "" && !isEditableVersionState(normalizedState) {
		checks = append(checks, CheckResult{
			ID:          "version.state.editable",
			Severity:    SeverityError,
			Field:       "appVersionState",
			Message:     fmt.Sprintf("version is in non-editable state %q", versionState),
			Remediation: "Use an editable version state before submission",
		})
	}

	if len(versionLocs) == 0 {
		checks = append(checks, CheckResult{
			ID:          "metadata.required.localizations",
			Severity:    SeverityError,
			Message:     "no version localizations found",
			Remediation: "Add at least one App Store version localization",
		})
	}

	if strings.TrimSpace(primaryLocale) != "" {
		if !hasLocale(versionLocs, primaryLocale) {
			checks = append(checks, CheckResult{
				ID:          "metadata.required.primary_locale",
				Severity:    SeverityError,
				Locale:      primaryLocale,
				Message:     "primary locale is missing from version localizations",
				Remediation: "Add a version localization for the primary locale",
			})
		}
		if !hasLocaleAppInfo(appInfoLocs, primaryLocale) {
			checks = append(checks, CheckResult{
				ID:          "metadata.required.primary_locale",
				Severity:    SeverityError,
				Locale:      primaryLocale,
				Message:     "primary locale is missing from app info localizations",
				Remediation: "Add an app info localization for the primary locale",
			})
		}
	}

	// Apple doesn't support a "What's New" section on initial App Store releases.
	// In practice, these initial releases commonly use versionString "1.0" (or
	// equivalent like "1.0.0"), and attempting to set `whatsNew` is rejected by
	// the API. Avoid warning users for an uneditable field.
	skipWhatsNew := isInitialReleaseVersionString(versionString)

	for _, loc := range versionLocs {
		if strings.TrimSpace(loc.Description) == "" {
			checks = append(checks, CheckResult{
				ID:           "metadata.required.description",
				Severity:     SeverityError,
				Locale:       loc.Locale,
				Field:        "description",
				ResourceType: "appStoreVersionLocalization",
				ResourceID:   loc.ID,
				Message:      "description is required",
				Remediation:  "Provide a description for this localization",
			})
		}
		if strings.TrimSpace(loc.Keywords) == "" {
			checks = append(checks, CheckResult{
				ID:           "metadata.required.keywords",
				Severity:     SeverityError,
				Locale:       loc.Locale,
				Field:        "keywords",
				ResourceType: "appStoreVersionLocalization",
				ResourceID:   loc.ID,
				Message:      "keywords are required",
				Remediation:  "Provide keywords for this localization",
			})
		}
		if strings.TrimSpace(loc.SupportURL) == "" {
			checks = append(checks, CheckResult{
				ID:           "metadata.required.support_url",
				Severity:     SeverityError,
				Locale:       loc.Locale,
				Field:        "supportUrl",
				ResourceType: "appStoreVersionLocalization",
				ResourceID:   loc.ID,
				Message:      "support URL is required",
				Remediation:  "Provide a support URL for this localization",
			})
		}
		if !skipWhatsNew && strings.TrimSpace(loc.WhatsNew) == "" {
			checks = append(checks, CheckResult{
				ID:           "metadata.required.whats_new",
				Severity:     SeverityWarning,
				Locale:       loc.Locale,
				Field:        "whatsNew",
				ResourceType: "appStoreVersionLocalization",
				ResourceID:   loc.ID,
				Message:      "what's new is empty",
				Remediation:  "Provide release notes for this localization",
			})
		}
	}

	for _, loc := range appInfoLocs {
		if strings.TrimSpace(loc.Name) == "" {
			checks = append(checks, CheckResult{
				ID:           "metadata.required.name",
				Severity:     SeverityError,
				Locale:       loc.Locale,
				Field:        "name",
				ResourceType: "appInfoLocalization",
				ResourceID:   loc.ID,
				Message:      "name is required",
				Remediation:  "Provide an app name for this localization",
			})
		}
		if strings.TrimSpace(loc.Subtitle) == "" {
			checks = append(checks, CheckResult{
				ID:           "metadata.required.subtitle",
				Severity:     SeverityWarning,
				Locale:       loc.Locale,
				Field:        "subtitle",
				ResourceType: "appInfoLocalization",
				ResourceID:   loc.ID,
				Message:      "subtitle is empty",
				Remediation:  "Provide a subtitle for this localization",
			})
		}
		if strings.TrimSpace(loc.PrivacyPolicyURL) == "" {
			checks = append(checks, CheckResult{
				ID:           "metadata.recommended.privacy_policy_url",
				Severity:     SeverityWarning,
				Locale:       loc.Locale,
				Field:        "privacyPolicyUrl",
				ResourceType: "appInfoLocalization",
				ResourceID:   loc.ID,
				Message:      "privacy policy URL is empty",
				Remediation:  "Provide a privacy policy URL for this localization",
			})
		}
	}

	if len(appInfoLocs) == 0 {
		checks = append(checks, CheckResult{
			ID:          "metadata.required.app_info_localizations",
			Severity:    SeverityWarning,
			Message:     "no app info localizations found",
			Remediation: "Add app info localizations with name/subtitle",
		})
	}

	return checks
}

func isEditableVersionState(state string) bool {
	switch state {
	case "PREPARE_FOR_SUBMISSION",
		"DEVELOPER_REJECTED",
		"REJECTED",
		"METADATA_REJECTED",
		"INVALID_BINARY",
		"DEVELOPER_REMOVED_FROM_SALE":
		return true
	default:
		return false
	}
}

func hasLocale(localizations []VersionLocalization, locale string) bool {
	for _, loc := range localizations {
		if loc.Locale == locale {
			return true
		}
	}
	return false
}

func hasLocaleAppInfo(localizations []AppInfoLocalization, locale string) bool {
	for _, loc := range localizations {
		if loc.Locale == locale {
			return true
		}
	}
	return false
}

func isInitialReleaseVersionString(versionString string) bool {
	trimmed := strings.TrimSpace(versionString)
	if trimmed == "" {
		return false
	}
	parts := strings.Split(trimmed, ".")
	// Require at least major.minor (e.g. "1.0") to avoid guessing.
	if len(parts) < 2 {
		return false
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 0 {
		return false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil || minor < 0 {
		return false
	}
	if major != 1 || minor != 0 {
		return false
	}

	// Treat "1.0", "1.0.0", "1.0.0.0", ... as initial releases.
	for _, part := range parts[2:] {
		if part == "" {
			return false
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return false
		}
		if n != 0 {
			return false
		}
	}
	return true
}
