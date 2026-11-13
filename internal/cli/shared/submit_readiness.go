package shared

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// SubmitReadinessIssue describes submission-blocking missing fields for a locale.
type SubmitReadinessIssue struct {
	Locale        string
	MissingFields []string
}

const (
	SubmitReadinessCreateModePlanned = "planned"
	SubmitReadinessCreateModeApplied = "applied"
)

// SubmitReadinessCreateWarning describes create-scope submission risk for one locale.
type SubmitReadinessCreateWarning struct {
	Locale        string
	Mode          string
	MissingFields []string
}

// SubmitReadinessOptions controls optional submit-readiness checks.
type SubmitReadinessOptions struct {
	// RequireWhatsNew enables whatsNew validation. This should be set for
	// app updates (when a released version already exists) because
	// App Store Connect requires whatsNew for every locale on updates.
	RequireWhatsNew bool
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

// SubmitReadinessCreateWarningForLocaleWithOptions returns a create warning when
// the provided attributes would leave the locale incomplete for submission.
func SubmitReadinessCreateWarningForLocaleWithOptions(locale string, attrs asc.AppStoreVersionLocalizationAttributes, mode string, opts SubmitReadinessOptions) (SubmitReadinessCreateWarning, bool) {
	missing := MissingSubmitRequiredLocalizationFieldsWithOptions(attrs, opts)
	if len(missing) == 0 {
		return SubmitReadinessCreateWarning{}, false
	}

	resolvedLocale := strings.TrimSpace(locale)
	if resolvedLocale == "" {
		resolvedLocale = strings.TrimSpace(attrs.Locale)
	}
	if resolvedLocale == "" {
		resolvedLocale = "<unknown>"
	}

	return SubmitReadinessCreateWarning{
		Locale:        resolvedLocale,
		Mode:          normalizeSubmitReadinessCreateMode(mode),
		MissingFields: append([]string(nil), missing...),
	}, true
}

// AppUpdateRequiresWhatsNew returns true when the target app/platform has a
// previously released App Store version, which means whatsNew is required on
// every localization for update submissions. A released version is one that
// is live under either state spelling, or one that was removed from sale
// (a legacy appStoreState-only condition).
func AppUpdateRequiresWhatsNew(ctx context.Context, client *asc.Client, appID, platform string) (bool, error) {
	filter := LiveAppStoreVersionStateFilter()
	filter.AppStoreStates = append(filter.AppStoreStates, "DEVELOPER_REMOVED_FROM_SALE", "REMOVED_FROM_SALE")
	return HasAppStoreVersionInStates(ctx, client, appID, platform, filter)
}

// ResolveSubmitReadinessOptionsForVersion resolves create-warning options for a
// version-localization workflow. Callers that only need advisory warnings
// should prefer the best-effort wrapper so auxiliary fetch failures do not
// block the primary mutation.
func ResolveSubmitReadinessOptionsForVersion(ctx context.Context, client *asc.Client, versionID, appID, platform string) (SubmitReadinessOptions, error) {
	if client == nil {
		return SubmitReadinessOptions{}, fmt.Errorf("client is required")
	}

	resolvedAppID := strings.TrimSpace(appID)
	resolvedPlatform := strings.TrimSpace(platform)
	if resolvedAppID == "" || resolvedPlatform == "" {
		trimmedVersionID := strings.TrimSpace(versionID)
		if trimmedVersionID == "" {
			return SubmitReadinessOptions{}, fmt.Errorf("version id is required when app or platform is unknown")
		}

		versionResp, err := client.GetAppStoreVersion(ctx, trimmedVersionID, asc.WithAppStoreVersionInclude([]string{"app"}))
		if err != nil {
			return SubmitReadinessOptions{}, err
		}
		if resolvedAppID == "" {
			relatedAppID, err := asc.AppStoreVersionAppID(versionResp)
			if err != nil {
				return SubmitReadinessOptions{}, err
			}
			resolvedAppID = strings.TrimSpace(relatedAppID)
		}
		if resolvedPlatform == "" {
			resolvedPlatform = strings.TrimSpace(string(versionResp.Data.Attributes.Platform))
		}
	}
	if resolvedAppID == "" || resolvedPlatform == "" {
		return SubmitReadinessOptions{}, fmt.Errorf("could not resolve app update context for version %q", strings.TrimSpace(versionID))
	}

	requireWhatsNew, err := AppUpdateRequiresWhatsNew(ctx, client, resolvedAppID, resolvedPlatform)
	if err != nil {
		return SubmitReadinessOptions{}, err
	}
	return SubmitReadinessOptions{RequireWhatsNew: requireWhatsNew}, nil
}

// ResolveSubmitReadinessOptionsForVersionBestEffort resolves create-warning
// options without failing the caller when advisory context fetches fail.
func ResolveSubmitReadinessOptionsForVersionBestEffort(ctx context.Context, client *asc.Client, versionID, appID, platform string) SubmitReadinessOptions {
	opts, err := ResolveSubmitReadinessOptionsForVersion(ctx, client, versionID, appID, platform)
	if err != nil {
		return SubmitReadinessOptions{}
	}
	return opts
}

// NormalizeSubmitReadinessCreateWarnings sorts and dedupes warnings so callers
// can emit them deterministically after their main output succeeds.
func NormalizeSubmitReadinessCreateWarnings(warnings []SubmitReadinessCreateWarning) []SubmitReadinessCreateWarning {
	if len(warnings) == 0 {
		return nil
	}

	type aggregate struct {
		index   int
		warning SubmitReadinessCreateWarning
	}

	order := make([]string, 0, len(warnings))
	aggregated := make(map[string]*aggregate, len(warnings))
	for _, warning := range warnings {
		normalized := normalizedSubmitReadinessCreateWarning(warning)
		key := submitReadinessCreateWarningKey(normalized)
		existing, ok := aggregated[key]
		if !ok {
			copyWarning := normalized
			aggregated[key] = &aggregate{
				index:   len(order),
				warning: copyWarning,
			}
			order = append(order, key)
			continue
		}
		existing.warning.MissingFields = mergeSubmitReadinessMissingFields(existing.warning.MissingFields, normalized.MissingFields)
	}

	normalized := make([]SubmitReadinessCreateWarning, 0, len(order))
	for _, key := range order {
		normalized = append(normalized, aggregated[key].warning)
	}

	sort.SliceStable(normalized, func(i, j int) bool {
		leftLocale := strings.ToLower(normalized[i].Locale)
		rightLocale := strings.ToLower(normalized[j].Locale)
		if leftLocale == rightLocale {
			leftMode := submitReadinessCreateModeSortKey(normalized[i].Mode)
			rightMode := submitReadinessCreateModeSortKey(normalized[j].Mode)
			if leftMode == rightMode {
				return normalized[i].Locale < normalized[j].Locale
			}
			return leftMode < rightMode
		}
		return leftLocale < rightLocale
	})

	return normalized
}

// FormatSubmitReadinessCreateWarning returns the user-facing warning line.
func FormatSubmitReadinessCreateWarning(warning SubmitReadinessCreateWarning) string {
	normalized := normalizedSubmitReadinessCreateWarning(warning)
	missingCSV := strings.Join(normalized.MissingFields, ", ")
	switch normalized.Mode {
	case SubmitReadinessCreateModeApplied:
		return fmt.Sprintf(
			"Warning: created locale %s now participates in submission validation and is still missing submit-required fields: %s. Fill the remaining metadata before submission.",
			normalized.Locale,
			missingCSV,
		)
	default:
		return fmt.Sprintf(
			"Warning: creating locale %s would make it participate in submission validation while still missing submit-required fields: %s. Fill the remaining metadata before submission.",
			normalized.Locale,
			missingCSV,
		)
	}
}

// PrintSubmitReadinessCreateWarnings emits normalized warnings to the writer.
func PrintSubmitReadinessCreateWarnings(w io.Writer, warnings []SubmitReadinessCreateWarning) error {
	for _, warning := range NormalizeSubmitReadinessCreateWarnings(warnings) {
		if _, err := fmt.Fprintln(w, FormatSubmitReadinessCreateWarning(warning)); err != nil {
			return err
		}
	}
	return nil
}

func normalizedSubmitReadinessCreateWarning(warning SubmitReadinessCreateWarning) SubmitReadinessCreateWarning {
	normalized := SubmitReadinessCreateWarning{
		Locale: strings.TrimSpace(warning.Locale),
		Mode:   normalizeSubmitReadinessCreateMode(warning.Mode),
	}
	if normalized.Locale == "" {
		normalized.Locale = "<unknown>"
	}
	normalized.MissingFields = mergeSubmitReadinessMissingFields(nil, warning.MissingFields)
	return normalized
}

func normalizeSubmitReadinessCreateMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case SubmitReadinessCreateModeApplied:
		return SubmitReadinessCreateModeApplied
	default:
		return SubmitReadinessCreateModePlanned
	}
}

func submitReadinessCreateWarningKey(warning SubmitReadinessCreateWarning) string {
	return strings.ToLower(strings.TrimSpace(warning.Locale)) + ":" + warning.Mode
}

func submitReadinessCreateModeSortKey(mode string) int {
	switch mode {
	case SubmitReadinessCreateModePlanned:
		return 0
	case SubmitReadinessCreateModeApplied:
		return 1
	default:
		return 2
	}
}

func mergeSubmitReadinessMissingFields(base []string, extra []string) []string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(base)+len(extra))
	merged := make([]string, 0, len(base)+len(extra))
	appendUnique := func(fields []string) {
		for _, field := range fields {
			field = strings.TrimSpace(field)
			if field == "" {
				continue
			}
			if _, ok := seen[field]; ok {
				continue
			}
			seen[field] = struct{}{}
			merged = append(merged, field)
		}
	}
	appendUnique(base)
	appendUnique(extra)
	sort.SliceStable(merged, func(i, j int) bool {
		left := submitReadinessMissingFieldSortKey(merged[i])
		right := submitReadinessMissingFieldSortKey(merged[j])
		if left == right {
			return merged[i] < merged[j]
		}
		return left < right
	})
	return merged
}

func submitReadinessMissingFieldSortKey(field string) int {
	switch field {
	case "description":
		return 0
	case "keywords":
		return 1
	case "supportUrl":
		return 2
	case "whatsNew":
		return 3
	default:
		return 4
	}
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
