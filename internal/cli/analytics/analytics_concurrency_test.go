package analytics

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestCollectAnalyticsReportsBoundsInstanceFetch(t *testing.T) {
	analyticsInstanceFetchMaxInFlight.Store(0)
	previous := fetchAnalyticsReportInstancesFn
	var inFlight atomic.Int32
	t.Cleanup(func() { fetchAnalyticsReportInstancesFn = previous })
	fetchAnalyticsReportInstancesFn = func(context.Context, *asc.Client, string, ...asc.AnalyticsReportInstancesOption) ([]asc.Resource[asc.AnalyticsReportInstanceAttributes], error) {
		current := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			seen := analyticsInstanceFetchMaxInFlight.Load()
			if current <= seen || analyticsInstanceFetchMaxInFlight.CompareAndSwap(seen, current) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		return []asc.Resource[asc.AnalyticsReportInstanceAttributes]{{ID: "instance"}}, nil
	}
	reports := make([]asc.Resource[asc.AnalyticsReportAttributes], 8)
	for i := range reports {
		reports[i].ID = string(rune('a' + i))
	}
	start := time.Now()
	collected, count, err := collectAnalyticsReports(context.Background(), nil, reports, nil, false, "")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if count != 8 || len(collected) != 8 {
		t.Fatalf("count=%d collected=%d", count, len(collected))
	}
	if collected[0].ID != "a" || collected[7].ID != "h" {
		t.Fatalf("order not preserved: %#v", collected)
	}
	if maxInFlight := analyticsInstanceFetchMaxInFlight.Load(); maxInFlight < 2 || maxInFlight > analyticsInstanceFetchConcurrency {
		t.Fatalf("max in-flight = %d, want 2-%d", maxInFlight, analyticsInstanceFetchConcurrency)
	}
	if elapsed > 160*time.Millisecond {
		t.Fatalf("elapsed %s, want bounded parallel fetch under 160ms", elapsed)
	}
}
