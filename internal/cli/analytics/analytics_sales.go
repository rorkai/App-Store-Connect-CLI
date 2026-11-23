package analytics

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAnalyticsSalesCommand downloads sales and trends reports.
func newAnalyticsSalesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("sales", flag.ExitOnError)

	vendor := fs.String("vendor", "", "Vendor number (or ASC_VENDOR_NUMBER/ASC_ANALYTICS_VENDOR_NUMBER env)")
	reportType := fs.String("type", "", "Report type: SALES, PRE_ORDER, NEWSSTAND, SUBSCRIPTION, SUBSCRIPTION_EVENT, SUBSCRIBER, SUBSCRIPTION_OFFER_CODE_REDEMPTION, INSTALLS (deprecated), FIRST_ANNUAL, WIN_BACK_ELIGIBILITY")
	reportSubType := fs.String("subtype", "", "Report subtype: SUMMARY, DETAILED, SUMMARY_INSTALL_TYPE, SUMMARY_TERRITORY, SUMMARY_CHANNEL")
	frequency := fs.String("frequency", "", "Frequency: DAILY, WEEKLY, MONTHLY, YEARLY")
	date := fs.String("date", "", "Report date: daily/weekly YYYY-MM-DD, monthly YYYY-MM or YYYY-MM-DD, yearly YYYY or YYYY-MM-DD (optional for DAILY)")
	version := fs.String("version", "", "Report format version allowed for the selected type, subtype, and frequency")
	output := fs.String("output", "", "Output file path (default: sales_report_{date|latest}_{type}.tsv.gz)")
	shared.BindFlagAlias(fs, "output-file", "output")
	shared.BindFlagAlias(fs, "output-path", "output")
	decompress := fs.Bool("decompress", false, "Decompress gzip output to .tsv")
	allowMissing := fs.Bool("allow-missing", false, "Return available=false (exit 0) when Apple says the report is unavailable or the period had no sales; use it when looping over dates")
	outputFlags := shared.BindMetadataOutputFlags(fs)

	return &ffcli.Command{
		Name:       "sales",
		ShortUsage: "asc analytics sales [flags]",
		ShortHelp:  "Download sales and trends reports.",
		LongHelp: `Download sales and trends reports.

Examples:
  asc analytics sales --vendor "12345678" --type SALES --subtype SUMMARY --frequency DAILY --date "2024-01-20"
  asc analytics sales --vendor "12345678" --type SALES --subtype SUMMARY --frequency DAILY
  asc analytics sales --vendor "12345678" --type SALES --subtype SUMMARY --frequency WEEKLY --date "2024-01-15" # Monday start accepted
  asc analytics sales --vendor "12345678" --type SUBSCRIBER --subtype DETAILED --frequency DAILY
  asc analytics sales --vendor "12345678" --type SALES --subtype SUMMARY --frequency DAILY --date "2024-01-20" --allow-missing
  asc analytics sales --vendor "12345678" --type SALES --subtype SUMMARY --frequency DAILY --date "2024-01-20" --decompress
  asc analytics sales --vendor "12345678" --type SALES --subtype SUMMARY --frequency DAILY --date "2024-01-20" --output "reports/daily_sales.tsv.gz"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.RejectOutputFormatAsPath(*output, "--output-format"); err != nil {
				return err
			}
			vendorNumber := d.ResolveVendorNumber(*vendor)
			if vendorNumber == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --vendor is required (or set ASC_VENDOR_NUMBER/ASC_ANALYTICS_VENDOR_NUMBER); find it in App Store Connect under Sales and Trends > Reports")
				return shared.MissingRequiredUsageError("--vendor")
			}
			if strings.TrimSpace(*reportType) == "" {
				return d.RequiredFlag("--type")
			}
			if strings.TrimSpace(*reportSubType) == "" {
				return d.RequiredFlag("--subtype")
			}
			if strings.TrimSpace(*frequency) == "" {
				return d.RequiredFlag("--frequency")
			}
			salesType, err := normalizeSalesReportType(*reportType)
			if err != nil {
				return d.UsageError(fmt.Sprintf("analytics sales: %v", err))
			}
			subType, err := normalizeSalesReportSubType(*reportSubType)
			if err != nil {
				return d.UsageError(fmt.Sprintf("analytics sales: %v", err))
			}
			freq, err := normalizeSalesReportFrequency(*frequency)
			if err != nil {
				return d.UsageError(fmt.Sprintf("analytics sales: %v", err))
			}
			reportDate, err := normalizeReportDate(*date, freq)
			if err != nil {
				return d.UsageError(fmt.Sprintf("analytics sales: %v", err))
			}
			reportVersion, err := normalizeSalesReportVersion(*version, salesType, subType, freq)
			if err != nil {
				return d.UsageError(fmt.Sprintf("analytics sales: %v", err))
			}
			if salesType == asc.SalesReportTypeInstalls {
				fmt.Fprintln(d.IO.ErrOut, "Warning: the INSTALLS report type is deprecated. Apple stopped generating new INSTALLS reports on November 4, 2026; reports generated before then can still be downloaded.")
			}

			outputDate := reportDate
			if outputDate == "" {
				outputDate = "latest"
			}
			defaultOutput := fmt.Sprintf("sales_report_%s_%s.tsv.gz", outputDate, string(salesType))
			compressedPath, decompressedPath := shared.ResolveReportOutputPaths(*output, defaultOutput, ".tsv", *decompress)

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("analytics sales: %w", err)
			}

			requestCtx, cancel := d.DownloadTimeout(ctx)
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
				if *allowMissing && isMissingSalesReportError(err) {
					available := false
					return d.PrintOutput(&asc.SalesReportResult{
						VendorNumber:  vendorNumber,
						ReportType:    string(salesType),
						ReportSubType: string(subType),
						Frequency:     string(freq),
						ReportDate:    reportDate,
						Version:       string(reportVersion),
						Available:     &available,
					}, *outputFlags.OutputFormat, *outputFlags.Pretty)
				}
				err = fmt.Errorf("analytics sales: failed to download report: %w", err)
				if isMissingSalesReportError(err) {
					retry := salesReportRetry{
						vendor:     strings.TrimSpace(*vendor),
						reportType: salesType,
						subType:    subType,
						frequency:  freq,
						decompress: *decompress,
					}
					if strings.TrimSpace(*version) != "" {
						retry.version = reportVersion
					}
					return &shared.HintedError{Err: err, Hint: missingSalesReportHint(err, retry, reportDate, d.Now())}
				}
				return err
			}
			defer download.Body.Close()

			compressedSize, err := d.WriteStreamToFile(compressedPath, download.Body)
			if err != nil {
				return fmt.Errorf("analytics sales: failed to write report: %w", err)
			}

			var decompressedSize int64
			if *decompress {
				decompressedSize, err = d.DecompressGzipFile(compressedPath, decompressedPath)
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

			return d.PrintOutput(result, *outputFlags.OutputFormat, *outputFlags.Pretty)
		},
	}
}

func isMissingSalesReportError(err error) bool {
	if !errors.Is(err, asc.ErrNotFound) {
		return false
	}

	var apiErr *asc.APIError
	if !errors.As(err, &apiErr) || apiErr == nil {
		return false
	}

	return isNoSalesDetail(apiErr) ||
		(apiErr.StatusCode == http.StatusNotFound && strings.HasPrefix(normalizedSalesDetail(apiErr), "report is not available yet."))
}

func isNoSalesDetail(apiErr *asc.APIError) bool {
	return strings.Contains(normalizedSalesDetail(apiErr), "no sales for the date specified")
}

func normalizedSalesDetail(apiErr *asc.APIError) string {
	return strings.ToLower(strings.TrimSpace(apiErr.Detail))
}
