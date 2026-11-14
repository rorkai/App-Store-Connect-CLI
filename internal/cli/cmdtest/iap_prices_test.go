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

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000001"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"inAppPurchases","id":"9000000001","attributes":{"name":"Lifetime Unlock","productId":"com.example.lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`))
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000001/iapPriceSchedule", "fields[inAppPurchasePriceSchedules]=baseTerritory,manualPrices,automaticPrices", "fields[inAppPurchasePrices]=startDate,endDate,manual,inAppPurchasePricePoint,territory", "fields[territories]=currency", "include=baseTerritory,manualPrices,automaticPrices", "limit[automaticPrices]=50", "limit[manualPrices]=50"),
		asctest.JSON(http.StatusOK, `{
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
		}`))
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000001/pricePoints", "fields[inAppPurchasePricePoints]=customerPrice,proceeds,territory", "fields[territories]=currency", "filter[territory]=USA", "include=territory", "limit=8000"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{"type":"inAppPurchasePricePoints","id":"pp-1","attributes":{"customerPrice":"9.99","proceeds":"8.49"}},
				{"type":"inAppPurchasePricePoints","id":"pp-2","attributes":{"customerPrice":"12.99","proceeds":"11.04"}}
			],
			"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],
			"links":{"next":""}
		}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"iap", "pricing", "summary", "--iap-id", "9000000001"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"9000000001"`) {
		t.Fatalf("expected iap id in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"currentPrice":{"amount":"9.99","currency":"USD"}`) {
		t.Fatalf("expected current price in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"scheduledChanges":[{"territory":"USA","fromPricePoint":"pp-1","toPricePoint":"pp-2","effectiveDate":"2030-01-01"}]`) {
		t.Fatalf("expected scheduled change in output, got %q", stdout)
	}
}

func TestIAPPricesByIDNormalizesTerritory(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000001"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"inAppPurchases","id":"9000000001","attributes":{"name":"Lifetime Unlock","productId":"com.example.lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`))
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000001/iapPriceSchedule", "fields[inAppPurchasePriceSchedules]=baseTerritory,manualPrices,automaticPrices", "fields[inAppPurchasePrices]=startDate,endDate,manual,inAppPurchasePricePoint,territory", "fields[territories]=currency", "include=baseTerritory,manualPrices,automaticPrices", "limit[automaticPrices]=50", "limit[manualPrices]=50"),
		asctest.JSON(http.StatusOK, `{
			"data":{"type":"inAppPurchasePriceSchedules","id":"schedule-1","relationships":{"baseTerritory":{"data":{"type":"territories","id":"USA"}}}},
			"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}]
		}`))
	reg.Register(asctest.GET("/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{
					"type":"inAppPurchasePrices",
					"id":"iap-price-1",
					"attributes":{"startDate":"2024-01-01","manual":true},
					"relationships":{
						"territory":{"data":{"type":"territories","id":"USA"}},
						"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-1"}}
					}
				}
			],
			"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],
			"links":{"next":""}
		}`))
	reg.Register(asctest.GET("/v1/inAppPurchasePriceSchedules/schedule-1/automaticPrices", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000001/pricePoints", "fields[inAppPurchasePricePoints]=customerPrice,proceeds,territory", "fields[territories]=currency", "filter[territory]=USA", "include=territory", "limit=8000"),
		asctest.JSON(http.StatusOK, `{
			"data":[{"type":"inAppPurchasePricePoints","id":"pp-1","attributes":{"customerPrice":"9.99","proceeds":"8.49"}}],
			"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],
			"links":{"next":""}
		}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if err := root.Parse([]string{"iap", "pricing", "summary", "--iap-id", "9000000001", "--territory", "US"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
}

func TestIAPPricesTableOutput(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000001"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"inAppPurchases","id":"9000000001","attributes":{"name":"Lifetime Unlock","productId":"com.example.lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`))
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000001/iapPriceSchedule", "fields[inAppPurchasePriceSchedules]=baseTerritory,manualPrices,automaticPrices", "fields[inAppPurchasePrices]=startDate,endDate,manual,inAppPurchasePricePoint,territory", "fields[territories]=currency", "include=baseTerritory,manualPrices,automaticPrices", "limit[automaticPrices]=50", "limit[manualPrices]=50"),
		asctest.JSON(http.StatusOK, `{
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
		}`))
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000001/pricePoints", "fields[inAppPurchasePricePoints]=customerPrice,proceeds,territory", "fields[territories]=currency", "filter[territory]=USA", "include=territory", "limit=8000"),
		asctest.JSON(http.StatusOK, `{
			"data":[{"type":"inAppPurchasePricePoints","id":"pp-1","attributes":{"customerPrice":"9.99","proceeds":"8.49"}}],
			"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],
			"links":{"next":""}
		}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"iap", "pricing", "summary", "--iap-id", "9000000001", "--output", "table"}); err != nil {
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

	scheduleBody := buildIAPPriceScheduleWithAutomaticIncludedCount(50)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000001"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"inAppPurchases","id":"9000000001","attributes":{"name":"Lifetime Unlock","productId":"com.example.lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`))
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000001/iapPriceSchedule", "fields[inAppPurchasePriceSchedules]=baseTerritory,manualPrices,automaticPrices", "fields[inAppPurchasePrices]=startDate,endDate,manual,inAppPurchasePricePoint,territory", "fields[territories]=currency", "include=baseTerritory,manualPrices,automaticPrices", "limit[automaticPrices]=50", "limit[manualPrices]=50"),
		asctest.JSON(http.StatusOK, scheduleBody))
	reg.Register(asctest.GET("/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices", "limit=200"),
		asctest.JSON(http.StatusOK, `{
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
		}`))
	reg.Register(asctest.GET("/v1/inAppPurchasePriceSchedules/schedule-1/automaticPrices", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{
					"type":"inAppPurchasePrices",
					"id":"mus-old",
					"attributes":{"startDate":"2024-01-01","endDate":"2099-01-01"},
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
		}`))
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000001/pricePoints", "fields[inAppPurchasePricePoints]=customerPrice,proceeds,territory", "fields[territories]=currency", "filter[territory]=USA", "include=territory", "limit=8000"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{"type":"inAppPurchasePricePoints","id":"pp-current","attributes":{"customerPrice":"9.99","proceeds":"8.49"}}
			],
			"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],
			"links":{"next":""}
		}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"iap", "pricing", "summary", "--iap-id", "9000000001"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	manualSubresourceCalls, automaticSubresourceCalls := 0, 0
	for _, req := range reg.Requests() {
		switch req.URL.Path {
		case "/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices":
			manualSubresourceCalls++
		case "/v1/inAppPurchasePriceSchedules/schedule-1/automaticPrices":
			automaticSubresourceCalls++
		}
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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000002"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"inAppPurchases","id":"9000000002","attributes":{"name":"Legacy Tip","productId":"com.example.legacy.tip","inAppPurchaseType":"CONSUMABLE"}}}`))
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000002/iapPriceSchedule", "fields[inAppPurchasePriceSchedules]=baseTerritory,manualPrices,automaticPrices", "fields[inAppPurchasePrices]=startDate,endDate,manual,inAppPurchasePricePoint,territory", "fields[territories]=currency", "include=baseTerritory,manualPrices,automaticPrices", "limit[automaticPrices]=50", "limit[manualPrices]=50"),
		asctest.JSON(http.StatusOK, `{
			"data":{
				"type":"inAppPurchasePriceSchedules",
				"id":"schedule-legacy",
				"relationships":{"baseTerritory":{"data":{"type":"territories","id":"USA"}}}
			},
			"included":[
				{
					"type":"inAppPurchasePrices",
					"id":"`+legacyPriceResourceID+`",
					"attributes":{"manual":true}
				},
				{
					"type":"territories",
					"id":"USA",
					"attributes":{"currency":"USD"}
				}
			]
		}`))
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000002/pricePoints", "fields[inAppPurchasePricePoints]=customerPrice,proceeds,territory", "fields[territories]=currency", "filter[territory]=USA", "include=territory", "limit=8000"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{
					"type":"inAppPurchasePricePoints",
					"id":"`+canonicalUSAResourceID+`",
					"attributes":{"customerPrice":"2.99","proceeds":"2.54"}
				}
			],
			"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],
			"links":{"next":""}
		}`))
	reg.Register(asctest.GET("/v1/inAppPurchasePriceSchedules/schedule-legacy/manualPrices", "fields[inAppPurchasePricePoints]=customerPrice,proceeds,territory", "fields[inAppPurchasePrices]=manual,inAppPurchasePricePoint,territory", "fields[territories]=currency", "include=inAppPurchasePricePoint,territory", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{
					"type":"inAppPurchasePrices",
					"id":"`+legacyPriceResourceID+`",
					"attributes":{"manual":true},
					"relationships":{
						"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"`+legacyPricePointResourceID+`"}},
						"territory":{"data":{"type":"territories","id":"USA"}}
					}
				}
			],
			"included":[
				{
					"type":"inAppPurchasePricePoints",
					"id":"`+legacyPricePointResourceID+`",
					"attributes":{"customerPrice":"2.99","proceeds":"2.54"}
				},
				{
					"type":"territories",
					"id":"USA",
					"attributes":{"currency":"USD"}
				}
			],
			"links":{"next":""}
		}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"iap", "pricing", "summary", "--iap-id", "9000000002"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	manualFallbackCalls := 0
	for _, req := range reg.Requests() {
		if req.URL.Path == "/v1/inAppPurchasePriceSchedules/schedule-legacy/manualPrices" {
			manualFallbackCalls++
		}
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
