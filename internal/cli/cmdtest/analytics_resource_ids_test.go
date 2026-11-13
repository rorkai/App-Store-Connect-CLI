package cmdtest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
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

	client := newReviewTestServerClient(t, server)
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

	stdout, stderr, runErr := runCommand(t, []string{
		"analytics", "instances", "links",
		"--instance-id", instanceID,
		"--output", "json",
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
	runAnalyticsDownloadIDCase(t, "r39-example-instance", "33333333-3333-3333-3333-333333333333", false)
}

func TestAnalyticsDownloadAcceptsPrefixedSegmentID(t *testing.T) {
	runAnalyticsDownloadIDCase(t, "22222222-2222-2222-2222-222222222222", "s39-example-segment", true)
}

func runAnalyticsDownloadIDCase(t *testing.T, instanceID, segmentID string, matchOnSecondPage bool) {
	t.Helper()
	setupAnalyticsResourceIDAuth(t)

	const reportPayload = "analytics-report-data"
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestCount++
		switch req.URL.Path {
		case "/v1/analyticsReportRequests/" + analyticsViewRequestID + "/reports":
			assertAnalyticsResourceIDRequest(t, req, "/v1/analyticsReportRequests/"+analyticsViewRequestID+"/reports", "limit=200", true)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"data":[{"type":"analyticsReports","id":"r39-example-report"}],"links":{}}`)
		case "/v1/analyticsReports/r39-example-report/instances":
			w.Header().Set("Content-Type", "application/json")
			const nextPage = "https://api.appstoreconnect.apple.com/v1/analyticsReports/r39-example-report/instances?cursor="
			switch req.URL.Query().Get("cursor") {
			case "unused":
				http.NotFound(w, req)
			case "match":
				assertAnalyticsResourceIDRequest(t, req, "/v1/analyticsReports/r39-example-report/instances", "cursor=match", true)
				_, _ = io.WriteString(w, `{"data":[{"type":"analyticsReportInstances","id":"`+instanceID+`"}],"links":{"next":"`+nextPage+`unused"}}`)
			default:
				assertAnalyticsResourceIDRequest(t, req, "/v1/analyticsReports/r39-example-report/instances", "limit=200", true)
				if matchOnSecondPage {
					_, _ = io.WriteString(w, `{"data":[{"type":"analyticsReportInstances","id":"another-instance"}],"links":{"next":"`+nextPage+`match"}}`)
				} else {
					_, _ = io.WriteString(w, `{"data":[{"type":"analyticsReportInstances","id":"`+instanceID+`"}],"links":{"next":"`+nextPage+`unused"}}`)
				}
			}
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

	client := newReviewTestServerClient(t, server)
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

	outputPath := filepath.Join(t.TempDir(), "analytics-report.csv.gz")
	stdout, stderr, runErr := runCommand(t, []string{
		"analytics", "download",
		"--request-id", analyticsViewRequestID,
		"--instance-id", instanceID,
		"--segment-id", segmentID,
		"--output", outputPath,
		"--output-format", "json",
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	wantRequests := 4
	if matchOnSecondPage {
		wantRequests++
	}
	if requestCount != wantRequests {
		t.Fatalf("request count = %d, want %d", requestCount, wantRequests)
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
	setupAuth(t)
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
