package insights

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type analyticsMetricsStub struct {
	requests          *asc.AnalyticsReportRequestsResponse
	reportsByRequest  map[string]*asc.AnalyticsReportsResponse
	instancesByReport map[string]*asc.AnalyticsReportInstancesResponse
	instanceErrs      map[string]error
	waitByReport      map[string]<-chan struct{}
	releaseByReport   map[string]chan struct{}

	mu            sync.Mutex
	instanceCalls []string
}

func (s *analyticsMetricsStub) GetAnalyticsReportRequests(_ context.Context, _ string, _ ...asc.AnalyticsReportRequestsOption) (*asc.AnalyticsReportRequestsResponse, error) {
	return s.requests, nil
}

func (s *analyticsMetricsStub) GetAnalyticsReports(_ context.Context, requestID string, _ ...asc.AnalyticsReportsOption) (*asc.AnalyticsReportsResponse, error) {
	if resp, ok := s.reportsByRequest[requestID]; ok && resp != nil {
		return resp, nil
	}
	return &asc.AnalyticsReportsResponse{}, nil
}

func (s *analyticsMetricsStub) GetAnalyticsReportInstances(_ context.Context, reportID string, _ ...asc.AnalyticsReportInstancesOption) (*asc.AnalyticsReportInstancesResponse, error) {
	s.mu.Lock()
	s.instanceCalls = append(s.instanceCalls, reportID)
	s.mu.Unlock()
	if release := s.releaseByReport[reportID]; release != nil {
		close(release)
	}
	if wait := s.waitByReport[reportID]; wait != nil {
		<-wait
	}

	if err, ok := s.instanceErrs[reportID]; ok && err != nil {
		return nil, err
	}
	if resp, ok := s.instancesByReport[reportID]; ok && resp != nil {
		return resp, nil
	}
	return &asc.AnalyticsReportInstancesResponse{}, nil
}

func analyticsMetricsTestWindows() (reportWeekWindow, reportWeekWindow) {
	thisWeek := reportWeekWindow{
		start: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
		end:   time.Date(2026, 1, 11, 0, 0, 0, 0, time.UTC),
	}
	previousWeek := reportWeekWindow{
		start: time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC),
		end:   time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC),
	}
	return thisWeek, previousWeek
}

func analyticsMetricsStubWithReports(reportCount int) *analyticsMetricsStub {
	reports := make([]asc.Resource[asc.AnalyticsReportAttributes], 0, reportCount)
	instancesByReport := make(map[string]*asc.AnalyticsReportInstancesResponse, reportCount)
	for i := 0; i < reportCount; i++ {
		reportID := fmt.Sprintf("report-%d", i)
		reports = append(reports, asc.Resource[asc.AnalyticsReportAttributes]{ID: reportID})
		instances := []asc.Resource[asc.AnalyticsReportInstanceAttributes]{
			{
				ID:         fmt.Sprintf("instance-this-%d", i),
				Attributes: asc.AnalyticsReportInstanceAttributes{ReportDate: "2026-01-06"},
			},
		}
		if i%2 == 0 {
			instances = append(instances, asc.Resource[asc.AnalyticsReportInstanceAttributes]{
				ID:         fmt.Sprintf("instance-last-%d", i),
				Attributes: asc.AnalyticsReportInstanceAttributes{ReportDate: "2025-12-30"},
			})
		}
		instancesByReport[reportID] = &asc.AnalyticsReportInstancesResponse{Data: instances}
	}

	return &analyticsMetricsStub{
		requests: &asc.AnalyticsReportRequestsResponse{
			Data: []asc.AnalyticsReportRequestResource{
				{
					ID: "request-1",
					Attributes: asc.AnalyticsReportRequestAttributes{
						State:       asc.AnalyticsReportRequestStateCompleted,
						CreatedDate: "2026-01-05T10:00:00Z",
					},
				},
			},
		},
		reportsByRequest: map[string]*asc.AnalyticsReportsResponse{
			"request-1": {Data: reports},
		},
		instancesByReport: instancesByReport,
	}
}

func TestCollectAnalyticsMetricsAggregatesParallelInstanceFetches(t *testing.T) {
	const reportCount = 7
	stub := analyticsMetricsStubWithReports(reportCount)
	thisWeek, previousWeek := analyticsMetricsTestWindows()

	metrics, requestCount, err := collectAnalyticsMetrics(context.Background(), stub, "app-1", thisWeek, previousWeek)
	if err != nil {
		t.Fatalf("collectAnalyticsMetrics() error = %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("requestCount = %d, want 1", requestCount)
	}
	if len(metrics) != 4 {
		t.Fatalf("expected 4 metrics, got %#v", metrics)
	}

	assertMetricValue := func(metric weeklyMetric, name string, thisValue, lastValue float64) {
		t.Helper()
		if metric.Name != name {
			t.Fatalf("metric name = %q, want %q", metric.Name, name)
		}
		if metric.ThisWeek == nil || *metric.ThisWeek != thisValue {
			t.Fatalf("%s thisWeek = %v, want %v", name, metric.ThisWeek, thisValue)
		}
		if metric.LastWeek == nil || *metric.LastWeek != lastValue {
			t.Fatalf("%s lastWeek = %v, want %v", name, metric.LastWeek, lastValue)
		}
	}

	// Every report has one instance this week; even-indexed reports also have
	// one instance the previous week (4 of 7 for reportCount=7).
	assertMetricValue(metrics[0], "completed_requests", 1, 0)
	assertMetricValue(metrics[1], "reports_available", reportCount, 4)
	assertMetricValue(metrics[2], "instances_available", reportCount, 4)

	if got := len(stub.instanceCalls); got != reportCount {
		t.Fatalf("expected %d instance fetches, got %d (%v)", reportCount, got, stub.instanceCalls)
	}
	seen := make(map[string]struct{}, len(stub.instanceCalls))
	for _, reportID := range stub.instanceCalls {
		if _, dup := seen[reportID]; dup {
			t.Fatalf("report %s fetched more than once: %v", reportID, stub.instanceCalls)
		}
		seen[reportID] = struct{}{}
	}
}

func TestCollectAnalyticsMetricsPropagatesInstanceError(t *testing.T) {
	stub := analyticsMetricsStubWithReports(5)
	wantErr := errors.New("instance fetch failed")
	stub.instanceErrs = map[string]error{"report-3": wantErr}
	thisWeek, previousWeek := analyticsMetricsTestWindows()

	metrics, requestCount, err := collectAnalyticsMetrics(context.Background(), stub, "app-1", thisWeek, previousWeek)
	if !errors.Is(err, wantErr) {
		t.Fatalf("collectAnalyticsMetrics() error = %v, want %v", err, wantErr)
	}
	if metrics != nil {
		t.Fatalf("expected nil metrics on error, got %#v", metrics)
	}
	if requestCount != 1 {
		t.Fatalf("requestCount = %d, want 1", requestCount)
	}
}

func TestCollectAnalyticsMetricsForbiddenInstanceErrorMapsToUnavailable(t *testing.T) {
	stub := analyticsMetricsStubWithReports(3)
	stub.instanceErrs = map[string]error{"report-1": asc.ErrForbidden}
	thisWeek, previousWeek := analyticsMetricsTestWindows()

	metrics, requestCount, err := collectAnalyticsMetrics(context.Background(), stub, "app-1", thisWeek, previousWeek)
	if err != nil {
		t.Fatalf("collectAnalyticsMetrics() error = %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("requestCount = %d, want 1", requestCount)
	}
	if len(metrics) != 4 {
		t.Fatalf("expected 4 metrics, got %#v", metrics)
	}
	for _, metric := range metrics[:3] {
		if metric.Status != "unavailable" {
			t.Fatalf("metric %s status = %q, want unavailable", metric.Name, metric.Status)
		}
		if !strings.Contains(metric.Reason, "not permitted") {
			t.Fatalf("metric %s reason = %q, want forbidden reason", metric.Name, metric.Reason)
		}
	}
}

func TestCollectAnalyticsMetricsChoosesErrorByReportOrder(t *testing.T) {
	stub := analyticsMetricsStubWithReports(2)
	releaseFirst := make(chan struct{})
	stub.instanceErrs = map[string]error{
		"report-0": asc.ErrNotFound,
		"report-1": asc.ErrForbidden,
	}
	stub.waitByReport = map[string]<-chan struct{}{"report-0": releaseFirst}
	stub.releaseByReport = map[string]chan struct{}{"report-1": releaseFirst}
	thisWeek, previousWeek := analyticsMetricsTestWindows()

	metrics, _, err := collectAnalyticsMetrics(context.Background(), stub, "app-1", thisWeek, previousWeek)
	if err != nil {
		t.Fatalf("collectAnalyticsMetrics() error = %v", err)
	}
	if len(metrics) == 0 || !strings.Contains(metrics[0].Reason, "unavailable for this app") {
		t.Fatalf("expected lower-index not-found result to win, got %#v", metrics)
	}
}
