package subscriptions

import (
	"strings"
	"testing"
)

func TestNormalizeSubscriptionBillingMode(t *testing.T) {
	tests := []struct {
		input string
		want  subscriptionBillingMode
	}{
		{input: "", want: subscriptionBillingModeUpfront},
		{input: "upfront", want: subscriptionBillingModeUpfront},
		{input: "standard", want: subscriptionBillingModeUpfront},
		{input: "monthly-commitment", want: subscriptionBillingModeMonthlyCommitment},
		{input: "monthly_with_12_month_commitment", want: subscriptionBillingModeMonthlyCommitment},
		{input: "installment-billed-yearly", want: subscriptionBillingModeMonthlyCommitment},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := normalizeSubscriptionBillingMode(test.input)
			if err != nil {
				t.Fatalf("normalizeSubscriptionBillingMode() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("normalizeSubscriptionBillingMode() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNormalizeSubscriptionBillingModeRejectsUnknown(t *testing.T) {
	_, err := normalizeSubscriptionBillingMode("monthly")
	if err == nil || !strings.Contains(err.Error(), "--billing-mode must be one of") {
		t.Fatalf("expected billing mode error, got %v", err)
	}
}

func TestNormalizeSubscriptionPlanType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "MONTHLY", want: "MONTHLY"},
		{input: "monthly", want: "MONTHLY"},
		{input: "UPFRONT", want: "UPFRONT"},
		{input: "upfront", want: "UPFRONT"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := normalizeSubscriptionPlanType(test.input)
			if err != nil {
				t.Fatalf("normalizeSubscriptionPlanType() error = %v", err)
			}
			if string(got) != test.want {
				t.Fatalf("normalizeSubscriptionPlanType() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNormalizeSubscriptionPlanTypeRejectsUnknown(t *testing.T) {
	_, err := normalizeSubscriptionPlanType("annual")
	if err == nil || !strings.Contains(err.Error(), "--plan-type must be one of") {
		t.Fatalf("expected plan type error, got %v", err)
	}
}

func TestFilterMonthlyCommitmentTerritories(t *testing.T) {
	eligible, excluded := filterMonthlyCommitmentTerritories([]string{"USA", "NOR", "SGP", "DEU", "USA"})
	if got := strings.Join(eligible, ","); got != "NOR,DEU" {
		t.Fatalf("eligible = %q, want NOR,DEU", got)
	}
	if got := strings.Join(excluded, ","); got != "USA,SGP" {
		t.Fatalf("excluded = %q, want USA,SGP", got)
	}
}

func TestValidateMonthlyCommitmentPriceRange(t *testing.T) {
	tests := []struct {
		name        string
		monthly     string
		upfront     string
		wantErrPart string
	}{
		{name: "equal upfront total", monthly: "10.00", upfront: "120.00"},
		{name: "inside range", monthly: "12.50", upfront: "120.00"},
		{name: "equal max total", monthly: "15.00", upfront: "120.00"},
		{name: "below upfront", monthly: "9.99", upfront: "120.00", wantErrPart: "outside the allowed range"},
		{name: "above max", monthly: "15.01", upfront: "120.00", wantErrPart: "outside the allowed range"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateMonthlyCommitmentPriceRange(test.monthly, test.upfront)
			if test.wantErrPart == "" {
				if err != nil {
					t.Fatalf("validateMonthlyCommitmentPriceRange() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErrPart) {
				t.Fatalf("expected error containing %q, got %v", test.wantErrPart, err)
			}
		})
	}
}
