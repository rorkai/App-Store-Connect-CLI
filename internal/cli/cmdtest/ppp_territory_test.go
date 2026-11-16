package cmdtest

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/handlertest"
)

// These scenarios guard replacement payloads, including future intervals that
// current-price resolution cannot preserve.
func TestPPPTerritorySchedulePreservation(t *testing.T) {
	for _, surface := range []scheduleSurfaceFixture{appScheduleSurface, iapScheduleSurface} {
		for _, test := range []struct {
			name, selection, start, failure string
			noop, dryRun                    bool
		}{
			{name: "immediate", selection: "IN,India"},
			{name: "future", selection: "IND", start: "2026-11-15"},
			{name: "automatic prefix", selection: "IND", start: "2026-11-15"},
			{name: "readback drops automatic prefix", selection: "IND", start: "2026-11-15", failure: "preservation verification failed"},
			{name: "overlapping intervals", selection: "IND", failure: "overlapping schedule intervals"},
			{name: "future changes current noop", selection: "IND", start: "2026-11-15"},
			{name: "noop base keeps future", selection: "IND,USA"},
			{name: "dry run", selection: "IND", dryRun: true},
			{name: "base guard", selection: "USA", failure: "cannot change the schedule base timeline"},
			{name: "all selected base guard", selection: "USA,IND,FRA,JPN,TWN", failure: "cannot change the schedule base timeline"},
			{name: "missing target", selection: "BRA", failure: "requested territory BRA has no current price"},
			{name: "noop keeps future", selection: "IND", noop: true},
			{name: "missing future relationship", selection: "IND", failure: "cannot resolve complete schedule price"},
			{name: "later page failure", selection: "IND", failure: "page unavailable"},
			{name: "planning drift", selection: "IND", failure: "price schedule changed during planning"},
			{name: "readback loses future", selection: "IND", failure: "preservation verification failed"},
			{name: "readback changes mode", selection: "IND", failure: "preservation verification failed"},
			{name: "readback automatic amount drift", selection: "IND", failure: "observed automatic pricing readback drift"},
			{name: "readback shared decoded price drift", selection: "IND", failure: "observed automatic pricing readback drift"},
		} {
			if test.name == "readback shared decoded price drift" && surface.priceType != "inAppPurchasePrices" {
				continue
			}
			t.Run(surface.priceType+"/"+test.name, func(t *testing.T) {
				setupAuth(t)
				t.Setenv("ASC_APP_ID", "")
				restore := setRootNow(func() time.Time { return time.Date(2026, 10, 29, 2, 0, 0, 0, time.UTC) })
				defer restore()
				fixture := handlertest.New(t)
				posts, reads := []string{}, 0
				canonical := func(territory, id string) string {
					if surface.priceType != "inAppPurchasePrices" {
						return id
					}
					payload, _ := json.Marshal(map[string]string{"s": "9000000001", "t": territory, "p": id})
					return base64.RawURLEncoding.EncodeToString(payload)
				}
				priceValues := make(map[string]string)
				point := func(territory, id, price, start, end string, manual bool) map[string]any {
					priceValues[canonical(territory, id)] = price
					return map[string]any{"type": surface.priceType, "id": canonical(territory, id), "attributes": map[string]any{"manual": manual, "startDate": start, "endDate": end}, "relationships": map[string]any{"territory": map[string]any{"data": map[string]string{"type": "territories", "id": territory}}, surface.pointRel: map[string]any{"data": map[string]string{"type": surface.pointType, "id": canonical(territory, id)}}}}
				}
				encode := func(data []map[string]any, next string) string {
					included := []map[string]any{}
					for _, item := range data {
						rels := item["relationships"].(map[string]any)
						if rels[surface.pointRel] != nil {
							id := rels[surface.pointRel].(map[string]any)["data"].(map[string]string)["id"]
							included = append(included, map[string]any{"type": surface.pointType, "id": id, "attributes": map[string]string{"customerPrice": priceValues[id]}})
						}
					}
					body, _ := json.Marshal(map[string]any{"data": data, "included": included, "links": map[string]string{"next": next}})
					return string(body)
				}
				installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case req.Method == http.MethodGet && req.URL.Path == surface.schedulePath:
						return jsonResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":%q,"id":"sched-1"}}`, surface.scheduleType))
					case req.Method == http.MethodGet && req.URL.Path == surface.pricesPath+"baseTerritory":
						return jsonResponse(http.StatusOK, `{"data":{"type":"territories","id":"USA"}}`)
					case req.Method == http.MethodGet && req.URL.Path == surface.pricesPath+"manualPrices":
						if req.URL.Query().Get("page") == "2" {
							if test.name == "later page failure" {
								return jsonResponse(http.StatusBadRequest, `{"errors":[{"status":"400","detail":"page unavailable"}]}`)
							}
							future := point("FRA", "fra-future", "5.99", "2026-11-10", "", true)
							if test.name == "missing future relationship" {
								delete(future["relationships"].(map[string]any), surface.pointRel)
								future["id"] = "unresolved-future"
							}
							if test.name == "readback loses future" && len(posts) > 0 {
								return jsonResponse(http.StatusOK, encode(nil, ""))
							}
							data := []map[string]any{future}
							if test.name == "overlapping intervals" {
								future["attributes"].(map[string]any)["startDate"] = "2026-11-09"
							}
							if test.name == "noop base keeps future" {
								data = append(data, point("USA", "usa-499", "4.99", "2026-11-10", "", true))
							}
							return jsonResponse(http.StatusOK, encode(data, ""))
						}
						reads++
						ind := "ind-199"
						if test.noop || test.name == "future changes current noop" || (len(posts) > 0 && test.start == "") {
							ind = "ind-115"
						}
						end := "2026-11-05"
						data := []map[string]any{point("USA", "usa-599", "5.99", "2026-01-01", "", true), point("FRA", "fra-current", "4.99", "", "2026-11-10", true)}
						if test.name == "noop base keeps future" {
							data[0]["attributes"].(map[string]any)["endDate"] = "2026-11-10"
						}
						if len(posts) > 0 && test.start == "" {
							data = append(data, point("IND", ind, "115", "", "", true))
						} else {
							data = append(data, point("IND", ind, "199", "", end, true), point("IND", "ind-199", "199", end, "", true))
							if len(posts) > 0 {
								data[len(data)-1]["attributes"].(map[string]any)["endDate"] = test.start
								data = append(data, point("IND", "ind-115", "115", test.start, "", true))
							}
						}
						if strings.Contains(test.name, "automatic prefix") {
							kept := data[:0]
							for _, item := range data {
								territory := item["relationships"].(map[string]any)["territory"].(map[string]any)["data"].(map[string]string)["id"]
								if territory != "IND" {
									kept = append(kept, item)
								}
							}
							data = kept
							if len(posts) > 0 {
								data = append(data, point("IND", "ind-115", "115", test.start, "", true))
							}
						}
						if test.name == "planning drift" && reads > 1 {
							data[0] = point("USA", "usa-499", "4.99", "", "", true)
						}
						if test.name == "readback changes mode" && len(posts) > 0 {
							data = append(data, point("JPN", "jpn-auto", "700", "", "", true))
						}
						return jsonResponse(http.StatusOK, encode(data, "https://api.appstoreconnect.apple.com"+surface.pricesPath+"manualPrices?page=2"))
					case req.Method == http.MethodGet && req.URL.Path == surface.pricesPath+"automaticPrices":
						if test.name == "readback changes mode" && len(posts) > 0 {
							return jsonResponse(http.StatusOK, encode(nil, ""))
						}
						automaticPrice := "700"
						if (test.name == "readback automatic amount drift" || test.name == "readback shared decoded price drift") && len(posts) > 0 {
							automaticPrice = "701"
						}
						data := []map[string]any{point("JPN", "jpn-auto", automaticPrice, "", "", false)}
						if test.name == "readback shared decoded price drift" {
							data = []map[string]any{point("JPN", "10001", automaticPrice, "", "", false), point("KOR", "10001", "7000", "", "", false)}
						}
						if strings.Contains(test.name, "automatic prefix") && (test.name != "readback drops automatic prefix" || len(posts) == 0) {
							data = append(data, point("IND", "ind-199", "199", "", "", false))
						}
						if test.name == "all selected base guard" {
							data = append(data, point("TWN", "twn-150", "150", "", "", false))
						}
						return jsonResponse(http.StatusOK, encode(data, ""))
					case req.Method == http.MethodGet && req.URL.Path == surface.ladderPath:
						points := []schedulePoint{}
						for _, territory := range strings.Split(req.URL.Query().Get("filter[territory]"), ",") {
							switch territory {
							case "USA":
								points = append(points, schedulePoint{"USA", "usa-499", "4.99"}, schedulePoint{"USA", "usa-599", "5.99"})
							case "IND":
								points = append(points, schedulePoint{"IND", "ind-115", "115"}, schedulePoint{"IND", "ind-199", "199"})
							case "JPN":
								points = append(points, schedulePoint{"JPN", "jpn-auto", "700"})
							case "TWN":
								points = append(points, schedulePoint{"TWN", "twn-150", "150"})
							case "FRA":
								points = append(points, schedulePoint{"FRA", "fra-current", "4.99"}, schedulePoint{"FRA", "fra-future", "5.99"})
							default:
								return nil, fixture.Errorf("unexpected ladder territory %s", territory)
							}
						}
						for i := range points {
							points[i].id = canonical(points[i].territory, points[i].id)
						}
						return jsonResponse(http.StatusOK, pricePointsBody(surface, points))
					case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/equalizations"):
						equalized := []schedulePoint{{"IND", "ind-499", "499"}}
						if test.name == "all selected base guard" {
							equalized = append(equalized, schedulePoint{"FRA", "fra-current", "4.99"}, schedulePoint{"JPN", "jpn-auto", "700"})
						}
						return jsonResponse(http.StatusOK, pricePointsBody(surface, equalized))
					case req.Method == http.MethodPost && req.URL.Path == surface.postPath:
						body, _ := io.ReadAll(req.Body)
						posts = append(posts, string(body))
						return jsonResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":%q,"id":"sched-1"}}`, surface.scheduleType))
					default:
						return nil, fixture.Errorf("unexpected request %s %s", req.Method, req.URL)
					}
				}))
				basePrice := "4.99"
				if test.name == "noop base keeps future" {
					basePrice = "5.99"
				}
				args := append(append([]string{}, surface.command...), "--base-price", basePrice, "--territory", test.selection, "--output", "json")
				if test.dryRun {
					args = append(args, "--dry-run")
				} else {
					args = append(args, "--confirm")
				}
				if test.start != "" {
					args = append(args, "--start-date", test.start)
				}
				stdout, stderr, err := runRootCommand(t, args)
				if test.failure != "" {
					if err == nil || !strings.Contains(err.Error(), test.failure) {
						t.Fatalf("error=%v, want %q; stderr=%s", err, test.failure, stderr)
					}
				} else if err != nil {
					t.Fatalf("run: %v; stderr=%s", err, stderr)
				}
				shouldPost := !test.noop && !test.dryRun && (test.failure == "" || strings.HasPrefix(test.name, "readback"))
				wantPosts := 0
				if shouldPost {
					wantPosts = 1
				}
				if len(posts) != wantPosts {
					t.Fatalf("POST count=%d, want post=%v", len(posts), shouldPost)
				}
				if shouldPost {
					want := []string{"base USA", "usa-599", "fra-current  2026-11-10", "fra-future 2026-11-10", "ind-115"}
					if test.start != "" {
						want = []string{"base USA", "usa-599", "fra-current  2026-11-10", "ind-199  2026-11-05", "ind-199 2026-11-05 2026-11-15", "fra-future 2026-11-10", "ind-115 2026-11-15"}
					}
					if strings.Contains(test.name, "automatic prefix") {
						want = []string{"base USA", "usa-599", "fra-current  2026-11-10", "fra-future 2026-11-10", "ind-115 2026-11-15"}
					}
					if test.name == "noop base keeps future" {
						want = []string{"base USA", "usa-599  2026-11-10", "fra-current  2026-11-10", "fra-future 2026-11-10", "usa-499 2026-11-10", "ind-115"}
					}
					if test.name == "future changes current noop" {
						want[3] = "ind-115  2026-11-05"
					}
					if surface.priceType == "inAppPurchasePrices" {
						for i, price := range want {
							if i == 0 {
								continue
							}
							parts := strings.SplitN(price, " ", 2)
							territory := strings.ToUpper(strings.SplitN(parts[0], "-", 2)[0])
							parts[0] = canonical(territory, parts[0])
							want[i] = strings.Join(parts, " ")
						}
					}
					if got := postedManualPrices(t, surface, posts[0]); !reflect.DeepEqual(got, want) {
						t.Fatalf("manual payload=%v, want %v", got, want)
					}
				}
				if test.failure == "" || shouldPost {
					var result asc.PriceSchedulePPPResult
					if err := json.Unmarshal([]byte(stdout), &result); err != nil {
						t.Fatalf("decode result: %v; stdout=%s", err, stdout)
					}
					wantRows := 1
					if test.name == "noop base keeps future" {
						wantRows = 2
					}
					if len(result.Rows) != wantRows || result.Rows[0].Territory != "IND" {
						t.Fatalf("selected rows=%+v", result.Rows)
					}
					if shouldPost && test.failure == "" && result.Verification.Status != "completed" {
						t.Fatalf("verification=%+v", result.Verification)
					}
					if test.failure != "" && result.Verification.Status != "failed" {
						t.Fatalf("verification=%+v", result.Verification)
					}
				}
			})
		}
	}
}
