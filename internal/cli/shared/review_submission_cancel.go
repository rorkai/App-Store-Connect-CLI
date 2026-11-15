package shared

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const reviewSubmissionNotCancellableDetail = "not in cancellable state"

// IsReviewSubmissionNotCancellableError reports whether err is App Store
// Connect refusing to cancel a review submission in its current state
// ("Resource state is invalid.: Resource is not in cancellable state"). It
// matches the specific detail, in any errors[] entry, not the generic title.
func IsReviewSubmissionNotCancellableError(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(strings.ToLower(err.Error()), reviewSubmissionNotCancellableDetail) {
		return true
	}
	apiErr, ok := errors.AsType[*asc.APIError](err)
	if !ok || apiErr == nil {
		return false
	}
	for _, entry := range apiErr.Entries {
		if strings.Contains(strings.ToLower(entry.Detail), reviewSubmissionNotCancellableDetail) {
			return true
		}
	}
	return false
}

// ReviewSubmissionNotCancellableError explains why App Store Connect refused
// to cancel a review submission, using the state read after the refusal. It
// wraps the original API error so exit codes and error matching are unchanged.
type ReviewSubmissionNotCancellableError struct {
	SubmissionID string
	State        asc.ReviewSubmissionState
	Err          error
}

func (e *ReviewSubmissionNotCancellableError) Error() string {
	return fmt.Sprintf(
		"review submission %s cannot be canceled in state %s: %s (App Store Connect: %v)",
		e.SubmissionID,
		e.State,
		reviewSubmissionNotCancellableGuidance(e.SubmissionID, e.State),
		e.Err,
	)
}

func (e *ReviewSubmissionNotCancellableError) Unwrap() error {
	return e.Err
}

func reviewSubmissionNotCancellableGuidance(submissionID string, state asc.ReviewSubmissionState) string {
	switch state {
	case asc.ReviewSubmissionStateReadyForReview:
		return fmt.Sprintf(
			"it is an unsubmitted draft, so there is no review to cancel. To change what it contains, list its items with `asc review items-list --submission %s` and remove one with `asc review items-remove --id ITEM_ID --confirm`; to send it for review, run `asc review submissions-submit --id %s --confirm`.",
			submissionID,
			submissionID,
		)
	case asc.ReviewSubmissionStateWaitingForReview, asc.ReviewSubmissionStateInReview:
		return fmt.Sprintf(
			"App Store Connect usually allows canceling in this state, so the submission may be changing state. Check it with `asc review submissions-get --id %s` before retrying.",
			submissionID,
		)
	case asc.ReviewSubmissionStateUnresolvedIssues:
		return fmt.Sprintf(
			"App Review returned it with issues to resolve, and it cannot be canceled in this state. Read the issues in the App Store Connect Resolution Center and reply there, or fix the items, mark each one with `asc review items-update --id ITEM_ID --resolved true`, and resubmit with `asc review submissions-submit --id %s --confirm`.",
			submissionID,
		)
	case asc.ReviewSubmissionStateCanceling:
		return fmt.Sprintf(
			"it is already being canceled, so no further action is needed. Check progress with `asc review submissions-get --id %s`.",
			submissionID,
		)
	case asc.ReviewSubmissionStateCompleting:
		return fmt.Sprintf(
			"App Review is finishing it, so there is nothing left to cancel. Check the result with `asc review submissions-get --id %s`.",
			submissionID,
		)
	case asc.ReviewSubmissionStateComplete:
		return fmt.Sprintf(
			"it has already finished (reviewed or canceled), so there is nothing left to cancel. Check the result with `asc review submissions-get --id %s`.",
			submissionID,
		)
	default:
		return fmt.Sprintf("check its current state with `asc review submissions-get --id %s`.", submissionID)
	}
}

// ExplainReviewSubmissionNotCancellable reads a review submission's current
// state after App Store Connect refused to cancel it and returns an error that
// explains the state and the next step. It returns cancelErr unchanged when
// cancelErr is not that refusal, or when the state cannot be read, so a failed
// follow-up read never hides the original error.
func ExplainReviewSubmissionNotCancellable(ctx context.Context, client *asc.Client, submissionID string, cancelErr error) error {
	return explainReviewSubmissionNotCancellable(ctx, ContextWithTimeout, client, submissionID, cancelErr)
}

// ExplainReviewSubmissionNotCancellable is ExplainReviewSubmissionNotCancellable
// with the follow-up read's deadline from d.Timeout.
func (d *Deps) ExplainReviewSubmissionNotCancellable(ctx context.Context, client *asc.Client, submissionID string, cancelErr error) error {
	return explainReviewSubmissionNotCancellable(ctx, d.Timeout, client, submissionID, cancelErr)
}

func explainReviewSubmissionNotCancellable(ctx context.Context, timeout func(context.Context) (context.Context, context.CancelFunc), client *asc.Client, submissionID string, cancelErr error) error {
	submissionID = strings.TrimSpace(submissionID)
	if client == nil || submissionID == "" || !IsReviewSubmissionNotCancellableError(cancelErr) {
		return cancelErr
	}

	readCtx, cancel := timeout(ctx)
	defer cancel()

	resp, err := client.GetReviewSubmissionStrict(readCtx, submissionID)
	if err != nil || resp == nil {
		return cancelErr
	}
	return NewReviewSubmissionNotCancellableError(submissionID, resp.Data.Attributes.SubmissionState, cancelErr)
}

// NewReviewSubmissionNotCancellableError explains a refused cancel from a
// state the caller already read. It returns cancelErr unchanged when state is
// empty.
func NewReviewSubmissionNotCancellableError(submissionID string, state asc.ReviewSubmissionState, cancelErr error) error {
	if strings.TrimSpace(string(state)) == "" {
		return cancelErr
	}
	return &ReviewSubmissionNotCancellableError{
		SubmissionID: strings.TrimSpace(submissionID),
		State:        asc.ReviewSubmissionState(strings.TrimSpace(string(state))),
		Err:          cancelErr,
	}
}
