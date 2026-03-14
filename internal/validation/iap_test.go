package validation

import "testing"

func TestIAPReviewReadinessChecks_Empty(t *testing.T) {
	checks := iapReviewReadinessChecks(nil)
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d (%v)", len(checks), checks)
	}
}

func TestIAPReviewReadinessChecks_WarnsForReadyToSubmit(t *testing.T) {
	checks := iapReviewReadinessChecks([]IAP{
		{ID: "iap-1", Name: "Pro", ProductID: "com.example.pro", State: "READY_TO_SUBMIT"},
	})
	if !hasCheckID(checks, "iap.review_readiness.needs_attention") {
		t.Fatalf("expected warning check, got %v", checks)
	}
	if checks[0].Severity != SeverityWarning {
		t.Fatalf("expected warning severity, got %s", checks[0].Severity)
	}
}

func TestIAPReviewReadinessChecks_AllowsInReview(t *testing.T) {
	checks := iapReviewReadinessChecks([]IAP{
		{ID: "iap-1", State: "IN_REVIEW"},
		{ID: "iap-2", State: "WAITING_FOR_REVIEW"},
		{ID: "iap-3", State: "APPROVED"},
	})
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d (%v)", len(checks), checks)
	}
}

func TestIAPReviewReadinessChecks_IgnoresRemovedFromSale(t *testing.T) {
	checks := iapReviewReadinessChecks([]IAP{
		{ID: "iap-1", State: "REMOVED_FROM_SALE"},
		{ID: "iap-2", State: "DEVELOPER_REMOVED_FROM_SALE"},
	})
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d (%v)", len(checks), checks)
	}
}

func TestIAPFetchChecks_EmptyReasonReturnsNil(t *testing.T) {
	checks := iapFetchChecks("")
	if len(checks) != 0 {
		t.Fatalf("expected no checks for empty reason, got %d", len(checks))
	}
}

func TestIAPFetchChecks_ReturnsInfoCheck(t *testing.T) {
	checks := iapFetchChecks("account cannot read IAPs")
	if len(checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(checks))
	}
	if checks[0].ID != "iap.readiness.unverified" {
		t.Fatalf("expected iap.readiness.unverified, got %s", checks[0].ID)
	}
	if checks[0].Severity != SeverityInfo {
		t.Fatalf("expected info severity, got %s", checks[0].Severity)
	}
}

func TestValidateIncludesIAPChecks(t *testing.T) {
	input := Input{
		AppID:     "app-1",
		VersionID: "ver-1",
		IAPs: []IAP{
			{ID: "iap-1", Name: "Coins", ProductID: "com.example.coins", State: "MISSING_METADATA"},
		},
	}
	report := Validate(input, false)
	if !hasCheckID(report.Checks, "iap.review_readiness.needs_attention") {
		t.Fatalf("expected IAP check in unified validate, got %+v", report.Checks)
	}
}

func TestValidateIncludesIAPFetchSkipReason(t *testing.T) {
	input := Input{
		AppID:              "app-1",
		VersionID:          "ver-1",
		IAPFetchSkipReason: "forbidden",
	}
	report := Validate(input, false)
	if !hasCheckID(report.Checks, "iap.readiness.unverified") {
		t.Fatalf("expected iap.readiness.unverified in unified validate, got %+v", report.Checks)
	}
}
