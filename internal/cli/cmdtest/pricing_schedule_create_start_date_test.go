package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// scheduleCreateStartDateCapture records the manual price start date sent to
// POST /v1/appPriceSchedules along with the number of create calls.
type scheduleCreateStartDateCapture struct {
	startDate    string
	createCalls  int
	pricePointID string
}

func stubAppPriceScheduleCreate(t *testing.T) *scheduleCreateStartDateCapture {
	t.Helper()

	capture := &scheduleCreateStartDateCapture{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/appPriceSchedules" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		capture.createCalls++

		var payload struct {
			Included []struct {
				Attributes struct {
					StartDate string `json:"startDate"`
				} `json:"attributes"`
				Relationships struct {
					AppPricePoint struct {
						Data struct {
							ID string `json:"id"`
						} `json:"data"`
					} `json:"appPricePoint"`
				} `json:"relationships"`
			} `json:"included"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Errorf("decode create payload: %v", err)
			http.Error(w, "bad payload", http.StatusBadRequest)
			return
		}
		if len(payload.Included) == 0 {
			t.Errorf("expected included manual price in create payload")
			http.Error(w, "missing manual price", http.StatusBadRequest)
			return
		}
		capture.startDate = payload.Included[0].Attributes.StartDate
		capture.pricePointID = payload.Included[0].Relationships.AppPricePoint.Data.ID

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"data":{"type":"appPriceSchedules","id":"sched-1","attributes":{}}}`)
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	serverTransport := server.Client().Transport
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return serverTransport.RoundTrip(cloned)
	})

	return capture
}

// fixPricingClock pins the clock pricing commands use to default --start-date.
func fixPricingClock(t *testing.T, now time.Time) {
	t.Helper()
	t.Cleanup(shared.SetPricingNowForTesting(func() time.Time { return now }))
}

func mustParseRFC3339(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse %q: %v", value, err)
	}
	return parsed
}

// App Store Connect decides "today" for appPriceSchedules start dates in US
// Pacific time. At 00:03 UTC on 2026-09-27 a startDate of 2026-09-27 (the UTC
// date) returned 409 ENTITY_ERROR.INVALID_START_DATE ("in the future") while
// 2026-09-26 (the Pacific date) returned 201, so the default must follow the
// Pacific calendar on both sides of each DST change.
func TestPricingScheduleCreateDefaultsStartDateToTodayInUSPacific(t *testing.T) {
	tests := []struct {
		name string
		now  string
		want string
	}{
		{name: "after UTC midnight before Pacific midnight (PDT)", now: "2026-09-27T00:03:00Z", want: "2026-09-26"},
		{name: "after Pacific midnight (PDT)", now: "2026-09-27T08:30:00Z", want: "2026-09-27"},
		{name: "before Pacific midnight just before spring forward (PST)", now: "2026-03-08T07:59:00Z", want: "2026-03-07"},
		{name: "after Pacific midnight on spring forward day (PST)", now: "2026-03-08T08:00:00Z", want: "2026-03-08"},
		{name: "after spring forward uses PDT offset", now: "2026-03-09T07:30:00Z", want: "2026-03-09"},
		{name: "before Pacific midnight on fall back day (PDT)", now: "2026-11-01T06:59:00Z", want: "2026-10-31"},
		{name: "after fall back uses PST offset", now: "2026-11-02T07:30:00Z", want: "2026-11-01"},
		{name: "after Pacific midnight following fall back (PST)", now: "2026-11-02T08:00:00Z", want: "2026-11-02"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			capture := stubAppPriceScheduleCreate(t)
			fixPricingClock(t, mustParseRFC3339(t, test.now))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"pricing", "schedule", "create",
					"--app", "app-1",
					"--price-point", "pp-099",
					"--base-territory", "USA",
					"--output", "json",
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if capture.createCalls != 1 {
				t.Fatalf("expected one create call, got %d", capture.createCalls)
			}
			if capture.startDate != test.want {
				t.Fatalf("startDate = %q, want %q", capture.startDate, test.want)
			}
			wantNote := "Note: --start-date not set; using " + test.want + " (today in US Pacific time, which App Store Connect uses). Apple requires today or later.\n"
			if stderr != wantNote {
				t.Fatalf("stderr = %q, want %q", stderr, wantNote)
			}
			if !strings.Contains(stdout, `"id":"sched-1"`) {
				t.Fatalf("expected schedule id in output, got %q", stdout)
			}
		})
	}
}

func TestPricingScheduleCreateExplicitStartDateStaysAuthoritative(t *testing.T) {
	// The clock sits between UTC midnight and Pacific midnight, where the
	// Pacific default differs from the UTC date; explicit values must still be
	// sent exactly as given.
	for _, startDate := range []string{"2026-09-27", "2026-09-26", "2030-03-01"} {
		t.Run(startDate, func(t *testing.T) {
			setupAuth(t)
			capture := stubAppPriceScheduleCreate(t)
			fixPricingClock(t, mustParseRFC3339(t, "2026-09-27T00:03:00Z"))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			_, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"pricing", "schedule", "create",
					"--app", "app-1",
					"--price-point", "pp-099",
					"--base-territory", "USA",
					"--start-date", startDate,
					"--output", "json",
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if capture.startDate != startDate {
				t.Fatalf("expected explicit start date %s, got %q", startDate, capture.startDate)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr for explicit start date, got %q", stderr)
			}
		})
	}
}

func TestPricingScheduleCreateMalformedStartDateIsUsageError(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"pricing", "schedule", "create",
			"--app", "app-1",
			"--price-point", "pp-099",
			"--base-territory", "USA",
			"--start-date", "03/01/2030",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr := root.Run(context.Background())
		if runErr == nil {
			t.Fatalf("expected malformed start date to fail")
		}
		if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", got, rootcmd.ExitUsage)
		}
	})

	if requests := len(reg.Requests()); requests != 0 {
		t.Fatalf("expected no HTTP requests, got %d", requests)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--start-date must be in YYYY-MM-DD format") {
		t.Fatalf("expected expected-format message on stderr, got %q", stderr)
	}
}

func TestAppSetupPricingSetDefaultStartDateUsesTodayInUSPacific(t *testing.T) {
	setupAuth(t)
	capture := stubAppPriceScheduleCreate(t)
	fixPricingClock(t, mustParseRFC3339(t, "2026-09-27T00:03:00Z"))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"app-setup", "pricing", "set",
			"--app", "app-1",
			"--price-point", "pp-099",
			"--base-territory", "USA",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if capture.startDate != "2026-09-26" {
		t.Fatalf("startDate = %q, want 2026-09-26", capture.startDate)
	}
	if !strings.Contains(stderr, "--start-date not set; using 2026-09-26 (today in US Pacific time") {
		t.Fatalf("expected defaulted start-date note on stderr, got %q", stderr)
	}
}
