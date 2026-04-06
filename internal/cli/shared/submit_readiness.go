package shared

import (
	"context"
	"fmt"
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

// AppUpdateRequiresWhatsNew returns true when the target app/platform has a
// previously released App Store version, which means whatsNew is required on
// every localization for update submissions.
func AppUpdateRequiresWhatsNew(ctx context.Context, client *asc.Client, appID, platform string) (bool, error) {
	opts := []asc.AppStoreVersionsOption{
		asc.WithAppStoreVersionsStates([]string{
			"READY_FOR_SALE",
			"DEVELOPER_REMOVED_FROM_SALE",
			"REMOVED_FROM_SALE",
		}),
		asc.WithAppStoreVersionsLimit(1),
	}
	if strings.TrimSpace(platform) != "" {
		opts = append(opts, asc.WithAppStoreVersionsPlatforms([]string{platform}))
	}

	versions, err := client.GetAppStoreVersions(ctx, appID, opts...)
	if err != nil {
		return false, err
	}
	return len(versions.Data) > 0, nil
}

// SubmitIncompleteLocaleWarning returns a user-facing warning when a locale is
// missing submit-required metadata fields.
func SubmitIncompleteLocaleWarning(locale string, attrs asc.AppStoreVersionLocalizationAttributes) string {
	return SubmitIncompleteLocaleWarningWithOptions(locale, attrs, SubmitReadinessOptions{})
}

// SubmitIncompleteLocaleWarningWithOptions returns a user-facing warning when a
// locale is missing submit-required metadata fields under the provided rules.
func SubmitIncompleteLocaleWarningWithOptions(locale string, attrs asc.AppStoreVersionLocalizationAttributes, opts SubmitReadinessOptions) string {
	missing := MissingSubmitRequiredLocalizationFieldsWithOptions(attrs, opts)
	if len(missing) == 0 {
		return ""
	}

	trimmedLocale := strings.TrimSpace(locale)
	if trimmedLocale == "" {
		trimmedLocale = "<unknown>"
	}

	return fmt.Sprintf(
		"Warning: locale %s is missing submit-required fields: %s. This may block `asc publish appstore --submit`.\n",
		trimmedLocale,
		strings.Join(missing, ", "),
	)
}
