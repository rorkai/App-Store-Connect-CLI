package asc

import (
	"fmt"
	"os"
)

// FinanceReportResult represents CLI output for finance report downloads.
type FinanceReportResult struct {
	VendorNumber      string `json:"vendorNumber"`
	ReportType        string `json:"reportType"`
	RegionCode        string `json:"regionCode"`
	ReportDate        string `json:"reportDate"`
	FilePath          string `json:"filePath"`
	Bytes             int64  `json:"fileSize"`
	Decompressed      bool   `json:"decompressed"`
	DecompressedPath  string `json:"decompressedPath,omitempty"`
	DecompressedBytes int64  `json:"decompressedSize,omitempty"`
}

func printFinanceReportResultTable(result *FinanceReportResult) error {
	headers := []string{"Vendor", "Type", "Region", "Date", "Compressed File", "Compressed Size", "Decompressed File", "Decompressed Size"}
	rows := [][]string{{
		result.VendorNumber,
		result.ReportType,
		result.RegionCode,
		result.ReportDate,
		result.FilePath,
		fmt.Sprintf("%d", result.Bytes),
		result.DecompressedPath,
		fmt.Sprintf("%d", result.DecompressedBytes),
	}}
	RenderTable(headers, rows)
	return nil
}

func printFinanceReportResultMarkdown(result *FinanceReportResult) error {
	fmt.Fprintln(os.Stdout, "| Vendor | Type | Region | Date | Compressed File | Compressed Size | Decompressed File | Decompressed Size |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- | --- | --- | --- | --- |")
	fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s | %s | %d | %s | %d |\n",
		escapeMarkdown(result.VendorNumber),
		escapeMarkdown(result.ReportType),
		escapeMarkdown(result.RegionCode),
		escapeMarkdown(result.ReportDate),
		escapeMarkdown(result.FilePath),
		result.Bytes,
		escapeMarkdown(result.DecompressedPath),
		result.DecompressedBytes,
	)
	return nil
}

func printFinanceRegionsTable(result *FinanceRegionsResult) error {
	headers := []string{"Region", "Currency", "Code", "Countries or Regions"}
	rows := make([][]string, 0, len(result.Regions))
	for _, region := range result.Regions {
		rows = append(rows, []string{
			region.ReportRegion,
			region.ReportCurrency,
			region.RegionCode,
			region.CountriesOrRegions,
		})
	}
	RenderTable(headers, rows)
	return nil
}

func printFinanceRegionsMarkdown(result *FinanceRegionsResult) error {
	fmt.Fprintln(os.Stdout, "| Region | Currency | Code | Countries or Regions |")
	fmt.Fprintln(os.Stdout, "| --- | --- | --- | --- |")
	for _, region := range result.Regions {
		fmt.Fprintf(os.Stdout, "| %s | %s | %s | %s |\n",
			escapeMarkdown(region.ReportRegion),
			escapeMarkdown(region.ReportCurrency),
			escapeMarkdown(region.RegionCode),
			escapeMarkdown(region.CountriesOrRegions),
		)
	}
	return nil
}
