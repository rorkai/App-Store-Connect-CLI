package cmdtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestAnalyticsInstancesLinksAcceptsPrefixedInstanceID(t *testing.T) {
	setupAnalyticsResourceIDAuth(t)

	const (
		instanceID = "r39-example-instance"
		segmentID  = "s39-example-segment"
	)
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestCount++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/analyticsReportInstances/"+instanceID+"/relationships/segments" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if req.URL.RawQuery != "" {
			t.Fatalf("unexpected query: %q", req.URL.RawQuery)
		}
		if req.Header.Get("Authorization") == "" {
			t.Fatal("expected Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"analyticsReportSegments","id":"`+segmentID+`"}],"links":{}}`)
	}))
	t.Cleanup(server.Close)

	stdout, stderr, runErr := runRootWithDeps(t, stubServerClient(t, server),
		"analytics", "instances", "links",
		"--instance-id", instanceID,
		"--output", "json",
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
	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("parse output: %v\nstdout=%s", err, stdout)
	}
	if len(response.Data) != 1 || response.Data[0].ID != segmentID {
		t.Fatalf("unexpected output: %s", stdout)
	}
}

func TestAnalyticsDownloadAcceptsPrefixedInstanceID(t *testing.T) {
	runAnalyticsDownloadIDCase(t, "r39-example-instance", "33333333-3333-3333-3333-333333333333")
}

func TestAnalyticsDownloadAcceptsPrefixedSegmentID(t *testing.T) {
	runAnalyticsDownloadIDCase(t, "22222222-2222-2222-2222-222222222222", "s39-example-segment")
}

func TestAnalyticsDownloadDefaultPathsSeparateExplicitSegments(t *testing.T) {
	setupAnalyticsResourceIDAuth(t)
	t.Chdir(t.TempDir())

	const (
		instanceID    = "r39-example-instance"
		firstSegment  = "s39:Segment|A"
		secondSegment = "s39:segment|a"
		firstReport   = "first-segment-data"
		secondReport  = "second-segment-data"
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v1/analyticsReportInstances/" + instanceID + "/segments":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"data":[{"type":"analyticsReportSegments","id":%q,"attributes":{"url":"https://mzstatic.com/first.gz"}},{"type":"analyticsReportSegments","id":%q,"attributes":{"url":"https://mzstatic.com/second.gz"}}]}`, firstSegment, secondSegment)
		case "/first.gz":
			_, _ = io.WriteString(w, firstReport)
		case "/second.gz":
			_, _ = io.WriteString(w, secondReport)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL)
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(server.Close)
	client := stubServerClient(t, server)
	download := func(segmentID string) (string, string, int) {
		return runWithDeps(t, client, "analytics", "download", "--request-id", analyticsViewRequestID,
			"--instance-id", instanceID, "--segment-id", segmentID, "--output-format", "json")
	}
	readReport := func(segmentID, want string) string {
		t.Helper()
		stdout, stderr, code := download(segmentID)
		if code != rootcmd.ExitSuccess || stderr != "" {
			t.Fatalf("download %q exited %d: %s", segmentID, code, stderr)
		}
		var result asc.AnalyticsReportDownloadResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("decode download result: %v", err)
		}
		if result.SegmentID != segmentID || !strings.HasSuffix(result.FilePath, ".csv.gz") || strings.ContainsAny(result.FilePath, `/\:*?"<>|`) {
			t.Fatalf("unexpected download result: %+v", result)
		}
		data, err := os.ReadFile(result.FilePath)
		if err != nil || string(data) != want {
			t.Fatalf("read %q: got %q, %v; want %q", result.FilePath, data, err, want)
		}
		return result.FilePath
	}
	firstPath := readReport(firstSegment, firstReport)
	secondPath := readReport(secondSegment, secondReport)
	if strings.EqualFold(firstPath, secondPath) {
		t.Fatalf("segment output paths collide: %q and %q", firstPath, secondPath)
	}
	const savedReport = "saved first report"
	if err := os.WriteFile(firstPath, []byte(savedReport), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := download(firstSegment)
	if code != rootcmd.ExitError || stdout != "" || !strings.Contains(stderr, "output file already exists") {
		t.Fatalf("retry exited %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if data, err := os.ReadFile(firstPath); err != nil || string(data) != savedReport {
		t.Fatalf("retry changed %q: got %q, %v; want %q", firstPath, data, err, savedReport)
	}
}

func runAnalyticsDownloadIDCase(t *testing.T, instanceID, segmentID string) {
	t.Helper()
	setupAnalyticsResourceIDAuth(t)

	const reportPayload = "analytics-report-data"
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestCount++
		switch req.URL.Path {
		case "/v1/analyticsReportInstances/" + instanceID + "/segments":
			assertAnalyticsResourceIDRequest(t, req, "/v1/analyticsReportInstances/"+instanceID+"/segments", "limit=200", true)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"data":[{"type":"analyticsReportSegments","id":"`+segmentID+`","attributes":{"url":"https://mzstatic.com/report.gz"}}],"links":{}}`)
		case "/report.gz":
			assertAnalyticsResourceIDRequest(t, req, "/report.gz", "", false)
			w.Header().Set("Content-Type", "application/a-gzip")
			_, _ = io.WriteString(w, reportPayload)
		default:
			t.Errorf("unexpected extra request: %s %s", req.Method, req.URL)
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(server.Close)

	outputPath := filepath.Join(t.TempDir(), "analytics-report.csv.gz")
	stdout, stderr, runErr := runRootWithDeps(t, stubServerClient(t, server),
		"analytics", "download",
		"--request-id", analyticsViewRequestID,
		"--instance-id", instanceID,
		"--segment-id", segmentID,
		"--output", outputPath,
		"--output-format", "json",
	)
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 2 {
		t.Fatalf("request count = %d, want 2 (segments, then the report file)", requestCount)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !bytes.Equal(data, []byte(reportPayload)) {
		t.Fatalf("report data = %q, want %q", data, reportPayload)
	}
	var result struct {
		RequestID  string `json:"requestId"`
		InstanceID string `json:"instanceId"`
		SegmentID  string `json:"segmentId"`
		FilePath   string `json:"filePath"`
		FileSize   int64  `json:"fileSize"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse output: %v\nstdout=%s", err, stdout)
	}
	if result.RequestID != analyticsViewRequestID || result.InstanceID != instanceID || result.SegmentID != segmentID || result.FilePath != outputPath || result.FileSize != int64(len(reportPayload)) {
		t.Fatalf("unexpected output: %+v", result)
	}
}

func setupAnalyticsResourceIDAuth(t *testing.T) {
	t.Helper()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
}

func assertAnalyticsResourceIDRequest(t *testing.T, req *http.Request, wantPath, wantQuery string, wantAuth bool) {
	t.Helper()
	if req.Method != http.MethodGet || req.URL.Path != wantPath {
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
	}
	if req.URL.RawQuery != wantQuery {
		t.Fatalf("query = %q, want %q", req.URL.RawQuery, wantQuery)
	}
	if got := req.Header.Get("Authorization"); wantAuth && got == "" {
		t.Fatal("expected Authorization header")
	} else if !wantAuth && got != "" {
		t.Fatalf("unexpected Authorization header on report download: %q", got)
	}
}
