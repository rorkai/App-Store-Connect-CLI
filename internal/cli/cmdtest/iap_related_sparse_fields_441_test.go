package cmdtest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestIAPRelatedSparseFieldFlagsSendExactQueries441(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		path         string
		fieldKey     string
		include      string
		response     string
		relationship string
		resourceType string
	}{
		{
			name: "content", args: []string{"iap", "content", "view", "--content-id", "content-1", "--iap-fields", "versions", "--output", "json"},
			path: "/v1/inAppPurchaseContents/content-1", fieldKey: "fields[inAppPurchases]", include: "inAppPurchaseV2",
			response: "{\"data\":{\"type\":\"inAppPurchaseContents\",\"id\":\"content-1\"}}",
		},
		{
			name: "iap promoted list", args: []string{"iap", "promoted-purchases", "list", "--app", "app-1", "--iap-fields", "versions", "--output", "json"},
			path: "/v1/apps/app-1/promotedPurchases", fieldKey: "fields[inAppPurchases]", include: "inAppPurchaseV2",
			relationship: "inAppPurchaseV2", resourceType: "inAppPurchases",
		},
		{
			name: "subscription promoted list", args: []string{"subscriptions", "promoted-purchases", "list", "--app", "app-1", "--subscription-fields", "versions", "--output", "json"},
			path: "/v1/apps/app-1/promotedPurchases", fieldKey: "fields[subscriptions]", include: "subscription",
			relationship: "subscription", resourceType: "subscriptions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupSubmitCreateAuth(t)
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requests++
				if req.Method != http.MethodGet || req.URL.Path != tt.path {
					t.Fatalf("request = %s %s, want GET %s", req.Method, req.URL.Path, tt.path)
				}
				if got := req.URL.Query().Get(tt.fieldKey); got != "versions" {
					t.Fatalf("%s = %q, want versions", tt.fieldKey, got)
				}
				if got := req.URL.Query().Get("include"); got != tt.include {
					t.Fatalf("include = %q, want %q", got, tt.include)
				}
				if len(req.URL.Query()) != 2 {
					t.Fatalf("query = %v, want exactly sparse field and include", req.URL.Query())
				}
				body := tt.response
				if body == "" {
					body = "{\"data\":[{\"type\":\"promotedPurchases\",\"id\":\"promo-1\",\"relationships\":{\"" +
						tt.relationship + "\":{\"data\":{\"type\":\"" + tt.resourceType + "\",\"id\":\"product-1\"}}}}]}"
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			setIAPRelatedTestServerClient(t, server)

			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run(tt.args, "1.2.3"); code != cmd.ExitSuccess {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
				}
			})
			if requests != 1 {
				t.Fatalf("requests = %d, want 1", requests)
			}
			if !strings.Contains(stdout, "\"data\"") {
				t.Fatalf("stdout = %q, want JSON response; stderr=%q", stdout, stderr)
			}
		})
	}
}

func TestIAPPromotedPurchaseViewByOwnerSendsRelationshipQuery441(t *testing.T) {
	setupSubmitCreateAuth(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/123456789/promotedPurchase" {
			t.Fatalf("request = %s %s, want relationship GET", req.Method, req.URL.Path)
		}
		if got := req.URL.Query().Get("fields[inAppPurchases]"); got != "versions" {
			t.Fatalf("fields[inAppPurchases] = %q, want versions", got)
		}
		if got := req.URL.Query().Get("include"); got != "inAppPurchaseV2" {
			t.Fatalf("include = %q, want inAppPurchaseV2", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"data\":{\"type\":\"promotedPurchases\",\"id\":\"promo-1\"}}"))
	}))
	defer server.Close()
	setIAPRelatedTestServerClient(t, server)

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"iap", "promoted-purchases", "view",
			"--iap-id", "123456789",
			"--iap-fields", "versions",
			"--output", "json",
		}, "1.2.3")
		if code != cmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
		}
	})
	if requests != 1 || !strings.Contains(stdout, "\"id\":\"promo-1\"") {
		t.Fatalf("requests=%d stdout=%q stderr=%q", requests, stdout, stderr)
	}
}

func TestIAPRelatedSparseFieldValidationPrecedesHTTP441(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v2/inAppPurchases/123/images?cursor=next"
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "invalid iap fields", args: []string{"iap", "content", "view", "--content-id", "content-1", "--iap-fields", "notAField"}, want: "--iap-fields must be one of"},
		{name: "invalid subscription fields", args: []string{"subscriptions", "promoted-purchases", "list", "--app", "app-1", "--subscription-fields", "notAField"}, want: "--subscription-fields must be one of"},
		{name: "images next conflict", args: []string{"iap", "images", "list", "--next", next, "--iap-fields", "versions"}, want: "--next cannot be combined with --iap-fields"},
		{name: "localizations next conflict", args: []string{"iap", "localizations", "list", "--next", next, "--iap-fields", "versions"}, want: "--next cannot be combined with --iap-fields"},
		{name: "iap promoted next conflict", args: []string{"iap", "promoted-purchases", "list", "--next", next, "--iap-fields", "versions"}, want: "--next cannot be combined with --iap-fields"},
		{name: "subscription promoted next conflict", args: []string{"subscriptions", "promoted-purchases", "list", "--next", next, "--subscription-fields", "versions"}, want: "--next cannot be combined with --subscription-fields"},
		{name: "promoted view selector conflict", args: []string{"iap", "promoted-purchases", "view", "--promoted-purchase-id", "promo-1", "--iap-id", "123"}, want: "--promoted-purchase-id and --iap-id are mutually exclusive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run(tt.args, "1.2.3"); code != cmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
				}
			})
			if strings.TrimSpace(stdout) != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, tt.want) {
				t.Fatalf("stderr = %q, want %q", stderr, tt.want)
			}
		})
	}
}

func TestIAPRelatedSparseFieldHelp441(t *testing.T) {
	root := RootCommand("1.2.3")
	tests := []struct {
		path []string
		flag string
	}{
		{path: []string{"iap", "review-screenshots", "view"}, flag: "--iap-fields"},
		{path: []string{"iap", "content", "view"}, flag: "--iap-fields"},
		{path: []string{"iap", "images", "list"}, flag: "--iap-fields"},
		{path: []string{"iap", "images", "view"}, flag: "--iap-fields"},
		{path: []string{"iap", "localizations", "list"}, flag: "--iap-fields"},
		{path: []string{"iap", "promoted-purchases", "list"}, flag: "--iap-fields"},
		{path: []string{"iap", "promoted-purchases", "view"}, flag: "--iap-id"},
		{path: []string{"iap", "promoted-purchases", "view"}, flag: "--iap-fields"},
		{path: []string{"subscriptions", "promoted-purchases", "list"}, flag: "--subscription-fields"},
		{path: []string{"subscriptions", "promoted-purchases", "view"}, flag: "--subscription-fields"},
	}
	for _, tt := range tests {
		command := findSubcommand(root, tt.path...)
		if command == nil {
			t.Fatalf("command %v not found", tt.path)
		}
		if usage := command.UsageFunc(command); !strings.Contains(usage, tt.flag) {
			t.Fatalf("help for %v does not contain %s: %q", tt.path, tt.flag, usage)
		}
	}
}

func setIAPRelatedTestServerClient(t *testing.T, server *httptest.Server) {
	t.Helper()
	client := newReviewTestServerClient(t, server)
	restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	})
	t.Cleanup(restore)
}
