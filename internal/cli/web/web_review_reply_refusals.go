package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// Codes Apple returns with a 409 from the Resolution Center write endpoints.
const (
	// resolutionCenterDraftExistsCode answers POST /resolutionCenterDraftMessages
	// when the thread already holds its one unsent draft.
	resolutionCenterDraftExistsCode = "STATE_ERROR.ALREADY_EXISTS"
	// resolutionCenterStateErrorCode answers POST /resolutionCenterMessages when
	// the thread does not take a developer message in its current state.
	resolutionCenterStateErrorCode = "STATE_ERROR"
)

// webReviewDefiniteRefusal returns Apple's API error when err is a 4xx other
// than 408: Apple answered and did not apply the write.
func webReviewDefiniteRefusal(err error) (*webcore.APIError, bool) {
	apiErr, ok := errors.AsType[*webcore.APIError](err)
	if !ok || apiErr.Status < 400 || apiErr.Status >= 500 || apiErr.Status == http.StatusRequestTimeout {
		return nil, false
	}
	return apiErr, true
}

func webReviewDraftDeleteCommand(threadID, draftID string) string {
	return fmt.Sprintf("asc web review drafts delete --app APP_ID --thread-id %q --draft-id %q --confirm", threadID, draftID)
}

// webReviewReplyDraftCreateError explains a refused draft create. When the
// thread already has a draft it reads that draft, best effort, so the caller
// learns which draft blocks the reply and how to remove it.
func webReviewReplyDraftCreateError(ctx context.Context, client *webcore.Client, threadID string, err error) error {
	apiErr, ok := webReviewDefiniteRefusal(err)
	if !ok || apiErr.Status != http.StatusConflict || !slices.Contains(apiErr.AllCodes(), resolutionCenterDraftExistsCode) {
		return webReviewMutationError(err, "web review reply")
	}
	existing, readErr := client.GetResolutionCenterDraftMessage(ctx, threadID, false)
	if readErr != nil || existing == nil || strings.TrimSpace(existing.ID) == "" {
		return &shared.HintedError{
			Err:  fmt.Errorf("web review reply failed: thread %q already has an unsent draft; nothing was sent: %w", threadID, err),
			Hint: "App Store Connect keeps one unsent draft per thread. Find its draft ID with `asc web review threads --app APP_ID --drafts`, delete it with `asc web review drafts delete --app APP_ID --thread-id THREAD_ID --draft-id DRAFT_ID --confirm`, then reply again.",
		}
	}
	draftID := strings.TrimSpace(existing.ID)
	created := ""
	if date := strings.TrimSpace(existing.CreatedDate); date != "" {
		created = " (created " + date + ")"
	}
	return &shared.HintedError{
		Err: fmt.Errorf("web review reply failed: thread %q already has unsent draft %q%s; nothing was sent: %w", threadID, draftID, created, err),
		Hint: fmt.Sprintf("App Store Connect keeps one unsent draft per thread, and a reply needs a new one. Read the draft with `asc web review threads --app APP_ID --drafts`. If it is not needed, delete it with `%s`, then reply again.",
			webReviewDraftDeleteCommand(threadID, draftID)),
	}
}

// webReviewReplySendError explains a failed send of a created draft. Only a
// definite refusal says the reply was not sent; anything else keeps the
// unknown-outcome warning because Apple may have posted the message.
func webReviewReplySendError(threadID, draftID string, err error) error {
	apiErr, ok := webReviewDefiniteRefusal(err)
	if !ok {
		return fmt.Errorf("web review reply failed: draft %s was created but send outcome is unknown; do not retry automatically: %w", draftID, err)
	}
	refusal := fmt.Errorf("web review reply failed: Apple refused to send draft %s, so the reply was not sent and the draft stays unsent on thread %q: %w", draftID, threadID, err)
	deleteDraft := fmt.Sprintf("Delete the unsent draft with `%s` before replying again.", webReviewDraftDeleteCommand(threadID, draftID))
	if apiErr.Status == http.StatusConflict && slices.Contains(apiErr.AllCodes(), resolutionCenterStateErrorCode) {
		return &shared.HintedError{
			Err: refusal,
			Hint: "Apple answers STATE_ERROR when the thread does not take a reply in its current state, for example after the rejected submission was resubmitted and is waiting for review. Check the submission with `asc review status --app APP_ID`; while it is in review, put what App Review needs to know in the review notes (`asc review details-update`) instead. " +
				deleteDraft,
		}
	}
	return &shared.HintedError{Err: refusal, Hint: deleteDraft}
}
