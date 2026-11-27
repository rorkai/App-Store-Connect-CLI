package pricing

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestPricingScheduleEditPreservesIntervals(t *testing.T) {
	tests := []struct {
		name, point, start, end, fail string
		wantUnchanged                 []string
		wantPrices                    []asc.AppPriceSchedulePrice
		wantError                     string
		noop                          bool
	}{
		{name: "split current interval", point: "sale", end: "2027-01-10", wantUnchanged: []string{"next", "other"}, wantPrices: []asc.AppPriceSchedulePrice{{PricePointID: "regular", EndDate: "2027-01-05"}, {PricePointID: "sale", StartDate: "2027-01-05", EndDate: "2027-01-10"}, {PricePointID: "regular", StartDate: "2027-01-10", EndDate: "2027-01-20"}}},
		{name: "start today omits ended prefix", point: "sale", start: "2027-01-01", end: "2027-01-10", wantUnchanged: []string{"next", "other"}, wantPrices: []asc.AppPriceSchedulePrice{{PricePointID: "sale", StartDate: "2027-01-01", EndDate: "2027-01-10"}, {PricePointID: "regular", StartDate: "2027-01-10", EndDate: "2027-01-20"}}},
		{name: "default end retains next change", point: "sale", wantUnchanged: []string{"next", "other"}, wantPrices: []asc.AppPriceSchedulePrice{{PricePointID: "regular", EndDate: "2027-01-05"}, {PricePointID: "sale", StartDate: "2027-01-05", EndDate: "2027-01-20"}}},
		{name: "end at next boundary", point: "sale", end: "2027-01-20", wantUnchanged: []string{"next", "other"}, wantPrices: []asc.AppPriceSchedulePrice{{PricePointID: "regular", EndDate: "2027-01-05"}, {PricePointID: "sale", StartDate: "2027-01-05", EndDate: "2027-01-20"}}},
		{name: "restore price beyond an intervening change", point: "sale", end: "2027-01-25", wantUnchanged: []string{"other"}, wantPrices: []asc.AppPriceSchedulePrice{{PricePointID: "regular", EndDate: "2027-01-05"}, {PricePointID: "sale", StartDate: "2027-01-05", EndDate: "2027-01-25"}, {PricePointID: "later", StartDate: "2027-01-25"}}},
		{name: "matching price skips write", point: "regular", end: "2027-01-10", noop: true},
		{name: "incomplete second page", point: "sale", fail: "page", wantError: "manual prices"},
		{name: "missing territory", point: "sale", fail: "territory", wantError: "territory"},
		{name: "overlapping intervals", point: "sale", fail: "overlap", wantError: "overlap"},
		{name: "unavailable price point", point: "unavailable", wantError: "base territory"},
		{name: "price changes during planning", point: "sale", fail: "drift", wantError: "schedule changed during the edit"},
		{name: "base territory changes during planning", point: "sale", fail: "base-drift", wantError: "schedule changed during the edit"},
		{name: "ended history", point: "sale", fail: "history", wantError: "ended manual price"},
		{name: "unknown price attribute", point: "sale", fail: "attribute", wantError: "unknown attribute"},
		{name: "unknown price relationship", point: "sale", fail: "relationship", wantError: "unknown relationship"},
		{name: "missing start boundary", point: "sale", fail: "missing-date", wantError: "missing startDate"},
		{name: "sparse continuation", point: "sale", fail: "projection", wantError: "collection query"},
		{name: "wrong territory type", point: "sale", fail: "territory-type", wantError: "territory linkage"},
		{name: "wrong point type", point: "sale", fail: "point-type", wantError: "appPricePoint linkage"},
		{name: "redirected schedule", point: "sale", fail: "redirect", wantError: "read schedule: API request failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			t.Setenv("ASC_APP_ID", "")
			var writes, pages, scheduleReads, redirectedReads int
			var payload asc.AppPriceScheduleCreateRequest
			price := func(id, territory, point, start, end string) map[string]any {
				return map[string]any{"type": "appPrices", "id": id, "attributes": map[string]any{"manual": true, "startDate": start, "endDate": end}, "relationships": map[string]any{"territory": map[string]any{"data": map[string]any{"type": "territories", "id": territory}}, "appPricePoint": map[string]any{"data": map[string]any{"type": "appPricePoints", "id": point}}}}
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method + " " + r.URL.Path {
				case "GET /v1/apps/other/appPriceSchedule":
					redirectedReads++
					fmt.Fprint(w, `{"data":{"type":"appPriceSchedules","id":"schedule"}}`)
				case "GET /v1/apps/app/appPriceSchedule":
					if tt.fail == "redirect" {
						w.Header().Set("Location", "https://api.appstoreconnect.apple.com/v1/apps/other/appPriceSchedule")
						w.WriteHeader(http.StatusFound)
						return
					}
					scheduleReads++
					fmt.Fprint(w, `{"data":{"type":"appPriceSchedules","id":"schedule"}}`)
				case "GET /v1/appPriceSchedules/schedule/baseTerritory":
					if tt.fail == "base-drift" && scheduleReads > 1 {
						fmt.Fprint(w, `{"data":{"type":"territories","id":"CAN"}}`)
					} else {
						fmt.Fprint(w, `{"data":{"type":"territories","id":"USA"}}`)
					}
				case "GET /v1/appPriceSchedules/schedule/manualPrices":
					pages++
					if r.URL.Query().Get("include") != "appPricePoint,territory" {
						t.Errorf("include = %q", r.URL.Query().Get("include"))
					}
					if r.URL.Query().Get("cursor") == "next" {
						if tt.fail == "page" {
							w.WriteHeader(http.StatusForbidden)
							fmt.Fprint(w, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Unavailable"}]}`)
							return
						}
						territory := "CAN"
						if tt.fail == "territory" {
							territory = ""
						}
						other := price("other", territory, "can", "", "")
						if tt.fail == "attribute" {
							other["attributes"].(map[string]any)["newPricingMode"] = true
						}
						if tt.fail == "relationship" {
							other["relationships"].(map[string]any)["newPricingRule"] = map[string]any{"data": nil}
						}
						if tt.fail == "territory-type" {
							other["relationships"].(map[string]any)["territory"].(map[string]any)["data"].(map[string]any)["type"] = "apps"
						}
						if tt.fail == "point-type" {
							other["relationships"].(map[string]any)["appPricePoint"].(map[string]any)["data"].(map[string]any)["type"] = "apps"
						}
						if tt.fail == "missing-date" {
							delete(other["attributes"].(map[string]any), "startDate")
						}
						if err := json.NewEncoder(w).Encode(map[string]any{"data": []any{other}}); err != nil {
							t.Error(err)
						}
						return
					}
					currentPoint := "regular"
					if tt.fail == "drift" && scheduleReads > 1 {
						currentPoint = "changed"
					}
					nextStart := "2027-01-20"
					if tt.fail == "overlap" {
						nextStart = "2027-01-15"
					}
					prices := []any{price("current", "USA", currentPoint, "2026-12-01", "2027-01-20"), price("next", "USA", "later", nextStart, "")}
					if tt.fail == "history" {
						prices = append(prices, price("history", "USA", "history", "2026-01-01", "2026-12-01"))
					}
					next := "https://api.appstoreconnect.apple.com/v1/appPriceSchedules/schedule/manualPrices?cursor=next"
					if tt.fail == "projection" {
						next += "&fields%5BappPrices%5D=manual,startDate,endDate,appPricePoint,territory"
					}
					if err := json.NewEncoder(w).Encode(map[string]any{"data": prices, "links": map[string]string{"next": next}}); err != nil {
						t.Error(err)
					}
				case "GET /v1/apps/app/appPricePoints":
					if r.URL.Query().Get("filter[territory]") != "USA" {
						t.Errorf("territory filter = %q", r.URL.Query().Get("filter[territory]"))
					}
					fmt.Fprint(w, `{"data":[{"type":"appPricePoints","id":"sale"},{"type":"appPricePoints","id":"regular"}]}`)
				case "POST /v1/appPriceSchedules":
					writes++
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
					}
					inline := map[string]bool{}
					for _, p := range payload.Included {
						inline[p.ID] = p.Relationships.AppPricePoint.Data.ID != ""
					}
					for _, p := range payload.Data.Relationships.ManualPrices.Data {
						if !inline[p.ID] {
							w.WriteHeader(http.StatusConflict)
							fmt.Fprint(w, `{"errors":[{"status":"409","code":"ENTITY_ERROR.RELATIONSHIP_REQUIRED","title":"You must provide appPricePoint"}]}`)
							return
						}
					}
					w.WriteHeader(http.StatusCreated)
					fmt.Fprint(w, `{"data":{"type":"appPriceSchedules","id":"saved"}}`)
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
					http.Error(w, "unexpected request", 500)
				}
			}))
			defer server.Close()
			client := currentPriceClient(t, func(r *http.Request) (*http.Response, error) {
				r.URL.Scheme = "http"
				r.URL.Host = strings.TrimPrefix(server.URL, "http://")
				return server.Client().Transport.RoundTrip(r)
			})
			d, out, errout := newTestDeps(func() (*asc.Client, error) { return client, nil })
			d.Timeout = shared.ContextWithTimeout
			d.Now = func() time.Time { return time.Date(2027, 1, 1, 12, 0, 0, 0, time.UTC) }
			start := tt.start
			if start == "" {
				start = "2027-01-05"
			}
			args := []string{"edit", "--app", "app", "--price-point", tt.point, "--start-date", start, "--confirm", "--output", "json"}
			if tt.end != "" {
				args = append(args, "--end-date", tt.end)
			}
			err := newPricingScheduleCommand(d).ParseAndRun(context.Background(), args)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) || writes != 0 || out.Len() != 0 || redirectedReads != 0 {
					t.Fatalf("err=%v writes=%d redirected reads=%d stdout=%q", err, writes, redirectedReads, out.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("run: %v; stderr=%s", err, errout.String())
			}
			wantPages := 4
			if tt.noop {
				wantPages = 2
			}
			if pages != wantPages {
				t.Fatalf("manual price pages = %d", pages)
			}
			var result asc.AppPriceScheduleResponse
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if tt.noop {
				if writes != 0 || result.Data.ID != "schedule" || !strings.Contains(errout.String(), "No changes") {
					t.Fatalf("writes=%d result=%s stderr=%s", writes, out.String(), errout.String())
				}
				return
			}
			if writes != 1 || result.Data.ID != "saved" {
				t.Fatalf("writes=%d output=%s", writes, out.String())
			}
			if payload.Data.Relationships.App.Data.ID != "app" || payload.Data.Relationships.BaseTerritory.Data.ID != "USA" {
				t.Fatalf("schedule relationships: %+v", payload.Data.Relationships)
			}
			linked := map[string]bool{}
			for _, r := range payload.Data.Relationships.ManualPrices.Data {
				if r.Type != asc.ResourceTypeAppPrices {
					t.Errorf("manual type=%s", r.Type)
				}
				linked[r.ID] = true
				if !strings.HasPrefix(r.ID, "${") {
					t.Errorf("price lacks temporary inline ID: %s", r.ID)
				}
			}
			var got []asc.AppPriceSchedulePrice
			for _, p := range payload.Included {
				if !linked[p.ID] {
					t.Errorf("unlinked inline price %s", p.ID)
				}
				got = append(got, asc.AppPriceSchedulePrice{PricePointID: p.Relationships.AppPricePoint.Data.ID, StartDate: p.Attributes.StartDate, EndDate: p.Attributes.EndDate})
			}
			unchanged := map[string]asc.AppPriceSchedulePrice{
				"next":  {PricePointID: "later", StartDate: "2027-01-20"},
				"other": {PricePointID: "can"},
			}
			var want []asc.AppPriceSchedulePrice
			for _, id := range tt.wantUnchanged {
				want = append(want, unchanged[id])
			}
			want = append(want, tt.wantPrices...)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("intervals=%+v want=%+v", got, want)
			}
		})
	}
}

func TestPricingScheduleEditRejectsInputBeforeReading(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"confirmation", nil, "--confirm is required"},
		{"invalid start", []string{"--confirm", "--start-date", "2027-02-30"}, "--start-date must be in YYYY-MM-DD format"},
		{"past start", []string{"--confirm", "--start-date", "2026-12-31"}, "--start-date must be today or later in US Pacific time"},
		{"empty end", []string{"--confirm", "--end-date", ""}, "--end-date must be in YYYY-MM-DD format"},
		{"equal end", []string{"--confirm", "--end-date", "2027-01-05"}, "--end-date must be after --start-date"},
		{"invalid output", []string{"--confirm", "--output", "yaml"}, "--output must be one of: json, table, markdown (got \"yaml\")"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			d, out, errout := newTestDeps(func() (*asc.Client, error) { calls++; return nil, fmt.Errorf("unexpected client creation") })
			d.Now = func() time.Time { return time.Date(2027, 1, 1, 12, 0, 0, 0, time.UTC) }
			args := append([]string{"--app", "app", "--price-point", "sale", "--start-date", "2027-01-05"}, tt.args...)
			c := newPricingScheduleEditCommand(d)
			if err := c.FlagSet.Parse(args); err != nil {
				t.Fatal(err)
			}
			err := c.Exec(context.Background(), nil)
			if !errors.Is(err, flag.ErrHelp) || !strings.Contains(errout.String(), tt.want) || calls != 0 || out.Len() != 0 {
				t.Fatalf("err=%v stderr=%q client calls=%d stdout=%q", err, errout.String(), calls, out.String())
			}
		})
	}
}
