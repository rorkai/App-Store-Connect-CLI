package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestSubscriptionsPricingPricesListResolvedUsesFreshDeadlinePerPage(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_TIMEOUT", "100ms")
	t.Setenv("ASC_MAX_RETRIES", "0")

	requestCount := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if requestCount == 1 {
			time.Sleep(60 * time.Millisecond)
			return jsonResponse(http.StatusOK, `{"data":[],"links":{"next":"/v1/subscriptions/8000000001/prices?cursor=next"}}`)
		}
		deadline, ok := req.Context().Deadline()
		if !ok || time.Until(deadline) < 70*time.Millisecond {
			t.Errorf("expected fresh second-page deadline, remaining=%s", time.Until(deadline))
			return nil, errors.New("test transport assertion failed")
		}
		return jsonResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`)
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
		var runErr error
		synctest.Test(t, func(*testing.T) { runErr = root.Run(context.Background()) })
		if err := runErr; err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 2 {
		t.Fatalf("expected two page requests, got %d; stdout=%q", requestCount, stdout)
	}
}

func TestSubscriptionsPricingPricesListResolvedJSON(t *testing.T) {
	setupAuth(t)

	const secondURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/8000000001/prices?cursor=Mg"

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/subscriptions/8000000001/prices", "fields[subscriptionPricePoints]=customerPrice,proceeds,proceedsYear2", "fields[subscriptionPrices]=startDate,preserved,planType,territory,subscriptionPricePoint", "fields[territories]=currency", "include=subscriptionPricePoint,territory", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{
					"type":"subscriptionPrices",
					"id":"price-current-usa",
					"attributes":{"startDate":"2025-01-01","preserved":false,"planType":"UPFRONT"},
					"relationships":{
						"territory":{"data":{"type":"territories","id":"USA"}},
						"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-current-usa"}}
					}
				},
				{
					"type":"subscriptionPrices",
					"id":"price-future-usa",
					"attributes":{"startDate":"2030-01-01","preserved":false,"planType":"UPFRONT"},
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
			"links":{"next":"`+secondURL+`"}
		}`))
	reg.Register(asctest.GET("/v1/subscriptions/8000000001/prices", "cursor=Mg", "fields[subscriptionPricePoints]=customerPrice,proceeds,proceedsYear2", "fields[subscriptionPrices]=startDate,preserved,planType,territory,subscriptionPricePoint", "fields[territories]=currency", "include=subscriptionPricePoint,territory", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{
					"type":"subscriptionPrices",
					"id":"price-current-gbr",
					"attributes":{"startDate":"2024-06-01","preserved":false,"planType":"UPFRONT"},
					"relationships":{
						"territory":{"data":{"type":"territories","id":"GBR"}},
						"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-current-gbr"}}
					}
				},
				{
					"type":"subscriptionPrices",
					"id":"price-undated-fra",
					"attributes":{"preserved":false,"planType":"UPFRONT"},
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
		}`))

	stdout, stderr, err := runCommand(t, []string{
		"subscriptions", "pricing", "prices", "list",
		"--subscription-id", "8000000001",
		"--resolved",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result shared.ResolvedPricesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal() error = %v, stdout = %q", err, stdout)
	}
	if len(result.Prices) != 3 {
		t.Fatalf("expected 3 resolved prices, got %+v", result.Prices)
	}
	if result.Prices[0].Territory != "FRA" || result.Prices[0].CustomerPrice != "6.99" || result.Prices[0].PlanType != "UPFRONT" {
		t.Fatalf("unexpected first resolved row: %+v", result.Prices[0])
	}
	if result.Prices[1].Territory != "GBR" || result.Prices[1].CustomerPrice != "7.99" || result.Prices[1].PlanType != "UPFRONT" {
		t.Fatalf("unexpected second resolved row: %+v", result.Prices[1])
	}
	if result.Prices[2].Territory != "USA" || result.Prices[2].CustomerPrice != "9.99" || result.Prices[2].PlanType != "UPFRONT" {
		t.Fatalf("unexpected third resolved row: %+v", result.Prices[2])
	}
}

func TestSubscriptionsPricingPricesListResolvedPlanTypeIncludesUndatedCurrentPrices(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/subscriptions/8000000001/prices", "fields[subscriptionPricePoints]=customerPrice,proceeds,proceedsYear2", "fields[subscriptionPrices]=startDate,preserved,planType,territory,subscriptionPricePoint", "fields[territories]=currency", "filter[planType]=MONTHLY", "include=subscriptionPricePoint,territory", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{
					"type":"subscriptionPrices",
					"id":"price-usa",
					"attributes":{"preserved":false},
					"relationships":{
						"territory":{"data":{"type":"territories","id":"USA"}},
						"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-usa"}}
					}
				},
				{
					"type":"subscriptionPrices",
					"id":"price-nor",
					"attributes":{"preserved":false},
					"relationships":{
						"territory":{"data":{"type":"territories","id":"NOR"}},
						"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-nor"}}
					}
				}
			],
			"included":[
				{"type":"subscriptionPricePoints","id":"pp-usa","attributes":{"customerPrice":"4.99","proceeds":"3.49"}},
				{"type":"subscriptionPricePoints","id":"pp-nor","attributes":{"customerPrice":"59.00","proceeds":"41.00"}},
				{"type":"territories","id":"USA","attributes":{"currency":"USD"}},
				{"type":"territories","id":"NOR","attributes":{"currency":"NOK"}}
			],
			"links":{"next":""}
		}`))

	stdout, stderr, err := runCommand(t, []string{
		"subscriptions", "pricing", "prices", "list",
		"--subscription-id", "8000000001",
		"--resolved",
		"--plan-type", "MONTHLY",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result shared.ResolvedPricesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal() error = %v, stdout = %q", err, stdout)
	}
	if len(result.Prices) != 2 {
		t.Fatalf("expected undated resolved prices, got %+v", result.Prices)
	}
	if result.Prices[0].Territory != "NOR" || result.Prices[0].CustomerPrice != "59.00" || result.Prices[0].Currency != "NOK" {
		t.Fatalf("unexpected first resolved row: %+v", result.Prices[0])
	}
	if result.Prices[1].Territory != "USA" || result.Prices[1].CustomerPrice != "4.99" || result.Prices[1].Currency != "USD" {
		t.Fatalf("unexpected second resolved row: %+v", result.Prices[1])
	}
}

func TestSubscriptionsPricingPricesListResolvedTerritoryFilter(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/subscriptions/8000000001/prices", "fields[subscriptionPricePoints]=customerPrice,proceeds,proceedsYear2", "fields[subscriptionPrices]=startDate,preserved,planType,territory,subscriptionPricePoint", "fields[territories]=currency", "filter[territory]=USA,GBR", "include=subscriptionPricePoint,territory", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[{
				"type":"subscriptionPrices",
				"id":"price-usa",
				"attributes":{"startDate":"2026-01-01"},
				"relationships":{
					"territory":{"data":{"type":"territories","id":"USA"}},
					"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-usa"}}
				}
			}],
			"included":[
				{"type":"subscriptionPricePoints","id":"pp-usa","attributes":{"customerPrice":"4.99"}},
				{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
			],
			"links":{"next":""}
		}`))

	stdout, stderr, err := runCommand(t, []string{
		"subscriptions", "pricing", "prices", "list",
		"--subscription-id", "8000000001",
		"--resolved",
		"--territory", "US,GBR",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var result shared.ResolvedPricesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal() error = %v, stdout = %q", err, stdout)
	}
	if len(result.Prices) != 1 || result.Prices[0].Territory != "USA" {
		t.Fatalf("expected one USA resolved price, got %+v", result.Prices)
	}
}

func TestSubscriptionsPricingPricesListResolvedPlanTypeSupportsRelativeNextURL(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/subscriptions/8000000001/prices", "fields[subscriptionPricePoints]=customerPrice,proceeds,proceedsYear2", "fields[subscriptionPrices]=startDate,preserved,planType,territory,subscriptionPricePoint", "fields[territories]=currency", "filter[planType]=MONTHLY", "include=subscriptionPricePoint,territory", "limit=200"),
		asctest.JSON(http.StatusOK, `{
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
		}`))
	reg.Register(asctest.GET("/v1/subscriptions/8000000001/prices", "cursor=next", "fields[subscriptionPricePoints]=customerPrice,proceeds,proceedsYear2", "fields[subscriptionPrices]=startDate,preserved,planType,territory,subscriptionPricePoint", "fields[territories]=currency", "filter[planType]=MONTHLY", "include=subscriptionPricePoint,territory", "limit=200"),
		asctest.JSON(http.StatusOK, `{
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
		}`))

	stdout, stderr, runErr := runCommand(t, []string{
		"subscriptions", "pricing", "prices", "list",
		"--subscription-id", "8000000001",
		"--resolved",
		"--plan-type", "MONTHLY",
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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/subscriptions/8000000001/prices", "fields[subscriptionPricePoints]=customerPrice,proceeds,proceedsYear2", "fields[subscriptionPrices]=startDate,preserved,planType,territory,subscriptionPricePoint", "fields[territories]=currency", "include=subscriptionPricePoint,territory", "limit=200"),
		asctest.JSON(http.StatusOK, `{
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
		}`))

	stdout, stderr, err := runCommand(t, []string{
		"subscriptions", "pricing", "prices", "list",
		"--subscription-id", "8000000001",
		"--resolved",
		"--output", "table",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/subscriptions/8000000001/prices"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"subscriptionPrices","id":"price-raw"}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{
		"subscriptions", "pricing", "prices", "list",
		"--subscription-id", "8000000001",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"type":"subscriptionPrices"`) || strings.Contains(stdout, `"prices":[`) {
		t.Fatalf("expected raw output, got %q", stdout)
	}
}
