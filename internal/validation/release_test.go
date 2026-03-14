package validation

import (
	"strings"
	"testing"
	"time"
)

func TestReleaseChecks_ScheduledDateInPast(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	checks := releaseChecks("SCHEDULED", past)
	if !hasCheckID(checks, "release.scheduled_date_past") {
		t.Fatalf("expected scheduled_date_past check, got %v", checks)
	}
	if checks[0].Severity != SeverityWarning {
		t.Fatalf("expected warning severity, got %s", checks[0].Severity)
	}
	if !strings.Contains(checks[0].Message, past) {
		t.Fatalf("expected message to include the date, got %s", checks[0].Message)
	}
}

func TestReleaseChecks_ScheduledDateInFuture(t *testing.T) {
	future := time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339)
	checks := releaseChecks("SCHEDULED", future)
	if hasCheckID(checks, "release.scheduled_date_past") {
		t.Fatalf("expected no past-date warning for future date, got %v", checks)
	}
}

func TestReleaseChecks_ManualReleaseInfo(t *testing.T) {
	checks := releaseChecks("MANUAL", "")
	if !hasCheckID(checks, "release.type_manual") {
		t.Fatalf("expected manual release info check, got %v", checks)
	}
	if checks[0].Severity != SeverityInfo {
		t.Fatalf("expected info severity, got %s", checks[0].Severity)
	}
}

func TestReleaseChecks_AfterApprovalNoChecks(t *testing.T) {
	checks := releaseChecks("AFTER_APPROVAL", "")
	if len(checks) != 0 {
		t.Fatalf("expected no checks for AFTER_APPROVAL, got %d (%v)", len(checks), checks)
	}
}

func TestReleaseChecks_EmptyReleaseTypeNoChecks(t *testing.T) {
	checks := releaseChecks("", "")
	if len(checks) != 0 {
		t.Fatalf("expected no checks for empty release type, got %d (%v)", len(checks), checks)
	}
}

func TestReleaseChecks_ScheduledCaseInsensitive(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	checks := releaseChecks("scheduled", past)
	if !hasCheckID(checks, "release.scheduled_date_past") {
		t.Fatalf("expected case-insensitive match for SCHEDULED, got %v", checks)
	}
}

func TestValidateIncludesReleaseChecks(t *testing.T) {
	report := Validate(Input{
		AppID:       "app-1",
		VersionID:   "ver-1",
		ReleaseType: "MANUAL",
	}, false)
	if !hasCheckID(report.Checks, "release.type_manual") {
		t.Fatalf("expected release check in unified validate, got %+v", report.Checks)
	}
}
