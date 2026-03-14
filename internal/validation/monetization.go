package validation

import "strings"

func hasActiveMonetization(subs []Subscription, iaps []IAP) bool {
	for _, sub := range subs {
		if isActiveMonetizationState(sub.State) {
			return true
		}
	}
	for _, iap := range iaps {
		if isActiveMonetizationState(iap.State) {
			return true
		}
	}
	return false
}

func hasReviewRelevantSubscriptions(subs []Subscription) bool {
	for _, sub := range subs {
		if isActiveMonetizationState(sub.State) {
			return true
		}
	}
	return false
}

func isActiveMonetizationState(state string) bool {
	normalized := normalizeMonetizationState(state)
	return normalized != "" && !isRemovedMonetizationState(normalized)
}

func isRemovedMonetizationState(state string) bool {
	switch normalizeMonetizationState(state) {
	case "REMOVED_FROM_SALE", "DEVELOPER_REMOVED_FROM_SALE":
		return true
	default:
		return false
	}
}

func normalizeMonetizationState(state string) string {
	return strings.ToUpper(strings.TrimSpace(state))
}
