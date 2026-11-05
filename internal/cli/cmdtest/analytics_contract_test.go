package cmdtest

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestAnalyticsRequestsUsesAccessTypeFilter(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestCount++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/analyticsReportRequests" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if got := req.URL.Query().Get("filter[accessType]"); got != "ONGOING" {
			t.Fatalf("filter[accessType] = %q, want ONGOING", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"analyticsReportRequests","id":"request-1","attributes":{"accessType":"ONGOING","stoppedDueToInactivity":false}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/analyticsReportRequests"}}`)
	}))
	t.Cleanup(server.Close)
	client := newReviewTestServerClient(t, server)
	restoreClient := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil })
	t.Cleanup(restoreClient)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"analytics", "requests",
			"--app", "app-1",
			"--access-type", "ongoing",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want 1", requestCount)
	}
	var response struct {
		Data []struct {
			Attributes struct {
				AccessType             string `json:"accessType"`
				StoppedDueToInactivity *bool  `json:"stoppedDueToInactivity"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].Attributes.AccessType != "ONGOING" {
		t.Fatalf("unexpected output: %s", stdout)
	}
	if response.Data[0].Attributes.StoppedDueToInactivity == nil || *response.Data[0].Attributes.StoppedDueToInactivity {
		t.Fatalf("expected explicit stoppedDueToInactivity=false, got: %s", stdout)
	}
}

func TestAnalyticsSalesSupportsNewTypeWithoutDailyDate(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	var payload bytes.Buffer
	writer := gzip.NewWriter(&payload)
	if _, err := writer.Write([]byte("report-data")); err != nil {
		t.Fatalf("write gzip: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestCount++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/salesReports" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		query := req.URL.Query()
		if got := query.Get("filter[reportType]"); got != "WIN_BACK_ELIGIBILITY" {
			t.Fatalf("filter[reportType] = %q, want WIN_BACK_ELIGIBILITY", got)
		}
		if got := query.Get("filter[reportSubType]"); got != "SUMMARY" {
			t.Fatalf("filter[reportSubType] = %q, want SUMMARY", got)
		}
		if query.Has("filter[reportDate]") {
			t.Fatalf("unexpected filter[reportDate]: %s", req.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/a-gzip")
		_, _ = w.Write(payload.Bytes())
	}))
	t.Cleanup(server.Close)
	client := newReviewTestServerClient(t, server)
	restoreClient := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil })
	t.Cleanup(restoreClient)

	outputPath := filepath.Join(t.TempDir(), "report.tsv.gz")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"analytics", "sales",
			"--vendor", "12345678",
			"--type", "WIN_BACK_ELIGIBILITY",
			"--subtype", "SUMMARY",
			"--frequency", "DAILY",
			"--output", outputPath,
			"--output-format", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want 1", requestCount)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !bytes.Equal(data, payload.Bytes()) {
		t.Fatal("downloaded report does not match response payload")
	}
	var result struct {
		ReportDate string `json:"reportDate"`
		FilePath   string `json:"filePath"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if result.ReportDate != "" || result.FilePath != outputPath {
		t.Fatalf("unexpected output: %s", stdout)
	}
}

func TestAnalyticsSalesUsesLivePeriodIdentifiers(t *testing.T) {
	tests := []struct {
		name      string
		frequency string
		date      string
		wantDate  string
	}{
		{name: "monthly period", frequency: "MONTHLY", date: "2024-02", wantDate: "2024-02"},
		{name: "monthly mid-period", frequency: "MONTHLY", date: "2024-02-15", wantDate: "2024-02"},
		{name: "monthly boundary", frequency: "MONTHLY", date: "2024-02-29", wantDate: "2024-02"},
		{name: "yearly period", frequency: "YEARLY", date: "2024", wantDate: "2024"},
		{name: "yearly mid-period", frequency: "YEARLY", date: "2024-06-30", wantDate: "2024"},
		{name: "yearly boundary", frequency: "YEARLY", date: "2024-12-31", wantDate: "2024"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			var payload bytes.Buffer
			writer := gzip.NewWriter(&payload)
			if _, err := writer.Write([]byte("report-data")); err != nil {
				t.Fatalf("write gzip: %v", err)
			}
			if err := writer.Close(); err != nil {
				t.Fatalf("close gzip: %v", err)
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodGet || req.URL.Path != "/v1/salesReports" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				}
				if got := req.URL.Query().Get("filter[reportDate]"); got != test.wantDate {
					t.Fatalf("filter[reportDate] = %q, want %q", got, test.wantDate)
				}
				w.Header().Set("Content-Type", "application/a-gzip")
				_, _ = w.Write(payload.Bytes())
			}))
			t.Cleanup(server.Close)

			client := newReviewTestServerClient(t, server)
			restoreClient := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil })
			t.Cleanup(restoreClient)

			outputPath := filepath.Join(t.TempDir(), "report.tsv.gz")
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"analytics", "sales",
					"--vendor", "12345678",
					"--type", "SALES",
					"--subtype", "SUMMARY",
					"--frequency", test.frequency,
					"--date", test.date,
					"--output", outputPath,
					"--output-format", "json",
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})
			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var result struct {
				ReportDate string `json:"reportDate"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("parse output: %v", err)
			}
			if result.ReportDate != test.wantDate {
				t.Fatalf("reportDate = %q, want %q", result.ReportDate, test.wantDate)
			}
		})
	}
}

func TestAnalyticsSalesRejectsMalformedPeriodDateBeforeHTTP(t *testing.T) {
	tests := []struct {
		name      string
		frequency string
		date      string
		wantErr   string
	}{
		{
			name:      "monthly",
			frequency: "MONTHLY",
			date:      "2024-02-30",
			wantErr:   "--date must be in YYYY-MM or YYYY-MM-DD format for monthly reports",
		},
		{
			name:      "yearly",
			frequency: "YEARLY",
			date:      "2024/12/31",
			wantErr:   "--date must be in YYYY or YYYY-MM-DD format for yearly reports",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalls := 0
			restoreClient := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientFactoryCalls++
				return nil, errors.New("client construction should not be attempted")
			})
			t.Cleanup(restoreClient)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"analytics", "sales",
					"--vendor", "12345678",
					"--type", "SALES",
					"--subtype", "SUMMARY",
					"--frequency", test.frequency,
					"--date", test.date,
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected usage error, got %v", runErr)
			}
			if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", got, rootcmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !bytes.Contains([]byte(stderr), []byte(test.wantErr)) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
			if clientFactoryCalls != 0 {
				t.Fatalf("client factory calls = %d, want 0", clientFactoryCalls)
			}
		})
	}
}

func TestAnalyticsSalesWarnsThatInstallsIsDeprecated(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestCount++
		if got := req.URL.Query().Get("filter[reportType]"); got != "INSTALLS" {
			t.Fatalf("filter[reportType] = %q, want INSTALLS", got)
		}
		w.Header().Set("Content-Type", "application/a-gzip")
		writer := gzip.NewWriter(w)
		_, _ = writer.Write([]byte("report-data"))
		_ = writer.Close()
	}))
	t.Cleanup(server.Close)
	client := newReviewTestServerClient(t, server)
	restoreClient := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil })
	t.Cleanup(restoreClient)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"analytics", "sales",
			"--vendor", "12345678",
			"--type", "INSTALLS",
			"--subtype", "SUMMARY",
			"--frequency", "MONTHLY",
			"--date", "2026-09",
			"--output", filepath.Join(t.TempDir(), "installs.tsv.gz"),
			"--output-format", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want 1", requestCount)
	}
	want := "Warning: the INSTALLS report type is deprecated. Apple stopped generating new INSTALLS reports on November 4, 2026; reports generated before then can still be downloaded.\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}
