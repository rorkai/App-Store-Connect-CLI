package validation

import (
	"strings"
	"time"
)

// Validate runs all validation rules and returns a report.
func Validate(input Input, strict bool) Report {
	activeMonetization := hasActiveMonetization(input.Subscriptions, input.IAPs)
	reviewRelevantSubscriptions := hasReviewRelevantSubscriptions(input.Subscriptions)
	availableTerritories := input.AvailableTerritories
	appAvailableTerritories := input.AppAvailableTerritories
	pricingTerritoryCount := input.PricingTerritoryCount
	pricingTerritories := input.PricingTerritories
	if pricingTerritoryCount == 0 && len(pricingTerritories) == 0 {
		pricingTerritoryCount = availableTerritories
		pricingTerritories = appAvailableTerritories
	}
	if input.PricingCoverageSkipReason != "" {
		pricingTerritoryCount = 0
		pricingTerritories = nil
	}

	checks := make([]CheckResult, 0)
	checks = append(checks, metadataLengthChecks(input.VersionLocalizations, input.AppInfoLocalizations)...)
	checks = append(checks, requiredFieldChecks(input.PrimaryLocale, input.VersionString, input.VersionState, activeMonetization, input.VersionLocalizations, input.AppInfoLocalizations)...)
	checks = append(checks, reviewDetailsChecks(input.ReviewDetails)...)
	checks = append(checks, categoryChecks(input.AppInfoID, input.PrimaryCategoryID)...)
	checks = append(checks, contentRightsChecks(input.AppID, input.ContentRightsDeclaration)...)
	checks = append(checks, buildChecks(input.Build)...)
	checks = append(checks, buildSubmissionChecks(input.Build)...)
	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}
	checks = append(checks, uploadRequirementChecks(input.Platform, input.Build, input.BuildSDKName, now)...)
	checks = append(checks, pricingChecks(input.AppID, input.PriceScheduleID, input.BasePriceMissing, input.BaseTerritory, input.PricingFetchSkipReason)...)
	checks = append(checks, availabilityChecks(input.AppID, input.AvailabilityID, input.AvailableTerritories, input.AvailabilityFetchSkipReason)...)
	checks = append(checks, screenshotPresenceChecks(input.PrimaryLocale, input.VersionLocalizations, input.ScreenshotSets)...)
	checks = append(checks, ipadScreenshotChecks(input.Platform, input.PrimaryLocale, input.VersionLocalizations, input.ScreenshotSets, input.SupportsIPad)...)
	checks = append(checks, screenshotChecks(input.Platform, input.ScreenshotSets)...)
	checks = append(checks, subscriptionFetchChecks(input.SubscriptionFetchSkipReason)...)
	checks = append(checks, subscriptionImageChecks(input.Subscriptions)...)
	checks = append(checks, subscriptionReviewReadinessChecks(input.Subscriptions)...)
	checks = append(checks, subscriptionPricingVerificationChecks(input.Subscriptions)...)
	checks = append(checks, subscriptionMetadataDiagnostics(input.Subscriptions)...)
	checks = append(checks, subscriptionPricingCoverageSkipChecks(input.AppID, input.PricingCoverageSkipReason)...)
	checks = append(checks, subscriptionPricingCoverageChecks(input.Subscriptions, pricingTerritoryCount, pricingTerritories)...)
	checks = append(checks, iapFetchChecks(input.IAPFetchSkipReason)...)
	checks = append(checks, iapReviewReadinessChecks(input.IAPs)...)
	checks = append(checks, ageRatingChecks(input.AgeRatingDeclaration)...)
	checks = append(checks, releaseChecks(input.ReleaseType, input.EarliestReleaseDate)...)
	checks = append(checks, legalChecks(input.Copyright, activeMonetization, reviewRelevantSubscriptions, input.VersionLocalizations, input.AppInfoLocalizations)...)
	checks = append(checks, contentChecks(input.VersionLocalizations, input.AppInfoLocalizations)...)
	checks = append(checks, MetadataQualityChecks(input.VersionLocalizations, input.AppInfoLocalizations)...)
	checks = append(checks, customProductPageKeywordChecks(input.VersionLocalizations, input.CustomProductPageLocalizations, input.CustomProductPageKeywordsSkipReason)...)
	checks = append(checks, privacyPublishStateChecks(input.AppID)...)

	summary := summarize(checks, strict)

	return Report{
		AppID:                 input.AppID,
		VersionID:             input.VersionID,
		VersionString:         input.VersionString,
		VersionState:          input.VersionState,
		Platform:              input.Platform,
		Summary:               summary,
		Remediation:           BuildRemediation(checks, strict),
		Checks:                checks,
		Strict:                strict,
		HasActiveMonetization: activeMonetization,
		MonetizationKnown:     strings.TrimSpace(input.SubscriptionFetchSkipReason) == "" && strings.TrimSpace(input.IAPFetchSkipReason) == "",
		HasPaidAppPrice:       input.HasPaidAppPrice,
		AppPricingKnown:       input.AppPricingKnown,
	}
}

func summarize(checks []CheckResult, strict bool) Summary {
	summary := Summary{}
	for _, check := range checks {
		switch check.Severity {
		case SeverityError:
			summary.Errors++
		case SeverityWarning:
			summary.Warnings++
		case SeverityInfo:
			summary.Infos++
		}
	}
	summary.Blocking = summary.Errors
	if strict {
		summary.Blocking += summary.Warnings
	}
	return summary
}

// SummarizeChecks aggregates validation checks for callers constructing a
// partial report after a known upstream blocker.
func SummarizeChecks(checks []CheckResult, strict bool) Summary {
	return summarize(checks, strict)
}
