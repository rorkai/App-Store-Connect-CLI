package validation

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestApplyDeepValidationReplacesPrivacyAdvisoryAndRebuildsDerivedFields(t *testing.T) {
	report := Report{
		AppID: "app-1",
		Checks: []CheckResult{
			PrivacyPublishStateAdvisory("app-1"),
			{
				ID:          "review_details.missing",
				Severity:    SeverityError,
				Message:     "review information is missing",
				Remediation: "Complete review information",
			},
		},
	}
	deep := DeepReport{
		SessionStatus: DeepSessionCached,
		Checks: []DeepCheck{{
			ID:      DeepCheckPrivacyPublishState,
			Status:  DeepStatusBlocked,
			Source:  DeepSourceWebSession,
			Message: "App Privacy answers are not published",
			Resolution: &Resolution{
				Fixability:         FixabilityWebFixable,
				Commands:           []string{`asc web privacy publish --app "app-1" --confirm`},
				AppStoreConnectURL: "https://appstoreconnect.apple.com/apps/app-1/appPrivacy",
			},
		}},
	}
	finding := CheckResult{
		ID:          "privacy.publish_state.unpublished",
		Severity:    SeverityError,
		Message:     "App Privacy answers are not published",
		Remediation: "Publish App Privacy answers",
		Resolution:  deep.Checks[0].Resolution,
	}

	got := ApplyDeepValidation(report, deep, []CheckResult{finding})
	if got.Deep == nil || got.Deep.Summary.Blocked != 1 {
		t.Fatalf("deep report = %#v, want one blocked check", got.Deep)
	}
	if got.Summary.Errors != 2 || got.Summary.Blocking != 2 {
		t.Fatalf("summary = %#v, want two blocking errors", got.Summary)
	}
	for _, check := range got.Checks {
		if check.ID == privacyPublishStateUnverifiedID {
			t.Fatalf("public-only privacy advisory survived deep evidence: %#v", got.Checks)
		}
		if check.Remediation != "" && check.Resolution == nil {
			t.Fatalf("actionable check %q has no deep resolution", check.ID)
		}
	}
	if got.Checks[0].Resolution.Fixability != FixabilityManual {
		t.Fatalf("existing human-input finding classification = %q, want manual", got.Checks[0].Resolution.Fixability)
	}
	if got.Remediation.TotalActionable != 2 || got.Remediation.Steps[0].Resolution == nil {
		t.Fatalf("remediation was not rebuilt with resolution: %#v", got.Remediation)
	}
}

func TestApplyDeepValidationClassifiesKnownPublicAPIRemediation(t *testing.T) {
	report := Report{
		AppID:     "app-1",
		VersionID: "version-1",
		Checks: []CheckResult{{
			ID:           "legal.required.copyright",
			Severity:     SeverityError,
			Field:        "copyright",
			ResourceType: "appStoreVersion",
			Message:      "copyright is required",
			Remediation:  "Set copyright",
		}},
	}

	got := ApplyDeepValidation(report, DeepReport{}, nil)
	resolution := got.Checks[0].Resolution
	if resolution == nil || resolution.Fixability != FixabilityAPIFixable {
		t.Fatalf("copyright resolution = %#v, want api-fixable", resolution)
	}
	if len(resolution.Commands) != 1 || !strings.Contains(resolution.Commands[0], `--version-id "version-1"`) || !strings.Contains(resolution.Commands[0], "--copyright") {
		t.Fatalf("copyright commands = %#v, want exact version update command", resolution.Commands)
	}
}

func TestDeepValidationJSONIsAdditiveAndCamelCase(t *testing.T) {
	report := Report{
		AppID: "app-1",
		Deep: &DeepReport{
			SessionStatus: DeepSessionUnavailable,
			Checks: []DeepCheck{{
				ID:      DeepCheckAgreementsActive,
				Status:  DeepStatusUnverified,
				Source:  DeepSourceWebSession,
				Message: "Could not verify agreements",
				Resolution: &Resolution{
					Fixability:         FixabilityManual,
					AppStoreConnectURL: "https://appstoreconnect.apple.com/agreements/#/",
				},
			}},
		},
	}

	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	jsonText := string(encoded)
	for _, want := range []string{`"deep"`, `"sessionStatus":"unavailable"`, `"appStoreConnectUrl"`, `"notApplicable"`} {
		if !strings.Contains(jsonText, want) {
			t.Fatalf("JSON %s missing %s", jsonText, want)
		}
	}
}

func TestSummarizeDeepChecksCountsEveryTerminalStatus(t *testing.T) {
	checks := []DeepCheck{
		{Status: DeepStatusPassed},
		{Status: DeepStatusBlocked},
		{Status: DeepStatusUnverified},
		{Status: DeepStatusNotApplicable},
	}
	got := SummarizeDeepChecks(checks)
	if got != (DeepSummary{Passed: 1, Blocked: 1, Unverified: 1, NotApplicable: 1}) {
		t.Fatalf("SummarizeDeepChecks() = %#v", got)
	}
}

// TestUserFixCommandsUsePlaceholdersOnly fails if a command that needs the
// developer's answer prints a runnable example value (such as an example.com
// URL) instead of an UPPERCASE placeholder.
func TestUserFixCommandsUsePlaceholdersOnly(t *testing.T) {
	report := Report{AppID: "app-1", VersionID: "ver-1"}
	tests := []struct {
		resourceType string
		field        string
		want         string
	}{
		{"appStoreVersionLocalization", "whatsNew", `asc localizations update --id "loc-1" --whats-new "WHATS_NEW"`},
		{"appStoreVersionLocalization", "marketingUrl", `asc localizations update --id "loc-1" --marketing-url "MARKETING_URL"`},
		{"appInfoLocalization", "name", `asc localizations update --type app-info --id "loc-1" --name "APP_NAME"`},
		{"appInfoLocalization", "privacyPolicyUrl", `asc localizations update --type app-info --id "loc-1" --privacy-policy-url "PRIVACY_POLICY_URL"`},
		{"appInfoLocalization", "privacyChoicesUrl", `asc localizations update --type app-info --id "loc-1" --privacy-choices-url "PRIVACY_CHOICES_URL"`},
	}
	for _, test := range tests {
		t.Run(test.resourceType+"/"+test.field, func(t *testing.T) {
			fix := publicAPIResolutionCommand(report, CheckResult{ID: "metadata.check", Field: test.field, ResourceType: test.resourceType, ResourceID: "loc-1"})
			if !fix.needsUserInput || len(fix.commands) != 1 || fix.commands[0] != test.want {
				t.Fatalf("fix = %+v, want %s with needsUserInput", fix, test.want)
			}
		})
	}
}
