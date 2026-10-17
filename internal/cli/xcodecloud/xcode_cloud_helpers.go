package xcodecloud

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// waitForBuildCompletion polls until the build run completes or times out.
func waitForBuildCompletion(ctx context.Context, client *asc.Client, buildRunID string, pollInterval time.Duration, outputFormat string, pretty bool) error {
	lastStatus := "unknown"
	_, err := asc.PollUntil(ctx, pollInterval, func(ctx context.Context) (struct{}, bool, error) {
		resp, err := getCiBuildRun(ctx, client, buildRunID)
		if err != nil {
			return struct{}{}, false, fmt.Errorf("xcode-cloud: failed to check status: %w", err)
		}
		lastStatus = string(resp.Data.Attributes.ExecutionProgress)

		if asc.IsBuildRunComplete(resp.Data.Attributes.ExecutionProgress) {
			result := buildStatusResult(resp)
			if err := shared.PrintOutput(result, outputFormat, pretty); err != nil {
				return struct{}{}, false, err
			}

			// Return error for failed builds
			if !asc.IsBuildRunSuccessful(resp.Data.Attributes.CompletionStatus) {
				return struct{}{}, false, fmt.Errorf("build run %s completed with status: %s", buildRunID, resp.Data.Attributes.CompletionStatus)
			}
			return struct{}{}, true, nil
		}
		return struct{}{}, false, nil
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return fmt.Errorf("xcode-cloud: canceled waiting for build run %s (last status: %s): %w", buildRunID, lastStatus, err)
		}
		if ctx.Err() != nil && errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("xcode-cloud: timed out waiting for build run %s (last status: %s); rerun with a longer --timeout: %w", buildRunID, lastStatus, err)
		}
		return err
	}

	return nil
}

// classifyXcodeCloudLookupError marks expected name and product lookup
// misses as validation failures on the flag the caller can change. The
// message and exit code are unchanged.
func classifyXcodeCloudLookupError(err error) error {
	switch {
	case errors.Is(err, asc.ErrCiProductNotFound):
		// The app exists but has no Xcode Cloud product until Xcode Cloud is
		// set up for it, which is a normal state rather than an asc defect.
		return shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticStateNotReady, "--app")
	case errors.Is(err, asc.ErrCiWorkflowNotFound):
		return shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticResourceNotFound, "--workflow")
	case errors.Is(err, asc.ErrCiWorkflowAmbiguous):
		return shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticInvalidInput, "--workflow-id")
	case errors.Is(err, asc.ErrGitReferenceNotFound):
		return shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticResourceNotFound, "--branch")
	case errors.Is(err, asc.ErrGitReferenceAmbiguous):
		return shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticInvalidInput, "--git-reference-id")
	default:
		return err
	}
}

// buildStatusResult converts a CiBuildRunResponse to XcodeCloudStatusResult.
func buildStatusResult(resp *asc.CiBuildRunResponse) *asc.XcodeCloudStatusResult {
	result := &asc.XcodeCloudStatusResult{
		BuildRunID:        resp.Data.ID,
		BuildNumber:       resp.Data.Attributes.Number,
		ExecutionProgress: string(resp.Data.Attributes.ExecutionProgress),
		CompletionStatus:  string(resp.Data.Attributes.CompletionStatus),
		StartReason:       resp.Data.Attributes.StartReason,
		CancelReason:      resp.Data.Attributes.CancelReason,
		CreatedDate:       resp.Data.Attributes.CreatedDate,
		StartedDate:       resp.Data.Attributes.StartedDate,
		FinishedDate:      resp.Data.Attributes.FinishedDate,
		SourceCommit:      resp.Data.Attributes.SourceCommit,
		IssueCounts:       resp.Data.Attributes.IssueCounts,
	}

	if resp.Data.Relationships != nil && resp.Data.Relationships.Workflow != nil {
		result.WorkflowID = resp.Data.Relationships.Workflow.Data.ID
	}

	return result
}

const defaultXcodeCloudTimeout = 30 * time.Minute

func contextWithXcodeCloudTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		timeout = asc.ResolveTimeoutWithDefault(defaultXcodeCloudTimeout)
	}
	return context.WithTimeout(ctx, timeout)
}

func getCiBuildRun(ctx context.Context, client *asc.Client, buildRunID string, opts ...asc.CiBuildRunGetOption) (*asc.CiBuildRunResponse, error) {
	return client.GetCiBuildRun(ctx, buildRunID, opts...)
}
