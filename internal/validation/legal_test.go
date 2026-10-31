package validation

import (
	"fmt"
	"testing"
	"time"
)

func TestLegalChecks_CopyrightEmpty(t *testing.T) {
	for _, value := range []string{"", "   "} {
		checks := legalChecks(value, false, false, nil, nil)
		if len(checks) != 1 || checks[0].ID != "legal.required.copyright" {
			t.Fatalf("copyright checks for %q = %+v, want only legal.required.copyright", value, checks)
		}
	}
}

func TestLegalChecks_CopyrightPresent(t *testing.T) {
	checks := legalChecks(
		"2026 My Company", false, false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://example.com"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy"}},
	)
	if hasCheckID(checks, "legal.required.copyright") {
		t.Fatal("did not expect copyright check when copyright is set")
	}
}

func TestLegalChecks_CopyrightAcceptsAppStoreConnectFreeTextForms(t *testing.T) {
	currentYear := time.Now().Year()
	for _, value := range []string{
		fmt.Sprintf("© %d Acme Inc.", currentYear),
		fmt.Sprintf("(c) %d Acme Inc.", currentYear),
		fmt.Sprintf("Copyright %d Acme Inc.", currentYear),
		fmt.Sprintf("2019-%d Acme Inc.", currentYear),
		fmt.Sprintf("%d, Acme Inc.", currentYear),
	} {
		checks := legalChecks(value, false, false, nil, nil)
		if hasCheckID(checks, "legal.format.copyright_year") {
			t.Fatalf("copyright %q reported legal.format.copyright_year, want no year check", value)
		}
	}
}

func TestLegalChecks_CopyrightRangeEndingBeforeCurrentYear(t *testing.T) {
	currentYear := time.Now().Year()
	tests := []struct {
		value        string
		wantOutdated bool
	}{
		{value: fmt.Sprintf("2019-%d Acme Inc.", currentYear-1), wantOutdated: true},
		{value: fmt.Sprintf("© 2019 – %d Acme Inc.", currentYear-2), wantOutdated: true},
		{value: fmt.Sprintf("2019-%d Acme Inc.", currentYear), wantOutdated: false},
		{value: "2019 Acme Inc.", wantOutdated: false},
		{value: fmt.Sprintf("%d-%d Acme Inc.", currentYear-1, currentYear-3), wantOutdated: false},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			checks := legalChecks(test.value, false, false, nil, nil)
			var found []CheckResult
			for _, check := range checks {
				if check.ID == "legal.format.copyright_year_outdated" {
					found = append(found, check)
				}
			}
			if !test.wantOutdated {
				if len(found) != 0 {
					t.Fatalf("checks = %+v, want no outdated copyright check", checks)
				}
				return
			}
			if len(found) != 1 || found[0].Severity != SeverityInfo || found[0].Remediation == "" {
				t.Fatalf("checks = %+v, want one outdated copyright info check with remediation", checks)
			}
		})
	}
}

func TestLegalChecks_CopyrightYearAdvisesInsteadOfBlocking(t *testing.T) {
	checks := legalChecks("Acme Inc.", false, false, nil, nil)

	var found []CheckResult
	for _, check := range checks {
		if check.ID == "legal.format.copyright_year" {
			found = append(found, check)
		}
	}
	if len(found) != 1 {
		t.Fatalf("copyright-year checks = %+v, want exactly one", checks)
	}
	if found[0].Severity != SeverityWarning {
		t.Fatalf("copyright-year severity = %q, want %q", found[0].Severity, SeverityWarning)
	}
	if summarize(checks, false).Blocking != 0 {
		t.Fatalf("copyright-year check is blocking in non-strict mode: %+v", summarize(checks, false))
	}
}

func TestHasValidLeadingCopyrightYear(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "past year", value: "2016 Acme", want: true},
		{name: "current year", value: "2026 Acme", want: true},
		{name: "past year before 1900", value: "1899 Acme", want: true},
		{name: "surrounding whitespace", value: "  2026 Acme  ", want: true},
		{name: "unicode whitespace", value: "\u00a02026\tAcme\u00a0", want: true},
		{name: "year without owner grammar", value: "2026", want: true},
		{name: "copyright sign prefix", value: "\u00a9 2026 Acme Inc.", want: true},
		{name: "copyright sign joined to year", value: "\u00a92026 Acme Inc.", want: true},
		{name: "parenthesized c prefix", value: "(c) 2026 Acme", want: true},
		{name: "parenthesized uppercase c prefix", value: "(C) 2026 Acme", want: true},
		{name: "copyright word prefix", value: "Copyright 2026 Acme", want: true},
		{name: "copyright word and sign prefix", value: "Copyright \u00a9 2026 Acme", want: true},
		{name: "lowercase copyright word prefix", value: "copyright 2026 Acme", want: true},
		{name: "year range", value: "2019-2026 Acme Inc.", want: true},
		{name: "spaced year range", value: "2019 - 2026 Acme Inc.", want: true},
		{name: "en dash year range", value: "2019\u20132026 Acme Inc.", want: true},
		{name: "copyright sign with year range", value: "\u00a9 2019-2026 Acme Inc.", want: true},
		{name: "comma after year", value: "2026, Acme", want: true},
		{name: "period after year", value: "2026. Acme", want: true},
		{name: "marker without year", value: "\u00a9 Acme Inc.", want: false},
		{name: "year zero", value: "0000 Acme", want: false},
		{name: "missing year", value: "Acme", want: false},
		{name: "year is not leading", value: "Acme 2026", want: false},
		{name: "year joined to owner", value: "2026会社", want: false},
		{name: "five digit prefix", value: "20260 Acme", want: false},
		{name: "non ASCII digits", value: "２０２６ Acme", want: false},
		{name: "future year", value: "2027 Acme", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := hasValidLeadingCopyrightYear(test.value, 2026); got != test.want {
				t.Fatalf("hasValidLeadingCopyrightYear(%q, 2026) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}

func TestLegalChecks_InvalidSupportURL(t *testing.T) {
	checks := legalChecks(
		"2026 My Company", false, false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "not-a-url"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy"}},
	)
	if !hasCheckID(checks, "legal.format.support_url") {
		t.Fatal("expected support URL format check")
	}
}

func TestLegalChecks_ValidSupportURL(t *testing.T) {
	checks := legalChecks(
		"2026 My Company", false, false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://example.com/support?q=1"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy"}},
	)
	if hasCheckID(checks, "legal.format.support_url") {
		t.Fatal("did not expect support URL format check for valid URL with query params")
	}
}

func TestLegalChecks_EmptySupportURL_NoFormatCheck(t *testing.T) {
	checks := legalChecks(
		"2026 My Company", false, false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: ""}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy"}},
	)
	if hasCheckID(checks, "legal.format.support_url") {
		t.Fatal("should not format-check an empty URL (handled by required check)")
	}
}

func TestLegalChecks_InvalidMarketingURL(t *testing.T) {
	checks := legalChecks(
		"2026 My Company", false, false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://example.com", MarketingURL: "ftp://bad"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy"}},
	)
	if !hasCheckID(checks, "legal.format.marketing_url") {
		t.Fatal("expected marketing URL format check")
	}
}

func TestLegalChecks_InvalidPrivacyPolicyURL(t *testing.T) {
	checks := legalChecks(
		"2026 My Company", false, false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://example.com"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "not-a-url"}},
	)
	if !hasCheckID(checks, "legal.format.privacy_policy_url") {
		t.Fatal("expected privacy policy URL format check")
	}
}

func TestLegalChecks_InvalidPrivacyChoicesURL(t *testing.T) {
	checks := legalChecks(
		"2026 My Company", false, false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://example.com"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy", PrivacyChoicesURL: "bad-url"}},
	)
	if !hasCheckID(checks, "legal.format.privacy_choices_url") {
		t.Fatal("expected privacy choices URL format check")
	}
}

func TestLegalChecks_URLSchemeNoHost(t *testing.T) {
	checks := legalChecks(
		"2026 My Company", false, false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy"}},
	)
	if !hasCheckID(checks, "legal.format.support_url") {
		t.Fatal("expected support URL format check for scheme-only URL with no host")
	}
}

func TestLegalChecks_HTTPURLAccepted(t *testing.T) {
	checks := legalChecks(
		"2026 My Company", false, false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "http://example.com"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy"}},
	)
	if hasCheckID(checks, "legal.format.support_url") {
		t.Fatal("http:// URLs should be accepted (not just https://)")
	}
}

func TestLegalChecks_RawSpaceInURLRejected(t *testing.T) {
	checks := legalChecks(
		"2026 My Company", false, false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://example.com/hello world"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy"}},
	)
	if !hasCheckID(checks, "legal.format.support_url") {
		t.Fatal("expected support URL format check for URL containing raw whitespace")
	}
}

func TestLegalChecks_EmptyHostnameRejected(t *testing.T) {
	checks := legalChecks(
		"2026 My Company", false, false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://user@:80"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy"}},
	)
	if !hasCheckID(checks, "legal.format.support_url") {
		t.Fatal("expected support URL format check for malformed authority with empty hostname")
	}
}

func TestLegalChecks_PrivacyPolicyRequired_WithIAPs(t *testing.T) {
	checks := legalChecks(
		"2026 My Company", true, false,
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
	checks := legalChecks(
		"2026 My Company", false, false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://example.com"}},
		[]AppInfoLocalization{{Locale: "en-US"}},
	)
	if hasCheckID(checks, "legal.required.privacy_policy_url") {
		t.Fatal("did not expect privacy policy error when no subscriptions/IAPs")
	}
}

func TestLegalChecks_MultipleLocales_InvalidURLs(t *testing.T) {
	checks := legalChecks(
		"2026 Co", false, false,
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

func TestHasTermsOfUseLink(t *testing.T) {
	tests := []struct {
		name        string
		description string
		want        bool
	}{
		{
			name:        "apple standard eula url",
			description: "Terms of Use: https://www.apple.com/legal/internet-services/itunes/dev/stdeula/",
			want:        true,
		},
		{
			name:        "mixed case scheme accepted",
			description: "Terms of Use: HTTPS://example.com/terms",
			want:        true,
		},
		{
			name:        "custom url with nearby terms wording",
			description: "Read our Terms of Use: https://example.com/legal/subscriber-agreement before subscribing.",
			want:        true,
		},
		{
			name:        "tos path url accepted",
			description: "https://example.com/tos",
			want:        true,
		},
		{
			name:        "keyword without url",
			description: "Terms of Use available in the app settings.",
			want:        false,
		},
		{
			name:        "random url without terms context",
			description: "Learn more at https://example.com/about",
			want:        false,
		},
		{
			name:        "photos url is not mistaken for tos",
			description: "Gallery: https://example.com/photos",
			want:        false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := HasTermsOfUseLink(test.description); got != test.want {
				t.Fatalf("HasTermsOfUseLink() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestValidate_TermsOfUseLinkRequiredForReviewRelevantSubscriptions(t *testing.T) {
	tests := []struct {
		name  string
		state string
	}{
		{name: "ready to submit", state: "READY_TO_SUBMIT"},
		{name: "missing metadata", state: "MISSING_METADATA"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := Validate(Input{
				AppID:         "app-1",
				VersionID:     "ver-1",
				VersionString: "2.0",
				VersionState:  "PREPARE_FOR_SUBMISSION",
				Copyright:     "2026 Co",
				VersionLocalizations: []VersionLocalization{
					{Locale: "en-US", Description: "Subscription description", Keywords: "kw", SupportURL: "https://example.com"},
				},
				AppInfoLocalizations: []AppInfoLocalization{
					{Locale: "en-US", Name: "App", PrivacyPolicyURL: "https://example.com/privacy"},
				},
				Subscriptions:     []Subscription{{ProductID: "sub-1", State: test.state}},
				PrimaryCategoryID: "cat-1",
			}, false)

			if !hasCheckID(report.Checks, "legal.subscription.terms_of_use_link") {
				t.Fatalf("expected terms link warning for state %q, got %+v", test.state, report.Checks)
			}
		})
	}
}

func TestValidate_TermsOfUseLinkNotRequiredWithoutSubscriptions(t *testing.T) {
	report := Validate(Input{
		AppID:         "app-1",
		VersionID:     "ver-1",
		VersionString: "2.0",
		VersionState:  "PREPARE_FOR_SUBMISSION",
		Copyright:     "2026 Co",
		VersionLocalizations: []VersionLocalization{
			{Locale: "en-US", Description: "Plain description", Keywords: "kw", SupportURL: "https://example.com"},
		},
		AppInfoLocalizations: []AppInfoLocalization{
			{Locale: "en-US", Name: "App", PrivacyPolicyURL: "https://example.com/privacy"},
		},
		PrimaryCategoryID: "cat-1",
	}, false)

	if hasCheckID(report.Checks, "legal.subscription.terms_of_use_link") {
		t.Fatalf("did not expect terms link warning without subscriptions, got %+v", report.Checks)
	}
}

func TestLegalChecks_AllValid_NoChecks(t *testing.T) {
	checks := legalChecks(
		"2026 My Company", false, false,
		[]VersionLocalization{{Locale: "en-US", SupportURL: "https://example.com", MarketingURL: "https://example.com/marketing"}},
		[]AppInfoLocalization{{Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy", PrivacyChoicesURL: "https://example.com/choices"}},
	)
	if len(checks) != 0 {
		t.Fatalf("expected no checks for fully valid input, got %d: %v", len(checks), checks)
	}
}

func TestIsValidHTTPURL(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{value: "https://example.com", want: true},
		{value: "http://example.com/support", want: true},
		{value: "  https://example.com/support  ", want: true},
		{value: "example.com", want: false},
		{value: "www.example.com/support", want: false},
		{value: "ftp://example.com", want: false},
		{value: "https://", want: false},
		{value: "https://exa mple.com", want: false},
		{value: "", want: false},
	}

	for _, test := range tests {
		if got := IsValidHTTPURL(test.value); got != test.want {
			t.Fatalf("IsValidHTTPURL(%q) = %v, want %v", test.value, got, test.want)
		}
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

func TestTVOSPrivacyPolicyTextChecks(t *testing.T) {
	locs := []AppInfoLocalization{
		{ID: "a", Locale: "ja", PrivacyPolicyURL: "https://example.com/privacy"},
		{ID: "b", Locale: "en-US", PrivacyPolicyURL: "https://example.com/privacy", PrivacyPolicyText: "  We collect nothing.  "},
		{ID: "c", Locale: "ko", PrivacyPolicyText: "   "},
	}

	checks := tvOSPrivacyPolicyTextChecks("TV_OS", locs)
	if len(checks) != 2 {
		t.Fatalf("expected an error for the two localizations without text, got %d: %#v", len(checks), checks)
	}
	for _, check := range checks {
		if check.ID != "legal.required.privacy_policy_text" || check.Severity != SeverityError || check.Field != "privacyPolicyText" {
			t.Fatalf("unexpected check %#v", check)
		}
	}
	if checks[0].Locale != "ja" || checks[0].ResourceID != "a" || checks[1].Locale != "ko" || checks[1].ResourceID != "c" {
		t.Fatalf("checks must name the failing localizations in order, got %#v", checks)
	}

	if got := tvOSPrivacyPolicyTextChecks(" tv_os ", locs); len(got) != 2 {
		t.Fatalf("platform match must ignore case and whitespace, got %d checks", len(got))
	}
	for _, platform := range []string{"IOS", "MAC_OS", ""} {
		if got := tvOSPrivacyPolicyTextChecks(platform, locs); len(got) != 0 {
			t.Fatalf("platform %q must not require privacy policy text, got %#v", platform, got)
		}
	}
}
