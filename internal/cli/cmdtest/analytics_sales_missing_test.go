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
	const hint = "Hint: Re-running the same request will not help until Apple has a report for it; try an earlier --date, or pass --allow-missing to get available=false instead of an error.\n"
	tests := []struct {
		name     string
		status   int
		detail   string
		wantCode int
		wantHint string
	}{
		{
			name:     "report not available yet",
			status:   http.StatusNotFound,
			detail:   "Report is not available yet. Daily reports for the Americas are available by 5 am Pacific Time.",
			wantCode: cmd.ExitNotFound,
			wantHint: hint,
		},
		{
			name:     "no sales",
			status:   http.StatusNotFound,
			detail:   "There were no sales for the date specified.",
			wantCode: cmd.ExitNotFound,
			wantHint: hint,
		},
		{
			name:     "unrelated not found",
			status:   http.StatusNotFound,
			detail:   "The requested vendor account was not found.",
			wantCode: cmd.ExitNotFound,
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
				"--frequency", "DAILY",
				"--date", "2026-08-18",
				"--output", outputPath,
				"--output-format", "json",
			}

			stdout, stderr, code := runWithDeps(t, client, args...)
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
			if test.wantHint != "" && (!strings.HasSuffix(stderr, test.wantHint) || strings.Count(stderr, "Hint:") != 1) {
				t.Fatalf("stderr = %q, want one hint line %q", stderr, test.wantHint)
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
