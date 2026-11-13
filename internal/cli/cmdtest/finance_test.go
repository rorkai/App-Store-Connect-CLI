package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
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
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

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

	runValidationTests(t, tests)
}

func TestFinanceRegionsCommandOutputsJSON(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"finance", "regions"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

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
	setupAuth(t)
	const detail = "The requested report was not found."
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/financeReports" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"`+detail+`"}]}`)
	}))
	outputPath := filepath.Join(t.TempDir(), "finance.tsv.gz")

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"finance", "reports", "--vendor", "12345678", "--report-type", "FINANCIAL", "--region", "US", "--date", "2026-10", "--output", outputPath}, "1.2.3")
	})
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
