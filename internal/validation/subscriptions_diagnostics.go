package validation

import (
	"fmt"
	"sort"
	"strings"
)

// DiagnosticStatus captures the state of a subscription diagnostics row.
type DiagnosticStatus string

const (
	DiagnosticStatusYes        DiagnosticStatus = "yes"
	DiagnosticStatusNo         DiagnosticStatus = "no"
	DiagnosticStatusUnverified DiagnosticStatus = "unverified"
	DiagnosticStatusUnknown    DiagnosticStatus = "unknown"
	DiagnosticStatusOptional   DiagnosticStatus = "optional"
)

// SubscriptionDiagnosticRow captures a single diagnostics row for a subscription.
type SubscriptionDiagnosticRow struct {
	Key         string           `json:"key"`
	Label       string           `json:"label"`
	Status      DiagnosticStatus `json:"status"`
	Source      string           `json:"source"`
	Blocking    bool             `json:"blocking"`
	Evidence    string           `json:"evidence,omitempty"`
	Remediation string           `json:"remediation,omitempty"`
}

// SubscriptionDiagnostics is the detailed diagnostics output for a subscription.
type SubscriptionDiagnostics struct {
	SubscriptionID string                      `json:"subscriptionId"`
	Name           string                      `json:"name,omitempty"`
	ProductID      string                      `json:"productId,omitempty"`
	State          string                      `json:"state,omitempty"`
	Conclusion     string                      `json:"conclusion"`
	Summary        string                      `json:"summary,omitempty"`
	Rows           []SubscriptionDiagnosticRow `json:"rows"`
}

func buildSubscriptionDiagnostics(input SubscriptionsInput) []SubscriptionDiagnostics {
	diagnostics := make([]SubscriptionDiagnostics, 0, len(input.Subscriptions))
	appTerritories := sortedUniqueNonEmpty(input.AppAvailableTerritories)
	appTerritoryCount := input.AvailableTerritories
	if len(appTerritories) > 0 {
		appTerritoryCount = len(appTerritories)
	}

	for _, sub := range input.Subscriptions {
		if isRemovedMonetizationState(sub.State) {
			continue
		}
		if normalizeMonetizationState(sub.State) != "MISSING_METADATA" {
			continue
		}

		rows := []SubscriptionDiagnosticRow{
			buildGroupLocalizationsDiagnosticRow(sub),
			buildSubscriptionLocalizationsDiagnosticRow(sub),
			buildReviewScreenshotDiagnosticRow(sub),
			buildSubscriptionAvailabilityDiagnosticRow(sub),
			buildPriceRecordsDiagnosticRow(sub),
			buildSubscriptionAvailabilityCoverageDiagnosticRow(sub),
			buildAppAvailabilityCoverageDiagnosticRow(sub, appTerritories, appTerritoryCount, input.PricingCoverageSkipReason),
			buildPromotionalImageDiagnosticRow(sub),
			buildAppBuildDiagnosticRow(input.AppBuildCount, input.BuildCheckSkipped, input.BuildCheckSkipReason),
			buildOptionalOfferDiagnosticRow(
				"introductory_offer",
				"Introductory offer configured",
				sub.IntroductoryOfferCount,
				sub.IntroductoryOfferCheckSkipped,
				sub.IntroductoryOfferCheckReason,
				"Optional: configure an introductory offer or free trial with `asc subscriptions introductory-offers create` if this subscription should launch with one.",
			),
			buildOptionalOfferDiagnosticRow(
				"promotional_offers",
				"Promotional offers configured",
				sub.PromotionalOfferCount,
				sub.PromotionalOfferCheckSkipped,
				sub.PromotionalOfferCheckReason,
				"Optional: configure promotional offers with `asc subscriptions promotional-offers create` if you plan to use them.",
			),
			buildOptionalOfferDiagnosticRow(
				"win_back_offers",
				"Win-back offers configured",
				sub.WinBackOfferCount,
				sub.WinBackOfferCheckSkipped,
				sub.WinBackOfferCheckReason,
				"Optional: configure win-back offers with `asc subscriptions offers win-back create` if you plan to use them.",
			),
		}

		conclusion, summary := summarizeSubscriptionDiagnostics(sub, rows)
		diagnostics = append(diagnostics, SubscriptionDiagnostics{
			SubscriptionID: strings.TrimSpace(sub.ID),
			Name:           strings.TrimSpace(sub.Name),
			ProductID:      strings.TrimSpace(sub.ProductID),
			State:          normalizeMonetizationState(sub.State),
			Conclusion:     conclusion,
			Summary:        summary,
			Rows:           rows,
		})
	}

	return diagnostics
}

func buildGroupLocalizationsDiagnosticRow(sub Subscription) SubscriptionDiagnosticRow {
	row := SubscriptionDiagnosticRow{
		Key:      "group_localizations",
		Label:    "Group localizations",
		Status:   DiagnosticStatusUnknown,
		Source:   "public-api",
		Blocking: true,
	}

	if sub.GroupLocalizationCheckSkipped {
		row.Status = DiagnosticStatusUnverified
		row.Remediation = fallbackString(sub.GroupLocalizationCheckReason, "Validation could not verify subscription group localizations automatically")
		return row
	}

	if len(sub.GroupLocalizations) == 0 {
		row.Status = DiagnosticStatusNo
		row.Evidence = "none"
		row.Remediation = "Create at least one subscription group localization with a display name via `asc subscriptions groups localizations create`."
		return row
	}

	locales := make([]string, 0, len(sub.GroupLocalizations))
	missing := make([]string, 0)
	for _, loc := range sub.GroupLocalizations {
		locale := fallbackString(strings.TrimSpace(loc.Locale), "(unknown locale)")
		locales = append(locales, locale)
		if strings.TrimSpace(loc.Name) == "" {
			missing = append(missing, locale)
		}
	}
	locales = sortedUniqueNonEmpty(locales)
	missing = sortedUniqueNonEmpty(missing)
	if len(missing) > 0 {
		row.Status = DiagnosticStatusNo
		row.Evidence = fmt.Sprintf("locales=%s missing_display_name=%s", formatList(locales), formatList(missing))
		row.Remediation = "Set a display name for each subscription group localization."
		return row
	}

	row.Status = DiagnosticStatusYes
	row.Evidence = formatList(locales)
	return row
}

func buildSubscriptionLocalizationsDiagnosticRow(sub Subscription) SubscriptionDiagnosticRow {
	row := SubscriptionDiagnosticRow{
		Key:      "subscription_localizations",
		Label:    "Subscription localizations",
		Status:   DiagnosticStatusUnknown,
		Source:   "public-api",
		Blocking: true,
	}

	if sub.LocalizationCheckSkipped {
		row.Status = DiagnosticStatusUnverified
		row.Remediation = fallbackString(sub.LocalizationCheckSkipReason, "Validation could not verify subscription localizations automatically")
		return row
	}

	if len(sub.Localizations) == 0 {
		row.Status = DiagnosticStatusNo
		row.Evidence = "none"
		row.Remediation = "Create at least one subscription localization with a display name and description via `asc subscriptions localizations create`."
		return row
	}

	locales := make([]string, 0, len(sub.Localizations))
	missing := make([]string, 0)
	for _, loc := range sub.Localizations {
		locale := fallbackString(strings.TrimSpace(loc.Locale), "(unknown locale)")
		locales = append(locales, locale)
		var parts []string
		if strings.TrimSpace(loc.Name) == "" {
			parts = append(parts, "display name")
		}
		if strings.TrimSpace(loc.Description) == "" {
			parts = append(parts, "description")
		}
		if len(parts) > 0 {
			missing = append(missing, fmt.Sprintf("%s missing %s", locale, strings.Join(parts, ", ")))
		}
	}
	locales = sortedUniqueNonEmpty(locales)
	missing = sortedUniqueNonEmpty(missing)
	if len(missing) > 0 {
		row.Status = DiagnosticStatusNo
		row.Evidence = fmt.Sprintf("locales=%s incomplete=%s", formatList(locales), formatList(missing))
		row.Remediation = "Complete the missing display name and description fields for each subscription localization."
		return row
	}

	row.Status = DiagnosticStatusYes
	row.Evidence = formatList(locales)
	return row
}

func buildReviewScreenshotDiagnosticRow(sub Subscription) SubscriptionDiagnosticRow {
	row := SubscriptionDiagnosticRow{
		Key:      "review_screenshot",
		Label:    "Review screenshot attached",
		Status:   DiagnosticStatusUnknown,
		Source:   "public-api",
		Blocking: true,
	}

	if sub.ReviewScreenshotCheckSkipped {
		row.Status = DiagnosticStatusUnverified
		row.Remediation = fallbackString(sub.ReviewScreenshotCheckReason, "Validation could not verify the subscription App Review screenshot automatically")
		return row
	}

	if strings.TrimSpace(sub.ReviewScreenshotID) == "" {
		row.Status = DiagnosticStatusNo
		row.Evidence = "none"
		row.Remediation = fmt.Sprintf("Upload an App Review screenshot with `asc subscriptions review screenshots create --subscription-id %q --file \"./review.png\"`.", fallbackString(strings.TrimSpace(sub.ID), "SUB_ID"))
		return row
	}

	row.Status = DiagnosticStatusYes
	row.Evidence = fmt.Sprintf("id=%s", strings.TrimSpace(sub.ReviewScreenshotID))
	return row
}

func buildPromotionalImageDiagnosticRow(sub Subscription) SubscriptionDiagnosticRow {
	row := SubscriptionDiagnosticRow{
		Key:      "promotional_image",
		Label:    "Promotional image present",
		Status:   DiagnosticStatusUnknown,
		Source:   "public-api",
		Blocking: false,
	}

	if sub.ImageCheckSkipped {
		row.Status = DiagnosticStatusUnverified
		row.Remediation = fallbackString(sub.ImageCheckSkipReason, "Validation could not verify the subscription promotional image automatically")
		return row
	}

	if !sub.HasImage {
		row.Status = DiagnosticStatusNo
		row.Evidence = "missing"
		row.Remediation = fmt.Sprintf("Upload a promotional image with `asc subscriptions images create --subscription-id %q --file \"./image.png\"` if you plan to use offer codes, win-back offers, or App Store promotion.", fallbackString(strings.TrimSpace(sub.ID), "SUB_ID"))
		return row
	}

	row.Status = DiagnosticStatusYes
	row.Evidence = "present"
	return row
}

func buildSubscriptionAvailabilityDiagnosticRow(sub Subscription) SubscriptionDiagnosticRow {
	row := SubscriptionDiagnosticRow{
		Key:      "subscription_availability",
		Label:    "Subscription availability",
		Status:   DiagnosticStatusUnknown,
		Source:   "public-api",
		Blocking: true,
	}

	if sub.AvailabilityCheckSkipped {
		row.Status = DiagnosticStatusUnverified
		row.Remediation = fallbackString(sub.AvailabilityCheckSkipReason, "Validation could not verify subscription availability automatically")
		return row
	}

	if strings.TrimSpace(sub.AvailabilityID) == "" {
		row.Status = DiagnosticStatusNo
		row.Evidence = "none"
		row.Remediation = fmt.Sprintf("Configure subscription availability with `asc subscriptions availability edit --subscription-id %q --territories \"USA\"`.", fallbackString(strings.TrimSpace(sub.ID), "SUB_ID"))
		return row
	}

	territories := sortedUniqueNonEmpty(sub.AvailabilityTerritories)
	if len(territories) == 0 {
		row.Status = DiagnosticStatusNo
		row.Evidence = fmt.Sprintf("id=%s territories=none", strings.TrimSpace(sub.AvailabilityID))
		row.Remediation = fmt.Sprintf("Add at least one available territory with `asc subscriptions availability edit --subscription-id %q --territories \"USA\"`.", fallbackString(strings.TrimSpace(sub.ID), "SUB_ID"))
		return row
	}

	row.Status = DiagnosticStatusYes
	row.Evidence = fmt.Sprintf("id=%s territories=%s", strings.TrimSpace(sub.AvailabilityID), formatList(territories))
	return row
}

func buildPriceRecordsDiagnosticRow(sub Subscription) SubscriptionDiagnosticRow {
	row := SubscriptionDiagnosticRow{
		Key:      "price_records",
		Label:    "Price records",
		Status:   DiagnosticStatusUnknown,
		Source:   "public-api",
		Blocking: true,
	}

	if sub.PriceCheckSkipped {
		row.Status = DiagnosticStatusUnverified
		row.Remediation = fallbackString(sub.PriceCheckSkipReason, "Validation could not verify subscription pricing automatically")
		return row
	}

	territories := sortedUniqueNonEmpty(sub.PriceTerritories)
	if len(territories) == 0 || sub.PriceCount == 0 {
		row.Status = DiagnosticStatusNo
		row.Evidence = "none"
		row.Remediation = fmt.Sprintf("Configure prices with `asc subscriptions pricing prices set --subscription-id %q ...` or `asc subscriptions pricing equalize --subscription-id %q --base-territory USA`.", fallbackString(strings.TrimSpace(sub.ID), "SUB_ID"), fallbackString(strings.TrimSpace(sub.ID), "SUB_ID"))
		return row
	}

	row.Status = DiagnosticStatusYes
	row.Evidence = formatList(territories)
	return row
}

func buildSubscriptionAvailabilityCoverageDiagnosticRow(sub Subscription) SubscriptionDiagnosticRow {
	row := SubscriptionDiagnosticRow{
		Key:      "price_coverage_subscription_availability",
		Label:    "Price coverage vs subscription availability",
		Status:   DiagnosticStatusUnknown,
		Source:   "derived",
		Blocking: true,
	}

	if sub.AvailabilityCheckSkipped || sub.PriceCheckSkipped {
		row.Status = DiagnosticStatusUnverified
		row.Remediation = firstNonEmpty(sub.PriceCheckSkipReason, sub.AvailabilityCheckSkipReason, "Validation could not compare price coverage against subscription availability automatically")
		return row
	}

	if strings.TrimSpace(sub.AvailabilityID) == "" {
		row.Evidence = "subscription availability missing"
		row.Remediation = "Configure subscription availability before checking pricing coverage."
		return row
	}

	available := sortedUniqueNonEmpty(sub.AvailabilityTerritories)
	if len(available) == 0 {
		row.Evidence = "subscription availability has no territories"
		row.Remediation = "Add available territories before checking pricing coverage."
		return row
	}

	priced := sortedUniqueNonEmpty(sub.PriceTerritories)
	missing := missingValues(available, priced)
	if len(missing) > 0 {
		row.Status = DiagnosticStatusNo
		row.Evidence = fmt.Sprintf("priced=%s missing=%s", formatList(priced), formatList(missing))
		row.Remediation = fmt.Sprintf("Add prices for the missing territories with `asc subscriptions pricing prices set --subscription-id %q ...` or `asc subscriptions pricing equalize --subscription-id %q --base-territory USA`.", fallbackString(strings.TrimSpace(sub.ID), "SUB_ID"), fallbackString(strings.TrimSpace(sub.ID), "SUB_ID"))
		return row
	}

	row.Status = DiagnosticStatusYes
	row.Evidence = fmt.Sprintf("priced=%s available=%s", formatList(priced), formatList(available))
	return row
}

func buildAppAvailabilityCoverageDiagnosticRow(sub Subscription, appTerritories []string, appTerritoryCount int, skipReason string) SubscriptionDiagnosticRow {
	row := SubscriptionDiagnosticRow{
		Key:      "price_coverage_app_availability",
		Label:    "Price coverage vs app availability",
		Status:   DiagnosticStatusUnknown,
		Source:   "derived",
		Blocking: true,
	}

	if strings.TrimSpace(skipReason) != "" {
		row.Status = DiagnosticStatusUnverified
		row.Remediation = strings.TrimSpace(skipReason)
		return row
	}

	if sub.PriceCheckSkipped {
		row.Status = DiagnosticStatusUnverified
		row.Remediation = fallbackString(sub.PriceCheckSkipReason, "Validation could not verify subscription pricing automatically")
		return row
	}

	if sub.AvailabilityCheckSkipped {
		row.Status = DiagnosticStatusUnverified
		row.Blocking = false
		row.Remediation = fallbackString(sub.AvailabilityCheckSkipReason, "Validation could not compare price coverage against app availability until subscription availability verification succeeds")
		return row
	}

	if strings.TrimSpace(sub.AvailabilityID) == "" {
		row.Blocking = false
		row.Evidence = "subscription availability missing"
		row.Remediation = "Configure subscription availability before comparing price coverage against app availability."
		return row
	}

	subscriptionTerritories := sortedUniqueNonEmpty(sub.AvailabilityTerritories)
	if len(subscriptionTerritories) == 0 {
		row.Blocking = false
		row.Evidence = "subscription availability has no territories"
		row.Remediation = "Add subscription availability territories before comparing price coverage against app availability."
		return row
	}

	priced := sortedUniqueNonEmpty(sub.PriceTerritories)
	if len(subscriptionTerritories) > 0 {
		if len(appTerritories) > 0 {
			appOnly := missingValues(appTerritories, subscriptionTerritories)
			if len(appOnly) > 0 {
				row.Status = DiagnosticStatusOptional
				row.Blocking = false
				row.Evidence = fmt.Sprintf("subscription=%s app_only=%s", formatList(subscriptionTerritories), formatList(appOnly))
				row.Remediation = "Optional: if this subscription should be sold everywhere the app is available, add the extra app territories to subscription availability first and then configure prices for them."
				return row
			}
		} else if appTerritoryCount > len(subscriptionTerritories) {
			row.Status = DiagnosticStatusOptional
			row.Blocking = false
			row.Evidence = fmt.Sprintf("subscription_count=%d app_count=%d", len(subscriptionTerritories), appTerritoryCount)
			row.Remediation = "Optional: if this subscription should be sold everywhere the app is available, expand subscription availability first and then configure prices for the extra territories."
			return row
		}
	}
	if len(appTerritories) == 0 {
		if appTerritoryCount <= 0 {
			row.Blocking = false
			row.Evidence = "app availability territories unavailable"
			row.Remediation = "App availability could not be compared for this app because no App Availability V2 territories were available."
			return row
		}

		pricedCount := sub.PriceCount
		if len(priced) > pricedCount {
			pricedCount = len(priced)
		}
		if pricedCount >= appTerritoryCount {
			row.Status = DiagnosticStatusYes
			row.Evidence = fmt.Sprintf("priced_count=%d app_count=%d", pricedCount, appTerritoryCount)
			return row
		}

		row.Status = DiagnosticStatusNo
		row.Evidence = fmt.Sprintf("priced_count=%d app_count=%d", pricedCount, appTerritoryCount)
		row.Remediation = fmt.Sprintf("Add prices for the missing app territories with `asc subscriptions pricing prices set --subscription-id %q ...` or `asc subscriptions pricing equalize --subscription-id %q --base-territory USA`.", fallbackString(strings.TrimSpace(sub.ID), "SUB_ID"), fallbackString(strings.TrimSpace(sub.ID), "SUB_ID"))
		return row
	}

	missing := missingValues(appTerritories, priced)
	if len(missing) > 0 {
		row.Status = DiagnosticStatusNo
		row.Evidence = fmt.Sprintf("priced=%s missing=%s", formatList(priced), formatList(missing))
		row.Remediation = fmt.Sprintf("Add prices for the missing app territories with `asc subscriptions pricing prices set --subscription-id %q ...` or `asc subscriptions pricing equalize --subscription-id %q --base-territory USA`.", fallbackString(strings.TrimSpace(sub.ID), "SUB_ID"), fallbackString(strings.TrimSpace(sub.ID), "SUB_ID"))
		return row
	}

	row.Status = DiagnosticStatusYes
	row.Evidence = fmt.Sprintf("priced=%s app=%s", formatList(priced), formatList(appTerritories))
	return row
}

func buildAppBuildDiagnosticRow(count int, skipped bool, skipReason string) SubscriptionDiagnosticRow {
	row := SubscriptionDiagnosticRow{
		Key:      "app_has_build",
		Label:    "App has build",
		Status:   DiagnosticStatusUnknown,
		Source:   "context",
		Blocking: false,
	}

	if skipped {
		row.Status = DiagnosticStatusUnverified
		row.Remediation = fallbackString(skipReason, "Validation could not determine whether this app has builds")
		return row
	}

	if count > 0 {
		row.Status = DiagnosticStatusYes
		row.Evidence = fmt.Sprintf("count=%d", count)
		return row
	}

	row.Status = DiagnosticStatusNo
	row.Evidence = "count=0"
	row.Remediation = "Attach or upload a build for this app and rerun readiness checks. This is app-level context and may still be separate from the subscription metadata issue."
	return row
}

func buildOptionalOfferDiagnosticRow(key, label string, count int, skipped bool, skipReason, remediation string) SubscriptionDiagnosticRow {
	row := SubscriptionDiagnosticRow{
		Key:      key,
		Label:    label,
		Status:   DiagnosticStatusUnknown,
		Source:   "public-api",
		Blocking: false,
	}

	if skipped {
		row.Status = DiagnosticStatusUnverified
		row.Remediation = fallbackString(skipReason, "Validation could not verify offer configuration automatically")
		return row
	}

	if count > 0 {
		row.Status = DiagnosticStatusYes
		row.Evidence = fmt.Sprintf("count=%d", count)
		return row
	}

	row.Status = DiagnosticStatusOptional
	row.Evidence = "not configured"
	row.Remediation = remediation
	return row
}

func summarizeSubscriptionDiagnostics(sub Subscription, rows []SubscriptionDiagnosticRow) (string, string) {
	blockingFailures := 0
	blockingUnknown := 0
	advisoryFailures := 0

	for _, row := range rows {
		switch {
		case row.Blocking && row.Status == DiagnosticStatusNo:
			blockingFailures++
		case row.Blocking && (row.Status == DiagnosticStatusUnknown || row.Status == DiagnosticStatusUnverified):
			blockingUnknown++
		case !row.Blocking && row.Status == DiagnosticStatusNo:
			advisoryFailures++
		}
	}

	state := normalizeMonetizationState(sub.State)
	switch {
	case blockingFailures > 0:
		return "known_blocker", fmt.Sprintf("%d known blocking subscription issue(s) found", blockingFailures)
	case blockingUnknown > 0:
		return "unknown", fmt.Sprintf("%d blocking subscription check(s) could not be verified automatically", blockingUnknown)
	case advisoryFailures > 0:
		return "advisory_only", "No blocking issues found; only advisory subscription findings remain."
	case state == "MISSING_METADATA":
		return "opaque_apple_state", "All verifiable public checks passed, but Apple still reports MISSING_METADATA."
	case state == "READY_TO_SUBMIT":
		return "ready_to_submit", "No public metadata issues found. Attach this subscription from the app version review flow if needed."
	default:
		return "ready", "No known subscription readiness issues found."
	}
}

// SortedUniqueNonEmptyStrings trims, deduplicates, and sorts a string slice.
func SortedUniqueNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func sortedUniqueNonEmpty(values []string) []string {
	return SortedUniqueNonEmptyStrings(values)
}

func missingValues(expected, actual []string) []string {
	expected = sortedUniqueNonEmpty(expected)
	actual = sortedUniqueNonEmpty(actual)
	if len(expected) == 0 {
		return nil
	}
	actualSet := make(map[string]struct{}, len(actual))
	for _, value := range actual {
		actualSet[value] = struct{}{}
	}
	missing := make([]string, 0)
	for _, value := range expected {
		if _, ok := actualSet[value]; !ok {
			missing = append(missing, value)
		}
	}
	return missing
}

func formatList(values []string) string {
	values = sortedUniqueNonEmpty(values)
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ",")
}

func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
