package analytics

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

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

func TestNormalizeReportDate_MonthlyValidation(t *testing.T) {
	_, err := normalizeReportDate("2024-01-02", asc.SalesReportFrequencyMonthly)
	if err == nil {
		t.Fatal("expected error for non-first day monthly date")
	}
}

func TestNormalizeReportDate_MonthlyFormat(t *testing.T) {
	date, err := normalizeReportDate("2024-01", asc.SalesReportFrequencyMonthly)
	if err != nil {
		t.Fatalf("expected monthly date to parse, got %v", err)
	}
	if date != "2024-01" {
		t.Fatalf("expected date to be 2024-01, got %q", date)
	}
}

func TestNormalizeReportDate_YearlyFormat(t *testing.T) {
	date, err := normalizeReportDate("2024", asc.SalesReportFrequencyYearly)
	if err != nil {
		t.Fatalf("expected yearly date to parse, got %v", err)
	}
	if date != "2024" {
		t.Fatalf("expected date to be 2024, got %q", date)
	}
}

func TestNormalizeReportDate_WeeklyMondayConvertsToSunday(t *testing.T) {
	date, err := normalizeReportDate("2026-02-09", asc.SalesReportFrequencyWeekly)
	if err != nil {
		t.Fatalf("expected weekly monday date to parse, got %v", err)
	}
	if date != "2026-02-15" {
		t.Fatalf("expected weekly monday date to normalize to sunday 2026-02-15, got %q", date)
	}
}

func TestNormalizeReportDate_WeeklySundayRemainsSunday(t *testing.T) {
	date, err := normalizeReportDate("2026-02-15", asc.SalesReportFrequencyWeekly)
	if err != nil {
		t.Fatalf("expected weekly sunday date to parse, got %v", err)
	}
	if date != "2026-02-15" {
		t.Fatalf("expected weekly sunday date to remain unchanged, got %q", date)
	}
}

func TestNormalizeReportDate_WeeklyRejectsNonBoundaryDate(t *testing.T) {
	_, err := normalizeReportDate("2026-02-11", asc.SalesReportFrequencyWeekly)
	if err == nil {
		t.Fatal("expected error for weekly non-monday/sunday date")
	}
}

func TestNormalizeSalesReportVersionSupportsCurrentVersion(t *testing.T) {
	version, err := normalizeSalesReportVersion("1_4", asc.SalesReportTypeSales)
	if err != nil {
		t.Fatalf("expected version 1_4 to parse, got %v", err)
	}
	if version != asc.SalesReportVersion1_4 {
		t.Fatalf("expected version 1_4, got %q", version)
	}
}

func TestNormalizeSalesReportVersionDefaultsSubscriptionTo1_4(t *testing.T) {
	version, err := normalizeSalesReportVersion("  ", asc.SalesReportTypeSubscription)
	if err != nil {
		t.Fatalf("expected empty version to use the default, got %v", err)
	}
	if version != asc.SalesReportVersion1_4 {
		t.Fatalf("expected subscription default version 1_4, got %q", version)
	}
}

func TestNormalizeSalesReportVersionPreservesOtherDefaults(t *testing.T) {
	version, err := normalizeSalesReportVersion("  ", asc.SalesReportTypeSales)
	if err != nil {
		t.Fatalf("expected empty version to use the default, got %v", err)
	}
	if version != asc.SalesReportVersion1_0 {
		t.Fatalf("expected sales default version 1_0, got %q", version)
	}
}

func TestNormalizeSalesReportVersionRejectsInvalidValue(t *testing.T) {
	_, err := normalizeSalesReportVersion("1.4", asc.SalesReportTypeSubscription)
	if err == nil {
		t.Fatal("expected invalid version error")
	}
}

func TestNormalizeSalesReportVersionAllowsFutureVersion(t *testing.T) {
	version, err := normalizeSalesReportVersion(" 1_5 ", asc.SalesReportTypeSubscription)
	if err != nil {
		t.Fatalf("expected future version to parse, got %v", err)
	}
	if version != asc.SalesReportVersion("1_5") {
		t.Fatalf("expected version 1_5, got %q", version)
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
