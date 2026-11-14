package asc

import (
	"context"
	"net/http"
	"testing"
)

func TestGetAnalyticsReportInstancesUsesDocumentedFilters(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", req.Method)
		}
		if req.URL.Path != "/v1/analyticsReports/report-1/instances" {
			t.Fatalf("path = %q, want analytics report instances path", req.URL.Path)
		}
		query := req.URL.Query()
		if got := query.Get("filter[granularity]"); got != "DAILY,WEEKLY" {
			t.Fatalf("filter[granularity] = %q, want %q", got, "DAILY,WEEKLY")
		}
		if got := query.Get("filter[processingDate]"); got != "2026-02-20,2026-02-27" {
			t.Fatalf("filter[processingDate] = %q, want %q", got, "2026-02-20,2026-02-27")
		}
		if got := query.Get("limit"); got != "200" {
			t.Fatalf("limit = %q, want %q", got, "200")
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
		WithAnalyticsReportInstancesGranularities([]string{"daily", " WEEKLY ", "DAILY"}),
		WithAnalyticsReportInstancesProcessingDates([]string{"2026-02-20", " 2026-02-27 ", "2026-02-20", ""}),
		Limit[AnalyticsReportInstancesOption](200),
	)
	if err != nil {
		t.Fatalf("GetAnalyticsReportInstances() error = %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].ID != "instance-1" {
		t.Fatalf("unexpected response %+v", response.Data)
	}
	if got := response.Data[0].Attributes.Granularity; got != "WEEKLY" {
		t.Fatalf("decoded granularity = %q, want WEEKLY", got)
	}
}
