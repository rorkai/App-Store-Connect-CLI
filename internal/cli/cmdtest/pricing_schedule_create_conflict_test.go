package cmdtest

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// The fixtures under testdata/pricing_schedule_create_409 are Apple's raw
// response bodies for POST /v1/appPriceSchedules, captured live against the
// disposable app 6759231657 on 2026-09-26 and 2026-09-27 and checked in byte
// for byte (including Apple's per-response error id).
func readPricingScheduleConflictFixture(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "pricing_schedule_create_409", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(body)
}

// newAppPriceScheduleCreateServer answers POST /v1/appPriceSchedules with the
// given status and body, answers the free price point lookup, and fails the
// test on any other request.
func newAppPriceScheduleCreateServer(t *testing.T, status int, body string) (*httptest.Server, *int) {
	t.Helper()
	creates := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appPriceSchedules":
			creates++
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appPricePoints":
			_, _ = w.Write([]byte(`{"data":[{"type":"appPricePoints","id":"pp-free","attributes":{"customerPrice":"0.0","proceeds":"0.0"}}],"links":{}}`))
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)
	return server, &creates
}

func TestPricingScheduleCreateExplainsKnownConflicts(t *testing.T) {
	tests := []struct {
		name          string
		fixture       string
		args          []string
		wantMessage   string
		wantAppleText string
		wantCode      shared.DiagnosticCode
		wantParameter string
	}{
		{
			name:    "start date in the past",
			fixture: "start_date_past.json",
			args: []string{
				"pricing", "schedule", "create", "--app", "app-1", "--price-point", "pp-1",
				"--base-territory", "USA", "--start-date", "2026-09-25",
			},
			wantMessage:   "pricing schedule create: --start-date 2026-09-25 is in the past. Pass --start-date with today's date in US Pacific time. App Store Connect used the US Pacific date as today in live checks, so for several hours after UTC midnight it rejects the UTC date as in the future.",
			wantAppleText: "Interval can not have a start date in the past",
			wantCode:      shared.DiagnosticInvalidInput,
			wantParameter: "--start-date",
		},
		{
			name:    "start date in the future",
			fixture: "start_date_future.json",
			args: []string{
				"pricing", "schedule", "create", "--app", "app-1", "--price-point", "pp-1",
				"--base-territory", "USA", "--start-date", "2026-10-26",
			},
			wantMessage:   "pricing schedule create: --start-date 2026-10-26 is in the future. This command replaces the app's whole price schedule with this one price, and App Store Connect requires the schedule to cover today, so a future price change cannot be scheduled with it. Use today's date in US Pacific time.",
			wantAppleText: "Entire timeline must be covered for USA. The first interval has a start date 2026-10-26T00:00 in the future",
			wantCode:      shared.DiagnosticInvalidInput,
			wantParameter: "--start-date",
		},
		{
			name:    "price point outside the base territory",
			fixture: "base_territory_interval_required.json",
			args: []string{
				"pricing", "schedule", "create", "--app", "app-1", "--price-point", "pp-usa",
				"--base-territory", "FRA", "--start-date", "2026-09-26",
			},
			wantMessage:   "pricing schedule create: the price point is not a FRA price point, and App Store Connect requires the price to be set in the base territory (FRA). Pass a --price-point listed by `asc pricing price-points --app app-1 --territory FRA`, or use --price, --tier, or --free",
			wantAppleText: "There must be at least one manual price for the base territory.",
			wantCode:      shared.DiagnosticConflictingInput,
			wantParameter: "--base-territory",
		},
		{
			name:    "unknown price point",
			fixture: "price_point_not_found.json",
			args: []string{
				"pricing", "schedule", "create", "--app", "app-1", "--price-point", "not-a-price-point",
				"--base-territory", "USA", "--start-date", "2026-09-26",
			},
			wantMessage:   "pricing schedule create: price point \"not-a-price-point\" was not found for app app-1. Price point IDs belong to one app and territory; list valid IDs with `asc pricing price-points --app app-1 --territory USA`.",
			wantAppleText: "The resource 'appPricePoints' with id 'not-a-price-point' was not found.",
			wantCode:      shared.DiagnosticResourceNotFound,
			wantParameter: "--price-point",
		},
		{
			name:    "app-setup pricing set shares the mapping",
			fixture: "start_date_past.json",
			args: []string{
				"app-setup", "pricing", "set", "--app", "app-1", "--price-point", "pp-1",
				"--base-territory", "USA", "--start-date", "2026-09-25",
			},
			wantMessage:   "app-setup pricing set: --start-date 2026-09-25 is in the past.",
			wantAppleText: "Interval can not have a start date in the past",
			wantCode:      shared.DiagnosticInvalidInput,
			wantParameter: "--start-date",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			server, creates := newAppPriceScheduleCreateServer(t, http.StatusConflict, readPricingScheduleConflictFixture(t, test.fixture))
			useServerTransport(t, server)

			stdout, stderr, runErr := runCommand(t, append(test.args, "--output", "json"))

			if runErr == nil {
				t.Fatal("expected conflict error")
			}
			if *creates != 1 {
				t.Fatalf("expected exactly one create request, got %d", *creates)
			}
			message := runErr.Error()
			if !strings.HasPrefix(message, test.wantMessage) {
				t.Fatalf("error = %q, want prefix %q", message, test.wantMessage)
			}
			if !strings.Contains(message, "\n\nApp Store Connect: There is a problem with the request entity: "+test.wantAppleText) {
				t.Fatalf("error = %q, want Apple's original response %q preserved", message, test.wantAppleText)
			}
			var apiErr *asc.APIError
			if !errors.As(runErr, &apiErr) || apiErr.StatusCode != http.StatusConflict {
				t.Fatalf("expected the 409 API error to stay in the chain, got %v", runErr)
			}
			if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitConflict {
				t.Fatalf("exit code = %d, want %d", got, cmd.ExitConflict)
			}
			diagnostic, ok := shared.DiagnosticFromError(runErr)
			if !ok || diagnostic.Code != test.wantCode || diagnostic.Parameter != test.wantParameter {
				t.Fatalf("diagnostic = %+v (ok=%v), want code=%q parameter=%q", diagnostic, ok, test.wantCode, test.wantParameter)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestPricingScheduleCreateLeavesOtherConflictsUnchanged(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{
			name:   "unrecognized 409 code",
			status: http.StatusConflict,
			body:   `{"errors":[{"id":"00000000-0000-0000-0000-000000000000","status":"409","code":"STATE_ERROR","title":"The request cannot be fulfilled because of the state of another resource.","detail":"Synthetic state conflict."}]}`,
		},
		{
			name:   "start date code with an unrecognized detail",
			status: http.StatusConflict,
			body:   `{"errors":[{"id":"00000000-0000-0000-0000-000000000000","status":"409","code":"ENTITY_ERROR.INVALID_START_DATE","title":"There is a problem with the request entity","detail":"Synthetic start date rejection."}]}`,
		},
		{
			name:   "start date code paired with another entry's past-date detail",
			status: http.StatusConflict,
			body:   `{"errors":[{"id":"00000000-0000-0000-0000-000000000000","status":"409","code":"ENTITY_ERROR.INVALID_START_DATE","title":"There is a problem with the request entity","detail":"Synthetic start date rejection."},{"id":"00000000-0000-0000-0000-000000000001","status":"409","code":"STATE_ERROR","title":"Synthetic state conflict","detail":"Interval can not have a start date in the past"}]}`,
		},
		{
			name:   "not found for another resource type",
			status: http.StatusConflict,
			body:   `{"errors":[{"id":"00000000-0000-0000-0000-000000000000","status":"409","code":"ENTITY_ERROR.NOT_FOUND","title":"There is a problem with the request entity","detail":"The resource 'territories' with id 'XYZ' was not found."}]}`,
		},
		{
			name:   "known code on a non-409 status",
			status: http.StatusUnprocessableEntity,
			body:   `{"errors":[{"id":"00000000-0000-0000-0000-000000000000","status":"422","code":"ENTITY_ERROR.INVALID_START_DATE","title":"There is a problem with the request entity","detail":"Interval can not have a start date in the past"}]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			server, _ := newAppPriceScheduleCreateServer(t, test.status, test.body)
			useServerTransport(t, server)

			_, _, runErr := runCommand(t, []string{
				"pricing", "schedule", "create", "--app", "app-1", "--price-point", "pp-1",
				"--base-territory", "USA", "--start-date", "2026-09-25", "--output", "json",
			})

			if runErr == nil {
				t.Fatal("expected API error")
			}
			want := "pricing schedule create: " + asc.ParseErrorWithStatus([]byte(test.body), test.status).Error()
			if runErr.Error() != want {
				t.Fatalf("error = %q, want unchanged %q", runErr.Error(), want)
			}
			if diagnostic, ok := shared.DiagnosticFromError(runErr); ok {
				t.Fatalf("expected no diagnostic for an unrecognized response, got %+v", diagnostic)
			}
			if got, want := cmd.ExitCodeFromError(runErr), cmd.HTTPStatusToExitCode(test.status); got != want {
				t.Fatalf("exit code = %d, want %d for status %d", got, want, test.status)
			}
		})
	}
}

func TestPricingScheduleCreateExplainsUnknownResolvedPricePoint(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	body := strings.ReplaceAll(readPricingScheduleConflictFixture(t, "price_point_not_found.json"), "not-a-price-point", "pp-free")
	server, _ := newAppPriceScheduleCreateServer(t, http.StatusConflict, body)
	useServerTransport(t, server)

	_, _, runErr := runCommand(t, []string{
		"pricing", "schedule", "create", "--app", "app-1", "--free",
		"--base-territory", "USA", "--start-date", "2026-09-26", "--output", "json",
	})

	if runErr == nil {
		t.Fatal("expected conflict error")
	}
	want := `pricing schedule create: App Store Connect did not find price point "pp-free", resolved from --free, for app app-1.`
	if !strings.HasPrefix(runErr.Error(), want+"\n\n") {
		t.Fatalf("error = %q, want prefix %q", runErr.Error(), want)
	}
	diagnostic, ok := shared.DiagnosticFromError(runErr)
	if !ok || diagnostic.Code != shared.DiagnosticResourceNotFound || diagnostic.Parameter != "--free" {
		t.Fatalf("diagnostic = %+v (ok=%v), want resource_not_found on --free", diagnostic, ok)
	}
}

func TestPricingScheduleCreateExplainsDefaultStartDateAheadOfAppStoreConnect(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	// Captured at 00:03 UTC on 2026-09-27: the UTC date was rejected as in the
	// future while the US Pacific date (2026-09-26) was accepted. The default
	// is now the US Pacific date, so the same rejection of a defaulted date
	// points at the local clock or an undocumented App Store Connect time zone.
	t.Cleanup(setRootNow(func() time.Time {
		return time.Date(2026, time.September, 27, 0, 3, 0, 0, time.UTC)
	}))
	server, _ := newAppPriceScheduleCreateServer(t, http.StatusConflict, readPricingScheduleConflictFixture(t, "start_date_utc_today_future.json"))
	useServerTransport(t, server)

	_, stderr, runErr := runCommand(t, []string{
		"pricing", "schedule", "create", "--app", "app-1", "--price-point", "pp-1",
		"--base-territory", "USA", "--output", "json",
	})

	if runErr == nil {
		t.Fatal("expected conflict error")
	}
	if !strings.Contains(stderr, "Note: --start-date not set; using 2026-09-26 (today in US Pacific time") {
		t.Fatalf("expected the US Pacific default start date note on stderr, got %q", stderr)
	}
	message := runErr.Error()
	want := "pricing schedule create: App Store Connect treats the default start date 2026-09-26 (today in US Pacific time) as in the future. " +
		"App Store Connect used the US Pacific date as today in live checks, but its time zone is not documented. " +
		"Check the system clock, or pass --start-date with the date App Store Connect accepts as today."
	if !strings.HasPrefix(message, want+"\n\n") {
		t.Fatalf("error = %q, want prefix %q", message, want)
	}
	if !strings.Contains(message, "\n\nApp Store Connect: There is a problem with the request entity: Entire timeline must be covered for USA.") {
		t.Fatalf("error = %q, want Apple's original response preserved", message)
	}
	diagnostic, ok := shared.DiagnosticFromError(runErr)
	if !ok || diagnostic.Code != shared.DiagnosticInvalidInput || diagnostic.Parameter != "--start-date" {
		t.Fatalf("diagnostic = %+v (ok=%v), want invalid_input on --start-date", diagnostic, ok)
	}
	if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitConflict {
		t.Fatalf("exit code = %d, want %d", got, cmd.ExitConflict)
	}
}
