package validation

import "strings"

const reviewDetailsDemoCredentialsRemediation = "You set demoAccountRequired=true, so provide both demoAccountName and demoAccountPassword in App Store Connect. Use notes for supplemental reviewer guidance, not as a replacement for required credentials."

func reviewDetailsChecks(details *ReviewDetails) []CheckResult {
	if details == nil {
		return []CheckResult{
			{
				ID:           "review_details.missing",
				Severity:     SeverityError,
				ResourceType: "appStoreReviewDetail",
				Message:      "app store review details are missing",
				Remediation:  "Create App Store review details for this version in App Store Connect",
			},
		}
	}

	var checks []CheckResult
	resourceID := strings.TrimSpace(details.ID)

	if strings.TrimSpace(details.ContactFirstName) == "" {
		checks = append(checks, missingReviewDetailsField("contactFirstName", resourceID))
	}
	if strings.TrimSpace(details.ContactLastName) == "" {
		checks = append(checks, missingReviewDetailsField("contactLastName", resourceID))
	}
	if strings.TrimSpace(details.ContactEmail) == "" {
		checks = append(checks, missingReviewDetailsField("contactEmail", resourceID))
	}
	if strings.TrimSpace(details.ContactPhone) == "" {
		checks = append(checks, missingReviewDetailsField("contactPhone", resourceID))
	}

	// Only require demo account credentials when the user explicitly marks them as required.
	if details.DemoAccountRequired {
		if strings.TrimSpace(details.DemoAccountName) == "" {
			checks = append(checks, missingReviewDetailsField("demoAccountName", resourceID))
		}
		if strings.TrimSpace(details.DemoAccountPassword) == "" {
			checks = append(checks, missingReviewDetailsField("demoAccountPassword", resourceID))
		}
	}

	return checks
}

func missingReviewDetailsField(field string, resourceID string) CheckResult {
	remediation := "Complete App Store review details in App Store Connect"
	switch field {
	case "demoAccountName", "demoAccountPassword":
		remediation = reviewDetailsDemoCredentialsRemediation
	}

	return CheckResult{
		ID:           "review_details.missing_field",
		Severity:     SeverityError,
		Field:        field,
		ResourceType: "appStoreReviewDetail",
		ResourceID:   resourceID,
		Message:      "review detail field is missing",
		Remediation:  remediation,
	}
}
