package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestAnalyticsSalesDefaultsSubscriptionsToVersion1_4(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

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
		_, _ = io.WriteString(w, "report-data")
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	})
	client, err := asc.NewClientWithHTTPClient(
		os.Getenv("ASC_KEY_ID"),
		os.Getenv("ASC_ISSUER_ID"),
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("create analytics sales test client: %v", err)
	}
	restoreClient := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	})
	t.Cleanup(restoreClient)

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
