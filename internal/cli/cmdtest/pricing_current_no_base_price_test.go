package cmdtest

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

const pricingCurrentScheduleBody = `{"data":{"type":"appPriceSchedules","id":"schedule-1","attributes":{}}}`

// newPricingCurrentScheduleServer serves an existing price schedule whose base
// territory is USA and whose manual prices are manualPricesBody.
func newPricingCurrentScheduleServer(t *testing.T, manualPricesBody string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appPriceSchedule":
			_, _ = w.Write([]byte(pricingCurrentScheduleBody))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appPriceSchedules/schedule-1/baseTerritory":
			_, _ = w.Write([]byte(`{"data":{"type":"territories","id":"USA","attributes":{"currency":"USD"}}}`))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appPriceSchedules/schedule-1/manualPrices":
			_, _ = w.Write([]byte(manualPricesBody))
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func appPricesBody(prices ...string) string {
	return `{"data":[` + strings.Join(prices, ",") + `],"included":[` +
		`{"type":"appPricePoints","id":"pp-free","attributes":{"customerPrice":"0.00","proceeds":"0.00"}},` +
		`{"type":"appPricePoints","id":"pp-paid","attributes":{"customerPrice":"1.99","proceeds":"1.69"}},` +
		`{"type":"territories","id":"USA","attributes":{"currency":"USD"}}` +
		`],"links":{"next":""}}`
}

func appPriceResource(id, pricePointID, startDate, endDate string) string {
	attributes := `"manual":true`
	if startDate != "" {
		attributes += `,"startDate":"` + startDate + `"`
	}
	if endDate != "" {
		attributes += `,"endDate":"` + endDate + `"`
	}
	return `{"type":"appPrices","id":"` + id + `","attributes":{` + attributes + `},"relationships":{` +
		`"territory":{"data":{"type":"territories","id":"USA"}},` +
		`"appPricePoint":{"data":{"type":"appPricePoints","id":"` + pricePointID + `"}}}}`
}

func TestPricingCurrentNoBaseTerritoryPriceIsExpectedNegative(t *testing.T) {
	tests := []struct {
		name       string
		prices     []string
		wantStdout string
		wantStderr string
	}{
		{
			name:       "no manual prices",
			wantStdout: `{"appId":"app-1","configured":false,"baseTerritory":"USA","reason":"no_current_base_price"}`,
			wantStderr: `App app-1 has no current price for base territory USA; set one with: asc pricing schedule create --app app-1 --free --base-territory "USA"`,
		},
		{
			name:       "base price ended today",
			prices:     []string{appPriceResource("price-1", "pp-paid", "2026-01-01", "2026-03-01")},
			wantStdout: `{"appId":"app-1","configured":false,"baseTerritory":"USA","reason":"no_current_base_price"}`,
			wantStderr: `App app-1 has no current price for base territory USA; set one with: asc pricing schedule create --app app-1 --free --base-territory "USA"`,
		},
		{
			name:       "only a future base price",
			prices:     []string{appPriceResource("price-1", "pp-paid", "2026-03-05", "")},
			wantStdout: `{"appId":"app-1","configured":false,"baseTerritory":"USA","reason":"no_current_base_price","nextStartDate":"2026-03-05"}`,
			wantStderr: `App app-1 has no current price for base territory USA (the next base price starts 2026-03-05); set one with: asc pricing schedule create --app app-1 --free --base-territory "USA"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			fixPricingClock(t, mustParseRFC3339(t, "2026-03-01T20:00:00Z"))
			useServerTransport(t, newPricingCurrentScheduleServer(t, appPricesBody(test.prices...)))

			stdout, stderr, runErr := runRootCommand(t, []string{"pricing", "current", "--app", "app-1", "--output", "json"})

			assertNotConfiguredExpectedNegative(t, runErr)
			if want := test.wantStdout + "\n"; stdout != want {
				t.Fatalf("stdout = %q, want %q", stdout, want)
			}
			if want := test.wantStderr + "\n"; stderr != want {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
			if !strings.Contains(runErr.Error(), `pricing current: app "app-1" has no current price for base territory USA`) {
				t.Fatalf("unexpected error: %v", runErr)
			}
		})
	}
}

func TestPricingCurrentNoBaseTerritoryPriceTableOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	fixPricingClock(t, mustParseRFC3339(t, "2026-03-01T20:00:00Z"))
	useServerTransport(t, newPricingCurrentScheduleServer(t, appPricesBody(appPriceResource("price-1", "pp-paid", "2026-03-05", ""))))

	stdout, _, runErr := runRootCommand(t, []string{"pricing", "current", "--app", "app-1", "--output", "table"})

	assertNotConfiguredExpectedNegative(t, runErr)
	for _, want := range []string{"Base Territory", "Reason", "Next Start Date", "USA", "no_current_base_price", "2026-03-05", "false"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected table to contain %q, got %q", want, stdout)
		}
	}
}

// App Store Connect's day is the US Pacific date, so at 00:30 UTC on
// 2026-03-01 (16:30 PST on 2026-02-28) a price starting 2026-03-01 has not
// started yet and the price ending 2026-03-01 still applies.
func TestPricingCurrentUsesUSPacificDayAtUTCMidnight(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	useServerTransport(t, newPricingCurrentScheduleServer(t, appPricesBody(
		appPriceResource("price-1", "pp-free", "", "2026-03-01"),
		appPriceResource("price-2", "pp-paid", "2026-03-01", ""),
	)))

	tests := []struct {
		now  string
		want string
	}{
		{now: "2026-03-01T00:30:00Z", want: `{"appId":"app-1","baseTerritory":"USA","customerPrice":"0.00","proceeds":"0.00","currency":"USD","isFree":true}`},
		{now: "2026-03-01T08:00:00Z", want: `{"appId":"app-1","baseTerritory":"USA","customerPrice":"1.99","proceeds":"1.69","currency":"USD","isFree":false}`},
	}
	for _, test := range tests {
		t.Run(test.now, func(t *testing.T) {
			fixPricingClock(t, mustParseRFC3339(t, test.now))

			stdout, stderr, runErr := runRootCommand(t, []string{"pricing", "current", "--app", "app-1", "--output", "json"})
			if runErr != nil {
				t.Fatalf("run error: %v (stderr %q)", runErr, stderr)
			}
			if want := test.want + "\n"; stdout != want {
				t.Fatalf("stdout = %q, want %q", stdout, want)
			}
		})
	}
}
