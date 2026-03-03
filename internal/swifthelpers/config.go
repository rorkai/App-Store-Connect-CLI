package swifthelpers

import (
	"os"
	"strings"
)

// Configuration constants for Swift helper usage
const (
	// EnvDisableSwiftHelpers disables all Swift helper usage when set to "true"
	EnvDisableSwiftHelpers = "ASC_DISABLE_SWIFT_HELPERS"

	// EnvPreferSwiftHelpers forces Swift helpers to be preferred even if not on macOS
	// (mostly for testing)
	EnvPreferSwiftHelpers = "ASC_PREFER_SWIFT_HELPERS"

	// EnvSwiftHelperPath allows specifying custom path for Swift helpers
	EnvSwiftHelperPath = "ASC_SWIFT_HELPER_PATH"
)

// UseSwiftHelpers returns true if Swift helpers should be used.
// Checks environment variable and platform availability.
func UseSwiftHelpers() bool {
	// Check if explicitly disabled
	if isDisabled := getEnvBool(EnvDisableSwiftHelpers); isDisabled {
		return false
	}

	// Check if explicitly preferred (for testing)
	if isPreferred := getEnvBool(EnvPreferSwiftHelpers); isPreferred {
		return true
	}

	// Default: use if available on this platform
	return IsAvailable()
}

// getEnvBool checks if an environment variable is set to a truthy value
func getEnvBool(key string) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// GetSwiftHelperPath returns the custom path for Swift helpers if set
func GetSwiftHelperPath() string {
	return os.Getenv(EnvSwiftHelperPath)
}
