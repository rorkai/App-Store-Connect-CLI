package validation

import (
	"strings"
	"testing"
)

func TestSubscriptionReviewReadinessChecks_Empty(t *testing.T) {
	checks := subscriptionReviewReadinessChecks(nil)
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d (%v)", len(checks), checks)
	}
}

func TestSubscriptionReviewReadinessChecks_WarnsForReadyToSubmit(t *testing.T) {
	checks := subscriptionReviewReadinessChecks([]Subscription{
		{ID: "sub-1", Name: "Monthly", ProductID: "com.example.monthly", State: "READY_TO_SUBMIT"},
	})
	if !hasCheckID(checks, "subscriptions.review_readiness.needs_attention") {
		t.Fatalf("expected warning check, got %v", checks)
	}
	if checks[0].Severity != SeverityWarning {
		t.Fatalf("expected warning severity, got %s", checks[0].Severity)
	}
}

func TestSubscriptionReviewReadinessChecks_AllowsApproved(t *testing.T) {
	checks := subscriptionReviewReadinessChecks([]Subscription{
		{ID: "sub-1", State: "APPROVED"},
		{ID: "sub-2", State: "IN_REVIEW"},
		{ID: "sub-3", State: "WAITING_FOR_REVIEW"},
	})
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d (%v)", len(checks), checks)
	}
}

func TestSubscriptionReviewReadinessChecks_IgnoresRemovedFromSale(t *testing.T) {
	checks := subscriptionReviewReadinessChecks([]Subscription{
		{ID: "sub-1", State: "REMOVED_FROM_SALE"},
		{ID: "sub-2", State: "DEVELOPER_REMOVED_FROM_SALE"},
	})
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d (%v)", len(checks), checks)
	}
}

func TestSubscriptionImageChecks_WarnsWhenImageMissing(t *testing.T) {
	checks := subscriptionImageChecks([]Subscription{
		{ID: "sub-1", Name: "Monthly", ProductID: "com.example.monthly"},
	})
	if !hasCheckID(checks, "subscriptions.images.recommended") {
		t.Fatalf("expected image check, got %v", checks)
	}
	if checks[0].Severity != SeverityWarning {
		t.Fatalf("expected warning severity, got %s", checks[0].Severity)
	}
	if checks[0].Remediation == "" {
		t.Fatalf("expected remediation explaining why image matters, got %+v", checks[0])
	}
}

func TestSubscriptionFetchChecks_AddsInfoWhenSkipped(t *testing.T) {
	checks := subscriptionFetchChecks("subscription permissions unavailable")
	if !hasCheckID(checks, "subscriptions.readiness.unverified") {
		t.Fatalf("expected readiness skip check, got %v", checks)
	}
	if checks[0].Severity != SeverityInfo {
		t.Fatalf("expected info severity, got %s", checks[0].Severity)
	}
}

func TestSubscriptionImageChecks_AllowsSubscriptionsWithImages(t *testing.T) {
	checks := subscriptionImageChecks([]Subscription{
		{ID: "sub-1", HasImage: true},
	})
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d (%v)", len(checks), checks)
	}
}

func TestSubscriptionImageChecks_IgnoresRemovedFromSale(t *testing.T) {
	checks := subscriptionImageChecks([]Subscription{
		{ID: "sub-1", State: "REMOVED_FROM_SALE"},
		{ID: "sub-2", State: "DEVELOPER_REMOVED_FROM_SALE"},
	})
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d (%v)", len(checks), checks)
	}
}

func TestSubscriptionImageChecks_AddsInfoWhenImageCheckSkipped(t *testing.T) {
	checks := subscriptionImageChecks([]Subscription{
		{
			ID:                   "sub-1",
			Name:                 "Monthly",
			ProductID:            "com.example.monthly",
			ImageCheckSkipped:    true,
			ImageCheckSkipReason: "permission denied",
		},
	})
	if !hasCheckID(checks, "subscriptions.images.unverified") {
		t.Fatalf("expected unverified image check, got %v", checks)
	}
	if checks[0].Severity != SeverityInfo {
		t.Fatalf("expected info severity, got %s", checks[0].Severity)
	}
}

func TestSubscriptionPricingCoverage_WarnsPartialTerritories(t *testing.T) {
	checks := subscriptionPricingCoverageChecks([]Subscription{
		{ID: "sub-1", Name: "Monthly", ProductID: "com.example.monthly", State: "APPROVED", PriceCount: 1},
	}, 175, nil)
	if !hasCheckID(checks, "subscriptions.pricing.partial_territory_coverage") {
		t.Fatalf("expected partial coverage warning, got %v", checks)
	}
	if checks[0].Severity != SeverityWarning {
		t.Fatalf("expected warning severity, got %s", checks[0].Severity)
	}
	if !strings.Contains(checks[0].Message, "1") || !strings.Contains(checks[0].Message, "175") {
		t.Fatalf("expected message to mention price count and territory count, got %s", checks[0].Message)
	}
}

func TestSubscriptionPricingCoverage_NoWarningWhenFullCoverage(t *testing.T) {
	checks := subscriptionPricingCoverageChecks([]Subscription{
		{ID: "sub-1", State: "APPROVED", PriceCount: 175},
	}, 175, nil)
	if len(checks) != 0 {
		t.Fatalf("expected no checks when fully covered, got %d (%v)", len(checks), checks)
	}
}

func TestSubscriptionPricingCoverage_SkipsWhenNoPrices(t *testing.T) {
	// PriceCount == 0 is already covered by subscriptionMetadataDiagnostics
	checks := subscriptionPricingCoverageChecks([]Subscription{
		{ID: "sub-1", State: "MISSING_METADATA", PriceCount: 0},
	}, 175, nil)
	if len(checks) != 0 {
		t.Fatalf("expected no checks when zero prices (handled elsewhere), got %d (%v)", len(checks), checks)
	}
}

func TestSubscriptionPricingCoverage_SkipsRemovedFromSale(t *testing.T) {
	checks := subscriptionPricingCoverageChecks([]Subscription{
		{ID: "sub-1", State: "REMOVED_FROM_SALE", PriceCount: 1},
	}, 175, nil)
	if len(checks) != 0 {
		t.Fatalf("expected no checks for removed subs, got %d (%v)", len(checks), checks)
	}
}

func TestSubscriptionPricingCoverage_SkipsWhenZeroAvailableTerritories(t *testing.T) {
	checks := subscriptionPricingCoverageChecks([]Subscription{
		{ID: "sub-1", State: "APPROVED", PriceCount: 1},
	}, 0, nil)
	if len(checks) != 0 {
		t.Fatalf("expected no checks when available territories unknown, got %d (%v)", len(checks), checks)
	}
}

func TestSubscriptionPricingCoverage_SkipsWhenPriceCheckSkipped(t *testing.T) {
	checks := subscriptionPricingCoverageChecks([]Subscription{
		{ID: "sub-1", State: "APPROVED", PriceCount: 1, PriceCheckSkipped: true},
	}, 175, nil)
	if len(checks) != 0 {
		t.Fatalf("expected no checks when price check was skipped, got %d (%v)", len(checks), checks)
	}
}

func TestSubscriptionPricingCoverage_RequiresFullPricingMatrixWhenAvailabilityIsNarrower(t *testing.T) {
	checks := subscriptionPricingCoverageChecks([]Subscription{
		{
			ID:                      "sub-1",
			Name:                    "Monthly",
			ProductID:               "com.example.monthly",
			State:                   "MISSING_METADATA",
			AvailabilityTerritories: []string{"USA"},
			PriceCount:              1,
			PriceTerritories:        []string{"USA"},
		},
	}, 2, []string{"USA", "CAN"})
	if !hasCheckID(checks, "subscriptions.pricing.partial_territory_coverage") {
		t.Fatalf("expected full pricing matrix warning even when sale availability is narrower, got %v", checks)
	}
	if !strings.Contains(checks[0].Message, "CAN") {
		t.Fatalf("expected missing pricing territory in warning, got %q", checks[0].Message)
	}
}

func TestSubscriptionPricingVerificationChecks_AddsInfoWhenPriceCheckSkipped(t *testing.T) {
	checks := subscriptionPricingVerificationChecks([]Subscription{
		{
			ID:                   "sub-1",
			Name:                 "Monthly",
			ProductID:            "com.example.monthly",
			State:                "APPROVED",
			PriceCheckSkipped:    true,
			PriceCheckSkipReason: "price endpoint forbidden",
		},
	})
	if !hasCheckID(checks, "subscriptions.pricing.unverified") {
		t.Fatalf("expected pricing-unverified check, got %v", checks)
	}
	if checks[0].Severity != SeverityInfo {
		t.Fatalf("expected info severity, got %s", checks[0].Severity)
	}
	if !strings.Contains(checks[0].Remediation, "price endpoint forbidden") {
		t.Fatalf("expected remediation to preserve skip reason, got %+v", checks[0])
	}
}

func TestSubscriptionPricingVerificationChecks_SkipsMissingMetadata(t *testing.T) {
	checks := subscriptionPricingVerificationChecks([]Subscription{
		{
			ID:                   "sub-1",
			State:                "MISSING_METADATA",
			PriceCheckSkipped:    true,
			PriceCheckSkipReason: "price endpoint forbidden",
		},
	})
	if len(checks) != 0 {
		t.Fatalf("expected MISSING_METADATA pricing skip to stay in diagnostics, got %v", checks)
	}
}

func TestSubscriptionPricingCoverageSkipChecks_AddsInfoWhenCoverageSkipped(t *testing.T) {
	checks := subscriptionPricingCoverageSkipChecks("app-1", "availability endpoint timed out")
	if !hasCheckID(checks, "subscriptions.pricing_coverage.unverified") {
		t.Fatalf("expected pricing-coverage unverified check, got %v", checks)
	}
	if checks[0].Severity != SeverityInfo {
		t.Fatalf("expected info severity, got %s", checks[0].Severity)
	}
	if checks[0].ResourceID != "app-1" {
		t.Fatalf("expected app resource ID, got %+v", checks[0])
	}
}

func TestSubscriptionMetadataDiagnostics_ReportsConcreteMissingItems(t *testing.T) {
	checks := subscriptionMetadataDiagnostics([]Subscription{
		{
			ID:        "sub-1",
			Name:      "Monthly",
			ProductID: "com.example.monthly",
			State:     "MISSING_METADATA",
			GroupID:   "group-1",
			GroupName: "Premium",
		},
	})

	if !hasCheckID(checks, "subscriptions.diagnostics.group_localization_missing") {
		t.Fatalf("expected group localization missing check, got %v", checks)
	}
	if !hasCheckID(checks, "subscriptions.diagnostics.localization_missing") {
		t.Fatalf("expected localization missing check, got %v", checks)
	}
	if !hasCheckID(checks, "subscriptions.diagnostics.pricing_missing") {
		t.Fatalf("expected pricing missing check, got %v", checks)
	}

	for _, check := range checks {
		if strings.HasPrefix(check.ID, "subscriptions.diagnostics.") && check.ID != "subscriptions.diagnostics.group_localization_unverified" && check.ID != "subscriptions.diagnostics.localization_unverified" && check.ID != "subscriptions.diagnostics.pricing_unverified" && check.Severity != SeverityWarning {
			t.Fatalf("expected concrete missing-metadata diagnostics to be warnings, got %+v", check)
		}
		switch check.ID {
		case "subscriptions.diagnostics.group_localization_missing":
			for _, want := range []string{
				`asc subscriptions groups versions list --group-id "group-1"`,
				`asc subscriptions groups versions create --group-id "group-1"`,
				`asc subscriptions groups versions localizations create --version-id "VERSION_ID"`,
			} {
				if !strings.Contains(check.Remediation, want) {
					t.Fatalf("expected group version remediation containing %q, got %+v", want, check)
				}
			}
		case "subscriptions.diagnostics.localization_missing":
			for _, want := range []string{
				`asc subscriptions versions list --subscription-id "sub-1"`,
				`asc subscriptions versions create --subscription-id "sub-1"`,
				`asc subscriptions versions localizations create --version-id "VERSION_ID"`,
			} {
				if !strings.Contains(check.Remediation, want) {
					t.Fatalf("expected subscription version remediation containing %q, got %+v", want, check)
				}
			}
		}
	}
}

func TestSubscriptionDiagnosticsUseVersionScopedMetadataRemediation(t *testing.T) {
	sub := Subscription{ID: "sub-1", GroupID: "group-1"}
	tests := []struct {
		name       string
		row        SubscriptionDiagnosticRow
		want       []string
		legacyPath string
	}{
		{
			name: "group localization",
			row:  buildGroupLocalizationsDiagnosticRow(sub),
			want: []string{
				`asc subscriptions groups versions list --group-id "group-1"`,
				`asc subscriptions groups versions create --group-id "group-1"`,
				`asc subscriptions groups versions localizations create --version-id "VERSION_ID"`,
			},
			legacyPath: "asc subscriptions groups localizations create",
		},
		{
			name: "subscription localization",
			row:  buildSubscriptionLocalizationsDiagnosticRow(sub),
			want: []string{
				`asc subscriptions versions list --subscription-id "sub-1"`,
				`asc subscriptions versions create --subscription-id "sub-1"`,
				`asc subscriptions versions localizations create --version-id "VERSION_ID"`,
			},
			legacyPath: "asc subscriptions localizations create",
		},
		{
			name: "subscription image",
			row:  buildPromotionalImageDiagnosticRow(sub),
			want: []string{
				`asc subscriptions versions list --subscription-id "sub-1"`,
				`asc subscriptions versions create --subscription-id "sub-1"`,
				`asc subscriptions versions images upload --version-id "VERSION_ID"`,
			},
			legacyPath: "asc subscriptions images create",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, want := range test.want {
				if !strings.Contains(test.row.Remediation, want) {
					t.Fatalf("remediation = %q, want containing %q", test.row.Remediation, want)
				}
			}
			if strings.Contains(test.row.Remediation, test.legacyPath) {
				t.Fatalf("remediation = %q, must not teach legacy path %q", test.row.Remediation, test.legacyPath)
			}
		})
	}
}

func TestSubscriptionMetadataDiagnostics_UsesInfoChecksWhenLocalizationVerificationSkipped(t *testing.T) {
	checks := subscriptionMetadataDiagnostics([]Subscription{
		{
			ID:                            "sub-1",
			Name:                          "Monthly",
			ProductID:                     "com.example.monthly",
			State:                         "MISSING_METADATA",
			GroupID:                       "group-1",
			GroupName:                     "Premium",
			GroupLocalizationCheckSkipped: true,
			GroupLocalizationCheckReason:  "permission denied",
			LocalizationCheckSkipped:      true,
			LocalizationCheckSkipReason:   "timed out",
			PriceCheckSkipped:             true,
			PriceCheckSkipReason:          "price endpoint forbidden",
		},
	})

	if !hasCheckID(checks, "subscriptions.diagnostics.group_localization_unverified") {
		t.Fatalf("expected group localization unverified check, got %v", checks)
	}
	if !hasCheckID(checks, "subscriptions.diagnostics.localization_unverified") {
		t.Fatalf("expected localization unverified check, got %v", checks)
	}
	if !hasCheckID(checks, "subscriptions.diagnostics.pricing_unverified") {
		t.Fatalf("expected pricing unverified check, got %v", checks)
	}
	if hasCheckID(checks, "subscriptions.diagnostics.group_localization_missing") {
		t.Fatalf("did not expect false group-localization missing check, got %v", checks)
	}
	if hasCheckID(checks, "subscriptions.diagnostics.localization_missing") {
		t.Fatalf("did not expect false localization missing check, got %v", checks)
	}
	if hasCheckID(checks, "subscriptions.diagnostics.pricing_missing") {
		t.Fatalf("did not expect pricing missing when price verification skipped, got %v", checks)
	}

	for _, check := range checks {
		if strings.HasSuffix(check.ID, "_unverified") && check.Severity != SeverityInfo {
			t.Fatalf("expected unverified checks to be informational, got %+v", check)
		}
		if check.ID == "subscriptions.diagnostics.pricing_unverified" && !strings.Contains(check.Remediation, "price endpoint forbidden") {
			t.Fatalf("expected pricing-unverified remediation to preserve skip reason, got %+v", check)
		}
	}
}

func TestSubscriptionMetadataDiagnostics_UsesCanonicalAvailabilityCommand(t *testing.T) {
	tests := []struct {
		name         string
		subscription Subscription
		checkID      string
		want         string
	}{
		{
			name:         "missing availability record",
			subscription: Subscription{ID: "sub-1", State: "MISSING_METADATA"},
			checkID:      "subscriptions.diagnostics.availability_missing",
			want:         "Configure subscription availability via `asc subscriptions pricing availability edit`",
		},
		{
			name: "availability has no territories",
			subscription: Subscription{
				ID:             "sub-1",
				State:          "MISSING_METADATA",
				AvailabilityID: "availability-1",
			},
			checkID: "subscriptions.diagnostics.availability_territories_missing",
			want:    "Enable at least one subscription availability territory via `asc subscriptions pricing availability edit`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checks := subscriptionMetadataDiagnostics([]Subscription{tt.subscription})
			for _, check := range checks {
				if check.ID != tt.checkID {
					continue
				}
				if check.Remediation != tt.want {
					t.Fatalf("remediation = %q, want %q", check.Remediation, tt.want)
				}
				return
			}
			t.Fatalf("missing check %q in %+v", tt.checkID, checks)
		})
	}
}

func TestValidateSubscriptionsDiagnosticsUseCanonicalAvailabilityCommand(t *testing.T) {
	tests := []struct {
		name         string
		subscription Subscription
		want         string
	}{
		{
			name:         "missing availability record",
			subscription: Subscription{ID: "sub-1", State: "MISSING_METADATA"},
			want:         "Configure subscription availability with `asc subscriptions pricing availability edit --subscription-id \"sub-1\" --territories \"USA\"`.",
		},
		{
			name: "availability has no territories",
			subscription: Subscription{
				ID:             "sub-1",
				State:          "MISSING_METADATA",
				AvailabilityID: "availability-1",
			},
			want: "Add at least one available territory with `asc subscriptions pricing availability edit --subscription-id \"sub-1\" --territories \"USA\"`.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := ValidateSubscriptions(SubscriptionsInput{
				Subscriptions: []Subscription{tt.subscription},
			}, false)
			if len(report.Diagnostics) != 1 {
				t.Fatalf("diagnostics = %+v, want one entry", report.Diagnostics)
			}
			row, ok := findSubscriptionDiagnosticRow(report.Diagnostics[0].Rows, "subscription_availability")
			if !ok {
				t.Fatalf("missing subscription_availability row in %+v", report.Diagnostics[0].Rows)
			}
			if row.Remediation != tt.want {
				t.Fatalf("remediation = %q, want %q", row.Remediation, tt.want)
			}
		})
	}
}

func TestValidateIncludesPricingCoverageCheck(t *testing.T) {
	report := Validate(Input{
		AppID:                "app-1",
		VersionID:            "ver-1",
		AvailableTerritories: 175,
		Subscriptions: []Subscription{
			{ID: "sub-1", Name: "Monthly", ProductID: "com.example.monthly", State: "APPROVED", PriceCount: 1},
		},
	}, false)
	if !hasCheckID(report.Checks, "subscriptions.pricing.partial_territory_coverage") {
		t.Fatalf("expected pricing coverage check in unified validate, got %+v", report.Checks)
	}
}

func TestValidateSubscriptionsIncludesPricingCoverageCheck(t *testing.T) {
	report := ValidateSubscriptions(SubscriptionsInput{
		AppID:                "app-1",
		AvailableTerritories: 175,
		Subscriptions: []Subscription{
			{ID: "sub-1", Name: "Monthly", ProductID: "com.example.monthly", State: "APPROVED", PriceCount: 1},
		},
	}, false)
	if !hasCheckID(report.Checks, "subscriptions.pricing.partial_territory_coverage") {
		t.Fatalf("expected pricing coverage check in standalone validate, got %+v", report.Checks)
	}
}

func TestValidateSubscriptionsIncludesPricingVerificationCheck(t *testing.T) {
	report := ValidateSubscriptions(SubscriptionsInput{
		AppID: "app-1",
		Subscriptions: []Subscription{
			{
				ID:                   "sub-1",
				Name:                 "Monthly",
				ProductID:            "com.example.monthly",
				State:                "APPROVED",
				PriceCheckSkipped:    true,
				PriceCheckSkipReason: "price endpoint forbidden",
			},
		},
	}, false)
	if !hasCheckID(report.Checks, "subscriptions.pricing.unverified") {
		t.Fatalf("expected pricing verification check in standalone validate, got %+v", report.Checks)
	}
}

func TestValidateSubscriptionsIncludesPricingCoverageSkipCheck(t *testing.T) {
	report := ValidateSubscriptions(SubscriptionsInput{
		AppID:                     "app-1",
		PricingCoverageSkipReason: "availability endpoint timed out",
	}, false)
	if !hasCheckID(report.Checks, "subscriptions.pricing_coverage.unverified") {
		t.Fatalf("expected pricing coverage skip check in standalone validate, got %+v", report.Checks)
	}
}

func TestValidateSubscriptionsIncludesDetailedDiagnosticsForOpaqueMissingMetadata(t *testing.T) {
	report := ValidateSubscriptions(SubscriptionsInput{
		AppID:                   "app-1",
		AppBuildCount:           1,
		AppAvailableTerritories: []string{"USA", "CAN"},
		Subscriptions: []Subscription{
			{
				ID:                                 "sub-1",
				Name:                               "Monthly",
				ProductID:                          "com.example.monthly",
				State:                              "MISSING_METADATA",
				GroupID:                            "group-1",
				GroupName:                          "Premium",
				GroupLocalizations:                 []SubscriptionGroupLocalizationInfo{{Locale: "en-US", Name: "Premium"}},
				Localizations:                      []SubscriptionLocalizationInfo{{Locale: "en-US", Name: "Monthly", Description: "Unlimited access"}},
				ReviewScreenshotID:                 "shot-1",
				ReviewScreenshotAssetDeliveryState: "COMPLETE",
				AvailabilityID:                     "avail-1",
				AvailabilityTerritories:            []string{"USA", "CAN"},
				SubscriptionPeriod:                 "ONE_MONTH",
				PlanAvailabilities: []SubscriptionPlanAvailabilityInfo{{
					ID: "plan-upfront", PlanType: "UPFRONT", Territories: []string{"USA", "CAN"},
				}},
				HasImage:         true,
				PriceCount:       2,
				PriceTerritories: []string{"USA", "CAN"},
			},
		},
	}, false)

	if len(report.Diagnostics) != 1 {
		t.Fatalf("expected one subscription diagnostics entry, got %+v", report.Diagnostics)
	}

	diag := report.Diagnostics[0]
	if diag.Conclusion != "opaque_apple_state" {
		t.Fatalf("expected opaque_apple_state conclusion, got %+v", diag)
	}
	if !strings.Contains(diag.Summary, "Apple still reports MISSING_METADATA") {
		t.Fatalf("expected opaque-state summary, got %+v", diag)
	}

	for _, key := range []string{
		"group_localizations",
		"subscription_localizations",
		"review_screenshot",
		"subscription_availability",
		"upfront_plan_availability",
		"availability_surface_consistency",
		"price_records",
		"price_coverage_subscription_availability",
		"price_coverage_app_availability",
		"complete_pricing_matrix",
		"promotional_image",
		"app_has_build",
	} {
		row, ok := findSubscriptionDiagnosticRow(diag.Rows, key)
		if !ok {
			t.Fatalf("expected diagnostic row %q, got %+v", key, diag.Rows)
		}
		if row.Status != DiagnosticStatusYes {
			t.Fatalf("expected %s row to be yes, got %+v", key, row)
		}
	}
	monthlyRow, ok := findSubscriptionDiagnosticRow(diag.Rows, "monthly_plan_availability")
	if !ok || monthlyRow.Status != DiagnosticStatusOptional || monthlyRow.Blocking {
		t.Fatalf("expected absent MONTHLY plan to be optional, got %+v", monthlyRow)
	}

	buildRow, ok := findSubscriptionDiagnosticRow(diag.Rows, "app_has_build")
	if !ok {
		t.Fatalf("expected app_has_build row, got %+v", diag.Rows)
	}
	if buildRow.Status != DiagnosticStatusYes {
		t.Fatalf("expected app_has_build=yes when app build count is non-zero, got %+v", buildRow)
	}
}

func TestSubscriptionPlanAvailabilityDiagnosticsTruthTable(t *testing.T) {
	base := Subscription{
		ID: "sub-1", Name: "Annual", ProductID: "com.example.annual", State: "MISSING_METADATA",
		SubscriptionPeriod: "ONE_YEAR", AvailabilityID: "legacy", AvailabilityTerritories: []string{"CAN", "FRA"},
	}

	tests := []struct {
		name       string
		mutate     func(*Subscription)
		rowKey     string
		wantStatus DiagnosticStatus
		wantBlock  bool
		wantCheck  string
	}{
		{name: "missing upfront", rowKey: "upfront_plan_availability", wantStatus: DiagnosticStatusNo, wantBlock: true, wantCheck: "subscriptions.diagnostics.upfront_plan_availability_missing"},
		{name: "unverified fetch", mutate: func(sub *Subscription) {
			sub.PlanAvailabilityCheckSkipped = true
			sub.PlanAvailabilityCheckReason = "forbidden"
		}, rowKey: "upfront_plan_availability", wantStatus: DiagnosticStatusUnverified, wantBlock: true, wantCheck: "subscriptions.diagnostics.plan_availability_unverified"},
		{name: "matching surfaces", mutate: func(sub *Subscription) {
			sub.PlanAvailabilities = []SubscriptionPlanAvailabilityInfo{{ID: "upfront", PlanType: "UPFRONT", Territories: []string{"FRA", "CAN"}}}
		}, rowKey: "availability_surface_consistency", wantStatus: DiagnosticStatusYes, wantBlock: true},
		{name: "mismatched surfaces", mutate: func(sub *Subscription) {
			sub.PlanAvailabilities = []SubscriptionPlanAvailabilityInfo{{ID: "upfront", PlanType: "UPFRONT", Territories: []string{"CAN", "GBR"}}}
		}, rowKey: "availability_surface_consistency", wantStatus: DiagnosticStatusNo, wantBlock: true, wantCheck: "subscriptions.diagnostics.availability_surfaces_mismatch"},
		{name: "mismatched new territory policy", mutate: func(sub *Subscription) {
			legacy, plan := true, false
			sub.AvailabilityInNewTerritories = &legacy
			sub.PlanAvailabilities = []SubscriptionPlanAvailabilityInfo{{ID: "upfront", PlanType: "UPFRONT", AvailableInNewTerritories: &plan, Territories: []string{"CAN", "FRA"}}}
		}, rowKey: "availability_surface_consistency", wantStatus: DiagnosticStatusNo, wantBlock: true, wantCheck: "subscriptions.diagnostics.availability_surfaces_mismatch"},
		{name: "monthly absent is optional", mutate: func(sub *Subscription) {
			sub.PlanAvailabilities = []SubscriptionPlanAvailabilityInfo{{ID: "upfront", PlanType: "UPFRONT", Territories: []string{"CAN", "FRA"}}}
		}, rowKey: "monthly_plan_availability", wantStatus: DiagnosticStatusOptional, wantBlock: false},
		{name: "monthly valid", mutate: func(sub *Subscription) {
			sub.PlanAvailabilities = []SubscriptionPlanAvailabilityInfo{{ID: "upfront", PlanType: "UPFRONT", Territories: []string{"CAN", "FRA"}}, {ID: "monthly", PlanType: "MONTHLY", Territories: []string{"CAN"}}}
		}, rowKey: "monthly_plan_availability", wantStatus: DiagnosticStatusYes, wantBlock: true},
		{name: "monthly outside upfront", mutate: func(sub *Subscription) {
			sub.PlanAvailabilities = []SubscriptionPlanAvailabilityInfo{{ID: "upfront", PlanType: "UPFRONT", Territories: []string{"CAN", "FRA"}}, {ID: "monthly", PlanType: "MONTHLY", Territories: []string{"GBR"}}}
		}, rowKey: "monthly_plan_availability", wantStatus: DiagnosticStatusNo, wantBlock: true, wantCheck: "subscriptions.diagnostics.monthly_plan_invalid"},
		{name: "monthly forbidden territory", mutate: func(sub *Subscription) {
			sub.PlanAvailabilities = []SubscriptionPlanAvailabilityInfo{{ID: "upfront", PlanType: "UPFRONT", Territories: []string{"CAN", "USA"}}, {ID: "monthly", PlanType: "MONTHLY", Territories: []string{"USA"}}}
		}, rowKey: "monthly_plan_availability", wantStatus: DiagnosticStatusNo, wantBlock: true, wantCheck: "subscriptions.diagnostics.monthly_plan_invalid"},
		{name: "monthly wrong period", mutate: func(sub *Subscription) {
			sub.SubscriptionPeriod = "ONE_MONTH"
			sub.PlanAvailabilities = []SubscriptionPlanAvailabilityInfo{{ID: "upfront", PlanType: "UPFRONT", Territories: []string{"CAN"}}, {ID: "monthly", PlanType: "MONTHLY", Territories: []string{"CAN"}}}
		}, rowKey: "monthly_plan_availability", wantStatus: DiagnosticStatusNo, wantBlock: true, wantCheck: "subscriptions.diagnostics.monthly_plan_invalid"},
		{name: "duplicate upfront does not report surface consistency", mutate: func(sub *Subscription) {
			sub.PlanAvailabilities = []SubscriptionPlanAvailabilityInfo{
				{ID: "upfront-1", PlanType: "UPFRONT", Territories: []string{"CAN", "FRA"}},
				{ID: "upfront-2", PlanType: "UPFRONT", Territories: []string{"GBR"}},
			}
		}, rowKey: "availability_surface_consistency", wantStatus: DiagnosticStatusNo, wantBlock: true, wantCheck: "subscriptions.diagnostics.plan_availability_duplicate"},
		{name: "duplicate monthly does not report monthly validity", mutate: func(sub *Subscription) {
			sub.PlanAvailabilities = []SubscriptionPlanAvailabilityInfo{
				{ID: "upfront", PlanType: "UPFRONT", Territories: []string{"CAN", "FRA"}},
				{ID: "monthly-1", PlanType: "MONTHLY", Territories: []string{"CAN"}},
				{ID: "monthly-2", PlanType: "MONTHLY", Territories: []string{"USA"}},
			}
		}, rowKey: "monthly_plan_availability", wantStatus: DiagnosticStatusNo, wantBlock: true, wantCheck: "subscriptions.diagnostics.plan_availability_duplicate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := base
			if tt.mutate != nil {
				tt.mutate(&sub)
			}
			report := ValidateSubscriptions(SubscriptionsInput{Subscriptions: []Subscription{sub}}, false)
			row, ok := findSubscriptionDiagnosticRow(report.Diagnostics[0].Rows, tt.rowKey)
			if !ok || row.Status != tt.wantStatus || row.Blocking != tt.wantBlock {
				t.Fatalf("row %q = %+v, want status=%s blocking=%v", tt.rowKey, row, tt.wantStatus, tt.wantBlock)
			}
			if tt.wantCheck != "" && !hasCheckID(report.Checks, tt.wantCheck) {
				t.Fatalf("missing check %q in %+v", tt.wantCheck, report.Checks)
			}
		})
	}
}

func TestValidateSubscriptionsPrefersAdvisoryConclusionOverOpaqueAppleState(t *testing.T) {
	report := ValidateSubscriptions(SubscriptionsInput{
		AppID:                   "app-1",
		AppBuildCount:           1,
		AppAvailableTerritories: []string{"USA", "CAN"},
		Subscriptions: []Subscription{
			{
				ID:                                 "sub-1",
				Name:                               "Monthly",
				ProductID:                          "com.example.monthly",
				State:                              "MISSING_METADATA",
				GroupID:                            "group-1",
				GroupName:                          "Premium",
				GroupLocalizations:                 []SubscriptionGroupLocalizationInfo{{Locale: "en-US", Name: "Premium"}},
				Localizations:                      []SubscriptionLocalizationInfo{{Locale: "en-US", Name: "Monthly", Description: "Unlimited access"}},
				ReviewScreenshotID:                 "shot-1",
				ReviewScreenshotAssetDeliveryState: "COMPLETE",
				AvailabilityID:                     "avail-1",
				AvailabilityTerritories:            []string{"USA", "CAN"},
				PlanAvailabilities:                 upfrontPlan("USA", "CAN"),
				PriceCount:                         2,
				PriceTerritories:                   []string{"USA", "CAN"},
			},
		},
	}, false)

	if len(report.Diagnostics) != 1 {
		t.Fatalf("expected one subscription diagnostics entry, got %+v", report.Diagnostics)
	}

	diag := report.Diagnostics[0]
	if diag.Conclusion != "opaque_apple_state" {
		t.Fatalf("expected opaque_apple_state conclusion when only advisory rows fail but Apple remains stuck, got %+v", diag)
	}
	if !strings.Contains(diag.Summary, "do not explain why Apple still reports MISSING_METADATA") {
		t.Fatalf("expected opaque Apple state summary, got %+v", diag)
	}

	imageRow, ok := findSubscriptionDiagnosticRow(diag.Rows, "promotional_image")
	if !ok {
		t.Fatalf("expected promotional_image row, got %+v", diag.Rows)
	}
	if imageRow.Status != DiagnosticStatusNo || imageRow.Blocking {
		t.Fatalf("expected promotional image finding to stay advisory, got %+v", imageRow)
	}
	if !strings.Contains(imageRow.Remediation, "undocumented recalculation attempt") || !strings.Contains(imageRow.Remediation, "1024x1024") {
		t.Fatalf("expected stuck-state promotional image guidance, got %+v", imageRow)
	}
}

func TestValidateSubscriptionsDiagnosticsShowExactMissingTerritories(t *testing.T) {
	report := ValidateSubscriptions(SubscriptionsInput{
		AppID:                   "app-1",
		AppAvailableTerritories: []string{"USA", "CAN"},
		Subscriptions: []Subscription{
			{
				ID:                                 "sub-1",
				Name:                               "Monthly",
				ProductID:                          "com.example.monthly",
				State:                              "MISSING_METADATA",
				GroupID:                            "group-1",
				GroupName:                          "Premium",
				GroupLocalizations:                 []SubscriptionGroupLocalizationInfo{{Locale: "en-US", Name: "Premium"}},
				Localizations:                      []SubscriptionLocalizationInfo{{Locale: "en-US", Name: "Monthly", Description: "Unlimited access"}},
				ReviewScreenshotID:                 "shot-1",
				ReviewScreenshotAssetDeliveryState: "COMPLETE",
				AvailabilityID:                     "avail-1",
				AvailabilityTerritories:            []string{"USA", "CAN"},
				PlanAvailabilities:                 upfrontPlan("USA", "CAN"),
				PriceCount:                         1,
				PriceTerritories:                   []string{"USA"},
			},
		},
	}, false)

	if !hasCheckID(report.Checks, "subscriptions.diagnostics.availability_pricing_gap") {
		t.Fatalf("expected subscription availability pricing gap check, got %+v", report.Checks)
	}
	if !hasCheckID(report.Checks, "subscriptions.pricing.partial_territory_coverage") {
		t.Fatalf("expected app territory pricing gap check, got %+v", report.Checks)
	}

	diag := report.Diagnostics[0]
	subCoverageRow, ok := findSubscriptionDiagnosticRow(diag.Rows, "price_coverage_subscription_availability")
	if !ok {
		t.Fatalf("expected subscription coverage row, got %+v", diag.Rows)
	}
	if subCoverageRow.Status != DiagnosticStatusNo || !strings.Contains(subCoverageRow.Evidence, "CAN") {
		t.Fatalf("expected exact missing subscription territory evidence, got %+v", subCoverageRow)
	}

	appCoverageRow, ok := findSubscriptionDiagnosticRow(diag.Rows, "price_coverage_app_availability")
	if !ok {
		t.Fatalf("expected app coverage row, got %+v", diag.Rows)
	}
	if appCoverageRow.Status != DiagnosticStatusNo || !strings.Contains(appCoverageRow.Evidence, "CAN") {
		t.Fatalf("expected exact missing app territory evidence, got %+v", appCoverageRow)
	}
}

func TestValidateSubscriptionsMarksSkippedBuildDiagnosticAsUnverified(t *testing.T) {
	report := ValidateSubscriptions(SubscriptionsInput{
		AppID:                   "app-1",
		AppAvailableTerritories: []string{"USA", "CAN"},
		AppBuildCount:           0,
		BuildCheckSkipped:       true,
		BuildCheckSkipReason:    "build endpoint forbidden",
		Subscriptions: []Subscription{
			{
				ID:                                 "sub-1",
				Name:                               "Monthly",
				ProductID:                          "com.example.monthly",
				State:                              "MISSING_METADATA",
				GroupID:                            "group-1",
				GroupName:                          "Premium",
				GroupLocalizations:                 []SubscriptionGroupLocalizationInfo{{Locale: "en-US", Name: "Premium"}},
				Localizations:                      []SubscriptionLocalizationInfo{{Locale: "en-US", Name: "Monthly", Description: "Unlimited access"}},
				ReviewScreenshotID:                 "shot-1",
				ReviewScreenshotAssetDeliveryState: "COMPLETE",
				AvailabilityID:                     "avail-1",
				AvailabilityTerritories:            []string{"USA", "CAN"},
				PlanAvailabilities:                 upfrontPlan("USA", "CAN"),
				PriceCount:                         2,
				PriceTerritories:                   []string{"USA", "CAN"},
			},
		},
	}, false)

	if len(report.Diagnostics) != 1 {
		t.Fatalf("expected one subscription diagnostics entry, got %+v", report.Diagnostics)
	}

	buildRow, ok := findSubscriptionDiagnosticRow(report.Diagnostics[0].Rows, "app_has_build")
	if !ok {
		t.Fatalf("expected app_has_build row, got %+v", report.Diagnostics[0].Rows)
	}
	if buildRow.Status != DiagnosticStatusUnverified {
		t.Fatalf("expected app_has_build=unverified when build check is skipped, got %+v", buildRow)
	}
	if buildRow.Remediation != "build endpoint forbidden" {
		t.Fatalf("expected build skip reason to be preserved, got %+v", buildRow)
	}
}

func TestValidateSubscriptionsFallsBackToAppTerritoryCountInDiagnostics(t *testing.T) {
	report := ValidateSubscriptions(SubscriptionsInput{
		AppID:                "app-1",
		AvailableTerritories: 2,
		Subscriptions: []Subscription{
			{
				ID:                                 "sub-1",
				Name:                               "Monthly",
				ProductID:                          "com.example.monthly",
				State:                              "MISSING_METADATA",
				GroupID:                            "group-1",
				GroupName:                          "Premium",
				GroupLocalizations:                 []SubscriptionGroupLocalizationInfo{{Locale: "en-US", Name: "Premium"}},
				Localizations:                      []SubscriptionLocalizationInfo{{Locale: "en-US", Name: "Monthly", Description: "Unlimited access"}},
				ReviewScreenshotID:                 "shot-1",
				ReviewScreenshotAssetDeliveryState: "COMPLETE",
				AvailabilityID:                     "avail-1",
				AvailabilityTerritories:            []string{"USA", "CAN"},
				PlanAvailabilities:                 upfrontPlan("USA", "CAN"),
				PriceCount:                         2,
				PriceTerritories:                   []string{"USA", "CAN"},
			},
		},
	}, false)

	if len(report.Diagnostics) != 1 {
		t.Fatalf("expected one subscription diagnostics entry, got %+v", report.Diagnostics)
	}

	appCoverageRow, ok := findSubscriptionDiagnosticRow(report.Diagnostics[0].Rows, "price_coverage_app_availability")
	if !ok {
		t.Fatalf("expected app coverage diagnostic row, got %+v", report.Diagnostics[0].Rows)
	}
	if appCoverageRow.Status != DiagnosticStatusYes {
		t.Fatalf("expected app coverage diagnostics to fall back to territory count, got %+v", appCoverageRow)
	}
	if !strings.Contains(appCoverageRow.Evidence, "priced_count=2") || !strings.Contains(appCoverageRow.Evidence, "app_count=2") {
		t.Fatalf("expected count-based evidence, got %+v", appCoverageRow)
	}
}

func TestValidateSubscriptionsTreatsIncompletePricingMatrixAsBlocker(t *testing.T) {
	report := ValidateSubscriptions(SubscriptionsInput{
		AppID:                   "app-1",
		AppBuildCount:           1,
		AppAvailableTerritories: []string{"USA", "CAN"},
		PricingTerritories:      []string{"USA", "CAN"},
		PricingTerritoryCount:   2,
		Subscriptions: []Subscription{
			{
				ID:                                 "sub-1",
				Name:                               "Monthly",
				ProductID:                          "com.example.monthly",
				State:                              "MISSING_METADATA",
				GroupID:                            "group-1",
				GroupName:                          "Premium",
				GroupLocalizations:                 []SubscriptionGroupLocalizationInfo{{Locale: "en-US", Name: "Premium"}},
				Localizations:                      []SubscriptionLocalizationInfo{{Locale: "en-US", Name: "Monthly", Description: "Unlimited access"}},
				ReviewScreenshotID:                 "shot-1",
				ReviewScreenshotAssetDeliveryState: "COMPLETE",
				AvailabilityID:                     "avail-1",
				AvailabilityTerritories:            []string{"USA"},
				PlanAvailabilities:                 upfrontPlan("USA"),
				HasImage:                           true,
				PriceCount:                         1,
				PriceTerritories:                   []string{"USA"},
			},
		},
	}, false)

	if len(report.Diagnostics) != 1 {
		t.Fatalf("expected one subscription diagnostics entry, got %+v", report.Diagnostics)
	}

	diag := report.Diagnostics[0]
	if diag.Conclusion != "known_blocker" {
		t.Fatalf("expected incomplete full pricing matrix to be a known blocker, got %+v", diag)
	}
	matrixRow, ok := findSubscriptionDiagnosticRow(diag.Rows, "complete_pricing_matrix")
	if !ok || matrixRow.Status != DiagnosticStatusNo || !matrixRow.Blocking {
		t.Fatalf("expected blocking full pricing matrix diagnostic, got %+v", matrixRow)
	}
	if strings.Contains(matrixRow.Remediation, "--subscription-id") {
		t.Fatalf("expected remediation to avoid unsupported setup flag, got %+v", matrixRow)
	}
	if !strings.Contains(matrixRow.Remediation, "Re-run `asc subscriptions setup`") || !strings.Contains(matrixRow.Remediation, "--repair") {
		t.Fatalf("expected valid setup repair guidance, got %+v", matrixRow)
	}

	appCoverageRow, ok := findSubscriptionDiagnosticRow(diag.Rows, "price_coverage_app_availability")
	if !ok {
		t.Fatalf("expected app coverage diagnostic row, got %+v", diag.Rows)
	}
	if appCoverageRow.Status != DiagnosticStatusOptional || appCoverageRow.Blocking {
		t.Fatalf("expected app-only territory gap to be advisory, got %+v", appCoverageRow)
	}
	if !strings.Contains(appCoverageRow.Evidence, "app_only=CAN") {
		t.Fatalf("expected advisory evidence to name app-only territory, got %+v", appCoverageRow)
	}
}

func TestValidateSubscriptionsDoesNotBlockDiagnosticsWhenAppAvailabilityIsMissing(t *testing.T) {
	report := ValidateSubscriptions(SubscriptionsInput{
		AppID:         "app-1",
		AppBuildCount: 1,
		Subscriptions: []Subscription{
			{
				ID:                                 "sub-1",
				Name:                               "Monthly",
				ProductID:                          "com.example.monthly",
				State:                              "MISSING_METADATA",
				GroupID:                            "group-1",
				GroupName:                          "Premium",
				GroupLocalizations:                 []SubscriptionGroupLocalizationInfo{{Locale: "en-US", Name: "Premium"}},
				Localizations:                      []SubscriptionLocalizationInfo{{Locale: "en-US", Name: "Monthly", Description: "Unlimited access"}},
				ReviewScreenshotID:                 "shot-1",
				ReviewScreenshotAssetDeliveryState: "COMPLETE",
				AvailabilityID:                     "avail-1",
				AvailabilityTerritories:            []string{"USA"},
				PlanAvailabilities:                 upfrontPlan("USA"),
				HasImage:                           true,
				PriceCount:                         1,
				PriceTerritories:                   []string{"USA"},
			},
		},
	}, false)

	if len(report.Diagnostics) != 1 {
		t.Fatalf("expected one subscription diagnostics entry, got %+v", report.Diagnostics)
	}

	diag := report.Diagnostics[0]
	if diag.Conclusion != "opaque_apple_state" {
		t.Fatalf("expected opaque_apple_state when app availability is unavailable, got %+v", diag)
	}

	appCoverageRow, ok := findSubscriptionDiagnosticRow(diag.Rows, "price_coverage_app_availability")
	if !ok {
		t.Fatalf("expected app coverage diagnostic row, got %+v", diag.Rows)
	}
	if appCoverageRow.Status != DiagnosticStatusUnknown || appCoverageRow.Blocking {
		t.Fatalf("expected missing app availability to be non-blocking unknown, got %+v", appCoverageRow)
	}
}

func TestValidateSubscriptionsKeepsAppAvailabilityDiagnosticVerifiedWhenPricingTerritoriesAreUnavailable(t *testing.T) {
	report := ValidateSubscriptions(SubscriptionsInput{
		AppID:                     "app-1",
		AppBuildCount:             1,
		AppAvailableTerritories:   []string{"USA"},
		PricingCoverageSkipReason: "App Store pricing territories could not be fetched",
		Subscriptions: []Subscription{
			{
				ID:                                 "sub-1",
				Name:                               "Monthly",
				ProductID:                          "com.example.monthly",
				State:                              "MISSING_METADATA",
				GroupID:                            "group-1",
				GroupName:                          "Premium",
				GroupLocalizations:                 []SubscriptionGroupLocalizationInfo{{Locale: "en-US", Name: "Premium"}},
				Localizations:                      []SubscriptionLocalizationInfo{{Locale: "en-US", Name: "Monthly", Description: "Unlimited access"}},
				ReviewScreenshotID:                 "shot-1",
				ReviewScreenshotAssetDeliveryState: "COMPLETE",
				AvailabilityID:                     "avail-1",
				AvailabilityTerritories:            []string{"USA"},
				HasImage:                           true,
				PriceCount:                         1,
				PriceTerritories:                   []string{"USA"},
			},
		},
	}, false)

	if len(report.Diagnostics) != 1 {
		t.Fatalf("expected one subscription diagnostics entry, got %+v", report.Diagnostics)
	}
	appCoverageRow, ok := findSubscriptionDiagnosticRow(report.Diagnostics[0].Rows, "price_coverage_app_availability")
	if !ok {
		t.Fatalf("expected app coverage diagnostic row, got %+v", report.Diagnostics[0].Rows)
	}
	if appCoverageRow.Status != DiagnosticStatusYes {
		t.Fatalf("expected fetched app availability to remain verified, got %+v", appCoverageRow)
	}
	matrixRow, ok := findSubscriptionDiagnosticRow(report.Diagnostics[0].Rows, "complete_pricing_matrix")
	if !ok || matrixRow.Status != DiagnosticStatusUnverified {
		t.Fatalf("expected only the pricing matrix to be unverified, got %+v", matrixRow)
	}
}

func TestSubscriptionPricingCoverageSkipCheckNamesPricingTerritories(t *testing.T) {
	checks := subscriptionPricingCoverageSkipChecks("app-1", "pricing endpoint unavailable")
	if len(checks) != 1 {
		t.Fatalf("expected one pricing coverage skip check, got %+v", checks)
	}
	if strings.Contains(strings.ToLower(checks[0].Message), "app availability") {
		t.Fatalf("expected pricing-territory-specific message, got %+v", checks[0])
	}
	if !strings.Contains(strings.ToLower(checks[0].Message), "app store pricing territories") {
		t.Fatalf("expected pricing-territory-specific message, got %+v", checks[0])
	}
}

func TestValidateSubscriptionsSkipsAppCoverageUntilSubscriptionAvailabilityExists(t *testing.T) {
	report := ValidateSubscriptions(SubscriptionsInput{
		AppID:                   "app-1",
		AppBuildCount:           1,
		AppAvailableTerritories: []string{"USA", "CAN"},
		Subscriptions: []Subscription{
			{
				ID:                                 "sub-1",
				Name:                               "Monthly",
				ProductID:                          "com.example.monthly",
				State:                              "MISSING_METADATA",
				GroupID:                            "group-1",
				GroupName:                          "Premium",
				GroupLocalizations:                 []SubscriptionGroupLocalizationInfo{{Locale: "en-US", Name: "Premium"}},
				Localizations:                      []SubscriptionLocalizationInfo{{Locale: "en-US", Name: "Monthly", Description: "Unlimited access"}},
				ReviewScreenshotID:                 "shot-1",
				ReviewScreenshotAssetDeliveryState: "COMPLETE",
				HasImage:                           true,
				PriceCount:                         1,
				PriceTerritories:                   []string{"USA"},
			},
		},
	}, false)

	if !hasCheckID(report.Checks, "subscriptions.pricing.partial_territory_coverage") {
		t.Fatalf("expected pricing matrix warning independent of subscription availability, got %+v", report.Checks)
	}
	if len(report.Diagnostics) != 1 {
		t.Fatalf("expected one subscription diagnostics entry, got %+v", report.Diagnostics)
	}

	diag := report.Diagnostics[0]
	if diag.Conclusion != "known_blocker" {
		t.Fatalf("expected missing subscription availability to remain the blocker, got %+v", diag)
	}

	appCoverageRow, ok := findSubscriptionDiagnosticRow(diag.Rows, "price_coverage_app_availability")
	if !ok {
		t.Fatalf("expected app coverage diagnostic row, got %+v", diag.Rows)
	}
	if appCoverageRow.Status != DiagnosticStatusUnknown || appCoverageRow.Blocking {
		t.Fatalf("expected app coverage row to stay non-blocking until subscription availability exists, got %+v", appCoverageRow)
	}
	if !strings.Contains(appCoverageRow.Evidence, "subscription availability missing") {
		t.Fatalf("expected app coverage evidence to explain the missing prerequisite, got %+v", appCoverageRow)
	}
}

func findSubscriptionDiagnosticRow(rows []SubscriptionDiagnosticRow, key string) (SubscriptionDiagnosticRow, bool) {
	for _, row := range rows {
		if row.Key == key {
			return row, true
		}
	}
	return SubscriptionDiagnosticRow{}, false
}

func upfrontPlan(territories ...string) []SubscriptionPlanAvailabilityInfo {
	return []SubscriptionPlanAvailabilityInfo{{ID: "plan-upfront", PlanType: "UPFRONT", Territories: territories}}
}

func TestReviewScreenshotDiagnosticsRespectAssetDeliveryState(t *testing.T) {
	tests := []struct {
		name           string
		state          string
		wantStatus     DiagnosticStatus
		wantConclusion string
		wantCheckID    string
		wantText       string
	}{
		{name: "complete", state: "COMPLETE", wantStatus: DiagnosticStatusYes, wantConclusion: "opaque_apple_state"},
		{name: "failed", state: "FAILED", wantStatus: DiagnosticStatusNo, wantConclusion: "known_blocker", wantCheckID: "subscriptions.diagnostics.review_screenshot_failed", wantText: "delete"},
		{name: "processing", state: "PROCESSING", wantStatus: DiagnosticStatusUnverified, wantConclusion: "unknown", wantCheckID: "subscriptions.diagnostics.review_screenshot_unverified", wantText: "COMPLETE"},
		{name: "unknown", state: "", wantStatus: DiagnosticStatusUnverified, wantConclusion: "unknown", wantCheckID: "subscriptions.diagnostics.review_screenshot_unverified", wantText: "delivery state"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := ValidateSubscriptions(SubscriptionsInput{
				AppID:         "app-1",
				AppBuildCount: 1,
				Subscriptions: []Subscription{{
					ID:                                 "sub-1",
					State:                              "MISSING_METADATA",
					GroupLocalizations:                 []SubscriptionGroupLocalizationInfo{{Locale: "en-US", Name: "Premium"}},
					Localizations:                      []SubscriptionLocalizationInfo{{Locale: "en-US", Name: "Monthly", Description: "Access"}},
					ReviewScreenshotID:                 "shot-1",
					ReviewScreenshotAssetDeliveryState: test.state,
					AvailabilityID:                     "avail-1",
					AvailabilityTerritories:            []string{"USA"},
					PlanAvailabilities:                 upfrontPlan("USA"),
					PriceCount:                         1,
					PriceTerritories:                   []string{"USA"},
					HasImage:                           true,
				}},
			}, false)

			if len(report.Diagnostics) != 1 {
				t.Fatalf("expected one diagnostics result, got %+v", report.Diagnostics)
			}
			diagnostic := report.Diagnostics[0]
			row, ok := findSubscriptionDiagnosticRow(diagnostic.Rows, "review_screenshot")
			if !ok {
				t.Fatalf("review screenshot row missing: %+v", diagnostic.Rows)
			}
			if row.Status != test.wantStatus || diagnostic.Conclusion != test.wantConclusion {
				t.Fatalf("row=%+v conclusion=%q, want status=%q conclusion=%q", row, diagnostic.Conclusion, test.wantStatus, test.wantConclusion)
			}
			if !strings.Contains(row.Evidence, "asset_delivery_state=") {
				t.Fatalf("expected delivery-state evidence, got %+v", row)
			}
			if test.wantText != "" && !strings.Contains(strings.ToLower(row.Remediation), strings.ToLower(test.wantText)) {
				t.Fatalf("remediation %q does not contain %q", row.Remediation, test.wantText)
			}
			if test.state == "FAILED" && !strings.Contains(strings.ToLower(row.Remediation), "re-upload") {
				t.Fatalf("failed screenshot remediation must instruct re-upload, got %q", row.Remediation)
			}
			if test.wantCheckID != "" && !hasCheckID(report.Checks, test.wantCheckID) {
				t.Fatalf("expected check %q, got %+v", test.wantCheckID, report.Checks)
			}
		})
	}
}
