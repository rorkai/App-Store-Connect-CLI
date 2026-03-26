package validation

import (
	"strings"
	"testing"
)

func TestPrivacyPublishStateChecks_PublicOnlyInfoAdvisory(t *testing.T) {
	checks := privacyPublishStateChecks("app-1")
	if len(checks) != 1 {
		t.Fatalf("expected one advisory check, got %d (%v)", len(checks), checks)
	}

	check := checks[0]
	if check.ID != "privacy.publish_state.unverified" {
		t.Fatalf("expected privacy.publish_state.unverified, got %q", check.ID)
	}
	if check.Severity != SeverityInfo {
		t.Fatalf("expected info severity, got %s", check.Severity)
	}
	if !strings.Contains(check.Message, "public App Store Connect API") {
		t.Fatalf("expected public API wording in message, got %q", check.Message)
	}
	if !strings.Contains(check.Remediation, "https://appstoreconnect.apple.com/apps/app-1/appPrivacy") {
		t.Fatalf("expected App Privacy page remediation, got %q", check.Remediation)
	}
	for _, disallowed := range []string{"asc web", "web privacy", "private/web"} {
		if strings.Contains(strings.ToLower(check.Remediation), disallowed) {
			t.Fatalf("did not expect %q in remediation, got %q", disallowed, check.Remediation)
		}
	}
}

func TestValidateIncludesPrivacyPublishStateAdvisoryWithoutBlocking(t *testing.T) {
	report := Validate(Input{
		AppID:     "app-1",
		VersionID: "ver-1",
	}, false)

	if !hasCheckID(report.Checks, "privacy.publish_state.unverified") {
		t.Fatalf("expected privacy publish state advisory in validate report, got %+v", report.Checks)
	}
	if report.Summary.Infos == 0 {
		t.Fatalf("expected info count to include privacy advisory, got %+v", report.Summary)
	}
	if report.Summary.Blocking != report.Summary.Errors {
		t.Fatalf("expected info advisory to remain non-blocking, got %+v", report.Summary)
	}
}
