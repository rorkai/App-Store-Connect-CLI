package validation

import (
	"fmt"
	"strings"
)

const (
	contentRightsDoesNotUseThirdPartyContent = "DOES_NOT_USE_THIRD_PARTY_CONTENT"
	contentRightsUsesThirdPartyContent       = "USES_THIRD_PARTY_CONTENT"

	appEncryptionDeclarationStateCreated  = "CREATED"
	appEncryptionDeclarationStateInReview = "IN_REVIEW"
	appEncryptionDeclarationStateApproved = "APPROVED"
	appEncryptionDeclarationStateRejected = "REJECTED"
	appEncryptionDeclarationStateInvalid  = "INVALID"
	appEncryptionDeclarationStateExpired  = "EXPIRED"
)

func contentRightsChecks(appID string, declaration *string) []CheckResult {
	if declaration == nil || strings.TrimSpace(*declaration) == "" {
		return []CheckResult{
			{
				ID:           "content_rights.missing",
				Severity:     SeverityError,
				Field:        "contentRightsDeclaration",
				ResourceType: "app",
				ResourceID:   strings.TrimSpace(appID),
				Message:      "content rights declaration is not set",
				Remediation:  "Declare whether the app uses third-party content",
			},
		}
	}

	switch strings.ToUpper(strings.TrimSpace(*declaration)) {
	case contentRightsDoesNotUseThirdPartyContent, contentRightsUsesThirdPartyContent:
		return nil
	default:
		return []CheckResult{
			{
				ID:           "content_rights.invalid",
				Severity:     SeverityError,
				Field:        "contentRightsDeclaration",
				ResourceType: "app",
				ResourceID:   strings.TrimSpace(appID),
				Message:      fmt.Sprintf("content rights declaration has unsupported value %q", strings.TrimSpace(*declaration)),
				Remediation:  "Declare whether the app uses third-party content",
			},
		}
	}
}

func buildEncryptionChecks(build *Build) []CheckResult {
	if build == nil || strings.TrimSpace(build.ID) == "" {
		return nil
	}

	if build.UsesNonExemptEncryption == nil {
		return []CheckResult{
			{
				ID:           "build.encryption.missing",
				Severity:     SeverityError,
				Field:        "usesNonExemptEncryption",
				ResourceType: "build",
				ResourceID:   strings.TrimSpace(build.ID),
				Message:      "build encryption state is not set",
				Remediation:  fmt.Sprintf("Set Uses Non-Exempt Encryption for build %s in App Store Connect", strings.TrimSpace(build.ID)),
			},
		}
	}

	if !*build.UsesNonExemptEncryption {
		return nil
	}

	declarationID := strings.TrimSpace(build.AppEncryptionDeclarationID)
	declarationState := strings.ToUpper(strings.TrimSpace(build.AppEncryptionDeclarationState))
	if declarationID == "" {
		return []CheckResult{
			{
				ID:           "build.encryption.declaration_missing",
				Severity:     SeverityError,
				Field:        "appEncryptionDeclaration",
				ResourceType: "build",
				ResourceID:   strings.TrimSpace(build.ID),
				Message:      "build uses non-exempt encryption but has no attached encryption declaration",
				Remediation:  fmt.Sprintf("Attach an encryption declaration: asc encryption declarations assign-builds --id DECLARATION_ID --build-id %s", strings.TrimSpace(build.ID)),
			},
		}
	}

	switch declarationState {
	case appEncryptionDeclarationStateApproved:
		return nil
	case appEncryptionDeclarationStateCreated, appEncryptionDeclarationStateInReview:
		return []CheckResult{
			{
				ID:           "build.encryption.declaration_pending",
				Severity:     SeverityError,
				Field:        "appEncryptionDeclarationState",
				ResourceType: "build",
				ResourceID:   strings.TrimSpace(build.ID),
				Message:      fmt.Sprintf("attached encryption declaration %s is still %s", declarationID, declarationState),
				Remediation:  "Wait for the encryption declaration to reach APPROVED in App Store Connect",
			},
		}
	case appEncryptionDeclarationStateRejected, appEncryptionDeclarationStateInvalid, appEncryptionDeclarationStateExpired:
		return []CheckResult{
			{
				ID:           "build.encryption.declaration_invalid",
				Severity:     SeverityError,
				Field:        "appEncryptionDeclarationState",
				ResourceType: "build",
				ResourceID:   strings.TrimSpace(build.ID),
				Message:      fmt.Sprintf("attached encryption declaration %s is %s", declarationID, declarationState),
				Remediation:  fmt.Sprintf("Attach an approved encryption declaration: asc encryption declarations assign-builds --id DECLARATION_ID --build-id %s", strings.TrimSpace(build.ID)),
			},
		}
	case "":
		return []CheckResult{
			{
				ID:           "build.encryption.declaration_state_missing",
				Severity:     SeverityError,
				Field:        "appEncryptionDeclarationState",
				ResourceType: "build",
				ResourceID:   strings.TrimSpace(build.ID),
				Message:      fmt.Sprintf("attached encryption declaration %s is missing approval state", declarationID),
				Remediation:  fmt.Sprintf("Inspect the attached declaration: asc builds app-encryption-declaration view --build-id %s", strings.TrimSpace(build.ID)),
			},
		}
	default:
		return []CheckResult{
			{
				ID:           "build.encryption.declaration_unknown_state",
				Severity:     SeverityError,
				Field:        "appEncryptionDeclarationState",
				ResourceType: "build",
				ResourceID:   strings.TrimSpace(build.ID),
				Message:      fmt.Sprintf("attached encryption declaration %s has unsupported state %q", declarationID, strings.TrimSpace(build.AppEncryptionDeclarationState)),
				Remediation:  fmt.Sprintf("Inspect the attached declaration: asc builds app-encryption-declaration view --build-id %s", strings.TrimSpace(build.ID)),
			},
		}
	}
}
