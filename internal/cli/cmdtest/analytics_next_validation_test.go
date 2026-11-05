package cmdtest

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestAnalyticsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"analytics", "requests"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/analyticsReportRequests?cursor=AQ&limit=200",
			dataType: "analyticsReportRequests",
		},
		{
			args:     []string{"analytics", "instances", "links"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/analyticsReportInstances/instance-1/relationships/segments?cursor=AQ&limit=200",
			dataType: "analyticsReportSegments",
		},
		{
			args:     []string{"analytics", "reports", "links"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/analyticsReports/report-1/relationships/instances?cursor=AQ&limit=200",
			dataType: "analyticsReportInstances",
		},
	})
}

func TestAnalyticsReportsRelationshipsAcceptsPrefixedReportID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const reportID = "r39-11111111-1111-1111-1111-111111111111"
	const expectedPath = "/v1/analyticsReports/" + reportID + "/relationships/instances"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != expectedPath {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		body := `{"data":[{"type":"analyticsReportInstances","id":"inst-1"}],"links":{}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"analytics", "reports", "links", "--report-id", reportID}); err != nil {
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
	if !strings.Contains(stdout, `"id":"inst-1"`) {
		t.Fatalf("expected output to contain instance ID, got %q", stdout)
	}
}

func TestAnalyticsViewPaginateFromNextFollowsReportPages(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const (
		firstReportsURL    = "https://api.appstoreconnect.apple.com/v1/analyticsReportRequests/request-1/reports?cursor=AQ&limit=200"
		secondReportsURL   = "https://api.appstoreconnect.apple.com/v1/analyticsReportRequests/request-1/reports?cursor=BQ&limit=200"
		firstInstancesURL  = "https://api.appstoreconnect.apple.com/v1/analyticsReports/analytics-report-next-1/instances?limit=200"
		secondInstancesURL = "https://api.appstoreconnect.apple.com/v1/analyticsReports/analytics-report-next-2/instances?limit=200"
	)

	bodies := map[string]string{
		firstReportsURL:    `{"data":[{"type":"analyticsReports","id":"analytics-report-next-1","attributes":{"name":"Retention","category":"APP_USAGE","granularity":"DAILY"}}],"links":{"first":"https://api.appstoreconnect.apple.com/v1/analyticsReportRequests/request-1/reports","prev":"https://api.appstoreconnect.apple.com/v1/analyticsReportRequests/request-1/reports?cursor=9w","next":"` + secondReportsURL + `"}}`,
		secondReportsURL:   `{"data":[{"type":"analyticsReports","id":"analytics-report-next-2","attributes":{"name":"Acquisition","category":"APP_STORE","granularity":"DAILY"}}],"links":{"next":""}}`,
		firstInstancesURL:  `{"data":[{"type":"analyticsReportInstances","id":"analytics-instance-next-1","attributes":{"reportDate":"2024-01-01","processingDate":"2024-01-02T00:00:00Z","granularity":"DAILY","version":"1"}}],"links":{"next":""}}`,
		secondInstancesURL: `{"data":[{"type":"analyticsReportInstances","id":"analytics-instance-next-2","attributes":{"reportDate":"2024-01-03","processingDate":"2024-01-04T00:00:00Z","granularity":"DAILY","version":"1"}}],"links":{"next":""}}`,
	}

	var requestMu sync.Mutex
	requestCount := 0
	seen := make(map[string]int, len(bodies))
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("unexpected method %s", req.Method)
		}
		body, ok := bodies[req.URL.String()]
		if !ok {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		requestMu.Lock()
		requestCount++
		seen[req.URL.String()]++
		requestMu.Unlock()
		return analyticsViewJSONResponse(body), nil
	})
	client, err := asc.NewClientWithHTTPClient(
		"TEST_KEY",
		"TEST_ISSUER",
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("create analytics pagination test client: %v", err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"analytics", "view", "--paginate", "--next", firstReportsURL}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 4 {
		t.Fatalf("request count = %d, want 4", requestCount)
	}
	for url, count := range seen {
		if count != 1 {
			t.Fatalf("%s fetched %d times, want 1", url, count)
		}
	}
	for _, link := range []string{
		`"first":"https://api.appstoreconnect.apple.com/v1/analyticsReportRequests/request-1/reports"`,
		`"prev":"https://api.appstoreconnect.apple.com/v1/analyticsReportRequests/request-1/reports?cursor=9w"`,
	} {
		if !strings.Contains(stdout, link) {
			t.Fatalf("expected output to contain %q, got %q", link, stdout)
		}
	}
	for _, id := range []string{
		"analytics-report-next-1",
		"analytics-report-next-2",
		"analytics-instance-next-1",
		"analytics-instance-next-2",
	} {
		if !strings.Contains(stdout, `"id":"`+id+`"`) {
			t.Fatalf("expected output to contain %q, got %q", id, stdout)
		}
	}
}
