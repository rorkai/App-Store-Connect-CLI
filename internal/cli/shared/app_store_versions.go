package shared

import (
	"fmt"
	"strings"
)

var appStoreVersionPlatforms = map[string]struct{}{
	"IOS":       {},
	"MAC_OS":    {},
	"TV_OS":     {},
	"VISION_OS": {},
}

var appStoreVersionStates = map[string]struct{}{
	"ACCEPTED":                      {},
	"DEVELOPER_REMOVED_FROM_SALE":   {},
	"DEVELOPER_REJECTED":            {},
	"IN_REVIEW":                     {},
	"INVALID_BINARY":                {},
	"METADATA_REJECTED":             {},
	"PENDING_APPLE_RELEASE":         {},
	"PENDING_CONTRACT":              {},
	"PENDING_DEVELOPER_RELEASE":     {},
	"PREPARE_FOR_SUBMISSION":        {},
	"PREORDER_READY_FOR_SALE":       {},
	"PROCESSING_FOR_DISTRIBUTION":   {},
	"PROCESSING_FOR_APP_STORE":      {},
	"READY_FOR_DISTRIBUTION":        {},
	"READY_FOR_REVIEW":              {},
	"READY_FOR_SALE":                {},
	"REJECTED":                      {},
	"REMOVED_FROM_SALE":             {},
	"WAITING_FOR_EXPORT_COMPLIANCE": {},
	"WAITING_FOR_REVIEW":            {},
	"REPLACED_WITH_NEW_VERSION":     {},
	"NOT_APPLICABLE":                {},
}

// NormalizeAppStoreVersionPlatform validates a single platform value.
func NormalizeAppStoreVersionPlatform(value string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return "", fmt.Errorf("--platform is required")
	}
	if _, ok := appStoreVersionPlatforms[normalized]; !ok {
		return "", fmt.Errorf("--platform must be one of: %s", strings.Join(appStoreVersionPlatformList(), ", "))
	}
	return normalized, nil
}

// NormalizeAppStoreVersionPlatforms validates multiple platform values.
func NormalizeAppStoreVersionPlatforms(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	for _, value := range values {
		if _, ok := appStoreVersionPlatforms[value]; !ok {
			return nil, fmt.Errorf("--platform must be one of: %s", strings.Join(appStoreVersionPlatformList(), ", "))
		}
	}
	return values, nil
}

// NormalizeAppStoreVersionStates validates multiple state values.
func NormalizeAppStoreVersionStates(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	for _, value := range values {
		if _, ok := appStoreVersionStates[value]; !ok {
			return nil, fmt.Errorf("--state must be one of: %s", strings.Join(appStoreVersionStateList(), ", "))
		}
	}
	return values, nil
}

// ValidateAppStoreVersionStateFilterCombination rejects state combinations that
// cannot be represented by one App Store Connect version-list request.
func ValidateAppStoreVersionStateFilterCombination(values []string) error {
	hasAppVersionOnly := false
	appStoreOnly := make([]string, 0)
	for _, value := range values {
		switch value {
		case "PROCESSING_FOR_DISTRIBUTION", "READY_FOR_DISTRIBUTION":
			hasAppVersionOnly = true
		case "DEVELOPER_REMOVED_FROM_SALE",
			"PENDING_CONTRACT",
			"PREORDER_READY_FOR_SALE",
			"PROCESSING_FOR_APP_STORE",
			"READY_FOR_SALE",
			"REMOVED_FROM_SALE",
			"NOT_APPLICABLE":
			appStoreOnly = append(appStoreOnly, value)
		}
	}
	if hasAppVersionOnly && len(appStoreOnly) > 0 {
		return fmt.Errorf("--state cannot mix appVersionState-only values with appStoreState-only values (%s); run separate commands for those states", strings.Join(appStoreOnly, ", "))
	}
	return nil
}

func appStoreVersionPlatformList() []string {
	return []string{"IOS", "MAC_OS", "TV_OS", "VISION_OS"}
}

func appStoreVersionStateList() []string {
	return []string{
		"ACCEPTED",
		"DEVELOPER_REMOVED_FROM_SALE",
		"DEVELOPER_REJECTED",
		"IN_REVIEW",
		"INVALID_BINARY",
		"METADATA_REJECTED",
		"PENDING_APPLE_RELEASE",
		"PENDING_CONTRACT",
		"PENDING_DEVELOPER_RELEASE",
		"PREPARE_FOR_SUBMISSION",
		"PREORDER_READY_FOR_SALE",
		"PROCESSING_FOR_DISTRIBUTION",
		"PROCESSING_FOR_APP_STORE",
		"READY_FOR_DISTRIBUTION",
		"READY_FOR_REVIEW",
		"READY_FOR_SALE",
		"REJECTED",
		"REMOVED_FROM_SALE",
		"WAITING_FOR_EXPORT_COMPLIANCE",
		"WAITING_FOR_REVIEW",
		"REPLACED_WITH_NEW_VERSION",
		"NOT_APPLICABLE",
	}
}
