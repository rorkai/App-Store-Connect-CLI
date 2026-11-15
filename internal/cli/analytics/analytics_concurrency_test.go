package analytics

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// analyticsFetchGateTimeout only guards against hangs: a gated fetch that
// never sees the pool saturate gives up after this long, switches gating off,
// and the test fails on the recorded peak instead of blocking forever.
const analyticsFetchGateTimeout = 5 * time.Second

// analyticsFetchGate holds every instance fetch until the worker pool is
// saturated (or every remaining fetch is in flight), so a pool bounded at B
// deterministically peaks at exactly B regardless of scheduling or host load.
// A serial fetch never saturates the gate; the first wait times out and the
// peak stays at 1.
type analyticsFetchGate struct {
	bound    int
	expected int

	mu          sync.Mutex
	cond        *sync.Cond
	gating      bool
	timedOut    bool
	inFlight    int
	maxInFlight int
	completed   int
}

func newAnalyticsFetchGate(bound, expected int) *analyticsFetchGate {
	gate := &analyticsFetchGate{bound: bound, expected: expected, gating: true}
	gate.cond = sync.NewCond(&gate.mu)
	return gate
}

func (g *analyticsFetchGate) begin() func() {
	g.mu.Lock()
	g.inFlight++
	g.maxInFlight = max(g.maxInFlight, g.inFlight)
	g.cond.Broadcast()
	if g.gating && !g.saturatedLocked() {
		timer := time.AfterFunc(analyticsFetchGateTimeout, func() {
			g.mu.Lock()
			defer g.mu.Unlock()
			if g.gating {
				g.gating = false
				g.timedOut = true
				g.cond.Broadcast()
			}
		})
		for g.gating && !g.saturatedLocked() {
			g.cond.Wait()
		}
		timer.Stop()
	}
	g.mu.Unlock()
	return func() {
		g.mu.Lock()
		g.inFlight--
		g.completed++
		g.cond.Broadcast()
		g.mu.Unlock()
	}
}

func (g *analyticsFetchGate) saturatedLocked() bool {
	return g.inFlight >= min(g.bound, g.expected-g.completed)
}

func (g *analyticsFetchGate) peak() (maxInFlight int, timedOut bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.maxInFlight, g.timedOut
}

func TestCollectAnalyticsReportsBoundsInstanceFetch(t *testing.T) {
	analyticsInstanceFetchMaxInFlight.Store(0)
	previous := fetchAnalyticsReportInstancesFn
	t.Cleanup(func() { fetchAnalyticsReportInstancesFn = previous })

	// More reports than workers so the pool must refill and the bound is
	// exercised, not just the report count.
	reportCount := 2*analyticsInstanceFetchConcurrency + 3
	gate := newAnalyticsFetchGate(analyticsInstanceFetchConcurrency, reportCount)
	fetchAnalyticsReportInstancesFn = func(_ context.Context, _ *shared.Deps, _ *asc.Client, reportID string, _ ...asc.AnalyticsReportInstancesOption) ([]asc.Resource[asc.AnalyticsReportInstanceAttributes], error) {
		done := gate.begin()
		defer done()
		return []asc.Resource[asc.AnalyticsReportInstanceAttributes]{{ID: "instance-" + reportID}}, nil
	}
	reports := make([]asc.Resource[asc.AnalyticsReportAttributes], reportCount)
	for i := range reports {
		reports[i].ID = fmt.Sprintf("report-%02d", i)
	}

	collected, count, err := collectAnalyticsReports(context.Background(), nil, nil, reports, nil, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if count != reportCount || len(collected) != reportCount {
		t.Fatalf("count=%d collected=%d, want %d", count, len(collected), reportCount)
	}
	for i, report := range collected {
		if report.ID != reports[i].ID {
			t.Fatalf("collected[%d].ID = %q, want %q (order not preserved)", i, report.ID, reports[i].ID)
		}
	}

	maxInFlight, timedOut := gate.peak()
	if timedOut {
		t.Errorf("fetch gate timed out waiting for %d concurrent fetches; peak was %d", analyticsInstanceFetchConcurrency, maxInFlight)
	}
	if maxInFlight != analyticsInstanceFetchConcurrency {
		t.Errorf("observed peak in-flight fetches = %d, want %d", maxInFlight, analyticsInstanceFetchConcurrency)
	}
	if recorded := analyticsInstanceFetchMaxInFlight.Load(); recorded != analyticsInstanceFetchConcurrency {
		t.Errorf("recorded max in-flight = %d, want %d", recorded, analyticsInstanceFetchConcurrency)
	}
}

func TestCollectAnalyticsReportsCancelsSiblingsAndPreservesFirstError(t *testing.T) {
	sentinel := errors.New("report fetch failed")
	previous := fetchAnalyticsReportInstancesFn
	t.Cleanup(func() { fetchAnalyticsReportInstancesFn = previous })
	ready := make(chan struct{}, 3)
	fetchAnalyticsReportInstancesFn = func(ctx context.Context, _ *shared.Deps, _ *asc.Client, reportID string, _ ...asc.AnalyticsReportInstancesOption) ([]asc.Resource[asc.AnalyticsReportInstanceAttributes], error) {
		if reportID == "fail" {
			for i := 0; i < 3; i++ {
				<-ready
			}
			return nil, sentinel
		}
		ready <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	reports := make([]asc.Resource[asc.AnalyticsReportAttributes], 4)
	for i, id := range []string{"a", "b", "c", "fail"} {
		reports[i].ID = id
	}

	_, _, err := collectAnalyticsReports(context.Background(), nil, nil, reports, nil, false, "")
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want original sentinel", err)
	}
}
