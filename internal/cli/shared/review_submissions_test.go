package shared

import (
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestCompareRFC3339DateStringsParsesOffsets(t *testing.T) {
	t.Parallel()

	older := "2026-02-20T01:00:00+01:00"
	newer := "2026-02-20T00:30:00Z"

	if cmp := CompareRFC3339DateStrings(newer, older); cmp <= 0 {
		t.Fatalf("expected %q to be newer than %q, got %d", newer, older, cmp)
	}
	if cmp := CompareRFC3339DateStrings(older, newer); cmp >= 0 {
		t.Fatalf("expected %q to be older than %q, got %d", older, newer, cmp)
	}
}

func TestShouldPreferLatestReviewSubmissionPrefersActiveSubmissionWithoutSubmittedDate(t *testing.T) {
	t.Parallel()

	current := asc.ReviewSubmissionResource{
		ID: "sub-ready",
		Attributes: asc.ReviewSubmissionAttributes{
			SubmissionState: asc.ReviewSubmissionStateReadyForReview,
			SubmittedDate:   "",
		},
	}
	best := asc.ReviewSubmissionResource{
		ID: "sub-complete",
		Attributes: asc.ReviewSubmissionAttributes{
			SubmissionState: asc.ReviewSubmissionStateComplete,
			SubmittedDate:   "2026-03-16T10:00:00Z",
		},
	}

	if !ShouldPreferLatestReviewSubmission(current, best) {
		t.Fatal("expected active ready-for-review submission to win")
	}
}

func TestShouldPreferLatestReviewSubmissionBreaksTiesByID(t *testing.T) {
	t.Parallel()

	current := asc.ReviewSubmissionResource{
		ID: "sub-2",
		Attributes: asc.ReviewSubmissionAttributes{
			SubmittedDate: "2026-02-20T00:00:00Z",
		},
	}
	best := asc.ReviewSubmissionResource{
		ID: "sub-1",
		Attributes: asc.ReviewSubmissionAttributes{
			SubmittedDate: "2026-02-20T00:00:00Z",
		},
	}

	if !ShouldPreferLatestReviewSubmission(current, best) {
		t.Fatal("expected larger ID to win deterministic tie-break")
	}
}
