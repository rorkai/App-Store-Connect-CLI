package validation

import (
	"net/url"
	"strings"
)

func legalChecks(copyright string, hasActiveMonetization bool, versionLocs []VersionLocalization, appInfoLocs []AppInfoLocalization) []CheckResult {
	var checks []CheckResult

	// Copyright is required by Apple.
	if strings.TrimSpace(copyright) == "" {
		checks = append(checks, CheckResult{
			ID:           "legal.required.copyright",
			Severity:     SeverityError,
			Field:        "copyright",
			ResourceType: "appStoreVersion",
			Message:      "copyright is required",
			Remediation:  "Set copyright via: asc versions update --version-id VERSION_ID --copyright \"2026 Your Company\"",
		})
	}

	// URL format checks on version localizations.
	for _, loc := range versionLocs {
		if u := strings.TrimSpace(loc.SupportURL); u != "" {
			if !isValidHTTPURL(u) {
				checks = append(checks, CheckResult{
					ID:           "legal.format.support_url",
					Severity:     SeverityWarning,
					Locale:       loc.Locale,
					Field:        "supportUrl",
					ResourceType: "appStoreVersionLocalization",
					ResourceID:   loc.ID,
					Message:      "support URL is not a valid HTTP/HTTPS URL",
					Remediation:  "Provide a valid https:// URL for support",
				})
			}
		}
		if u := strings.TrimSpace(loc.MarketingURL); u != "" {
			if !isValidHTTPURL(u) {
				checks = append(checks, CheckResult{
					ID:           "legal.format.marketing_url",
					Severity:     SeverityWarning,
					Locale:       loc.Locale,
					Field:        "marketingUrl",
					ResourceType: "appStoreVersionLocalization",
					ResourceID:   loc.ID,
					Message:      "marketing URL is not a valid HTTP/HTTPS URL",
					Remediation:  "Provide a valid https:// URL for marketing",
				})
			}
		}
	}

	// URL format + conditional requirement checks on app info localizations.
	for _, loc := range appInfoLocs {
		privacyURL := strings.TrimSpace(loc.PrivacyPolicyURL)

		// When app has active subscriptions or IAPs, privacy policy is required (error).
		if privacyURL == "" && hasActiveMonetization {
			checks = append(checks, CheckResult{
				ID:           "legal.required.privacy_policy_url",
				Severity:     SeverityError,
				Locale:       loc.Locale,
				Field:        "privacyPolicyUrl",
				ResourceType: "appInfoLocalization",
				ResourceID:   loc.ID,
				Message:      "privacy policy URL is required for apps with subscriptions or in-app purchases",
				Remediation:  "Provide a privacy policy URL for this localization",
			})
		}

		if privacyURL != "" && !isValidHTTPURL(privacyURL) {
			checks = append(checks, CheckResult{
				ID:           "legal.format.privacy_policy_url",
				Severity:     SeverityWarning,
				Locale:       loc.Locale,
				Field:        "privacyPolicyUrl",
				ResourceType: "appInfoLocalization",
				ResourceID:   loc.ID,
				Message:      "privacy policy URL is not a valid HTTP/HTTPS URL",
				Remediation:  "Provide a valid https:// URL for privacy policy",
			})
		}

		if u := strings.TrimSpace(loc.PrivacyChoicesURL); u != "" {
			if !isValidHTTPURL(u) {
				checks = append(checks, CheckResult{
					ID:           "legal.format.privacy_choices_url",
					Severity:     SeverityWarning,
					Locale:       loc.Locale,
					Field:        "privacyChoicesUrl",
					ResourceType: "appInfoLocalization",
					ResourceID:   loc.ID,
					Message:      "privacy choices URL is not a valid HTTP/HTTPS URL",
					Remediation:  "Provide a valid https:// URL for privacy choices",
				})
			}
		}
	}

	return checks
}

// isValidHTTPURL returns true for absolute HTTP/HTTPS URLs with a hostname and no raw whitespace.
func isValidHTTPURL(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, " \t\r\n") {
		return false
	}

	u, err := url.ParseRequestURI(s)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != ""
}
