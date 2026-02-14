package analytics

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AnalyticsSalesCommand downloads sales and trends reports.
func AnalyticsSalesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("sales", flag.ExitOnError)

	vendor := fs.String("vendor", "", "Vendor number (or ASC_VENDOR_NUMBER/ASC_ANALYTICS_VENDOR_NUMBER env)")
	reportType := fs.String("type", "", "Report type: SALES, PRE_ORDER, NEWSSTAND, SUBSCRIPTION, SUBSCRIPTION_EVENT")
	reportSubType := fs.String("subtype", "", "Report subtype: SUMMARY, DETAILED")
	frequency := fs.String("frequency", "", "Frequency: DAILY, WEEKLY, MONTHLY, YEARLY")
	date := fs.String("date", "", "Report date: daily/weekly YYYY-MM-DD, monthly YYYY-MM, yearly YYYY")
	version := fs.String("version", "1_0", "Report format version: 1_0 (default), 1_1")
	output := fs.String("output", "", "Output file path (default: sales_report_{date}_{type}.tsv.gz)")
	decompress := fs.Bool("decompress", false, "Decompress gzip output to .tsv")
	outputFlags := shared.BindMetadataOutputFlags(fs)

	return &ffcli.Command{
		Name:       "sales",
		ShortUsage: "asc analytics sales [flags]",
		ShortHelp:  "Download sales and trends reports.",
		LongHelp: `Download sales and trends reports.

Examples:
  asc analytics sales --vendor "12345678" --type SALES --subtype SUMMARY --frequency DAILY --date "2024-01-20"
  asc analytics sales --vendor "12345678" --type SUBSCRIPTION --subtype DETAILED --frequency MONTHLY --date "2024-01"
  asc analytics sales --vendor "12345678" --type SALES --subtype SUMMARY --frequency DAILY --date "2024-01-20" --decompress
  asc analytics sales --vendor "12345678" --type SALES --subtype SUMMARY --frequency DAILY --date "2024-01-20" --output "reports/daily_sales.tsv.gz"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			vendorNumber := shared.ResolveVendorNumber(*vendor)
			if vendorNumber == "" {
				fmt.Fprintln(os.Stderr, "Error: --vendor is required (or set ASC_VENDOR_NUMBER/ASC_ANALYTICS_VENDOR_NUMBER)")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*reportType) == "" {
				fmt.Fprintln(os.Stderr, "Error: --type is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*reportSubType) == "" {
				fmt.Fprintln(os.Stderr, "Error: --subtype is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*frequency) == "" {
				fmt.Fprintln(os.Stderr, "Error: --frequency is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*date) == "" {
				fmt.Fprintln(os.Stderr, "Error: --date is required")
				return flag.ErrHelp
			}

			salesType, err := normalizeSalesReportType(*reportType)
			if err != nil {
				return fmt.Errorf("analytics sales: %w", err)
			}
			subType, err := normalizeSalesReportSubType(*reportSubType)
			if err != nil {
				return fmt.Errorf("analytics sales: %w", err)
			}
			freq, err := normalizeSalesReportFrequency(*frequency)
			if err != nil {
				return fmt.Errorf("analytics sales: %w", err)
			}
			reportDate, err := normalizeReportDate(*date, freq)
			if err != nil {
				return fmt.Errorf("analytics sales: %w", err)
			}
			reportVersion, err := normalizeSalesReportVersion(*version)
			if err != nil {
				return fmt.Errorf("analytics sales: %w", err)
			}

			defaultOutput := fmt.Sprintf("sales_report_%s_%s.tsv.gz", reportDate, string(salesType))
			compressedPath, decompressedPath := shared.ResolveReportOutputPaths(*output, defaultOutput, ".tsv", *decompress)

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("analytics sales: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			download, err := client.GetSalesReport(requestCtx, asc.SalesReportParams{
				VendorNumber:  vendorNumber,
				ReportType:    salesType,
				ReportSubType: subType,
				Frequency:     freq,
				ReportDate:    reportDate,
				Version:       reportVersion,
			})
			if err != nil {
				return fmt.Errorf("analytics sales: failed to download report: %w", err)
			}
			defer download.Body.Close()

			compressedSize, err := shared.WriteStreamToFile(compressedPath, download.Body)
			if err != nil {
				return fmt.Errorf("analytics sales: failed to write report: %w", err)
			}

			var decompressedSize int64
			if *decompress {
				decompressedSize, err = shared.DecompressGzipFile(compressedPath, decompressedPath)
				if err != nil {
					return fmt.Errorf("analytics sales: %w", err)
				}
			}

			result := &asc.SalesReportResult{
				VendorNumber:     vendorNumber,
				ReportType:       string(salesType),
				ReportSubType:    string(subType),
				Frequency:        string(freq),
				ReportDate:       reportDate,
				Version:          string(reportVersion),
				FilePath:         compressedPath,
				FileSize:         compressedSize,
				Decompressed:     *decompress,
				DecompressedPath: decompressedPath,
				DecompressedSize: decompressedSize,
			}

			return shared.PrintOutput(result, *outputFlags.OutputFormat, *outputFlags.Pretty)
		},
	}
}
