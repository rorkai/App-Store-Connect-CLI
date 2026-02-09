package shared

import "flag"

// CI report format types
const (
	ReportFormatJUnit = "junit"
)

var (
	reportFormat string
	reportFile   string
)

// BindCIFlags registers CI-related flags for report output.
// These are separate from BindRootFlags to keep CI concerns isolated.
func BindCIFlags(fs *flag.FlagSet) {
	fs.StringVar(&reportFormat, "report", "", "Report format for CI output (e.g., junit)")
	fs.StringVar(&reportFile, "report-file", "", "Path to write CI report file")
}

// ReportFormat returns the configured report format.
func ReportFormat() string {
	return reportFormat
}

// ReportFile returns the configured report file path.
func ReportFile() string {
	return reportFile
}

// SetReportFormat sets the report format (for testing).
func SetReportFormat(format string) {
	reportFormat = format
}

// SetReportFile sets the report file path (for testing).
func SetReportFile(path string) {
	reportFile = path
}
