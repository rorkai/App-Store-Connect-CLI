package validation

import (
	"strings"
	"testing"
)

func TestReviewDetailsChecks_MissingDetails(t *testing.T) {
	checks := reviewDetailsChecks(nil)
	if !hasCheckID(checks, "review_details.missing") {
		t.Fatalf("expected review_details.missing check, got %v", checks)
	}
}

func TestReviewDetailsChecks_MissingContactFields(t *testing.T) {
	checks := reviewDetailsChecks(&ReviewDetails{
		ID: "detail-1",
		// All contact fields empty.
	})

	needFields := map[string]bool{
		"contactFirstName": false,
		"contactLastName":  false,
		"contactEmail":     false,
		"contactPhone":     false,
	}

	for _, c := range checks {
		if c.ID != "review_details.missing_field" {
			continue
		}
		if _, ok := needFields[c.Field]; ok {
			needFields[c.Field] = true
		}
	}

	for field, found := range needFields {
		if !found {
			t.Fatalf("expected missing field check for %s, got %v", field, checks)
		}
	}
}

func TestReviewDetailsChecks_DemoAccountRequiredMissingCredentials(t *testing.T) {
	checks := reviewDetailsChecks(&ReviewDetails{
		ID:                  "detail-1",
		ContactFirstName:    "A",
		ContactLastName:     "B",
		ContactEmail:        "a@example.com",
		ContactPhone:        "123",
		DemoAccountRequired: true,
		// Missing demo account name/password.
	})

	needFields := map[string]bool{
		"demoAccountName":     false,
		"demoAccountPassword": false,
	}

	for _, c := range checks {
		if c.ID != "review_details.missing_field" {
			continue
		}
		if _, ok := needFields[c.Field]; ok {
			needFields[c.Field] = true
		}
	}

	for field, found := range needFields {
		if !found {
			t.Fatalf("expected missing field check for %s, got %v", field, checks)
		}
	}
}

func TestReviewDetailsChecks_Pass(t *testing.T) {
	checks := reviewDetailsChecks(&ReviewDetails{
		ID:                  "detail-1",
		ContactFirstName:    "A",
		ContactLastName:     "B",
		ContactEmail:        "a@example.com",
		ContactPhone:        "123",
		DemoAccountRequired: false,
	})
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d (%v)", len(checks), checks)
	}
}

func TestReviewDetailsChecks_PassWithDemoAccount(t *testing.T) {
	checks := reviewDetailsChecks(&ReviewDetails{
		ID:                  "detail-1",
		ContactFirstName:    "A",
		ContactLastName:     "B",
		ContactEmail:        "a@example.com",
		ContactPhone:        "123",
		DemoAccountRequired: true,
		DemoAccountName:     "demo",
		DemoAccountPassword: "pass",
	})
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %d (%v)", len(checks), checks)
	}
}

func TestReviewDetailsChecks_DemoCredentialPermutations(t *testing.T) {
	for _, required := range []bool{false, true} {
		for _, hasName := range []bool{false, true} {
			for _, hasPassword := range []bool{false, true} {
				for _, hasNotes := range []bool{false, true} {
					name := ""
					if hasName {
						name = "reviewer@example.com"
					}
					password := ""
					if hasPassword {
						password = "app-specific-password"
					}
					notes := ""
					if hasNotes {
						notes = "Reviewer can use the guest flow from the welcome screen."
					}

					caseName := strings.Join([]string{
						"required=" + boolLabel(required),
						"name=" + boolLabel(hasName),
						"password=" + boolLabel(hasPassword),
						"notes=" + boolLabel(hasNotes),
					}, "/")

					t.Run(caseName, func(t *testing.T) {
						checks := reviewDetailsChecks(&ReviewDetails{
							ID:                  "detail-1",
							ContactFirstName:    "A",
							ContactLastName:     "B",
							ContactEmail:        "a@example.com",
							ContactPhone:        "123",
							DemoAccountRequired: required,
							DemoAccountName:     name,
							DemoAccountPassword: password,
							Notes:               notes,
						})

						expectedMissing := map[string]bool{}
						if required {
							if !hasName {
								expectedMissing["demoAccountName"] = false
							}
							if !hasPassword {
								expectedMissing["demoAccountPassword"] = false
							}
						}

						foundMissing := map[string]bool{}
						for _, check := range checks {
							if check.ID != "review_details.missing_field" {
								t.Fatalf("unexpected check for %s: %+v", caseName, check)
							}
							if _, ok := expectedMissing[check.Field]; !ok {
								t.Fatalf("unexpected missing field for %s: %+v", caseName, check)
							}
							foundMissing[check.Field] = true
							if !strings.Contains(check.Remediation, "demoAccountRequired=true") {
								t.Fatalf("expected remediation to mention explicit opt-in for %s: %+v", caseName, check)
							}
							if !strings.Contains(check.Remediation, "notes") {
								t.Fatalf("expected remediation to mention notes for %s: %+v", caseName, check)
							}
						}

						if len(expectedMissing) != len(foundMissing) {
							t.Fatalf("expected %d missing fields for %s, got %d (%+v)", len(expectedMissing), caseName, len(foundMissing), checks)
						}
						for field := range expectedMissing {
							if !foundMissing[field] {
								t.Fatalf("expected missing-field check for %s in %s, got %+v", field, caseName, checks)
							}
						}
						if !required && len(checks) != 0 {
							t.Fatalf("expected no demo credential checks when demoAccountRequired=false for %s, got %+v", caseName, checks)
						}
					})
				}
			}
		}
	}
}

func boolLabel(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
