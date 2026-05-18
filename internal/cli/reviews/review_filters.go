package reviews

import (
	"fmt"
	"strings"
)

const (
	reviewResponseStateAny         = "any"
	reviewResponseStateUnresponded = "unresponded"
	reviewResponseStateResponded   = "responded"
)

func normalizeReviewResponseState(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		normalized = reviewResponseStateAny
	}
	switch normalized {
	case reviewResponseStateAny, reviewResponseStateUnresponded, reviewResponseStateResponded:
		return normalized, nil
	case "unreplied":
		return reviewResponseStateUnresponded, nil
	case "replied":
		return reviewResponseStateResponded, nil
	default:
		return "", fmt.Errorf("--response-state must be one of: any, unresponded, unreplied, responded, replied")
	}
}

func normalizeReviewResponseFields(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	allowed := map[string]bool{
		"responseBody":     true,
		"lastModifiedDate": true,
		"state":            true,
		"review":           true,
	}
	fields := strings.Split(value, ",")
	normalized := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if !allowed[field] {
			return nil, fmt.Errorf("--response-fields must be a comma-separated list of: responseBody,lastModifiedDate,state,review")
		}
		normalized = append(normalized, field)
	}
	return normalized, nil
}

func publishedResponseExistsFilter(responseState string) (bool, bool) {
	switch responseState {
	case reviewResponseStateUnresponded:
		return false, true
	case reviewResponseStateResponded:
		return true, true
	default:
		return false, false
	}
}
