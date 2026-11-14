package cmdtest

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestIAPPromotedPurchasesCreateAcceptsNormalizedProductTypeAlias(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/promotedPurchases").JSONBody(`{"data":{"type":"promotedPurchases","attributes":{"visibleForAllUsers":true},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}},"inAppPurchaseV2":{"data":{"type":"inAppPurchases","id":"IAP_ID"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"promotedPurchases","id":"promo-1","attributes":{"enabled":true}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"iap", "promoted-purchases", "create",
		"--app", "APP_ID",
		"--product-id", "IAP_ID",
		"--product-type", "in-app-purchase",
		"--visible-for-all-users", "true",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "promo-1" {
		t.Fatalf("expected promoted purchase id promo-1, got %q", out.Data.ID)
	}
}

func TestSubscriptionsPromotedPurchasesCreateAcceptsNormalizedProductTypeAlias(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/promotedPurchases").JSONBody(`{"data":{"type":"promotedPurchases","attributes":{"visibleForAllUsers":true},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}},"subscription":{"data":{"type":"subscriptions","id":"SUB_ID"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"promotedPurchases","id":"promo-2","attributes":{"enabled":true}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"subscriptions", "promoted-purchases", "create",
		"--app", "APP_ID",
		"--product-id", "SUB_ID",
		"--product-type", "subscription",
		"--visible-for-all-users", "true",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "promo-2" {
		t.Fatalf("expected promoted purchase id promo-2, got %q", out.Data.ID)
	}
}
