package cmdtest

import (
	"encoding/json"
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
		wantQuery    map[string]string
		response     string
		wantDataID   string
		wantIncluded string
	}{
		{
			name: "content", args: []string{"iap", "content", "view", "--content-id", "content-1", "--iap-fields", "versions", "--output", "json"},
			path: "/v1/inAppPurchaseContents/content-1", wantQuery: map[string]string{
				"fields[inAppPurchases]": "versions",
				"include":                "inAppPurchaseV2",
			},
			response: "{\"data\":{\"type\":\"inAppPurchaseContents\",\"id\":\"content-1\"}}",
		},
		{
			name: "iap promoted list", args: []string{"iap", "promoted-purchases", "list", "--app", "app-1", "--iap-fields", "versions", "--subscription-fields", "versions", "--output", "json"},
			path: "/v1/apps/app-1/promotedPurchases", wantQuery: map[string]string{
				"fields[inAppPurchases]": "versions",
				"fields[subscriptions]":  "versions",
				"include":                "inAppPurchaseV2,subscription",
			},
			wantDataID: "promo-iap", wantIncluded: "inAppPurchases:iap-1",
		},
		{
			name: "subscription promoted list", args: []string{"subscriptions", "promoted-purchases", "list", "--app", "app-1", "--iap-fields", "versions", "--subscription-fields", "versions", "--output", "json"},
			path: "/v1/apps/app-1/promotedPurchases", wantQuery: map[string]string{
				"fields[inAppPurchases]": "versions",
				"fields[subscriptions]":  "versions",
				"include":                "inAppPurchaseV2,subscription",
			},
			wantDataID: "promo-subscription", wantIncluded: "subscriptions:subscription-1",
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
				assertExactQueryValues441(t, req, tt.wantQuery)
				body := tt.response
				if body == "" {
					body = `{"data":[` +
						`{"type":"promotedPurchases","id":"promo-iap","relationships":{"inAppPurchaseV2":{"data":{"type":"inAppPurchases","id":"iap-1"}}}},` +
						`{"type":"promotedPurchases","id":"promo-subscription","relationships":{"subscription":{"data":{"type":"subscriptions","id":"subscription-1"}}}}` +
						`],"included":[` +
						`{"type":"inAppPurchases","id":"iap-1","attributes":{"versions":"iap-version"}},` +
						`{"type":"subscriptions","id":"subscription-1","attributes":{"versions":"subscription-version"}}` +
						`]}`
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
			if tt.wantDataID != "" {
				var output struct {
					Data []struct {
						ID string `json:"id"`
					} `json:"data"`
					Included []struct {
						Type string `json:"type"`
						ID   string `json:"id"`
					} `json:"included"`
				}
				if err := json.Unmarshal([]byte(stdout), &output); err != nil {
					t.Fatalf("decode stdout: %v; stdout=%q stderr=%q", err, stdout, stderr)
				}
				if len(output.Data) != 1 || output.Data[0].ID != tt.wantDataID {
					t.Fatalf("data = %+v, want only %q", output.Data, tt.wantDataID)
				}
				if len(output.Included) != 1 || output.Included[0].Type+":"+output.Included[0].ID != tt.wantIncluded {
					t.Fatalf("included = %+v, want only %q", output.Included, tt.wantIncluded)
				}
			}
		})
	}
}

func TestIAPPromotedPurchaseViewByOwnerSendsRelationshipQuery441(t *testing.T) {
	setupSubmitCreateAuth(t)
	t.Setenv("ASC_APP_ID", "")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/123456789/promotedPurchase" {
			t.Fatalf("request = %s %s, want relationship GET", req.Method, req.URL.Path)
		}
		wantQuery := map[string]string{
			"fields[inAppPurchases]": "versions",
			"fields[subscriptions]":  "versions",
			"include":                "inAppPurchaseV2,subscription",
		}
		assertExactQueryValues441(t, req, wantQuery)
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
			"--subscription-fields", "versions",
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

func TestIAPPromotedPurchaseViewResolvesStableSelector441(t *testing.T) {
	tests := []struct {
		name         string
		selector     string
		useAppEnv    bool
		wantRequests int
	}{
		{name: "product ID via ASC_APP_ID", selector: "com.example.pro", useAppEnv: true, wantRequests: 2},
		{name: "exact current name", selector: "Premium", wantRequests: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupSubmitCreateAuth(t)
			if tt.useAppEnv {
				t.Setenv("ASC_APP_ID", "app-1")
			} else {
				t.Setenv("ASC_APP_ID", "")
			}
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requests++
				if req.Method != http.MethodGet {
					t.Fatalf("request method = %s, want GET", req.Method)
				}
				w.Header().Set("Content-Type", "application/json")
				switch req.URL.Path {
				case "/v1/apps/app-1/inAppPurchasesV2":
					productValues := req.URL.Query()["filter[productId]"]
					nameValues := req.URL.Query()["filter[name]"]
					if len(productValues) == 1 {
						assertExactQueryValues441(t, req, map[string]string{"filter[productId]": tt.selector, "limit": "200"})
						if tt.selector == "com.example.pro" {
							_, _ = w.Write([]byte(`{"data":[{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Premium","productId":"com.example.pro"}}]}`))
							return
						}
						_, _ = w.Write([]byte(`{"data":[]}`))
						return
					}
					if len(nameValues) == 1 {
						assertExactQueryValues441(t, req, map[string]string{"filter[name]": tt.selector, "limit": "200"})
						_, _ = w.Write([]byte(`{"data":[{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Premium","productId":"com.example.other"}}]}`))
						return
					}
					t.Fatalf("unexpected lookup query: %v", req.URL.Query())
				case "/v2/inAppPurchases/iap-1/promotedPurchase":
					assertExactQueryValues441(t, req, map[string]string{
						"fields[inAppPurchases]": "versions",
						"include":                "inAppPurchaseV2",
					})
					_, _ = w.Write([]byte(`{"data":{"type":"promotedPurchases","id":"promo-1"}}`))
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				}
			}))
			defer server.Close()
			setIAPRelatedTestServerClient(t, server)

			args := []string{
				"iap", "promoted-purchases", "view",
				"--iap-id", tt.selector,
				"--iap-fields", "versions",
				"--output", "json",
			}
			if !tt.useAppEnv {
				args = append(args, "--app", "app-1")
			}
			stdout, stderr := captureOutput(t, func() {
				code := cmd.Run(args, "1.2.3")
				if code != cmd.ExitSuccess {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
				}
			})
			if requests != tt.wantRequests || !strings.Contains(stdout, `"id":"promo-1"`) {
				t.Fatalf("requests=%d stdout=%q stderr=%q", requests, stdout, stderr)
			}
		})
	}
}

func TestIAPPromotedPurchaseViewStableSelectorErrors441(t *testing.T) {
	t.Run("missing app", func(t *testing.T) {
		setupSubmitCreateAuth(t)
		t.Setenv("ASC_APP_ID", "")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}))
		defer server.Close()
		setIAPRelatedTestServerClient(t, server)

		stdout, stderr := captureOutput(t, func() {
			if code := cmd.Run([]string{"iap", "promoted-purchases", "view", "--iap-id", "com.example.pro"}, "1.2.3"); code != cmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
			}
		})
		if strings.TrimSpace(stdout) != "" || !strings.Contains(stderr, "--app is required (or set ASC_APP_ID) when --iap-id is a product ID or name") {
			t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
		}
	})

	t.Run("ambiguous name", func(t *testing.T) {
		setupSubmitCreateAuth(t)
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			requests++
			if req.Method != http.MethodGet {
				t.Fatalf("request method = %s, want GET", req.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			if req.URL.Path != "/v1/apps/app-1/inAppPurchasesV2" {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("filter[productId]") == "Premium" {
				assertExactQueryValues441(t, req, map[string]string{"filter[productId]": "Premium", "limit": "200"})
				_, _ = w.Write([]byte(`{"data":[]}`))
				return
			}
			assertExactQueryValues441(t, req, map[string]string{"filter[name]": "Premium", "limit": "200"})
			_, _ = w.Write([]byte(`{"data":[{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Premium","productId":"com.example.one"}},{"type":"inAppPurchases","id":"iap-2","attributes":{"name":"Premium","productId":"com.example.two"}}]}`))
		}))
		defer server.Close()
		setIAPRelatedTestServerClient(t, server)

		_, stderr := captureOutput(t, func() {
			if code := cmd.Run([]string{"iap", "promoted-purchases", "view", "--app", "app-1", "--iap-id", "Premium"}, "1.2.3"); code == cmd.ExitSuccess {
				t.Fatal("expected ambiguity failure")
			}
		})
		if requests != 2 || !strings.Contains(stderr, "Use the explicit ASC ID to disambiguate") {
			t.Fatalf("requests=%d stderr=%q", requests, stderr)
		}
	})

	t.Run("lookup API error", func(t *testing.T) {
		setupSubmitCreateAuth(t)
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			requests++
			if req.Method != http.MethodGet {
				t.Fatalf("request method = %s, want GET", req.Method)
			}
			if req.URL.Path != "/v1/apps/app-1/inAppPurchasesV2" {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			}
			assertExactQueryValues441(t, req, map[string]string{"filter[productId]": "com.example.pro", "limit": "200"})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errors":[{"status":"403","code":"FORBIDDEN.REQUIRED_ROLE","detail":"forbidden"}]}`))
		}))
		defer server.Close()
		setIAPRelatedTestServerClient(t, server)

		_, stderr := captureOutput(t, func() {
			if code := cmd.Run([]string{"iap", "promoted-purchases", "view", "--app", "app-1", "--iap-id", "com.example.pro"}, "1.2.3"); code == cmd.ExitSuccess {
				t.Fatal("expected lookup error")
			}
		})
		if requests != 1 || !strings.Contains(stderr, "resolve in-app purchase by product ID") {
			t.Fatalf("requests=%d stderr=%q", requests, stderr)
		}
	})
}

func TestPromotedPurchasePaginatedScopedListsPruneIncluded441(t *testing.T) {
	tests := []struct {
		name         string
		command      []string
		wantDataID   string
		wantIncluded string
		wantVersion  string
	}{
		{
			name: "iap scope", command: []string{"iap", "promoted-purchases", "list"},
			wantDataID: "promo-iap", wantIncluded: "inAppPurchases:iap-1", wantVersion: "iap-version",
		},
		{
			name: "subscription scope", command: []string{"subscriptions", "promoted-purchases", "list"},
			wantDataID: "promo-subscription", wantIncluded: "subscriptions:subscription-1", wantVersion: "subscription-version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupSubmitCreateAuth(t)
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requests++
				if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/promotedPurchases" {
					t.Fatalf("request = %s %s, want promoted-purchases GET", req.Method, req.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				if req.URL.Query().Get("cursor") == "next" {
					assertExactQueryValues441(t, req, map[string]string{"cursor": "next"})
					_, _ = w.Write([]byte(`{"data":[{"type":"promotedPurchases","id":"promo-subscription","relationships":{"subscription":{"data":{"type":"subscriptions","id":"subscription-1"}}}}],"included":[{"type":"subscriptions","id":"subscription-1","attributes":{"versions":["subscription-version"],"name":"Subscription"}}]}`))
					return
				}

				wantFirstQuery := map[string]string{
					"fields[inAppPurchases]": "versions",
					"fields[subscriptions]":  "versions",
					"include":                "inAppPurchaseV2,subscription",
					"limit":                  "200",
				}
				assertExactQueryValues441(t, req, wantFirstQuery)
				body := `{"data":[{"type":"promotedPurchases","id":"promo-iap","relationships":{"inAppPurchaseV2":{"data":{"type":"inAppPurchases","id":"iap-1"}}}}],"included":[{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"IAP","versions":["iap-version"]}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/promotedPurchases?cursor=next"}}`
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			setIAPRelatedTestServerClient(t, server)

			args := append(append([]string{}, tt.command...), "--app", "app-1", "--iap-fields", "versions", "--subscription-fields", "versions", "--paginate", "--output", "json")
			exitCode := -1
			stdout, stderr := captureOutput(t, func() {
				exitCode = cmd.Run(args, "1.2.3")
			})
			if exitCode != cmd.ExitSuccess {
				t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", exitCode, cmd.ExitSuccess, stdout, stderr)
			}
			if requests != 2 {
				t.Fatalf("requests = %d, want two pages", requests)
			}

			var output struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
				Included []struct {
					Type       string `json:"type"`
					ID         string `json:"id"`
					Attributes struct {
						Versions []string `json:"versions"`
					} `json:"attributes"`
				} `json:"included"`
			}
			if err := json.Unmarshal([]byte(stdout), &output); err != nil {
				t.Fatalf("decode stdout: %v; stdout=%q stderr=%q", err, stdout, stderr)
			}
			if len(output.Data) != 1 || output.Data[0].ID != tt.wantDataID {
				t.Fatalf("data = %+v, want only %q", output.Data, tt.wantDataID)
			}
			if len(output.Included) != 1 || output.Included[0].Type+":"+output.Included[0].ID != tt.wantIncluded {
				t.Fatalf("included = %+v, want only %q", output.Included, tt.wantIncluded)
			}
			if len(output.Included[0].Attributes.Versions) != 1 || output.Included[0].Attributes.Versions[0] != tt.wantVersion {
				t.Fatalf("included attributes = %+v, want version %q", output.Included[0].Attributes, tt.wantVersion)
			}
		})
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
		{name: "invalid cross-scope iap fields", args: []string{"subscriptions", "promoted-purchases", "view", "--promoted-purchase-id", "promo-1", "--iap-fields", "notAField"}, want: "--iap-fields must be one of"},
		{name: "invalid cross-scope subscription fields", args: []string{"iap", "promoted-purchases", "view", "--promoted-purchase-id", "promo-1", "--subscription-fields", "notAField"}, want: "--subscription-fields must be one of"},
		{name: "images next conflict", args: []string{"iap", "images", "list", "--next", next, "--iap-fields", "versions"}, want: "--next cannot be combined with --iap-fields"},
		{name: "localizations next conflict", args: []string{"iap", "localizations", "list", "--next", next, "--iap-fields", "versions"}, want: "--next cannot be combined with --iap-fields"},
		{name: "iap promoted next conflict", args: []string{"iap", "promoted-purchases", "list", "--next", next, "--subscription-fields", "versions"}, want: "--next cannot be combined with --iap-fields or --subscription-fields"},
		{name: "subscription promoted next conflict", args: []string{"subscriptions", "promoted-purchases", "list", "--next", next, "--iap-fields", "versions"}, want: "--next cannot be combined with --iap-fields or --subscription-fields"},
		{name: "promoted view missing selector", args: []string{"iap", "promoted-purchases", "view"}, want: "--promoted-purchase-id or --iap-id is required"},
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
		{path: []string{"iap", "promoted-purchases", "list"}, flag: "--subscription-fields"},
		{path: []string{"iap", "promoted-purchases", "view"}, flag: "--iap-id"},
		{path: []string{"iap", "promoted-purchases", "view"}, flag: "--iap-fields"},
		{path: []string{"iap", "promoted-purchases", "view"}, flag: "--subscription-fields"},
		{path: []string{"subscriptions", "promoted-purchases", "list"}, flag: "--iap-fields"},
		{path: []string{"subscriptions", "promoted-purchases", "list"}, flag: "--subscription-fields"},
		{path: []string{"subscriptions", "promoted-purchases", "view"}, flag: "--iap-fields"},
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

	iapView := findSubcommand(root, "iap", "promoted-purchases", "view")
	iapUsage := iapView.UsageFunc(iapView)
	if !strings.Contains(iapUsage, "In-app purchase ID, product ID, or exact current name") {
		t.Fatalf("IAP promoted view help does not describe stable selectors: %q", iapUsage)
	}
	if !strings.Contains(iapUsage, "App Store Connect app ID (or ASC_APP_ID env; required when --iap-id uses a product ID or name)") {
		t.Fatalf("IAP promoted view help does not describe app lookup context: %q", iapUsage)
	}
	if !strings.Contains(iapUsage, "--iap-id IAP_SELECTOR") || !strings.Contains(iapUsage, `--iap-id "IAP_SELECTOR"`) {
		t.Fatalf("IAP promoted view help does not use the stable-selector placeholder: %q", iapUsage)
	}

	subscriptionView := findSubcommand(root, "subscriptions", "promoted-purchases", "view")
	subscriptionUsage := subscriptionView.UsageFunc(subscriptionView)
	if !strings.Contains(subscriptionUsage, "--subscription-id SUBSCRIPTION_SELECTOR") ||
		!strings.Contains(subscriptionUsage, "Subscription ID, product ID, or exact current name") ||
		!strings.Contains(subscriptionUsage, subscriptionLookupAppUsageForTest) {
		t.Fatalf("subscription promoted view does not expose the stable selector contract: %q", subscriptionUsage)
	}
}

const subscriptionLookupAppUsageForTest = "App Store Connect app ID (or ASC_APP_ID env; required when --subscription-id uses a product ID or name)"

func setIAPRelatedTestServerClient(t *testing.T, server *httptest.Server) {
	t.Helper()
	client := newReviewTestServerClient(t, server)
	restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	})
	t.Cleanup(restore)
}

func assertExactQueryValues441(t *testing.T, req *http.Request, want map[string]string) {
	t.Helper()
	query := req.URL.Query()
	if len(query) != len(want) {
		t.Fatalf("query = %v, want exactly %v", query, want)
	}
	for key, wantValue := range want {
		values, ok := query[key]
		if !ok || len(values) != 1 || values[0] != wantValue {
			t.Fatalf("query[%q] = %v, want exactly [%q]", key, values, wantValue)
		}
	}
}
