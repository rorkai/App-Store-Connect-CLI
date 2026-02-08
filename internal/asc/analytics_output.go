package asc

import (
	"fmt"
	"os"
)

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

func printSalesReportResultTable(result *SalesReportResult) error {
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
	RenderTable(headers, rows)
	return nil
}

func printSalesReportResultMarkdown(result *SalesReportResult) error {
	fmt.Fprintln(os.Stdout, "| Vendor | Type | Subtype | Frequency | Date | Version | Compressed File | Compressed Size | Decompressed File | Decompressed Size |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s | %s | %d | %s | %d |\n",
		escapeMarkdown(result.VendorNumber),
		escapeMarkdown(result.ReportType),
		escapeMarkdown(result.ReportSubType),
		escapeMarkdown(result.Frequency),
		escapeMarkdown(result.ReportDate),
		escapeMarkdown(result.Version),
		escapeMarkdown(result.FilePath),
		result.FileSize,
		escapeMarkdown(result.DecompressedPath),
		result.DecompressedSize,
	)
	return nil
}

func printAnalyticsReportRequestResultTable(result *AnalyticsReportRequestResult) error {
	headers := []string{"Request ID", "App ID", "Access Type", "State", "Created Date"}
	rows := [][]string{{result.RequestID, result.AppID, result.AccessType, result.State, result.CreatedDate}}
	RenderTable(headers, rows)
	return nil
}

func printAnalyticsReportRequestResultMarkdown(result *AnalyticsReportRequestResult) error {
	fmt.Fprintln(os.Stdout, "| Request ID | App ID | Access Type | State | Created Date |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
		escapeMarkdown(result.RequestID),
		escapeMarkdown(result.AppID),
		escapeMarkdown(result.AccessType),
		escapeMarkdown(result.State),
		escapeMarkdown(result.CreatedDate),
	)
	return nil
}

func printAnalyticsReportRequestDeleteResultTable(result *AnalyticsReportRequestDeleteResult) error {
	headers := []string{"Request ID", "Deleted"}
	rows := [][]string{{result.RequestID, fmt.Sprintf("%t", result.Deleted)}}
	RenderTable(headers, rows)
	return nil
}

func printAnalyticsReportRequestDeleteResultMarkdown(result *AnalyticsReportRequestDeleteResult) error {
	fmt.Fprintln(os.Stdout, "| Request ID | Deleted |")
	fmt.Fprintln(os.Stdout, "| --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %t |\n",
		escapeMarkdown(result.RequestID),
		result.Deleted,
	)
	return nil
}

func printAnalyticsReportRequestsTable(resp *AnalyticsReportRequestsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printAnalyticsReportRequestsMarkdown(resp *AnalyticsReportRequestsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Access Type | State | Created Date | App ID |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		appID := ""
		if item.Relationships != nil && item.Relationships.App != nil {
			appID = item.Relationships.App.Data.ID
		}
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(string(item.Attributes.AccessType)),
			escapeMarkdown(string(item.Attributes.State)),
			escapeMarkdown(item.Attributes.CreatedDate),
			escapeMarkdown(appID),
		)
	}
	return nil
}

func printAnalyticsReportDownloadResultTable(result *AnalyticsReportDownloadResult) error {
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
	RenderTable(headers, rows)
	return nil
}

func printAnalyticsReportDownloadResultMarkdown(result *AnalyticsReportDownloadResult) error {
	fmt.Fprintln(os.Stdout, "| Request ID | Instance ID | Segment ID | Compressed File | Compressed Size | Decompressed File | Decompressed Size |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %d | %s | %d |\n",
		escapeMarkdown(result.RequestID),
		escapeMarkdown(result.InstanceID),
		escapeMarkdown(result.SegmentID),
		escapeMarkdown(result.FilePath),
		result.FileSize,
		escapeMarkdown(result.DecompressedPath),
		result.DecompressedSize,
	)
	return nil
}

func printAnalyticsReportGetResultTable(result *AnalyticsReportGetResult) error {
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
	RenderTable(headers, rows)
	return nil
}

func printAnalyticsReportGetResultMarkdown(result *AnalyticsReportGetResult) error {
	fmt.Fprintln(os.Stdout, "| Report ID | Name | Category | Granularity | Instances | Segments |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	for _, report := range result.Data {
		name := report.Name
		if name == "" {
			name = report.ReportType
		}
		segments := countSegments(report.Instances)
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %d | %d |\n",
			escapeMarkdown(report.ID),
			escapeMarkdown(name),
			escapeMarkdown(report.Category),
			escapeMarkdown(report.Granularity),
			len(report.Instances),
			segments,
		)
	}
	return nil
}

func printAnalyticsReportsTable(resp *AnalyticsReportsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printAnalyticsReportsMarkdown(resp *AnalyticsReportsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Name | Report Type | Category | Subcategory | Granularity |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.Name),
			escapeMarkdown(item.Attributes.ReportType),
			escapeMarkdown(item.Attributes.Category),
			escapeMarkdown(item.Attributes.SubCategory),
			escapeMarkdown(item.Attributes.Granularity),
		)
	}
	return nil
}

func printAnalyticsReportInstancesTable(resp *AnalyticsReportInstancesResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printAnalyticsReportInstancesMarkdown(resp *AnalyticsReportInstancesResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Report Date | Processing Date | Granularity | Version |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.ReportDate),
			escapeMarkdown(item.Attributes.ProcessingDate),
			escapeMarkdown(item.Attributes.Granularity),
			escapeMarkdown(item.Attributes.Version),
		)
	}
	return nil
}

func printAnalyticsReportSegmentsTable(resp *AnalyticsReportSegmentsResponse) error {
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
	RenderTable(headers, rows)
	return nil
}

func printAnalyticsReportSegmentsMarkdown(resp *AnalyticsReportSegmentsResponse) error {
	fmt.Fprintln(os.Stdout, "| ID | Download URL | Checksum | Size (bytes) | URL Expires |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- |")
	for _, item := range resp.Data {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %d | %s |\n",
			escapeMarkdown(item.ID),
			escapeMarkdown(item.Attributes.URL),
			escapeMarkdown(item.Attributes.Checksum),
			item.Attributes.SizeInBytes,
			escapeMarkdown(item.Attributes.URLExpirationDate),
		)
	}
	return nil
}

func countSegments(instances []AnalyticsReportGetInstance) int {
	total := 0
	for _, instance := range instances {
		total += len(instance.Segments)
	}
	return total
}
