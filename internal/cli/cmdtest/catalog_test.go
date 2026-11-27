package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func catalogFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCatalogRejectsEntireInvalidManifestBeforeHTTP(t *testing.T) {
	for _, tc := range []struct{ body, message string }{
		{`{"groups":[{"referenceName":"Pro"}],"inAppPurchases":[{"productId":"p","name":"P","type":"INVALID"}]}`, "type"},
		{`{"groups":[{"referenceName":"Pro"}],"inAppPurchases":[{"productId":"valid","name":"Valid","type":"CONSUMABLE"},{"productId":"pkg!bad","name":"Invalid","type":"CONSUMABLE"}]}`, "productId is invalid"},
		{`{"groups":[{"referenceName":"Pro"}],"subscriptions":[{"group":"Pro","productId":"valid","name":"Valid","subscriptionPeriod":"ONE_MONTH"},{"group":"Pro","productId":"pkg!bad","name":"Invalid","subscriptionPeriod":"ONE_MONTH"}]}`, "productId is invalid"},
		{`{"groups":[{"referenceName":"Pro"}],"prices":[]}`, "unknown field"},
		{`{"inAppPurchases":[{"productId":"p","name":"P","type":"CONSUMABLE"},{"productId":"p","name":"Other","type":"CONSUMABLE"}]}`, "duplicate productId"},
		{`{"groups":[{"referenceName":"Pro"}]} {}`, "one JSON object"},
		{`{"groups":[{"referenceName":"Pro"}],"groups":[]}`, "duplicate JSON field"},
		{`{"groups":[{"referenceName":"Pro"}],"Groups":[]}`, "duplicate JSON field"},
		{`{"groups":[{"referenceName":"Pro"}],"inAppPurchases":null}`, "must not be null"},
		{`{"groups":[{"referenceName":"Pro"}]}` + strings.Repeat(" ", 4<<20) + `{}`, "exceeds 4 MiB"},
		{`{"groups":[{"referenceName":"Pro"}],"inAppPurchases":[{"productId":"p","name":"Pro","type":"NON_CONSUMABLE","localizations":[{"locale":"en-US","name":"Pro","description":"` + strings.Repeat("x", 46) + `"}]}]}`, "description exceeds 45 characters"},
	} {
		t.Run(tc.message, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			out, stderr, err := runCommand(t, []string{"catalog", "apply", "--app", "app-1", "--file", catalogFile(t, tc.body)})
			if !isUsageClassError(err) || out != "" || !strings.Contains(stderr, tc.message) {
				t.Fatalf("out=%q stderr=%q err=%v", out, stderr, err)
			}
		})
	}
}

func TestCatalogApplyUsesFreshRequestTimeoutAfterPlanning(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_TIMEOUT", "1s")
	t.Setenv("ASC_MAX_RETRIES", "0")
	reg := asctest.NewRegistry(t)
	reg.Install()
	delayed := func(body string) asctest.Responder {
		return func(req *http.Request) (*http.Response, error) {
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(750 * time.Millisecond):
				return asctest.JSON(http.StatusOK, body)(req)
			}
		}
	}
	reg.Register(asctest.GET("/v1/apps/123/subscriptionGroups", "limit=200"), delayed(`{"data":[]}`))
	reg.Register(asctest.POST("/v1/subscriptionGroups"), delayed(`{"data":{"type":"subscriptionGroups","id":"group-1"}}`))
	out, stderr, err := runCommandOnFakeClock(t, []string{"catalog", "apply", "--app", "123", "--file", catalogFile(t, `{"groups":[{"referenceName":"Pro"}]}`), "--output", "json"})
	if err != nil {
		t.Fatalf("out=%s stderr=%s err=%v", out, stderr, err)
	}
	var receipt asc.CatalogResult
	if err := json.Unmarshal([]byte(out), &receipt); err != nil || len(receipt.Items) != 1 || receipt.Items[0].ID != "group-1" || receipt.Items[0].Status != "applied" {
		t.Fatalf("receipt=%s err=%v", out, err)
	}
}

func TestCatalogPlanAndApplyUseStableProductID(t *testing.T) {
	for _, command := range []string{"plan", "apply"} {
		t.Run(command, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), asctest.JSON(200, `{"data":[],"links":{}}`))
			reg.Register(asctest.GET("/v1/apps/app-1/inAppPurchasesV2", "limit=200"), asctest.JSON(200, `{"data":[{"type":"inAppPurchases","id":"iap-1","attributes":{"productId":"p","name":"Old","inAppPurchaseType":"CONSUMABLE"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/inAppPurchasesV2"}}`))
			if command == "apply" {
				reg.Register(asctest.PATCH("/v2/inAppPurchases/iap-1").JSONBody(`{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"New"}}}`), asctest.JSON(200, `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"productId":"p","name":"New","inAppPurchaseType":"CONSUMABLE"}}}`))
			}
			out, stderr, err := runCommand(t, []string{"catalog", command, "--app", "app-1", "--file", catalogFile(t, `{"inAppPurchases":[{"productId":"p","name":"New","type":"CONSUMABLE"}]}`), "--output", "json"})
			if err != nil {
				t.Fatalf("stderr=%q err=%v", stderr, err)
			}
			var receipt struct {
				Items []struct{ Key, Action, Status, ID string }
			}
			if err := json.Unmarshal([]byte(out), &receipt); err != nil {
				t.Fatal(err)
			}
			if len(receipt.Items) != 1 || receipt.Items[0].Key != "p" || receipt.Items[0].Action != "update" || receipt.Items[0].ID != "iap-1" {
				t.Fatalf("receipt=%s", out)
			}
		})
	}
}

func TestCatalogCreatesSubscriptionAfterGroup(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), asctest.JSON(200, `{"data":[],"links":{}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/inAppPurchasesV2", "limit=200"), asctest.JSON(200, `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/inAppPurchasesV2"}}`))
	reg.Register(asctest.POST("/v1/subscriptionGroups").JSONBody(`{"data":{"type":"subscriptionGroups","attributes":{"referenceName":"Pro"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`), asctest.JSON(201, `{"data":{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}}`))
	reg.Register(asctest.POST("/v1/subscriptions").JSONBody(`{"data":{"type":"subscriptions","attributes":{"name":"Monthly","productId":"com.example.monthly","subscriptionPeriod":"ONE_MONTH"},"relationships":{"group":{"data":{"type":"subscriptionGroups","id":"group-1"}}}}}`), asctest.JSON(201, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly","subscriptionPeriod":"ONE_MONTH"}}}`))
	out, stderr, err := runCommand(t, []string{"catalog", "apply", "--app", "app-1", "--file", catalogFile(t, `{"groups":[{"referenceName":"Pro"}],"subscriptions":[{"group":"Pro","productId":"com.example.monthly","name":"Monthly","subscriptionPeriod":"ONE_MONTH"}]}`), "--output", "json"})
	if err != nil {
		t.Fatalf("out=%s stderr=%s err=%v", out, stderr, err)
	}
	var receipt struct {
		Items []struct{ Kind, ID, Status string }
	}
	if err := json.Unmarshal([]byte(out), &receipt); err != nil {
		t.Fatal(err)
	}
	if len(receipt.Items) != 2 || receipt.Items[1].Kind != "subscription" || receipt.Items[1].ID != "sub-1" || receipt.Items[1].Status != "applied" {
		t.Fatalf("receipt=%s", out)
	}
}

func TestCatalogIAPOnlyChecksSubscriptionIdentityAcrossGroupPages(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), asctest.JSON(200, `{"data":[],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/subscriptionGroups?cursor=two"}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "cursor=two"), asctest.JSON(200, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Existing"}}],"links":{}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/inAppPurchasesV2", "limit=200"), asctest.JSON(200, `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/inAppPurchasesV2"}}`))
	reg.Register(asctest.GET("/v1/subscriptionGroups/group-1/subscriptions", "limit=200"), asctest.JSON(200, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"productId":"p","name":"Subscription","subscriptionPeriod":"ONE_MONTH"}}],"links":{}}`))
	out, _, err := runCommand(t, []string{"catalog", "apply", "--app", "app-1", "--file", catalogFile(t, `{"inAppPurchases":[{"productId":"p","name":"Purchase","type":"NON_CONSUMABLE"}]}`)})
	if err == nil || out != "" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestCatalogPricingRefusesUnsafeReplacementAcrossPages(t *testing.T) {
	for _, tc := range []struct{ name, id, territory, start, end string }{
		{"future price", "second", "USA", "2999-01-01", ""},
		{"outside territory", "second", "CAN", "", ""},
		{"ambiguous period", "second", "USA", "", ""},
		{"bounded period", "second", "USA", "2020-01-01", "2999-01-01"},
		{"duplicate resource", "first", "USA", "2999-01-01", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), asctest.JSON(200, `{"data":[],"links":{}}`))
			reg.Register(asctest.GET("/v1/apps/app-1/inAppPurchasesV2", "limit=200"), asctest.JSON(200, `{"data":[{"type":"inAppPurchases","id":"iap-1","attributes":{"productId":"p","name":"Old","inAppPurchaseType":"NON_CONSUMABLE"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/inAppPurchasesV2"}}`))
			reg.Register(asctest.GET("/v2/inAppPurchases/iap-1/pricePoints", "filter[territory]=USA", "limit=200"), asctest.JSON(200, `{"data":[{"type":"inAppPurchasePricePoints","id":"new-price","attributes":{"customerPrice":"9.99"}}],"links":{}}`))
			reg.Register(asctest.GET("/v2/inAppPurchases/iap-1/iapPriceSchedule", "include=baseTerritory"), asctest.JSON(200, `{"data":{"type":"inAppPurchasePriceSchedules","id":"schedule-1","relationships":{"baseTerritory":{"data":{"type":"territories","id":"USA"}}}}}`))
			reg.Register(asctest.GET("/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices", "include=territory,inAppPurchasePricePoint", "limit=200"), asctest.JSON(200, `{"data":[{"type":"inAppPurchasePrices","id":"first","attributes":{},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"old-price"}}}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices?cursor=two"}}`))
			second := map[string]any{"data": []any{map[string]any{"type": "inAppPurchasePrices", "id": tc.id, "attributes": map[string]any{"startDate": tc.start, "endDate": tc.end}, "relationships": map[string]any{"territory": map[string]any{"data": map[string]any{"type": "territories", "id": tc.territory}}, "inAppPurchasePricePoint": map[string]any{"data": map[string]any{"type": "inAppPurchasePricePoints", "id": "other-price"}}}}}, "links": map[string]any{}}
			body, err := json.Marshal(second)
			if err != nil {
				t.Fatal(err)
			}
			reg.Register(asctest.GET("/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices", "cursor=two", "include=territory,inAppPurchasePricePoint"), asctest.JSON(200, string(body)))
			out, _, err := runCommand(t, []string{"catalog", "apply", "--app", "app-1", "--file", catalogFile(t, `{"inAppPurchases":[{"productId":"p","name":"New","type":"NON_CONSUMABLE","pricing":{"baseTerritory":"USA","customerPrice":"9.99","equalize":true}}]}`)})
			if err == nil || out != "" {
				t.Fatalf("out=%q err=%v", out, err)
			}
		})
	}
}

func TestCatalogGroupLocalizationDoesNotUseProductNameLimit(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), asctest.JSON(200, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}],"links":{}}`))
	reg.Register(asctest.GET("/v1/subscriptionGroups/group-1/versions", "limit=200"), asctest.JSON(200, `{"data":[{"type":"subscriptionGroupVersions","id":"version-1","attributes":{"state":"PREPARE_FOR_SUBMISSION","version":1}}],"links":{}}`))
	reg.Register(asctest.GET("/v1/subscriptionGroupVersions/version-1/localizations", "limit=200"), asctest.JSON(200, `{"data":[],"links":{}}`))
	name := strings.Repeat("Group", 7)
	body, err := json.Marshal(map[string]any{"groups": []any{map[string]any{"referenceName": "Pro", "localizations": []any{map[string]any{"locale": "en-US", "name": name}}}}})
	if err != nil {
		t.Fatal(err)
	}
	out, stderr, err := runCommand(t, []string{"catalog", "plan", "--app", "app-1", "--file", catalogFile(t, string(body)), "--output", "json"})
	if err != nil {
		t.Fatalf("out=%q stderr=%q err=%v", out, stderr, err)
	}
	var receipt struct {
		Items []struct{ Kind, Action string }
	}
	if err := json.Unmarshal([]byte(out), &receipt); err != nil {
		t.Fatal(err)
	}
	if len(receipt.Items) != 2 || receipt.Items[1].Kind != "groupLocalization" || receipt.Items[1].Action != "create" {
		t.Fatalf("receipt=%s", out)
	}
}

func TestCatalogRechecksPriceScopeAfterEarlierWrites(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), asctest.JSON(200, `{"data":[],"links":{}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/inAppPurchasesV2", "limit=200"), asctest.JSON(200, `{"data":[{"type":"inAppPurchases","id":"iap-1","attributes":{"productId":"p","name":"Old","inAppPurchaseType":"NON_CONSUMABLE"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/inAppPurchasesV2"}}`))
	reg.Register(asctest.GET("/v2/inAppPurchases/iap-1/pricePoints", "filter[territory]=USA", "limit=200"), asctest.JSON(200, `{"data":[{"type":"inAppPurchasePricePoints","id":"new-price","attributes":{"customerPrice":"9.99"}}],"links":{}}`))
	schedule := `{"data":{"type":"inAppPurchasePriceSchedules","id":"schedule-1","relationships":{"baseTerritory":{"data":{"type":"territories","id":"USA"}}}}}`
	prices := `{"data":[{"type":"inAppPurchasePrices","id":"price-1","attributes":{},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"old-price"}}}}],"links":{}}`
	reg.Register(asctest.GET("/v2/inAppPurchases/iap-1/iapPriceSchedule", "include=baseTerritory"), asctest.JSON(200, schedule))
	reg.Register(asctest.GET("/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices", "include=territory,inAppPurchasePricePoint", "limit=200"), asctest.JSON(200, prices))
	reg.Register(asctest.PATCH("/v2/inAppPurchases/iap-1").JSONBody(`{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"New"}}}`), asctest.JSON(200, `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"productId":"p","name":"New","inAppPurchaseType":"NON_CONSUMABLE"}}}`))
	reg.Register(asctest.GET("/v2/inAppPurchases/iap-1/iapPriceSchedule", "include=baseTerritory"), asctest.JSON(200, schedule))
	reg.Register(asctest.GET("/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices", "include=territory,inAppPurchasePricePoint", "limit=200"), asctest.JSON(200, strings.Replace(prices, `"attributes":{}`, `"attributes":{"startDate":"2999-01-01"}`, 1)))
	out, _, err := runCommand(t, []string{"catalog", "apply", "--app", "app-1", "--file", catalogFile(t, `{"inAppPurchases":[{"productId":"p","name":"New","type":"NON_CONSUMABLE","pricing":{"baseTerritory":"USA","customerPrice":"9.99","equalize":true}}]}`), "--output", "json"})
	if err == nil {
		t.Fatal("expected a partial failure after the concurrent schedule change")
	}
	var receipt struct {
		Status string
		Items  []struct{ Status, Kind string }
	}
	if err := json.Unmarshal([]byte(out), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "partial" || len(receipt.Items) != 2 || receipt.Items[0].Status != "applied" || receipt.Items[1].Kind != "inAppPurchasePricing" || receipt.Items[1].Status != "failed" {
		t.Fatalf("receipt=%s", out)
	}
}

func TestCatalogRejectsInvalidOutputBeforeHTTP(t *testing.T) {
	for _, mode := range []string{"plan", "apply"} {
		t.Run(mode, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			out, _, err := runCommand(t, []string{
				"catalog", mode, "--app", "app-1", "--file", catalogFile(t, `{"groups":[{"referenceName":"Pro"}]}`),
				"--output", "invalid",
			})
			if !isUsageClassError(err) || out != "" {
				t.Fatalf("out=%q err=%v", out, err)
			}
		})
	}
}

func TestCatalogRejectsDuplicateIAPResourceBeforeWrites(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), asctest.JSON(200, `{"data":[],"links":{}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/inAppPurchasesV2", "limit=200"), asctest.JSON(200, `{"data":[{"type":"inAppPurchases","id":"iap-1","attributes":{"productId":"p1","name":"First","inAppPurchaseType":"CONSUMABLE"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/inAppPurchasesV2","next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/inAppPurchasesV2?cursor=two"}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/inAppPurchasesV2", "cursor=two"), asctest.JSON(200, `{"data":[{"type":"inAppPurchases","id":"iap-1","attributes":{"productId":"p2","name":"Second","inAppPurchaseType":"CONSUMABLE"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/inAppPurchasesV2?cursor=two"}}`))
	out, _, err := runCommand(t, []string{"catalog", "apply", "--app", "app-1", "--file", catalogFile(t, `{"inAppPurchases":[{"productId":"p1","name":"First","type":"CONSUMABLE"},{"productId":"p2","name":"Renamed","type":"CONSUMABLE"}]}`)})
	if err == nil || out != "" || !strings.Contains(err.Error(), "duplicate resource") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestCatalogRejectsDiscoveryOutsideSelectedCollection(t *testing.T) {
	for _, tc := range []struct{ name, body, location string }{
		{name: "continuation", body: `{"data":[],"links":{"next":"/v1/apps/app-2/subscriptionGroups?cursor=next"}}`},
		{name: "redirect", location: "https://api.appstoreconnect.apple.com/v1/apps/app-2/subscriptionGroups"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			response := asctest.JSON(200, tc.body)
			if tc.location != "" {
				response = func(req *http.Request) (*http.Response, error) {
					r, err := asctest.JSON(302, `{}`)(req)
					r.Header.Set("Location", tc.location)
					return r, err
				}
			}
			reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), response)
			out, _, err := runCommand(t, []string{"catalog", "apply", "--app", "app-1", "--file", catalogFile(t, `{"groups":[{"referenceName":"Pro"}]}`)})
			if err == nil || out != "" || len(reg.Requests()) != 1 {
				t.Fatalf("out=%q err=%v request count=%d", out, err, len(reg.Requests()))
			}
			if tc.location == "" && !strings.Contains(err.Error(), "scoped collection") {
				t.Fatalf("expected scoped continuation refusal, got %v", err)
			}
		})
	}
}

func TestCatalogSubscriptionPaginationPreservesQuery(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(map[bool]string{false: "cursor keeps query", true: "changed territory refused"}[conflict], func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), asctest.JSON(200, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}],"links":{}}`))
			reg.Register(asctest.GET("/v1/apps/app-1/inAppPurchasesV2", "limit=200"), asctest.JSON(200, `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/inAppPurchasesV2"}}`))
			reg.Register(asctest.GET("/v1/subscriptionGroups/group-1/subscriptions", "limit=200"), asctest.JSON(200, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"productId":"p","name":"Monthly","subscriptionPeriod":"ONE_MONTH"}}],"links":{}}`))
			next := "/v1/subscriptions/sub-1/pricePoints?cursor=two"
			if conflict {
				next += "&filter%5Bterritory%5D=CAN"
			}
			reg.Register(asctest.GET("/v1/subscriptions/sub-1/pricePoints", "filter[territory]=USA", "limit=200"), asctest.JSON(200, `{"data":[],"links":{"next":"`+next+`"}}`))
			if !conflict {
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/pricePoints", "cursor=two", "filter[territory]=USA"), asctest.JSON(200, `{"data":[{"type":"subscriptionPricePoints","id":"point-1","attributes":{"customerPrice":"9.99"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}],"links":{}}`))
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/prices", "include=territory,subscriptionPricePoint,additionalPriceTiers", "limit=200"), asctest.JSON(200, `{"data":[],"links":{}}`))
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/introductoryOffers", "include=territory,subscriptionPricePoint", "limit=200"), asctest.JSON(200, `{"data":[],"links":{"next":"/v1/subscriptions/sub-1/introductoryOffers?cursor=two"}}`))
				reg.Register(asctest.GET("/v1/subscriptions/sub-1/introductoryOffers", "include=territory,subscriptionPricePoint", "cursor=two"), asctest.JSON(200, `{"data":[],"links":{}}`))
			}
			out, _, err := runCommand(t, []string{"catalog", "plan", "--app", "app-1", "--file", catalogFile(t, `{"groups":[{"referenceName":"Pro"}],"subscriptions":[{"group":"Pro","productId":"p","name":"Monthly","subscriptionPeriod":"ONE_MONTH","pricing":{"baseTerritory":"USA","customerPrice":"9.99"},"introductoryOffers":[]}]}`), "--output", "json"})
			if conflict {
				if err == nil || out != "" || !strings.Contains(err.Error(), "scoped collection query") || len(reg.Requests()) != 4 {
					t.Fatalf("out=%q err=%v request count=%d", out, err, len(reg.Requests()))
				}
				return
			}
			if err != nil {
				t.Fatalf("catalog pagination: %v", err)
			}
			var result struct {
				Items []struct{ Kind, Action string }
			}
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatal(err)
			}
			if len(result.Items) != 3 || result.Items[2].Kind != "subscriptionPricing" || result.Items[2].Action != "create" {
				t.Fatalf("receipt=%s", out)
			}
		})
	}
}

func TestCatalogSubscriptionPricingPreservesAdditionalTiers(t *testing.T) {
	for _, tc := range []struct {
		name, tiers, startDate             string
		matching, allowed, paginated, late bool
		wantWrites                         int
	}{
		{name: "linked tiers", tiers: `,"additionalPriceTiers":{"data":[{"type":"subscriptionPriceTiers","id":"tier-1"}]}`},
		{name: "missing linkage"},
		{name: "null linkage", tiers: `,"additionalPriceTiers":{"data":null}`},
		{name: "malformed linkage", tiers: `,"additionalPriceTiers":{"data":{}}`},
		{name: "incomplete linkage", tiers: `,"additionalPriceTiers":{"data":[],"meta":{"paging":{"total":1}}}`},
		{name: "omitted linkage on next page", paginated: true},
		{name: "tiers appear before write", late: true, tiers: `,"additionalPriceTiers":{"data":[{"type":"subscriptionPriceTiers","id":"tier-1"}]}`},
		{name: "matching tiered price", matching: true, allowed: true, tiers: `,"additionalPriceTiers":{"data":[{"type":"subscriptionPriceTiers","id":"tier-1"}]}`},
		{name: "matching without linkage", matching: true, allowed: true},
		{name: "matching price already active on the Pacific day", matching: true, allowed: true, startDate: "2026-10-02"},
		{name: "complete empty linkage", allowed: true, wantWrites: 1, tiers: `,"additionalPriceTiers":{"data":[]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupAuth(t)
			point := "old-price"
			if tc.matching {
				point = "new-price"
			}
			writes, priceReads := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				var body string
				switch r.Method + " " + r.URL.Path {
				case "GET /v1/apps/app-1/subscriptionGroups":
					body = `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}],"links":{}}`
				case "GET /v1/apps/app-1/inAppPurchasesV2":
					body = `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/inAppPurchasesV2"}}`
				case "GET /v1/subscriptionGroups/group-1/subscriptions":
					body = `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"productId":"p","name":"Monthly","subscriptionPeriod":"ONE_MONTH"}}],"links":{}}`
				case "GET /v1/subscriptions/sub-1/pricePoints":
					body = `{"data":[{"type":"subscriptionPricePoints","id":"new-price","attributes":{"customerPrice":"9.99"}}],"links":{}}`
				case "GET /v1/subscriptions/sub-1/prices":
					if tc.paginated && r.URL.Query().Get("cursor") == "" {
						body = `{"data":[],"links":{"next":"https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/prices?cursor=two"}}`
						break
					}
					priceReads++
					tiers := tc.tiers
					if tc.late && priceReads == 1 {
						tiers = `,"additionalPriceTiers":{"data":[]}`
					}
					if !strings.Contains(r.URL.Query().Get("include"), "additionalPriceTiers") {
						tiers = `,"additionalPriceTiers":{"links":{"related":"https://api.appstoreconnect.apple.com/v1/subscriptionPrices/price-1/additionalPriceTiers"}}`
					}
					body = fmt.Sprintf(`{"data":[{"type":"subscriptionPrices","id":"price-1","attributes":{"preserved":false,"startDate":%q},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":%q}}%s}}],"links":{}}`, tc.startDate, point, tiers)
				case "PATCH /v1/subscriptions/sub-1":
					writes++
					point = "new-price"
					body = `{"data":{"type":"subscriptions","id":"sub-1"}}`
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			// Oct 2 in California is still Oct 1 on the client's clock.
			now := time.Date(2026, 10, 1, 23, 0, 0, 0, time.FixedZone("UTC-12", -12*60*60))
			out, stderr, err := runPricingCurrentAt(t, server, now, "catalog", "apply", "--app", "app-1", "--file", catalogFile(t, `{"groups":[{"referenceName":"Pro"}],"subscriptions":[{"group":"Pro","productId":"p","name":"Monthly","subscriptionPeriod":"ONE_MONTH","pricing":{"baseTerritory":"USA","customerPrice":"9.99"}}]}`), "--output", "json")
			if !tc.allowed {
				if err == nil || writes != 0 || !strings.Contains(err.Error(), "additional price tiers") {
					t.Fatalf("unsafe replacement: writes=%d err=%v stdout=%s stderr=%s", writes, err, out, stderr)
				}
				return
			}
			if err != nil || writes != tc.wantWrites {
				t.Fatalf("allowed operation: writes=%d err=%v stdout=%s stderr=%s", writes, err, out, stderr)
			}
		})
	}
}

func TestCatalogSavedPlanRefusesDriftWithSameOperation(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	for _, name := range []string{"Old A", "Old B"} {
		reg.Register(asctest.GET("/v1/apps/123/subscriptionGroups", "limit=200"), asctest.JSON(200, `{"data":[]}`))
		reg.Register(asctest.GET("/v1/apps/123/inAppPurchasesV2", "limit=200"), asctest.JSON(200, fmt.Sprintf(
			`{"data":[{"type":"inAppPurchases","id":"iap-1","attributes":{"productId":"p","name":%q,"inAppPurchaseType":"CONSUMABLE"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/123/inAppPurchasesV2"}}`, name)))
	}
	manifestPath := catalogFile(t, `{"inAppPurchases":[{"productId":"p","name":"New","type":"CONSUMABLE"}]}`)
	planPath := filepath.Join(t.TempDir(), "approved.json")
	_, stderr, err := runCommand(t, []string{"catalog", "plan", "--app", "123", "--file", manifestPath, "--out", planPath, "--output", "json"})
	if err != nil {
		t.Fatalf("plan stderr=%q err=%v", stderr, err)
	}
	out, stderr, err := runCommand(t, []string{"catalog", "apply", "--app", "123", "--file", manifestPath, "--plan", planPath})
	if !isUsageClassError(err) || out != "" || !strings.Contains(stderr, "plan drifted") {
		t.Fatalf("apply stdout=%q stderr=%q err=%v", out, stderr, err)
	}
	if got := len(reg.Requests()); got != 4 {
		t.Fatalf("expected four preflight GETs and no writes, got %d requests", got)
	}
}

func TestCatalogSavedPlanFailureReceipts(t *testing.T) {
	offers := `{"data":[{"type":"subscriptionIntroductoryOffers","id":"offer-1","attributes":{"offerMode":"FREE_TRIAL","duration":"ONE_WEEK","numberOfPeriods":1},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}},{"type":"subscriptionIntroductoryOffers","id":"offer-2","attributes":{"offerMode":"FREE_TRIAL","duration":"ONE_WEEK","numberOfPeriods":1},"relationships":{"territory":{"data":{"type":"territories","id":"CAN"}}}}]}`
	canceled := func(*http.Request) (*http.Response, error) { return nil, context.Canceled }
	for _, tc := range []struct {
		name         string
		readFailure  asctest.Responder
		writeFailure asctest.Responder
		rateLimited  bool
		readAttempts int
		wantStatus   string
		wantWrites   int
	}{
		{name: "changed offer", readFailure: asctest.JSON(200, strings.ReplaceAll(offers, "ONE_WEEK", "TWO_WEEKS")), readAttempts: 1, wantStatus: "failed"},
		{name: "prewrite 503", readFailure: asctest.JSON(503, `{}`), readAttempts: 2, wantStatus: "failed"},
		{name: "prewrite cancellation", readFailure: canceled, readAttempts: 1, wantStatus: "failed"},
		{name: "changed offer after rejected write", readFailure: asctest.JSON(200, strings.ReplaceAll(offers, "ONE_WEEK", "TWO_WEEKS")), rateLimited: true, readAttempts: 1, wantStatus: "failed", wantWrites: 1},
		{name: "write 503", writeFailure: asctest.JSON(503, `{}`), wantStatus: "unknown", wantWrites: 1},
		{name: "write cancellation", writeFailure: canceled, wantStatus: "unknown", wantWrites: 1},
		{name: "write 503 after rejection", writeFailure: asctest.JSON(503, `{}`), rateLimited: true, wantStatus: "unknown", wantWrites: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_MAX_RETRIES", "1")
			reg := asctest.NewRegistry(t)
			reg.Install()
			readOffers := asctest.GET("/v1/subscriptions/sub-1/introductoryOffers", "include=territory,subscriptionPricePoint", "limit=200")
			deleteOffer := asctest.DELETE("/v1/subscriptionIntroductoryOffers/offer-1")
			for range 2 {
				reg.Register(asctest.GET("/v1/apps/123/subscriptionGroups", "limit=200"), asctest.JSON(200, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}]}`))
				reg.Register(asctest.GET("/v1/apps/123/inAppPurchasesV2", "limit=200"), asctest.JSON(200, `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/123/inAppPurchasesV2"}}`))
				reg.Register(asctest.GET("/v1/subscriptionGroups/group-1/subscriptions", "limit=200"), asctest.JSON(200, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"productId":"p","name":"Monthly","subscriptionPeriod":"ONE_MONTH"}}]}`))
				reg.Register(readOffers, asctest.JSON(200, offers))
			}
			if tc.rateLimited {
				reg.Register(readOffers, asctest.JSON(200, offers))
				reg.Register(deleteOffer, asctest.JSON(429, `{}`))
			}
			if tc.readFailure != nil {
				for range tc.readAttempts {
					reg.Register(readOffers, tc.readFailure)
				}
			} else {
				reg.Register(readOffers, asctest.JSON(200, offers))
				reg.Register(deleteOffer, tc.writeFailure)
			}
			file := catalogFile(t, `{"groups":[{"referenceName":"Pro"}],"subscriptions":[{"group":"Pro","productId":"p","name":"Monthly","subscriptionPeriod":"ONE_MONTH","introductoryOffers":[]}]}`)
			plan := filepath.Join(t.TempDir(), "approved.json")
			if out, stderr, err := runCommand(t, []string{"catalog", "plan", "--app", "123", "--file", file, "--out", plan, "--output", "json"}); err != nil {
				t.Fatalf("plan stdout=%s stderr=%s err=%v", out, stderr, err)
			}
			out, stderr, err := runCommandOnFakeClock(t, []string{"catalog", "apply", "--app", "123", "--file", file, "--plan", plan, "--confirm", "--output", "json"})
			var receipt asc.CatalogResult
			if decodeErr := json.Unmarshal([]byte(out), &receipt); decodeErr != nil {
				t.Fatalf("receipt=%s decode=%v command=%v stderr=%s", out, decodeErr, err, stderr)
			}
			writes := 0
			for _, request := range reg.Requests() {
				if request.Method != http.MethodGet {
					writes++
				}
			}
			if err == nil || writes != tc.wantWrites || receipt.Status != "partial" || len(receipt.Items) != 4 || receipt.Items[2].Status != tc.wantStatus || receipt.Items[3].Status != "pending" || receipt.Recovery == "" {
				t.Fatalf("writes=%d want=%d status=%s stdout=%s stderr=%s err=%v", writes, tc.wantWrites, tc.wantStatus, out, stderr, err)
			}
			if tc.wantStatus == "failed" && receipt.Items[2].Error != "The operation stopped before writing." {
				t.Fatalf("prewrite failure message=%q", receipt.Items[2].Error)
			}
		})
	}
}

func TestCatalogApplyRefusesExplicitEmptyPlanBeforeHTTP(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	manifestPath := catalogFile(t, `{"groups":[{"referenceName":"Pro"}]}`)
	out, stderr, err := runCommand(t, []string{"catalog", "apply", "--app", "123", "--file", manifestPath, "--plan", ""})
	if !isUsageClassError(err) || out != "" || !strings.Contains(stderr, "--plan requires a file path") || len(reg.Requests()) != 0 {
		t.Fatalf("stdout=%q stderr=%q err=%v requests=%v", out, stderr, err, reg.Requests())
	}
}

func TestCatalogSavedPlanRefusesMissingAvailabilityDecision(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()

	groups := asctest.GET("/v1/apps/123/subscriptionGroups", "limit=200")
	products := asctest.GET("/v1/apps/123/inAppPurchasesV2", "limit=200")
	availability := asctest.GET("/v2/inAppPurchases/iap-1/inAppPurchaseAvailability")
	for _, attributes := range []string{`{"availableInNewTerritories":false}`, `{"availableInNewTerritories":null}`} {
		reg.Register(groups, asctest.JSON(200, `{"data":[]}`))
		reg.Register(products, asctest.JSON(200, `{"data":[{"type":"inAppPurchases","id":"iap-1","attributes":{"productId":"p","name":"Purchase","inAppPurchaseType":"CONSUMABLE"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/123/inAppPurchasesV2"}}`))
		reg.Register(availability, asctest.JSON(200, fmt.Sprintf(`{"data":{"type":"inAppPurchaseAvailabilities","id":"available","attributes":%s}}`, attributes)))
	}
	reg.Register(asctest.GET("/v1/inAppPurchaseAvailabilities/available/availableTerritories", "limit=200"),
		asctest.JSON(200, `{"data":[{"type":"territories","id":"USA","attributes":{}}]}`))

	manifestPath := catalogFile(t, `{"inAppPurchases":[{"productId":"p","name":"Purchase","type":"CONSUMABLE","availability":{"territories":["CAN","USA"],"availableInNewTerritories":false}}]}`)
	planPath := filepath.Join(t.TempDir(), "approved.json")
	_, stderr, err := runCommand(t, []string{"catalog", "plan", "--app", "123", "--file", manifestPath, "--out", planPath, "--output", "json"})
	if err != nil {
		t.Fatalf("plan stderr=%q err=%v", stderr, err)
	}
	out, stderr, err := runCommand(t, []string{"catalog", "apply", "--app", "123", "--file", manifestPath, "--plan", planPath, "--output", "json"})
	if err == nil || !strings.Contains(err.Error(), "omitted availableInNewTerritories") || out != "" {
		t.Fatalf("apply stdout=%q stderr=%q err=%v", out, stderr, err)
	}
	if got := len(reg.Requests()); got != 7 {
		t.Fatalf("expected seven preflight GETs and no writes, got %d requests", got)
	}
}

func TestCatalogPlanRefusesSymlinkedOutputParent(t *testing.T) {
	other := t.TempDir()
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(other, link); err != nil {
		t.Skipf("cannot create directory symlink: %v", err)
	}
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/subscriptionGroups", "limit=200"), asctest.JSON(200, `{"data":[]}`))

	manifestPath := catalogFile(t, `{"groups":[{"referenceName":"Pro"}]}`)
	out, stderr, err := runCommand(t, []string{"catalog", "plan", "--app", "123", "--file", manifestPath, "--out", filepath.Join(link, "plan.json")})
	if err == nil || !strings.Contains(err.Error(), "symlink") || out != "" {
		t.Fatalf("plan stdout=%q stderr=%q err=%v", out, stderr, err)
	}
	if _, err := os.Stat(filepath.Join(other, "plan.json")); !os.IsNotExist(err) {
		t.Fatalf("plan publication outside root returned %v", err)
	}
}
