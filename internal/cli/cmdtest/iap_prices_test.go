package cmdtest

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestIAPPricesValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing app and iap-id",
			args:    []string{"iap", "pricing", "summary"},
			wantErr: "Error: --app or --iap-id is required",
		},
		{
			name:    "app and iap-id both set",
			args:    []string{"iap", "pricing", "summary", "--app", "APP_ID", "--iap-id", "IAP_ID"},
			wantErr: "Error: --app and --iap-id are mutually exclusive",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestIAPPricesByIDSuccess(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v2/inAppPurchases/iap-1":
			body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Lifetime Unlock","productId":"com.example.lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v2/inAppPurchases/iap-1/iapPriceSchedule":
			query := req.URL.Query()
			if query.Get("include") != "baseTerritory,manualPrices,automaticPrices" {
				t.Fatalf("unexpected include query: %q", query.Get("include"))
			}
			if query.Get("fields[inAppPurchasePrices]") != "startDate,endDate,manual,inAppPurchasePricePoint,territory" {
				t.Fatalf("unexpected price fields: %q", query.Get("fields[inAppPurchasePrices]"))
			}
			if query.Get("fields[territories]") != "currency" {
				t.Fatalf("unexpected territory fields: %q", query.Get("fields[territories]"))
			}
			if query.Get("limit[manualPrices]") != "50" {
				t.Fatalf("unexpected manual prices limit: %q", query.Get("limit[manualPrices]"))
			}
			if query.Get("limit[automaticPrices]") != "50" {
				t.Fatalf("unexpected automatic prices limit: %q", query.Get("limit[automaticPrices]"))
			}

			body := `{
				"data":{
					"type":"inAppPurchasePriceSchedules",
					"id":"schedule-1",
					"relationships":{
						"baseTerritory":{"data":{"type":"territories","id":"USA"}}
					}
				},
				"included":[
					{
						"type":"inAppPurchasePrices",
						"id":"iap-price-1",
						"attributes":{"startDate":"2024-01-01","manual":true},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"USA"}},
							"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-1"}}
						}
					},
					{
						"type":"inAppPurchasePrices",
						"id":"iap-price-2",
						"attributes":{"startDate":"2030-01-01","manual":true},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"USA"}},
							"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-2"}}
						}
					},
					{
						"type":"territories",
						"id":"USA",
						"attributes":{"currency":"USD"}
					}
				]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v2/inAppPurchases/iap-1/pricePoints":
			query := req.URL.Query()
			if query.Get("filter[territory]") != "USA" {
				t.Fatalf("unexpected territory filter: %q", query.Get("filter[territory]"))
			}
			if query.Get("include") != "territory" {
				t.Fatalf("unexpected include query: %q", query.Get("include"))
			}
			if query.Get("fields[inAppPurchasePricePoints]") != "customerPrice,proceeds,territory" {
				t.Fatalf("unexpected fields[inAppPurchasePricePoints]: %q", query.Get("fields[inAppPurchasePricePoints]"))
			}
			body := `{
				"data":[
					{"type":"inAppPurchasePricePoints","id":"pp-1","attributes":{"customerPrice":"9.99","proceeds":"8.49"}},
					{"type":"inAppPurchasePricePoints","id":"pp-2","attributes":{"customerPrice":"12.99","proceeds":"11.04"}}
				],
				"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"iap", "pricing", "summary", "--iap-id", "iap-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"iap-1"`) {
		t.Fatalf("expected iap id in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"currentPrice":{"amount":"9.99","currency":"USD"}`) {
		t.Fatalf("expected current price in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"scheduledChanges":[{"territory":"USA","fromPricePoint":"pp-1","toPricePoint":"pp-2","effectiveDate":"2030-01-01"}]`) {
		t.Fatalf("expected scheduled change in output, got %q", stdout)
	}
}

func TestIAPPricesTableOutput(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v2/inAppPurchases/iap-1":
			body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Lifetime Unlock","productId":"com.example.lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v2/inAppPurchases/iap-1/iapPriceSchedule":
			body := `{
				"data":{
					"type":"inAppPurchasePriceSchedules",
					"id":"schedule-1",
					"relationships":{"baseTerritory":{"data":{"type":"territories","id":"USA"}}}
				},
				"included":[
					{
						"type":"inAppPurchasePrices",
						"id":"iap-price-1",
						"attributes":{"startDate":"2024-01-01","manual":true},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"USA"}},
							"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-1"}}
						}
					},
					{
						"type":"territories",
						"id":"USA",
						"attributes":{"currency":"USD"}
					}
				]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v2/inAppPurchases/iap-1/pricePoints":
			body := `{
				"data":[{"type":"inAppPurchasePricePoints","id":"pp-1","attributes":{"customerPrice":"9.99","proceeds":"8.49"}}],
				"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"iap", "pricing", "summary", "--iap-id", "iap-1", "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Current Price") {
		t.Fatalf("expected table headers in output, got %q", stdout)
	}
	if !strings.Contains(stdout, "9.99 USD") {
		t.Fatalf("expected formatted current price in output, got %q", stdout)
	}
}

func TestIAPPricesFetchesAllScheduleEntriesWhenIncludedHitsLimit(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	scheduleBody := buildIAPPriceScheduleWithAutomaticIncludedCount(50)
	manualSubresourceCalls := 0
	automaticSubresourceCalls := 0

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v2/inAppPurchases/iap-1":
			body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Lifetime Unlock","productId":"com.example.lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v2/inAppPurchases/iap-1/iapPriceSchedule":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(scheduleBody)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices":
			manualSubresourceCalls++
			body := `{
				"data":[
					{
						"type":"inAppPurchasePrices",
						"id":"manual-current",
						"attributes":{"startDate":"2024-01-01","manual":true},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"USA"}},
							"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-current"}}
						}
					}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/inAppPurchasePriceSchedules/schedule-1/automaticPrices":
			automaticSubresourceCalls++
			body := `{
				"data":[
					{
						"type":"inAppPurchasePrices",
						"id":"mus-old",
						"attributes":{"startDate":"2024-01-01","endDate":"2098-12-31"},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"MUS"}},
							"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-mus-old"}}
						}
					},
					{
						"type":"inAppPurchasePrices",
						"id":"mus-new",
						"attributes":{"startDate":"2099-01-01"},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"MUS"}},
							"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-mus-new"}}
						}
					}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v2/inAppPurchases/iap-1/pricePoints":
			body := `{
				"data":[
					{"type":"inAppPurchasePricePoints","id":"pp-current","attributes":{"customerPrice":"9.99","proceeds":"8.49"}}
				],
				"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"iap", "pricing", "summary", "--iap-id", "iap-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if manualSubresourceCalls == 0 {
		t.Fatalf("expected manualPrices subresource to be requested when included hits limit")
	}
	if automaticSubresourceCalls == 0 {
		t.Fatalf("expected automaticPrices subresource to be requested when included hits limit")
	}
	if !strings.Contains(stdout, `"scheduledChanges":[{"territory":"MUS","fromPricePoint":"pp-mus-old","toPricePoint":"pp-mus-new","effectiveDate":"2099-01-01"}]`) {
		t.Fatalf("expected MUS scheduled change in output, got %q", stdout)
	}
}

func TestIAPPricesResolvesLegacyManualPricePointValues(t *testing.T) {
	setupAuth(t)

	const legacyPriceResourceID = "eyJzIjoiMTU1OTI5NDEzOSIsInQiOiJVU0EiLCJwIjoiMyIsInNkIjowLjAsImVkIjowLjB9"
	const legacyPricePointResourceID = "eyJzIjoiMTU1OTI5NDEzOSIsInQiOiJVU0EiLCJwIjoiMyJ9"
	const canonicalUSAResourceID = "eyJzIjoiMTU1OTI5NDEzOSIsInQiOiJVU0EiLCJwIjoiMTAwMzYifQ"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	manualFallbackCalls := 0

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v2/inAppPurchases/iap-legacy":
			body := `{"data":{"type":"inAppPurchases","id":"iap-legacy","attributes":{"name":"Legacy Tip","productId":"com.example.legacy.tip","inAppPurchaseType":"CONSUMABLE"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v2/inAppPurchases/iap-legacy/iapPriceSchedule":
			body := `{
				"data":{
					"type":"inAppPurchasePriceSchedules",
					"id":"schedule-legacy",
					"relationships":{"baseTerritory":{"data":{"type":"territories","id":"USA"}}}
				},
				"included":[
					{
						"type":"inAppPurchasePrices",
						"id":"` + legacyPriceResourceID + `",
						"attributes":{"manual":true}
					},
					{
						"type":"territories",
						"id":"USA",
						"attributes":{"currency":"USD"}
					}
				]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v2/inAppPurchases/iap-legacy/pricePoints":
			body := `{
				"data":[
					{
						"type":"inAppPurchasePricePoints",
						"id":"` + canonicalUSAResourceID + `",
						"attributes":{"customerPrice":"2.99","proceeds":"2.54"}
					}
				],
				"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/inAppPurchasePriceSchedules/schedule-legacy/manualPrices":
			manualFallbackCalls++
			query := req.URL.Query()
			if query.Get("include") != "inAppPurchasePricePoint,territory" {
				t.Fatalf("unexpected include query: %q", query.Get("include"))
			}
			if query.Get("fields[inAppPurchasePrices]") != "manual,inAppPurchasePricePoint,territory" {
				t.Fatalf("unexpected fields[inAppPurchasePrices]: %q", query.Get("fields[inAppPurchasePrices]"))
			}
			if query.Get("fields[inAppPurchasePricePoints]") != "customerPrice,proceeds,territory" {
				t.Fatalf("unexpected fields[inAppPurchasePricePoints]: %q", query.Get("fields[inAppPurchasePricePoints]"))
			}
			if query.Get("fields[territories]") != "currency" {
				t.Fatalf("unexpected fields[territories]: %q", query.Get("fields[territories]"))
			}
			body := `{
				"data":[
					{
						"type":"inAppPurchasePrices",
						"id":"` + legacyPriceResourceID + `",
						"attributes":{"manual":true},
						"relationships":{
							"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"` + legacyPricePointResourceID + `"}},
							"territory":{"data":{"type":"territories","id":"USA"}}
						}
					}
				],
				"included":[
					{
						"type":"inAppPurchasePricePoints",
						"id":"` + legacyPricePointResourceID + `",
						"attributes":{"customerPrice":"2.99","proceeds":"2.54"}
					},
					{
						"type":"territories",
						"id":"USA",
						"attributes":{"currency":"USD"}
					}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"iap", "pricing", "summary", "--iap-id", "iap-legacy"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if manualFallbackCalls == 0 {
		t.Fatalf("expected manual schedule fallback endpoint to be called")
	}
	if !strings.Contains(stdout, `"currentPrice":{"amount":"2.99","currency":"USD"}`) {
		t.Fatalf("expected current price from manual fallback, got %q", stdout)
	}
	if !strings.Contains(stdout, `"estimatedProceeds":{"amount":"2.54","currency":"USD"}`) {
		t.Fatalf("expected proceeds from manual fallback, got %q", stdout)
	}
}

func buildIAPPriceScheduleWithAutomaticIncludedCount(automaticCount int) string {
	included := make([]string, 0, automaticCount+2)
	for i := range automaticCount {
		included = append(included, fmt.Sprintf(`{
			"type":"inAppPurchasePrices",
			"id":"auto-%d",
			"attributes":{"startDate":"2024-01-01"},
			"relationships":{
				"territory":{"data":{"type":"territories","id":"USA"}},
				"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-auto-%d"}}
			}
		}`, i, i))
	}

	included = append(included, `{
		"type":"inAppPurchasePrices",
		"id":"manual-current",
		"attributes":{"startDate":"2024-01-01","manual":true},
		"relationships":{
			"territory":{"data":{"type":"territories","id":"USA"}},
			"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-current"}}
		}
	}`)
	included = append(included, `{
		"type":"territories",
		"id":"USA",
		"attributes":{"currency":"USD"}
	}`)

	return fmt.Sprintf(`{
		"data":{
			"type":"inAppPurchasePriceSchedules",
			"id":"schedule-1",
			"relationships":{"baseTerritory":{"data":{"type":"territories","id":"USA"}}}
		},
		"included":[%s]
	}`, strings.Join(included, ","))
}
