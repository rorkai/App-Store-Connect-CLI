package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	perfPowerMetricsAcceptHeader = "application/vnd.apple.xcode-metrics+json"
	diagnosticLogsAcceptHeader   = "application/vnd.apple.diagnostic-logs+json"
)

// GetPerfPowerMetricsForApp retrieves performance/power metrics for an app.
func (c *Client) GetPerfPowerMetricsForApp(ctx context.Context, appID string, opts ...PerfPowerMetricsOption) (*PerfPowerMetricsResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app ID is required")
	}

	path := fmt.Sprintf("/v1/apps/%s/perfPowerMetrics", appID)
	query := &perfPowerMetricsQuery{}
	for _, opt := range opts {
		opt(query)
	}
	if queryString := buildPerfPowerMetricsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	return c.fetchPerfPowerMetrics(ctx, path)
}

// GetPerfPowerMetricsForBuild retrieves performance/power metrics for a build.
func (c *Client) GetPerfPowerMetricsForBuild(ctx context.Context, buildID string, opts ...PerfPowerMetricsOption) (*PerfPowerMetricsResponse, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return nil, fmt.Errorf("build ID is required")
	}

	path := fmt.Sprintf("/v1/builds/%s/perfPowerMetrics", buildID)
	query := &perfPowerMetricsQuery{}
	for _, opt := range opts {
		opt(query)
	}
	if queryString := buildPerfPowerMetricsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	return c.fetchPerfPowerMetrics(ctx, path)
}

// DownloadPerfPowerMetricsForApp streams performance/power metrics for an app.
func (c *Client) DownloadPerfPowerMetricsForApp(ctx context.Context, appID string, opts ...PerfPowerMetricsOption) (*ReportDownload, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app ID is required")
	}

	path := fmt.Sprintf("/v1/apps/%s/perfPowerMetrics", appID)
	query := &perfPowerMetricsQuery{}
	for _, opt := range opts {
		opt(query)
	}
	if queryString := buildPerfPowerMetricsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	resp, err := c.doStream(ctx, path, perfPowerMetricsAcceptHeader)
	if err != nil {
		return nil, err
	}

	return &ReportDownload{Body: resp.Body, ContentLength: resp.ContentLength}, nil
}

// DownloadPerfPowerMetricsForBuild streams performance/power metrics for a build.
func (c *Client) DownloadPerfPowerMetricsForBuild(ctx context.Context, buildID string, opts ...PerfPowerMetricsOption) (*ReportDownload, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return nil, fmt.Errorf("build ID is required")
	}

	path := fmt.Sprintf("/v1/builds/%s/perfPowerMetrics", buildID)
	query := &perfPowerMetricsQuery{}
	for _, opt := range opts {
		opt(query)
	}
	if queryString := buildPerfPowerMetricsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	resp, err := c.doStream(ctx, path, perfPowerMetricsAcceptHeader)
	if err != nil {
		return nil, err
	}

	return &ReportDownload{Body: resp.Body, ContentLength: resp.ContentLength}, nil
}

func (c *Client) fetchPerfPowerMetrics(ctx context.Context, path string) (*PerfPowerMetricsResponse, error) {
	resp, err := c.doStream(ctx, path, perfPowerMetricsAcceptHeader)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read perf power metrics: %w", err)
	}

	return &PerfPowerMetricsResponse{Data: data}, nil
}

// GetDiagnosticSignaturesForBuild retrieves diagnostic signatures for a build.
func (c *Client) GetDiagnosticSignaturesForBuild(ctx context.Context, buildID string, opts ...DiagnosticSignaturesOption) (*DiagnosticSignaturesResponse, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return nil, fmt.Errorf("build ID is required")
	}

	query := &diagnosticSignaturesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/builds/%s/diagnosticSignatures", buildID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("diagnosticSignatures: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildDiagnosticSignaturesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response DiagnosticSignaturesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse diagnostic signatures response: %w", err)
	}

	return &response, nil
}

// GetDiagnosticSignatureLogs retrieves diagnostic logs for a signature.
func (c *Client) GetDiagnosticSignatureLogs(ctx context.Context, signatureID string, opts ...DiagnosticLogsOption) (*DiagnosticLogsResponse, error) {
	signatureID = strings.TrimSpace(signatureID)
	if signatureID == "" {
		return nil, fmt.Errorf("diagnostic signature ID is required")
	}

	query := &diagnosticLogsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/diagnosticSignatures/%s/logs", signatureID)
	if queryString := buildDiagnosticLogsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	resp, err := c.doStream(ctx, path, diagnosticLogsAcceptHeader)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read diagnostic logs: %w", err)
	}

	return &DiagnosticLogsResponse{Data: data}, nil
}

// DownloadDiagnosticSignatureLogs streams diagnostic logs for a signature.
func (c *Client) DownloadDiagnosticSignatureLogs(ctx context.Context, signatureID string, opts ...DiagnosticLogsOption) (*ReportDownload, error) {
	signatureID = strings.TrimSpace(signatureID)
	if signatureID == "" {
		return nil, fmt.Errorf("diagnostic signature ID is required")
	}

	query := &diagnosticLogsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/diagnosticSignatures/%s/logs", signatureID)
	if queryString := buildDiagnosticLogsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	resp, err := c.doStream(ctx, path, diagnosticLogsAcceptHeader)
	if err != nil {
		return nil, err
	}

	return &ReportDownload{Body: resp.Body, ContentLength: resp.ContentLength}, nil
}
