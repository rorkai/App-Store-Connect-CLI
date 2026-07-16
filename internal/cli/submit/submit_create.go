package submit

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

var submitCreateRecentlyCanceledRetryDelays = []time.Duration{
	250 * time.Millisecond,
	500 * time.Millisecond,
	time.Second,
	2 * time.Second,
}

func addVersionToSubmissionOrRecover(
	ctx context.Context,
	client *asc.Client,
	submissionID, versionID string,
	recentlyCanceledSubmissionIDs map[string]struct{},
	emit func(string),
) (string, error) {
	for attempt := 0; ; attempt++ {
		_, err := client.AddReviewSubmissionItem(ctx, submissionID, versionID)
		if err == nil {
			return submissionID, nil
		}

		conflict := extractSubmissionConflict(err)
		conflictSubmissionID := strings.TrimSpace(conflict.SubmissionID)
		if conflictSubmissionID == "" {
			return "", err
		}

		switch conflict.Kind {
		case submissionConflictAlreadyAttached:
			if _, ok := recentlyCanceledSubmissionIDs[conflictSubmissionID]; !ok {
				message := fmt.Sprintf("Version already in review submission %s, reusing it.", conflictSubmissionID)
				if emit != nil {
					emit(message)
				} else {
					fmt.Fprintln(os.Stderr, message)
				}
				return conflictSubmissionID, nil
			}
		case submissionConflictStillInProgress:
			if _, ok := recentlyCanceledSubmissionIDs[conflictSubmissionID]; !ok {
				return "", err
			}
		default:
			return "", err
		}
		if attempt >= len(submitCreateRecentlyCanceledRetryDelays) {
			return "", fmt.Errorf(
				"version is still attached to recently canceled review submission %s after %d retries: %w",
				conflictSubmissionID,
				len(submitCreateRecentlyCanceledRetryDelays),
				err,
			)
		}

		delay := submitCreateRecentlyCanceledRetryDelays[attempt]
		message := fmt.Sprintf(
			"Version is still detaching from recently canceled review submission %s, retrying add in %s.",
			conflictSubmissionID,
			delay,
		)
		if emit != nil {
			emit(message)
		} else {
			fmt.Fprintln(os.Stderr, message)
		}
		if err := sleepWithContext(ctx, delay); err != nil {
			return "", fmt.Errorf("waiting for recently canceled review submission %s to clear: %w", conflictSubmissionID, err)
		}
	}
}

type submitCreateReviewSubmissionPreparation struct {
	reuseSubmissionID         string
	reuseSubmissionHasVersion bool
	canceledSubmissionIDs     map[string]struct{}
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

	result := submitCreateReviewSubmissionPreparation{
		canceledSubmissionIDs: make(map[string]struct{}, len(submissions)),
	}
	normalizedPlatform := strings.ToUpper(strings.TrimSpace(platform))
	targetVersionID := strings.TrimSpace(versionID)

	for i := range submissions {
		sub := submissions[i]
		if sub.Attributes.SubmissionState != asc.ReviewSubmissionStateReadyForReview {
			continue
		}
		if normalizedPlatform != "" && !strings.EqualFold(string(sub.Attributes.Platform), normalizedPlatform) {
			continue
		}
		if currentVersionID := reviewSubmissionAppStoreVersionID(&sub); targetVersionID != "" && currentVersionID == targetVersionID {
			reusable, hasVersion, reuseErr := reviewSubmissionCanBeReusedForCreate(ctx, client, &sub, targetVersionID)
			if reuseErr != nil {
				emitMessage("Warning: failed to inspect review submission %s before reuse: %v", sub.ID, reuseErr)
				continue
			}
			if reusable {
				emitMessage("Reusing existing review submission %s because the target version is already attached.", sub.ID)
				result.reuseSubmissionID = strings.TrimSpace(sub.ID)
				result.reuseSubmissionHasVersion = hasVersion
				result.canceledSubmissionIDs = nil
				return result
			}
		}
	}

	for i := range submissions {
		sub := submissions[i]
		if sub.Attributes.SubmissionState != asc.ReviewSubmissionStateReadyForReview {
			continue
		}
		if normalizedPlatform != "" && !strings.EqualFold(string(sub.Attributes.Platform), normalizedPlatform) {
			continue
		}

		if _, cancelErr := client.CancelReviewSubmission(ctx, sub.ID); cancelErr != nil {
			if isExpectedNonCancellableReviewSubmissionError(cancelErr) {
				reuseSubmission, reuseHasVersion, reuseErr := reusableReviewSubmissionForCreate(ctx, client, &sub, targetVersionID)
				if reuseErr == nil && reuseSubmission != "" {
					if reuseHasVersion {
						emitMessage("Reusing existing review submission %s because the target version is already attached and App Store Connect would not cancel it.", reuseSubmission)
					} else {
						emitMessage("Reusing existing empty review submission %s because App Store Connect would not cancel it.", reuseSubmission)
					}
					result.reuseSubmissionID = reuseSubmission
					result.reuseSubmissionHasVersion = reuseHasVersion
					if len(result.canceledSubmissionIDs) == 0 {
						result.canceledSubmissionIDs = nil
					}
					return result
				}
				if reuseErr != nil {
					emitMessage("Warning: failed to inspect stale submission %s after cancel conflict: %v", sub.ID, reuseErr)
				}
				emitMessage("Skipped stale submission %s: already transitioned to a non-cancellable state", sub.ID)
			} else {
				emitMessage("Warning: failed to cancel stale submission %s: %v", sub.ID, cancelErr)
			}
			continue
		}
		result.canceledSubmissionIDs[sub.ID] = struct{}{}
		emitMessage("Canceled stale review submission %s", sub.ID)
	}

	if len(result.canceledSubmissionIDs) == 0 {
		result.canceledSubmissionIDs = nil
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

func reviewSubmissionCanBeReusedForCreate(
	ctx context.Context,
	client *asc.Client,
	submission *asc.ReviewSubmissionResource,
	targetVersionID string,
) (reusable bool, hasVersion bool, err error) {
	if submission == nil {
		return false, false, nil
	}

	submissionID := strings.TrimSpace(submission.ID)
	if submissionID == "" {
		return false, false, nil
	}

	itemSummary, err := summarizeReviewSubmissionItems(ctx, client, submissionID, targetVersionID)
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

func reusableReviewSubmissionForCreate(
	ctx context.Context,
	client *asc.Client,
	submission *asc.ReviewSubmissionResource,
	targetVersionID string,
) (submissionID string, hasVersion bool, err error) {
	if submission == nil {
		return "", false, nil
	}

	submissionID = strings.TrimSpace(submission.ID)
	if submissionID == "" {
		return "", false, nil
	}
	refreshed, err := refreshReviewSubmission(ctx, client, submissionID)
	if err != nil {
		return "", false, err
	}
	if refreshed == nil || refreshed.Attributes.SubmissionState != asc.ReviewSubmissionStateReadyForReview {
		return "", false, nil
	}

	reusable, hasVersion, err := reviewSubmissionCanBeReusedForCreate(ctx, client, refreshed, targetVersionID)
	if err != nil {
		return "", false, err
	}
	if reusable {
		return submissionID, hasVersion, nil
	}
	return "", false, nil
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

	resp, err := client.GetReviewSubmissionItems(ctx, submissionID, asc.WithReviewSubmissionItemsLimit(200))
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

func cleanupEmptyReviewSubmission(ctx context.Context, client *asc.Client, submissionID string, emit func(string)) {
	if strings.TrimSpace(submissionID) == "" {
		return
	}
	if _, cancelErr := client.CancelReviewSubmission(ctx, submissionID); cancelErr != nil && !isExpectedNonCancellableReviewSubmissionError(cancelErr) {
		message := fmt.Sprintf("Warning: failed to cancel empty submission %s: %v", submissionID, cancelErr)
		if emit != nil {
			emit(message)
		} else {
			fmt.Fprintln(os.Stderr, message)
		}
	}
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
