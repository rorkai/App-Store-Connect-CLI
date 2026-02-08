package asc

import (
	"encoding/json"
	"fmt"
)

// PerformanceDownloadResult represents CLI output for performance downloads.
type PerformanceDownloadResult struct {
	DownloadType          string `json:"downloadType"`
	AppID                 string `json:"appId,omitempty"`
	BuildID               string `json:"buildId,omitempty"`
	DiagnosticSignatureID string `json:"diagnosticSignatureId,omitempty"`
	FilePath              string `json:"filePath"`
	FileSize              int64  `json:"fileSize"`
	Decompressed          bool   `json:"decompressed"`
	DecompressedPath      string `json:"decompressedPath,omitempty"`
	DecompressedSize      int64  `json:"decompressedSize,omitempty"`
}

type perfPowerMetricsSummary struct {
	Version         string
	ProductCount    int
	TrendingUpCount int
	RegressionCount int
}

type diagnosticLogsSummary struct {
	Version      string
	ProductCount int
	LogCount     int
	InsightCount int
}

func summarizePerfPowerMetrics(resp *PerfPowerMetricsResponse) (perfPowerMetricsSummary, error) {
	if resp == nil {
		return perfPowerMetricsSummary{}, fmt.Errorf("perf power metrics response is nil")
	}
	if len(resp.Data) == 0 {
		return perfPowerMetricsSummary{}, fmt.Errorf("perf power metrics response is empty")
	}

	var payload struct {
		Version  string `json:"version"`
		Insights struct {
			TrendingUp  []json.RawMessage `json:"trendingUp"`
			Regressions []json.RawMessage `json:"regressions"`
		} `json:"insights"`
		ProductData []json.RawMessage `json:"productData"`
	}
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		return perfPowerMetricsSummary{}, fmt.Errorf("decode perf power metrics: %w", err)
	}

	return perfPowerMetricsSummary{
		Version:         payload.Version,
		ProductCount:    len(payload.ProductData),
		TrendingUpCount: len(payload.Insights.TrendingUp),
		RegressionCount: len(payload.Insights.Regressions),
	}, nil
}

func summarizeDiagnosticLogs(resp *DiagnosticLogsResponse) (diagnosticLogsSummary, error) {
	if resp == nil {
		return diagnosticLogsSummary{}, fmt.Errorf("diagnostic logs response is nil")
	}
	if len(resp.Data) == 0 {
		return diagnosticLogsSummary{}, fmt.Errorf("diagnostic logs response is empty")
	}

	var payload struct {
		Version     string `json:"version"`
		ProductData []struct {
			DiagnosticLogs     []json.RawMessage `json:"diagnosticLogs"`
			DiagnosticInsights []json.RawMessage `json:"diagnosticInsights"`
		} `json:"productData"`
	}
	if err := json.Unmarshal(resp.Data, &payload); err != nil {
		return diagnosticLogsSummary{}, fmt.Errorf("decode diagnostic logs: %w", err)
	}

	logCount := 0
	insightCount := 0
	for _, item := range payload.ProductData {
		logCount += len(item.DiagnosticLogs)
		insightCount += len(item.DiagnosticInsights)
	}

	return diagnosticLogsSummary{
		Version:      payload.Version,
		ProductCount: len(payload.ProductData),
		LogCount:     logCount,
		InsightCount: insightCount,
	}, nil
}

func perfPowerMetricsRows(resp *PerfPowerMetricsResponse) ([]string, [][]string, error) {
	summary, err := summarizePerfPowerMetrics(resp)
	if err != nil {
		return nil, nil, err
	}
	headers := []string{"Version", "Products", "Trending Up", "Regressions"}
	rows := [][]string{{
		summary.Version,
		fmt.Sprintf("%d", summary.ProductCount),
		fmt.Sprintf("%d", summary.TrendingUpCount),
		fmt.Sprintf("%d", summary.RegressionCount),
	}}
	return headers, rows, nil
}

func printPerfPowerMetricsTable(resp *PerfPowerMetricsResponse) error {
	h, r, err := perfPowerMetricsRows(resp)
	if err != nil {
		return err
	}
	RenderTable(h, r)
	return nil
}

func printPerfPowerMetricsMarkdown(resp *PerfPowerMetricsResponse) error {
	h, r, err := perfPowerMetricsRows(resp)
	if err != nil {
		return err
	}
	RenderMarkdown(h, r)
	return nil
}

func diagnosticSignaturesRows(resp *DiagnosticSignaturesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Type", "Weight", "Insight", "Signature"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		insight := ""
		if item.Attributes.Insight != nil {
			insight = string(item.Attributes.Insight.Direction)
		}
		rows = append(rows, []string{
			item.ID,
			string(item.Attributes.DiagnosticType),
			fmt.Sprintf("%.2f", item.Attributes.Weight),
			insight,
			item.Attributes.Signature,
		})
	}
	return headers, rows
}

func printDiagnosticSignaturesTable(resp *DiagnosticSignaturesResponse) error {
	h, r := diagnosticSignaturesRows(resp)
	RenderTable(h, r)
	return nil
}

func printDiagnosticSignaturesMarkdown(resp *DiagnosticSignaturesResponse) error {
	h, r := diagnosticSignaturesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func diagnosticLogsRows(resp *DiagnosticLogsResponse) ([]string, [][]string, error) {
	summary, err := summarizeDiagnosticLogs(resp)
	if err != nil {
		return nil, nil, err
	}
	headers := []string{"Version", "Products", "Logs", "Insights"}
	rows := [][]string{{
		summary.Version,
		fmt.Sprintf("%d", summary.ProductCount),
		fmt.Sprintf("%d", summary.LogCount),
		fmt.Sprintf("%d", summary.InsightCount),
	}}
	return headers, rows, nil
}

func printDiagnosticLogsTable(resp *DiagnosticLogsResponse) error {
	h, r, err := diagnosticLogsRows(resp)
	if err != nil {
		return err
	}
	RenderTable(h, r)
	return nil
}

func printDiagnosticLogsMarkdown(resp *DiagnosticLogsResponse) error {
	h, r, err := diagnosticLogsRows(resp)
	if err != nil {
		return err
	}
	RenderMarkdown(h, r)
	return nil
}

func performanceDownloadResultRows(result *PerformanceDownloadResult) ([]string, [][]string) {
	headers := []string{"Type", "App ID", "Build ID", "Diagnostic ID", "Compressed File", "Compressed Size", "Decompressed File", "Decompressed Size"}
	rows := [][]string{{
		result.DownloadType,
		result.AppID,
		result.BuildID,
		result.DiagnosticSignatureID,
		result.FilePath,
		fmt.Sprintf("%d", result.FileSize),
		result.DecompressedPath,
		fmt.Sprintf("%d", result.DecompressedSize),
	}}
	return headers, rows
}

func printPerformanceDownloadResultTable(result *PerformanceDownloadResult) error {
	h, r := performanceDownloadResultRows(result)
	RenderTable(h, r)
	return nil
}

func printPerformanceDownloadResultMarkdown(result *PerformanceDownloadResult) error {
	h, r := performanceDownloadResultRows(result)
	RenderMarkdown(h, r)
	return nil
}
