package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyticsSalesDefaultsSubscriptionsToVersion1_4(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/salesReports" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		query := req.URL.Query()
		if got := query.Get("filter[reportType]"); got != "SUBSCRIPTION" {
			t.Fatalf("filter[reportType] = %q, want SUBSCRIPTION", got)
		}
		if got := query.Get("filter[reportSubType]"); got != "SUMMARY" {
			t.Fatalf("filter[reportSubType] = %q, want SUMMARY", got)
		}
		if got := query.Get("filter[frequency]"); got != "DAILY" {
			t.Fatalf("filter[frequency] = %q, want DAILY", got)
		}
		if got := query.Get("filter[reportDate]"); got != "2026-07-30" {
			t.Fatalf("filter[reportDate] = %q, want 2026-07-30", got)
		}
		if got := query.Get("filter[version]"); got != "1_4" {
			t.Fatalf("filter[version] = %q, want 1_4", got)
		}
		if req.Header.Get("Authorization") == "" {
			t.Fatal("expected Authorization header")
		}
		return &http.Response{
			StatusCode:    http.StatusOK,
			Body:          io.NopCloser(strings.NewReader("report-data")),
			ContentLength: int64(len("report-data")),
			Header:        http.Header{"Content-Type": []string{"application/a-gzip"}},
		}, nil
	})

	outputPath := filepath.Join(t.TempDir(), "subscription.tsv.gz")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"analytics", "sales",
			"--vendor", "12345678",
			"--type", "SUBSCRIPTION",
			"--subtype", "SUMMARY",
			"--frequency", "DAILY",
			"--date", "2026-07-30",
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
	if got := string(data); got != "report-data" {
		t.Fatalf("report contents = %q, want report-data", got)
	}

	var result struct {
		Version  string `json:"version"`
		FilePath string `json:"filePath"`
		FileSize int64  `json:"fileSize"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON output: %v\nstdout=%s", err, stdout)
	}
	if result.Version != "1_4" || result.FilePath != outputPath || result.FileSize != int64(len("report-data")) {
		t.Fatalf("unexpected result: %+v", result)
	}
}
