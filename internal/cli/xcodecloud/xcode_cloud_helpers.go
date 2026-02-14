package xcodecloud

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// waitForBuildCompletion polls until the build run completes or times out.
func waitForBuildCompletion(ctx context.Context, client *asc.Client, buildRunID string, pollInterval time.Duration, outputFormat string, pretty bool) error {
	lastStatus := "unknown"
	_, err := asc.PollUntil(ctx, pollInterval, func(ctx context.Context) (struct{}, bool, error) {
		resp, err := getCiBuildRunWithRetry(ctx, client, buildRunID)
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
			return fmt.Errorf("xcode-cloud: canceled waiting for build run %s (last status: %s)", buildRunID, lastStatus)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("xcode-cloud: timed out waiting for build run %s (last status: %s)", buildRunID, lastStatus)
		}
		return err
	}

	return nil
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

func getCiBuildRunWithRetry(ctx context.Context, client *asc.Client, buildRunID string) (*asc.CiBuildRunResponse, error) {
	retryOpts := asc.ResolveRetryOptions()
	return asc.WithRetry(ctx, func() (*asc.CiBuildRunResponse, error) {
		resp, err := client.GetCiBuildRun(ctx, buildRunID)
		if err != nil {
			if isTransientNetworkError(err) {
				return nil, &asc.RetryableError{Err: err}
			}
			return nil, err
		}
		return resp, nil
	}, retryOpts)
}

func isTransientNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}
	if _, ok := errors.AsType[net.Error](err); ok {
		return true
	}
	return errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNREFUSED)
}
