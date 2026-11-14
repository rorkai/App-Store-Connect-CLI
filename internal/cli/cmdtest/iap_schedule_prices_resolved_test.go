package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestIAPPricingSchedulesManualPricesResolvedJSON(t *testing.T) {
	setupAuth(t)

	const secondURL = "https://api.appstoreconnect.apple.com/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices?cursor=Mg"

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices", "fields[inAppPurchasePricePoints]=customerPrice,proceeds,territory", "fields[inAppPurchasePrices]=manual,startDate,endDate,inAppPurchasePricePoint,territory", "fields[territories]=currency", "include=inAppPurchasePricePoint,territory", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{
					"type":"inAppPurchasePrices",
					"id":"manual-price-usa",
					"attributes":{"startDate":"2025-01-01","manual":true},
					"relationships":{
						"territory":{"data":{"type":"territories","id":"USA"}},
						"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-manual-usa"}}
					}
				}
			],
			"included":[
				{"type":"inAppPurchasePricePoints","id":"pp-manual-usa","attributes":{"customerPrice":"9.99","proceeds":"8.49"}},
				{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
			],
			"links":{"next":"`+secondURL+`"}
		}`))
	reg.Register(asctest.GET("/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices", "cursor=Mg", "fields[inAppPurchasePricePoints]=customerPrice,proceeds,territory", "fields[inAppPurchasePrices]=manual,startDate,endDate,inAppPurchasePricePoint,territory", "fields[territories]=currency", "include=inAppPurchasePricePoint,territory", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{
					"type":"inAppPurchasePrices",
					"id":"manual-price-gbr",
					"attributes":{"startDate":"2024-06-01","manual":true},
					"relationships":{
						"territory":{"data":{"type":"territories","id":"GBR"}},
						"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-manual-gbr"}}
					}
				}
			],
			"included":[
				{"type":"inAppPurchasePricePoints","id":"pp-manual-gbr","attributes":{"customerPrice":"7.99","proceeds":"6.79"}},
				{"type":"territories","id":"GBR","attributes":{"currency":"GBP"}}
			],
			"links":{"next":""}
		}`))
	reg.Register(asctest.GET("/v1/inAppPurchasePriceSchedules/schedule-1/automaticPrices", "fields[inAppPurchasePricePoints]=customerPrice,proceeds,territory", "fields[inAppPurchasePrices]=manual,startDate,endDate,inAppPurchasePricePoint,territory", "fields[territories]=currency", "include=inAppPurchasePricePoint,territory", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{
					"type":"inAppPurchasePrices",
					"id":"automatic-price-usa",
					"attributes":{"startDate":"2025-01-01","manual":false},
					"relationships":{
						"territory":{"data":{"type":"territories","id":"USA"}},
						"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-automatic-usa"}}
					}
				}
			],
			"included":[
				{"type":"inAppPurchasePricePoints","id":"pp-automatic-usa","attributes":{"customerPrice":"4.99","proceeds":"3.49"}},
				{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
			],
			"links":{"next":""}
		}`))

	stdout, stderr, err := runCommand(t, []string{
		"iap", "pricing", "schedules", "manual-prices",
		"--schedule-id", "schedule-1",
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
	if len(result.Prices) != 2 {
		t.Fatalf("expected 2 resolved prices, got %+v", result.Prices)
	}
	if result.Prices[0].Territory != "GBR" || result.Prices[0].CustomerPrice != "7.99" {
		t.Fatalf("unexpected GBR row: %+v", result.Prices[0])
	}
	if result.Prices[1].Territory != "USA" || result.Prices[1].CustomerPrice != "9.99" {
		t.Fatalf("unexpected USA row: %+v", result.Prices[1])
	}
	if result.Prices[1].Manual == nil || !*result.Prices[1].Manual {
		t.Fatalf("expected manual USA row, got %+v", result.Prices[1].Manual)
	}
}

func TestIAPPricingSchedulesAutomaticPricesResolvedTable(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/inAppPurchasePriceSchedules/schedule-1/automaticPrices", "fields[inAppPurchasePricePoints]=customerPrice,proceeds,territory", "fields[inAppPurchasePrices]=manual,startDate,endDate,inAppPurchasePricePoint,territory", "fields[territories]=currency", "include=inAppPurchasePricePoint,territory", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{
					"type":"inAppPurchasePrices",
					"id":"automatic-price-usa",
					"attributes":{"startDate":"2025-01-01","manual":false},
					"relationships":{
						"territory":{"data":{"type":"territories","id":"USA"}},
						"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-automatic-usa"}}
					}
				}
			],
			"included":[
				{"type":"inAppPurchasePricePoints","id":"pp-automatic-usa","attributes":{"customerPrice":"4.99","proceeds":"3.49"}},
				{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
			],
			"links":{"next":""}
		}`))
	reg.Register(asctest.GET("/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices", "fields[inAppPurchasePricePoints]=customerPrice,proceeds,territory", "fields[inAppPurchasePrices]=manual,startDate,endDate,inAppPurchasePricePoint,territory", "fields[territories]=currency", "include=inAppPurchasePricePoint,territory", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{
					"type":"inAppPurchasePrices",
					"id":"manual-price-usa",
					"attributes":{"startDate":"2025-01-01","manual":true},
					"relationships":{
						"territory":{"data":{"type":"territories","id":"USA"}},
						"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-manual-usa"}}
					}
				}
			],
			"included":[
				{"type":"inAppPurchasePricePoints","id":"pp-manual-usa","attributes":{"customerPrice":"9.99","proceeds":"8.49"}},
				{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
			],
			"links":{"next":""}
		}`))

	stdout, stderr, err := runCommand(t, []string{
		"iap", "pricing", "schedules", "automatic-prices",
		"--schedule-id", "schedule-1",
		"--resolved",
		"--output", "table",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Manual") || !strings.Contains(stdout, "Proceeds") {
		t.Fatalf("expected resolved table headers, got %q", stdout)
	}
	if !strings.Contains(stdout, "USA") || !strings.Contains(stdout, "9.99") || !strings.Contains(stdout, "true") {
		t.Fatalf("expected resolved table row, got %q", stdout)
	}
}

func TestIAPPricingSchedulesManualPricesResolvedRejectsNext(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	const nextURL = "https://api.appstoreconnect.apple.com/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices?cursor=Mg"

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "pricing", "schedules", "manual-prices",
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

func TestIAPPricingSchedulesManualPricesRawOutputUnchanged(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"inAppPurchasePrices","id":"price-raw"}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{
		"iap", "pricing", "schedules", "manual-prices",
		"--schedule-id", "schedule-1",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"type":"inAppPurchasePrices"`) || strings.Contains(stdout, `"prices":[`) {
		t.Fatalf("expected raw output, got %q", stdout)
	}
}
