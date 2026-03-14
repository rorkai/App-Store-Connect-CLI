package shared

import (
	"fmt"
	"strings"
)

var reviewSubmissionStates = map[string]struct{}{
	"READY_FOR_REVIEW":   {},
	"WAITING_FOR_REVIEW": {},
	"IN_REVIEW":          {},
	"UNRESOLVED_ISSUES":  {},
	"CANCELING":          {},
	"COMPLETING":         {},
	"COMPLETE":           {},
}

// NormalizeReviewSubmissionStates validates multiple review submission state values.
func NormalizeReviewSubmissionStates(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	for _, value := range values {
		if _, ok := reviewSubmissionStates[strings.ToUpper(strings.TrimSpace(value))]; !ok {
			return nil, fmt.Errorf("--state must be one of: %s", strings.Join(reviewSubmissionStateList(), ", "))
		}
	}
	return values, nil
}

func reviewSubmissionStateList() []string {
	return []string{
		"READY_FOR_REVIEW",
		"WAITING_FOR_REVIEW",
		"IN_REVIEW",
		"UNRESOLVED_ISSUES",
		"CANCELING",
		"COMPLETING",
		"COMPLETE",
	}
}
