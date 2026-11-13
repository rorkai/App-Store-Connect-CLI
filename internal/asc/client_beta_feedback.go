package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetBetaCrashLog retrieves a beta crash log by ID.
func (c *Client) GetBetaCrashLog(ctx context.Context, logID string) (*BetaCrashLogResponse, error) {
	logID = strings.TrimSpace(logID)
	if logID == "" {
		return nil, fmt.Errorf("logID is required")
	}

	return getJSON[BetaCrashLogResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/betaCrashLogs/%s", logID)})
}

// GetBetaFeedbackCrashSubmission retrieves a beta feedback crash submission by ID.
func (c *Client) GetBetaFeedbackCrashSubmission(ctx context.Context, submissionID string) (*BetaFeedbackCrashSubmissionResponse, error) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return nil, fmt.Errorf("submissionID is required")
	}

	return getJSON[BetaFeedbackCrashSubmissionResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/betaFeedbackCrashSubmissions/%s", submissionID)})
}

// DeleteBetaFeedbackCrashSubmission deletes a beta feedback crash submission by ID.
func (c *Client) DeleteBetaFeedbackCrashSubmission(ctx context.Context, submissionID string) error {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return fmt.Errorf("submissionID is required")
	}

	return sendNoContent(ctx, c, "DELETE", fmt.Sprintf("/v1/betaFeedbackCrashSubmissions/%s", submissionID), nil)
}

// GetBetaFeedbackCrashSubmissionCrashLog retrieves the crash log for a beta feedback crash submission.
func (c *Client) GetBetaFeedbackCrashSubmissionCrashLog(ctx context.Context, submissionID string) (*BetaCrashLogResponse, error) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return nil, fmt.Errorf("submissionID is required")
	}

	return getJSON[BetaCrashLogResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/betaFeedbackCrashSubmissions/%s/crashLog", submissionID)})
}

// GetBetaFeedbackScreenshotSubmission retrieves a beta feedback screenshot submission by ID.
func (c *Client) GetBetaFeedbackScreenshotSubmission(ctx context.Context, submissionID string) (*BetaFeedbackScreenshotSubmissionResponse, error) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return nil, fmt.Errorf("submissionID is required")
	}

	return getJSON[BetaFeedbackScreenshotSubmissionResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/betaFeedbackScreenshotSubmissions/%s", submissionID)})
}

// DeleteBetaFeedbackScreenshotSubmission deletes a beta feedback screenshot submission by ID.
func (c *Client) DeleteBetaFeedbackScreenshotSubmission(ctx context.Context, submissionID string) error {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return fmt.Errorf("submissionID is required")
	}

	return sendNoContent(ctx, c, "DELETE", fmt.Sprintf("/v1/betaFeedbackScreenshotSubmissions/%s", submissionID), nil)
}
