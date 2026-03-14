package validation

import (
	"fmt"
	"strings"
	"time"
)

// releaseChecks validates the version release configuration.
func releaseChecks(releaseType, earliestReleaseDate string) []CheckResult {
	rt := strings.ToUpper(strings.TrimSpace(releaseType))
	if rt == "" {
		return nil
	}

	var checks []CheckResult

	if rt == "SCHEDULED" {
		date := strings.TrimSpace(earliestReleaseDate)
		if date != "" {
			parsed, err := time.Parse(time.RFC3339, date)
			if err == nil && parsed.Before(time.Now()) {
				checks = append(checks, CheckResult{
					ID:          "release.scheduled_date_past",
					Severity:    SeverityWarning,
					Field:       "earliestReleaseDate",
					Message:     fmt.Sprintf("Scheduled release date %s is in the past; the app will be released immediately after approval", date),
					Remediation: "Update the earliest release date to a future date using `asc versions update --version-id VERSION --earliest-release-date DATE` or change the release type",
				})
			}
		}
	}

	if rt == "MANUAL" {
		checks = append(checks, CheckResult{
			ID:          "release.type_manual",
			Severity:    SeverityInfo,
			Field:       "releaseType",
			Message:     "Release type is MANUAL — the app will not be released automatically after approval",
			Remediation: "After approval, release manually using `asc versions release --version-id VERSION --confirm` or change the release type to AFTER_APPROVAL",
		})
	}

	return checks
}
