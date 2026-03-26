package validation

import (
	"fmt"
	"strings"
)

const privacyPublishStateUnverifiedID = "privacy.publish_state.unverified"

// PrivacyPublishStateAdvisory returns a public-only advisory for App Privacy.
func PrivacyPublishStateAdvisory(appID string) CheckResult {
	trimmedAppID := strings.TrimSpace(appID)
	if trimmedAppID == "" {
		return CheckResult{}
	}

	return CheckResult{
		ID:           privacyPublishStateUnverifiedID,
		Severity:     SeverityInfo,
		ResourceType: "appPrivacy",
		ResourceID:   trimmedAppID,
		Message:      "App Privacy publish state is not verifiable via the public App Store Connect API and may still block submission",
		Remediation:  fmt.Sprintf("Confirm App Privacy is published in App Store Connect before submitting: https://appstoreconnect.apple.com/apps/%s/appPrivacy", trimmedAppID),
	}
}

func privacyPublishStateChecks(appID string) []CheckResult {
	advisory := PrivacyPublishStateAdvisory(appID)
	if advisory.ID == "" {
		return nil
	}
	return []CheckResult{advisory}
}
