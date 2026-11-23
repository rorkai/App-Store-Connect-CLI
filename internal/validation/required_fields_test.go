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

func TestRequiredFieldChecks_WhatsNewFollowsPriorReleaseOverVersionString(t *testing.T) {
	released := true
	neverReleased := false
	tests := []struct {
		name            string
		versionString   string
		hasPriorRelease *bool
		wantWarning     bool
	}{
		// First App Store version numbered other than 1.0 (issue #4043).
		{name: "first release numbered 1.3", versionString: "1.3", hasPriorRelease: &neverReleased, wantWarning: false},
		{name: "update after a released version", versionString: "1.3", hasPriorRelease: &released, wantWarning: true},
		// A failed lookup keeps the version-string fallback.
		{name: "unknown history falls back for 1.0", versionString: "1.0", hasPriorRelease: nil, wantWarning: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checks := requiredFieldChecks("", tt.versionString, tt.hasPriorRelease, "PREPARE_FOR_SUBMISSION", false, []VersionLocalization{
				{Locale: "en-US", Description: "desc", Keywords: "kw", SupportURL: "https://example.com"},
			}, []AppInfoLocalization{
				{Locale: "en-US", Name: "Name", PrivacyPolicyURL: "https://example.com/privacy"},
			})
			if got := hasCheckID(checks, "metadata.required.whats_new"); got != tt.wantWarning {
				t.Fatalf("whatsNew warning = %v, want %v", got, tt.wantWarning)
			}
		})
	}
}
