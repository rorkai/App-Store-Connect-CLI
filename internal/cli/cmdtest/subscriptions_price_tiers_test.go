package cmdtest

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSubscriptionPriceTiersListQueriesAndPagination(t *testing.T) {
	setupAuth(t)
	requests := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionPriceTiers" {
			t.Fatalf("request = %s %s", req.Method, req.URL)
		}
		if requests == 1 {
			want := url.Values{"filter[subscription]": {"8000000001"}, "filter[territory]": {"USA,GBR"}, "fields[subscriptionPriceTiers]": {"minQuantity,subscriptionPricePoint"}, "fields[subscriptionPricePoints]": {"customerPrice"}, "include": {"subscriptionPricePoint"}, "limit": {"200"}}
			if !reflect.DeepEqual(req.URL.Query(), want) {
				t.Fatalf("query = %v, want %v", req.URL.Query(), want)
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPriceTiers","id":"tier-1","attributes":{"minQuantity":10}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/subscriptionPriceTiers?cursor=next"}}`)
		}
		if requests != 2 || req.URL.Query().Get("cursor") != "next" {
			t.Fatalf("unexpected page %s", req.URL)
		}
		if req.URL.Query().Get("filter[subscription]") != "8000000001" || req.URL.Query().Get("fields[subscriptionPriceTiers]") != "minQuantity,subscriptionPricePoint" || req.URL.Query().Get("include") != "subscriptionPricePoint" {
			t.Fatalf("continuation dropped requested filters or sparse relationships: %s", req.URL)
		}
		return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPriceTiers","id":"tier-2","attributes":{"minQuantity":20}}],"links":{}}`)
	}))
	stdout, stderr, err := runCommand(t, []string{"subscriptions", "pricing", "price-tiers", "list", "--subscription-id", "8000000001", "--territory", "US,GB", "--fields", "minQuantity", "--price-point-fields", "customerPrice", "--paginate", "--output", "json"})
	if err != nil {
		t.Fatalf("run = %v stderr=%q", err, stderr)
	}
	var got struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				MinQuantity int `json:"minQuantity"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if requests != 2 || len(got.Data) != 2 || got.Data[1].ID != "tier-2" || got.Data[1].Attributes.MinQuantity != 20 {
		t.Fatalf("requests=%d output=%s", requests, stdout)
	}
}

func TestSubscriptionPriceTierAdditionalReads(t *testing.T) {
	for _, test := range []struct{ name, path, response string }{
		{"additional-price-tiers", "/v1/subscriptionPrices/price-1/additionalPriceTiers", `{"data":[{"type":"subscriptionPriceTiers","id":"tier-1","attributes":{"minQuantity":10}}],"links":{}}`},
		{"additional-price-tiers-links", "/v1/subscriptionPrices/price-1/relationships/additionalPriceTiers", `{"data":[{"type":"subscriptionPriceTiers","id":"tier-1"}],"links":{}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != test.path || req.URL.Query().Get("limit") != "25" {
					t.Fatalf("request = %s %s", req.Method, req.URL)
				}
				return jsonResponse(http.StatusOK, test.response)
			}))
			stdout, stderr, err := runCommand(t, []string{"subscriptions", "pricing", "prices", test.name, "--price-id", "price-1", "--limit", "25", "--output", "json"})
			if err != nil {
				t.Fatalf("run = %v stderr=%q", err, stderr)
			}
			var got struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatal(err)
			}
			if len(got.Data) != 1 || got.Data[0].ID != "tier-1" {
				t.Fatalf("output=%s", stdout)
			}
		})
	}
}

func TestSubscriptionPriceTierCalculationsSendRequestAndRender(t *testing.T) {
	for _, test := range []struct {
		name, path, input, flag, response string
		args                              []string
	}{
		{
			name:     "equalize",
			path:     "/v1/subscriptionPriceTierEqualizations",
			input:    `[{"minQuantity":10,"subscriptionPricePointId":"tier-point"}]`,
			flag:     "--additional-price-tiers",
			response: `{"data":{"type":"subscriptionPriceTierEqualizations","id":"equalized-1","attributes":{"equalizedPrices":[{"territoryId":"USA","basePricePointId":"base-point","additionalPriceTiers":[{"minQuantity":10,"subscriptionPricePointId":"tier-point"}]}]}}}`,
			args: []string{
				"price-tiers", "equalize", "--base-price-point", "base-point",
				"--include", "priceTierPricePoints", "--price-point-fields", "customerPrice",
				"--fields", "equalizedPrices,priceTierPricePoints", "--price-points-limit", "25",
			},
		},
		{
			name:     "preview",
			path:     "/v1/subscriptionPriceChangePreviews",
			input:    `[{"territoryId":"USA","startDate":"2027-01-01","basePricePointId":"base-point","additionalPriceTiers":[{"minQuantity":10,"subscriptionPricePointId":"tier-point"}]}]`,
			flag:     "--proposed-changes",
			response: `{"data":{"type":"subscriptionPriceChangePreviews","id":"preview-1","attributes":{"changeResults":[{"territoryId":"USA","changeType":"INCREASE_PRESERVABLE"}]}}}`,
			args:     []string{"price-change-preview"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			input := filepath.Join(t.TempDir(), "input.json")
			if err := os.WriteFile(input, []byte(test.input), 0o600); err != nil {
				t.Fatal(err)
			}
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost || req.URL.Path != test.path {
					t.Fatalf("request = %s %s", req.Method, req.URL)
				}
				var payload struct {
					Data struct {
						Type          string                     `json:"type"`
						Attributes    map[string]json.RawMessage `json:"attributes"`
						Relationships map[string]struct {
							Data struct {
								ID string `json:"id"`
							} `json:"data"`
						} `json:"relationships"`
					} `json:"data"`
				}
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if payload.Data.Type != strings.TrimPrefix(test.path, "/v1/") {
					t.Fatalf("resource type = %q", payload.Data.Type)
				}
				if payload.Data.Relationships["subscription"].Data.ID != "8000000001" {
					t.Fatalf("subscription = %+v", payload)
				}
				key := "proposedChanges"
				if test.name == "equalize" {
					key = "additionalPriceTiers"
					if payload.Data.Relationships["basePricePoint"].Data.ID != "base-point" {
						t.Fatalf("base point = %+v", payload)
					}
					if req.URL.Query().Get("limit[priceTierPricePoints]") != "25" || req.URL.Query().Get("fields[subscriptionPricePoints]") != "customerPrice" {
						t.Fatalf("query=%s", req.URL.RawQuery)
					}
				}
				var got, want any
				if err := json.Unmarshal(payload.Data.Attributes[key], &got); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(test.input), &want); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("attributes=%v want=%v", got, want)
				}
				return jsonResponse(http.StatusCreated, test.response)
			}))
			args := append([]string{"subscriptions", "pricing"}, test.args...)
			args = append(args, "--subscription-id", "8000000001", test.flag, input, "--output", "table")
			stdout, stderr, err := runCommand(t, args)
			if err != nil {
				t.Fatalf("run = %v stderr=%q", err, stderr)
			}
			if !strings.Contains(stdout, "USA") || (test.name == "preview" && !strings.Contains(stdout, "INCREASE_PRESERVABLE")) || (test.name == "equalize" && !strings.Contains(stdout, "tier-point")) {
				t.Fatalf("output=%s", stdout)
			}
		})
	}
}

func TestSubscriptionPriceTierUsageValidation(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"price-tiers", "list"}, "--subscription-id is required"},
		{[]string{"price-tiers", "list", "--subscription-id", "8000000001", "--fields", "invalid"}, "--fields"},
		{[]string{"price-tiers", "equalize", "--subscription-id", "8000000001"}, "--base-price-point is required"},
		{[]string{"price-tiers", "equalize", "--subscription-id", "8000000001", "--base-price-point", "base-point", "--price-points-limit", "51"}, "--price-points-limit"},
		{[]string{"prices", "additional-price-tiers-links"}, "--price-id is required"},
		{[]string{"prices", "additional-price-tiers", "--price-id", "price-1", "--next", "https://api.appstoreconnect.apple.com/v1/subscriptionPrices/price-2/additionalPriceTiers?cursor=next"}, "--price-id"},
		{[]string{"prices", "additional-price-tiers-links", "--price-id", "price-1", "--next", "https://api.appstoreconnect.apple.com/v1/subscriptionPrices/price-2/relationships/additionalPriceTiers?cursor=next"}, "--price-id"},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			args := append([]string{"subscriptions", "pricing"}, test.args...)
			stdout, stderr, err := runCommand(t, args)
			if err == nil || !strings.Contains(stderr, test.want) || stdout != "" {
				t.Fatalf("err=%v stderr=%q stdout=%q", err, stderr, stdout)
			}
		})
	}
}

func TestSubscriptionPriceChangePreviewRejectsInvalidInputBeforeHTTP(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{`[{"territoryId":"USA","additionalPriceTiers":[{"subscriptionPricePointId":"point"}]}]`, "minQuantity is required"},
		{`[{"territoryId":"USA","additionalPriceTiers":null}]`, "additionalPriceTiers must be an array"},
		{`[{"territoryId":"USA","startDate":"tomorrow"}]`, "startDate must use YYYY-MM-DD"},
		{`[{"territoryId":null}]`, "territoryId must be a string"},
		{`[null]`, "must be an object"},
		{`[{"pricePointId":"point"}]`, "unknown field"},
	} {
		t.Run(test.want, func(t *testing.T) {
			setupAuth(t)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
				return nil, nil
			}))
			input := filepath.Join(t.TempDir(), "changes.json")
			if err := os.WriteFile(input, []byte(test.input), 0o600); err != nil {
				t.Fatal(err)
			}
			stdout, stderr, err := runCommand(t, []string{"subscriptions", "pricing", "price-change-preview", "--subscription-id", "8000000001", "--proposed-changes", input})
			if err == nil || !strings.Contains(stderr, test.want) || stdout != "" {
				t.Fatalf("err=%v stderr=%q stdout=%q", err, stderr, stdout)
			}
		})
	}
}

func TestSubscriptionPriceCalculationsPreserveEmptyResponseArrays(t *testing.T) {
	for _, test := range []struct {
		name, path, response string
		args                 []string
	}{
		{name: "equalize empty", path: "/v1/subscriptionPriceTierEqualizations", response: `{"data":{"type":"subscriptionPriceTierEqualizations","id":"result-1","attributes":{"equalizedPrices":[]}}}`, args: []string{"price-tiers", "equalize", "--base-price-point", "point-1"}},
		{name: "equalize nested empty", path: "/v1/subscriptionPriceTierEqualizations", response: `{"data":{"type":"subscriptionPriceTierEqualizations","id":"result-1","attributes":{"equalizedPrices":[{"territoryId":"USA","additionalPriceTiers":[]}]}}}`, args: []string{"price-tiers", "equalize", "--base-price-point", "point-1"}},
		{name: "equalize sparse", path: "/v1/subscriptionPriceTierEqualizations", response: `{"data":{"type":"subscriptionPriceTierEqualizations","id":"result-1","attributes":{}}}`, args: []string{"price-tiers", "equalize", "--base-price-point", "point-1"}},
		{name: "equalize null and unknown", path: "/v1/subscriptionPriceTierEqualizations", response: `{"data":{"type":"subscriptionPriceTierEqualizations","id":"result-1","attributes":{"equalizedPrices":null,"futureField":false}}}`, args: []string{"price-tiers", "equalize", "--base-price-point", "point-1"}},
		{name: "preview empty", path: "/v1/subscriptionPriceChangePreviews", response: `{"data":{"type":"subscriptionPriceChangePreviews","id":"result-1","attributes":{"changeResults":[]}}}`, args: []string{"price-change-preview"}},
		{name: "preview sparse", path: "/v1/subscriptionPriceChangePreviews", response: `{"data":{"type":"subscriptionPriceChangePreviews","id":"result-1","attributes":{}}}`, args: []string{"price-change-preview"}},
		{name: "preview null and unknown", path: "/v1/subscriptionPriceChangePreviews", response: `{"data":{"type":"subscriptionPriceChangePreviews","id":"result-1","attributes":{"changeResults":null,"futureField":false}}}`, args: []string{"price-change-preview"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost || req.URL.Path != test.path {
					t.Fatalf("request=%s %s", req.Method, req.URL)
				}
				return jsonResponse(http.StatusCreated, test.response)
			}))
			args := append([]string{"subscriptions", "pricing"}, test.args...)
			args = append(args, "--subscription-id", "8000000001", "--output", "json")
			stdout, stderr, err := runCommand(t, args)
			if err != nil {
				t.Fatalf("run=%v stderr=%q", err, stderr)
			}
			var got, want struct {
				Data struct {
					Attributes map[string]any `json:"attributes"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(test.response), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Data.Attributes, want.Data.Attributes) {
				t.Fatalf("attributes=%v want=%v", got.Data.Attributes, want.Data.Attributes)
			}
		})
	}
}
