package performance

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// PerformanceDownloadCommand returns the download subcommand.
func PerformanceDownloadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("download", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	buildID := fs.String("build", "", "Build ID to download metrics for")
	diagnosticID := fs.String("diagnostic-id", "", "Diagnostic signature ID to download logs for")
	platform := fs.String("platform", "", "Platform filter (IOS)")
	metricType := fs.String("metric-type", "", "Metric types (comma-separated: "+strings.Join(perfPowerMetricTypeList(), ", ")+")")
	deviceType := fs.String("device-type", "", "Device types (comma-separated, e.g., iPhone15,2)")
	limit := fs.Int("limit", 0, "Limit number of logs (max 200, diagnostic logs only)")
	output := fs.String("output", "", "Output file path (default: metrics/diagnostic file name)")
	decompress := fs.Bool("decompress", false, "Decompress gzip output (if compressed)")
	outputFormat := fs.String("output-format", "json", "Output format for metadata: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "download",
		ShortUsage: "asc performance download [flags]",
		ShortHelp:  "Download metrics or diagnostic logs.",
		LongHelp: `Download metrics or diagnostic logs.

Examples:
  asc performance download --app "APP_ID" --output ./metrics.json
  asc performance download --build "BUILD_ID" --output ./metrics.json
  asc performance download --diagnostic-id "SIGNATURE_ID" --output ./diagnostic.json --decompress`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			appFlag := strings.TrimSpace(*appID)
			trimmedBuildID := strings.TrimSpace(*buildID)
			trimmedDiagnosticID := strings.TrimSpace(*diagnosticID)

			selectionCount := 0
			if appFlag != "" {
				selectionCount++
			}
			if trimmedBuildID != "" {
				selectionCount++
			}
			if trimmedDiagnosticID != "" {
				selectionCount++
			}
			if selectionCount == 0 {
				appFlag = shared.ResolveAppID(*appID)
				if appFlag == "" {
					fmt.Fprintln(os.Stderr, "Error: --app, --build, or --diagnostic-id is required")
					return flag.ErrHelp
				}
				selectionCount = 1
			}
			if selectionCount > 1 {
				return shared.UsageError("--app, --build, and --diagnostic-id are mutually exclusive")
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("--limit must be between 1 and 200")
			}
			if trimmedDiagnosticID != "" && (strings.TrimSpace(*platform) != "" || strings.TrimSpace(*metricType) != "" || strings.TrimSpace(*deviceType) != "") {
				return shared.UsageError("metric filters are not valid with --diagnostic-id")
			}
			if trimmedDiagnosticID == "" && *limit > 0 {
				return shared.UsageError("--limit is only valid with --diagnostic-id")
			}

			platforms, err := normalizePerfPowerMetricPlatforms(shared.SplitCSVUpper(*platform), "--platform")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			metricTypes, err := normalizePerfPowerMetricTypes(shared.SplitCSVUpper(*metricType))
			if err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("performance download: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			switch {
			case trimmedDiagnosticID != "":
				defaultOutput := fmt.Sprintf("diagnostic_logs_%s.json", trimmedDiagnosticID)

				download, err := client.DownloadDiagnosticSignatureLogs(requestCtx, trimmedDiagnosticID, asc.WithDiagnosticLogsLimit(*limit))
				if err != nil {
					return fmt.Errorf("performance download: %w", err)
				}
				defer download.Body.Close()

				reader, isGzip, err := preparePerformanceDownloadReader(download.Body, *decompress)
				if err != nil {
					return fmt.Errorf("performance download: %w", err)
				}
				shouldDecompress := *decompress && isGzip
				compressedPath, decompressedPath := shared.ResolveReportOutputPaths(*output, defaultOutput, ".json", shouldDecompress)

				compressedSize, err := shared.WriteStreamToFile(compressedPath, reader)
				if err != nil {
					return fmt.Errorf("performance download: %w", err)
				}

				var decompressedSize int64
				if shouldDecompress {
					decompressedSize, err = shared.DecompressGzipFile(compressedPath, decompressedPath)
					if err != nil {
						return fmt.Errorf("performance download: %w", err)
					}
				}

				result := &asc.PerformanceDownloadResult{
					DownloadType:          "diagnostic-logs",
					DiagnosticSignatureID: trimmedDiagnosticID,
					FilePath:              compressedPath,
					FileSize:              compressedSize,
					Decompressed:          shouldDecompress,
					DecompressedPath:      decompressedPath,
					DecompressedSize:      decompressedSize,
				}

				return shared.PrintOutput(result, *outputFormat, *pretty)
			case trimmedBuildID != "":
				defaultOutput := fmt.Sprintf("perf_power_metrics_%s.json", trimmedBuildID)

				download, err := client.DownloadPerfPowerMetricsForBuild(requestCtx, trimmedBuildID,
					asc.WithPerfPowerMetricsPlatforms(platforms),
					asc.WithPerfPowerMetricsMetricTypes(metricTypes),
					asc.WithPerfPowerMetricsDeviceTypes(shared.SplitCSV(*deviceType)),
				)
				if err != nil {
					return fmt.Errorf("performance download: %w", err)
				}
				defer download.Body.Close()

				reader, isGzip, err := preparePerformanceDownloadReader(download.Body, *decompress)
				if err != nil {
					return fmt.Errorf("performance download: %w", err)
				}
				shouldDecompress := *decompress && isGzip
				compressedPath, decompressedPath := shared.ResolveReportOutputPaths(*output, defaultOutput, ".json", shouldDecompress)

				compressedSize, err := shared.WriteStreamToFile(compressedPath, reader)
				if err != nil {
					return fmt.Errorf("performance download: %w", err)
				}

				var decompressedSize int64
				if shouldDecompress {
					decompressedSize, err = shared.DecompressGzipFile(compressedPath, decompressedPath)
					if err != nil {
						return fmt.Errorf("performance download: %w", err)
					}
				}

				result := &asc.PerformanceDownloadResult{
					DownloadType:     "metrics",
					BuildID:          trimmedBuildID,
					FilePath:         compressedPath,
					FileSize:         compressedSize,
					Decompressed:     shouldDecompress,
					DecompressedPath: decompressedPath,
					DecompressedSize: decompressedSize,
				}

				return shared.PrintOutput(result, *outputFormat, *pretty)
			default:
				defaultOutput := fmt.Sprintf("perf_power_metrics_%s.json", appFlag)

				download, err := client.DownloadPerfPowerMetricsForApp(requestCtx, appFlag,
					asc.WithPerfPowerMetricsPlatforms(platforms),
					asc.WithPerfPowerMetricsMetricTypes(metricTypes),
					asc.WithPerfPowerMetricsDeviceTypes(shared.SplitCSV(*deviceType)),
				)
				if err != nil {
					return fmt.Errorf("performance download: %w", err)
				}
				defer download.Body.Close()

				reader, isGzip, err := preparePerformanceDownloadReader(download.Body, *decompress)
				if err != nil {
					return fmt.Errorf("performance download: %w", err)
				}
				shouldDecompress := *decompress && isGzip
				compressedPath, decompressedPath := shared.ResolveReportOutputPaths(*output, defaultOutput, ".json", shouldDecompress)

				compressedSize, err := shared.WriteStreamToFile(compressedPath, reader)
				if err != nil {
					return fmt.Errorf("performance download: %w", err)
				}

				var decompressedSize int64
				if shouldDecompress {
					decompressedSize, err = shared.DecompressGzipFile(compressedPath, decompressedPath)
					if err != nil {
						return fmt.Errorf("performance download: %w", err)
					}
				}

				result := &asc.PerformanceDownloadResult{
					DownloadType:     "metrics",
					AppID:            appFlag,
					FilePath:         compressedPath,
					FileSize:         compressedSize,
					Decompressed:     shouldDecompress,
					DecompressedPath: decompressedPath,
					DecompressedSize: decompressedSize,
				}

				return shared.PrintOutput(result, *outputFormat, *pretty)
			}
		},
	}
}

func preparePerformanceDownloadReader(reader io.Reader, decompress bool) (io.Reader, bool, error) {
	if !decompress {
		return reader, false, nil
	}

	buffered := bufio.NewReader(reader)
	header, err := buffered.Peek(2)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, false, err
	}
	isGzip := len(header) >= 2 && header[0] == 0x1f && header[1] == 0x8b
	return buffered, isGzip, nil
}
