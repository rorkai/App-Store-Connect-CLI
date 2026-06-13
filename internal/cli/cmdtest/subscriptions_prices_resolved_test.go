package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestSubscriptionsPricingPricesListResolvedJSON(t *testing.T) {
	setupAuth(t)

	const secondURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/8000000001/prices?cursor=Mg"

	requestCount := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/prices" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("include") != "subscriptionPricePoint,territory" {
				t.Fatalf("expected include query, got %q", query.Get("include"))
			}
			if query.Get("fields[subscriptionPricePoints]") != "customerPrice,proceeds,proceedsYear2" {
				t.Fatalf("unexpected price point fields: %q", query.Get("fields[subscriptionPricePoints]"))
			}
			if query.Get("fields[territories]") != "currency" {
				t.Fatalf("unexpected territory fields: %q", query.Get("fields[territories]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", query.Get("limit"))
			}

			body := `{
				"data":[
					{
						"type":"subscriptionPrices",
						"id":"price-current-usa",
						"attributes":{"startDate":"2025-01-01","preserved":false},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"USA"}},
							"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-current-usa"}}
						}
					},
					{
						"type":"subscriptionPrices",
						"id":"price-future-usa",
						"attributes":{"startDate":"2030-01-01","preserved":false},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"USA"}},
							"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-future-usa"}}
						}
					}
				],
				"included":[
					{"type":"subscriptionPricePoints","id":"pp-current-usa","attributes":{"customerPrice":"9.99","proceeds":"7.00","proceedsYear2":"8.49"}},
					{"type":"subscriptionPricePoints","id":"pp-future-usa","attributes":{"customerPrice":"12.99","proceeds":"10.00","proceedsYear2":"11.49"}},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				],
				"links":{"next":"` + secondURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/prices" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("cursor") != "Mg" {
				t.Fatalf("expected cursor=Mg, got %q", query.Get("cursor"))
			}
			if query.Get("include") != "subscriptionPricePoint,territory" {
				t.Fatalf("expected include query on paginated request, got %q", query.Get("include"))
			}
			if query.Get("fields[subscriptionPricePoints]") != "customerPrice,proceeds,proceedsYear2" {
				t.Fatalf("unexpected paginated price point fields: %q", query.Get("fields[subscriptionPricePoints]"))
			}
			if query.Get("fields[territories]") != "currency" {
				t.Fatalf("unexpected paginated territory fields: %q", query.Get("fields[territories]"))
			}

			body := `{
				"data":[
					{
						"type":"subscriptionPrices",
						"id":"price-current-gbr",
						"attributes":{"startDate":"2024-06-01","preserved":false},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"GBR"}},
							"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-current-gbr"}}
						}
					},
					{
						"type":"subscriptionPrices",
						"id":"price-undated-fra",
						"attributes":{"preserved":false},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"FRA"}},
							"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-undated-fra"}}
						}
					}
				],
				"included":[
					{"type":"subscriptionPricePoints","id":"pp-current-gbr","attributes":{"customerPrice":"7.99","proceeds":"5.60","proceedsYear2":"6.40"}},
					{"type":"subscriptionPricePoints","id":"pp-undated-fra","attributes":{"customerPrice":"6.99","proceeds":"4.90","proceedsYear2":"5.70"}},
					{"type":"territories","id":"GBR","attributes":{"currency":"GBP"}},
					{"type":"territories","id":"FRA","attributes":{"currency":"EUR"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "list",
			"--subscription-id", "8000000001",
			"--resolved",
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
	if len(result.Prices) != 2 {
		t.Fatalf("expected 2 resolved prices, got %+v", result.Prices)
	}
	if result.Prices[0].Territory != "GBR" || result.Prices[0].CustomerPrice != "7.99" {
		t.Fatalf("unexpected first resolved row: %+v", result.Prices[0])
	}
	if result.Prices[1].Territory != "USA" || result.Prices[1].CustomerPrice != "9.99" {
		t.Fatalf("unexpected second resolved row: %+v", result.Prices[1])
	}
}

func TestSubscriptionsPricingPricesListResolvedPlanTypeSupportsRelativeNextURL(t *testing.T) {
	setupAuth(t)

	requestCount := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/prices" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		query := req.URL.Query()
		if got := query.Get("filter[planType]"); got != "MONTHLY" {
			t.Fatalf("request %d: expected filter[planType]=MONTHLY, got %q", requestCount, got)
		}
		if got := query.Get("include"); got != "subscriptionPricePoint,territory" {
			t.Fatalf("request %d: expected resolved includes, got %q", requestCount, got)
		}

		switch requestCount {
		case 1:
			return jsonResponse(http.StatusOK, `{
				"data":[{
					"type":"subscriptionPrices",
					"id":"price-nor",
					"relationships":{
						"territory":{"data":{"type":"territories","id":"NOR"}},
						"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-nor"}}
					}
				}],
				"included":[
					{"type":"subscriptionPricePoints","id":"pp-nor","attributes":{"customerPrice":"5.00"}},
					{"type":"territories","id":"NOR","attributes":{"currency":"NOK"}}
				],
				"links":{"next":"/v1/subscriptions/8000000001/prices?cursor=next"}
			}`)
		case 2:
			if got := query.Get("cursor"); got != "next" {
				t.Fatalf("expected cursor=next, got %q", got)
			}
			return jsonResponse(http.StatusOK, `{
				"data":[{
					"type":"subscriptionPrices",
					"id":"price-deu",
					"relationships":{
						"territory":{"data":{"type":"territories","id":"DEU"}},
						"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-deu"}}
					}
				}],
				"included":[
					{"type":"subscriptionPricePoints","id":"pp-deu","attributes":{"customerPrice":"4.99"}},
					{"type":"territories","id":"DEU","attributes":{"currency":"EUR"}}
				],
				"links":{"next":""}
			}`)
		default:
			t.Fatalf("unexpected request count: %d", requestCount)
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "list",
			"--subscription-id", "8000000001",
			"--resolved",
			"--plan-type", "MONTHLY",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q stdout=%q", runErr, stderr, stdout)
	}

	var result shared.ResolvedPricesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal() error = %v, stdout = %q", err, stdout)
	}
	if len(result.Prices) != 2 {
		t.Fatalf("expected two resolved prices, got %+v", result.Prices)
	}
}

func TestSubscriptionsPricingPricesListResolvedTable(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/prices" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}

		body := `{
			"data":[
				{
					"type":"subscriptionPrices",
					"id":"price-current-usa",
					"attributes":{"startDate":"2025-01-01","preserved":false},
					"relationships":{
						"territory":{"data":{"type":"territories","id":"USA"}},
						"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-current-usa"}}
					}
				}
			],
			"included":[
				{"type":"subscriptionPricePoints","id":"pp-current-usa","attributes":{"customerPrice":"9.99","proceeds":"7.00","proceedsYear2":"8.49"}},
				{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
			],
			"links":{"next":""}
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "list",
			"--subscription-id", "8000000001",
			"--resolved",
			"--output", "table",
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
	if !strings.Contains(stdout, "Customer Price") || !strings.Contains(stdout, "Proceeds Y2") {
		t.Fatalf("expected resolved table headers, got %q", stdout)
	}
	if !strings.Contains(stdout, "USA") || !strings.Contains(stdout, "9.99") || !strings.Contains(stdout, "8.49") {
		t.Fatalf("expected resolved table row, got %q", stdout)
	}
}

func TestSubscriptionsPricingPricesListResolvedRejectsNext(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	const nextURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/8000000001/prices?cursor=Mg"

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "list",
			"--next", nextURL,
			"--resolved",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --resolved cannot be combined with --next") {
		t.Fatalf("expected resolved next usage error, got %q", stderr)
	}
}

func TestSubscriptionsPricingPricesListRawOutputUnchanged(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/prices" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if req.URL.RawQuery != "" {
			t.Fatalf("expected raw request without include fields, got %q", req.URL.RawQuery)
		}

		body := `{"data":[{"type":"subscriptionPrices","id":"price-raw"}],"links":{"next":""}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "list",
			"--subscription-id", "8000000001",
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
	if !strings.Contains(stdout, `"type":"subscriptionPrices"`) || strings.Contains(stdout, `"prices":[`) {
		t.Fatalf("expected raw output, got %q", stdout)
	}
}
