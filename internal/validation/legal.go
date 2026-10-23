package validation

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const AppleStandardEULAURL = "https://www.apple.com/legal/internet-services/itunes/dev/stdeula/"

var (
	descriptionURLPattern = regexp.MustCompile(`(?i)https?://[^\s]+`)
	// App Store Connect stores copyright as free text, so the leading acquisition
	// year may be preceded by copyright markers and may be written as a range.
	copyrightYearPattern = regexp.MustCompile(`(?i)^(?:(?:copyright|copr\.|\(c\)|©)[\s.,]*)*([0-9]{4})(?:\s*[-–—]\s*([0-9]{4}))?(?:[\s,.;:]|$)`)
	termsKeywordPattern  = regexp.MustCompile(`(?i)\bterms of use\b|\bterms\b|\beula\b`)
	termsURLPattern      = regexp.MustCompile(`(^|[^a-z0-9])(terms?|eula|tos|termsofservice)([^a-z0-9]|$)`)
)

func legalChecks(copyright string, hasActiveMonetization bool, hasReviewRelevantSubscriptions bool, versionLocs []VersionLocalization, appInfoLocs []AppInfoLocalization) []CheckResult {
	var checks []CheckResult
	currentYear := time.Now().Year()

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
	} else if !hasValidLeadingCopyrightYear(copyright, currentYear) {
		// App Store Connect accepts copyright as free text, so this is advisory:
		// it flags values that do not lead with the acquisition year Apple asks for.
		checks = append(checks, CheckResult{
			ID:           "legal.format.copyright_year",
			Severity:     SeverityWarning,
			Field:        "copyright",
			ResourceType: "appStoreVersion",
			Message:      "copyright does not start with the four-digit year the rights were obtained",
			Remediation:  "Apple recommends a current or past year followed by the rights owner, for example: \"2020 Your Company\"",
		})
	} else if start, end := leadingCopyrightYears(copyright); start <= end && end < currentYear {
		checks = append(checks, CheckResult{
			ID:           "legal.format.copyright_year_outdated",
			Severity:     SeverityInfo,
			Field:        "copyright",
			ResourceType: "appStoreVersion",
			Message:      fmt.Sprintf("copyright year range ends in %d, before the current year", end),
			Remediation:  fmt.Sprintf("Extend the range to %d or keep only the year the rights were obtained", currentYear),
		})
	}

	// URL format checks on version localizations.
	for _, loc := range versionLocs {
		if u := strings.TrimSpace(loc.SupportURL); u != "" {
			if !IsValidHTTPURL(u) {
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
			if !IsValidHTTPURL(u) {
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

		if hasReviewRelevantSubscriptions && strings.TrimSpace(loc.Description) != "" && !HasTermsOfUseLink(loc.Description) {
			checks = append(checks, CheckResult{
				ID:           "legal.subscription.terms_of_use_link",
				Severity:     SeverityWarning,
				Locale:       loc.Locale,
				Field:        "description",
				ResourceType: "appStoreVersionLocalization",
				ResourceID:   loc.ID,
				Message:      "description is missing a Terms of Use / EULA link for subscription review",
				Remediation:  `Append a Terms of Use / EULA link to the description, for example: "Terms of Use: https://www.apple.com/legal/internet-services/itunes/dev/stdeula/" or your custom terms URL`,
			})
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

		if privacyURL != "" && !IsValidHTTPURL(privacyURL) {
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
			if !IsValidHTTPURL(u) {
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

// hasValidLeadingCopyrightYear reports whether the copyright value leads with a
// four-digit acquisition year, allowing the copyright markers and year ranges
// App Store Connect accepts (for example "© 2026 Acme" or "2019-2026 Acme").
func hasValidLeadingCopyrightYear(value string, currentYear int) bool {
	year, _ := leadingCopyrightYears(value)
	return year > 0 && year <= currentYear
}

// leadingCopyrightYears returns the leading year and, for a range such as
// "2019-2024 Acme", its closing year; missing years are 0.
func leadingCopyrightYears(value string) (start int, end int) {
	// Collapse Unicode whitespace so the pattern only has to handle ASCII gaps.
	match := copyrightYearPattern.FindStringSubmatch(strings.Join(strings.Fields(value), " "))
	if len(match) != 3 {
		return 0, 0
	}
	start, _ = strconv.Atoi(match[1])
	end, _ = strconv.Atoi(match[2])
	return start, end
}

// HasTermsOfUseLink reports whether a description includes a functional Terms of Use / EULA link.
func HasTermsOfUseLink(description string) bool {
	for _, match := range descriptionURLPattern.FindAllStringIndex(description, -1) {
		rawURL := description[match[0]:match[1]]
		normalizedURL := normalizeDescriptionURL(rawURL)
		if !IsValidHTTPURL(normalizedURL) {
			continue
		}
		if isAppleStandardEULAURL(normalizedURL) || urlLooksLikeTermsLink(normalizedURL) {
			return true
		}

		contextStart := max(match[0]-80, 0)
		contextEnd := min(match[1]+80, len(description))
		if termsKeywordPattern.MatchString(description[contextStart:contextEnd]) {
			return true
		}
	}

	return false
}

func normalizeDescriptionURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimLeft(trimmed, `"'([{<`)
	trimmed = strings.TrimRight(trimmed, `"'.,);:!?]}>`)
	return trimmed
}

func isAppleStandardEULAURL(raw string) bool {
	return strings.EqualFold(strings.TrimSuffix(raw, "/"), strings.TrimSuffix(AppleStandardEULAURL, "/"))
}

func urlLooksLikeTermsLink(raw string) bool {
	lower := strings.ToLower(raw)
	return termsURLPattern.MatchString(lower)
}

// IsValidHTTPURL reports whether a value is an absolute HTTP/HTTPS URL with a
// hostname and no raw whitespace, matching the URL syntax App Store Connect
// accepts for metadata URL fields. It is exported so offline metadata checks
// apply the same rule as the online validation report.
func IsValidHTTPURL(s string) bool {
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
