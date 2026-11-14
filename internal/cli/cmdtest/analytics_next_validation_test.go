package cmdtest

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestAnalyticsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"analytics", "requests"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/123456789/analyticsReportRequests?cursor=AQ&limit=200",
			dataType: "analyticsReportRequests",
		},
		{
			args:     []string{"analytics", "requests", "--app", "123456789"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/123456789/analyticsReportRequests?cursor=AQ&limit=200",
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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET(expectedPath), asctest.JSON(http.StatusOK, `{"data":[{"type":"analyticsReportInstances","id":"inst-1"}],"links":{}}`))

	stdout, stderr, runErr := runCommand(t, []string{"analytics", "reports", "links", "--report-id", reportID})

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
	t.Run("cursor only", func(t *testing.T) {
		testAnalyticsViewPaginateFromNext(t)
	})
	t.Run("with matching request id", func(t *testing.T) {
		testAnalyticsViewPaginateFromNext(t, "--request-id", analyticsUsageExitRequestID)
	})
}

func testAnalyticsViewPaginateFromNext(t *testing.T, requestArgs ...string) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const (
		reportsURL         = "https://api.appstoreconnect.apple.com/v1/analyticsReportRequests/" + analyticsUsageExitRequestID + "/reports"
		firstReportsURL    = reportsURL + "?cursor=AQ&limit=200"
		secondReportsURL   = reportsURL + "?cursor=BQ&limit=200"
		firstInstancesURL  = "https://api.appstoreconnect.apple.com/v1/analyticsReports/analytics-report-next-1/instances?limit=200"
		secondInstancesURL = "https://api.appstoreconnect.apple.com/v1/analyticsReports/analytics-report-next-2/instances?limit=200"
	)

	bodies := map[string]string{
		firstReportsURL:    `{"data":[{"type":"analyticsReports","id":"analytics-report-next-1","attributes":{"name":"Retention","category":"APP_USAGE","granularity":"DAILY"}}],"links":{"first":"` + reportsURL + `","prev":"` + reportsURL + `?cursor=9w","next":"` + secondReportsURL + `"}}`,
		secondReportsURL:   `{"data":[{"type":"analyticsReports","id":"analytics-report-next-2","attributes":{"name":"Acquisition","category":"APP_STORE","granularity":"DAILY"}}],"links":{"next":""}}`,
		firstInstancesURL:  `{"data":[{"type":"analyticsReportInstances","id":"analytics-instance-next-1","attributes":{"reportDate":"2024-01-01","processingDate":"2024-01-02T00:00:00Z","granularity":"DAILY","version":"1"}}],"links":{"next":""}}`,
		secondInstancesURL: `{"data":[{"type":"analyticsReportInstances","id":"analytics-instance-next-2","attributes":{"reportDate":"2024-01-03","processingDate":"2024-01-04T00:00:00Z","granularity":"DAILY","version":"1"}}],"links":{"next":""}}`,
	}

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/analyticsReportRequests/"+analyticsUsageExitRequestID+"/reports", "cursor=AQ", "limit=200"), asctest.JSON(http.StatusOK, bodies[firstReportsURL]))
	reg.Register(asctest.GET("/v1/analyticsReportRequests/"+analyticsUsageExitRequestID+"/reports", "cursor=BQ", "limit=200"), asctest.JSON(http.StatusOK, bodies[secondReportsURL]))
	reg.Register(asctest.GET("/v1/analyticsReports/analytics-report-next-1/instances", "limit=200"), asctest.JSON(http.StatusOK, bodies[firstInstancesURL]))
	reg.Register(asctest.GET("/v1/analyticsReports/analytics-report-next-2/instances", "limit=200"), asctest.JSON(http.StatusOK, bodies[secondInstancesURL]))
	client, err := asc.NewClientWithHTTPClient(
		"TEST_KEY",
		"TEST_ISSUER",
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: reg},
	)
	if err != nil {
		t.Fatalf("create analytics pagination test client: %v", err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))

	stdout, stderr, err := runCommand(t, append([]string{"analytics", "view", "--paginate", "--next", firstReportsURL}, requestArgs...))
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	requests := reg.Requests()
	if requestCount := len(requests); requestCount != 4 {
		t.Fatalf("request count = %d, want 4", requestCount)
	}
	seen := make(map[string]int, len(requests))
	for _, req := range requests {
		seen[req.URL.String()]++
	}
	for url, count := range seen {
		if count != 1 {
			t.Fatalf("%s fetched %d times, want 1", url, count)
		}
	}
	for _, link := range []string{
		`"first":"` + reportsURL + `"`,
		`"prev":"` + reportsURL + `?cursor=9w"`,
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
