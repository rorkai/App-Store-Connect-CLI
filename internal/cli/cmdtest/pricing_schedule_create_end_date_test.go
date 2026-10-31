package cmdtest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type scheduleManualPrice struct {
	PricePoint string
	StartDate  *string
	EndDate    *string
}

func dateOf(value string) *string { return &value }

// stubTemporaryPriceCreate records every manual price of each schedule create.
func stubTemporaryPriceCreate(t *testing.T) *[][]scheduleManualPrice {
	t.Helper()
	creates := &[][]scheduleManualPrice{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/appPriceSchedules" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		var payload struct {
			Data struct {
				Relationships struct {
					ManualPrices struct {
						Data []struct {
							ID string `json:"id"`
						} `json:"data"`
					} `json:"manualPrices"`
				} `json:"relationships"`
			} `json:"data"`
			Included []struct {
				ID         string `json:"id"`
				Attributes struct {
					StartDate *string `json:"startDate"`
					EndDate   *string `json:"endDate"`
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
		linked := payload.Data.Relationships.ManualPrices.Data
		if len(linked) != len(payload.Included) {
			t.Errorf("manualPrices links %d entries, included has %d", len(linked), len(payload.Included))
		}
		prices := make([]scheduleManualPrice, 0, len(payload.Included))
		for index, entry := range payload.Included {
			if index < len(linked) && linked[index].ID != entry.ID {
				t.Errorf("manualPrices[%d] = %q, included id = %q", index, linked[index].ID, entry.ID)
			}
			prices = append(prices, scheduleManualPrice{
				PricePoint: entry.Relationships.AppPricePoint.Data.ID,
				StartDate:  entry.Attributes.StartDate,
				EndDate:    entry.Attributes.EndDate,
			})
		}
		*creates = append(*creates, prices)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"type":"appPriceSchedules","id":"sched-1","attributes":{}}}`))
	}))
	t.Cleanup(server.Close)
	useServerTransport(t, server)
	return creates
}

func describeManualPrices(prices []scheduleManualPrice) string {
	parts := make([]string, 0, len(prices))
	for _, price := range prices {
		start, end := "null", "null"
		if price.StartDate != nil {
			start = *price.StartDate
		}
		if price.EndDate != nil {
			end = *price.EndDate
		}
		parts = append(parts, price.PricePoint+"["+start+","+end+")")
	}
	return strings.Join(parts, " ")
}

// App Store Connect rejects a base territory timeline that does not cover
// today, and ends one price on the date the next starts, so a temporary price
// is always sent with the entries that surround it.
func TestPricingScheduleCreateTemporaryPriceSendsContinuousTimeline(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		want       []scheduleManualPrice
		wantStderr string
	}{
		{
			name: "starting today",
			args: []string{"--start-date", "2026-10-31", "--end-date", "2026-11-08"},
			want: []scheduleManualPrice{
				{PricePoint: "pp-sale", StartDate: dateOf("2026-10-31"), EndDate: dateOf("2026-11-08")},
				{PricePoint: "pp-regular", StartDate: dateOf("2026-11-08")},
			},
		},
		{
			name: "defaulted start date",
			args: []string{"--end-date", "2026-11-08"},
			want: []scheduleManualPrice{
				{PricePoint: "pp-sale", StartDate: dateOf("2026-10-31"), EndDate: dateOf("2026-11-08")},
				{PricePoint: "pp-regular", StartDate: dateOf("2026-11-08")},
			},
			wantStderr: "Note: --start-date not set; using 2026-10-31 (today in US Pacific time, which App Store Connect uses). Apple requires today or later.\n",
		},
		{
			name: "starting in the future covers today with the return price",
			args: []string{"--start-date", "2026-11-20", "--end-date", "2026-11-30"},
			want: []scheduleManualPrice{
				{PricePoint: "pp-regular", EndDate: dateOf("2026-11-20")},
				{PricePoint: "pp-sale", StartDate: dateOf("2026-11-20"), EndDate: dateOf("2026-11-30")},
				{PricePoint: "pp-regular", StartDate: dateOf("2026-11-30")},
			},
			wantStderr: "Note: --return-price-point also applies from today until 2026-11-20, because --start-date is in the future.\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			creates := stubTemporaryPriceCreate(t)
			fixPricingClock(t, mustParseRFC3339(t, "2026-10-31T12:00:00Z"))

			args := append([]string{
				"pricing", "schedule", "create", "--app", "app-1", "--price-point", "pp-sale",
				"--base-territory", "USA", "--return-price-point", "pp-regular", "--confirm", "--output", "json",
			}, test.args...)
			stdout, stderr, runErr := runCommand(t, args)

			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}
			if len(*creates) != 1 {
				t.Fatalf("expected one create request, got %d", len(*creates))
			}
			if got := (*creates)[0]; !reflect.DeepEqual(got, test.want) {
				t.Fatalf("manual prices = %s, want %s", describeManualPrices(got), describeManualPrices(test.want))
			}
			if stderr != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if !strings.Contains(stdout, `"id":"sched-1"`) {
				t.Fatalf("expected Apple's schedule response on stdout, got %q", stdout)
			}
		})
	}
}

func TestPricingScheduleCreateTemporaryPriceUsageErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "end date on the start date",
			args:       []string{"--start-date", "2026-11-08", "--end-date", "2026-11-08", "--return-price-point", "pp-regular", "--confirm"},
			wantStderr: "--end-date must be after the start date 2026-11-08",
		},
		{
			name:       "end date before the start date",
			args:       []string{"--start-date", "2026-11-08", "--end-date", "2026-11-07", "--return-price-point", "pp-regular", "--confirm"},
			wantStderr: "--end-date must be after the start date 2026-11-08",
		},
		{
			name:       "end date not after the default start date",
			args:       []string{"--end-date", "2026-10-31", "--return-price-point", "pp-regular", "--confirm"},
			wantStderr: "--end-date must be after the start date 2026-10-31",
		},
		{
			name:       "malformed end date",
			args:       []string{"--end-date", "11/08/2026", "--return-price-point", "pp-regular", "--confirm"},
			wantStderr: "--end-date must be in YYYY-MM-DD format",
		},
		{
			name:       "impossible end date",
			args:       []string{"--end-date", "2026-11-31", "--return-price-point", "pp-regular", "--confirm"},
			wantStderr: "--end-date must be in YYYY-MM-DD format",
		},
		{
			name:       "empty end date never becomes a permanent price",
			args:       []string{"--end-date", ""},
			wantStderr: "--end-date must be in YYYY-MM-DD format",
		},
		{
			name:       "end date without a price to return to",
			args:       []string{"--end-date", "2026-11-08", "--confirm"},
			wantStderr: "--end-date requires --return-price-point",
		},
		{
			name:       "return price without an end date",
			args:       []string{"--return-price-point", "pp-regular", "--confirm"},
			wantStderr: "--return-price-point requires --end-date",
		},
		{
			name:       "temporary price without confirmation",
			args:       []string{"--end-date", "2026-11-08", "--return-price-point", "pp-regular"},
			wantStderr: "--confirm is required with --end-date because this replaces the app's whole price schedule",
		},
		{
			name:       "confirm without a temporary price",
			args:       []string{"--confirm"},
			wantStderr: "--confirm requires --end-date",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			creates := stubTemporaryPriceCreate(t)
			fixPricingClock(t, mustParseRFC3339(t, "2026-10-31T12:00:00Z"))

			args := append([]string{
				"pricing", "schedule", "create", "--app", "app-1", "--price-point", "pp-sale", "--base-territory", "USA",
			}, test.args...)
			stdout, stderr, runErr := runCommand(t, args)

			if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, cmd.ExitUsage, runErr)
			}
			if len(*creates) != 0 {
				t.Fatalf("expected no create request, got %d", len(*creates))
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
		})
	}
}

// The default start date is resolved after the --free lookup, so a run crossing
// US Pacific midnight must not send an interval that ends on the day it starts.
func TestPricingScheduleCreateTemporaryPriceRechecksDefaultStartDate(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	server, creates := newAppPriceScheduleCreateServer(t, http.StatusCreated, `{"data":{"type":"appPriceSchedules","id":"sched-1"}}`)
	useServerTransport(t, server)

	beforeMidnight := mustParseRFC3339(t, "2026-11-01T06:59:00Z")
	reads := 0
	t.Cleanup(shared.SetPricingNowForTesting(func() time.Time {
		reads++
		if reads == 1 {
			return beforeMidnight
		}
		return beforeMidnight.Add(2 * time.Minute)
	}))

	_, stderr, runErr := runCommand(t, []string{
		"pricing", "schedule", "create", "--app", "app-1", "--free", "--base-territory", "USA",
		"--end-date", "2026-11-01", "--return-price-point", "pp-regular", "--confirm",
	})

	if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, cmd.ExitUsage, runErr)
	}
	if *creates != 0 {
		t.Fatalf("expected no create request, got %d", *creates)
	}
	if !strings.Contains(stderr, "--end-date must be after the start date 2026-11-01") {
		t.Fatalf("stderr = %q", stderr)
	}
}

// The guidance for these 409s assumes one price and names --price-point or
// --start-date as the fix, which would blame the wrong flag for a temporary
// price. Apple's own text names the rejected interval or price point.
func TestPricingScheduleCreateTemporaryPriceKeepsAppleConflictText(t *testing.T) {
	for _, fixture := range []string{"start_date_future.json", "base_territory_interval_required.json", "price_point_not_found.json"} {
		t.Run(fixture, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			server, _ := newAppPriceScheduleCreateServer(t, http.StatusConflict, readPricingScheduleConflictFixture(t, fixture))
			useServerTransport(t, server)
			fixPricingClock(t, mustParseRFC3339(t, "2026-10-31T12:00:00Z"))

			_, _, runErr := runCommand(t, []string{
				"pricing", "schedule", "create", "--app", "app-1", "--price-point", "pp-sale", "--base-territory", "USA",
				"--end-date", "2026-11-08", "--return-price-point", "pp-regular", "--confirm",
			})

			if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitConflict {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, cmd.ExitConflict, runErr)
			}
			want := "pricing schedule create: There is a problem with the request entity"
			if !strings.HasPrefix(runErr.Error(), want) {
				t.Fatalf("error = %q, want prefix %q", runErr.Error(), want)
			}
		})
	}
}
