package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

const analyticsViewRequestID = "11111111-1111-1111-1111-111111111111"

func TestAnalyticsViewProcessingDateForwardsFilter(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/analyticsReportRequests/"+analyticsViewRequestID+"/reports", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[{"type":"analyticsReports","id":"report-1","attributes":{"name":"App Sessions","granularity":"DAILY"}}],
		"links":{}
	}`))
	reg.Register(asctest.GET("/v1/analyticsReports/report-1/instances", "filter[processingDate]=2024-01-20", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[{"type":"analyticsReportInstances","id":"instance-1","attributes":{"processingDate":"2024-01-20","granularity":"DAILY"}}],
		"links":{}
	}`))

	_, stderr := runAnalyticsViewForOutput(
		t,
		"--request-id", analyticsViewRequestID,
		"--processing-date", "2024-01-20",
		"--output", "json",
	)
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestAnalyticsViewGranularityOnlyPreservesReportMetadata(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/analyticsReportRequests/"+analyticsViewRequestID+"/reports", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[{"type":"analyticsReports","id":"report-1","attributes":{"name":"Sales","reportType":"STANDARD","category":"COMMERCE","granularity":"MONTHLY"}}],
		"links":{}
	}`))
	reg.Register(asctest.GET("/v1/analyticsReports/report-1/instances", "filter[granularity]=MONTHLY", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))

	stdout, stderr := runAnalyticsViewForOutput(
		t,
		"--request-id", analyticsViewRequestID,
		"--granularity", "monthly",
		"--output", "json",
	)
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	assertAnalyticsViewJSONEqual(t, stdout, `{
		"requestId":"11111111-1111-1111-1111-111111111111",
		"data":[{
			"id":"report-1",
			"reportType":"STANDARD",
			"name":"Sales",
			"category":"COMMERCE",
			"granularity":"MONTHLY"
		}],
		"links":{}
	}`)
	if requestCount := len(reg.Requests()); requestCount != 2 {
		t.Fatalf("expected 2 requests, got %d", requestCount)
	}
}

func TestAnalyticsViewFiltersPaginateAndPreserveOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const (
		reportsNextURL   = "https://api.appstoreconnect.apple.com/v1/analyticsReportRequests/11111111-1111-1111-1111-111111111111/reports?cursor=reports-next"
		instancesNextURL = "https://api.appstoreconnect.apple.com/v1/analyticsReports/report-1/instances?cursor=instances-next"
	)

	instanceFilters := []string{"filter[processingDate]=2024-01-20", "filter[granularity]=DAILY,WEEKLY,MONTHLY", "limit=200"}
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/analyticsReportRequests/"+analyticsViewRequestID+"/reports", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[{"type":"analyticsReports","id":"report-1","attributes":{"name":"App Sessions","reportType":"STANDARD","category":"APP_USAGE","granularity":"DAILY"}}],
		"links":{"self":"https://api.appstoreconnect.apple.com/v1/analyticsReportRequests/11111111-1111-1111-1111-111111111111/reports","next":"`+reportsNextURL+`"}
	}`))
	reg.Register(asctest.GET("/v1/analyticsReportRequests/"+analyticsViewRequestID+"/reports", "cursor=reports-next"), asctest.JSON(http.StatusOK, `{
		"data":[{"type":"analyticsReports","id":"report-2","attributes":{"name":"Store Discovery","reportType":"STANDARD","category":"APP_STORE_ENGAGEMENT","granularity":"WEEKLY"}}],
		"links":{}
	}`))
	reg.Register(asctest.GET("/v1/analyticsReports/report-1/instances", instanceFilters...), asctest.JSON(http.StatusOK, `{
		"data":[{"type":"analyticsReportInstances","id":"instance-1","attributes":{"reportDate":"2024-01-19","processingDate":"2024-01-20","granularity":"DAILY","version":"1.0"}}],
		"links":{"next":"`+instancesNextURL+`"}
	}`))
	reg.Register(asctest.GET("/v1/analyticsReports/report-1/instances", "cursor=instances-next"), asctest.JSON(http.StatusOK, `{
		"data":[{"type":"analyticsReportInstances","id":"instance-2","attributes":{"reportDate":"2024-01-19","processingDate":"2024-01-20","granularity":"WEEKLY","version":"1.0"}}],
		"links":{}
	}`))
	reg.Register(asctest.GET("/v1/analyticsReports/report-2/instances", instanceFilters...), asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))

	stdout, stderr := runAnalyticsViewForOutput(
		t,
		"--request-id", analyticsViewRequestID,
		"--processing-date", "2024-01-20",
		"--granularity", "daily, weekly, monthly",
		"--paginate",
		"--output", "json",
	)
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	assertAnalyticsViewJSONEqual(t, stdout, `{
		"requestId":"11111111-1111-1111-1111-111111111111",
		"data":[{
			"id":"report-1",
			"reportType":"STANDARD",
			"name":"App Sessions",
			"category":"APP_USAGE",
			"granularity":"DAILY",
			"instances":[
				{"id":"instance-1","reportDate":"2024-01-19","processingDate":"2024-01-20","granularity":"DAILY","version":"1.0"},
				{"id":"instance-2","reportDate":"2024-01-19","processingDate":"2024-01-20","granularity":"WEEKLY","version":"1.0"}
			]
		}],
		"links":{"self":"https://api.appstoreconnect.apple.com/v1/analyticsReportRequests/11111111-1111-1111-1111-111111111111/reports"}
	}`)
	if requestCount := len(reg.Requests()); requestCount != 5 {
		t.Fatalf("expected 5 requests, got %d", requestCount)
	}
}

func TestAnalyticsViewAcceptsPrefixedInstanceAndPreservesSegmentBehavior(t *testing.T) {
	setupAnalyticsResourceIDAuth(t)

	const instanceID = "r39-example-instance"
	const reportsNextURL = "https://api.appstoreconnect.apple.com/v1/analyticsReportRequests/" + analyticsViewRequestID + "/reports?cursor=unused-report"
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/analyticsReportRequests/"+analyticsViewRequestID+"/reports", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[{"type":"analyticsReports","id":"report-1","attributes":{"name":"App Sessions","granularity":"DAILY"}}],
		"links":{"next":"`+reportsNextURL+`"}
	}`))
	reg.Register(asctest.GET("/v1/analyticsReports/report-1/instances", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[
			{"type":"analyticsReportInstances","id":"ignored-instance","attributes":{"processingDate":"2024-01-19"}},
			{"type":"analyticsReportInstances","id":"`+instanceID+`","attributes":{"reportDate":"2024-01-19","processingDate":"2024-01-20","granularity":"DAILY","version":"1.0"}}
		],
		"links":{"next":"https://api.appstoreconnect.apple.com/v1/analyticsReports/report-1/instances?cursor=unused"}
	}`))
	reg.Register(asctest.GET("/v1/analyticsReportInstances/"+instanceID+"/segments", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data":[{"type":"analyticsReportSegments","id":"segment-1","attributes":{"url":"https://example.com/report.gz","checksum":"abc123","sizeInBytes":42,"urlExpirationDate":"2024-01-21T00:00:00Z"}}],
		"links":{}
	}`))

	stdout, stderr := runAnalyticsViewForOutput(
		t,
		"--request-id", analyticsViewRequestID,
		"--instance-id", instanceID,
		"--include-segments",
		"--output", "json",
	)
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	assertAnalyticsViewJSONEqual(t, stdout, `{
		"requestId":"11111111-1111-1111-1111-111111111111",
		"data":[{
			"id":"report-1",
			"name":"App Sessions",
			"granularity":"DAILY",
			"instances":[{
				"id":"r39-example-instance",
				"reportDate":"2024-01-19",
				"processingDate":"2024-01-20",
				"granularity":"DAILY",
				"version":"1.0",
				"segments":[{
					"id":"segment-1",
					"downloadUrl":"https://example.com/report.gz",
					"checksum":"abc123",
					"sizeInBytes":42,
					"urlExpirationDate":"2024-01-21T00:00:00Z"
				}]
			}]
		}],
		"links":{}
	}`)
	requests := reg.Requests()
	if requestCount := len(requests); requestCount != 3 {
		t.Fatalf("expected 3 requests, got %d", requestCount)
	}
	for _, req := range requests {
		if req.Header.Get("Authorization") == "" {
			t.Fatal("expected Authorization header")
		}
	}
}

func runAnalyticsViewForOutput(t *testing.T, args ...string) (string, string) {
	t.Helper()
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	fullArgs := append([]string{"analytics", "view"}, args...)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(fullArgs); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	return stdout, stderr
}

func assertAnalyticsViewJSONEqual(t *testing.T, gotJSON, wantJSON string) {
	t.Helper()
	var got any
	if err := json.Unmarshal([]byte(gotJSON), &got); err != nil {
		t.Fatalf("failed to parse command output: %v\noutput=%s", err, gotJSON)
	}
	var want any
	if err := json.Unmarshal([]byte(wantJSON), &want); err != nil {
		t.Fatalf("failed to parse expected output: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected JSON output\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}

func analyticsViewJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}
