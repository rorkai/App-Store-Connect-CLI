package cmdtest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestSubscriptionsPricesSetAdditionalTiers(t *testing.T) {
	for _, test := range []struct {
		name            string
		initial         bool
		unchanged       bool
		noTerritory     bool
		empty           bool
		explicitFalse   bool
		initialRejected bool
	}{
		{name: "initial price", initial: true},
		{name: "initial replay sees desired tiers", initial: true, initialRejected: true, unchanged: true},
		{name: "initial replay sees different tiers", initial: true, initialRejected: true},
		{name: "changed tiers"},
		{name: "unchanged tiers", unchanged: true},
		{name: "unchanged tiers implicit territory", unchanged: true, noTerritory: true},
		{name: "clear tiers", empty: true},
		{name: "initial empty tiers", initial: true, empty: true},
		{name: "explicit preservation false", explicitFalse: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			captured := subscriptionPricesListQuerySurfaceStub(t)
			input := filepath.Join(t.TempDir(), "tiers.json")
			tiersJSON := `[{"minQuantity":10,"subscriptionPricePointId":"bulk-point"}]`
			if test.empty {
				tiersJSON = `[]`
			}
			if err := os.WriteFile(input, []byte(tiersJSON), 0o600); err != nil {
				t.Fatal(err)
			}
			writes := 0
			if test.initialRejected {
				setPriceReplayRetryOptions(t)
			}
			captured.response = func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/relationships/prices"):
					prices := `[{"type":"subscriptionPrices","id":"current"}]`
					if test.initial && (!test.initialRejected || writes == 0) {
						prices = `[]`
					}
					_, _ = fmt.Fprintf(w, `{"data":%s,"links":{}}`, prices)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000003/prices":
					fixture := subscriptionPriceListFixture("base-point", "USA", "")
					fixture = strings.Replace(fixture, `"additionalPriceTiers":{"data":[]}`, `"additionalPriceTiers":{"data":[{"type":"subscriptionPriceTiers","id":"tier-1"}]}`, 1)
					_, _ = io.WriteString(w, fixture)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionPrices/existing-price-1/additionalPriceTiers":
					point := "old-bulk-point"
					if test.unchanged {
						point = "bulk-point"
					}
					_, _ = fmt.Fprintf(w, `{"data":[{"type":"subscriptionPriceTiers","id":"tier-1","attributes":{"minQuantity":10},"relationships":{"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":%q}}}}],"links":{}}`, point)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000003":
					_, _ = io.WriteString(w, `{"data":{"type":"subscriptions","id":"8000000003"}}`)
				case req.Method == http.MethodPatch || req.Method == http.MethodPost:
					writes++
					wantPath, wantMethod := "/v1/subscriptionPrices", http.MethodPost
					if test.initial {
						wantPath, wantMethod = "/v1/subscriptions/8000000003", http.MethodPatch
					}
					if req.URL.Path != wantPath || req.Method != wantMethod {
						t.Fatalf("write = %s %s, want %s %s", req.Method, req.URL.Path, wantMethod, wantPath)
					}
					var payload struct {
						Data struct {
							Attributes    map[string]any             `json:"attributes"`
							Relationships map[string]json.RawMessage `json:"relationships"`
						} `json:"data"`
						Included []struct {
							Type          string                     `json:"type"`
							ID            string                     `json:"id"`
							Attributes    map[string]any             `json:"attributes"`
							Relationships map[string]json.RawMessage `json:"relationships"`
						} `json:"included"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatal(err)
					}
					attributes := payload.Data.Attributes
					relationships := payload.Data.Relationships
					tiers := payload.Included
					if test.initial {
						if len(tiers) == 0 || tiers[0].Type != "subscriptionPrices" {
							t.Fatalf("initial included = %+v", tiers)
						}
						attributes = tiers[0].Attributes
						relationships = tiers[0].Relationships
						tiers = tiers[1:]
					}
					if (test.empty || test.explicitFalse) && attributes["preserveCurrentPrice"] != false {
						t.Fatalf("preserveCurrentPrice must be explicit false, got %+v", attributes)
					}
					var linkage struct {
						Data []struct {
							Type string `json:"type"`
							ID   string `json:"id"`
						} `json:"data"`
					}
					if test.empty {
						if got := string(relationships["additionalPriceTiers"]); got != `{"data":[]}` || len(tiers) != 0 {
							t.Errorf("single-tier price linkage = %q, included tiers = %+v", got, tiers)
						}
					} else {
						if err := json.Unmarshal(relationships["additionalPriceTiers"], &linkage); err != nil {
							t.Fatalf("additionalPriceTiers linkage: %v", err)
						}
						if len(linkage.Data) != 1 || len(tiers) != 1 || linkage.Data[0].Type != "subscriptionPriceTiers" || linkage.Data[0].ID != tiers[0].ID {
							t.Fatalf("tier linkage = %+v, included = %+v", linkage, tiers)
						}
						if tiers[0].Type != "subscriptionPriceTiers" || !reflect.DeepEqual(tiers[0].Attributes, map[string]any{"minQuantity": float64(10)}) {
							t.Fatalf("included tier = %+v", tiers[0])
						}
						if got := string(tiers[0].Relationships["subscriptionPricePoint"]); got != `{"data":{"type":"subscriptionPricePoints","id":"bulk-point"}}` {
							t.Fatalf("tier price point = %s", got)
						}
					}
					if test.initialRejected && writes == 1 {
						w.WriteHeader(http.StatusTooManyRequests)
						_, _ = io.WriteString(w, `{"errors":[{"status":"429","code":"RATE_LIMIT_EXCEEDED"}]}`)
						return
					}
					resourceType := "subscriptionPrices"
					if test.initial {
						resourceType = "subscriptions"
					}
					_, _ = fmt.Fprintf(w, `{"data":{"type":%q,"id":"saved"}}`, resourceType)
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL)
				}
			}
			args := []string{"subscriptions", "pricing", "prices", "set", "--subscription-id", "8000000003", "--price-point", "base-point", "--additional-price-tiers", input, "--output", "json"}
			if !test.noTerritory {
				args = append(args, "--territory", "USA")
			}
			if test.explicitFalse {
				args = append(args, "--preserved=false")
			}
			stdout, stderr, err := runSubscriptionPricesListQuerySurface(t, args...)
			if test.initialRejected && !test.unchanged {
				if err == nil || !strings.Contains(err.Error(), "cannot replay the initial price matrix") || writes != 1 {
					t.Fatalf("err=%v writes=%d", err, writes)
				}
				return
			}
			if err != nil || stderr != "" {
				t.Fatalf("run = %v, stderr %q", err, stderr)
			}
			wantWrites, wantID := 1, "saved"
			if test.initialRejected {
				wantID = "8000000003"
			} else if test.unchanged {
				wantWrites, wantID = 0, "existing-price-1"
			}
			var result struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatal(err)
			}
			if writes != wantWrites || result.Data.ID != wantID {
				t.Fatalf("writes=%d, result ID=%q, want %d and %q", writes, result.Data.ID, wantWrites, wantID)
			}
		})
	}
}

func TestSubscriptionsPricesSetAdditionalTiersRejectsUnsafeInput(t *testing.T) {
	for _, test := range []struct {
		name     string
		input    string
		force    bool
		planType string
		want     string
	}{
		{name: "null", input: "null", want: "--additional-price-tiers must contain an array"},
		{name: "force", input: "[]", force: true, want: "--additional-price-tiers cannot be combined with --force"},
		{name: "monthly", input: `[{"minQuantity":10,"subscriptionPricePointId":"bulk-point"}]`, planType: "MONTHLY", want: "--additional-price-tiers requires UPFRONT plan type"},
	} {
		t.Run(test.name, func(t *testing.T) {
			captured := subscriptionPricesListQuerySurfaceStub(t)
			path := filepath.Join(t.TempDir(), "tiers.json")
			if err := os.WriteFile(path, []byte(test.input), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"subscriptions", "pricing", "prices", "set", "--subscription-id", "8000000003", "--price-point", "base-point", "--territory", "NOR", "--additional-price-tiers", path}
			if test.force {
				args = append(args, "--force")
			}
			if test.planType != "" {
				args = append(args, "--plan-type", test.planType)
			}
			_, stderr, err := runSubscriptionPricesListQuerySurface(t, args...)
			if rootcmd.ExitCodeFromError(err) != rootcmd.ExitUsage || !strings.Contains(stderr, test.want) {
				t.Fatalf("run = %v, stderr %q, want %q", err, stderr, test.want)
			}
			captured.assertNoRequest(t)
		})
	}
}

func TestSubscriptionsPricesSetRequiresTierIntentWhenChangingTieredPrice(t *testing.T) {
	for _, test := range []struct {
		name, tierTerritory, pointResponse string
		duplicatePage                      bool
		force                              bool
		noTerritory                        bool
		retry                              bool
		baseChanged                        bool
		want                               string
	}{
		{name: "selected territory", want: "--additional-price-tiers is required when changing a price with volume tiers"},
		{name: "implicit territory", noTerritory: true, want: "--additional-price-tiers is required when changing a price with volume tiers"},
		{name: "other territory tiers", noTerritory: true, tierTerritory: "JPN"},
		{name: "incomplete distinct price membership", tierTerritory: "JPN", noTerritory: true, duplicatePage: true, want: "--additional-price-tiers is required"},
		{name: "wrong price point returned", noTerritory: true, tierTerritory: "JPN", pointResponse: `{"data":{"type":"subscriptionPricePoints","id":"other","relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}}`, want: "--additional-price-tiers is required"},
		{name: "unverified point territory", noTerritory: true, tierTerritory: "JPN", pointResponse: `{"data":{"type":"subscriptionPricePoints","id":"new-base","relationships":{"territory":{"data":null}}}}`, want: "--additional-price-tiers is required"},
		{name: "unverified tiered price territory", noTerritory: true, tierTerritory: "unknown", want: "--additional-price-tiers is required"},
		{name: "base changed before force replay", force: true, retry: true, baseChanged: true, want: "base price changed"},
		{name: "global force repair", force: true, want: "--force cannot rebuild a subscription with volume tiers"},
		{name: "volume appeared after rate limit", retry: true, want: "--additional-price-tiers is required when changing a price with volume tiers"},
		{name: "volume appeared before force replay", force: true, retry: true, want: "--force cannot rebuild a subscription with volume tiers"},
	} {
		t.Run(test.name, func(t *testing.T) {
			captured := subscriptionPricesListQuerySurfaceStub(t)
			writes := 0
			if test.retry {
				setPriceReplayRetryOptions(t)
			}
			captured.response = func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if req.Method != http.MethodGet {
					writes++
					if test.retry && writes == 1 {
						w.WriteHeader(http.StatusTooManyRequests)
						_, _ = io.WriteString(w, `{"errors":[{"status":"429","code":"RATE_LIMIT_EXCEEDED"}]}`)
						return
					}
					if test.want == "" && req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices" {
						var payload struct {
							Data struct{ Relationships map[string]asc.Relationship }
						}
						if err := json.NewDecoder(req.Body).Decode(&payload); err != nil || len(payload.Data.Relationships) != 2 || payload.Data.Relationships["subscriptionPricePoint"].Data.ID != "new-base" || payload.Data.Relationships["subscription"].Data.ID != "8000000003" {
							t.Errorf("unexpected implicit-territory write: %+v, %v", payload, err)
						}
						_, _ = io.WriteString(w, `{"data":{"type":"subscriptionPrices","id":"saved"}}`)
						return
					}
					t.Errorf("unexpected mutation %s %s", req.Method, req.URL)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if strings.HasSuffix(req.URL.Path, "/relationships/prices") {
					_, _ = io.WriteString(w, `{"data":[{"type":"subscriptionPrices","id":"current"}],"links":{}}`)
					return
				}
				if req.URL.Path == "/v1/subscriptionPricePoints/new-base" {
					if !test.noTerritory || req.URL.Query().Get("include") != "territory" {
						t.Errorf("unexpected price point lookup %s", req.URL)
					}
					body := test.pointResponse
					if body == "" {
						body = `{"data":{"type":"subscriptionPricePoints","id":"new-base","relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}}`
					}
					_, _ = io.WriteString(w, body)
					return
				}
				if req.URL.Path == "/v1/subscriptionPriceTiers" {
					if !test.force || req.URL.Query().Get("filter[subscription]") != "8000000003" {
						t.Fatalf("unexpected volume tier query %s", req.URL)
					}
					if test.retry && (writes == 0 || test.baseChanged) {
						_, _ = io.WriteString(w, `{"data":[]}`)
						return
					}
					_, _ = io.WriteString(w, `{"data":[{"type":"subscriptionPriceTiers","id":"other-territory-tier"}],"links":{}}`)
					return
				}
				if strings.HasSuffix(req.URL.Path, "/equalizations") {
					_, _ = io.WriteString(w, `{"data":[]}`)
					return
				}
				if req.URL.Query().Get("include") != "subscriptionPricePoint,territory,additionalPriceTiers" {
					t.Fatalf("price read must include tiers, got %s", req.URL.RawQuery)
				}
				if test.force {
					point := "new-base"
					if test.baseChanged && writes > 0 {
						point = "concurrent-alternative"
					}
					_, _ = io.WriteString(w, subscriptionPriceListFixture(point, "USA", ""))
					return
				}
				if test.retry && writes == 0 {
					_, _ = io.WriteString(w, subscriptionPriceListFixture("old-base", "USA", ""))
					return
				}
				tierTerritory := test.tierTerritory
				if tierTerritory == "" {
					tierTerritory = "USA"
				}
				nextURL := ""
				if test.duplicatePage && req.URL.Query().Get("cursor") == "" {
					nextURL = "/v1/subscriptions/8000000003/prices?cursor=second"
				}
				total := 1
				if test.duplicatePage {
					total = 2
				}
				_, _ = fmt.Fprintf(w, `{"data":[{"type":"subscriptionPrices","id":"current","relationships":{"territory":{"data":{"type":"territories","id":%q}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"old-base"}},"additionalPriceTiers":{"data":[{"type":"subscriptionPriceTiers","id":"tier-1"}]}}}],"links":{"next":%q},"meta":{"paging":{"total":%d}}}`, tierTerritory, nextURL, total)
			}
			args := []string{"subscriptions", "pricing", "prices", "set", "--subscription-id", "8000000003", "--price-point", "new-base"}
			if !test.noTerritory {
				args = append(args, "--territory", "USA")
			}
			if test.force {
				args = append(args, "--force")
			}
			_, stderr, err := runSubscriptionPricesListQuerySurface(t, args...)
			if test.want == "" {
				if err != nil || writes != 1 {
					t.Fatalf("implicit price change: writes=%d stderr=%q err=%v", writes, stderr, err)
				}
				return
			}
			if test.retry && writes != 1 {
				t.Fatalf("expected only the rejected write, got %d", writes)
			}
			if test.baseChanged {
				if err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("run = %v, want %q", err, test.want)
				}
				return
			}
			if rootcmd.ExitCodeFromError(err) != rootcmd.ExitUsage || !strings.Contains(stderr, test.want) {
				t.Fatalf("run = %v, stderr %q, want %q", err, stderr, test.want)
			}
		})
	}
}

func TestSubscriptionsPricesSetRequiresCompleteTierProofForChangedPrices(t *testing.T) {
	for _, test := range []struct {
		name, relationship, planType string
		matching, allowed            bool
	}{
		{name: "missing linkage"},
		{name: "explicit upfront missing linkage", planType: "UPFRONT"},
		{name: "null linkage", relationship: `,"additionalPriceTiers":{"data":null}`},
		{name: "malformed linkage", relationship: `,"additionalPriceTiers":{"data":{}}`},
		{name: "links only", relationship: `,"additionalPriceTiers":{"links":{"related":"tiers"}}`},
		{name: "unchanged price", matching: true, allowed: true},
		{name: "monthly price", planType: "MONTHLY", allowed: true},
		{name: "verified single tier", relationship: `,"additionalPriceTiers":{"data":[]}`, allowed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			captured := subscriptionPricesListQuerySurfaceStub(t)
			writes := 0
			captured.response = func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if req.Method != http.MethodGet {
					writes++
					_, _ = io.WriteString(w, `{"data":{"type":"subscriptionPrices","id":"saved"}}`)
					return
				}
				if strings.HasSuffix(req.URL.Path, "/relationships/prices") {
					_, _ = io.WriteString(w, `{"data":[{"type":"subscriptionPrices","id":"current"}],"links":{}}`)
					return
				}
				if req.URL.Path != "/v1/subscriptions/8000000003/prices" {
					t.Fatalf("unexpected request %s %s", req.Method, req.URL)
				}
				point := "old-base"
				if test.matching {
					point = "new-base"
				}
				_, _ = fmt.Fprintf(w, `{"data":[{"type":"subscriptionPrices","id":"current","attributes":{"planType":%q},"relationships":{"territory":{"data":{"type":"territories","id":"NOR"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":%q}}%s}}],"links":{}}`, test.planType, point, test.relationship)
			}
			args := []string{"subscriptions", "pricing", "prices", "set", "--subscription-id", "8000000003", "--price-point", "new-base", "--territory", "NOR"}
			if test.planType != "" {
				args = append(args, "--plan-type", test.planType)
			}
			stdout, stderr, err := runSubscriptionPricesListQuerySurface(t, args...)
			wantWrites := 0
			if test.allowed && !test.matching {
				wantWrites = 1
			}
			if (err == nil) != test.allowed || writes != wantWrites {
				t.Fatalf("allowed=%t writes=%d stdout=%s stderr=%s err=%v", test.allowed, writes, stdout, stderr, err)
			}
		})
	}
}
