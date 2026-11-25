package cmdtest

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
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

func TestSubscriptionPriceTierPaginationKeepsSelectedOperation(t *testing.T) {
	const tierPath = "/v1/subscriptionPriceTiers"
	const relatedPath = "/v1/subscriptionPrices/price-1/additionalPriceTiers"
	const linksPath = "/v1/subscriptionPrices/price-1/relationships/additionalPriceTiers"
	savedSelectors := url.Values{
		"filter[subscription]":            {"8000000001"},
		"include":                         {"subscriptionPricePoint"},
		"fields[subscriptionPriceTiers]":  {"subscriptionPricePoint"},
		"fields[subscriptionPricePoints]": {"customerPrice"},
	}
	for _, tc := range []struct {
		name, path, next string
		wantQuery        url.Values
		args             []string
		redirectAt       int
		wantRequests     int
		wantError        bool
	}{
		{name: "list collection drift", path: tierPath, next: "/v1/subscriptions?cursor=foreign", args: []string{"price-tiers", "list", "--subscription-id", "8000000001", "--paginate"}, wantRequests: 1, wantError: true},
		{name: "list filter drift", path: tierPath, next: tierPath + "?filter%5Bsubscription%5D=8000000002&cursor=foreign", args: []string{"price-tiers", "list", "--subscription-id", "8000000001", "--paginate"}, wantRequests: 1, wantError: true},
		{name: "manual list filter drift", path: tierPath, next: tierPath + "?filter%5Bsubscription%5D=8000000002&cursor=foreign", args: []string{"price-tiers", "list", "--next", "https://api.appstoreconnect.apple.com" + tierPath + "?filter%5Bsubscription%5D=8000000001&cursor=first", "--paginate"}, wantRequests: 1, wantError: true},
		{name: "related price drift", path: relatedPath, next: "/v1/subscriptionPrices/price-2/additionalPriceTiers?cursor=foreign", args: []string{"prices", "additional-price-tiers", "--price-id", "price-1", "--paginate"}, wantRequests: 1, wantError: true},
		{name: "relationship price drift", path: linksPath, next: "/v1/subscriptionPrices/price-2/relationships/additionalPriceTiers?cursor=foreign", args: []string{"prices", "additional-price-tiers-links", "--price-id", "price-1", "--paginate"}, wantRequests: 1, wantError: true},
		{name: "related collection drift", path: relatedPath, next: linksPath + "?cursor=foreign", args: []string{"prices", "additional-price-tiers", "--price-id", "price-1", "--paginate"}, wantRequests: 1, wantError: true},
		{name: "manual absolute related cursor", path: relatedPath, args: []string{"prices", "additional-price-tiers", "--next", "https://api.appstoreconnect.apple.com" + relatedPath + "?cursor=first"}, wantRequests: 1},
		{name: "manual absolute relationship cursor", path: linksPath, args: []string{"prices", "additional-price-tiers-links", "--next", "https://api.appstoreconnect.apple.com" + linksPath + "?cursor=first"}, wantRequests: 1},
		{name: "manual list wrong operation", path: tierPath, args: []string{"price-tiers", "list", "--next", "https://api.appstoreconnect.apple.com/v1/subscriptions?cursor=foreign"}, wantError: true},
		{name: "manual related wrong operation", path: relatedPath, args: []string{"prices", "additional-price-tiers", "--next", "https://api.appstoreconnect.apple.com" + linksPath + "?cursor=foreign"}, wantError: true},
		{name: "manual encoded collection alias", path: relatedPath, args: []string{"prices", "additional-price-tiers", "--next", "https://api.appstoreconnect.apple.com/v1/subscriptionPrices/price-1%2Frelationships/additionalPriceTiers?cursor=foreign"}, wantError: true},
		{name: "manual cursor pagination drift", path: linksPath, next: "/v1/subscriptionPrices/price-2/relationships/additionalPriceTiers?cursor=foreign", args: []string{"prices", "additional-price-tiers-links", "--next", "https://api.appstoreconnect.apple.com" + linksPath + "?cursor=first", "--paginate"}, wantRequests: 1, wantError: true},
		{name: "initial redirect drift", path: relatedPath, redirectAt: 1, args: []string{"prices", "additional-price-tiers", "--price-id", "price-1"}, wantRequests: 1, wantError: true},
		{name: "continuation redirect drift", path: linksPath, next: linksPath + "?cursor=second", redirectAt: 2, args: []string{"prices", "additional-price-tiers-links", "--price-id", "price-1", "--paginate"}, wantRequests: 2, wantError: true},
		{name: "manual list opaque cursor", path: tierPath, next: tierPath + "?cursor=second", wantQuery: savedSelectors, args: []string{"price-tiers", "list", "--next", "https://api.appstoreconnect.apple.com" + tierPath + "?" + savedSelectors.Encode() + "&cursor=first", "--paginate"}, wantRequests: 2},
		{name: "related valid pagination", path: relatedPath, next: relatedPath + "?cursor=second", args: []string{"prices", "additional-price-tiers", "--price-id", "price-1", "--paginate"}, wantRequests: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupAuth(t)
			requests := 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.Method != http.MethodGet || req.URL.Path != tc.path {
					t.Errorf("request left selected operation: %s %s", req.Method, req.URL)
				}
				for key, want := range tc.wantQuery {
					if got := req.URL.Query()[key]; !reflect.DeepEqual(got, want) {
						t.Errorf("page %d query %s = %v, want %v", requests, key, got, want)
					}
				}
				if requests == tc.redirectAt {
					return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"https://api.appstoreconnect.apple.com/v1/subscriptions?cursor=redirect"}}, Body: http.NoBody}, nil
				}
				next := ""
				if requests == 1 {
					next = tc.next
				}
				return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPriceTiers","id":`+strconv.Quote("tier-"+strconv.Itoa(requests))+`}],"links":{"next":`+strconv.Quote(next)+`}}`)
			}))
			args := append([]string{"subscriptions", "pricing"}, tc.args...)
			args = append(args, "--output", "json")
			stdout, stderr, err := runCommand(t, args)
			if (err != nil) != tc.wantError || requests != tc.wantRequests {
				t.Fatalf("requests=%d want=%d stdout=%q stderr=%q err=%v", requests, tc.wantRequests, stdout, stderr, err)
			}
			if tc.wantError {
				if stdout != "" {
					t.Fatalf("failed pagination printed partial output: %q", stdout)
				}
				return
			}
			var result struct{ Data []json.RawMessage }
			if err := json.Unmarshal([]byte(stdout), &result); err != nil || len(result.Data) != tc.wantRequests {
				t.Fatalf("output=%q error=%v", stdout, err)
			}
		})
	}
}

func TestSubscriptionPriceTierIncludedPricePointOutput(t *testing.T) {
	const included = `"included":[{
		"type":"subscriptionPricePoints","id":"point-1",
		"attributes":{"customerPrice":"4.99","proceeds":"3.50","proceedsYear2":"4.25"},
		"relationships":{"territory":{"data":{"type":"territories","id":"USA"}},
			"equalizations":{"links":{"related":"/standard"}},"adjustedEqualizations":{"links":{"related":"/adjusted"}}}
	}]`
	const points = `"relationships":{"priceTierPricePoints":{"data":[{"type":"subscriptionPricePoints","id":"point-1"},{"type":"subscriptionPricePoints","id":"point-2"}]}}`
	for _, tc := range []struct {
		name, response string
		args, want     []string
	}{
		{
			name:     "tier list",
			response: `{"data":[{"type":"subscriptionPriceTiers","id":"tier-1","relationships":{"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"point-1"}}}}],` + included + `}`,
			args:     []string{"list", "--include", "subscriptionPricePoint", "--output", "markdown"},
			want:     []string{"Customer Price", "tier-1", "point-1", "4.99", "3.50", "4.25", "USA", "/standard", "/adjusted"},
		},
		{
			name:     "equalization with matrix",
			response: `{"data":{"type":"subscriptionPriceTierEqualizations","id":"eq","attributes":{"equalizedPrices":[{"territoryId":"GBR","basePricePointId":"base-point"}]},` + points + `},` + included + `}`,
			args:     []string{"equalize", "--base-price-point", "base-point", "--include", "priceTierPricePoints", "--output", "table"},
			want:     []string{"Base Price Point", "GBR", "base-point", "point-1", "Customer Price", "4.99", "point-2"},
		},
		{
			name:     "relationship only",
			response: `{"data":{"type":"subscriptionPriceTierEqualizations","id":"eq",` + points + `}}`,
			args:     []string{"equalize", "--base-price-point", "base-point", "--fields", "priceTierPricePoints", "--output", "markdown"},
			want:     []string{"Price Point", "point-1", "point-2"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupAuth(t)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, tc.response)
			}))
			args := append([]string{"subscriptions", "pricing", "price-tiers"}, tc.args...)
			stdout, stderr, err := runCommand(t, append(args, "--subscription-id", "8000000001"))
			if err != nil {
				t.Fatalf("run = %v stderr=%q", err, stderr)
			}
			for _, value := range tc.want {
				if !strings.Contains(stdout, value) {
					t.Errorf("missing %q in %s", value, stdout)
				}
			}
		})
	}
}
