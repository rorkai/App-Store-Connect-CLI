package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// Subscription prices carry only a startDate: each price ends on the date the
// next one starts, the end-equals-next-start shape App Store Connect uses for
// app and in-app purchase schedules.
const subscriptionPacificDayPricesBody = `{
	"data":[
		{"type":"subscriptionPrices","id":"price-old-usa","attributes":{"startDate":"2026-01-01","preserved":false,"planType":"UPFRONT"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-old-usa"}}}},
		{"type":"subscriptionPrices","id":"price-new-usa","attributes":{"startDate":"2026-10-01","preserved":false,"planType":"UPFRONT"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-new-usa"}}}}
	],
	"included":[
		{"type":"subscriptionPricePoints","id":"pp-old-usa","attributes":{"customerPrice":"0.99","proceeds":"0.84","proceedsYear2":"0.84"}},
		{"type":"subscriptionPricePoints","id":"pp-new-usa","attributes":{"customerPrice":"1.99","proceeds":"1.69","proceedsYear2":"1.69"}},
		{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
	],
	"links":{"next":""}
}`

func TestSubscriptionsPricingPricesListResolvedUsesUSPacificDay(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want string
	}{
		// 17:30 PDT on 2026-09-30: the 2026-10-01 price has not started.
		{name: "00:30 UTC is the previous Pacific day", now: time.Date(2026, time.October, 1, 0, 30, 0, 0, time.UTC), want: "0.99"},
		// Pacific midnight: the old price ends as the new one starts.
		{name: "Pacific midnight starts the next price", now: time.Date(2026, time.October, 1, 7, 0, 0, 0, time.UTC), want: "1.99"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Cleanup(shared.SetPricingNowForTesting(func() time.Time { return test.now }))
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/prices" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				}
				return jsonHTTPResponse(http.StatusOK, subscriptionPacificDayPricesBody), nil
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"subscriptions", "pricing", "prices", "list",
					"--subscription-id", "8000000001",
					"--resolved",
					"--output", "json",
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			var result shared.ResolvedPricesResult
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("json.Unmarshal() error = %v, stdout = %q", err, stdout)
			}
			if len(result.Prices) != 1 || result.Prices[0].CustomerPrice != test.want {
				t.Fatalf("expected the USA price %s, got %+v", test.want, result.Prices)
			}
		})
	}
}

func TestSubscriptionsPricingEqualize_ReconcilesAutoScheduledPriceOnItsStartDate(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "0")
	// 17:30 PDT on 2026-09-30: the auto-scheduled start date is 2026-10-01.
	t.Cleanup(shared.SetPricingNowForTesting(func() time.Time {
		return time.Date(2026, time.October, 1, 0, 30, 0, 0, time.UTC)
	}))

	basePricePointID := testSubscriptionPricePointID("USA")
	canPricePointID := testSubscriptionPricePointID("CAN")
	canAttempts := 0
	verifyReads := 0

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/territories":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"CAN"}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/pricePoints":
			body := `{"data":[{"type":"subscriptionPricePoints","id":"` + basePricePointID + `","attributes":{"customerPrice":"0.99"}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionPricePoints/"+basePricePointID+"/equalizations":
			body := `{"data":[{"type":"subscriptionPricePoints","id":"` + canPricePointID + `","attributes":{"customerPrice":"1.29"},"relationships":{"territory":{"data":{"type":"territories","id":"CAN"}}}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/subscriptionAvailability":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":true}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAvailabilities/avail-1/availableTerritories":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"CAN"}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/relationships/prices":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionPrices","id":"price-existing"}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptions","id":"8000000001","attributes":{"state":"APPROVED"}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/prices":
			verifyReads++
			// The scheduled CAN price landed despite the 429: it starts on
			// 2026-10-01 and ends the old CAN price on that date.
			body := `{
				"data":[
					{"type":"subscriptionPrices","id":"price-can-old","attributes":{"startDate":"2025-01-01","preserved":false},"relationships":{"territory":{"data":{"type":"territories","id":"CAN"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-can-old"}}}},
					{"type":"subscriptionPrices","id":"price-can-new","attributes":{"startDate":"2026-10-01","preserved":false},"relationships":{"territory":{"data":{"type":"territories","id":"CAN"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"` + canPricePointID + `"}}}},
					{"type":"subscriptionPrices","id":"price-usa-new","attributes":{"startDate":"2026-10-01","preserved":false},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"` + basePricePointID + `"}}}}
				],
				"included":[
					{"type":"subscriptionPricePoints","id":"pp-can-old","attributes":{"customerPrice":"0.99","proceeds":"0.70","proceedsYear2":"0.84"}},
					{"type":"subscriptionPricePoints","id":"` + canPricePointID + `","attributes":{"customerPrice":"1.29","proceeds":"0.90","proceedsYear2":"1.05"}},
					{"type":"subscriptionPricePoints","id":"` + basePricePointID + `","attributes":{"customerPrice":"0.99","proceeds":"0.84","proceedsYear2":"0.84"}},
					{"type":"territories","id":"CAN","attributes":{"currency":"CAD"}},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				],
				"links":{"next":""}
			}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll() error: %v", err)
			}
			if !strings.Contains(string(body), `"startDate":"2026-10-01"`) {
				t.Fatalf("expected the Pacific next day 2026-10-01 as startDate, got %s", string(body))
			}
			switch {
			case strings.Contains(string(body), `"id":"USA"`):
				return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionPrices","id":"price-usa"}}`), nil
			case strings.Contains(string(body), `"id":"CAN"`):
				canAttempts++
				if canAttempts > 1 {
					t.Fatalf("unsafe replay: the scheduled CAN price was already visible during reconciliation")
				}
				return jsonHTTPResponse(http.StatusTooManyRequests, `{"errors":[{"status":"429","code":"RATE_LIMIT_EXCEEDED","title":"Too Many Requests","detail":"retry later"}]}`), nil
			default:
				t.Fatalf("unexpected subscription price body: %s", string(body))
				return nil, nil
			}
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "equalize",
			"--subscription-id", "8000000001",
			"--base-price", "0.99",
			"--confirm",
			"--workers", "1",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if canAttempts != 1 || verifyReads != 2 {
		t.Fatalf("expected one CAN attempt, one reconciliation read and one final verification read, got attempts=%d reads=%d", canAttempts, verifyReads)
	}
	var result struct {
		StartDate     string `json:"startDate"`
		AutoScheduled bool   `json:"autoScheduled"`
		Succeeded     int    `json:"succeeded"`
		Failed        int    `json:"failed"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON result: %v", err)
	}
	if result.StartDate != "2026-10-01" || !result.AutoScheduled || result.Succeeded != 2 || result.Failed != 0 {
		t.Fatalf("expected the scheduled CAN price to reconcile, got %+v", result)
	}
}

func TestSubscriptionsPricingEqualize_AutoScheduleMovesToTheEarliestStartDateAppleAccepts(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "0")
	// 11:01 PST on 2026-11-12: the Pacific next day is 2026-11-13, but App
	// Store Connect only accepts 2026-11-14 (observed 2026-11-12).
	t.Cleanup(shared.SetPricingNowForTesting(func() time.Time {
		return time.Date(2026, time.November, 12, 19, 1, 0, 0, time.UTC)
	}))

	basePricePointID := testSubscriptionPricePointID("USA")
	canPricePointID := testSubscriptionPricePointID("CAN")
	ledger := newSubscriptionPriceLedger()
	var posts []string

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/territories":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"CAN"}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/pricePoints":
			body := `{"data":[{"type":"subscriptionPricePoints","id":"` + basePricePointID + `","attributes":{"customerPrice":"0.99"}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionPricePoints/"+basePricePointID+"/equalizations":
			body := `{"data":[{"type":"subscriptionPricePoints","id":"` + canPricePointID + `","attributes":{"customerPrice":"1.29"},"relationships":{"territory":{"data":{"type":"territories","id":"CAN"}}}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/subscriptionAvailability":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":true}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAvailabilities/avail-1/availableTerritories":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"CAN"}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/relationships/prices":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionPrices","id":"price-existing"}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptions","id":"8000000001","attributes":{"state":"READY_FOR_SALE"}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/prices":
			return ledger.response(), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll() error: %v", err)
			}
			var payload struct {
				Data struct {
					Attributes struct {
						StartDate string `json:"startDate"`
					} `json:"attributes"`
					Relationships struct {
						Territory struct {
							Data struct {
								ID string `json:"id"`
							} `json:"data"`
						} `json:"territory"`
					} `json:"relationships"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("parse body: %v", err)
			}
			posts = append(posts, payload.Data.Relationships.Territory.Data.ID+"@"+payload.Data.Attributes.StartDate)
			if payload.Data.Attributes.StartDate < "2026-11-14" {
				return jsonHTTPResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"An attribute value is invalid.","detail":"Invalid startDate=`+payload.Data.Attributes.StartDate+`, must be on or after 2026-11-14","source":{"pointer":"/data/attributes/startDate"}}]}`), nil
			}
			ledger.record(t, body)
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionPrices","id":"price-created"}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "equalize",
			"--subscription-id", "8000000001",
			"--base-price", "0.99",
			"--confirm",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if got := strings.Join(posts, ","); got != "USA@2026-11-13,USA@2026-11-14,CAN@2026-11-14" {
		t.Fatalf("expected one rejected base POST, then every territory on Apple's earliest date, got %s", got)
	}
	if !strings.Contains(stderr, "scheduling price changes for 2026-11-14") {
		t.Fatalf("expected stderr to name the adopted start date, got %q", stderr)
	}
	var result struct {
		StartDate     string `json:"startDate"`
		AutoScheduled bool   `json:"autoScheduled"`
		Succeeded     int    `json:"succeeded"`
		Failed        int    `json:"failed"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON result: %v", err)
	}
	if result.StartDate != "2026-11-14" || !result.AutoScheduled || result.Succeeded != 2 || result.Failed != 0 {
		t.Fatalf("expected both territories scheduled for 2026-11-14, got %+v", result)
	}
}
