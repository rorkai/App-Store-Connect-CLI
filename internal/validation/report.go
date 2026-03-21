package validation

// Validate runs all validation rules and returns a report.
func Validate(input Input, strict bool) Report {
	activeMonetization := hasActiveMonetization(input.Subscriptions, input.IAPs)
	reviewRelevantSubscriptions := hasReviewRelevantSubscriptions(input.Subscriptions)
	availableTerritories := input.AvailableTerritories
	if input.PricingCoverageSkipReason != "" {
		availableTerritories = 0
	}

	checks := make([]CheckResult, 0)
	checks = append(checks, metadataLengthChecks(input.VersionLocalizations, input.AppInfoLocalizations)...)
	checks = append(checks, requiredFieldChecks(input.PrimaryLocale, input.VersionString, input.VersionState, activeMonetization, input.VersionLocalizations, input.AppInfoLocalizations)...)
	checks = append(checks, reviewDetailsChecks(input.ReviewDetails)...)
	checks = append(checks, categoryChecks(input.AppInfoID, input.PrimaryCategoryID)...)
	checks = append(checks, buildChecks(input.Build)...)
	checks = append(checks, pricingChecks(input.AppID, input.PriceScheduleID, input.PricingFetchSkipReason)...)
	checks = append(checks, availabilityChecks(input.AppID, input.AvailabilityID, input.AvailableTerritories, input.AvailabilityFetchSkipReason)...)
	checks = append(checks, screenshotPresenceChecks(input.PrimaryLocale, input.VersionLocalizations, input.ScreenshotSets)...)
	checks = append(checks, screenshotChecks(input.Platform, input.ScreenshotSets)...)
	checks = append(checks, subscriptionFetchChecks(input.SubscriptionFetchSkipReason)...)
	checks = append(checks, subscriptionImageChecks(input.Subscriptions)...)
	checks = append(checks, subscriptionReviewReadinessChecks(input.Subscriptions)...)
	checks = append(checks, subscriptionPricingVerificationChecks(input.Subscriptions)...)
	checks = append(checks, subscriptionMetadataDiagnostics(input.Subscriptions)...)
	checks = append(checks, subscriptionPricingCoverageSkipChecks(input.AppID, input.PricingCoverageSkipReason)...)
	checks = append(checks, subscriptionPricingCoverageChecks(input.Subscriptions, availableTerritories)...)
	checks = append(checks, iapFetchChecks(input.IAPFetchSkipReason)...)
	checks = append(checks, iapReviewReadinessChecks(input.IAPs)...)
	checks = append(checks, ageRatingChecks(input.AgeRatingDeclaration)...)
	checks = append(checks, releaseChecks(input.ReleaseType, input.EarliestReleaseDate)...)
	checks = append(checks, legalChecks(input.Copyright, activeMonetization, reviewRelevantSubscriptions, input.VersionLocalizations, input.AppInfoLocalizations)...)

	summary := summarize(checks, strict)

	return Report{
		AppID:         input.AppID,
		VersionID:     input.VersionID,
		VersionString: input.VersionString,
		Platform:      input.Platform,
		Summary:       summary,
		Checks:        checks,
		Strict:        strict,
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
