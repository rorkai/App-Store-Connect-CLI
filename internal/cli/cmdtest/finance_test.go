package cmdtest

import (
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestFinanceReportsValidationErrors(t *testing.T) {
	t.Setenv("ASC_VENDOR_NUMBER", "")
	t.Setenv("ASC_ANALYTICS_VENDOR_NUMBER", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing vendor",
			args:    []string{"finance", "reports", "--report-type", "FINANCIAL", "--region", "US", "--date", "2025-12"},
			wantErr: "--vendor is required",
		},
		{
			name:    "missing report type",
			args:    []string{"finance", "reports", "--vendor", "12345678", "--region", "US", "--date", "2025-12"},
			wantErr: "--report-type is required",
		},
		{
			name:    "missing region",
			args:    []string{"finance", "reports", "--vendor", "12345678", "--report-type", "FINANCIAL", "--date", "2025-12"},
			wantErr: "--region is required",
		},
		{
			name:    "missing date",
			args:    []string{"finance", "reports", "--vendor", "12345678", "--report-type", "FINANCIAL", "--region", "US"},
			wantErr: "--date is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, err := runRootWithDeps(t, nil, test.args...)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", err)
			}

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestFinanceReportsInvalidFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "invalid report type",
			args:    []string{"finance", "reports", "--vendor", "12345678", "--report-type", "INVALID", "--region", "US", "--date", "2025-12"},
			wantErr: "Error: finance reports: --report-type must be FINANCIAL or FINANCE_DETAIL",
		},
		{
			name:    "invalid date format",
			args:    []string{"finance", "reports", "--vendor", "12345678", "--report-type", "FINANCIAL", "--region", "US", "--date", "2025-13"},
			wantErr: "Error: finance reports: --date must be in YYYY-MM format",
		},
		{
			name:    "invalid finance detail region",
			args:    []string{"finance", "reports", "--vendor", "12345678", "--report-type", "FINANCE_DETAIL", "--region", "US", "--date", "2025-12"},
			wantErr: "Error: finance reports: --region must be Z1 for FINANCE_DETAIL reports",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExitWithDeps(t, nil, test.args, test.wantErr)
		})
	}
}

func TestFinanceRegionsCommandOutputsJSON(t *testing.T) {
	stdout, stderr, err := runRootWithDeps(t, nil, "finance", "regions")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Regions []struct {
			RegionCode string `json:"regionCode"`
		} `json:"regions"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v", err)
	}
	if len(payload.Regions) == 0 {
		t.Fatal("expected regions in output")
	}
	found := false
	for _, region := range payload.Regions {
		if region.RegionCode == "Z1" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected region Z1 in output")
	}
}

func TestFinanceReportsNotFoundExplainsReportAvailability(t *testing.T) {
	const detail = "The requested report was not found."
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/financeReports", "filter[vendorNumber]=12345678", "filter[reportType]=FINANCIAL", "filter[regionCode]=US", "filter[reportDate]=2026-10"),
		asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"`+detail+`"}]}`))
	outputPath := filepath.Join(t.TempDir(), "finance.tsv.gz")

	stdout, stderr, code := runWithDeps(t, stubClient(t, reg), "finance", "reports", "--vendor", "12345678", "--report-type", "FINANCIAL", "--region", "US", "--date", "2026-10", "--output", outputPath)
	if code != rootcmd.ExitNotFound {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, rootcmd.ExitNotFound, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	const hint = "Hint: Apple publishes a financial report only after the fiscal month closes (by the first Friday of the next fiscal month) and only for regions with purchases or refunds that month. Check --date and --vendor; for FINANCIAL, try --region ZZ or a code from `asc finance regions`.\n"
	if !strings.Contains(stderr, detail) || !strings.HasSuffix(stderr, hint) {
		t.Fatalf("stderr = %q, want Apple's detail and hint %q", stderr, hint)
	}
	if _, err := os.Stat(outputPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output file stat error = %v, want not-exist", err)
	}
}
