package cmdtest

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSubscriptionBulkWritersRefuseVolumeTierReplacement(t *testing.T) {
	for _, surface := range []string{"equalize", "import", "setup", "equalize empty membership", "equalize positive total", "setup positive total", "setup initial replay", "setup repair replay"} {
		t.Run(surface, func(t *testing.T) {
			setupAuth(t)
			t.Chdir(t.TempDir())
			tiersRead, writes := 0, 0
			replay := strings.HasSuffix(surface, " replay")
			if replay {
				t.Setenv("ASC_TIMEOUT", "100ms")
				setPriceReplayRetryOptions(t)
			}
			const subscriptionID = "8000000001"
			const prices = `{"data":[{"type":"subscriptionPrices","id":"price-old","attributes":{"planType":"UPFRONT"},"relationships":{"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"point-old"}},"territory":{"data":{"type":"territories","id":"USA"}}}}],"links":{}}`
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if replay && (req.Method != http.MethodGet || writes > 0) {
					if err := delayPriceRetryRequest(req); err != nil {
						return nil, err
					}
				}
				if req.Method != http.MethodGet {
					writes++
					if replay && writes == 1 {
						return jsonHTTPResponse(429, `{"errors":[{"status":"429","code":"RATE_LIMIT_EXCEEDED"}]}`), nil
					}
					return nil, fmt.Errorf("unexpected bulk mutation")
				}
				switch req.URL.Path {
				case "/v1/subscriptionGroups/group-1/subscriptions":
					return jsonHTTPResponse(200, `{"data":[{"type":"subscriptions","id":"8000000001","attributes":{"name":"Monthly","productId":"com.example.monthly","subscriptionPeriod":"ONE_MONTH"}}],"links":{}}`), nil
				case "/v1/territories", "/v1/subscriptionAvailabilities/availability-1/availableTerritories":
					return jsonHTTPResponse(200, `{"data":[{"type":"territories","id":"USA"}],"links":{}}`), nil
				case "/v1/subscriptions/" + subscriptionID + "/subscriptionAvailability":
					return jsonHTTPResponse(200, `{"data":{"type":"subscriptionAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":false}}}`), nil
				case "/v1/subscriptions/" + subscriptionID + "/pricePoints":
					return jsonHTTPResponse(200, `{"data":[{"type":"subscriptionPricePoints","id":"point-new","attributes":{"customerPrice":"9.99"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}],"links":{}}`), nil
				case "/v1/subscriptionPricePoints/point-new/equalizations":
					return jsonHTTPResponse(200, `{"data":[],"links":{}}`), nil
				case "/v1/subscriptions/" + subscriptionID + "/relationships/prices":
					if surface == "equalize positive total" {
						return jsonHTTPResponse(200, `{"data":[],"meta":{"paging":{"total":1}}}`), nil
					}
					if surface == "equalize empty membership" {
						return jsonHTTPResponse(200, `{"data":[],"links":{"next":"/v1/subscriptions/8000000001/relationships/prices?cursor=two"}}`), nil
					}
					return jsonHTTPResponse(200, `{"data":[{"type":"subscriptionPrices","id":"price-old"}],"links":{}}`), nil
				case "/v1/subscriptions/" + subscriptionID + "/prices":
					if surface == "setup initial replay" && writes == 0 {
						return jsonHTTPResponse(200, `{"data":[]}`), nil
					}
					if surface == "setup positive total" && req.URL.Query().Get("filter[planType]") == "" {
						return jsonHTTPResponse(200, `{"data":[],"meta":{"paging":{"total":1}}}`), nil
					}
					return jsonHTTPResponse(200, prices), nil
				case "/v1/subscriptionPriceTiers":
					tiersRead++
					if surface == "equalize empty membership" || surface == "setup positive total" || (replay && writes == 0) {
						return jsonHTTPResponse(200, `{"data":[],"links":{}}`), nil
					}
					if req.URL.Query().Get("filter[subscription]") != subscriptionID {
						t.Fatalf("tier proof query=%s", req.URL.RawQuery)
					}
					if surface == "import" && req.URL.Query().Get("filter[territory]") != "USA" {
						t.Fatalf("import must scope tier proof to USA: %s", req.URL.RawQuery)
					}
					return jsonHTTPResponse(200, `{"data":[{"type":"subscriptionPriceTiers","id":"tier-1","attributes":{"minQuantity":10}}],"links":{}}`), nil
				default:
					t.Fatalf("unexpected GET %s", req.URL.Path)
					return nil, nil
				}
			}))
			args := []string{"subscriptions", "pricing", "equalize", "--subscription-id", subscriptionID, "--base-price", "9.99", "--auto-start-date=false", "--confirm"}
			if surface == "import" {
				csv := filepath.Join(t.TempDir(), "prices.csv")
				if err := os.WriteFile(csv, []byte("territory,price,price_point_id\nUSA,9.99,point-new\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				args = []string{"subscriptions", "pricing", "prices", "import", "--subscription-id", subscriptionID, "--input", csv, "--confirm"}
			}
			if strings.HasPrefix(surface, "setup") {
				args = []string{"subscriptions", "setup", "--group-id", "group-1", "--reference-name", "Monthly", "--product-id", "com.example.monthly", "--subscription-period", "ONE_MONTH", "--price-point-id", "point-new", "--price-territory", "USA", "--no-verify"}
				if surface == "setup" || surface == "setup repair replay" {
					args = append(args, "--repair")
				}
			}
			out, _, err := runCommandOnFakeClock(t, append(args, "--output", "json"))
			wantTierReads, diagnostic := 1, "asc subscriptions pricing prices set --additional-price-tiers"
			if strings.HasPrefix(surface, "equalize ") {
				wantTierReads, diagnostic = 0, "unverified existing price membership"
			}
			if surface == "setup positive total" {
				wantTierReads, diagnostic = 0, "incomplete price collection"
			}
			wantWrites := 0
			switch surface {
			case "setup initial replay":
				wantWrites, wantTierReads, diagnostic = 1, 0, "cannot replay the initial price matrix"
			case "setup repair replay":
				wantWrites, wantTierReads = 1, 2
			}
			if err == nil || !strings.Contains(err.Error()+out, diagnostic) || tiersRead != wantTierReads || writes != wantWrites {
				t.Fatalf("surface=%s err=%v tier reads=%d writes=%d output=%s", surface, err, tiersRead, writes, out)
			}
		})
	}
}

func TestSubscriptionPriceImportPreservesUnaffectedWrites(t *testing.T) {
	for _, mode := range []string{"dry-run", "matched", "new", "monthly", "other territory", "missing price collection", "incomplete price collection", "tiers added between rows", "tiers after 503", "tiers after 429", "unchanged after 429"} {
		t.Run(mode, func(t *testing.T) {
			setupAuth(t)
			t.Chdir(t.TempDir())
			reads, writes := 0, 0
			retryStatus := map[string]int{"tiers after 503": 503, "tiers after 429": 429, "unchanged after 429": 429}[mode]
			if mode == "unchanged after 429" {
				t.Setenv("ASC_TIMEOUT", "100ms")
			}
			if retryStatus != 0 {
				setPriceReplayRetryOptions(t)
			}
			territory, point, plan := "USA", "point-new", "UPFRONT"
			if mode == "matched" {
				point = "point-old"
			}
			if mode == "monthly" {
				plan = "MONTHLY"
				territory = "DEU"
			}
			if mode == "other territory" {
				territory = "CAN"
			}
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if mode == "unchanged after 429" && (req.Method != http.MethodGet || writes > 0) {
					if err := delayPriceRetryRequest(req); err != nil {
						return nil, err
					}
				}
				if req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices" {
					writes++
					if writes == 1 && retryStatus != 0 {
						return jsonHTTPResponse(retryStatus, fmt.Sprintf(`{"errors":[{"status":"%d","detail":"retry later"}]}`, retryStatus)), nil
					}
					return jsonHTTPResponse(201, `{"data":{"type":"subscriptionPrices","id":"price-new"}}`), nil
				}
				if req.Method != http.MethodGet {
					t.Fatalf("unexpected mutation %s %s", req.Method, req.URL.Path)
				}
				switch req.URL.Path {
				case "/v1/subscriptions/8000000001/prices":
					if mode == "incomplete price collection" {
						return jsonHTTPResponse(200, `{"data":[],"meta":{"paging":{"total":1}}}`), nil
					}
					if mode == "missing price collection" {
						return jsonHTTPResponse(200, `{"data":null,"links":{}}`), nil
					}
					if mode == "new" || mode == "monthly" {
						return jsonHTTPResponse(200, `{"data":[],"links":{}}`), nil
					}
					return jsonHTTPResponse(200, `{"data":[{"type":"subscriptionPrices","id":"price-old","attributes":{"planType":"UPFRONT"},"relationships":{"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"point-old"}},"territory":{"data":{"type":"territories","id":"USA"}}}}],"links":{}}`), nil
				case "/v1/subscriptionPriceTiers":
					reads++
					wantTerritory := ""
					if retryStatus != 0 {
						wantTerritory = "USA"
						if writes > 0 && mode != "unchanged after 429" {
							return jsonHTTPResponse(200, `{"data":[{"type":"subscriptionPriceTiers","id":"new-tier"}]}`), nil
						}
					}
					if mode == "other territory" {
						wantTerritory = "CAN"
					}
					if mode == "tiers added between rows" {
						if writes > 0 && req.URL.Query().Get("filter[territory]") == "CAN" {
							return jsonHTTPResponse(200, `{"data":[{"type":"subscriptionPriceTiers","id":"added-between-rows"}],"links":{}}`), nil
						}
					} else if req.URL.Query().Get("filter[territory]") != wantTerritory || req.URL.Query().Get("filter[subscription]") != "8000000001" {
						t.Fatalf("unnecessary tier query %s", req.URL.RawQuery)
					}
					return jsonHTTPResponse(200, `{"data":[],"links":{}}`), nil
				default:
					t.Fatalf("unexpected GET %s", req.URL.Path)
					return nil, nil
				}
			}))
			csv := filepath.Join(t.TempDir(), "prices.csv")
			body := "territory,price,price_point_id,plan_type\n" + territory + ",9.99," + point + "," + plan + "\n"
			if mode == "tiers added between rows" {
				body += "CAN,9.99,point-can,UPFRONT\n"
			}
			if err := os.WriteFile(csv, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "8000000001", "--input", csv, "--output", "json", "--confirm"}
			if mode == "dry-run" {
				args[len(args)-1] = "--dry-run"
			}
			out, _, err := runCommandOnFakeClock(t, args)
			wantWrites, wantReads := 1, 0
			if mode == "dry-run" || mode == "matched" || strings.HasSuffix(mode, "price collection") {
				wantWrites = 0
			}
			if mode == "tiers added between rows" || retryStatus != 0 {
				wantReads, wantWrites = 2, 1
			}
			if mode == "unchanged after 429" {
				wantWrites = 2
			}
			if mode == "other territory" {
				wantReads = 1
			}
			if strings.HasSuffix(mode, "price collection") {
				if err == nil || !strings.Contains(err.Error(), "price collection") {
					t.Fatalf("expected refusal of missing price proof, got %v", err)
				}
			} else if mode == "tiers added between rows" || (retryStatus != 0 && mode != "unchanged after 429") {
				if err == nil || !strings.Contains(out, "has volume price tiers") {
					t.Fatalf("expected refusal after a new tier appeared: %v, %s", err, out)
				}
			} else if err != nil {
				t.Fatalf("import %s: %v output=%s", mode, err, out)
			}
			if writes != wantWrites || reads != wantReads {
				t.Fatalf("mode=%s tier reads=%d want %d writes=%d want %d", mode, reads, wantReads, writes, wantWrites)
			}
		})
	}
}

func setPriceReplayRetryOptions(t *testing.T) {
	t.Helper()
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
}

// Each call fits ASC_TIMEOUT, but multiple calls cannot share the same deadline.
func delayPriceRetryRequest(req *http.Request) error {
	select {
	case <-time.After(60 * time.Millisecond):
		return nil
	case <-req.Context().Done():
		return req.Context().Err()
	}
}
