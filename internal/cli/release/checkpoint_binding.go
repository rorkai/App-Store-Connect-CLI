package release

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// verifyResumedCheckpointBinding re-establishes a checkpoint's app, version,
// build, and submission binding from authenticated API state.
//
// The checkpoint file is unsigned local JSON, so matching user-facing arguments
// proves nothing about the IDs and completed-step flags stored alongside them. A
// stored ID that cannot be tied back to the selected app, version string, and
// platform aborts the run. A completed mutation step that current API state
// contradicts is discarded so the step runs again against the verified target
// instead of being reported as done.
func verifyResumedCheckpointBinding(
	ctx context.Context,
	client *asc.Client,
	opts runOptions,
	checkpoint *runCheckpoint,
	emit func(string),
) error {
	if checkpoint == nil {
		return nil
	}
	emitMessage := func(format string, args ...any) {
		message := fmt.Sprintf(format, args...)
		if emit != nil {
			emit(message)
			return
		}
		fmt.Fprintln(os.Stderr, message)
	}

	for name, completed := range checkpoint.Completed {
		if !completed {
			delete(checkpoint.Completed, name)
			continue
		}
		if name == stepSubmitReview && !opts.SubmitForReview {
			return fmt.Errorf("checkpoint completed step %q requires resuming with --submit-for-review", name)
		}
		if !isReleasePipelineStep(name, opts.SubmitForReview, strings.TrimSpace(opts.RoutingCoverageFile) != "") {
			return fmt.Errorf("checkpoint records unknown completed step %q", name)
		}
	}

	if !checkpoint.Completed[stepSubmitReview] && checkpoint.SubmissionID != "" {
		checkpoint.SubmissionID = ""
		emitMessage("Rechecking %s: a submission ID is trusted only with a completed and verified submit step.", stepSubmitReview)
	}

	versionID := strings.TrimSpace(checkpoint.VersionID)
	if versionID == "" {
		if len(checkpoint.Completed) > 0 {
			return fmt.Errorf("checkpoint reports completed steps without a version ID to verify them against")
		}
		return nil
	}

	version, err := shared.ResolveOwnedAppStoreVersionByID(ctx, client, opts.AppID, versionID, opts.Platform)
	if err != nil {
		return fmt.Errorf("checkpoint version %s could not be verified: %w", versionID, err)
	}
	if resolved := strings.TrimSpace(version.Attributes.VersionString); !strings.EqualFold(resolved, strings.TrimSpace(opts.Version)) {
		return fmt.Errorf("checkpoint version %s is version %q, not %q", versionID, resolved, strings.TrimSpace(opts.Version))
	}

	// These completions cannot be authenticated from current remote state.
	// Local inputs may have changed since the checkpoint was written, and
	// readiness is a point-in-time observation rather than a durable server
	// state. An unsigned checkpoint must never be able to suppress these steps.
	if checkpoint.Completed[stepApplyMetadata] {
		delete(checkpoint.Completed, stepApplyMetadata)
		emitMessage("Rechecking %s: an unsigned checkpoint cannot prove the current metadata input was applied.", stepApplyMetadata)
	}
	if checkpoint.Completed[stepApplyRoutingCoverage] {
		delete(checkpoint.Completed, stepApplyRoutingCoverage)
		emitMessage("Rechecking %s: an unsigned checkpoint cannot prove the current routing coverage file was applied.", stepApplyRoutingCoverage)
	}
	if checkpoint.Completed[stepValidateReadiness] {
		delete(checkpoint.Completed, stepValidateReadiness)
		emitMessage("Rechecking %s: readiness must be evaluated again against current App Store Connect state.", stepValidateReadiness)
	}

	if checkpoint.Completed[stepAttachBuild] {
		attachedBuildID, buildErr := attachedAppStoreVersionBuildID(ctx, client, versionID)
		attachDiscarded := false
		switch {
		case buildErr != nil:
			attachDiscarded = true
			delete(checkpoint.Completed, stepAttachBuild)
			emitMessage("Rechecking %s: could not confirm the build attached to version %s (%v).", stepAttachBuild, versionID, buildErr)
		case attachedBuildID != strings.TrimSpace(opts.BuildID):
			attachDiscarded = true
			delete(checkpoint.Completed, stepAttachBuild)
			emitMessage(
				"Rechecking %s: version %s currently has build %q attached, not %q.",
				stepAttachBuild,
				versionID,
				attachedBuildID,
				strings.TrimSpace(opts.BuildID),
			)
		}
		// Readiness was validated against whatever build was attached at the
		// time, so an unproven attachment invalidates that result as well.
		if attachDiscarded && checkpoint.Completed[stepValidateReadiness] {
			delete(checkpoint.Completed, stepValidateReadiness)
			emitMessage("Rechecking %s: it depends on %s, which could not be confirmed.", stepValidateReadiness, stepAttachBuild)
		}
	}

	// A real run completes the pipeline in order and persists each step before
	// the next one starts, so a completed validate_readiness always follows
	// completed prerequisites. An unsigned checkpoint can claim otherwise: the
	// pipeline would then apply the missing mutation and skip readiness, leaving
	// the version unvalidated against the state that mutation produced.
	if checkpoint.Completed[stepValidateReadiness] {
		prerequisites := []string{stepEnsureVersion, stepApplyMetadata, stepAttachBuild}
		if strings.TrimSpace(opts.RoutingCoverageFile) != "" {
			prerequisites = append(prerequisites, stepApplyRoutingCoverage)
		}
		for _, prerequisite := range prerequisites {
			if checkpoint.Completed[prerequisite] {
				continue
			}
			delete(checkpoint.Completed, stepValidateReadiness)
			emitMessage(
				"Rechecking %s: prerequisite step %s is not complete, so readiness must run again after it does.",
				stepValidateReadiness,
				prerequisite,
			)
			break
		}
	}

	if checkpoint.Completed[stepSubmitReview] {
		submissionID := strings.TrimSpace(checkpoint.SubmissionID)
		if submissionID == "" {
			delete(checkpoint.Completed, stepSubmitReview)
			checkpoint.SubmissionID = ""
			emitMessage("Rechecking %s: the checkpoint reports a submission without recording its ID.", stepSubmitReview)
		} else {
			bound, submissionState, submissionErr := reviewSubmissionBoundToVersion(ctx, client, submissionID, versionID)
			switch {
			case submissionErr != nil && asc.IsNotFound(submissionErr):
				// The recorded ID may come from the legacy
				// appStoreVersionSubmissions flow, which the modern endpoint
				// reports as missing. Verify it against the legacy per-version
				// relationship before treating the completion as disproven.
				legacyProven, legacyErr := legacySubmissionMatches(ctx, client, versionID, submissionID)
				switch {
				case legacyErr != nil && asc.IsNotFound(legacyErr):
					delete(checkpoint.Completed, stepSubmitReview)
					checkpoint.SubmissionID = ""
					emitMessage("Rechecking %s: review submission %s no longer exists.", stepSubmitReview, submissionID)
				case legacyErr != nil:
					return fmt.Errorf("checkpoint completed step %q could not be verified for submission %s: %w", stepSubmitReview, submissionID, legacyErr)
				case !legacyProven:
					delete(checkpoint.Completed, stepSubmitReview)
					checkpoint.SubmissionID = ""
					emitMessage("Rechecking %s: review submission %s no longer exists.", stepSubmitReview, submissionID)
				}
			case submissionErr != nil:
				// An indeterminate read must not discard the completion:
				// re-running submit_review is not idempotent and could create
				// a second submission. Preserve the checkpoint and stop.
				return fmt.Errorf("checkpoint completed step %q could not be verified for submission %s: %w", stepSubmitReview, submissionID, submissionErr)
			case !bound:
				delete(checkpoint.Completed, stepSubmitReview)
				checkpoint.SubmissionID = ""
				emitMessage(
					"Rechecking %s: review submission %s is not bound to version %q.",
					stepSubmitReview,
					submissionID,
					versionID,
				)
			case !reviewSubmissionStateProvesSubmission(submissionState):
				delete(checkpoint.Completed, stepSubmitReview)
				checkpoint.SubmissionID = ""
				emitMessage(
					"Rechecking %s: review submission %s is in state %q, which does not prove it was submitted.",
					stepSubmitReview,
					submissionID,
					string(submissionState),
				)
			}
		}
	}

	return nil
}

func isReleasePipelineStep(name string, submitForReview, hasRoutingCoverage bool) bool {
	switch name {
	case stepEnsureVersion, stepApplyMetadata, stepAttachBuild, stepValidateReadiness:
		return true
	case stepApplyRoutingCoverage:
		return hasRoutingCoverage
	case stepSubmitReview:
		return submitForReview
	default:
		return false
	}
}

func attachedAppStoreVersionBuildID(ctx context.Context, client *asc.Client, versionID string) (string, error) {
	resp, err := client.GetAppStoreVersionBuild(ctx, versionID)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("empty build response for version %s", versionID)
	}
	return strings.TrimSpace(resp.Data.ID), nil
}

// reviewSubmissionBoundToVersion reports whether the submission is bound to
// the given version, using the appStoreVersionForReview linkage when present
// and scanning every item page for the version when the linkage is absent (a
// plain GET may omit it).
func reviewSubmissionBoundToVersion(ctx context.Context, client *asc.Client, submissionID, versionID string) (bool, asc.ReviewSubmissionState, error) {
	resp, err := client.GetReviewSubmission(ctx, submissionID)
	if err != nil {
		return false, "", err
	}
	if resp == nil {
		return false, "", nil
	}
	state := resp.Data.Attributes.SubmissionState
	if resp.Data.Relationships != nil && resp.Data.Relationships.AppStoreVersionForReview != nil {
		if linked := strings.TrimSpace(resp.Data.Relationships.AppStoreVersionForReview.Data.ID); linked != "" {
			return linked == versionID, state, nil
		}
	}

	included, err := reviewSubmissionItemsIncludeVersion(ctx, client, submissionID, versionID)
	if err != nil {
		return false, state, err
	}
	return included, state, nil
}

func reviewSubmissionItemsIncludeVersion(ctx context.Context, client *asc.Client, submissionID, versionID string) (bool, error) {
	resp, err := client.GetReviewSubmissionItems(ctx, submissionID, asc.WithReviewSubmissionItemsLimit(200))
	if err != nil {
		return false, err
	}

	for {
		for _, item := range resp.Data {
			if item.Relationships == nil || item.Relationships.AppStoreVersion == nil {
				continue
			}
			if strings.TrimSpace(item.Relationships.AppStoreVersion.Data.ID) == versionID {
				return true, nil
			}
		}
		nextURL := strings.TrimSpace(resp.Links.Next)
		if nextURL == "" {
			return false, nil
		}
		resp, err = client.GetReviewSubmissionItems(ctx, submissionID, asc.WithReviewSubmissionItemsNextURL(nextURL))
		if err != nil {
			return false, err
		}
	}
}

// legacySubmissionMatches reports whether the version's legacy
// appStoreVersionSubmission exists and carries the recorded submission ID.
func legacySubmissionMatches(ctx context.Context, client *asc.Client, versionID, submissionID string) (bool, error) {
	resp, err := client.GetAppStoreVersionSubmissionForVersion(ctx, versionID)
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}
	return strings.TrimSpace(resp.Data.ID) == submissionID, nil
}

// reviewSubmissionStateProvesSubmission reports whether a fetched submission
// state proves the submission was actually sent to App Review. A draft
// (READY_FOR_REVIEW) or a withdrawal (CANCELING) contradicts a completed
// submit_review step, so the step must run again.
func reviewSubmissionStateProvesSubmission(state asc.ReviewSubmissionState) bool {
	switch state {
	case asc.ReviewSubmissionStateWaitingForReview,
		asc.ReviewSubmissionStateInReview,
		asc.ReviewSubmissionStateUnresolvedIssues,
		asc.ReviewSubmissionStateCompleting,
		asc.ReviewSubmissionStateComplete:
		return true
	default:
		return false
	}
}
