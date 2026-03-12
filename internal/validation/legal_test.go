package validation

import (
	"testing"
)

func TestLegalChecks_CopyrightEmpty(t *testing.T) {
	checks := legalChecks("", false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://example.com"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy"}},
	)
	if !hasCheckID(checks, "legal.required.copyright") {
		t.Fatal("expected copyright required check")
	}
}

func TestLegalChecks_CopyrightWhitespaceOnly(t *testing.T) {
	checks := legalChecks("   ", false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://example.com"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy"}},
	)
	if !hasCheckID(checks, "legal.required.copyright") {
		t.Fatal("expected copyright required check for whitespace-only")
	}
}

func TestLegalChecks_CopyrightPresent(t *testing.T) {
	checks := legalChecks("2026 My Company", false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://example.com"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy"}},
	)
	if hasCheckID(checks, "legal.required.copyright") {
		t.Fatal("did not expect copyright check when copyright is set")
	}
}

func TestLegalChecks_InvalidSupportURL(t *testing.T) {
	checks := legalChecks("2026 My Company", false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "not-a-url"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy"}},
	)
	if !hasCheckID(checks, "legal.format.support_url") {
		t.Fatal("expected support URL format check")
	}
}

func TestLegalChecks_ValidSupportURL(t *testing.T) {
	checks := legalChecks("2026 My Company", false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://example.com/support?q=1"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy"}},
	)
	if hasCheckID(checks, "legal.format.support_url") {
		t.Fatal("did not expect support URL format check for valid URL with query params")
	}
}

func TestLegalChecks_EmptySupportURL_NoFormatCheck(t *testing.T) {
	checks := legalChecks("2026 My Company", false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: ""}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy"}},
	)
	if hasCheckID(checks, "legal.format.support_url") {
		t.Fatal("should not format-check an empty URL (handled by required check)")
	}
}

func TestLegalChecks_InvalidMarketingURL(t *testing.T) {
	checks := legalChecks("2026 My Company", false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://example.com", MarketingURL: "ftp://bad"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy"}},
	)
	if !hasCheckID(checks, "legal.format.marketing_url") {
		t.Fatal("expected marketing URL format check")
	}
}

func TestLegalChecks_InvalidPrivacyPolicyURL(t *testing.T) {
	checks := legalChecks("2026 My Company", false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://example.com"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "not-a-url"}},
	)
	if !hasCheckID(checks, "legal.format.privacy_policy_url") {
		t.Fatal("expected privacy policy URL format check")
	}
}

func TestLegalChecks_InvalidPrivacyChoicesURL(t *testing.T) {
	checks := legalChecks("2026 My Company", false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://example.com"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy", PrivacyChoicesURL: "bad-url"}},
	)
	if !hasCheckID(checks, "legal.format.privacy_choices_url") {
		t.Fatal("expected privacy choices URL format check")
	}
}

func TestLegalChecks_URLSchemeNoHost(t *testing.T) {
	checks := legalChecks("2026 My Company", false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy"}},
	)
	if !hasCheckID(checks, "legal.format.support_url") {
		t.Fatal("expected support URL format check for scheme-only URL with no host")
	}
}

func TestLegalChecks_HTTPURLAccepted(t *testing.T) {
	checks := legalChecks("2026 My Company", false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "http://example.com"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy"}},
	)
	if hasCheckID(checks, "legal.format.support_url") {
		t.Fatal("http:// URLs should be accepted (not just https://)")
	}
}

func TestLegalChecks_RawSpaceInURLRejected(t *testing.T) {
	checks := legalChecks("2026 My Company", false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://example.com/hello world"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy"}},
	)
	if !hasCheckID(checks, "legal.format.support_url") {
		t.Fatal("expected support URL format check for URL containing raw whitespace")
	}
}

func TestLegalChecks_EmptyHostnameRejected(t *testing.T) {
	checks := legalChecks("2026 My Company", false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://user@:80"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy"}},
	)
	if !hasCheckID(checks, "legal.format.support_url") {
		t.Fatal("expected support URL format check for malformed authority with empty hostname")
	}
}

func TestLegalChecks_PrivacyPolicyRequired_WithSubscriptions(t *testing.T) {
	checks := legalChecks("2026 My Company", true,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://example.com"}},
		[]AppInfoLocalization{{Locale: "en-US"}},
	)
	found := false
	for _, c := range checks {
		if c.ID == "legal.required.privacy_policy_url" && c.Severity == SeverityError {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected privacy policy error when app has subscriptions")
	}
}

func TestLegalChecks_PrivacyPolicyRequired_WithIAPs(t *testing.T) {
	checks := legalChecks("2026 My Company", true,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://example.com"}},
		[]AppInfoLocalization{{Locale: "en-US"}},
	)
	found := false
	for _, c := range checks {
		if c.ID == "legal.required.privacy_policy_url" && c.Severity == SeverityError {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected privacy policy error when app has IAPs")
	}
}

func TestLegalChecks_PrivacyPolicyNotRequired_NoSubscriptionsNoIAPs(t *testing.T) {
	checks := legalChecks("2026 My Company", false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://example.com"}},
		[]AppInfoLocalization{{Locale: "en-US"}},
	)
	if hasCheckID(checks, "legal.required.privacy_policy_url") {
		t.Fatal("did not expect privacy policy error when no subscriptions/IAPs")
	}
}

func TestLegalChecks_MultipleLocales_InvalidURLs(t *testing.T) {
	checks := legalChecks("2026 Co", false,
		[]VersionLocalization{
			{Locale: "en-US", SupportURL: "bad"},
			{Locale: "fr-FR", SupportURL: "also-bad"},
		},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://ok.com"}},
	)
	count := 0
	for _, c := range checks {
		if c.ID == "legal.format.support_url" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("expected 2 support URL format checks for 2 locales, got %d", count)
	}
}

func TestLegalChecks_AllValid_NoChecks(t *testing.T) {
	checks := legalChecks("2026 My Company", false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://example.com", MarketingURL: "https://example.com/marketing"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy", PrivacyChoicesURL: "https://example.com/choices"}},
	)
	if len(checks) != 0 {
		t.Fatalf("expected no checks for fully valid input, got %d: %v", len(checks), checks)
	}
}

func TestValidate_IncludesLegalChecks(t *testing.T) {
	report := Validate(Input{
		AppID:         "app-1",
		VersionID:     "ver-1",
		VersionString: "2.0",
		VersionState:  "PREPARE_FOR_SUBMISSION",
		VersionLocalizations: []VersionLocalization{
			{Locale: "en-US", Description: "desc", Keywords: "kw", SupportURL: "https://example.com"},
		},
		AppInfoLocalizations: []AppInfoLocalization{
			{Locale: "en-US", Name: "App", PrivacyPolicyURL: "https://example.com/privacy"},
		},
		PrimaryCategoryID: "cat-1",
	}, false)

	if !hasCheckID(report.Checks, "legal.required.copyright") {
		t.Fatal("expected legal.required.copyright in full Validate output")
	}
}

func TestValidate_NoDuplicatePrivacyPolicyChecks_WithSubscriptions(t *testing.T) {
	report := Validate(Input{
		AppID:         "app-1",
		VersionID:     "ver-1",
		VersionString: "2.0",
		VersionState:  "PREPARE_FOR_SUBMISSION",
		Copyright:     "2026 Co",
		VersionLocalizations: []VersionLocalization{
			{Locale: "en-US", Description: "desc", Keywords: "kw", SupportURL: "https://example.com"},
		},
		AppInfoLocalizations: []AppInfoLocalization{
			{Locale: "en-US", Name: "App"},
		},
		Subscriptions:     []Subscription{{ProductID: "sub-1", State: "APPROVED"}},
		PrimaryCategoryID: "cat-1",
	}, false)

	if !hasCheckID(report.Checks, "legal.required.privacy_policy_url") {
		t.Fatal("expected legal.required.privacy_policy_url error")
	}
	if hasCheckID(report.Checks, "metadata.recommended.privacy_policy_url") {
		t.Fatal("should suppress metadata.recommended.privacy_policy_url when legal.required fires")
	}
}

func TestValidate_PrivacyPolicyUsesActiveMonetizationOnly(t *testing.T) {
	report := Validate(Input{
		AppID:         "app-1",
		VersionID:     "ver-1",
		VersionString: "2.0",
		VersionState:  "PREPARE_FOR_SUBMISSION",
		Copyright:     "2026 Co",
		VersionLocalizations: []VersionLocalization{
			{Locale: "en-US", Description: "desc", Keywords: "kw", SupportURL: "https://example.com"},
		},
		AppInfoLocalizations: []AppInfoLocalization{
			{Locale: "en-US", Name: "App"},
		},
		Subscriptions: []Subscription{
			{ProductID: "sub-1", State: "DEVELOPER_REMOVED_FROM_SALE"},
		},
		IAPs: []IAP{
			{ProductID: "iap-1", State: "REMOVED_FROM_SALE"},
		},
		PrimaryCategoryID: "cat-1",
	}, false)

	if hasCheckID(report.Checks, "legal.required.privacy_policy_url") {
		t.Fatal("did not expect legal.required.privacy_policy_url when monetization is only removed from sale")
	}
	if !hasCheckID(report.Checks, "metadata.recommended.privacy_policy_url") {
		t.Fatal("expected metadata.recommended.privacy_policy_url when only removed monetization exists")
	}
}
