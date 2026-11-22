package reviews

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// reviewItemNotReviewableCode is Apple's 409 code on POST
// /v1/reviewSubmissionItems when the item itself cannot be reviewed; the
// reasons arrive as meta.associatedErrors keyed by the resource that blocks it.
const reviewItemNotReviewableCode = "STATE_ERROR.ENTITY_STATE_INVALID"

var reviewItemOnAnotherSubmissionPattern = regexp.MustCompile(`already added to another reviewSubmission with id ([0-9A-Za-z-]+)`)

// withReviewItemsAddConflictHint replaces the generic conflict hint of
// review items add with what the 409 means. A submission that is no longer a
// draft cannot take items, so the hint names its state and the draft to use.
// An item Apple calls not reviewable needs its associated errors fixed first,
// so the hint names the read that lists them. The submission read is best
// effort; when it fails, or the 409 is neither case, addErr is returned
// unchanged.
func withReviewItemsAddConflictHint(
	ctx context.Context,
	client *asc.Client,
	submissionID string,
	itemType asc.ReviewSubmissionItemType,
	itemID string,
	addErr error,
) error {
	apiErr, ok := errors.AsType[*asc.APIError](addErr)
	if !ok || apiErr.StatusCode != http.StatusConflict {
		return addErr
	}
	if apiErr.Code == reviewItemNotReviewableCode {
		return &shared.HintedError{Err: addErr, Hint: reviewItemNotReviewableHint(apiErr, itemType, itemID)}
	}
	if !strings.Contains(strings.ToLower(apiErr.Title+" "+apiErr.Detail), reviewSubmissionClosedText) {
		return addErr
	}

	submission, err := client.GetReviewSubmission(ctx, submissionID, asc.WithReviewSubmissionInclude([]string{"app"}))
	if err != nil {
		return addErr
	}
	state := strings.TrimSpace(string(submission.Data.Attributes.SubmissionState))
	if state == "" || state == string(asc.ReviewSubmissionStateReadyForReview) {
		return addErr
	}
	platform := strings.TrimSpace(string(submission.Data.Attributes.Platform))
	appID := ""
	if rel := submission.Data.Relationships; rel != nil && rel.App != nil {
		appID = strings.TrimSpace(rel.App.Data.ID)
	}
	hint := fmt.Sprintf("Submission %s is %s, and App Store Connect adds items only to a draft submission (READY_FOR_REVIEW).", submissionID, state)
	if draft := firstDraftReviewSubmission(ctx, client, appID, platform, submissionID); draft != "" {
		hint += fmt.Sprintf(" Submission %s is a draft for %s: add the item there with `asc review items add --submission %s --item-type %s --item-id %s`.", draft, platform, draft, itemType, itemID)
	} else {
		hint += fmt.Sprintf(" Create a draft with `asc review submissions-create --app %s --platform %s`, then add the item to it.", valueOrPlaceholder(appID, "APP_ID"), valueOrPlaceholder(platform, "PLATFORM"))
	}
	return &shared.HintedError{Err: addErr, Hint: hint}
}

// reviewSubmissionClosedText is in Apple's 409 when the submission is no
// longer a draft: "reviewSubmission state does not allow adding more items."
const reviewSubmissionClosedText = "reviewsubmission state does not allow adding"

// reviewItemNotReviewableHint points at the associated errors Apple already
// listed and the read that explains the common ones.
func reviewItemNotReviewableHint(apiErr *asc.APIError, itemType asc.ReviewSubmissionItemType, itemID string) string {
	hint := "App Store Connect cannot review this item yet: fix the associated errors listed above, then add it again."
	if itemType == asc.ReviewSubmissionItemTypeAppStoreVersion {
		hint += fmt.Sprintf(" `asc validate --app APP_ID --version-id %s` lists what the version is missing.", itemID)
	}
	for _, resource := range slices.Sorted(maps.Keys(apiErr.AssociatedErrors)) {
		for _, entry := range apiErr.AssociatedErrors[resource] {
			if match := reviewItemOnAnotherSubmissionPattern.FindStringSubmatch(entry.Detail); match != nil {
				hint += fmt.Sprintf(" The item is already on submission %s: inspect it with `asc review submissions-get --id %s --include items`, and submit that one or remove the item from it with `asc review items remove`.", match[1], match[1])
			}
		}
	}
	if associatedErrorsMention(apiErr, "compliance requirement") {
		hint += " An unfulfilled compliance requirement is an App Store Regulations & Permits answer the app still needs: list them with `asc web apps declarations list --app APP_ID`."
	}
	return hint
}

// firstDraftReviewSubmission returns another READY_FOR_REVIEW submission of
// the app on the same platform, or "" when there is none or the read fails.
func firstDraftReviewSubmission(ctx context.Context, client *asc.Client, appID, platform, exclude string) string {
	if appID == "" {
		return ""
	}
	opts := []asc.ReviewSubmissionsOption{asc.WithReviewSubmissionsStates([]string{string(asc.ReviewSubmissionStateReadyForReview)})}
	if platform != "" {
		opts = append(opts, asc.WithReviewSubmissionsPlatforms([]string{platform}))
	}
	drafts, err := client.GetReviewSubmissions(ctx, appID, opts...)
	if err != nil {
		return ""
	}
	for _, draft := range drafts.Data {
		if id := strings.TrimSpace(draft.ID); id != "" && id != exclude {
			return id
		}
	}
	return ""
}

func associatedErrorsMention(apiErr *asc.APIError, text string) bool {
	for _, associated := range apiErr.AssociatedErrors {
		for _, entry := range associated {
			if strings.Contains(strings.ToLower(entry.Detail), text) {
				return true
			}
		}
	}
	return false
}

func valueOrPlaceholder(value, placeholder string) string {
	if value == "" {
		return placeholder
	}
	return value
}
