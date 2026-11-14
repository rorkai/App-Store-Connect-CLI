package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestIAPCreateFamilySharable(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v2/inAppPurchases").JSONBody(`{"data":{"type":"inAppPurchases","attributes":{"name":"Pro","productId":"com.example.pro","inAppPurchaseType":"CONSUMABLE","familySharable":true},"relationships":{"app":{"data":{"type":"apps","id":"APP1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"inAppPurchases","id":"new-iap","attributes":{"name":"Pro","productId":"com.example.pro","inAppPurchaseType":"CONSUMABLE","familySharable":true}}}`))

	stdout, stderr, err := runCommand(t, []string{"iap", "create", "--app", "APP1", "--type", "CONSUMABLE", "--ref-name", "Pro", "--product-id", "com.example.pro", "--family-sharable"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	// Verify output contains the response
	if !strings.Contains(stdout, `"familySharable":true`) {
		t.Fatalf("expected familySharable in output, got %q", stdout)
	}
}

func TestIAPUpdateFamilySharableAsSoleFlag(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.PATCH("/v2/inAppPurchases/iap-1").JSONBody(`{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"familySharable":true}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Pro","productId":"com.example.pro","inAppPurchaseType":"CONSUMABLE","familySharable":true}}}`))

	stdout, stderr, err := runCommand(t, []string{"iap", "update", "--id", "iap-1", "--family-sharable"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	if !strings.Contains(stdout, `"familySharable":true`) {
		t.Fatalf("expected familySharable in output, got %q", stdout)
	}
}
