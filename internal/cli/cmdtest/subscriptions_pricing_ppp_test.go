package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSubscriptionsPricingPPPValidationErrors(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing subscription",
			args:    []string{"--base-price", "9.99", "--dry-run"},
			wantErr: "--subscription-id is required",
		},
		{
			name:    "missing base price",
			args:    []string{"--subscription-id", "8000000001", "--dry-run"},
			wantErr: "--base-price is required",
		},
		{
			name:    "zero base price",
			args:    []string{"--subscription-id", "8000000001", "--base-price", "0", "--dry-run"},
			wantErr: "--base-price must be a positive decimal",
		},
		{
			name:    "base territory without price level",
			args:    []string{"--subscription-id", "8000000001", "--base-price", "9.99", "--base-territory", "TWN", "--dry-run"},
			wantErr: "invalid --base-territory: no purchasing-power price level for TWN",
		},
		{
			name:    "dry run and confirm conflict",
			args:    []string{"--subscription-id", "8000000001", "--base-price", "9.99", "--dry-run", "--confirm"},
			wantErr: "--dry-run cannot be combined with --confirm",
		},
		{
			name:    "requires confirm",
			args:    []string{"--subscription-id", "8000000001", "--base-price", "9.99"},
			wantErr: "--confirm is required unless --dry-run is set",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, append([]string{"subscriptions", "pricing", "ppp"}, test.args...), test.wantErr)
		})
	}
}

func TestPPPTerritoryScopeValidation(t *testing.T) {
	setupAuth(t)
	requests := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		return jsonResponse(http.StatusInternalServerError, `{}`)
	}))
	t.Cleanup(func() {
		if requests != 0 {
			t.Errorf("validation made %d requests", requests)
		}
	})
	for _, command := range [][]string{
		{"subscriptions", "pricing", "ppp", "--subscription-id", "8000000001"},
		{"pricing", "ppp", "--app", "app-1"},
		{"iap", "pricing", "ppp", "--iap-id", "9000000001"},
	} {
		assertUsageExit(t, append(append([]string{}, command...), "--base-price", "4.99", "--dry-run", "--territory", "IN", "--territory", "BR"), "--territory specified multiple times")
		for _, value := range []string{"", ",,", "Atlantis"} {
			t.Run(strings.Join(command, " ")+value, func(t *testing.T) {
				assertUsageExit(t, append(append([]string{}, command...), "--base-price", "4.99", "--dry-run", "--territory", value), "invalid --territory")
			})
		}
	}
}

func pppTestTransport(t *testing.T, priceReads *int, posts *[]string) roundTripFunc {
	const subID = "8000000001"
	return func(req *http.Request) (*http.Response, error) {
		query := req.URL.Query()
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionPriceTiers":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+subID+"/prices":
			*priceReads++
			if *priceReads > 1 {
				return jsonResponse(http.StatusOK, subscriptionPriceDerivePricesFixture([]deriveHTTPPrice{
					{priceID: "p-ind-new", pricePointID: "ind-199", territory: "IND", currency: "INR", customerPrice: "199"},
					{priceID: "p-twn", pricePointID: "twn-390", territory: "TWN", currency: "TWD", customerPrice: "390"},
					{priceID: "p-usa", pricePointID: "usa-999", territory: "USA", currency: "USD", customerPrice: "9.99"},
					{priceID: "p-vgb", pricePointID: "vgb-999", territory: "VGB", currency: "USD", customerPrice: "9.99"},
				}))
			}
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricesFixture([]deriveHTTPPrice{
				{priceID: "p-ind", pricePointID: "ind-999", territory: "IND", currency: "INR", customerPrice: "999"},
				{priceID: "p-twn", pricePointID: "twn-390", territory: "TWN", currency: "TWD", customerPrice: "390"},
				{priceID: "p-usa", pricePointID: "usa-999", territory: "USA", currency: "USD", customerPrice: "9.99"},
				{priceID: "p-vgb", pricePointID: "vgb-999", territory: "VGB", currency: "USD", customerPrice: "9.99"},
			}))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+subID+"/pricePoints" && query.Get("filter[territory]") == "USA":
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricePointsFixture([]deriveHTTPPricePoint{
				{id: "usa-899", territory: "USA", customerPrice: "8.99"},
				{id: "usa-999", territory: "USA", customerPrice: "9.99"},
			}))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionPricePoints/usa-999/equalizations":
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricePointsFixture([]deriveHTTPPricePoint{
				{id: "ind-899", territory: "IND", customerPrice: "899"},
				{id: "twn-300", territory: "TWN", customerPrice: "300"},
			}))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+subID+"/pricePoints":
			if got := query.Get("filter[territory]"); got != "IND,TWN,USA" {
				t.Fatalf("ladder territory filter = %q, want IND,TWN,USA", got)
			}
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricePointsFixture([]deriveHTTPPricePoint{
				{id: "ind-199", territory: "IND", customerPrice: "199"},
				{id: "ind-219", territory: "IND", customerPrice: "219"},
				{id: "ind-999", territory: "IND", customerPrice: "999"},
				{id: "twn-390", territory: "TWN", customerPrice: "390"},
				{id: "usa-999", territory: "USA", customerPrice: "9.99"},
			}))
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices" && posts != nil:
			body, _ := io.ReadAll(req.Body)
			*posts = append(*posts, string(body))
			return jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionPrices","id":"p-ind-new","attributes":{"planType":"UPFRONT"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}
}

type pppTestResult struct {
	BaseTerritory    string `json:"baseTerritory"`
	BasePricePointID string `json:"basePricePointId"`
	Summary          struct {
		Planned  int `json:"planned"`
		Noop     int `json:"noop"`
		Skipped  int `json:"skipped"`
		Applied  int `json:"applied"`
		Verified int `json:"verified"`
	} `json:"summary"`
	Rows []struct {
		Territory          string `json:"territory"`
		SourcePrice        string `json:"sourcePrice"`
		RequestedMultiple  string `json:"requestedMultiple"`
		DesiredPrice       string `json:"desiredPrice"`
		TargetPricePointID string `json:"targetPricePointId"`
		Status             string `json:"status"`
	} `json:"rows"`
	Verification struct {
		Status   string `json:"status"`
		Verified int    `json:"verified"`
	} `json:"verification"`
}

func runPPPCommand(t *testing.T, args ...string) pppTestResult {
	t.Helper()
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse(append([]string{"subscriptions", "pricing", "ppp", "--subscription-id", "8000000001", "--base-price", "9.99", "--workers", "1", "--auto-start-date=false"}, args...)); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	var result pppTestResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON output: %v\nstdout=%s", err, stdout)
	}
	return result
}

func TestSubscriptionsPricingPPPDryRunPlansIndexedTerritoriesWithoutMutating(t *testing.T) {
	setupAuth(t)
	priceReads := 0
	installDefaultTransport(t, pppTestTransport(t, &priceReads, nil))

	result := runPPPCommand(t, "--dry-run")

	if result.BaseTerritory != "USA" || result.BasePricePointID != "usa-999" {
		t.Fatalf("unexpected plan metadata: %+v", result)
	}
	if result.Summary.Planned != 1 || result.Summary.Noop != 1 || result.Summary.Skipped != 2 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
	if len(result.Rows) != 4 || result.Rows[3].Territory != "VGB" || result.Rows[3].Status != "skipped" {
		t.Fatalf("expected unindexed territory without an equalization to be skipped: %+v", result.Rows)
	}
	ind, twn, usa := result.Rows[0], result.Rows[1], result.Rows[2]
	if ind.Territory != "IND" || ind.SourcePrice != "899" || ind.RequestedMultiple != "0.2305" || ind.DesiredPrice != "207.2195" || ind.TargetPricePointID != "ind-199" || ind.Status != "planned" {
		t.Fatalf("unexpected IND row: %+v", ind)
	}
	if twn.Territory != "TWN" || twn.Status != "skipped" || twn.TargetPricePointID != "" {
		t.Fatalf("unexpected TWN row: %+v", twn)
	}
	if usa.Territory != "USA" || usa.RequestedMultiple != "1" || usa.TargetPricePointID != "usa-999" || usa.Status != "noop" {
		t.Fatalf("unexpected USA row: %+v", usa)
	}
}

func TestSubscriptionsPricingPPPConfirmAppliesChangedTerritoriesAndVerifies(t *testing.T) {
	setupAuth(t)
	priceReads := 0
	var posts []string
	installDefaultTransport(t, pppTestTransport(t, &priceReads, &posts))

	result := runPPPCommand(t, "--confirm")

	if len(posts) != 1 || !strings.Contains(posts[0], `"id":"ind-199"`) || !strings.Contains(posts[0], `"id":"IND"`) {
		t.Fatalf("expected one IND mutation to ind-199, got %q", posts)
	}
	if priceReads != 2 {
		t.Fatalf("price reads = %d, want planning + verification", priceReads)
	}
	if result.Summary.Applied != 1 || result.Summary.Verified != 1 || result.Verification.Status != "completed" || result.Verification.Verified != 1 {
		t.Fatalf("unexpected apply result: summary=%+v verification=%+v", result.Summary, result.Verification)
	}
}

func TestSubscriptionsPricingPPPTerritorySelection(t *testing.T) {
	for _, test := range []struct {
		selection, territory, status, start string
		posts                               int
	}{
		{"IN,India", "IND", "verified", "", 1},
		{"USA", "USA", "noop", "", 0},
		{"TWN", "TWN", "skipped", "", 0},
		{"IND", "IND", "verified", "2026-11-15", 1},
	} {
		t.Run(test.selection+"/"+test.start, func(t *testing.T) {
			setupAuth(t)
			restore := setRootNow(func() time.Time { return time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC) })
			defer restore()
			priceReads := 0
			posts := []string{}
			transport := pppTestTransport(t, &priceReads, &posts)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if test.start != "" && req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/prices" {
					prices := []deriveHTTPPrice{{priceID: "p-ind-current", pricePointID: "ind-999", territory: "IND", currency: "INR", customerPrice: "999"}, {priceID: "p-ind-successor", pricePointID: "ind-219", territory: "IND", currency: "INR", customerPrice: "219"}}
					if len(posts) > 0 {
						prices = []deriveHTTPPrice{{priceID: "p-ind-applied", pricePointID: "ind-199", territory: "IND", currency: "INR", customerPrice: "199"}}
					}
					var payload map[string]any
					if err := json.Unmarshal([]byte(subscriptionPriceDerivePricesFixture(prices)), &payload); err != nil {
						return nil, err
					}
					for _, item := range payload["data"].([]any) {
						row := item.(map[string]any)
						start := ""
						if row["id"] == "p-ind-successor" {
							start = test.start
						}
						if row["id"] == "p-ind-applied" {
							start = test.start
						}
						row["attributes"].(map[string]any)["startDate"] = start
					}
					body, err := json.Marshal(payload)
					if err != nil {
						return nil, err
					}
					return jsonResponse(http.StatusOK, string(body))
				}

				if req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/pricePoints" && req.URL.Query().Get("filter[territory]") == "IND" {
					return jsonResponse(http.StatusOK, subscriptionPriceDerivePricePointsFixture([]deriveHTTPPricePoint{{id: "ind-199", territory: "IND", customerPrice: "199"}, {id: "ind-219", territory: "IND", customerPrice: "219"}, {id: "ind-999", territory: "IND", customerPrice: "999"}}))
				}
				if req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/pricePoints" && req.URL.Query().Get("filter[territory]") == "TWN" {
					return jsonResponse(http.StatusOK, subscriptionPriceDerivePricePointsFixture([]deriveHTTPPricePoint{{id: "twn-390", territory: "TWN", customerPrice: "390"}}))
				}
				return transport(req)
			}))
			args := []string{"--territory", test.selection, "--confirm"}
			if test.start != "" {
				args = append(args, "--start-date", test.start)
			}
			result := runPPPCommand(t, args...)
			if len(result.Rows) != 1 || result.Rows[0].Territory != test.territory || result.Rows[0].Status != test.status {
				t.Fatalf("rows=%+v", result.Rows)
			}
			if len(posts) != test.posts {
				t.Fatalf("POST count=%d, want %d", len(posts), test.posts)
			}
			if len(posts) > 0 {
				var body struct {
					Data struct {
						Attributes struct {
							StartDate string `json:"startDate"`
						} `json:"attributes"`
						Relationships map[string]struct{ Data struct{ ID string } }
					}
				}
				if err := json.Unmarshal([]byte(posts[0]), &body); err != nil {
					t.Fatal(err)
				}
				if body.Data.Attributes.StartDate != test.start {
					t.Fatalf("POST startDate=%q, want %q", body.Data.Attributes.StartDate, test.start)
				}
				if body.Data.Relationships["territory"].Data.ID != "IND" || body.Data.Relationships["subscriptionPricePoint"].Data.ID != "ind-199" {
					t.Fatalf("POST=%s", posts[0])
				}
			}
		})
	}
}

func TestSubscriptionsPricingPPPRefusesChangedVolumeTerritories(t *testing.T) {
	setupAuth(t)
	priceReads, tierReads := 0, 0
	var posts []string
	transport := pppTestTransport(t, &priceReads, &posts)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionPriceTiers" {
			tierReads++
			if req.URL.Query().Get("filter[territory]") != "IND" || req.URL.Query().Get("filter[subscription]") != "8000000001" {
				t.Fatalf("guard must include only changed IND territory: %s", req.URL.RawQuery)
			}
			return jsonHTTPResponse(200, `{"data":[{"type":"subscriptionPriceTiers","id":"tier-ind","attributes":{"minQuantity":10}}],"links":{}}`), nil
		}
		return transport(req)
	}))
	out, _, err := runCommand(t, []string{"subscriptions", "pricing", "ppp", "--subscription-id", "8000000001", "--base-price", "9.99", "--workers", "1", "--auto-start-date=false", "--confirm", "--output", "json"})
	if err == nil || !strings.Contains(err.Error()+out, "asc subscriptions pricing prices set --additional-price-tiers") || len(posts) != 0 || tierReads != 1 {
		t.Fatalf("err=%v posts=%d tier reads=%d output=%s", err, len(posts), tierReads, out)
	}
}
