package shared

import (
	"fmt"
	"strings"
)

// NormalizeEnumToken normalizes enum-like flag values by uppercasing and
// converting separators to underscore form.
func NormalizeEnumToken(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	normalized := strings.ToUpper(trimmed)
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	return normalized
}

// NormalizeOfferCodeEnvironment validates the --environment flag of one-time
// use offer code creation. An empty value stays empty so the attribute is omitted.
func NormalizeOfferCodeEnvironment(value string) (string, error) {
	normalized := strings.TrimSpace(strings.ToUpper(value))
	if normalized == "" {
		return "", nil
	}

	switch normalized {
	case "PRODUCTION", "SANDBOX":
		return normalized, nil
	default:
		return "", fmt.Errorf("--environment must be one of: PRODUCTION, SANDBOX")
	}
}

// ParseBoolFlag parses common bool-like flag values and returns a usage-style
// error with the provided flagName when parsing fails.
func ParseBoolFlag(value, flagName string) (bool, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be true or false", flagName)
	}
}
