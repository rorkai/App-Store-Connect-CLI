package cmdtest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
			reg.Register(asctest.GET("/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices", "cursor=two"), asctest.JSON(200, string(body)))
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
