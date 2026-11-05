package analytics

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestAnalyticsDownloadDefaultOutputSanitizesInstanceID(t *testing.T) {
	const requestID = "11111111-1111-1111-1111-111111111111"
	tests := []struct {
		name       string
		instanceID string
		want       string
	}{
		{
			name:       "preserves prefixed ID",
			instanceID: "r39-example-instance",
			want:       "analytics_report_11111111-1111-1111-1111-111111111111_r39-example-instance.csv.gz",
		},
		{
			name:       "removes path syntax",
			instanceID: `r39/../../target\segment:part`,
			want:       "analytics_report_11111111-1111-1111-1111-111111111111_r39_.._.._target_segment_part.csv.gz",
		},
		{
			name:       "uses fallback for only path syntax",
			instanceID: `../\:`,
			want:       "analytics_report_11111111-1111-1111-1111-111111111111_instance.csv.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analyticsDownloadDefaultOutput(requestID, tt.instanceID)
			if got != tt.want {
				t.Fatalf("analyticsDownloadDefaultOutput() = %q, want %q", got, tt.want)
			}
			if filepath.Base(got) != got || strings.ContainsAny(got, `/\`) {
				t.Fatalf("analyticsDownloadDefaultOutput() returned path syntax: %q", got)
			}
		})
	}
}

func TestResolveReportOutputPaths_Decompress(t *testing.T) {
	compressed, decompressed := shared.ResolveReportOutputPaths("report.tsv.gz", "default.tsv.gz", ".tsv", true)
	if compressed != "report.tsv.gz" {
		t.Fatalf("expected compressed path report.tsv.gz, got %q", compressed)
	}
	if decompressed != "report.tsv" {
		t.Fatalf("expected decompressed path report.tsv, got %q", decompressed)
	}

	compressed, decompressed = shared.ResolveReportOutputPaths("report.tsv", "default.tsv.gz", ".tsv", true)
	if compressed != "report.tsv.gz" {
		t.Fatalf("expected compressed path report.tsv.gz, got %q", compressed)
	}
	if decompressed != "report.tsv" {
		t.Fatalf("expected decompressed path report.tsv, got %q", decompressed)
	}

	compressed, decompressed = shared.ResolveReportOutputPaths("report", "default.tsv.gz", ".tsv", true)
	if compressed != "report" {
		t.Fatalf("expected compressed path report, got %q", compressed)
	}
	if decompressed != "report.tsv" {
		t.Fatalf("expected decompressed path report.tsv, got %q", decompressed)
	}
}

func TestNormalizeReportDate(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		frequency asc.SalesReportFrequency
		want      string
		wantErr   bool
	}{
		{name: "daily omitted", value: "", frequency: asc.SalesReportFrequencyDaily, want: ""},
		{name: "monthly key", value: "2024-01", frequency: asc.SalesReportFrequencyMonthly, want: "2024-01"},
		{name: "monthly full date", value: "2024-02-15", frequency: asc.SalesReportFrequencyMonthly, want: "2024-02"},
		{name: "monthly leap day", value: "2024-02-29", frequency: asc.SalesReportFrequencyMonthly, want: "2024-02"},
		{name: "monthly malformed", value: "2024/01/02", frequency: asc.SalesReportFrequencyMonthly, wantErr: true},
		{name: "yearly key", value: "2024", frequency: asc.SalesReportFrequencyYearly, want: "2024"},
		{name: "yearly mid-year date", value: "2024-06-30", frequency: asc.SalesReportFrequencyYearly, want: "2024"},
		{name: "yearly last day", value: "2024-12-31", frequency: asc.SalesReportFrequencyYearly, want: "2024"},
		{name: "weekly monday becomes sunday", value: "2026-02-09", frequency: asc.SalesReportFrequencyWeekly, want: "2026-02-15"},
		{name: "weekly sunday unchanged", value: "2026-02-15", frequency: asc.SalesReportFrequencyWeekly, want: "2026-02-15"},
		{name: "weekly midweek rejected", value: "2026-02-11", frequency: asc.SalesReportFrequencyWeekly, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeReportDate(test.value, test.frequency)
			if test.wantErr {
				if err == nil {
					t.Fatalf("normalizeReportDate(%q, %s) = %q, want error", test.value, test.frequency, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeReportDate(%q, %s) error: %v", test.value, test.frequency, err)
			}
			if got != test.want {
				t.Fatalf("normalizeReportDate(%q, %s) = %q, want %q", test.value, test.frequency, got, test.want)
			}
		})
	}
}

func TestNormalizeSalesReportTypeSupportsDocumentedValues(t *testing.T) {
	for _, value := range []string{
		"SUBSCRIBER",
		"SUBSCRIPTION_OFFER_CODE_REDEMPTION",
		"INSTALLS",
		"FIRST_ANNUAL",
		"WIN_BACK_ELIGIBILITY",
	} {
		t.Run(value, func(t *testing.T) {
			got, err := normalizeSalesReportType(value)
			if err != nil {
				t.Fatalf("expected %s to be accepted, got %v", value, err)
			}
			if string(got) != value {
				t.Fatalf("expected %s, got %q", value, got)
			}
		})
	}
}

func TestNormalizeSalesReportSubTypeSupportsDocumentedValues(t *testing.T) {
	for _, value := range []string{
		"SUMMARY_INSTALL_TYPE",
		"SUMMARY_TERRITORY",
		"SUMMARY_CHANNEL",
	} {
		t.Run(value, func(t *testing.T) {
			got, err := normalizeSalesReportSubType(value)
			if err != nil {
				t.Fatalf("expected %s to be accepted, got %v", value, err)
			}
			if string(got) != value {
				t.Fatalf("expected %s, got %q", value, got)
			}
		})
	}
}

func TestNormalizeSalesReportVersion(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		reportType asc.SalesReportType
		subType    asc.SalesReportSubType
		frequency  asc.SalesReportFrequency
		want       asc.SalesReportVersion
		wantErr    bool
	}{
		{name: "subscription defaults to 1_4", value: "  ", reportType: asc.SalesReportTypeSubscription, subType: asc.SalesReportSubTypeSummary, frequency: asc.SalesReportFrequencyDaily, want: asc.SalesReportVersion1_4},
		{name: "sales defaults to 1_0", value: "  ", reportType: asc.SalesReportTypeSales, subType: asc.SalesReportSubTypeSummary, frequency: asc.SalesReportFrequencyDaily, want: asc.SalesReportVersion1_0},
		{name: "subscriber defaults to 1_3", reportType: asc.SalesReportTypeSubscriber, subType: asc.SalesReportSubTypeDetailed, frequency: asc.SalesReportFrequencyDaily, want: asc.SalesReportVersion1_3},
		{name: "monthly installs defaults to 1_2", reportType: asc.SalesReportTypeInstalls, subType: asc.SalesReportSubTypeSummary, frequency: asc.SalesReportFrequencyMonthly, want: asc.SalesReportVersion1_2},
		{name: "version outside tuple", value: "1_4", reportType: asc.SalesReportTypeSales, subType: asc.SalesReportSubTypeSummary, frequency: asc.SalesReportFrequencyDaily, wantErr: true},
		{name: "dotted version", value: "1.4", reportType: asc.SalesReportTypeSubscription, subType: asc.SalesReportSubTypeSummary, frequency: asc.SalesReportFrequencyDaily, wantErr: true},
		{name: "undocumented future version", value: " 1_5 ", reportType: asc.SalesReportTypeSubscription, subType: asc.SalesReportSubTypeSummary, frequency: asc.SalesReportFrequencyDaily, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeSalesReportVersion(test.value, test.reportType, test.subType, test.frequency)
			if test.wantErr {
				if err == nil {
					t.Fatalf("normalizeSalesReportVersion(%q) = %q, want error", test.value, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeSalesReportVersion(%q) error: %v", test.value, err)
			}
			if got != test.want {
				t.Fatalf("normalizeSalesReportVersion(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestValidateSalesReportTupleRejectsUnsupportedCombination(t *testing.T) {
	err := validateSalesReportTuple(
		asc.SalesReportTypeWinBackEligibility,
		asc.SalesReportSubTypeDetailed,
		asc.SalesReportFrequencyWeekly,
	)
	if err == nil {
		t.Fatal("expected unsupported WIN_BACK_ELIGIBILITY/DETAILED/WEEKLY tuple to be rejected")
	}
}

func TestDecompressGzipFile(t *testing.T) {
	tempDir := t.TempDir()
	source := filepath.Join(tempDir, "source.tsv.gz")
	dest := filepath.Join(tempDir, "dest.tsv")

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte("hello")); err != nil {
		t.Fatalf("failed to write gzip: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("failed to close gzip: %v", err)
	}
	if err := os.WriteFile(source, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("failed to write source gzip: %v", err)
	}

	size, err := shared.DecompressGzipFile(source, dest)
	if err != nil {
		t.Fatalf("decompressGzipFile() error: %v", err)
	}
	if size == 0 {
		t.Fatalf("expected non-zero decompressed size")
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("failed to read dest file: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("expected decompressed content to be hello, got %q", string(data))
	}
}
