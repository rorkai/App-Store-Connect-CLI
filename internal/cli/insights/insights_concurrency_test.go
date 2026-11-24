package insights

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func insightsBarrier(t *testing.T, size int32) func(*http.Request) {
	t.Helper()
	var arrived atomic.Int32
	release := make(chan struct{})
	return func(req *http.Request) {
		if arrived.Add(1) == size {
			close(release)
		}
		select {
		case <-release:
		case <-req.Context().Done():
		case <-time.After(2 * time.Second):
			t.Errorf("request %s did not overlap with %d peers", req.URL.Path, size-1)
		}
	}
}

func insightsStatusResponse(req *http.Request, status int) *http.Response {
	resp := jsonInsightsResponse(req, fmt.Sprintf(`{"errors":[{"status":"%d","code":"ERROR","title":"error","detail":"error"}]}`, status))
	resp.StatusCode = status
	return resp
}

func TestFetchAppSalesReportsOverlapsAppLookup(t *testing.T) {
	report := gzipText(t, "SKU\tApple Identifier\tUnits\nAPP\t123\t2\nOTHER\t999\t5\n")
	await := insightsBarrier(t, 3)
	client := newInsightsTestClient(t, func(req *http.Request) (*http.Response, error) {
		await(req)
		if req.URL.Path == "/v1/apps/123" {
			return jsonInsightsResponse(req, `{"data":{"type":"apps","id":"123","attributes":{"sku":"APP"}}}`), nil
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(report)), Request: req}, nil
	})

	params := salesSummaryReportParams("V", asc.SalesReportFrequencyDaily, "2026-02-01")
	scope, metrics, errs, err := fetchAppSalesReports(context.Background(), client, "123", [2]asc.SalesReportParams{params, params}, false)
	if err != nil || errs[0] != nil || errs[1] != nil {
		t.Fatalf("err=%v report errs=%v", err, errs)
	}
	if scope.AppSKU != "APP" || metrics[0].UnitsTotal != 2 || metrics[1].UnitsTotal != 2 {
		t.Fatalf("scope=%+v metrics=%+v", scope, metrics)
	}
}

func TestFetchAppSalesReportsAppErrorTakesPrecedence(t *testing.T) {
	client := newInsightsTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/v1/apps/123" {
			return insightsStatusResponse(req, http.StatusNotFound), nil
		}
		return insightsStatusResponse(req, http.StatusForbidden), nil
	})

	params := salesSummaryReportParams("V", asc.SalesReportFrequencyWeekly, "2026-02-01")
	_, _, _, err := fetchAppSalesReports(context.Background(), client, "123", [2]asc.SalesReportParams{params, params}, false)
	if !asc.IsNotFound(err) {
		t.Fatalf("expected app lookup not-found error, got %v", err)
	}
}

func TestCollectAnalyticsMetricsOverlapsInstancesInReportOrder(t *testing.T) {
	thisWeek := weekWindowFromStart(time.Date(2026, 2, 16, 0, 0, 0, 0, time.UTC))
	previousWeek := weekWindowFromStart(time.Date(2026, 2, 9, 0, 0, 0, 0, time.UTC))
	await := insightsBarrier(t, 4)
	notFoundSent := make(chan struct{})
	client := newInsightsTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(req.URL.Path, "/analyticsReportRequests"):
			return jsonInsightsResponse(req, `{"data":[{"type":"analyticsReportRequests","id":"request-1","attributes":{}}],"links":{}}`), nil
		case strings.HasSuffix(req.URL.Path, "/reports"):
			return jsonInsightsResponse(req, `{"data":[
				{"type":"analyticsReports","id":"report-1"},{"type":"analyticsReports","id":"report-2"},
				{"type":"analyticsReports","id":"report-3"},{"type":"analyticsReports","id":"report-4"}
			],"links":{}}`), nil
		}
		await(req)
		switch {
		case strings.Contains(req.URL.Path, "/report-1/"):
			select {
			case <-notFoundSent:
			case <-time.After(2 * time.Second):
			}
			return insightsStatusResponse(req, http.StatusForbidden), nil
		case strings.Contains(req.URL.Path, "/report-2/"):
			defer close(notFoundSent)
			return insightsStatusResponse(req, http.StatusNotFound), nil
		default:
			return jsonInsightsResponse(req, `{"data":[],"links":{}}`), nil
		}
	})

	metrics, requestCount, err := collectAnalyticsMetrics(context.Background(), client, "123", thisWeek, previousWeek)
	if err != nil || requestCount != 1 {
		t.Fatalf("requestCount=%d err=%v", requestCount, err)
	}
	want := "analytics report instance endpoints are not permitted for the current API key"
	if got := findWeeklyMetric(t, metrics, "instances_available").Reason; got != want {
		t.Fatalf("reason=%q, want %q", got, want)
	}
}
