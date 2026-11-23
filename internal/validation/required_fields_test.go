package validation

import "testing"

func TestRequiredFieldChecks_MissingPrimaryLocale(t *testing.T) {
	checks := requiredFieldChecks("en-US", "1.2.3", nil, "PREPARE_FOR_SUBMISSION", false, []VersionLocalization{
		{Locale: "fr-FR", Description: "desc", Keywords: "kw", SupportURL: "https://example.com"},
	}, []AppInfoLocalization{
		{Locale: "fr-FR", Name: "Name", PrivacyPolicyURL: "https://example.com/privacy"},
	})

	if !hasCheckID(checks, "metadata.required.primary_locale") {
		t.Fatalf("expected primary locale check")
	}
}

func TestRequiredFieldChecks_MissingFields(t *testing.T) {
	checks := requiredFieldChecks("", "1.2.3", nil, "PREPARE_FOR_SUBMISSION", false, []VersionLocalization{
		{Locale: "en-US"},
	}, []AppInfoLocalization{
		{Locale: "en-US"},
	})

	if !hasCheckID(checks, "metadata.required.description") {
		t.Fatalf("expected description required check")
	}
	if !hasCheckID(checks, "metadata.required.keywords") {
		t.Fatalf("expected keywords required check")
	}
	if !hasCheckID(checks, "metadata.required.support_url") {
		t.Fatalf("expected support url required check")
	}
	if !hasCheckID(checks, "metadata.required.name") {
		t.Fatalf("expected name required check")
	}
}

func TestRequiredFieldChecks_SkipsWhatsNewOnInitialRelease(t *testing.T) {
	checks := requiredFieldChecks("", "1.0", nil, "PREPARE_FOR_SUBMISSION", false, []VersionLocalization{
		{Locale: "en-US", Description: "desc", Keywords: "kw", SupportURL: "https://example.com"},
	}, []AppInfoLocalization{
		{Locale: "en-US", Name: "Name", PrivacyPolicyURL: "https://example.com/privacy"},
	})

	if hasCheckID(checks, "metadata.required.whats_new") {
		t.Fatalf("did not expect whatsNew warning for initial release")
	}
}

func TestRequiredFieldChecks_WarnsWhatsNewOnUpdateRelease(t *testing.T) {
	checks := requiredFieldChecks("", "1.0.1", nil, "PREPARE_FOR_SUBMISSION", false, []VersionLocalization{
		{Locale: "en-US", Description: "desc", Keywords: "kw", SupportURL: "https://example.com"},
	}, []AppInfoLocalization{
		{Locale: "en-US", Name: "Name", PrivacyPolicyURL: "https://example.com/privacy"},
	})

	if !hasCheckID(checks, "metadata.required.whats_new") {
		t.Fatalf("expected whatsNew warning for update release")
	}
}

func TestRequiredFieldChecks_FailsForNonEditableVersionState(t *testing.T) {
	checks := requiredFieldChecks("", "1.2.3", nil, "WAITING_FOR_REVIEW", false, []VersionLocalization{
		{Locale: "en-US", Description: "desc", Keywords: "kw", SupportURL: "https://example.com"},
	}, []AppInfoLocalization{
		{Locale: "en-US", Name: "Name", PrivacyPolicyURL: "https://example.com/privacy"},
	})

	if !hasCheckID(checks, "version.state.editable") {
		t.Fatalf("expected version state check")
	}
}

func TestRequiredFieldChecks_AllowsReadyForReviewVersionState(t *testing.T) {
	checks := requiredFieldChecks("", "1.2.3", nil, "READY_FOR_REVIEW", false, []VersionLocalization{
		{Locale: "en-US", Description: "desc", Keywords: "kw", SupportURL: "https://example.com"},
	}, []AppInfoLocalization{
		{Locale: "en-US", Name: "Name", PrivacyPolicyURL: "https://example.com/privacy"},
	})

	if hasCheckID(checks, "version.state.editable") {
		t.Fatalf("did not expect READY_FOR_REVIEW to produce a version state blocker: %#v", checks)
	}
}

func TestRequiredFieldChecks_WarnsWhenPrivacyPolicyMissing(t *testing.T) {
	checks := requiredFieldChecks("", "1.2.3", nil, "PREPARE_FOR_SUBMISSION", false, []VersionLocalization{
		{Locale: "en-US", Description: "desc", Keywords: "kw", SupportURL: "https://example.com"},
	}, []AppInfoLocalization{
		{Locale: "en-US", Name: "Name", Subtitle: "Subtitle"},
	})

	if !hasCheckID(checks, "metadata.recommended.privacy_policy_url") {
		t.Fatalf("expected privacy policy check")
	}
}

func TestRequiredFieldChecks_SkipsPrivacyPolicyWarning_WhenSubscriptionsPresent(t *testing.T) {
	checks := requiredFieldChecks("", "1.2.3", nil, "PREPARE_FOR_SUBMISSION", true,
		[]VersionLocalization{
			{Locale: "en-US", Description: "desc", Keywords: "kw", SupportURL: "https://example.com"},
		}, []AppInfoLocalization{
			{Locale: "en-US", Name: "Name"},
		})

	if hasCheckID(checks, "metadata.recommended.privacy_policy_url") {
		t.Fatal("should suppress privacy policy warning when subscriptions/IAPs trigger the error")
	}
}

// TestRequiredFieldChecks_WhatsNewFollowsTheAppsVersionHistory covers a first
// App Store version that is not numbered 1.0: App Store Connect takes no What's
// New on it, so the warning is skipped when the version history says it is the
// app's first version, and kept when an earlier version exists.
func TestRequiredFieldChecks_WhatsNewFollowsTheAppsVersionHistory(t *testing.T) {
	first, notFirst := true, false
	locs := []VersionLocalization{{Locale: "en-US", Description: "desc", Keywords: "kw", SupportURL: "https://example.com"}}
	appInfo := []AppInfoLocalization{{Locale: "en-US", Name: "Name", PrivacyPolicyURL: "https://example.com/privacy"}}

	if checks := requiredFieldChecks("", "1.3", &first, "PREPARE_FOR_SUBMISSION", false, locs, appInfo); hasCheckID(checks, "metadata.required.whats_new") {
		t.Fatalf("expected no what's new warning on the app's first version 1.3")
	}
	if checks := requiredFieldChecks("", "1.0", &notFirst, "PREPARE_FOR_SUBMISSION", false, locs, appInfo); !hasCheckID(checks, "metadata.required.whats_new") {
		t.Fatalf("expected a what's new warning on 1.0 when an earlier version exists")
	}
}
