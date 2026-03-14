package shared

import (
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// SubmitReadinessIssue describes submission-blocking missing fields for a locale.
type SubmitReadinessIssue struct {
	Locale        string
	MissingFields []string
}

// SubmitReadinessOptions controls optional submit-readiness checks.
type SubmitReadinessOptions struct {
	// RequireWhatsNew enables whatsNew validation. This should be set for
	// app updates (when a READY_FOR_SALE version already exists) because
	// App Store Connect requires whatsNew for every locale on updates.
	RequireWhatsNew bool
}

// MissingSubmitRequiredLocalizationFields returns missing metadata fields that
// block App Store submission for a version localization.
func MissingSubmitRequiredLocalizationFields(attrs asc.AppStoreVersionLocalizationAttributes) []string {
	return MissingSubmitRequiredLocalizationFieldsWithOptions(attrs, SubmitReadinessOptions{})
}

// MissingSubmitRequiredLocalizationFieldsWithOptions returns missing metadata
// fields that block App Store submission, with configurable checks.
func MissingSubmitRequiredLocalizationFieldsWithOptions(attrs asc.AppStoreVersionLocalizationAttributes, opts SubmitReadinessOptions) []string {
	missing := make([]string, 0, 4)
	if strings.TrimSpace(attrs.Description) == "" {
		missing = append(missing, "description")
	}
	if strings.TrimSpace(attrs.Keywords) == "" {
		missing = append(missing, "keywords")
	}
	if strings.TrimSpace(attrs.SupportURL) == "" {
		missing = append(missing, "supportUrl")
	}
	if opts.RequireWhatsNew && strings.TrimSpace(attrs.WhatsNew) == "" {
		missing = append(missing, "whatsNew")
	}
	return missing
}

// SubmitReadinessIssuesByLocale evaluates all localizations and returns
// per-locale missing submit-required fields.
func SubmitReadinessIssuesByLocale(localizations []asc.Resource[asc.AppStoreVersionLocalizationAttributes]) []SubmitReadinessIssue {
	return SubmitReadinessIssuesByLocaleWithOptions(localizations, SubmitReadinessOptions{})
}

// SubmitReadinessIssuesByLocaleWithOptions evaluates all localizations with
// configurable checks and returns per-locale missing submit-required fields.
func SubmitReadinessIssuesByLocaleWithOptions(localizations []asc.Resource[asc.AppStoreVersionLocalizationAttributes], opts SubmitReadinessOptions) []SubmitReadinessIssue {
	issues := make([]SubmitReadinessIssue, 0, len(localizations))
	for _, localization := range localizations {
		missing := MissingSubmitRequiredLocalizationFieldsWithOptions(localization.Attributes, opts)
		if len(missing) == 0 {
			continue
		}

		locale := strings.TrimSpace(localization.Attributes.Locale)
		if locale == "" {
			locale = "<unknown>"
		}
		issues = append(issues, SubmitReadinessIssue{
			Locale:        locale,
			MissingFields: missing,
		})
	}

	sort.SliceStable(issues, func(i, j int) bool {
		return issues[i].Locale < issues[j].Locale
	})
	return issues
}
