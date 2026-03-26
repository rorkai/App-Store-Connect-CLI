package validation

import "testing"

func TestPricingChecks_MissingSchedule(t *testing.T) {
	checks := pricingChecks("app-1", "", "")
	if !hasCheckID(checks, "pricing.schedule.missing") {
		t.Fatalf("expected pricing.schedule.missing check, got %v", checks)
	}
}

func TestPricingChecks_Pass(t *testing.T) {
	checks := pricingChecks("app-1", "sched-1", "")
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d (%v)", len(checks), checks)
	}
}

func TestPricingChecks_UnverifiedWhenSkipped(t *testing.T) {
	checks := pricingChecks("app-1", "", "pricing access unavailable")
	if !hasCheckID(checks, "pricing.schedule.unverified") {
		t.Fatalf("expected pricing.schedule.unverified check, got %v", checks)
	}
	if hasCheckID(checks, "pricing.schedule.missing") {
		t.Fatalf("did not expect pricing.schedule.missing when pricing is unverified, got %v", checks)
	}
	if checks[0].Severity != SeverityWarning {
		t.Fatalf("expected warning severity, got %s", checks[0].Severity)
	}
}

func TestAvailabilityChecks_MissingAvailability(t *testing.T) {
	checks := availabilityChecks("app-1", "", 0, "")
	if !hasCheckID(checks, "availability.missing") {
		t.Fatalf("expected availability.missing check, got %v", checks)
	}
}

func TestAvailabilityChecks_NoTerritories(t *testing.T) {
	checks := availabilityChecks("app-1", "avail-1", 0, "")
	if !hasCheckID(checks, "availability.territories.none") {
		t.Fatalf("expected availability.territories.none check, got %v", checks)
	}
}

func TestAvailabilityChecks_Pass(t *testing.T) {
	checks := availabilityChecks("app-1", "avail-1", 3, "")
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d (%v)", len(checks), checks)
	}
}

func TestAvailabilityChecks_UnverifiedWhenSkipped(t *testing.T) {
	checks := availabilityChecks("app-1", "", 0, "availability access unavailable")
	if !hasCheckID(checks, "availability.unverified") {
		t.Fatalf("expected availability.unverified check, got %v", checks)
	}
	if hasCheckID(checks, "availability.missing") {
		t.Fatalf("did not expect availability.missing when availability is unverified, got %v", checks)
	}
	if checks[0].Severity != SeverityWarning {
		t.Fatalf("expected warning severity, got %s", checks[0].Severity)
	}
}

func TestValidateIncludesPricingAndAvailabilitySkipWarnings(t *testing.T) {
	report := Validate(Input{
		AppID:                       "app-1",
		VersionID:                   "ver-1",
		PricingFetchSkipReason:      "pricing access unavailable",
		AvailabilityFetchSkipReason: "availability access unavailable",
	}, false)
	if !hasCheckID(report.Checks, "pricing.schedule.unverified") {
		t.Fatalf("expected pricing.schedule.unverified in unified validate, got %+v", report.Checks)
	}
	if !hasCheckID(report.Checks, "availability.unverified") {
		t.Fatalf("expected availability.unverified in unified validate, got %+v", report.Checks)
	}
}

func TestValidateAvailabilitySkipSuppressesPartialCoverageAndAddsCoverageSkipCheck(t *testing.T) {
	report := Validate(Input{
		AppID:                       "app-1",
		VersionID:                   "ver-1",
		AvailabilityFetchSkipReason: "availability access unavailable",
		PricingCoverageSkipReason:   "subscription pricing coverage verification was skipped because availability access was unavailable",
		AvailableTerritories:        175,
		AppAvailableTerritories:     []string{"USA", "CAN"},
		Subscriptions: []Subscription{
			{
				ID:               "sub-1",
				State:            "APPROVED",
				PriceCount:       1,
				PriceTerritories: []string{"USA"},
			},
		},
	}, false)

	if hasCheckID(report.Checks, "subscriptions.pricing.partial_territory_coverage") {
		t.Fatalf("did not expect partial coverage check when availability is unverified, got %+v", report.Checks)
	}
	if !hasCheckID(report.Checks, "subscriptions.pricing_coverage.unverified") {
		t.Fatalf("expected pricing coverage skip check in unified validate, got %+v", report.Checks)
	}
}
