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
	pricingTerritories := sortedUniqueNonEmpty(input.PricingTerritories)
	pricingTerritoryCount := input.PricingTerritoryCount
	if len(pricingTerritories) == 0 && pricingTerritoryCount == 0 {
		pricingTerritories = appTerritories
		pricingTerritoryCount = appTerritoryCount
	}
	pricingMatrixContextAvailable := len(pricingTerritories) > 0 || pricingTerritoryCount > 0 || strings.TrimSpace(input.PricingCoverageSkipReason) != ""

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
			buildUpfrontPlanAvailabilityDiagnosticRow(sub),
			buildAvailabilitySurfaceConsistencyDiagnosticRow(sub),
			buildMonthlyPlanAvailabilityDiagnosticRow(sub),
			buildPriceRecordsDiagnosticRow(sub),
			buildSubscriptionAvailabilityCoverageDiagnosticRow(sub),
			buildAppAvailabilityCoverageDiagnosticRow(sub, appTerritories, appTerritoryCount, input.AppAvailabilityCoverageSkipReason),
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
		if pricingMatrixContextAvailable {
			rows = append(rows, buildPricingMatrixDiagnosticRow(sub, pricingTerritories, pricingTerritoryCount, input.PricingCoverageSkipReason))
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

type subscriptionPlanAvailabilityAnalysis struct {
	unverified             bool
	duplicateTypes         bool
	upfront                *SubscriptionPlanAvailabilityInfo
	monthly                *SubscriptionPlanAvailabilityInfo
	upfrontTerritories     []string
	monthlyTerritories     []string
	surfaceMismatch        bool
	legacyOnly             []string
	planOnly               []string
	newTerritoriesMismatch bool
	monthlyIssues          []string
}

func analyzeSubscriptionPlanAvailability(sub Subscription) subscriptionPlanAvailabilityAnalysis {
	analysis := subscriptionPlanAvailabilityAnalysis{unverified: sub.PlanAvailabilityCheckSkipped}
	counts := map[string]int{}
	for i := range sub.PlanAvailabilities {
		plan := sub.PlanAvailabilities[i]
		plan.PlanType = strings.ToUpper(strings.TrimSpace(plan.PlanType))
		counts[plan.PlanType]++
		switch plan.PlanType {
		case "UPFRONT":
			if analysis.upfront == nil {
				analysis.upfront = &plan
				analysis.upfrontTerritories = sortedUniqueNonEmpty(plan.Territories)
			}
		case "MONTHLY":
			if analysis.monthly == nil {
				analysis.monthly = &plan
				analysis.monthlyTerritories = sortedUniqueNonEmpty(plan.Territories)
			}
		}
	}
	analysis.duplicateTypes = counts["UPFRONT"] > 1 || counts["MONTHLY"] > 1

	if !analysis.duplicateTypes && !sub.AvailabilityCheckSkipped && strings.TrimSpace(sub.AvailabilityID) != "" && analysis.upfront != nil {
		legacy := sortedUniqueNonEmpty(sub.AvailabilityTerritories)
		analysis.legacyOnly = missingValues(legacy, analysis.upfrontTerritories)
		analysis.planOnly = missingValues(analysis.upfrontTerritories, legacy)
		legacyNew, planNew := sub.AvailabilityInNewTerritories, analysis.upfront.AvailableInNewTerritories
		analysis.newTerritoriesMismatch = legacyNew != nil && planNew != nil && *legacyNew != *planNew
		analysis.surfaceMismatch = len(analysis.legacyOnly) > 0 || len(analysis.planOnly) > 0 || analysis.newTerritoriesMismatch
	}

	if !analysis.duplicateTypes && analysis.monthly != nil && len(analysis.monthlyTerritories) > 0 {
		if strings.ToUpper(strings.TrimSpace(sub.SubscriptionPeriod)) != "ONE_YEAR" {
			analysis.monthlyIssues = append(analysis.monthlyIssues, "subscription period is not ONE_YEAR")
		}
		if outside := missingValues(analysis.monthlyTerritories, analysis.upfrontTerritories); len(outside) > 0 {
			analysis.monthlyIssues = append(analysis.monthlyIssues, "not in UPFRONT: "+formatList(outside))
		}
		forbidden := make([]string, 0, 2)
		for _, territory := range analysis.monthlyTerritories {
			if territory == "USA" || territory == "SGP" {
				forbidden = append(forbidden, territory)
			}
		}
		if len(forbidden) > 0 {
			analysis.monthlyIssues = append(analysis.monthlyIssues, "unsupported: "+formatList(forbidden))
		}
	}
	return analysis
}

func buildUpfrontPlanAvailabilityDiagnosticRow(sub Subscription) SubscriptionDiagnosticRow {
	row := SubscriptionDiagnosticRow{Key: "upfront_plan_availability", Label: "UPFRONT plan availability", Source: "public-api", Blocking: true}
	analysis := analyzeSubscriptionPlanAvailability(sub)
	if analysis.unverified {
		row.Status = DiagnosticStatusUnverified
		row.Remediation = fallbackString(sub.PlanAvailabilityCheckReason, "Validation could not verify billing-plan availability")
		return row
	}
	if analysis.duplicateTypes {
		row.Status = DiagnosticStatusNo
		row.Evidence = "duplicate UPFRONT or MONTHLY records"
		row.Remediation = "Review and repair duplicate plan availability records in App Store Connect."
		return row
	}
	if analysis.upfront == nil {
		row.Status = DiagnosticStatusNo
		row.Evidence = "none"
		row.Remediation = "Configure UPFRONT plan availability for at least one territory."
		return row
	}
	if len(analysis.upfrontTerritories) == 0 {
		row.Status = DiagnosticStatusNo
		row.Evidence = fmt.Sprintf("id=%s territories=none", strings.TrimSpace(analysis.upfront.ID))
		row.Remediation = "Enable at least one territory for the UPFRONT plan."
		return row
	}
	row.Status = DiagnosticStatusYes
	row.Evidence = fmt.Sprintf("id=%s territories=%s", strings.TrimSpace(analysis.upfront.ID), formatList(analysis.upfrontTerritories))
	return row
}

func buildAvailabilitySurfaceConsistencyDiagnosticRow(sub Subscription) SubscriptionDiagnosticRow {
	row := SubscriptionDiagnosticRow{Key: "availability_surface_consistency", Label: "Legacy and UPFRONT availability agree", Source: "derived", Blocking: true}
	analysis := analyzeSubscriptionPlanAvailability(sub)
	if analysis.unverified || sub.AvailabilityCheckSkipped {
		row.Status = DiagnosticStatusUnverified
		row.Remediation = firstNonEmpty(sub.PlanAvailabilityCheckReason, sub.AvailabilityCheckSkipReason, "Validation could not compare availability surfaces")
		return row
	}
	if analysis.duplicateTypes {
		row.Status = DiagnosticStatusNo
		row.Evidence = "duplicate UPFRONT or MONTHLY records prevent a reliable comparison"
		row.Remediation = "Review and repair duplicate plan availability records in App Store Connect."
		return row
	}
	if strings.TrimSpace(sub.AvailabilityID) == "" || analysis.upfront == nil {
		row.Status = DiagnosticStatusUnknown
		row.Blocking = false
		row.Evidence = "one or both availability surfaces are missing"
		return row
	}
	if analysis.surfaceMismatch {
		row.Status = DiagnosticStatusNo
		row.Evidence = fmt.Sprintf("legacy_only=%s plan_only=%s", formatList(analysis.legacyOnly), formatList(analysis.planOnly))
		if analysis.newTerritoriesMismatch {
			row.Evidence += fmt.Sprintf(" available_in_new_territories legacy=%t plan=%t", *sub.AvailabilityInNewTerritories, *analysis.upfront.AvailableInNewTerritories)
		}
		row.Remediation = "Make legacy subscription availability and UPFRONT plan availability agree, then re-validate."
		return row
	}
	row.Status = DiagnosticStatusYes
	row.Evidence = fmt.Sprintf("territories=%s", formatList(analysis.upfrontTerritories))
	return row
}

func buildMonthlyPlanAvailabilityDiagnosticRow(sub Subscription) SubscriptionDiagnosticRow {
	row := SubscriptionDiagnosticRow{Key: "monthly_plan_availability", Label: "MONTHLY commitment availability", Source: "public-api", Blocking: false}
	analysis := analyzeSubscriptionPlanAvailability(sub)
	if analysis.unverified {
		row.Status = DiagnosticStatusUnverified
		row.Remediation = fallbackString(sub.PlanAvailabilityCheckReason, "Validation could not verify MONTHLY plan availability")
		return row
	}
	if analysis.duplicateTypes {
		row.Status = DiagnosticStatusNo
		row.Blocking = true
		row.Evidence = "duplicate UPFRONT or MONTHLY records prevent reliable MONTHLY validation"
		row.Remediation = "Review and repair duplicate plan availability records in App Store Connect."
		return row
	}
	if analysis.monthly == nil || len(analysis.monthlyTerritories) == 0 {
		row.Status = DiagnosticStatusOptional
		row.Evidence = "not configured"
		return row
	}
	row.Blocking = true
	if len(analysis.monthlyIssues) > 0 {
		row.Status = DiagnosticStatusNo
		row.Evidence = strings.Join(analysis.monthlyIssues, "; ")
		row.Remediation = "Use MONTHLY only for ONE_YEAR subscriptions, only in UPFRONT territories, and exclude USA and SGP."
		return row
	}
	row.Status = DiagnosticStatusYes
	row.Evidence = fmt.Sprintf("id=%s territories=%s", strings.TrimSpace(analysis.monthly.ID), formatList(analysis.monthlyTerritories))
	return row
}

func buildPricingMatrixDiagnosticRow(sub Subscription, pricingTerritories []string, pricingTerritoryCount int, skipReason string) SubscriptionDiagnosticRow {
	row := SubscriptionDiagnosticRow{
		Key:      "complete_pricing_matrix",
		Label:    "Complete App Store pricing matrix",
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
	pricingTerritories = sortedUniqueNonEmpty(pricingTerritories)
	if len(pricingTerritories) > 0 {
		missing := missingValues(pricingTerritories, sortedUniqueNonEmpty(sub.PriceTerritories))
		if len(missing) == 0 {
			row.Status = DiagnosticStatusYes
			row.Evidence = fmt.Sprintf("priced=%d required=%d", len(pricingTerritories), len(pricingTerritories))
			return row
		}
		row.Status = DiagnosticStatusNo
		row.Evidence = fmt.Sprintf("priced=%d required=%d missing=%s", len(sortedUniqueNonEmpty(sub.PriceTerritories)), len(pricingTerritories), formatList(missing))
		row.Remediation = "Re-run `asc subscriptions setup` with the original group, subscription, and pricing flags plus `--repair`; sale availability does not narrow Apple's pricing requirement."
		return row
	}
	if pricingTerritoryCount <= 0 {
		row.Status = DiagnosticStatusUnverified
		row.Remediation = "App Store pricing territories were unavailable."
		return row
	}
	pricedCount := max(sub.PriceCount, len(sortedUniqueNonEmpty(sub.PriceTerritories)))
	if pricedCount >= pricingTerritoryCount {
		row.Status = DiagnosticStatusYes
	} else {
		row.Status = DiagnosticStatusNo
		row.Remediation = "Re-run `asc subscriptions setup` with the original group, subscription, and pricing flags plus `--repair`."
	}
	row.Evidence = fmt.Sprintf("priced=%d required=%d", pricedCount, pricingTerritoryCount)
	return row
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
		Label:    "Review screenshot delivery",
		Status:   DiagnosticStatusUnknown,
		Source:   "public-api",
		Blocking: true,
	}

	if sub.ReviewScreenshotCheckSkipped {
		row.Status = DiagnosticStatusUnverified
		if screenshotID := strings.TrimSpace(sub.ReviewScreenshotID); screenshotID != "" {
			state := fallbackString(strings.ToUpper(strings.TrimSpace(sub.ReviewScreenshotAssetDeliveryState)), "unknown")
			row.Evidence = fmt.Sprintf("id=%s asset_delivery_state=%s", screenshotID, state)
		}
		row.Remediation = fallbackString(sub.ReviewScreenshotCheckReason, "Validation could not verify the subscription App Review screenshot automatically")
		return row
	}

	if strings.TrimSpace(sub.ReviewScreenshotID) == "" {
		row.Status = DiagnosticStatusNo
		row.Evidence = "none"
		row.Remediation = fmt.Sprintf("Upload an App Review screenshot with `asc subscriptions review screenshots create --subscription-id %q --file \"./review.png\"`.", fallbackString(strings.TrimSpace(sub.ID), "SUB_ID"))
		return row
	}

	state := strings.ToUpper(strings.TrimSpace(sub.ReviewScreenshotAssetDeliveryState))
	row.Evidence = fmt.Sprintf("id=%s asset_delivery_state=%s", strings.TrimSpace(sub.ReviewScreenshotID), fallbackString(state, "unknown"))
	if len(sub.ReviewScreenshotAssetDeliveryErrors) > 0 {
		row.Evidence += " errors=" + strings.Join(sub.ReviewScreenshotAssetDeliveryErrors, "; ")
	}
	switch state {
	case "COMPLETE":
		row.Status = DiagnosticStatusYes
	case "FAILED":
		row.Status = DiagnosticStatusNo
		row.Remediation = reviewScreenshotFailedRemediation(sub)
	default:
		row.Status = DiagnosticStatusUnverified
		row.Remediation = reviewScreenshotDeliveryRemediation(state)
	}
	return row
}

func reviewScreenshotFailedRemediation(sub Subscription) string {
	screenshotID := fallbackString(strings.TrimSpace(sub.ReviewScreenshotID), "SHOT_ID")
	subscriptionID := fallbackString(strings.TrimSpace(sub.ID), "SUB_ID")
	return fmt.Sprintf("Delete the failed screenshot with `asc subscriptions review screenshots delete --screenshot-id %q --confirm`, then re-upload it with `asc subscriptions review screenshots create --subscription-id %q --file \"./review.png\"`.", screenshotID, subscriptionID)
}

func reviewScreenshotDeliveryRemediation(state string) string {
	state = strings.ToUpper(strings.TrimSpace(state))
	if state == "" {
		return "Apple did not return the screenshot asset delivery state; retry validation and confirm it reaches COMPLETE before submission."
	}
	return fmt.Sprintf("The screenshot asset delivery state is %s; wait for it to reach COMPLETE, then re-run validation.", state)
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
		row.Remediation = fmt.Sprintf("Apple documents this image as optional unless you use offers or App Store promotion. For an otherwise-complete subscription stuck in MISSING_METADATA, uploading a 1024x1024 image with `asc subscriptions images create --subscription-id %q --file \"./image.png\"` can also serve as an undocumented recalculation attempt; re-run validation afterward.", fallbackString(strings.TrimSpace(sub.ID), "SUB_ID"))
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
	case state == "MISSING_METADATA" && advisoryFailures > 0:
		return "opaque_apple_state", fmt.Sprintf("All blocking public checks passed; %d advisory finding(s) remain, but they do not explain why Apple still reports MISSING_METADATA.", advisoryFailures)
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
