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
	var hint string
	if associatedErrorsMention(apiErr, reviewItemEmbeddedAssetText) {
		// Fixing the errors and adding the asset again cannot work, so this
		// replaces the generic advice; the additions below still apply.
		hint = reviewItemEmbeddedAssetHint
	} else {
		hint = "App Store Connect cannot review this item yet: fix the associated errors listed above, then add it again."
		if itemType == asc.ReviewSubmissionItemTypeAppStoreVersion {
			hint += fmt.Sprintf(" `asc validate --app APP_ID --version-id %s` lists what the version is missing.", itemID)
		}
	}
	for _, resource := range slices.Sorted(maps.Keys(apiErr.AssociatedErrors)) {
		for _, entry := range apiErr.AssociatedErrors[resource] {
			if match := reviewItemOnAnotherSubmissionPattern.FindStringSubmatch(entry.Detail); match != nil {
				hint += fmt.Sprintf(" The item is already on submission %s: inspect it with `asc review submissions-get --id %s --include items`, and submit that one or remove the item from it with `asc review items remove`.", match[1], match[1])
			}
		}
	}
	if associatedErrorsMention(apiErr, reviewComplianceRequirementText) {
		hint += " " + reviewComplianceRequirementHint
	}
	return hint
}

// reviewItemEmbeddedAssetText is in the associated error Apple lists for an
// Asset Library image that is locked to another review submission: "Asset is
// locked by an embedded review submission." It was reported for images placed
// on a version that went into review; submitting that version took them with
// it. There is nothing to fix on the asset, and adding it again cannot work.
const reviewItemEmbeddedAssetText = "locked by an embedded review submission"

const reviewItemEmbeddedAssetHint = "App Store Connect locks an asset that is placed on an app version (`asc localizations placements create`) while that version is in, or goes into, a review submission; the asset is then reviewed with the version, not as its own item. Submit the version with `asc review submit --app APP_ID --version-id VERSION_ID --build-id BUILD_ID --confirm`, and the assets placed on it go with it."

// reviewComplianceRequirementText is in the associated error Apple lists on
// the app's appInfos, once per open requirement: "Cannot submit version due to
// unfulfilled compliance requirement."
const reviewComplianceRequirementText = "compliance requirement"

// reviewComplianceRequirementHint names the reads for both kinds of
// requirement App Store Connect can hold a submission on; they live on two
// separate web-session services and neither appears in the public API.
const reviewComplianceRequirementHint = "App Store Connect does not review the app until every compliance requirement has an answer. List the App Compliance requirements, such as the China mainland Security Assessment and AI Generated or Synthetic Content declarations, with `asc web apps compliance list --app APP_ID` and answer them with `asc web apps compliance set`; list the App Store Regulations & Permits declarations with `asc web apps declarations list --app APP_ID`."

// withComplianceRequirementHint names the compliance reads when a 409 on
// resolving an item or submitting a submission is blocked by an unanswered
// compliance requirement; any other error, or one that already carries a
// hint, is returned unchanged.
func withComplianceRequirementHint(err error) error {
	if _, hinted := errors.AsType[*shared.HintedError](err); hinted {
		return err
	}
	apiErr, ok := errors.AsType[*asc.APIError](err)
	if !ok || apiErr.StatusCode != http.StatusConflict || !associatedErrorsMention(apiErr, reviewComplianceRequirementText) {
		return err
	}
	return &shared.HintedError{Err: err, Hint: reviewComplianceRequirementHint}
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
