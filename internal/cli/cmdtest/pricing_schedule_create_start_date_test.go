package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
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

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appPriceSchedules":
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
				t.Fatalf("decode create payload: %v", err)
			}
			if len(payload.Included) == 0 {
				t.Fatalf("expected included manual price in create payload")
			}
			capture.startDate = payload.Included[0].Attributes.StartDate
			capture.pricePointID = payload.Included[0].Relationships.AppPricePoint.Data.ID

			body := `{"data":{"type":"appPriceSchedules","id":"sched-1","attributes":{}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	return capture
}

func TestPricingScheduleCreateDefaultsStartDateToTodayUTC(t *testing.T) {
	setupAuth(t)
	capture := stubAppPriceScheduleCreate(t)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	before := time.Now().UTC().Format("2006-01-02")

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

	after := time.Now().UTC().Format("2006-01-02")

	if capture.createCalls != 1 {
		t.Fatalf("expected one create call, got %d", capture.createCalls)
	}
	if capture.startDate != before && capture.startDate != after {
		t.Fatalf("expected default start date %q (or %q), got %q", before, after, capture.startDate)
	}
	if !strings.Contains(stderr, "--start-date not set; using "+capture.startDate+" (today, UTC)") {
		t.Fatalf("expected defaulted start-date note on stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"sched-1"`) {
		t.Fatalf("expected schedule id in output, got %q", stdout)
	}
}

func TestPricingScheduleCreateExplicitStartDateStaysAuthoritative(t *testing.T) {
	setupAuth(t)
	capture := stubAppPriceScheduleCreate(t)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"pricing", "schedule", "create",
			"--app", "app-1",
			"--price-point", "pp-099",
			"--base-territory", "USA",
			"--start-date", "2030-03-01",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if capture.startDate != "2030-03-01" {
		t.Fatalf("expected explicit start date 2030-03-01, got %q", capture.startDate)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr for explicit start date, got %q", stderr)
	}
}

func TestPricingScheduleCreateMalformedStartDateIsUsageError(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var requests int
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

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

	if requests != 0 {
		t.Fatalf("expected no HTTP requests, got %d", requests)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--start-date must be in YYYY-MM-DD format") {
		t.Fatalf("expected expected-format message on stderr, got %q", stderr)
	}
}

func TestAppSetupPricingSetDefaultStartDateAnnouncesTodayUTC(t *testing.T) {
	setupAuth(t)
	capture := stubAppPriceScheduleCreate(t)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	before := time.Now().UTC().Format("2006-01-02")

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

	after := time.Now().UTC().Format("2006-01-02")

	if capture.startDate != before && capture.startDate != after {
		t.Fatalf("expected default start date %q (or %q), got %q", before, after, capture.startDate)
	}
	if !strings.Contains(stderr, "--start-date not set; using "+capture.startDate+" (today, UTC)") {
		t.Fatalf("expected defaulted start-date note on stderr, got %q", stderr)
	}
}
