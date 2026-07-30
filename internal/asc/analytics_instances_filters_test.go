package asc

import (
	"context"
	"net/http"
	"testing"
)

func TestGetAnalyticsReportInstances_UsesDocumentedFilters(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/analyticsReports/report-1/instances" {
			t.Fatalf("unexpected path %q", req.URL.Path)
		}
		query := req.URL.Query()
		if got := query.Get("filter[granularity]"); got != "WEEKLY" {
			t.Fatalf("expected weekly granularity filter, got %q", got)
		}
		if got := query.Get("filter[processingDate]"); got != "2026-02-20,2026-02-27" {
			t.Fatalf("unexpected processing-date filter %q", got)
		}
		if got := query.Get("limit"); got != "200" {
			t.Fatalf("expected limit 200, got %q", got)
		}
		assertAuthorized(t, req)
	}, jsonResponse(http.StatusOK, `{
		"data":[{
			"type":"analyticsReportInstances",
			"id":"instance-1",
			"attributes":{"granularity":"WEEKLY","processingDate":"2026-02-27"}
		}],
		"links":{"self":"https://api.appstoreconnect.apple.com/v1/analyticsReports/report-1/instances"}
	}`))

	response, err := client.GetAnalyticsReportInstances(
		context.Background(),
		"report-1",
		WithAnalyticsReportInstancesGranularity("WEEKLY"),
		WithAnalyticsReportInstancesProcessingDates("2026-02-20", "2026-02-27"),
		WithAnalyticsReportInstancesLimit(200),
	)
	if err != nil {
		t.Fatalf("GetAnalyticsReportInstances() error = %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].ID != "instance-1" {
		t.Fatalf("unexpected response %+v", response.Data)
	}
}
