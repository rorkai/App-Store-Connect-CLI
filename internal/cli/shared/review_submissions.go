package shared

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
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

// FetchAllAppStoreVersions returns all App Store versions for an app, following pagination links.
func FetchAllAppStoreVersions(ctx context.Context, client *asc.Client, appID string, opts ...asc.AppStoreVersionsOption) ([]asc.Resource[asc.AppStoreVersionAttributes], error) {
	firstPage, err := client.GetAppStoreVersions(ctx, appID, opts...)
	if err != nil {
		return nil, err
	}
	if firstPage == nil {
		return []asc.Resource[asc.AppStoreVersionAttributes]{}, nil
	}

	resp, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetAppStoreVersions(ctx, appID, asc.WithAppStoreVersionsNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}

	aggregated, ok := resp.(*asc.AppStoreVersionsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected app store versions pagination response type %T", resp)
	}
	if aggregated == nil || aggregated.Data == nil {
		return []asc.Resource[asc.AppStoreVersionAttributes]{}, nil
	}

	return aggregated.Data, nil
}

// FetchAllReviewSubmissions returns all review submissions for an app, following pagination links.
func FetchAllReviewSubmissions(ctx context.Context, client *asc.Client, appID string, opts ...asc.ReviewSubmissionsOption) ([]asc.ReviewSubmissionResource, error) {
	firstPage, err := client.GetReviewSubmissions(ctx, appID, opts...)
	if err != nil {
		return nil, err
	}
	if firstPage == nil {
		return []asc.ReviewSubmissionResource{}, nil
	}

	resp, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetReviewSubmissions(ctx, appID, asc.WithReviewSubmissionsNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}

	aggregated, ok := resp.(*asc.ReviewSubmissionsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected review submissions pagination response type %T", resp)
	}
	if aggregated == nil || aggregated.Data == nil {
		return []asc.ReviewSubmissionResource{}, nil
	}

	return aggregated.Data, nil
}

// ShouldPreferLatestReviewSubmission reports whether current should win over best when selecting the most relevant submission.
func ShouldPreferLatestReviewSubmission(current, best asc.ReviewSubmissionResource) bool {
	currentPriority := reviewSubmissionPriority(current.Attributes.SubmissionState)
	bestPriority := reviewSubmissionPriority(best.Attributes.SubmissionState)

	currentTime, currentValid := ParseRFC3339Date(current.Attributes.SubmittedDate)
	bestTime, bestValid := ParseRFC3339Date(best.Attributes.SubmittedDate)

	switch {
	case currentValid && bestValid:
		if currentTime.After(bestTime) {
			return true
		}
		if currentTime.Before(bestTime) {
			return false
		}
	case currentValid != bestValid:
		if currentPriority != bestPriority {
			return currentPriority > bestPriority
		}
		return currentValid
	}

	if currentPriority != bestPriority {
		return currentPriority > bestPriority
	}
	return current.ID > best.ID
}

// CompareRFC3339DateStrings compares two RFC3339 timestamps, falling back to trimmed string comparison when parsing fails.
func CompareRFC3339DateStrings(current, best string) int {
	currentTime, currentValid := ParseRFC3339Date(current)
	bestTime, bestValid := ParseRFC3339Date(best)

	switch {
	case currentValid && bestValid:
		if currentTime.After(bestTime) {
			return 1
		}
		if currentTime.Before(bestTime) {
			return -1
		}
		return 0
	case currentValid:
		return 1
	case bestValid:
		return -1
	default:
		current = strings.TrimSpace(current)
		best = strings.TrimSpace(best)
		if current > best {
			return 1
		}
		if current < best {
			return -1
		}
		return 0
	}
}

// ParseRFC3339Date parses a timestamp in RFC3339 or RFC3339Nano form.
func ParseRFC3339Date(value string) (time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed, true
	}
	if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
		return parsed, true
	}
	return time.Time{}, false
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

func reviewSubmissionPriority(state asc.ReviewSubmissionState) int {
	switch state {
	case asc.ReviewSubmissionStateReadyForReview,
		asc.ReviewSubmissionStateWaitingForReview,
		asc.ReviewSubmissionStateInReview,
		asc.ReviewSubmissionStateUnresolvedIssues,
		asc.ReviewSubmissionStateCanceling:
		return 2
	default:
		return 1
	}
}
