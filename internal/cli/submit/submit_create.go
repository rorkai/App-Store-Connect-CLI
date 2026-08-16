package submit

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func addVersionToSubmissionOrRecover(
	ctx context.Context,
	client *asc.Client,
	submissionID, versionID, appID, platform string,
	emit func(string),
) (string, error) {
	_, err := client.AddReviewSubmissionItem(ctx, submissionID, versionID)
	if err == nil {
		return submissionID, nil
	}

	conflict := extractSubmissionConflict(err)
	conflictSubmissionID := strings.TrimSpace(conflict.SubmissionID)
	if conflict.Kind != submissionConflictAlreadyAttached || conflictSubmissionID == "" {
		return "", err
	}
	if verifyErr := verifyReviewSubmissionForSubmit(ctx, client, conflictSubmissionID, appID, platform, versionID); verifyErr != nil {
		return "", fmt.Errorf(
			"conflict review submission %s could not be safely reused: %w",
			conflictSubmissionID,
			verifyErr,
		)
	}

	message := fmt.Sprintf("Version already in review submission %s, reusing it.", conflictSubmissionID)
	if emit != nil {
		emit(message)
	} else {
		fmt.Fprintln(os.Stderr, message)
	}
	return conflictSubmissionID, nil
}

func verifyReviewSubmissionForSubmit(
	ctx context.Context,
	client *asc.Client,
	submissionID, appID, platform, versionID string,
) error {
	submissionID = strings.TrimSpace(submissionID)
	appID = strings.TrimSpace(appID)
	platform = strings.ToUpper(strings.TrimSpace(platform))
	versionID = strings.TrimSpace(versionID)
	if submissionID == "" || appID == "" || platform == "" || versionID == "" {
		return fmt.Errorf("submission, app, platform, and version IDs are required")
	}

	refreshed, err := client.GetReviewSubmission(
		ctx,
		submissionID,
		asc.WithReviewSubmissionInclude([]string{"app"}),
	)
	if err != nil {
		return fmt.Errorf("refresh review submission: %w", err)
	}
	if refreshed == nil || strings.TrimSpace(refreshed.Data.ID) != submissionID {
		return fmt.Errorf("app store connect did not return review submission %s", submissionID)
	}
	submission := &refreshed.Data
	if submission.Attributes.SubmissionState != asc.ReviewSubmissionStateReadyForReview {
		return fmt.Errorf(
			"review submission %s is in state %q, not %q",
			submissionID,
			submission.Attributes.SubmissionState,
			asc.ReviewSubmissionStateReadyForReview,
		)
	}
	if !strings.EqualFold(string(submission.Attributes.Platform), platform) {
		return fmt.Errorf(
			"review submission %s is for platform %q, not %q",
			submissionID,
			submission.Attributes.Platform,
			platform,
		)
	}
	if actualAppID := reviewSubmissionAppID(submission); actualAppID != appID {
		if actualAppID == "" {
			return fmt.Errorf("review submission %s did not prove its app relationship", submissionID)
		}
		return fmt.Errorf("review submission %s belongs to app %q, not %q", submissionID, actualAppID, appID)
	}

	summary, err := summarizeReviewSubmissionItems(ctx, client, submissionID, versionID)
	if err != nil {
		return fmt.Errorf("inspect review submission items: %w", err)
	}
	if !summary.hasTargetVersion {
		return fmt.Errorf("review submission %s does not contain target version %s", submissionID, versionID)
	}
	if summary.hasOtherItems {
		return fmt.Errorf("review submission %s contains unrelated review items", submissionID)
	}
	return nil
}

func reviewSubmissionAppID(submission *asc.ReviewSubmissionResource) string {
	if submission == nil || submission.Relationships == nil || submission.Relationships.App == nil {
		return ""
	}
	return strings.TrimSpace(submission.Relationships.App.Data.ID)
}

type submitCreateReviewSubmissionPreparation struct {
	reuseSubmissionID         string
	reuseSubmissionHasVersion bool
}

func prepareReviewSubmissionForCreate(
	ctx context.Context,
	client *asc.Client,
	appID, platform, versionID string,
	emit func(string),
) submitCreateReviewSubmissionPreparation {
	emitMessage := func(format string, args ...any) {
		message := fmt.Sprintf(format, args...)
		if emit != nil {
			emit(message)
			return
		}
		fmt.Fprintln(os.Stderr, message)
	}

	existing, err := client.GetReviewSubmissions(
		ctx,
		appID,
		asc.WithReviewSubmissionsStates([]string{string(asc.ReviewSubmissionStateReadyForReview)}),
		asc.WithReviewSubmissionsPlatforms([]string{platform}),
		asc.WithReviewSubmissionsInclude([]string{"appStoreVersionForReview"}),
		asc.WithReviewSubmissionsLimit(200),
	)
	if err != nil {
		emitMessage("Warning: failed to query stale review submissions: %v", err)
		return submitCreateReviewSubmissionPreparation{}
	}

	submissions := make([]asc.ReviewSubmissionResource, 0, len(existing.Data))
	for {
		submissions = append(submissions, existing.Data...)
		nextURL := strings.TrimSpace(existing.Links.Next)
		if nextURL == "" {
			break
		}
		existing, err = client.GetReviewSubmissions(ctx, appID, asc.WithReviewSubmissionsNextURL(nextURL))
		if err != nil {
			emitMessage("Warning: failed to query stale review submissions: %v", err)
			return submitCreateReviewSubmissionPreparation{}
		}
	}

	if len(submissions) == 0 {
		return submitCreateReviewSubmissionPreparation{}
	}

	result := submitCreateReviewSubmissionPreparation{}
	normalizedPlatform := strings.ToUpper(strings.TrimSpace(platform))
	targetVersionID := strings.TrimSpace(versionID)
	inspector := newReviewSubmissionInspector(client, targetVersionID)

	for i := range submissions {
		sub := submissions[i]
		if sub.Attributes.SubmissionState != asc.ReviewSubmissionStateReadyForReview {
			continue
		}
		if normalizedPlatform != "" && !strings.EqualFold(string(sub.Attributes.Platform), normalizedPlatform) {
			continue
		}
		reusable, hasVersion, reuseErr := inspector.canReuse(ctx, &sub)
		if reuseErr != nil {
			emitMessage(
				"Skipped stale review submission %s: could not confirm which versions it holds (%v). Cancel it explicitly with `asc submit cancel --id %s --confirm` if you intend to replace it.",
				sub.ID,
				reuseErr,
				sub.ID,
			)
			continue
		}
		if reusable {
			if hasVersion {
				emitMessage("Reusing existing review submission %s because the target version is already attached.", sub.ID)
			} else {
				emitMessage("Reusing existing review submission %s because it has no review items.", sub.ID)
			}
			result.reuseSubmissionID = strings.TrimSpace(sub.ID)
			result.reuseSubmissionHasVersion = hasVersion
			return result
		}
		emitMessage(
			"Skipped stale review submission %s because it is not exclusively usable for version %s. Cancel it explicitly with `asc submit cancel --id %s --confirm` if you intend to replace it.",
			sub.ID,
			targetVersionID,
			sub.ID,
		)
	}
	return result
}

func reviewSubmissionAppStoreVersionID(submission *asc.ReviewSubmissionResource) string {
	if submission == nil || submission.Relationships == nil || submission.Relationships.AppStoreVersionForReview == nil {
		return ""
	}
	return strings.TrimSpace(submission.Relationships.AppStoreVersionForReview.Data.ID)
}

type reviewSubmissionItemSummary struct {
	hasItems         bool
	hasTargetVersion bool
	hasOtherItems    bool
}

// reviewSubmissionInspector answers reuse and cancellation questions about the
// app's ready-for-review submissions. Item membership does not change while
// preparation runs, so each submission is inspected at most once per pass.
type reviewSubmissionInspector struct {
	client          *asc.Client
	targetVersionID string
	summaries       map[string]reviewSubmissionItemSummary
	summaryErrors   map[string]error
}

func newReviewSubmissionInspector(client *asc.Client, targetVersionID string) *reviewSubmissionInspector {
	return &reviewSubmissionInspector{
		client:          client,
		targetVersionID: strings.TrimSpace(targetVersionID),
		summaries:       make(map[string]reviewSubmissionItemSummary),
		summaryErrors:   make(map[string]error),
	}
}

func (i *reviewSubmissionInspector) summarize(ctx context.Context, submissionID string) (reviewSubmissionItemSummary, error) {
	submissionID = strings.TrimSpace(submissionID)
	if cached, ok := i.summaries[submissionID]; ok {
		return cached, nil
	}
	if cachedErr, ok := i.summaryErrors[submissionID]; ok {
		return reviewSubmissionItemSummary{}, cachedErr
	}
	summary, err := summarizeReviewSubmissionItems(ctx, i.client, submissionID, i.targetVersionID)
	if err != nil {
		i.summaryErrors[submissionID] = err
		return reviewSubmissionItemSummary{}, err
	}
	i.summaries[submissionID] = summary
	return summary, nil
}

func (i *reviewSubmissionInspector) canReuse(ctx context.Context, submission *asc.ReviewSubmissionResource) (reusable bool, hasVersion bool, err error) {
	if submission == nil {
		return false, false, nil
	}

	submissionID := strings.TrimSpace(submission.ID)
	if submissionID == "" {
		return false, false, nil
	}
	if linkedVersionID := reviewSubmissionAppStoreVersionID(submission); linkedVersionID != "" && linkedVersionID != i.targetVersionID {
		return false, false, nil
	}

	itemSummary, err := i.summarize(ctx, submissionID)
	if err != nil {
		return false, false, err
	}
	if itemSummary.hasItems {
		if itemSummary.hasTargetVersion && !itemSummary.hasOtherItems {
			return true, true, nil
		}
		return false, false, nil
	}

	return true, false, nil
}

func summarizeReviewSubmissionItems(
	ctx context.Context,
	client *asc.Client,
	submissionID, targetVersionID string,
) (reviewSubmissionItemSummary, error) {
	var summary reviewSubmissionItemSummary

	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" || client == nil {
		return summary, nil
	}

	// appStoreVersion must be INCLUDED, not merely named in fields[]. fields[] is a sparse-fieldset
	// selector: it narrows what comes back, and the API answers it with items that carry "links" only
	// and no "relationships" key at all. Every item then reads as an empty version id,
	// hasTargetVersion never becomes true, and a correctly prepared submission is rejected as not
	// containing its version. include= is what makes App Store Connect materialise the linkage;
	// fields[] rides along to keep the item payload narrow.
	resp, err := client.GetReviewSubmissionItems(
		ctx,
		submissionID,
		asc.WithReviewSubmissionItemsInclude([]string{"appStoreVersion"}),
		asc.WithReviewSubmissionItemsFields([]string{"appStoreVersion"}),
		asc.WithReviewSubmissionItemsLimit(200),
	)
	if err != nil {
		return summary, err
	}

	for {
		accumulateReviewSubmissionItemSummary(&summary, resp.Data, targetVersionID)

		nextURL := strings.TrimSpace(resp.Links.Next)
		if nextURL == "" {
			return summary, nil
		}

		resp, err = client.GetReviewSubmissionItems(ctx, submissionID, asc.WithReviewSubmissionItemsNextURL(nextURL))
		if err != nil {
			return summary, err
		}
	}
}

func accumulateReviewSubmissionItemSummary(summary *reviewSubmissionItemSummary, items []asc.ReviewSubmissionItemResource, targetVersionID string) {
	if summary == nil {
		return
	}

	targetVersionID = strings.TrimSpace(targetVersionID)
	for _, item := range items {
		summary.hasItems = true

		versionID := reviewSubmissionItemVersionID(item)
		switch {
		case targetVersionID != "" && versionID == targetVersionID:
			summary.hasTargetVersion = true
		case versionID != "":
			summary.hasOtherItems = true
		default:
			// If the item is not the target version, treat it as unrelated and
			// avoid reusing the submission implicitly.
			summary.hasOtherItems = true
		}
	}
}

func reviewSubmissionItemVersionID(item asc.ReviewSubmissionItemResource) string {
	if item.Relationships == nil || item.Relationships.AppStoreVersion == nil {
		return ""
	}
	return strings.TrimSpace(item.Relationships.AppStoreVersion.Data.ID)
}

func refreshReviewSubmission(ctx context.Context, client *asc.Client, submissionID string) (*asc.ReviewSubmissionResource, error) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" || client == nil {
		return nil, nil
	}
	resp, err := client.GetReviewSubmission(ctx, submissionID)
	if err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func reviewSubmissionIsState(ctx context.Context, client *asc.Client, submissionID string, wantState asc.ReviewSubmissionState) (bool, error) {
	refreshed, err := refreshReviewSubmission(ctx, client, submissionID)
	if err != nil || refreshed == nil {
		return false, err
	}
	return refreshed.Attributes.SubmissionState == wantState, nil
}

func preserveCreatedReviewSubmission(submissionID string, emit func(string)) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return
	}
	message := fmt.Sprintf(
		"Preserved newly created review submission %s instead of canceling it automatically because it may now contain review items. Retry the command if this run reports an error; it will reuse the draft when safe. To cancel it intentionally, run `asc submit cancel --id %s --confirm`.",
		submissionID,
		submissionID,
	)
	if emit != nil {
		emit(message)
	} else {
		fmt.Fprintln(os.Stderr, message)
	}
}
