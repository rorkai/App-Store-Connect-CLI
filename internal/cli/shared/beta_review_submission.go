package shared

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type buildBetaReviewSubmissionClient interface {
	GetBuildBetaAppReviewSubmission(ctx context.Context, buildID string) (*asc.BetaAppReviewSubmissionResponse, error)
	CreateBetaAppReviewSubmission(ctx context.Context, buildID string) (*asc.BetaAppReviewSubmissionResponse, error)
}

// BuildBetaReviewSubmissionResult reports the beta app review submission outcome.
type BuildBetaReviewSubmissionResult struct {
	Submitted    bool
	SubmissionID string
	Message      string
}

// SubmitBuildBetaReviewIfNeeded submits a build for beta app review when requested
// and when the build was added to at least one external beta group.
func SubmitBuildBetaReviewIfNeeded(ctx context.Context, client buildBetaReviewSubmissionClient, buildID string, groups []ResolvedBetaGroup, addedGroupIDs []string, submit bool, operationName string) (*BuildBetaReviewSubmissionResult, error) {
	result := &BuildBetaReviewSubmissionResult{}
	if !submit {
		return result, nil
	}

	if !hasAddedExternalBuildBetaGroup(groups, addedGroupIDs) {
		result.Message = fmt.Sprintf("Skipped beta app review submission for build %s because no external groups were added", buildID)
		return result, nil
	}

	existingSubmission, err := client.GetBuildBetaAppReviewSubmission(ctx, buildID)
	if err == nil {
		submissionID := strings.TrimSpace(existingSubmission.Data.ID)
		result.Submitted = true
		result.SubmissionID = submissionID
		if submissionID == "" {
			result.Message = fmt.Sprintf("Build %s already has a beta app review submission", buildID)
			return result, nil
		}
		result.Message = fmt.Sprintf("Build %s already has beta app review submission %s", buildID, submissionID)
		return result, nil
	}
	if !asc.IsNotFound(err) {
		return nil, fmt.Errorf("%s: failed to inspect beta app review submission: %w", operationName, err)
	}

	submission, err := client.CreateBetaAppReviewSubmission(ctx, buildID)
	if err != nil {
		return nil, fmt.Errorf("%s: beta groups were added to build %q, but beta app review submission failed: %w", operationName, buildID, err)
	}

	submissionID := strings.TrimSpace(submission.Data.ID)
	result.Submitted = true
	result.SubmissionID = submissionID
	if submissionID == "" {
		result.Message = fmt.Sprintf("Submitted build %s for beta app review", buildID)
		return result, nil
	}
	result.Message = fmt.Sprintf("Submitted build %s for beta app review (%s)", buildID, submissionID)
	return result, nil
}

func hasAddedExternalBuildBetaGroup(groups []ResolvedBetaGroup, addedGroupIDs []string) bool {
	if len(groups) == 0 || len(addedGroupIDs) == 0 {
		return false
	}

	for _, group := range groups {
		if group.IsInternalGroup {
			continue
		}
		if slices.Contains(addedGroupIDs, group.ID) {
			return true
		}
	}

	return false
}
