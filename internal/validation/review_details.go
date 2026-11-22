package validation

import (
	"net/mail"
	"strings"
	"unicode"
)

const reviewDetailsDemoCredentialsRemediation = "You set demoAccountRequired=true, so provide both demoAccountName and demoAccountPassword in App Store Connect. Use notes for supplemental reviewer guidance, not as a replacement for required credentials."

// App Review has to be able to dial the contact number, so a value with too
// few digits cannot be a reachable number. The upper bound stays well above
// E.164's 15 digits so numbers written with an extension are not flagged.
const (
	minReviewContactPhoneDigits = 7
	maxReviewContactPhoneDigits = 20
)

func reviewDetailsChecks(details *ReviewDetails) []CheckResult {
	if details == nil {
		return []CheckResult{
			{
				ID:           "review_details.missing",
				Severity:     SeverityError,
				ResourceType: "appStoreReviewDetail",
				Message:      "app store review details are missing",
				Remediation:  "Create App Store review details for this version",
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
	if email := strings.TrimSpace(details.ContactEmail); email == "" {
		checks = append(checks, missingReviewDetailsField("contactEmail", resourceID))
	} else if !isBareEmailAddress(email) {
		checks = append(checks, CheckResult{
			ID:           "review_details.format.contact_email",
			Severity:     SeverityError,
			Field:        "contactEmail",
			ResourceType: "appStoreReviewDetail",
			ResourceID:   resourceID,
			Message:      "review contact email is not a valid email address",
			Remediation:  "Provide a plain email address App Review can reach, with no display name, for example: reviewer@example.com",
		})
	}

	if phone := strings.TrimSpace(details.ContactPhone); phone == "" {
		checks = append(checks, missingReviewDetailsField("contactPhone", resourceID))
	} else if !hasPlausiblePhoneDigits(phone) {
		checks = append(checks, CheckResult{
			ID:           "review_details.format.contact_phone",
			Severity:     SeverityWarning,
			Field:        "contactPhone",
			ResourceType: "appStoreReviewDetail",
			ResourceID:   resourceID,
			Message:      "review contact phone does not look like a reachable phone number",
			Remediation:  "Provide a phone number App Review can dial, including country and area code, for example: +1 555 010 1234",
		})
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

// isBareEmailAddress reports whether a value is a single address with no RFC
// display name, which is what App Store Connect stores in its contact email
// fields. It matches the sandbox tester email rule used elsewhere in the CLI.
func isBareEmailAddress(value string) bool {
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed != nil && strings.TrimSpace(parsed.Address) == value
}

// hasPlausiblePhoneDigits reports whether a free-text phone value carries a
// digit count a real phone number can have. App Store Connect stores the
// contact phone as free text, so formatting characters are ignored.
func hasPlausiblePhoneDigits(value string) bool {
	digits := 0
	for _, r := range value {
		if unicode.IsDigit(r) {
			digits++
		}
	}
	return digits >= minReviewContactPhoneDigits && digits <= maxReviewContactPhoneDigits
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
