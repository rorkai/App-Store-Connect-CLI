package cmdtest

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyticsSalesDefaultsSubscriptionsToVersion1_4(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	var reportPayload bytes.Buffer
	gzipWriter := gzip.NewWriter(&reportPayload)
	if _, err := gzipWriter.Write([]byte("report-data")); err != nil {
		t.Fatalf("create report payload: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close report payload: %v", err)
	}

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
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
		w.Header().Set("Content-Type", "application/a-gzip")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(reportPayload.Bytes())
	}))
	t.Cleanup(server.Close)

	outputPath := filepath.Join(t.TempDir(), "subscription.tsv.gz")
	stdout, stderr, runErr := runRootWithDeps(t, stubServerClient(t, server),
		"analytics", "sales",
		"--vendor", "12345678",
		"--type", "SUBSCRIPTION",
		"--subtype", "SUMMARY",
		"--frequency", "DAILY",
		"--date", "2026-07-30",
		"--output", outputPath,
		"--output-format", "json",
	)
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
	if !bytes.Equal(data, reportPayload.Bytes()) {
		t.Fatal("downloaded report does not match the gzip response payload")
	}
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open downloaded report: %v", err)
	}
	uncompressed, err := io.ReadAll(gzipReader)
	if err != nil {
		t.Fatalf("read downloaded report: %v", err)
	}
	if err := gzipReader.Close(); err != nil {
		t.Fatalf("close downloaded report: %v", err)
	}
	if got := string(uncompressed); got != "report-data" {
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
	if result.Version != "1_4" || result.FilePath != outputPath || result.FileSize != int64(reportPayload.Len()) {
		t.Fatalf("unexpected result: %+v", result)
	}
}
