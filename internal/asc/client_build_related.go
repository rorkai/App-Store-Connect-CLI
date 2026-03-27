package asc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// MissingBuildBetaAppReviewSubmissionError indicates that ASC returned a null
// beta review submission relationship for an otherwise valid build.
type MissingBuildBetaAppReviewSubmissionError struct {
	BuildID string
}

func (e MissingBuildBetaAppReviewSubmissionError) Error() string {
	buildID := strings.TrimSpace(e.BuildID)
	if buildID == "" {
		return "beta app review submission not found"
	}
	return fmt.Sprintf("beta app review submission not found for build %q", buildID)
}

func (e MissingBuildBetaAppReviewSubmissionError) Unwrap() error {
	return ErrNotFound
}

// GetBuildApp retrieves the app for a build.
func (c *Client) GetBuildApp(ctx context.Context, buildID string) (*AppResponse, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return nil, fmt.Errorf("buildID is required")
	}

	path := fmt.Sprintf("/v1/builds/%s/app", buildID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBuildBetaAppReviewSubmission retrieves the beta app review submission for a build.
func (c *Client) GetBuildBetaAppReviewSubmission(ctx context.Context, buildID string) (*BetaAppReviewSubmissionResponse, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return nil, fmt.Errorf("buildID is required")
	}

	path := fmt.Sprintf("/v1/builds/%s/betaAppReviewSubmission", buildID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	// ASC returns HTTP 200 with {"data":null} when a build has no beta review submission yet.
	if bytes.Equal(bytes.TrimSpace(envelope.Data), []byte("null")) {
		return nil, MissingBuildBetaAppReviewSubmissionError{BuildID: buildID}
	}

	var response BetaAppReviewSubmissionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBuildBuildBetaDetail retrieves the build beta detail for a build.
func (c *Client) GetBuildBuildBetaDetail(ctx context.Context, buildID string) (*BuildBetaDetailResponse, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return nil, fmt.Errorf("buildID is required")
	}

	path := fmt.Sprintf("/v1/builds/%s/buildBetaDetail", buildID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BuildBetaDetailResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBuildIcons retrieves build icons for a build.
func (c *Client) GetBuildIcons(ctx context.Context, buildID string, opts ...BuildIconsOption) (*BuildIconsResponse, error) {
	query := &listQuery{}
	for _, opt := range opts {
		opt(query)
	}

	buildID = strings.TrimSpace(buildID)
	if query.nextURL == "" && buildID == "" {
		return nil, fmt.Errorf("buildID is required")
	}

	path := fmt.Sprintf("/v1/builds/%s/icons", buildID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("buildIcons: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildListQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BuildIconsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBuildPreReleaseVersion retrieves the pre-release version for a build.
func (c *Client) GetBuildPreReleaseVersion(ctx context.Context, buildID string) (*PreReleaseVersionResponse, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return nil, fmt.Errorf("buildID is required")
	}

	path := fmt.Sprintf("/v1/builds/%s/preReleaseVersion", buildID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response PreReleaseVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
