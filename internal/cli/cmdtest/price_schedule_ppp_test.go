package cmdtest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type scheduleSurfaceFixture struct {
	command      []string
	schedulePath string
	scheduleType string
	pricesPath   string
	priceType    string
	pointType    string
	pointRel     string
	ladderPath   string
	eqPath       string
	postPath     string
}

var (
	iapScheduleSurface = scheduleSurfaceFixture{
		command:      []string{"iap", "pricing", "ppp", "--iap-id", "9000000001"},
		schedulePath: "/v2/inAppPurchases/9000000001/iapPriceSchedule",
		scheduleType: "inAppPurchasePriceSchedules",
		pricesPath:   "/v1/inAppPurchasePriceSchedules/sched-1/",
		priceType:    "inAppPurchasePrices",
		pointType:    "inAppPurchasePricePoints",
		pointRel:     "inAppPurchasePricePoint",
		ladderPath:   "/v2/inAppPurchases/9000000001/pricePoints",
		eqPath:       "/v1/inAppPurchasePricePoints/usa-499/equalizations",
		postPath:     "/v1/inAppPurchasePriceSchedules",
	}
	appScheduleSurface = scheduleSurfaceFixture{
		command:      []string{"pricing", "ppp", "--app", "app-1"},
		schedulePath: "/v1/apps/app-1/appPriceSchedule",
		scheduleType: "appPriceSchedules",
		pricesPath:   "/v1/appPriceSchedules/sched-1/",
		priceType:    "appPrices",
		pointType:    "appPricePoints",
		pointRel:     "appPricePoint",
		ladderPath:   "/v1/apps/app-1/appPricePoints",
		eqPath:       "/v3/appPricePoints/usa-499/equalizations",
		postPath:     "/v1/appPriceSchedules",
	}
)

type schedulePoint struct{ territory, id, price string }

func schedulePricesBody(surface scheduleSurfaceFixture, points []schedulePoint) string {
	var data, included []string
	for _, point := range points {
		data = append(data, fmt.Sprintf(`{"type":%q,"id":"price-%s","attributes":{"manual":true},"relationships":{"territory":{"data":{"type":"territories","id":%q}},%q:{"data":{"type":%q,"id":%q}}}}`,
			surface.priceType, point.id, point.territory, surface.pointRel, surface.pointType, point.id))
		included = append(included, fmt.Sprintf(`{"type":%q,"id":%q,"attributes":{"customerPrice":%q}}`, surface.pointType, point.id, point.price))
	}
	return `{"data":[` + strings.Join(data, ",") + `],"included":[` + strings.Join(included, ",") + `],"links":{}}`
}

func pricePointsBody(surface scheduleSurfaceFixture, points []schedulePoint) string {
	var data []string
	for _, point := range points {
		data = append(data, fmt.Sprintf(`{"type":%q,"id":%q,"attributes":{"customerPrice":%q},"relationships":{"territory":{"data":{"type":"territories","id":%q}}}}`,
			surface.pointType, point.id, point.price, point.territory))
	}
	return `{"data":[` + strings.Join(data, ",") + `],"links":{}}`
}

// scheduleTransport serves USA (base, 4.99), IND (indexed, 199) and TWN (no
// price level, 150). Prices read after the POST come from readback.
func scheduleTransport(t *testing.T, surface scheduleSurfaceFixture, equalizations []schedulePoint, readback []schedulePoint, posts *[]string) roundTripFunc {
	current := []schedulePoint{{"IND", "ind-199", "199"}, {"TWN", "twn-150", "150"}, {"USA", "usa-499", "4.99"}}
	ladder := []schedulePoint{
		{"IND", "ind-0", "0.0"},
		{"IND", "ind-99", "99"},
		{"IND", "ind-115", "115"},
		{"IND", "ind-199", "199"},
		{"TWN", "twn-0", "0"},
		{"TWN", "twn-150", "150"},
		{"USA", "usa-0", "0.0"},
		{"USA", "usa-499", "4.99"},
		{"USA", "usa-599", "5.99"},
	}
	return func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == surface.schedulePath:
			return jsonResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":%q,"id":"sched-1"}}`, surface.scheduleType))
		case req.Method == http.MethodGet && req.URL.Path == surface.pricesPath+"manualPrices":
			if len(*posts) > 0 {
				return jsonResponse(http.StatusOK, schedulePricesBody(surface, readback))
			}
			return jsonResponse(http.StatusOK, schedulePricesBody(surface, current))
		case req.Method == http.MethodGet && req.URL.Path == surface.pricesPath+"automaticPrices":
			return jsonResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == surface.ladderPath:
			if got := req.URL.Query().Get("filter[territory]"); got != "IND,TWN,USA" {
				t.Fatalf("ladder filter[territory] = %q, want one batched read", got)
			}
			return jsonResponse(http.StatusOK, pricePointsBody(surface, ladder))
		case req.Method == http.MethodGet && req.URL.Path == surface.eqPath:
			return jsonResponse(http.StatusOK, pricePointsBody(surface, equalizations))
		case req.Method == http.MethodPost && req.URL.Path == surface.postPath:
			body, _ := io.ReadAll(req.Body)
			*posts = append(*posts, string(body))
			return jsonResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":%q,"id":"sched-1"}}`, surface.scheduleType))
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}
}

// postedManualPrices lists the posted manual prices as "pointId start end".
func postedManualPrices(t *testing.T, surface scheduleSurfaceFixture, body string) []string {
	t.Helper()
	var request struct {
		Data struct {
			Relationships struct {
				BaseTerritory struct {
					Data struct{ ID string } `json:"data"`
				} `json:"baseTerritory"`
			} `json:"relationships"`
		} `json:"data"`
		Included []struct {
			Attributes struct {
				StartDate string `json:"startDate"`
				EndDate   string `json:"endDate"`
			} `json:"attributes"`
			Relationships map[string]struct {
				Data struct{ ID string } `json:"data"`
			} `json:"relationships"`
		} `json:"included"`
	}
	if err := json.Unmarshal([]byte(body), &request); err != nil {
		t.Fatalf("parse POST body: %v", err)
	}
	prices := []string{"base " + request.Data.Relationships.BaseTerritory.Data.ID}
	for _, item := range request.Included {
		prices = append(prices, strings.TrimSpace(item.Relationships[surface.pointRel].Data.ID+" "+item.Attributes.StartDate+" "+item.Attributes.EndDate))
	}
	return prices
}

func TestPriceSchedulePPPPlansAppliesFullSetAndVerifies(t *testing.T) {
	equalized := []schedulePoint{{"IND", "ind-499", "499"}, {"TWN", "twn-170", "170"}}
	applied := []schedulePoint{{"IND", "ind-115", "115"}, {"TWN", "twn-150", "150"}, {"USA", "usa-499", "4.99"}}
	for _, test := range []struct {
		name          string
		surface       scheduleSurfaceFixture
		args          []string
		equalizations []schedulePoint
		readback      []schedulePoint
		wantErr       string
		wantPost      []string
		wantStatuses  string
		wantVerified  string
		missingBase   bool
	}{
		{
			name:          "dry run plans indexed territories and skips TWN at its current price",
			surface:       iapScheduleSurface,
			args:          []string{"--dry-run"},
			equalizations: equalized,
			wantStatuses:  "IND planned ind-199>ind-115, TWN skipped ind:twn-150, USA noop usa-499>usa-499",
			wantVerified:  "skipped 0",
		},
		{
			name:          "indexed territory without an equalization fails closed before writing",
			surface:       iapScheduleSurface,
			args:          []string{"--confirm"},
			equalizations: equalized[1:],
			wantErr:       "1 territories could not be resolved; no changes were applied",
			wantStatuses:  "IND unresolved ind-199>, TWN skipped ind:twn-150, USA noop usa-499>usa-499",
			wantVerified:  "skipped 0",
		},
		{
			name:          "iap confirm replaces the schedule with every territory",
			surface:       iapScheduleSurface,
			args:          []string{"--confirm"},
			equalizations: equalized,
			readback:      applied,
			wantPost:      []string{"base USA", "ind-115", "twn-150", "usa-499"},
			wantStatuses:  "IND verified ind-199>ind-115, TWN skipped ind:twn-150, USA noop usa-499>usa-499",
			wantVerified:  "completed 3",
		},
		{
			name:          "app confirm replaces the schedule with every territory",
			surface:       appScheduleSurface,
			args:          []string{"--confirm"},
			equalizations: equalized,
			readback:      applied,
			wantPost:      []string{"base USA", "ind-115", "twn-150", "usa-499"},
			wantStatuses:  "IND verified ind-199>ind-115, TWN skipped ind:twn-150, USA noop usa-499>usa-499",
			wantVerified:  "completed 3",
		},
		{
			name:          "start date keeps the current price until the new one starts",
			surface:       iapScheduleSurface,
			args:          []string{"--confirm", "--start-date", "2099-01-01"},
			equalizations: equalized,
			readback:      applied,
			wantPost:      []string{"base USA", "ind-199  2099-01-01", "ind-115 2099-01-01", "twn-150", "usa-499"},
			wantStatuses:  "IND verified ind-199>ind-115, TWN skipped ind:twn-150, USA noop usa-499>usa-499",
			wantVerified:  "completed 3",
		},
		{
			name:          "start date also defers a newly priced base territory",
			surface:       iapScheduleSurface,
			args:          []string{"--confirm", "--start-date", "2099-01-01"},
			equalizations: equalized,
			readback:      applied,
			wantPost:      []string{"base USA", "ind-199  2099-01-01", "ind-115 2099-01-01", "twn-150", "usa-499 2099-01-01"},
			wantStatuses:  "IND verified ind-199>ind-115, TWN skipped ind:twn-150, USA verified >usa-499",
			wantVerified:  "completed 3",
			missingBase:   true,
		},
		{
			name:          "readback mismatch fails verification",
			surface:       iapScheduleSurface,
			args:          []string{"--confirm"},
			equalizations: equalized,
			readback:      []schedulePoint{{"IND", "ind-199", "199"}, {"TWN", "twn-150", "150"}, {"USA", "usa-499", "4.99"}},
			wantErr:       "verification failed for 1 territories",
			wantPost:      []string{"base USA", "ind-115", "twn-150", "usa-499"},
			wantStatuses:  "IND failed ind-199>ind-115, TWN skipped ind:twn-150, USA noop usa-499>usa-499",
			wantVerified:  "failed 2",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", "")
			var posts []string
			transport := scheduleTransport(t, test.surface, test.equalizations, test.readback, &posts)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if test.missingBase && req.Method == http.MethodGet && req.URL.Path == test.surface.pricesPath+"manualPrices" && len(posts) == 0 {
					return jsonResponse(http.StatusOK, schedulePricesBody(test.surface, []schedulePoint{{"IND", "ind-199", "199"}, {"TWN", "twn-150", "150"}}))
				}
				return transport(req)
			}))

			args := append(append(append([]string{}, test.surface.command...), "--base-price", "4.99", "--output", "json"), test.args...)
			stdout, stderr, err := runRootCommand(t, args)
			if test.wantErr == "" && err != nil {
				t.Fatalf("run error: %v\nstderr=%s", err, stderr)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}

			var result struct {
				Rows []struct {
					Territory            string `json:"territory"`
					Status               string `json:"status"`
					CurrentTargetPointID string `json:"currentTargetPricePointId"`
					TargetPricePointID   string `json:"targetPricePointId"`
				} `json:"rows"`
				Verification struct {
					Status   string `json:"status"`
					Verified int    `json:"verified"`
				} `json:"verification"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("parse output: %v\nstdout=%s", err, stdout)
			}
			var statuses []string
			for _, row := range result.Rows {
				if row.Status == "skipped" {
					statuses = append(statuses, row.Territory+" skipped ind:"+row.CurrentTargetPointID)
					continue
				}
				statuses = append(statuses, row.Territory+" "+row.Status+" "+row.CurrentTargetPointID+">"+row.TargetPricePointID)
			}
			if got := strings.Join(statuses, ", "); got != test.wantStatuses {
				t.Fatalf("rows = %q, want %q", got, test.wantStatuses)
			}
			if got := fmt.Sprintf("%s %d", result.Verification.Status, result.Verification.Verified); got != test.wantVerified {
				t.Fatalf("verification = %q, want %q", got, test.wantVerified)
			}
			if test.wantPost == nil {
				if len(posts) != 0 {
					t.Fatalf("expected no schedule write, got %q", posts)
				}
				return
			}
			if len(posts) != 1 {
				t.Fatalf("schedule writes = %d, want 1", len(posts))
			}
			if got := strings.Join(postedManualPrices(t, test.surface, posts[0]), " | "); got != strings.Join(test.wantPost, " | ") {
				t.Fatalf("posted prices = %q, want %q", got, strings.Join(test.wantPost, " | "))
			}
		})
	}
}

func TestPriceSchedulePPPRejectsUnresolvedCurrentPricesOnLaterPages(t *testing.T) {
	for _, surface := range []scheduleSurfaceFixture{appScheduleSurface, iapScheduleSurface} {
		for _, missingRelationship := range []string{"included point", "territory", "price point"} {
			t.Run(surface.priceType+"/"+missingRelationship, func(t *testing.T) {
				setupAuth(t)
				t.Setenv("ASC_APP_ID", "")
				var posts []string
				readback := []schedulePoint{{"IND", "ind-115", "115"}, {"TWN", "twn-170", "170"}, {"USA", "usa-499", "4.99"}}
				transport := scheduleTransport(t, surface, []schedulePoint{{"IND", "ind-499", "499"}}, readback, &posts)
				pages := 0
				installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method == http.MethodGet && req.URL.Path == surface.pricesPath+"manualPrices" && len(posts) == 0 {
						pages++
						if req.URL.Query().Get("include") != surface.pointRel+",territory" {
							t.Fatalf("missing includes on inventory page %d: %s", pages, req.URL.RawQuery)
						}
						if pages == 1 {
							body := schedulePricesBody(surface, []schedulePoint{{"IND", "ind-199", "199"}, {"USA", "usa-499", "4.99"}})
							next := "https://api.appstoreconnect.apple.com" + surface.pricesPath + "manualPrices?cursor=Mg"
							return jsonResponse(http.StatusOK, strings.Replace(body, `"links":{}`, `"links":{"next":"`+next+`"}`, 1))
						}
						if pages != 2 || req.URL.Query().Get("cursor") != "Mg" {
							t.Fatalf("unexpected inventory page: %s", req.URL.String())
						}
						// Apple may omit included resources; the manual price still exists.
						body := fmt.Sprintf(`{"data":[{"type":%q,"id":"price-twn-150","attributes":{"manual":true},"relationships":{"territory":{"data":{"type":"territories","id":"TWN"}},%q:{"data":{"type":%q,"id":"twn-150"}}}}],"links":{}}`, surface.priceType, surface.pointRel, surface.pointType)
						switch missingRelationship {
						case "territory":
							body = strings.Replace(body, `"territory":{"data":{"type":"territories","id":"TWN"}},`, "", 1)
						case "price point":
							body = strings.Replace(body, fmt.Sprintf(`,%q:{"data":{"type":%q,"id":"twn-150"}}`, surface.pointRel, surface.pointType), "", 1)
						}
						return jsonResponse(http.StatusOK, body)
					}
					if req.Method == http.MethodGet && req.URL.Path == surface.ladderPath {
						// Permit the former incomplete plan to reach its POST so the
						// regression demonstrates the unintended full replacement.
						return jsonResponse(http.StatusOK, pricePointsBody(surface, []schedulePoint{{"IND", "ind-199", "199"}, {"IND", "ind-115", "115"}, {"USA", "usa-499", "4.99"}}))
					}
					return transport(req)
				}))

				args := append(append([]string{}, surface.command...), "--base-price", "4.99", "--confirm", "--output", "json")
				stdout, _, err := runRootCommand(t, args)
				if pages != 2 {
					t.Fatalf("inventory pages = %d, want 2", pages)
				}
				if len(posts) != 0 {
					t.Fatalf("incomplete inventory caused %d schedule writes: %v; error=%v; output=%s", len(posts), postedManualPrices(t, surface, posts[0]), err, stdout)
				}
				if err == nil || !strings.Contains(err.Error(), `cannot resolve active price "price-twn-150"`) {
					t.Fatalf("error = %v, want unresolved current price", err)
				}
				if stdout != "" {
					t.Fatalf("unexpected successful result: %s", stdout)
				}
			})
		}
	}
}

func TestPriceSchedulePPPIgnoresUnresolvedSupersededPrices(t *testing.T) {
	for _, surface := range []scheduleSurfaceFixture{appScheduleSurface, iapScheduleSurface} {
		for _, test := range []struct {
			name            string
			automatic       bool
			unresolvedFirst bool
		}{
			{name: "automatic overridden by manual", automatic: true},
			{name: "older manual before winner", unresolvedFirst: true},
			{name: "older manual after winner"},
		} {
			t.Run(surface.priceType+"/"+test.name, func(t *testing.T) {
				setupAuth(t)
				t.Setenv("ASC_APP_ID", "")
				t.Cleanup(setRootNow(func() time.Time {
					return time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
				}))
				var posts []string
				readback := []schedulePoint{{"IND", "ind-115", "115"}, {"TWN", "twn-150", "150"}, {"USA", "usa-499", "4.99"}}
				transport := scheduleTransport(t, surface, []schedulePoint{{"IND", "ind-499", "499"}}, readback, &posts)
				unresolved := fmt.Sprintf(`{"type":%q,"id":"unresolved-old","attributes":{"manual":%t},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}},%q:{"data":{"type":%q,"id":"usa-old"}}}}`, surface.priceType, !test.automatic, surface.pointRel, surface.pointType)
				installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method == http.MethodGet && req.URL.Path == surface.pricesPath+"automaticPrices" && test.automatic {
						return jsonResponse(http.StatusOK, `{"data":[`+unresolved+`],"links":{}}`)
					}
					resp, err := transport(req)
					if err != nil || req.Method != http.MethodGet || req.URL.Path != surface.pricesPath+"manualPrices" || test.automatic {
						return resp, err
					}
					raw, err := io.ReadAll(resp.Body)
					_ = resp.Body.Close()
					if err != nil {
						return nil, err
					}
					body := strings.ReplaceAll(string(raw), `"manual":true`, `"manual":true,"startDate":"2025-01-01"`)
					if test.unresolvedFirst {
						body = strings.Replace(body, `"data":[`, `"data":[`+unresolved+`,`, 1)
					} else {
						body = strings.Replace(body, `],"included":`, `,`+unresolved+`],"included":`, 1)
					}
					return jsonResponse(resp.StatusCode, body)
				}))

				args := append(append([]string{}, surface.command...), "--base-price", "4.99", "--confirm", "--output", "json")
				stdout, _, err := runRootCommand(t, args)
				if err != nil {
					t.Fatalf("resolved effective schedule was rejected: %v", err)
				}
				if len(posts) != 1 {
					t.Fatalf("schedule writes = %d, want 1", len(posts))
				}
				if got := strings.Join(postedManualPrices(t, surface, posts[0]), " | "); got != "base USA | ind-115 | twn-150 | usa-499" {
					t.Fatalf("replacement prices = %q", got)
				}
				var result struct {
					Verification struct {
						Status   string `json:"status"`
						Verified int    `json:"verified"`
					} `json:"verification"`
				}
				if err := json.Unmarshal([]byte(stdout), &result); err != nil {
					t.Fatalf("parse output: %v", err)
				}
				if result.Verification.Status != "completed" || result.Verification.Verified != 3 {
					t.Fatalf("verification = %+v, want completed for all three territories", result.Verification)
				}
			})
		}
	}
}
