package cmdtest

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubscriptionBulkWritersRefuseVolumeTierReplacement(t *testing.T) {
	for _, surface := range []string{"equalize", "import", "setup", "equalize empty membership"} {
		t.Run(surface, func(t *testing.T) {
			setupAuth(t)
			t.Chdir(t.TempDir())
			tiersRead, writes := 0, 0
			const subscriptionID = "8000000001"
			const prices = `{"data":[{"type":"subscriptionPrices","id":"price-old","attributes":{"planType":"UPFRONT"},"relationships":{"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"point-old"}},"territory":{"data":{"type":"territories","id":"USA"}}}}],"links":{}}`
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					writes++
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
					if surface == "equalize empty membership" {
						return jsonHTTPResponse(200, `{"data":[],"links":{"next":"/v1/subscriptions/8000000001/relationships/prices?cursor=two"}}`), nil
					}
					return jsonHTTPResponse(200, `{"data":[{"type":"subscriptionPrices","id":"price-old"}],"links":{}}`), nil
				case "/v1/subscriptions/" + subscriptionID + "/prices":
					return jsonHTTPResponse(200, prices), nil
				case "/v1/subscriptionPriceTiers":
					tiersRead++
					if surface == "equalize empty membership" {
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
			if surface == "setup" {
				args = []string{"subscriptions", "setup", "--group-id", "group-1", "--reference-name", "Monthly", "--product-id", "com.example.monthly", "--subscription-period", "ONE_MONTH", "--price-point-id", "point-new", "--price-territory", "USA", "--repair", "--no-verify"}
			}
			out, _, err := runCommand(t, append(args, "--output", "json"))
			wantTierReads, diagnostic := 1, "asc subscriptions pricing prices set --additional-price-tiers"
			if surface == "equalize empty membership" {
				wantTierReads, diagnostic = 0, "unverified existing price membership"
			}
			if err == nil || !strings.Contains(err.Error()+out, diagnostic) || tiersRead != wantTierReads || writes != 0 {
				t.Fatalf("surface=%s err=%v tier reads=%d writes=%d output=%s", surface, err, tiersRead, writes, out)
			}
		})
	}
}

func TestSubscriptionPriceImportPreservesUnaffectedWrites(t *testing.T) {
	for _, mode := range []string{"dry-run", "matched", "new", "monthly", "other territory", "missing price collection"} {
		t.Run(mode, func(t *testing.T) {
			setupAuth(t)
			t.Chdir(t.TempDir())
			reads, writes := 0, 0
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
				if req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices" {
					writes++
					return jsonHTTPResponse(201, `{"data":{"type":"subscriptionPrices","id":"price-new"}}`), nil
				}
				if req.Method != http.MethodGet {
					t.Fatalf("unexpected mutation %s %s", req.Method, req.URL.Path)
				}
				switch req.URL.Path {
				case "/v1/subscriptions/8000000001/prices":
					if mode == "missing price collection" {
						return jsonHTTPResponse(200, `{"data":null,"links":{}}`), nil
					}
					if mode == "new" || mode == "monthly" {
						return jsonHTTPResponse(200, `{"data":[],"links":{}}`), nil
					}
					return jsonHTTPResponse(200, `{"data":[{"type":"subscriptionPrices","id":"price-old","attributes":{"planType":"UPFRONT"},"relationships":{"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"point-old"}},"territory":{"data":{"type":"territories","id":"USA"}}}}],"links":{}}`), nil
				case "/v1/subscriptionPriceTiers":
					reads++
					if mode != "other territory" || req.URL.Query().Get("filter[territory]") != "CAN" || req.URL.Query().Get("filter[subscription]") != "8000000001" {
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
			if err := os.WriteFile(csv, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"subscriptions", "pricing", "prices", "import", "--subscription-id", "8000000001", "--input", csv, "--output", "json", "--confirm"}
			if mode == "dry-run" {
				args[len(args)-1] = "--dry-run"
			}
			out, _, err := runCommand(t, args)
			wantWrites, wantReads := 1, 0
			if mode == "dry-run" || mode == "matched" || mode == "missing price collection" {
				wantWrites = 0
			}
			if mode == "other territory" {
				wantReads = 1
			}
			if mode == "missing price collection" {
				if err == nil || !strings.Contains(err.Error(), "missing price collection") {
					t.Fatalf("expected refusal of missing price proof, got %v", err)
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
