package cmdtest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPricingCurrentValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing app",
			args:    []string{"pricing", "current"},
			wantErr: "Error: --app is required (or set ASC_APP_ID)",
		},
		{
			name:    "territory and all territories are mutually exclusive",
			args:    []string{"pricing", "current", "--app", "app-1", "--territory", "USA", "--all-territories"},
			wantErr: "Error: --territory and --all-territories are mutually exclusive",
		},
		{
			name:    "invalid territory csv",
			args:    []string{"pricing", "current", "--app", "app-1", "--territory", ",,,"},
			wantErr: "Error: --territory must include at least one territory code",
		},
		{
			name:    "invalid territory value",
			args:    []string{"pricing", "current", "--app", "app-1", "--territory", "Atlantis"},
			wantErr: "Error: territory \"Atlantis\" could not be mapped to an App Store Connect territory ID",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}

func TestPricingCurrentBaseTerritoryJSONUsesDecodedPricePointMatch(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	baseManualID := mustEncodeAppPricingResource(t, map[string]any{
		"s":  "app-1",
		"t":  "USA",
		"p":  "10000",
		"sd": 0.0,
		"ed": 0.0,
	})
	basePricePointID := mustEncodeAppPricingResource(t, map[string]any{
		"s": "app-1",
		"t": "USA",
		"p": "10000",
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appPriceSchedule":
			body := `{"data":{"type":"appPriceSchedules","id":"schedule-1","attributes":{}}}`
			return pricingCurrentJSONResponse(body), nil

		case "/v1/appPriceSchedules/schedule-1/baseTerritory":
			body := `{"data":{"type":"territories","id":"USA","attributes":{"currency":"USD"}}}`
			return pricingCurrentJSONResponse(body), nil

		case "/v1/appPriceSchedules/schedule-1/manualPrices":
			query := req.URL.Query()
			if query.Get("include") != "appPricePoint,territory" {
				t.Fatalf("expected include=appPricePoint,territory, got %q", query.Get("include"))
			}
			if query.Get("fields[appPrices]") != "manual,startDate,endDate,appPricePoint,territory" {
				t.Fatalf("unexpected fields[appPrices]: %q", query.Get("fields[appPrices]"))
			}
			if query.Get("fields[appPricePoints]") != "customerPrice,proceeds,territory" {
				t.Fatalf("unexpected fields[appPricePoints]: %q", query.Get("fields[appPricePoints]"))
			}
			if query.Get("fields[territories]") != "currency" {
				t.Fatalf("unexpected fields[territories]: %q", query.Get("fields[territories]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("unexpected limit: %q", query.Get("limit"))
			}

			body := `{
				"data":[
					{
						"type":"appPrices",
						"id":"` + baseManualID + `",
						"attributes":{"startDate":"2024-01-01","manual":true}
					}
				],
				"included":[
					{
						"type":"appPricePoints",
						"id":"` + basePricePointID + `",
						"attributes":{"customerPrice":"0.00","proceeds":"0.00"}
					},
					{
						"type":"territories",
						"id":"USA",
						"attributes":{"currency":"USD"}
					}
				],
				"links":{"next":""}
			}`
			return pricingCurrentJSONResponse(body), nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"pricing", "current", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"appId":"app-1"`) {
		t.Fatalf("expected app id in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"baseTerritory":"USA"`) {
		t.Fatalf("expected base territory in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"customerPrice":"0.00"`) {
		t.Fatalf("expected customerPrice 0.00 in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"proceeds":"0.00"`) {
		t.Fatalf("expected proceeds 0.00 in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"currency":"USD"`) {
		t.Fatalf("expected currency USD in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"isFree":true`) {
		t.Fatalf("expected isFree=true in output, got %q", stdout)
	}
}

func TestPricingCurrentPaginationPreservesIncludeFields(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	futureManualID := mustEncodeAppPricingResource(t, map[string]any{
		"s":  "app-1",
		"t":  "USA",
		"p":  "future-price",
		"sd": 1893456000.0,
		"ed": 0.0,
	})
	currentManualID := mustEncodeAppPricingResource(t, map[string]any{
		"s":  "app-1",
		"t":  "USA",
		"p":  "current-price",
		"sd": 1704067200.0,
		"ed": 0.0,
	})
	currentPricePointID := mustEncodeAppPricingResource(t, map[string]any{
		"s": "app-1",
		"t": "USA",
		"p": "current-price",
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appPriceSchedule":
			return pricingCurrentJSONResponse(`{"data":{"type":"appPriceSchedules","id":"schedule-1","attributes":{}}}`), nil

		case "/v1/appPriceSchedules/schedule-1/baseTerritory":
			return pricingCurrentJSONResponse(`{"data":{"type":"territories","id":"USA","attributes":{"currency":"USD"}}}`), nil

		case "/v1/appPriceSchedules/schedule-1/manualPrices":
			query := req.URL.Query()
			if query.Get("include") != "appPricePoint,territory" {
				t.Fatalf("expected include on paginated request, got %q", query.Get("include"))
			}
			if query.Get("fields[appPrices]") != "manual,startDate,endDate,appPricePoint,territory" {
				t.Fatalf("unexpected fields[appPrices]: %q", query.Get("fields[appPrices]"))
			}
			if query.Get("fields[appPricePoints]") != "customerPrice,proceeds,territory" {
				t.Fatalf("unexpected fields[appPricePoints]: %q", query.Get("fields[appPricePoints]"))
			}
			if query.Get("fields[territories]") != "currency" {
				t.Fatalf("unexpected fields[territories]: %q", query.Get("fields[territories]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("unexpected limit: %q", query.Get("limit"))
			}

			if query.Get("cursor") == "" {
				body := `{
					"data":[
						{
							"type":"appPrices",
							"id":"` + futureManualID + `",
							"attributes":{"startDate":"2030-01-01","manual":true}
						}
					],
					"included":[],
					"links":{"next":"https://api.appstoreconnect.apple.com/v1/appPriceSchedules/schedule-1/manualPrices?cursor=abc"}
				}`
				return pricingCurrentJSONResponse(body), nil
			}

			if query.Get("cursor") != "abc" {
				t.Fatalf("expected cursor=abc, got %q", query.Get("cursor"))
			}

			body := `{
				"data":[
					{
						"type":"appPrices",
						"id":"` + currentManualID + `",
						"attributes":{"startDate":"2024-01-01","manual":true}
					}
				],
				"included":[
					{
						"type":"appPricePoints",
						"id":"` + currentPricePointID + `",
						"attributes":{"customerPrice":"2.99","proceeds":"2.09"}
					},
					{
						"type":"territories",
						"id":"USA",
						"attributes":{"currency":"USD"}
					}
				],
				"links":{"next":""}
			}`
			return pricingCurrentJSONResponse(body), nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"pricing", "current", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"customerPrice":"2.99"`) {
		t.Fatalf("expected paginated current customerPrice in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"currency":"USD"`) {
		t.Fatalf("expected USD currency in output, got %q", stdout)
	}
}

func TestPricingCurrentAllTerritoriesJSON(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	baseManualID := mustEncodeAppPricingResource(t, map[string]any{
		"s":  "app-1",
		"t":  "USA",
		"p":  "10001",
		"sd": 0.0,
		"ed": 0.0,
	})
	basePricePointID := mustEncodeAppPricingResource(t, map[string]any{
		"s": "app-1",
		"t": "USA",
		"p": "10001",
	})
	gbrAutomaticID := mustEncodeAppPricingResource(t, map[string]any{
		"s":  "app-1",
		"t":  "GBR",
		"p":  "10001",
		"sd": 0.0,
		"ed": 0.0,
	})
	gbrPricePointID := mustEncodeAppPricingResource(t, map[string]any{
		"s": "app-1",
		"t": "GBR",
		"p": "10001",
	})
	deuAutomaticID := mustEncodeAppPricingResource(t, map[string]any{
		"s":  "app-1",
		"t":  "DEU",
		"p":  "10001",
		"sd": 0.0,
		"ed": 0.0,
	})
	deuPricePointID := mustEncodeAppPricingResource(t, map[string]any{
		"s": "app-1",
		"t": "DEU",
		"p": "10001",
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appPriceSchedule":
			body := `{"data":{"type":"appPriceSchedules","id":"schedule-1","attributes":{}}}`
			return pricingCurrentJSONResponse(body), nil

		case "/v1/appPriceSchedules/schedule-1/baseTerritory":
			body := `{"data":{"type":"territories","id":"USA","attributes":{"currency":"USD"}}}`
			return pricingCurrentJSONResponse(body), nil

		case "/v1/appPriceSchedules/schedule-1/manualPrices":
			body := `{
				"data":[
					{
						"type":"appPrices",
						"id":"` + baseManualID + `",
						"attributes":{"startDate":"2024-01-01","manual":true}
					}
				],
				"included":[
					{"type":"appPricePoints","id":"` + basePricePointID + `","attributes":{"customerPrice":"1.99","proceeds":"1.39"}},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				],
				"links":{"next":""}
			}`
			return pricingCurrentJSONResponse(body), nil

		case "/v1/appPriceSchedules/schedule-1/automaticPrices":
			body := `{
				"data":[
					{
						"type":"appPrices",
						"id":"` + gbrAutomaticID + `",
						"attributes":{"startDate":"2024-01-01","manual":false}
					},
					{
						"type":"appPrices",
						"id":"` + deuAutomaticID + `",
						"attributes":{"startDate":"2024-01-01","manual":false}
					}
				],
				"included":[
					{"type":"appPricePoints","id":"` + gbrPricePointID + `","attributes":{"customerPrice":"1.79","proceeds":"1.25"}},
					{"type":"appPricePoints","id":"` + deuPricePointID + `","attributes":{"customerPrice":"2.29","proceeds":"1.60"}},
					{"type":"territories","id":"GBR","attributes":{"currency":"GBP"}},
					{"type":"territories","id":"DEU","attributes":{"currency":"EUR"}}
				],
				"links":{"next":""}
			}`
			return pricingCurrentJSONResponse(body), nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"pricing", "current", "--app", "app-1", "--all-territories"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, want := range []string{
		`"baseTerritory":"USA"`,
		`"territory":"USA","customerPrice":"1.99","proceeds":"1.39","currency":"USD"`,
		`"territory":"GBR","customerPrice":"1.79","proceeds":"1.25","currency":"GBP"`,
		`"territory":"DEU","customerPrice":"2.29","proceeds":"1.60","currency":"EUR"`,
		`"isFree":false`,
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q in output, got %q", want, stdout)
		}
	}
}

func TestPricingCurrentAllTerritoriesSkipsInactiveTerritories(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	baseManualID := mustEncodeAppPricingResource(t, map[string]any{
		"s":  "app-1",
		"t":  "USA",
		"p":  "10001",
		"sd": 0.0,
		"ed": 0.0,
	})
	basePricePointID := mustEncodeAppPricingResource(t, map[string]any{
		"s": "app-1",
		"t": "USA",
		"p": "10001",
	})
	futureGBRAutomaticID := mustEncodeAppPricingResource(t, map[string]any{
		"s":  "app-1",
		"t":  "GBR",
		"p":  "10001",
		"sd": 4102444800.0,
		"ed": 0.0,
	})
	futureGBRPricePointID := mustEncodeAppPricingResource(t, map[string]any{
		"s": "app-1",
		"t": "GBR",
		"p": "10001",
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appPriceSchedule":
			body := `{"data":{"type":"appPriceSchedules","id":"schedule-1","attributes":{}}}`
			return pricingCurrentJSONResponse(body), nil

		case "/v1/appPriceSchedules/schedule-1/baseTerritory":
			body := `{"data":{"type":"territories","id":"USA","attributes":{"currency":"USD"}}}`
			return pricingCurrentJSONResponse(body), nil

		case "/v1/appPriceSchedules/schedule-1/manualPrices":
			body := `{
				"data":[
					{
						"type":"appPrices",
						"id":"` + baseManualID + `",
						"attributes":{"startDate":"2024-01-01","manual":true}
					}
				],
				"included":[
					{"type":"appPricePoints","id":"` + basePricePointID + `","attributes":{"customerPrice":"1.99","proceeds":"1.39"}},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				],
				"links":{"next":""}
			}`
			return pricingCurrentJSONResponse(body), nil

		case "/v1/appPriceSchedules/schedule-1/automaticPrices":
			body := `{
				"data":[
					{
						"type":"appPrices",
						"id":"` + futureGBRAutomaticID + `",
						"attributes":{"startDate":"2100-01-01","manual":false}
					}
				],
				"included":[
					{"type":"appPricePoints","id":"` + futureGBRPricePointID + `","attributes":{"customerPrice":"1.79","proceeds":"1.25"}},
					{"type":"territories","id":"GBR","attributes":{"currency":"GBP"}}
				],
				"links":{"next":""}
			}`
			return pricingCurrentJSONResponse(body), nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"pricing", "current", "--app", "app-1", "--all-territories"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"territory":"USA","customerPrice":"1.99","proceeds":"1.39","currency":"USD"`) {
		t.Fatalf("expected USA current price in output, got %q", stdout)
	}
	if strings.Contains(stdout, `"territory":"GBR"`) {
		t.Fatalf("did not expect future-only territory in output, got %q", stdout)
	}
}

func TestPricingCurrentTerritoryFilterTableOutput(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	baseManualID := mustEncodeAppPricingResource(t, map[string]any{
		"s":  "app-1",
		"t":  "USA",
		"p":  "10001",
		"sd": 0.0,
		"ed": 0.0,
	})
	basePricePointID := mustEncodeAppPricingResource(t, map[string]any{
		"s": "app-1",
		"t": "USA",
		"p": "10001",
	})
	gbrAutomaticID := mustEncodeAppPricingResource(t, map[string]any{
		"s":  "app-1",
		"t":  "GBR",
		"p":  "10001",
		"sd": 0.0,
		"ed": 0.0,
	})
	gbrPricePointID := mustEncodeAppPricingResource(t, map[string]any{
		"s": "app-1",
		"t": "GBR",
		"p": "10001",
	})
	deuAutomaticID := mustEncodeAppPricingResource(t, map[string]any{
		"s":  "app-1",
		"t":  "DEU",
		"p":  "10001",
		"sd": 0.0,
		"ed": 0.0,
	})
	deuPricePointID := mustEncodeAppPricingResource(t, map[string]any{
		"s": "app-1",
		"t": "DEU",
		"p": "10001",
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/appPriceSchedule":
			body := `{"data":{"type":"appPriceSchedules","id":"schedule-1","attributes":{}}}`
			return pricingCurrentJSONResponse(body), nil

		case "/v1/appPriceSchedules/schedule-1/baseTerritory":
			body := `{"data":{"type":"territories","id":"USA","attributes":{"currency":"USD"}}}`
			return pricingCurrentJSONResponse(body), nil

		case "/v1/appPriceSchedules/schedule-1/manualPrices":
			body := `{
				"data":[
					{"type":"appPrices","id":"` + baseManualID + `","attributes":{"startDate":"2024-01-01","manual":true}}
				],
				"included":[
					{"type":"appPricePoints","id":"` + basePricePointID + `","attributes":{"customerPrice":"1.99","proceeds":"1.39"}},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				],
				"links":{"next":""}
			}`
			return pricingCurrentJSONResponse(body), nil

		case "/v1/appPriceSchedules/schedule-1/automaticPrices":
			body := `{
				"data":[
					{"type":"appPrices","id":"` + gbrAutomaticID + `","attributes":{"startDate":"2024-01-01","manual":false}},
					{"type":"appPrices","id":"` + deuAutomaticID + `","attributes":{"startDate":"2024-01-01","manual":false}}
				],
				"included":[
					{"type":"appPricePoints","id":"` + gbrPricePointID + `","attributes":{"customerPrice":"1.79","proceeds":"1.25"}},
					{"type":"appPricePoints","id":"` + deuPricePointID + `","attributes":{"customerPrice":"2.29","proceeds":"1.60"}},
					{"type":"territories","id":"GBR","attributes":{"currency":"GBP"}},
					{"type":"territories","id":"DEU","attributes":{"currency":"EUR"}}
				],
				"links":{"next":""}
			}`
			return pricingCurrentJSONResponse(body), nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"pricing", "current", "--app", "app-1", "--territory", "US,United Kingdom", "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, want := range []string{"Territory", "USA", "GBR", "USD", "GBP", "1.99", "1.79", "1.39", "1.25"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q in table output, got %q", want, stdout)
		}
	}
	if strings.Contains(stdout, "DEU") {
		t.Fatalf("did not expect DEU in filtered table output, got %q", stdout)
	}
}

func mustEncodeAppPricingResource(t *testing.T, payload map[string]any) string {
	t.Helper()

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	return base64.RawURLEncoding.EncodeToString(data)
}

func pricingCurrentJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}
