package validation

import (
	"fmt"
	"strings"
)

// Subscription represents an auto-renewable subscription for review-readiness validation.
type Subscription struct {
	ID                           string
	Name                         string
	ProductID                    string
	State                        string
	GroupID                      string
	GroupName                    string
	HasImage                     bool
	ImageCheckSkipped            bool
	ImageCheckSkipReason         string
	ReviewScreenshotID           string
	ReviewScreenshotCheckSkipped bool
	ReviewScreenshotCheckReason  string
	AvailabilityID               string
	AvailabilityTerritories      []string
	AvailabilityCheckSkipped     bool
	AvailabilityCheckSkipReason  string

	// Deep diagnostics (populated when State is MISSING_METADATA).
	Localizations                 []SubscriptionLocalizationInfo
	LocalizationCheckSkipped      bool
	LocalizationCheckSkipReason   string
	GroupLocalizations            []SubscriptionGroupLocalizationInfo
	GroupLocalizationCheckSkipped bool
	GroupLocalizationCheckReason  string
	PriceCount                    int
	PriceTerritories              []string
	PriceCheckSkipped             bool
	PriceCheckSkipReason          string
	IntroductoryOfferCount        int
	IntroductoryOfferCheckSkipped bool
	IntroductoryOfferCheckReason  string
	PromotionalOfferCount         int
	PromotionalOfferCheckSkipped  bool
	PromotionalOfferCheckReason   string
	WinBackOfferCount             int
	WinBackOfferCheckSkipped      bool
	WinBackOfferCheckReason       string
}

// SubscriptionLocalizationInfo holds per-locale metadata for a subscription.
type SubscriptionLocalizationInfo struct {
	Locale      string
	Name        string
	Description string
}

// SubscriptionGroupLocalizationInfo holds per-locale metadata for a subscription group.
type SubscriptionGroupLocalizationInfo struct {
	Locale string
	Name   string
}

// SubscriptionsInput collects subscription validation inputs.
type SubscriptionsInput struct {
	AppID                     string
	Subscriptions             []Subscription
	AvailableTerritories      int
	AppAvailableTerritories   []string
	PricingCoverageSkipReason string
	AppBuildCount             int
	BuildCheckSkipped         bool
	BuildCheckSkipReason      string
}

// SubscriptionsReport is the top-level validate subscriptions output.
type SubscriptionsReport struct {
	AppID             string                    `json:"appId"`
	SubscriptionCount int                       `json:"subscriptionCount,omitempty"`
	Summary           Summary                   `json:"summary"`
	Checks            []CheckResult             `json:"checks"`
	Diagnostics       []SubscriptionDiagnostics `json:"diagnostics,omitempty"`
	Strict            bool                      `json:"strict,omitempty"`
}

// ValidateSubscriptions validates subscription review readiness and returns a report.
func ValidateSubscriptions(input SubscriptionsInput, strict bool) SubscriptionsReport {
	availableTerritories := input.AvailableTerritories
	appAvailableTerritories := input.AppAvailableTerritories
	if input.PricingCoverageSkipReason != "" {
		availableTerritories = 0
		appAvailableTerritories = nil
	}

	checks := make([]CheckResult, 0)
	checks = append(checks, subscriptionImageChecks(input.Subscriptions)...)
	checks = append(checks, subscriptionReviewReadinessChecks(input.Subscriptions)...)
	checks = append(checks, subscriptionPricingVerificationChecks(input.Subscriptions)...)
	checks = append(checks, subscriptionPricingCoverageSkipChecks(input.AppID, input.PricingCoverageSkipReason)...)
	checks = append(checks, subscriptionMetadataDiagnostics(input.Subscriptions)...)
	checks = append(checks, subscriptionPricingCoverageChecks(input.Subscriptions, availableTerritories, appAvailableTerritories)...)
	diagnostics := buildSubscriptionDiagnostics(input)
	summary := summarize(checks, strict)

	return SubscriptionsReport{
		AppID:             strings.TrimSpace(input.AppID),
		SubscriptionCount: len(input.Subscriptions),
		Summary:           summary,
		Checks:            checks,
		Diagnostics:       diagnostics,
		Strict:            strict,
	}
}

func subscriptionFetchChecks(reason string) []CheckResult {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil
	}

	return []CheckResult{{
		ID:          "subscriptions.readiness.unverified",
		Severity:    SeverityInfo,
		Field:       "subscriptions",
		Message:     "Could not verify subscription readiness for this app",
		Remediation: reason,
	}}
}

func subscriptionImageChecks(subs []Subscription) []CheckResult {
	var checks []CheckResult
	for _, sub := range subs {
		if isRemovedMonetizationState(sub.State) {
			continue
		}
		label := formatSubscriptionLabel(sub)
		if sub.ImageCheckSkipped {
			remediation := strings.TrimSpace(sub.ImageCheckSkipReason)
			if remediation == "" {
				remediation = "Review this subscription's promotional image in App Store Connect; validation could not verify image presence automatically"
			}
			checks = append(checks, CheckResult{
				ID:           "subscriptions.images.unverified",
				Severity:     SeverityInfo,
				Field:        "images",
				ResourceType: "subscription",
				ResourceID:   strings.TrimSpace(sub.ID),
				Message:      fmt.Sprintf("Could not verify whether %s has a subscription promotional image", label),
				Remediation:  remediation,
			})
			continue
		}
		if sub.HasImage {
			continue
		}

		checks = append(checks, CheckResult{
			ID:           "subscriptions.images.recommended",
			Severity:     SeverityWarning,
			Field:        "images",
			ResourceType: "subscription",
			ResourceID:   strings.TrimSpace(sub.ID),
			Message:      fmt.Sprintf("%s has no subscription promotional image", label),
			Remediation:  "Upload a unique promotional image if you plan to promote this subscription on the App Store, support offer-code redemption pages, or run win-back offers; the App Review screenshot is separate and review-only",
		})
	}

	return checks
}

func subscriptionReviewReadinessChecks(subs []Subscription) []CheckResult {
	// These checks are warnings by default. Many apps have subscriptions that
	// aren't relevant to a given release. Use --strict to gate in CI.
	okStates := map[string]struct{}{
		"APPROVED":                {},
		"WAITING_FOR_REVIEW":      {},
		"IN_REVIEW":               {},
		"PENDING_BINARY_APPROVAL": {},
	}
	var checks []CheckResult
	for _, sub := range subs {
		state := normalizeMonetizationState(sub.State)
		if state == "" {
			continue
		}
		if _, ok := okStates[state]; ok {
			continue
		}
		if isRemovedMonetizationState(state) {
			continue
		}

		label := formatSubscriptionLabel(sub)
		message := fmt.Sprintf("%s is %s", label, state)
		remediation := remediationForSubscriptionState(state)

		checks = append(checks, CheckResult{
			ID:           "subscriptions.review_readiness.needs_attention",
			Severity:     SeverityWarning,
			Field:        "state",
			ResourceType: "subscription",
			ResourceID:   strings.TrimSpace(sub.ID),
			Message:      message,
			Remediation:  remediation,
		})
	}

	return checks
}

func formatSubscriptionLabel(sub Subscription) string {
	name := strings.TrimSpace(sub.Name)
	productID := strings.TrimSpace(sub.ProductID)

	switch {
	case name != "" && productID != "":
		return fmt.Sprintf("Subscription %q (%s)", name, productID)
	case name != "":
		return fmt.Sprintf("Subscription %q", name)
	case productID != "":
		return fmt.Sprintf("Subscription %s", productID)
	default:
		return "Subscription"
	}
}

func remediationForSubscriptionState(state string) string {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "MISSING_METADATA":
		return "See diagnostic checks below for specific missing items (group localizations, subscription localizations, pricing); fix all issues then re-validate"
	case "READY_TO_SUBMIT":
		return "Submit this subscription for review in App Store Connect so it is attached to the next app review submission; note: first-time subscriptions must be submitted via the app version page in App Store Connect (not the API)"
	case "DEVELOPER_ACTION_NEEDED":
		return "Resolve developer action required issues for this subscription in App Store Connect"
	case "REJECTED":
		return "Address the rejection feedback for this subscription and resubmit in App Store Connect"
	default:
		return "Review this subscription in App Store Connect and submit it for review if needed"
	}
}

func subscriptionPricingVerificationChecks(subs []Subscription) []CheckResult {
	var checks []CheckResult
	for _, sub := range subs {
		state := normalizeMonetizationState(sub.State)
		if state == "MISSING_METADATA" || isRemovedMonetizationState(state) {
			continue
		}
		if !sub.PriceCheckSkipped {
			continue
		}

		label := formatSubscriptionLabel(sub)
		remediation := strings.TrimSpace(sub.PriceCheckSkipReason)
		if remediation == "" {
			remediation = "Review this subscription's pricing in App Store Connect; validation could not verify it automatically"
		}
		checks = append(checks, CheckResult{
			ID:           "subscriptions.pricing.unverified",
			Severity:     SeverityInfo,
			Field:        "pricing",
			ResourceType: "subscription",
			ResourceID:   strings.TrimSpace(sub.ID),
			Message:      fmt.Sprintf("Could not verify whether %s has territory prices configured", label),
			Remediation:  remediation,
		})
	}

	return checks
}

func subscriptionPricingCoverageSkipChecks(appID, reason string) []CheckResult {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil
	}

	return []CheckResult{{
		ID:           "subscriptions.pricing_coverage.unverified",
		Severity:     SeverityInfo,
		Field:        "pricing",
		ResourceType: "app",
		ResourceID:   strings.TrimSpace(appID),
		Message:      "Could not verify subscription pricing coverage against app availability territories",
		Remediation:  reason,
	}}
}

// subscriptionPricingCoverageChecks warns when a subscription has prices configured
// but doesn't cover all territories the app is available in. This catches the common
// submission failure where only a single territory (e.g., US) has pricing set.
func subscriptionPricingCoverageChecks(subs []Subscription, availableTerritories int, appAvailableTerritories []string) []CheckResult {
	appAvailableTerritories = sortedUniqueNonEmpty(appAvailableTerritories)
	if len(appAvailableTerritories) == 0 && availableTerritories <= 0 {
		return nil
	}
	if len(appAvailableTerritories) > 0 {
		availableTerritories = len(appAvailableTerritories)
	}

	var checks []CheckResult
	for _, sub := range subs {
		state := normalizeMonetizationState(sub.State)
		if isRemovedMonetizationState(state) {
			continue
		}
		if sub.PriceCheckSkipped || sub.PriceCount == 0 {
			continue
		}

		label := formatSubscriptionLabel(sub)
		priceTerritories := sortedUniqueNonEmpty(sub.PriceTerritories)
		subscriptionAvailabilityTerritories := sortedUniqueNonEmpty(sub.AvailabilityTerritories)
		if state == "MISSING_METADATA" {
			availabilityUnknown := sub.AvailabilityCheckSkipped || strings.TrimSpace(sub.AvailabilityID) == ""
			availabilityEmpty := strings.TrimSpace(sub.AvailabilityID) != "" && len(subscriptionAvailabilityTerritories) == 0
			if availabilityUnknown || availabilityEmpty {
				continue
			}
		}
		if len(subscriptionAvailabilityTerritories) > 0 {
			if len(priceTerritories) > 0 {
				missing := missingValues(subscriptionAvailabilityTerritories, priceTerritories)
				if len(missing) == 0 {
					continue
				}
				checks = append(checks, CheckResult{
					ID:           "subscriptions.pricing.partial_territory_coverage",
					Severity:     SeverityWarning,
					Field:        "pricing",
					ResourceType: "subscription",
					ResourceID:   strings.TrimSpace(sub.ID),
					Message:      fmt.Sprintf("%s has pricing for %d of %d subscription availability territories; missing: %s", label, len(priceTerritories), len(subscriptionAvailabilityTerritories), strings.Join(missing, ",")),
					Remediation:  "Set prices for all subscription availability territories using `asc subscriptions pricing equalize` or `asc subscriptions pricing prices set`; missing territory pricing blocks App Store submission",
				})
				continue
			}
			if sub.PriceCount >= len(subscriptionAvailabilityTerritories) {
				continue
			}
			checks = append(checks, CheckResult{
				ID:           "subscriptions.pricing.partial_territory_coverage",
				Severity:     SeverityWarning,
				Field:        "pricing",
				ResourceType: "subscription",
				ResourceID:   strings.TrimSpace(sub.ID),
				Message:      fmt.Sprintf("%s has pricing for %d of %d subscription availability territories", label, sub.PriceCount, len(subscriptionAvailabilityTerritories)),
				Remediation:  "Set prices for all subscription availability territories using `asc subscriptions pricing equalize` or `asc subscriptions pricing prices set`; missing territory pricing blocks App Store submission",
			})
			continue
		}
		if len(appAvailableTerritories) > 0 && len(priceTerritories) > 0 {
			missing := missingValues(appAvailableTerritories, priceTerritories)
			if len(missing) == 0 {
				continue
			}
			checks = append(checks, CheckResult{
				ID:           "subscriptions.pricing.partial_territory_coverage",
				Severity:     SeverityWarning,
				Field:        "pricing",
				ResourceType: "subscription",
				ResourceID:   strings.TrimSpace(sub.ID),
				Message:      fmt.Sprintf("%s has pricing for %d of %d app availability territories; missing: %s", label, len(priceTerritories), len(appAvailableTerritories), strings.Join(missing, ",")),
				Remediation:  "Set prices for all app availability territories using `asc subscriptions pricing equalize` or `asc subscriptions pricing prices set`; missing territory pricing blocks App Store submission",
			})
			continue
		}
		if sub.PriceCount >= availableTerritories {
			continue
		}
		checks = append(checks, CheckResult{
			ID:           "subscriptions.pricing.partial_territory_coverage",
			Severity:     SeverityWarning,
			Field:        "pricing",
			ResourceType: "subscription",
			ResourceID:   strings.TrimSpace(sub.ID),
			Message:      fmt.Sprintf("%s has pricing for %d of %d available territories", label, sub.PriceCount, availableTerritories),
			Remediation:  "Set prices for all available territories using `asc subscriptions pricing equalize` or `asc subscriptions pricing prices set`; missing territory pricing blocks App Store submission",
		})
	}

	return checks
}

// subscriptionMetadataDiagnostics produces specific diagnostic checks for subscriptions
// in MISSING_METADATA state, identifying exactly what's missing.
func subscriptionMetadataDiagnostics(subs []Subscription) []CheckResult {
	var checks []CheckResult

	// Track groups we've already checked to avoid duplicate group localization warnings.
	checkedGroups := make(map[string]bool)

	for _, sub := range subs {
		state := strings.ToUpper(strings.TrimSpace(sub.State))
		if state != "MISSING_METADATA" {
			continue
		}

		label := formatSubscriptionLabel(sub)

		// Check group localizations (only once per group).
		groupID := strings.TrimSpace(sub.GroupID)
		if groupID != "" && !checkedGroups[groupID] {
			checkedGroups[groupID] = true
			groupLabel := groupID
			if strings.TrimSpace(sub.GroupName) != "" {
				groupLabel = fmt.Sprintf("%q (%s)", sub.GroupName, groupID)
			}
			if sub.GroupLocalizationCheckSkipped {
				remediation := strings.TrimSpace(sub.GroupLocalizationCheckReason)
				if remediation == "" {
					remediation = "Review this subscription group's localizations in App Store Connect; validation could not verify them automatically"
				}
				checks = append(checks, CheckResult{
					ID:           "subscriptions.diagnostics.group_localization_unverified",
					Severity:     SeverityInfo,
					Field:        "groupLocalizations",
					ResourceType: "subscriptionGroup",
					ResourceID:   groupID,
					Message:      fmt.Sprintf("Could not verify whether subscription group %s has localizations", groupLabel),
					Remediation:  remediation,
				})
			} else if len(sub.GroupLocalizations) == 0 {
				checks = append(checks, CheckResult{
					ID:           "subscriptions.diagnostics.group_localization_missing",
					Severity:     SeverityWarning,
					Field:        "groupLocalizations",
					ResourceType: "subscriptionGroup",
					ResourceID:   groupID,
					Message:      fmt.Sprintf("Subscription group %s has no localizations", groupLabel),
					Remediation:  "Create at least one subscription group localization (with group display name) via App Store Connect or `asc subscriptions groups localizations create`; this is a common cause of MISSING_METADATA",
				})
			} else {
				for _, loc := range sub.GroupLocalizations {
					locale := strings.TrimSpace(loc.Locale)
					if locale == "" {
						locale = "(unknown locale)"
					}
					if strings.TrimSpace(loc.Name) == "" {
						checks = append(checks, CheckResult{
							ID:           "subscriptions.diagnostics.group_localization_name_empty",
							Severity:     SeverityWarning,
							Field:        "groupLocalizations",
							ResourceType: "subscriptionGroup",
							ResourceID:   groupID,
							Locale:       locale,
							Message:      fmt.Sprintf("Subscription group %s localization for %s has an empty display name", groupLabel, locale),
							Remediation:  "Set a display name for this group localization",
						})
					}
				}
			}
		}

		// Check subscription localizations.
		if sub.LocalizationCheckSkipped {
			remediation := strings.TrimSpace(sub.LocalizationCheckSkipReason)
			if remediation == "" {
				remediation = "Review this subscription's localizations in App Store Connect; validation could not verify them automatically"
			}
			checks = append(checks, CheckResult{
				ID:           "subscriptions.diagnostics.localization_unverified",
				Severity:     SeverityInfo,
				Field:        "localizations",
				ResourceType: "subscription",
				ResourceID:   strings.TrimSpace(sub.ID),
				Message:      fmt.Sprintf("Could not verify whether %s has localizations", label),
				Remediation:  remediation,
			})
		} else if len(sub.Localizations) == 0 {
			checks = append(checks, CheckResult{
				ID:           "subscriptions.diagnostics.localization_missing",
				Severity:     SeverityWarning,
				Field:        "localizations",
				ResourceType: "subscription",
				ResourceID:   strings.TrimSpace(sub.ID),
				Message:      fmt.Sprintf("%s has no localizations (display name and description)", label),
				Remediation:  "Create at least one subscription localization with display name and description via App Store Connect or `asc subscriptions localizations create`",
			})
		} else {
			for _, loc := range sub.Localizations {
				var missing []string
				locale := strings.TrimSpace(loc.Locale)
				if locale == "" {
					locale = "(unknown locale)"
				}
				if strings.TrimSpace(loc.Name) == "" {
					missing = append(missing, "display name")
				}
				if strings.TrimSpace(loc.Description) == "" {
					missing = append(missing, "description")
				}
				if len(missing) > 0 {
					checks = append(checks, CheckResult{
						ID:           "subscriptions.diagnostics.localization_incomplete",
						Severity:     SeverityWarning,
						Field:        "localizations",
						ResourceType: "subscription",
						ResourceID:   strings.TrimSpace(sub.ID),
						Locale:       locale,
						Message:      fmt.Sprintf("%s localization for %s is missing: %s", label, locale, strings.Join(missing, ", ")),
						Remediation:  "Complete the missing fields for this subscription localization",
					})
				}
			}
		}

		// Check pricing.
		if sub.ReviewScreenshotCheckSkipped {
			remediation := strings.TrimSpace(sub.ReviewScreenshotCheckReason)
			if remediation == "" {
				remediation = "Review this subscription's App Review screenshot in App Store Connect; validation could not verify it automatically"
			}
			checks = append(checks, CheckResult{
				ID:           "subscriptions.diagnostics.review_screenshot_unverified",
				Severity:     SeverityInfo,
				Field:        "reviewScreenshot",
				ResourceType: "subscription",
				ResourceID:   strings.TrimSpace(sub.ID),
				Message:      fmt.Sprintf("Could not verify whether %s has an App Review screenshot", label),
				Remediation:  remediation,
			})
		} else if strings.TrimSpace(sub.ReviewScreenshotID) == "" {
			checks = append(checks, CheckResult{
				ID:           "subscriptions.diagnostics.review_screenshot_missing",
				Severity:     SeverityWarning,
				Field:        "reviewScreenshot",
				ResourceType: "subscription",
				ResourceID:   strings.TrimSpace(sub.ID),
				Message:      fmt.Sprintf("%s has no App Review screenshot", label),
				Remediation:  "Upload a subscription App Review screenshot via `asc subscriptions review screenshots create`",
			})
		}

		if sub.AvailabilityCheckSkipped {
			remediation := strings.TrimSpace(sub.AvailabilityCheckSkipReason)
			if remediation == "" {
				remediation = "Review this subscription's availability in App Store Connect; validation could not verify it automatically"
			}
			checks = append(checks, CheckResult{
				ID:           "subscriptions.diagnostics.availability_unverified",
				Severity:     SeverityInfo,
				Field:        "subscriptionAvailability",
				ResourceType: "subscription",
				ResourceID:   strings.TrimSpace(sub.ID),
				Message:      fmt.Sprintf("Could not verify whether %s has subscription availability configured", label),
				Remediation:  remediation,
			})
		} else if strings.TrimSpace(sub.AvailabilityID) == "" {
			checks = append(checks, CheckResult{
				ID:           "subscriptions.diagnostics.availability_missing",
				Severity:     SeverityWarning,
				Field:        "subscriptionAvailability",
				ResourceType: "subscription",
				ResourceID:   strings.TrimSpace(sub.ID),
				Message:      fmt.Sprintf("%s has no subscription availability configured", label),
				Remediation:  "Configure subscription availability via `asc subscriptions availability edit`",
			})
		} else if len(sortedUniqueNonEmpty(sub.AvailabilityTerritories)) == 0 {
			checks = append(checks, CheckResult{
				ID:           "subscriptions.diagnostics.availability_territories_missing",
				Severity:     SeverityWarning,
				Field:        "subscriptionAvailability",
				ResourceType: "subscription",
				ResourceID:   strings.TrimSpace(sub.ID),
				Message:      fmt.Sprintf("%s has subscription availability configured but no available territories", label),
				Remediation:  "Enable at least one subscription availability territory via `asc subscriptions availability edit`",
			})
		}

		if sub.PriceCheckSkipped {
			remediation := strings.TrimSpace(sub.PriceCheckSkipReason)
			if remediation == "" {
				remediation = "Review this subscription's pricing in App Store Connect; validation could not verify it automatically"
			}
			checks = append(checks, CheckResult{
				ID:           "subscriptions.diagnostics.pricing_unverified",
				Severity:     SeverityInfo,
				Field:        "pricing",
				ResourceType: "subscription",
				ResourceID:   strings.TrimSpace(sub.ID),
				Message:      fmt.Sprintf("Could not verify whether %s has territory prices configured", label),
				Remediation:  remediation,
			})
		} else if sub.PriceCount == 0 {
			checks = append(checks, CheckResult{
				ID:           "subscriptions.diagnostics.pricing_missing",
				Severity:     SeverityWarning,
				Field:        "pricing",
				ResourceType: "subscription",
				ResourceID:   strings.TrimSpace(sub.ID),
				Message:      fmt.Sprintf("%s has no territory prices configured", label),
				Remediation:  "Set prices for all available territories using `asc subscriptions pricing equalize` or `asc subscriptions pricing prices set`",
			})
		} else if !sub.AvailabilityCheckSkipped && strings.TrimSpace(sub.AvailabilityID) != "" && len(sub.AvailabilityTerritories) > 0 {
			missing := missingValues(sub.AvailabilityTerritories, sub.PriceTerritories)
			if len(missing) > 0 {
				checks = append(checks, CheckResult{
					ID:           "subscriptions.diagnostics.availability_pricing_gap",
					Severity:     SeverityWarning,
					Field:        "pricing",
					ResourceType: "subscription",
					ResourceID:   strings.TrimSpace(sub.ID),
					Message:      fmt.Sprintf("%s is missing price records for subscription availability territories: %s", label, strings.Join(missing, ",")),
					Remediation:  "Set prices for each subscription availability territory using `asc subscriptions pricing equalize` or `asc subscriptions pricing prices set`",
				})
			}
		}
	}

	return checks
}
