package validation

import (
	"fmt"
	"strings"
)

// IAP represents an in-app purchase for review-readiness validation.
type IAP struct {
	ID        string
	Name      string
	ProductID string
	Type      string
	State     string
}

// IAPInput collects in-app purchase validation inputs.
type IAPInput struct {
	AppID string
	IAPs  []IAP
}

// IAPReport is the top-level validate iap output.
type IAPReport struct {
	AppID    string        `json:"appId"`
	IAPCount int           `json:"iapCount,omitempty"`
	Summary  Summary       `json:"summary"`
	Checks   []CheckResult `json:"checks"`
	Strict   bool          `json:"strict,omitempty"`
}

// ValidateIAP validates IAP review readiness and returns a report.
func ValidateIAP(input IAPInput, strict bool) IAPReport {
	checks := iapReviewReadinessChecks(input.IAPs)
	summary := summarize(checks, strict)

	return IAPReport{
		AppID:    strings.TrimSpace(input.AppID),
		IAPCount: len(input.IAPs),
		Summary:  summary,
		Checks:   checks,
		Strict:   strict,
	}
}

func iapFetchChecks(reason string) []CheckResult {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil
	}

	return []CheckResult{{
		ID:          "iap.readiness.unverified",
		Severity:    SeverityInfo,
		Field:       "inAppPurchases",
		Message:     "Could not verify in-app purchase readiness for this app",
		Remediation: reason,
	}}
}

func iapReviewReadinessChecks(iaps []IAP) []CheckResult {
	// These checks are warnings by default. Many apps have legacy IAPs that
	// aren't relevant to a given release. Use --strict to gate in CI.
	okStates := map[string]struct{}{
		"APPROVED":                {},
		"WAITING_FOR_REVIEW":      {},
		"IN_REVIEW":               {},
		"PENDING_BINARY_APPROVAL": {},
	}
	var checks []CheckResult
	for _, iap := range iaps {
		state := normalizeMonetizationState(iap.State)
		if state == "" {
			continue
		}
		if _, ok := okStates[state]; ok {
			continue
		}
		if isRemovedMonetizationState(state) {
			continue
		}

		label := formatIAPLabel(iap)
		message := fmt.Sprintf("%s is %s", label, state)
		remediation := remediationForIAPState(state)

		checks = append(checks, CheckResult{
			ID:           "iap.review_readiness.needs_attention",
			Severity:     SeverityWarning,
			Field:        "state",
			ResourceType: "inAppPurchaseV2",
			ResourceID:   strings.TrimSpace(iap.ID),
			Message:      message,
			Remediation:  remediation,
		})
	}

	return checks
}

func formatIAPLabel(iap IAP) string {
	name := strings.TrimSpace(iap.Name)
	productID := strings.TrimSpace(iap.ProductID)

	switch {
	case name != "" && productID != "":
		return fmt.Sprintf("IAP %q (%s)", name, productID)
	case name != "":
		return fmt.Sprintf("IAP %q", name)
	case productID != "":
		return fmt.Sprintf("IAP %s", productID)
	default:
		return "IAP"
	}
}

func remediationForIAPState(state string) string {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "MISSING_METADATA":
		return "Complete required metadata for this in-app purchase in App Store Connect"
	case "READY_TO_SUBMIT":
		return "Submit this in-app purchase for review in App Store Connect"
	case "DEVELOPER_ACTION_NEEDED":
		return "Resolve developer action required issues for this in-app purchase in App Store Connect"
	case "REJECTED":
		return "Address the rejection feedback for this in-app purchase and resubmit in App Store Connect"
	case "WAITING_FOR_UPLOAD":
		return "Upload the required content for this in-app purchase in App Store Connect"
	case "PROCESSING_CONTENT":
		return "Wait for this in-app purchase content to finish processing in App Store Connect"
	default:
		return "Review this in-app purchase in App Store Connect and submit it for review if needed"
	}
}
