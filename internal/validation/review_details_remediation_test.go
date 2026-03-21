package validation

import (
	"strings"
	"testing"
)

func TestReviewDetailsChecks_DemoCredentialRemediationExplainsOptIn(t *testing.T) {
	checks := reviewDetailsChecks(&ReviewDetails{
		ID:                  "detail-1",
		ContactFirstName:    "A",
		ContactLastName:     "B",
		ContactEmail:        "a@example.com",
		ContactPhone:        "123",
		DemoAccountRequired: true,
	})

	foundFields := map[string]bool{
		"demoAccountName":     false,
		"demoAccountPassword": false,
	}

	for _, check := range checks {
		if _, ok := foundFields[check.Field]; !ok {
			continue
		}
		foundFields[check.Field] = true
		if !strings.Contains(check.Remediation, "demoAccountRequired=true") {
			t.Fatalf("expected remediation for %s to mention explicit opt-in, got %q", check.Field, check.Remediation)
		}
		if !strings.Contains(check.Remediation, "notes") {
			t.Fatalf("expected remediation for %s to mention notes as supplemental guidance, got %q", check.Field, check.Remediation)
		}
	}

	for field, found := range foundFields {
		if !found {
			t.Fatalf("expected check for %s, got %v", field, checks)
		}
	}
}
