package cmdtest

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestAnalyticsSalesAllowMissingReturnsStructuredResultWithoutFile(t *testing.T) {
	t.Run("no sales", func(t *testing.T) {
		assertSalesReportUnavailable(t, "There were no sales for the date specified.")
	})
	t.Run("not published yet", func(t *testing.T) {
		assertSalesReportUnavailable(t, "Report is not available yet. Daily reports for the Americas are available by 5 am Pacific Time.")
	})
}

func assertSalesReportUnavailable(t *testing.T, detail string) {
	t.Helper()
	client, requestCount := newSalesReportNotFoundTestClient(t, detail)

	outputPath := filepath.Join(t.TempDir(), "missing.tsv.gz")
	stdout, stderr, runErr := runRootWithDeps(t, client,
		"analytics", "sales",
		"--vendor", "12345678",
		"--type", "SALES",
		"--subtype", "SUMMARY",
		"--frequency", "DAILY",
		"--date", "2026-08-18",
		"--output", outputPath,
		"--output-format", "json",
		"--allow-missing",
	)
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if *requestCount != 1 {
		t.Fatalf("request count = %d, want 1", *requestCount)
	}
	if _, err := os.Stat(outputPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output file stat error = %v, want not-exist", err)
	}

	var result struct {
		VendorNumber  string `json:"vendorNumber"`
		ReportType    string `json:"reportType"`
		ReportSubType string `json:"reportSubType"`
		Frequency     string `json:"frequency"`
		ReportDate    string `json:"reportDate"`
		Version       string `json:"version"`
		Available     *bool  `json:"available"`
		FilePath      string `json:"filePath"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON output: %v; stdout=%q", err, stdout)
	}
	if result.Available == nil || *result.Available {
		t.Fatalf("available = %v, want false", result.Available)
	}
	if result.VendorNumber != "12345678" || result.ReportType != "SALES" || result.ReportSubType != "SUMMARY" || result.Frequency != "DAILY" || result.ReportDate != "2026-08-18" || result.Version != "1_0" {
		t.Fatalf("unexpected result metadata: %+v", result)
	}
	if result.FilePath != "" {
		t.Fatalf("filePath = %q, want empty", result.FilePath)
	}
}

func TestAnalyticsSalesMissingReportFailsByDefault(t *testing.T) {
	client, _ := newMissingSalesReportTestClient(t)

	outputPath := filepath.Join(t.TempDir(), "missing.tsv.gz")
	stdout, _, runErr := runRootWithDeps(t, client,
		"analytics", "sales",
		"--vendor", "12345678",
		"--type", "SALES",
		"--subtype", "SUMMARY",
		"--frequency", "DAILY",
		"--date", "2026-08-18",
		"--output", outputPath,
		"--output-format", "json",
	)
	if runErr == nil || !strings.Contains(runErr.Error(), "There were no sales for the date specified") {
		t.Fatalf("run error = %v, want missing-report API error", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if _, err := os.Stat(outputPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output file stat error = %v, want not-exist", err)
	}
}

func TestAnalyticsSalesAllowMissingDoesNotSwallowUnrelatedNotFound(t *testing.T) {
	client, _ := newSalesReportNotFoundTestClient(t, "The requested vendor account was not found.")

	stdout, _, runErr := runRootWithDeps(t, client,
		"analytics", "sales",
		"--vendor", "12345678",
		"--type", "SALES",
		"--subtype", "SUMMARY",
		"--frequency", "DAILY",
		"--date", "2026-08-18",
		"--output-format", "json",
		"--allow-missing",
	)
	if runErr == nil || !strings.Contains(runErr.Error(), "requested vendor account was not found") {
		t.Fatalf("run error = %v, want unrelated not-found error", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
}

func TestAnalyticsSalesReportFailureHints(t *testing.T) {
	const (
		noSales      = "There were no sales for the date specified."
		notPublished = "Report is not available yet. Daily reports for the Americas are available by 5 am Pacific Time."
	)
	// 2026-11-23 03:00 UTC is 2026-11-22 19:00 Pacific: Apple has published
	// the DAILY report for 2026-11-21 but not the one for 2026-11-22.
	now := time.Date(2026, 11, 23, 3, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		status    int
		detail    string
		frequency string
		date      string
		extraArgs []string
		wantCode  int
		wantHint  string
	}{
		{
			name:      "no sales in a published day",
			status:    http.StatusNotFound,
			detail:    noSales,
			frequency: "DAILY",
			date:      "2026-11-01",
			wantCode:  cmd.ExitNotFound,
			wantHint:  "Apple publishes no DAILY report for a period without sales, so 2026-11-01 had no sales (for example, it is before the first sale); count it as zero, re-running will not change it. When looping over dates, add --allow-missing to get available=false with exit 0 for such periods.",
		},
		{
			name:      "month that has not been published",
			status:    http.StatusNotFound,
			detail:    noSales,
			frequency: "MONTHLY",
			date:      "2026-11",
			extraArgs: []string{"--decompress"},
			wantCode:  cmd.ExitNotFound,
			wantHint:  "Apple has not published the MONTHLY report for 2026-11 yet: MONTHLY reports appear five days after the month ends, by about 8 a.m. Pacific, so this one should appear on 2026-12-05. The latest MONTHLY report that should be published is 2026-10: asc analytics sales --vendor 12345678 --type SALES --subtype SUMMARY --frequency MONTHLY --date 2026-10 --decompress",
		},
		{
			name:      "today has not been published",
			status:    http.StatusNotFound,
			detail:    notPublished,
			frequency: "DAILY",
			date:      "2026-11-22",
			wantCode:  cmd.ExitNotFound,
			wantHint:  "Apple has not published the DAILY report for 2026-11-22 yet: DAILY reports appear the day after, by about 8 a.m. Pacific, so this one should appear on 2026-11-23. The latest DAILY report that should be published is 2026-11-21: asc analytics sales --vendor 12345678 --type SALES --subtype SUMMARY --frequency DAILY --date 2026-11-21",
		},
		{
			name:      "week that has not ended",
			status:    http.StatusNotFound,
			detail:    noSales,
			frequency: "WEEKLY",
			date:      "2026-11-16",
			wantCode:  cmd.ExitNotFound,
			wantHint:  "Apple has not published the WEEKLY report for 2026-11-22 yet: WEEKLY reports appear on the Monday after the week, by about 8 a.m. Pacific, so this one should appear on 2026-11-23. The latest WEEKLY report that should be published is 2026-11-15: asc analytics sales --vendor 12345678 --type SALES --subtype SUMMARY --frequency WEEKLY --date 2026-11-15",
		},
		{
			name:      "year that has not ended",
			status:    http.StatusNotFound,
			detail:    noSales,
			frequency: "YEARLY",
			date:      "2026",
			wantCode:  cmd.ExitNotFound,
			wantHint:  "Apple has not published the YEARLY report for 2026 yet: YEARLY reports appear six days after the year ends, by about 8 a.m. Pacific, so this one should appear on 2027-01-06. The latest YEARLY report that should be published is 2025: asc analytics sales --vendor 12345678 --type SALES --subtype SUMMARY --frequency YEARLY --date 2025",
		},
		{
			name:      "published day that Apple is late with",
			status:    http.StatusNotFound,
			detail:    notPublished,
			frequency: "DAILY",
			date:      "2026-11-21",
			wantCode:  cmd.ExitNotFound,
			wantHint:  "Apple has not published the DAILY report for 2026-11-21 yet, although it is usually out by now; re-running will not help until it is. The period before is 2026-11-20: asc analytics sales --vendor 12345678 --type SALES --subtype SUMMARY --frequency DAILY --date 2026-11-20, or add --allow-missing to get available=false instead of an error.",
		},
		{
			name:      "unrelated not found",
			status:    http.StatusNotFound,
			detail:    "The requested vendor account was not found.",
			frequency: "DAILY",
			date:      "2026-11-01",
			wantCode:  cmd.ExitNotFound,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, _ := newSalesReportErrorTestClient(t, test.status, test.detail)
			outputPath := filepath.Join(t.TempDir(), "report.tsv.gz")
			args := []string{
				"analytics", "sales",
				"--vendor", "12345678",
				"--type", "SALES",
				"--subtype", "SUMMARY",
				"--frequency", test.frequency,
				"--date", test.date,
				"--output", outputPath,
				"--output-format", "json",
			}
			args = append(args, test.extraArgs...)

			d, stdoutBuf, stderrBuf := newTestDeps(t, client)
			d.Now = func() time.Time { return now }
			code := cmd.RunWithDeps(args, "1.2.3", d)
			stdout, stderr := stdoutBuf.String(), stderrBuf.String()
			if code != test.wantCode {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, test.wantCode, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.detail) {
				t.Fatalf("stderr = %q, want Apple's detail %q", stderr, test.detail)
			}
			if test.wantHint == "" && strings.Contains(stderr, "Hint:") {
				t.Fatalf("stderr = %q, want no hint", stderr)
			}
			if want := "Hint: " + test.wantHint + "\n"; test.wantHint != "" && (!strings.HasSuffix(stderr, want) || strings.Count(stderr, "Hint:") != 1) {
				t.Fatalf("stderr = %q, want one hint line %q", stderr, want)
			}
			if _, err := os.Stat(outputPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("output file stat error = %v, want not-exist", err)
			}
		})
	}
}

func newMissingSalesReportTestClient(t *testing.T) (func() (*asc.Client, error), *int) {
	return newSalesReportNotFoundTestClient(t, "There were no sales for the date specified.")
}

func newSalesReportNotFoundTestClient(t *testing.T, detail string) (func() (*asc.Client, error), *int) {
	t.Helper()
	return newSalesReportErrorTestClient(t, http.StatusNotFound, detail)
}

func newSalesReportErrorTestClient(t *testing.T, status int, detail string) (func() (*asc.Client, error), *int) {
	t.Helper()
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestCount++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/salesReports" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = fmt.Fprintf(w, `{"errors":[{"status":"%d","code":"%s","title":"%s","detail":"%s"}]}`, status, strings.ToUpper(strings.ReplaceAll(http.StatusText(status), " ", "_")), http.StatusText(status), detail)
	}))
	t.Cleanup(server.Close)
	return stubServerClient(t, server), &requestCount
}
