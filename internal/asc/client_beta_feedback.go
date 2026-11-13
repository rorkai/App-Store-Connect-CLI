package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GetBetaCrashLog retrieves a beta crash log by ID.
func (c *Client) GetBetaCrashLog(ctx context.Context, logID string) (*BetaCrashLogResponse, error) {
	logID = strings.TrimSpace(logID)
	if logID == "" {
		return nil, fmt.Errorf("logID is required")
	}

	path := fmt.Sprintf("/v1/betaCrashLogs/%s", logID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaCrashLogResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBetaFeedbackCrashSubmission retrieves a beta feedback crash submission by ID.
func (c *Client) GetBetaFeedbackCrashSubmission(ctx context.Context, submissionID string) (*BetaFeedbackCrashSubmissionResponse, error) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return nil, fmt.Errorf("submissionID is required")
	}

	path := fmt.Sprintf("/v1/betaFeedbackCrashSubmissions/%s", submissionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaFeedbackCrashSubmissionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteBetaFeedbackCrashSubmission deletes a beta feedback crash submission by ID.
func (c *Client) DeleteBetaFeedbackCrashSubmission(ctx context.Context, submissionID string) error {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return fmt.Errorf("submissionID is required")
	}

	path := fmt.Sprintf("/v1/betaFeedbackCrashSubmissions/%s", submissionID)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// GetBetaFeedbackCrashSubmissionCrashLog retrieves the crash log for a beta feedback crash submission.
func (c *Client) GetBetaFeedbackCrashSubmissionCrashLog(ctx context.Context, submissionID string) (*BetaCrashLogResponse, error) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return nil, fmt.Errorf("submissionID is required")
	}

	path := fmt.Sprintf("/v1/betaFeedbackCrashSubmissions/%s/crashLog", submissionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaCrashLogResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBetaFeedbackScreenshotSubmission retrieves a beta feedback screenshot submission by ID.
func (c *Client) GetBetaFeedbackScreenshotSubmission(ctx context.Context, submissionID string) (*BetaFeedbackScreenshotSubmissionResponse, error) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return nil, fmt.Errorf("submissionID is required")
	}

	path := fmt.Sprintf("/v1/betaFeedbackScreenshotSubmissions/%s", submissionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaFeedbackScreenshotSubmissionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteBetaFeedbackScreenshotSubmission deletes a beta feedback screenshot submission by ID.
func (c *Client) DeleteBetaFeedbackScreenshotSubmission(ctx context.Context, submissionID string) error {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" {
		return fmt.Errorf("submissionID is required")
	}

	path := fmt.Sprintf("/v1/betaFeedbackScreenshotSubmissions/%s", submissionID)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}
