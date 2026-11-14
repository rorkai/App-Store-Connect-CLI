package cmdtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestScopedPromotedPurchasesResolveOmittedProductLinkage(t *testing.T) {
	tests := []struct {
		group string
		id    string
	}{
		{group: "iap", id: "promo-iap"},
		{group: "subscriptions", id: "promo-sub"},
	}
	for _, test := range tests {
		for _, action := range []string{"list", "view"} {
			t.Run(test.group+"/"+action, func(t *testing.T) {
				setupSubmitCreateAuth(t)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					if req.Method != http.MethodGet {
						t.Errorf("unexpected method %s", req.Method)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					switch req.URL.Path {
					case "/v1/apps/app-1/promotedPurchases":
						fmt.Fprint(w, `{"data":[{"type":"promotedPurchases","id":"promo-iap"},{"type":"promotedPurchases","id":"promo-sub"}]}`)
					case "/v1/promotedPurchases/promo-iap", "/v1/promotedPurchases/promo-sub":
						id := strings.TrimPrefix(req.URL.Path, "/v1/promotedPurchases/")
						if req.URL.Query().Get("include") != "inAppPurchaseV2,subscription" {
							fmt.Fprintf(w, `{"data":{"type":"promotedPurchases","id":%q}}`, id)
							return
						}
						if id == "promo-iap" {
							fmt.Fprint(w, `{"data":{"type":"promotedPurchases","id":"promo-iap","relationships":{"inAppPurchaseV2":{"data":{"type":"inAppPurchases","id":"iap-1"}}}},"included":[{"type":"inAppPurchases","id":"iap-1"}]}`)
						} else {
							fmt.Fprint(w, `{"data":{"type":"promotedPurchases","id":"promo-sub","relationships":{"subscription":{"data":{"type":"subscriptions","id":"sub-1"}}}},"included":[{"type":"subscriptions","id":"sub-1"}]}`)
						}
					default:
						t.Errorf("unexpected path %s", req.URL.Path)
						w.WriteHeader(http.StatusBadRequest)
					}
				}))
				defer server.Close()
				setIAPRelatedTestServerClient(t, server)

				args := []string{test.group, "promoted-purchases", action, "--output", "json"}
				var wantData any = map[string]any{"type": "promotedPurchases", "id": test.id}
				if action == "list" {
					args = append(args, "--app", "app-1")
					wantData = []any{wantData}
				} else {
					args = append(args, "--promoted-purchase-id", test.id)
				}
				var exit int
				stdout, stderr := captureOutput(t, func() { exit = cmd.Run(args, "1.2.3") })
				if exit != cmd.ExitSuccess || stderr != "" {
					t.Fatalf("exit = %d, stderr = %q", exit, stderr)
				}
				var result struct {
					Data     any             `json:"data"`
					Included json.RawMessage `json:"included"`
				}
				if err := json.Unmarshal([]byte(stdout), &result); err != nil {
					t.Fatalf("decode result: %v", err)
				}
				if !reflect.DeepEqual(result.Data, wantData) || len(result.Included) != 0 {
					t.Fatalf("result = %s, want data %v without internal includes", stdout, wantData)
				}
			})
		}
	}
}

func TestIAPPromotedPurchasesGetRejectsSubscriptionPromotedPurchase(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/promotedPurchases/promo-sub", "include=inAppPurchaseV2,subscription"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"promotedPurchases","id":"promo-sub","relationships":{"subscription":{"data":{"type":"subscriptions","id":"sub-1"}}}}}`))

	stdout, stderr, runErr := runCommand(t, []string{"iap", "promoted-purchases", "view", "--promoted-purchase-id", "promo-sub"})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), `belongs to subscription "sub-1", not an in-app purchase`) {
		t.Fatalf("expected scope mismatch error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestIAPPromotedPurchasesLinkPreservesSubscriptionPromotedPurchases(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/promotedPurchases", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[`+
			`{"type":"promotedPurchases","id":"promo-sub","relationships":{"subscription":{"data":{"type":"subscriptions","id":"sub-1"}}}},`+
			`{"type":"promotedPurchases","id":"promo-iap-old","relationships":{"inAppPurchaseV2":{"data":{"type":"inAppPurchases","id":"iap-old"}}}}`+
			`],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/promotedPurchases/promo-iap-new", "include=inAppPurchaseV2,subscription"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"promotedPurchases","id":"promo-iap-new","relationships":{"inAppPurchaseV2":{"data":{"type":"inAppPurchases","id":"iap-new"}}}}}`))
	reg.Register(asctest.PATCH("/v1/apps/app-1/relationships/promotedPurchases").JSONBody(`{"data":[{"type":"promotedPurchases","id":"promo-sub"},{"type":"promotedPurchases","id":"promo-iap-new"}]}`),
		asctest.JSON(http.StatusNoContent, ""))

	stdout, stderr, err := runCommand(t, []string{"iap", "promoted-purchases", "link", "--app", "app-1", "--promoted-purchase-id", "promo-iap-new"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"promo-sub"`) || !strings.Contains(stdout, `"promo-iap-new"`) {
		t.Fatalf("expected output to contain preserved subscription and new iap IDs, got %q", stdout)
	}
	if strings.Contains(stdout, `"promo-iap-old"`) {
		t.Fatalf("expected output to exclude replaced iap ID, got %q", stdout)
	}
}
