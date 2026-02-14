package finance

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

// FinanceCommand returns the finance command with subcommands.
func FinanceCommand() *ffcli.Command {
	fs := flag.NewFlagSet("finance", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "finance",
		ShortUsage: "asc finance <subcommand> [flags]",
		ShortHelp:  "Download payments and financial reports.",
		LongHelp: `Download payments and financial reports.

Finance reports are monthly and available through the App Store Connect API.
Requires Account Holder, Admin, or Finance role.

Examples:
  asc finance reports --vendor "12345678" --report-type FINANCIAL --region "US" --date "2025-12"
  asc finance regions --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			FinanceReportsCommand(),
			FinanceRegionsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// FinanceReportsCommand downloads finance reports.
func FinanceReportsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("reports", flag.ExitOnError)

	vendor := fs.String("vendor", "", "Vendor number (or ASC_VENDOR_NUMBER env)")
	reportType := fs.String("report-type", "", "Report type: FINANCIAL or FINANCE_DETAIL (see help for UI mapping)")
	region := fs.String("region", "", "Region code (e.g., US, ZZ, Z1; see 'asc finance regions')")
	date := fs.String("date", "", "Report date (YYYY-MM, Apple fiscal month)")
	output := fs.String("output", "", "Output file path (default: finance_report_{date}_{type}_{region}.tsv.gz)")
	decompress := fs.Bool("decompress", false, "Decompress gzip output to .tsv")
	outputFlags := shared.BindMetadataOutputFlags(fs)

	return &ffcli.Command{
		Name:       "reports",
		ShortUsage: "asc finance reports [flags]",
		ShortHelp:  "Download financial reports from App Store Connect.",
		LongHelp: `Download financial reports from App Store Connect.

Requires Account Holder, Admin, or Finance role.

REPORT TYPES (API to UI mapping):

The App Store Connect UI shows four report types, but only two are available via API:

  FINANCIAL       Aggregated monthly financial report
                  UI: "All Countries or Regions (Single File)" with --region ZZ
                  UI: "All Countries or Regions (Multiple Files)" with --region US, EU, etc.

  FINANCE_DETAIL  Detailed report with transaction and settlement dates
                  UI: "All Countries or Regions (Detailed)"
                  IMPORTANT: Requires --region Z1 (the only valid region for this type)

  Transaction Tax reports are NOT available via API. Download them manually from
  App Store Connect: Payments and Financial Reports > Create Reports > Transaction Tax.

REGION CODES:

  Individual countries: US, AU, CA, JP, GB, etc.
  Euro-Zone aggregate:  EU
  Latin America:        LL
  Asia-Pacific:         AP
  Rest of World:        WW
  Consolidated (all):   ZZ (for FINANCIAL)
  Financial Detail:     Z1 (required for FINANCE_DETAIL)

  Run 'asc finance regions' for the complete list.

Examples:
  # Download single consolidated report (all regions)
  asc finance reports --vendor "12345678" --report-type FINANCIAL --region "ZZ" --date "2025-12"

  # Download US-only report
  asc finance reports --vendor "12345678" --report-type FINANCIAL --region "US" --date "2025-12"

  # Download detailed report (transaction-level data)
  asc finance reports --vendor "12345678" --report-type FINANCE_DETAIL --region "Z1" --date "2025-12" --decompress

  # Save to custom path
  asc finance reports --vendor "12345678" --report-type FINANCIAL --region "US" --date "2025-12" --output "reports/finance.tsv.gz"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			vendorNumber := shared.ResolveVendorNumber(*vendor)
			if vendorNumber == "" {
				fmt.Fprintln(os.Stderr, "Error: --vendor is required (or set ASC_VENDOR_NUMBER)")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*reportType) == "" {
				fmt.Fprintln(os.Stderr, "Error: --report-type is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*region) == "" {
				fmt.Fprintln(os.Stderr, "Error: --region is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*date) == "" {
				fmt.Fprintln(os.Stderr, "Error: --date is required")
				return flag.ErrHelp
			}

			normalizedReportType, err := normalizeFinanceReportType(*reportType)
			if err != nil {
				return fmt.Errorf("finance reports: %w", err)
			}
			reportDate, err := normalizeFinanceReportDate(*date)
			if err != nil {
				return fmt.Errorf("finance reports: %w", err)
			}
			regionCode, err := normalizeFinanceReportRegion(normalizedReportType, *region)
			if err != nil {
				return fmt.Errorf("finance reports: %w", err)
			}
			defaultOutput := fmt.Sprintf("finance_report_%s_%s_%s.tsv.gz", reportDate, string(normalizedReportType), regionCode)
			compressedPath, decompressedPath := shared.ResolveReportOutputPaths(*output, defaultOutput, ".tsv", *decompress)

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("finance reports: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			download, err := client.DownloadFinanceReport(requestCtx, asc.FinanceReportParams{
				VendorNumber: vendorNumber,
				ReportType:   normalizedReportType,
				RegionCode:   regionCode,
				ReportDate:   reportDate,
			})
			if err != nil {
				return fmt.Errorf("finance reports: failed to download report: %w", err)
			}
			defer download.Body.Close()

			compressedSize, err := shared.WriteStreamToFile(compressedPath, download.Body)
			if err != nil {
				return fmt.Errorf("finance reports: failed to write report: %w", err)
			}

			var decompressedSize int64
			if *decompress {
				decompressedSize, err = shared.DecompressGzipFile(compressedPath, decompressedPath)
				if err != nil {
					return fmt.Errorf("finance reports: %w", err)
				}
			}

			result := &asc.FinanceReportResult{
				VendorNumber:      vendorNumber,
				ReportType:        string(normalizedReportType),
				RegionCode:        regionCode,
				ReportDate:        reportDate,
				FilePath:          compressedPath,
				Bytes:             compressedSize,
				Decompressed:      *decompress,
				DecompressedPath:  decompressedPath,
				DecompressedBytes: decompressedSize,
			}

			return shared.PrintOutput(result, *outputFlags.OutputFormat, *outputFlags.Pretty)
		},
	}
}

// FinanceRegionsCommand lists finance report regions and currencies.
func FinanceRegionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("regions", flag.ExitOnError)

	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "regions",
		ShortUsage: "asc finance regions [flags]",
		ShortHelp:  "List finance report region codes and currencies.",
		LongHelp: `List finance report region codes and currencies.

Use these codes with 'asc finance reports --region <code>':

  - Individual countries: US, AU, CA, JP, GB, etc. (with FINANCIAL)
  - Regional aggregates:  EU, LL, AP, WW (with FINANCIAL)
  - Consolidated:         ZZ (with FINANCIAL, all regions in one file)
  - Detailed:             Z1 (required for FINANCE_DETAIL)

Source: https://developer.apple.com/help/app-store-connect/reference/financial-report-regions-and-currencies/

Examples:
  asc finance regions
  asc finance regions --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			result := &asc.FinanceRegionsResult{Regions: asc.FinanceRegions()}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
