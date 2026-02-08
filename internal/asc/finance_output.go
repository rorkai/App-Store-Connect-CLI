package asc

import "fmt"

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

func financeReportResultRows(result *FinanceReportResult) ([]string, [][]string) {
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
	return headers, rows
}

func printFinanceReportResultTable(result *FinanceReportResult) error {
	h, r := financeReportResultRows(result)
	RenderTable(h, r)
	return nil
}

func printFinanceReportResultMarkdown(result *FinanceReportResult) error {
	h, r := financeReportResultRows(result)
	RenderMarkdown(h, r)
	return nil
}

func financeRegionsRows(result *FinanceRegionsResult) ([]string, [][]string) {
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
	return headers, rows
}

func printFinanceRegionsTable(result *FinanceRegionsResult) error {
	h, r := financeRegionsRows(result)
	RenderTable(h, r)
	return nil
}

func printFinanceRegionsMarkdown(result *FinanceRegionsResult) error {
	h, r := financeRegionsRows(result)
	RenderMarkdown(h, r)
	return nil
}
