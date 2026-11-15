package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Fan-out bounds for the two commands tracked by issue #2615. They mirror
// bulkAvailabilityWorkers (internal/cli/shared), which issue #2786 raised to
// asc.BulkMutatingRequestLimit, and analyticsInstanceFetchConcurrency
// (internal/cli/analytics).
const (
	pricingAvailabilityEditFanOut = 16
	analyticsViewInstanceFanOut   = 8

	fanOutTerritoryCount = 175 // territories Apple returns for a typical app
	fanOutReportCount    = 156 // reports Apple returns for a new ONGOING request

	fanOutGateTimeout = 2 * time.Second
	fanOutRequestID   = "22222222-2222-2222-2222-222222222222"
)

// fanOutProbe is a fake App Store Connect transport state that counts
// requests per route and records the peak number of concurrent requests on
// one tracked route.
//
// With latency == 0 every tracked request waits until the fan-out bound is
// saturated (or until every remaining tracked request is in flight), so a
// bounded worker pool of size B deterministically peaks at exactly B. A
// serial fan-out never saturates the gate; the first wait times out, gating
// switches off, and the test fails without hanging.
//
// With latency > 0 gating is off and every request sleeps for latency to
// simulate App Store Connect round trips (used by the timing harness).
type fanOutProbe struct {
	latency  time.Duration
	bound    int
	expected int

	mu          sync.Mutex
	cond        *sync.Cond
	gating      bool
	timedOut    bool
	counts      map[string]int
	inFlight    int
	maxInFlight int
	completed   int
}

func newFanOutProbe(bound, expected int, latency time.Duration) *fanOutProbe {
	probe := &fanOutProbe{
		latency:  latency,
		bound:    bound,
		expected: expected,
		gating:   latency == 0,
		counts:   make(map[string]int),
	}
	probe.cond = sync.NewCond(&probe.mu)
	return probe
}

func (p *fanOutProbe) begin(route string, tracked bool) func() {
	p.mu.Lock()
	p.counts[route]++
	if tracked {
		p.inFlight++
		p.maxInFlight = max(p.maxInFlight, p.inFlight)
		p.cond.Broadcast()
		if p.gating && !p.saturatedLocked() {
			timer := time.AfterFunc(fanOutGateTimeout, func() {
				p.mu.Lock()
				defer p.mu.Unlock()
				if p.gating {
					p.gating = false
					p.timedOut = true
					p.cond.Broadcast()
				}
			})
			for p.gating && !p.saturatedLocked() {
				p.cond.Wait()
			}
			timer.Stop()
		}
	}
	p.mu.Unlock()

	if p.latency > 0 {
		time.Sleep(p.latency)
	}
	return func() {
		if !tracked {
			return
		}
		p.mu.Lock()
		p.inFlight--
		p.completed++
		p.cond.Broadcast()
		p.mu.Unlock()
	}
}

func (p *fanOutProbe) saturatedLocked() bool {
	return p.inFlight >= min(p.bound, p.expected-p.completed)
}

type fanOutSnapshot struct {
	counts      map[string]int
	maxInFlight int
	timedOut    bool
}

func (p *fanOutProbe) snapshot() fanOutSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	counts := make(map[string]int, len(p.counts))
	for route, count := range p.counts {
		counts[route] = count
	}
	return fanOutSnapshot{counts: counts, maxInFlight: p.maxInFlight, timedOut: p.timedOut}
}

func fanOutTerritoryID(index int) string {
	return fmt.Sprintf("T%02d", index)
}

// installPricingAvailabilityFanOutFake serves one app availability with
// territoryCount territories that all start unavailable. PATCH requests flip
// the stored state so the command's final verification observes them.
func installPricingAvailabilityFanOutFake(t *testing.T, probe *fanOutProbe, territoryCount int) {
	t.Helper()

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var mu sync.Mutex
	available := make(map[string]bool, territoryCount)
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appAvailabilityV2":
			defer probe.begin("GET appAvailabilityV2", false)()
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":false}},"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appAvailabilityV2"}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v2/appAvailabilities/availability-1/territoryAvailabilities":
			defer probe.begin("GET territoryAvailabilities", false)()
			mu.Lock()
			items := make([]string, 0, territoryCount)
			for index := range territoryCount {
				territory := fanOutTerritoryID(index)
				items = append(items, fmt.Sprintf(
					`{"type":"territoryAvailabilities","id":"ta-%s","attributes":{"available":%t},"relationships":{"territory":{"data":{"type":"territories","id":%q}}}}`,
					territory, available["ta-"+territory], territory,
				))
			}
			mu.Unlock()
			return jsonHTTPResponse(http.StatusOK, `{"data":[`+strings.Join(items, ",")+`],"links":{"next":""}}`), nil
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/territoryAvailabilities/"):
			defer probe.begin("PATCH territoryAvailabilities", true)()
			id := strings.TrimPrefix(req.URL.Path, "/v1/territoryAvailabilities/")
			var payload struct {
				Data struct {
					Attributes struct {
						Available *bool `json:"available"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil || payload.Data.Attributes.Available == nil {
				return jsonHTTPResponse(http.StatusBadRequest, `{"errors":[{"status":"400","title":"bad payload"}]}`), nil
			}
			mu.Lock()
			available[id] = *payload.Data.Attributes.Available
			mu.Unlock()
			return jsonHTTPResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"territoryAvailabilities","id":%q,"attributes":{"available":%t}}}`, id, *payload.Data.Attributes.Available)), nil
		default:
			return jsonHTTPResponse(http.StatusNotFound, fmt.Sprintf(`{"errors":[{"status":"404","title":"unexpected request %s %s"}]}`, req.Method, req.URL.Path)), nil
		}
	})
}

func installAnalyticsViewFanOutFake(t *testing.T, probe *fanOutProbe, reportCount int) {
	t.Helper()

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = analyticsViewFanOutTransport(probe, reportCount)
}

// analyticsViewFanOutTransport serves one report request with reportCount
// reports and no instances, which is what Apple returns for a new ONGOING
// request.
func analyticsViewFanOutTransport(probe *fanOutProbe, reportCount int) http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/analyticsReportRequests/"+fanOutRequestID+"/reports":
			defer probe.begin("GET reports", false)()
			items := make([]string, 0, reportCount)
			for index := range reportCount {
				items = append(items, fmt.Sprintf(`{"type":"analyticsReports","id":"report-%03d","attributes":{"name":"Report %03d","category":"APP_USAGE"}}`, index, index))
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[`+strings.Join(items, ",")+`],"links":{}}`), nil
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/analyticsReports/") && strings.HasSuffix(req.URL.Path, "/instances"):
			defer probe.begin("GET instances", true)()
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		default:
			return jsonHTTPResponse(http.StatusNotFound, fmt.Sprintf(`{"errors":[{"status":"404","title":"unexpected request %s %s"}]}`, req.Method, req.URL.Path)), nil
		}
	})
}

func runFanOutCommand(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func pricingAvailabilityEditAllTerritoriesArgs() []string {
	return []string{
		"pricing", "availability", "edit",
		"--app", "app-1",
		"--all-territories",
		"--available", "true",
		"--available-in-new-territories", "false",
		"--output", "json",
	}
}

func analyticsViewFanOutArgs() []string {
	return []string{"analytics", "view", "--request-id", fanOutRequestID, "--paginate", "--output", "json"}
}

func setupFanOutAuth(t *testing.T) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
}

func TestPricingAvailabilityEditAllTerritoriesBoundsPatchFanOut(t *testing.T) {
	setupFanOutAuth(t)
	probe := newFanOutProbe(pricingAvailabilityEditFanOut, fanOutTerritoryCount, 0)
	installPricingAvailabilityFanOutFake(t, probe, fanOutTerritoryCount)

	stdout, stderr, err := runFanOutCommand(t, pricingAvailabilityEditAllTerritoriesArgs()...)
	if err != nil {
		t.Fatalf("run error: %v (stderr %q)", err, stderr)
	}

	got := probe.snapshot()
	if got.timedOut {
		t.Fatalf("territory PATCH requests never reached %d in flight (peak %d)", pricingAvailabilityEditFanOut, got.maxInFlight)
	}
	if got.maxInFlight != pricingAvailabilityEditFanOut {
		t.Fatalf("peak concurrent territory PATCH requests = %d, want exactly %d", got.maxInFlight, pricingAvailabilityEditFanOut)
	}
	wantCounts := map[string]int{
		"GET appAvailabilityV2":         1,
		"GET territoryAvailabilities":   2, // initial read + final verification
		"PATCH territoryAvailabilities": fanOutTerritoryCount,
	}
	for route, want := range wantCounts {
		if got.counts[route] != want {
			t.Fatalf("%s requests = %d, want %d (all counts %v)", route, got.counts[route], want, got.counts)
		}
	}
	if len(got.counts) != len(wantCounts) {
		t.Fatalf("unexpected routes: %v", got.counts)
	}
	wantStdout := `{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":false}},"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appAvailabilityV2"}}`
	if strings.TrimSpace(stdout) != wantStdout {
		t.Fatalf("stdout = %s, want %s", stdout, wantStdout)
	}
	wantStderr := fmt.Sprintf("Updating availability for %d territories (0 already matched)...\nUpdated %d territories; 0 already matched; verified %d updated territories.\n", fanOutTerritoryCount, fanOutTerritoryCount, fanOutTerritoryCount)
	if stderr != wantStderr {
		t.Fatalf("stderr = %q, want %q", stderr, wantStderr)
	}
}

func TestAnalyticsViewBoundsInstanceFetchFanOut(t *testing.T) {
	setupFanOutAuth(t)
	probe := newFanOutProbe(analyticsViewInstanceFanOut, fanOutReportCount, 0)

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, analyticsViewFanOutTransport(probe, fanOutReportCount)), analyticsViewFanOutArgs()...)
	if err != nil {
		t.Fatalf("run error: %v (stderr %q)", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	got := probe.snapshot()
	if got.timedOut {
		t.Fatalf("instance requests never reached %d in flight (peak %d)", analyticsViewInstanceFanOut, got.maxInFlight)
	}
	if got.maxInFlight != analyticsViewInstanceFanOut {
		t.Fatalf("peak concurrent instance requests = %d, want exactly %d", got.maxInFlight, analyticsViewInstanceFanOut)
	}
	if got.counts["GET reports"] != 1 || got.counts["GET instances"] != fanOutReportCount || len(got.counts) != 2 {
		t.Fatalf("request counts = %v, want 1 reports page and %d instance lists", got.counts, fanOutReportCount)
	}

	var result struct {
		RequestID string `json:"requestId"`
		Data      []struct {
			ID        string            `json:"id"`
			Instances []json.RawMessage `json:"instances"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout)
	}
	if result.RequestID != fanOutRequestID || len(result.Data) != fanOutReportCount {
		t.Fatalf("requestId=%q reports=%d, want %q and %d", result.RequestID, len(result.Data), fanOutRequestID, fanOutReportCount)
	}
	for index, report := range result.Data {
		if want := fmt.Sprintf("report-%03d", index); report.ID != want {
			t.Fatalf("report %d = %q, want %q (report order must match Apple's order)", index, report.ID, want)
		}
		if len(report.Instances) != 0 {
			t.Fatalf("report %q instances = %d, want 0", report.ID, len(report.Instances))
		}
	}
}

// TestFanOutSimulatedLatencyTimings is the before/after timing harness for
// issue #2615. It is skipped unless ASC_FANOUT_SIMULATED_LATENCY is set to a
// per-request latency such as 100ms, and logs wall time, request counts, and
// peak concurrency for each scenario:
//
//	ASC_FANOUT_SIMULATED_LATENCY=100ms go test ./internal/cli/cmdtest -run TestFanOutSimulatedLatencyTimings -v
func TestFanOutSimulatedLatencyTimings(t *testing.T) {
	rawLatency := strings.TrimSpace(os.Getenv("ASC_FANOUT_SIMULATED_LATENCY"))
	if rawLatency == "" {
		t.Skip("set ASC_FANOUT_SIMULATED_LATENCY (for example 100ms) to run the simulated-latency timing harness")
	}
	latency, err := time.ParseDuration(rawLatency)
	if err != nil || latency <= 0 {
		t.Fatalf("invalid ASC_FANOUT_SIMULATED_LATENCY %q", rawLatency)
	}

	scenarios := []struct {
		name    string
		install func(*testing.T, *fanOutProbe)
		args    []string
	}{
		{
			name: fmt.Sprintf("pricing availability edit --all-territories (%d territories)", fanOutTerritoryCount),
			install: func(t *testing.T, probe *fanOutProbe) {
				installPricingAvailabilityFanOutFake(t, probe, fanOutTerritoryCount)
			},
			args: pricingAvailabilityEditAllTerritoriesArgs(),
		},
		{
			name:    fmt.Sprintf("analytics view --paginate (%d reports)", fanOutReportCount),
			install: func(t *testing.T, probe *fanOutProbe) { installAnalyticsViewFanOutFake(t, probe, fanOutReportCount) },
			args:    analyticsViewFanOutArgs(),
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			setupFanOutAuth(t)
			probe := newFanOutProbe(0, 0, latency)
			scenario.install(t, probe)

			start := time.Now()
			_, stderr, err := runFanOutCommand(t, scenario.args...)
			elapsed := time.Since(start)
			if err != nil {
				t.Fatalf("run error: %v (stderr %q)", err, stderr)
			}
			got := probe.snapshot()
			total := 0
			for _, count := range got.counts {
				total += count
			}
			t.Logf("latency=%s wall=%s requests=%d peak_in_flight=%d round_trips=%.1f counts=%v",
				latency, elapsed.Round(time.Millisecond), total, got.maxInFlight, float64(elapsed)/float64(latency), got.counts)
		})
	}
}
