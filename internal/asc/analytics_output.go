package asc

import "fmt"

// SalesReportResult represents CLI output for sales report downloads.
type SalesReportResult struct {
	VendorNumber     string `json:"vendorNumber"`
	ReportType       string `json:"reportType"`
	ReportSubType    string `json:"reportSubType"`
	Frequency        string `json:"frequency"`
	ReportDate       string `json:"reportDate"`
	Version          string `json:"version,omitempty"`
	FilePath         string `json:"filePath"`
	FileSize         int64  `json:"fileSize"`
	Decompressed     bool   `json:"decompressed"`
	DecompressedPath string `json:"decompressedPath,omitempty"`
	DecompressedSize int64  `json:"decompressedSize,omitempty"`
}

// AnalyticsReportRequestResult represents CLI output for created requests.
type AnalyticsReportRequestResult struct {
	RequestID   string `json:"requestId"`
	AppID       string `json:"appId"`
	AccessType  string `json:"accessType"`
	State       string `json:"state,omitempty"`
	CreatedDate string `json:"createdDate,omitempty"`
}

// AnalyticsReportRequestDeleteResult represents CLI output for deleted requests.
type AnalyticsReportRequestDeleteResult struct {
	RequestID string `json:"requestId"`
	Deleted   bool   `json:"deleted"`
}

// AnalyticsReportDownloadResult represents CLI output for analytics downloads.
type AnalyticsReportDownloadResult struct {
	RequestID        string `json:"requestId"`
	InstanceID       string `json:"instanceId"`
	SegmentID        string `json:"segmentId,omitempty"`
	FilePath         string `json:"filePath"`
	FileSize         int64  `json:"fileSize"`
	Decompressed     bool   `json:"decompressed"`
	DecompressedPath string `json:"decompressedPath,omitempty"`
	DecompressedSize int64  `json:"decompressedSize,omitempty"`
}

// AnalyticsReportGetResult represents CLI output for report metadata with instances.
type AnalyticsReportGetResult struct {
	RequestID string                     `json:"requestId"`
	Data      []AnalyticsReportGetReport `json:"data"`
	Links     Links                      `json:"links,omitempty"`
}

// AnalyticsReportGetReport represents an analytics report with instances.
type AnalyticsReportGetReport struct {
	ID          string                       `json:"id"`
	ReportType  string                       `json:"reportType,omitempty"`
	Name        string                       `json:"name,omitempty"`
	Category    string                       `json:"category,omitempty"`
	Granularity string                       `json:"granularity,omitempty"`
	Instances   []AnalyticsReportGetInstance `json:"instances,omitempty"`
}

// AnalyticsReportGetInstance represents a report instance with segments.
type AnalyticsReportGetInstance struct {
	ID             string                      `json:"id"`
	ReportDate     string                      `json:"reportDate,omitempty"`
	ProcessingDate string                      `json:"processingDate,omitempty"`
	Granularity    string                      `json:"granularity,omitempty"`
	Version        string                      `json:"version,omitempty"`
	Segments       []AnalyticsReportGetSegment `json:"segments,omitempty"`
}

// AnalyticsReportGetSegment represents a report segment with download URL.
type AnalyticsReportGetSegment struct {
	ID                string `json:"id"`
	DownloadURL       string `json:"downloadUrl,omitempty"`
	Checksum          string `json:"checksum,omitempty"`
	SizeInBytes       int64  `json:"sizeInBytes,omitempty"`
	URLExpirationDate string `json:"urlExpirationDate,omitempty"`
}

func salesReportResultRows(result *SalesReportResult) ([]string, [][]string) {
	headers := []string{"Vendor", "Type", "Subtype", "Frequency", "Date", "Version", "Compressed File", "Compressed Size", "Decompressed File", "Decompressed Size"}
	rows := [][]string{{
		result.VendorNumber,
		result.ReportType,
		result.ReportSubType,
		result.Frequency,
		result.ReportDate,
		result.Version,
		result.FilePath,
		fmt.Sprintf("%d", result.FileSize),
		result.DecompressedPath,
		fmt.Sprintf("%d", result.DecompressedSize),
	}}
	return headers, rows
}

func printSalesReportResultTable(result *SalesReportResult) error {
	h, r := salesReportResultRows(result)
	RenderTable(h, r)
	return nil
}

func printSalesReportResultMarkdown(result *SalesReportResult) error {
	h, r := salesReportResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func analyticsReportRequestResultRows(result *AnalyticsReportRequestResult) ([]string, [][]string) {
	headers := []string{"Request ID", "App ID", "Access Type", "State", "Created Date"}
	rows := [][]string{{result.RequestID, result.AppID, result.AccessType, result.State, result.CreatedDate}}
	return headers, rows
}

func printAnalyticsReportRequestResultTable(result *AnalyticsReportRequestResult) error {
	h, r := analyticsReportRequestResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAnalyticsReportRequestResultMarkdown(result *AnalyticsReportRequestResult) error {
	h, r := analyticsReportRequestResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func analyticsReportRequestDeleteResultRows(result *AnalyticsReportRequestDeleteResult) ([]string, [][]string) {
	headers := []string{"Request ID", "Deleted"}
	rows := [][]string{{result.RequestID, fmt.Sprintf("%t", result.Deleted)}}
	return headers, rows
}

func printAnalyticsReportRequestDeleteResultTable(result *AnalyticsReportRequestDeleteResult) error {
	h, r := analyticsReportRequestDeleteResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAnalyticsReportRequestDeleteResultMarkdown(result *AnalyticsReportRequestDeleteResult) error {
	h, r := analyticsReportRequestDeleteResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func analyticsReportRequestsRows(resp *AnalyticsReportRequestsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Access Type", "State", "Created Date", "App ID"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		appID := ""
		if item.Relationships != nil && item.Relationships.App != nil {
			appID = item.Relationships.App.Data.ID
		}
		rows = append(rows, []string{
			item.ID,
			string(item.Attributes.AccessType),
			string(item.Attributes.State),
			item.Attributes.CreatedDate,
			appID,
		})
	}
	return headers, rows
}

func printAnalyticsReportRequestsTable(resp *AnalyticsReportRequestsResponse) error {
	h, r := analyticsReportRequestsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAnalyticsReportRequestsMarkdown(resp *AnalyticsReportRequestsResponse) error {
	h, r := analyticsReportRequestsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func analyticsReportDownloadResultRows(result *AnalyticsReportDownloadResult) ([]string, [][]string) {
	headers := []string{"Request ID", "Instance ID", "Segment ID", "Compressed File", "Compressed Size", "Decompressed File", "Decompressed Size"}
	rows := [][]string{{
		result.RequestID,
		result.InstanceID,
		result.SegmentID,
		result.FilePath,
		fmt.Sprintf("%d", result.FileSize),
		result.DecompressedPath,
		fmt.Sprintf("%d", result.DecompressedSize),
	}}
	return headers, rows
}

func printAnalyticsReportDownloadResultTable(result *AnalyticsReportDownloadResult) error {
	h, r := analyticsReportDownloadResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAnalyticsReportDownloadResultMarkdown(result *AnalyticsReportDownloadResult) error {
	h, r := analyticsReportDownloadResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func analyticsReportGetResultRows(result *AnalyticsReportGetResult) ([]string, [][]string) {
	headers := []string{"Report ID", "Name", "Category", "Granularity", "Instances", "Segments"}
	rows := make([][]string, 0, len(result.Data))
	for _, report := range result.Data {
		name := report.Name
		if name == "" {
			name = report.ReportType
		}
		segments := countSegments(report.Instances)
		rows = append(rows, []string{
			report.ID,
			name,
			report.Category,
			report.Granularity,
			fmt.Sprintf("%d", len(report.Instances)),
			fmt.Sprintf("%d", segments),
		})
	}
	return headers, rows
}

func printAnalyticsReportGetResultTable(result *AnalyticsReportGetResult) error {
	h, r := analyticsReportGetResultRows(result)
	RenderTable(h, r)
	return nil
}

func printAnalyticsReportGetResultMarkdown(result *AnalyticsReportGetResult) error {
	h, r := analyticsReportGetResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func analyticsReportsRows(resp *AnalyticsReportsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Name", "Report Type", "Category", "Subcategory", "Granularity"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.Name,
			item.Attributes.ReportType,
			item.Attributes.Category,
			item.Attributes.SubCategory,
			item.Attributes.Granularity,
		})
	}
	return headers, rows
}

func printAnalyticsReportsTable(resp *AnalyticsReportsResponse) error {
	h, r := analyticsReportsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAnalyticsReportsMarkdown(resp *AnalyticsReportsResponse) error {
	h, r := analyticsReportsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func analyticsReportInstancesRows(resp *AnalyticsReportInstancesResponse) ([]string, [][]string) {
	headers := []string{"ID", "Report Date", "Processing Date", "Granularity", "Version"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.ReportDate,
			item.Attributes.ProcessingDate,
			item.Attributes.Granularity,
			item.Attributes.Version,
		})
	}
	return headers, rows
}

func printAnalyticsReportInstancesTable(resp *AnalyticsReportInstancesResponse) error {
	h, r := analyticsReportInstancesRows(resp)
	RenderTable(h, r)
	return nil
}

func printAnalyticsReportInstancesMarkdown(resp *AnalyticsReportInstancesResponse) error {
	h, r := analyticsReportInstancesRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func analyticsReportSegmentsRows(resp *AnalyticsReportSegmentsResponse) ([]string, [][]string) {
	headers := []string{"ID", "Download URL", "Checksum", "Size (bytes)", "URL Expires"}
	rows := make([][]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		rows = append(rows, []string{
			item.ID,
			item.Attributes.URL,
			item.Attributes.Checksum,
			fmt.Sprintf("%d", item.Attributes.SizeInBytes),
			item.Attributes.URLExpirationDate,
		})
	}
	return headers, rows
}

func printAnalyticsReportSegmentsTable(resp *AnalyticsReportSegmentsResponse) error {
	h, r := analyticsReportSegmentsRows(resp)
	RenderTable(h, r)
	return nil
}

func printAnalyticsReportSegmentsMarkdown(resp *AnalyticsReportSegmentsResponse) error {
	h, r := analyticsReportSegmentsRows(resp)
	RenderMarkdown(h, r)
	return nil
}

func countSegments(instances []AnalyticsReportGetInstance) int {
	total := 0
	for _, instance := range instances {
		total += len(instance.Segments)
	}
	return total
}
