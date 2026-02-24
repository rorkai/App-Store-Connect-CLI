package validation

// Validate runs all validation rules and returns a report.
func Validate(input Input, strict bool) Report {
	checks := make([]CheckResult, 0)
	checks = append(checks, metadataLengthChecks(input.VersionLocalizations, input.AppInfoLocalizations)...)
	checks = append(checks, requiredFieldChecks(input.PrimaryLocale, input.VersionString, input.VersionState, input.VersionLocalizations, input.AppInfoLocalizations)...)
	checks = append(checks, reviewDetailsChecks(input.ReviewDetails)...)
	checks = append(checks, categoryChecks(input.AppInfoID, input.PrimaryCategoryID)...)
	checks = append(checks, buildChecks(input.Build)...)
	checks = append(checks, pricingChecks(input.AppID, input.PriceScheduleID)...)
	checks = append(checks, availabilityChecks(input.AppID, input.AvailabilityID, input.AvailableTerritories)...)
	checks = append(checks, screenshotPresenceChecks(input.PrimaryLocale, input.VersionLocalizations, input.ScreenshotSets)...)
	checks = append(checks, screenshotChecks(input.Platform, input.ScreenshotSets)...)
	checks = append(checks, ageRatingChecks(input.AgeRatingDeclaration)...)

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
