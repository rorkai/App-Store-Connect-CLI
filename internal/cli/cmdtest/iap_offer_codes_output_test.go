package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestIAPOfferCodesCreateUsesDefaultEligibilitiesAndParsedPrices(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseOfferCodes" {
			t.Fatalf("expected path /v1/inAppPurchaseOfferCodes, got %s", req.URL.Path)
		}

		rawBody, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			t.Fatalf("decode request body: %v\nbody=%s", err, string(rawBody))
		}

		data := payload["data"].(map[string]any)
		attrs := data["attributes"].(map[string]any)
		if attrs["name"] != "SPRING" {
			t.Fatalf("expected name SPRING, got %#v", attrs["name"])
		}

		eligibilityItems := attrs["customerEligibilities"].([]any)
		gotEligibilities := make([]string, 0, len(eligibilityItems))
		for _, item := range eligibilityItems {
			gotEligibilities = append(gotEligibilities, item.(string))
		}
		wantEligibilities := []string{"NON_SPENDER", "ACTIVE_SPENDER", "CHURNED_SPENDER"}
		if !slices.Equal(gotEligibilities, wantEligibilities) {
			t.Fatalf("expected default eligibilities %v, got %v", wantEligibilities, gotEligibilities)
		}

		relationships := data["relationships"].(map[string]any)
		iapRelationship := relationships["inAppPurchase"].(map[string]any)["data"].(map[string]any)
		if iapRelationship["id"] != "9000000001" {
			t.Fatalf("expected inAppPurchase id 9000000001, got %#v", iapRelationship["id"])
		}

		included := payload["included"].([]any)
		if len(included) != 2 {
			t.Fatalf("expected 2 included price objects, got %d", len(included))
		}

		territoryIDs := make(map[string]bool, 2)
		for _, resource := range included {
			relationships := resource.(map[string]any)["relationships"].(map[string]any)
			territory := relationships["territory"].(map[string]any)["data"].(map[string]any)
			territoryID := territory["id"].(string)
			territoryIDs[territoryID] = true
			switch territoryID {
			case "USA":
				pricePoint := relationships["pricePoint"].(map[string]any)["data"].(map[string]any)
				if pricePoint["id"] != "pp-us" {
					t.Fatalf("expected USA price point pp-us, got %#v", pricePoint["id"])
				}
			case "JPN":
				if _, exists := relationships["pricePoint"]; exists {
					t.Fatalf("expected free territory to omit pricePoint relationship")
				}
			default:
				t.Fatalf("unexpected territory %q", territoryID)
			}
		}
		if !territoryIDs["USA"] || !territoryIDs["JPN"] {
			t.Fatalf("expected normalized territory ids [USA JPN], got %v", territoryIDs)
		}

		body := `{"data":{"type":"inAppPurchaseOfferCodes","id":"offer-1","attributes":{"name":"SPRING","active":true}}}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	setIAPRelatedTestServerClient(t, server)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "offer-codes", "create",
			"--iap-id", "9000000001",
			"--name", "SPRING",
			"--prices", "usa:pp-us,jpn:FREE",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

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
	if out.Data.ID != "offer-1" {
		t.Fatalf("expected created offer code id offer-1, got %q", out.Data.ID)
	}
}

func TestIAPOfferCodePricesRequestsRelationshipsForTableOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseOfferCodes/offer-1/prices" {
			t.Fatalf("expected offer-code prices path, got %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("fields[inAppPurchaseOfferPrices]"); got != "territory,pricePoint" {
			t.Fatalf("expected offer price relationship fields, got %q", got)
		}
		if got := req.URL.Query().Get("include"); got != "territory,pricePoint" {
			t.Fatalf("expected offer price relationships to be included, got %q", got)
		}

		body := `{"data":[` +
			`{"type":"inAppPurchaseOfferPrices","id":"paid-1","relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"pricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-us"}}}},` +
			`{"type":"inAppPurchaseOfferPrices","id":"free-1","relationships":{"territory":{"data":{"type":"territories","id":"CAN"}},"pricePoint":{"data":null}}}` +
			`]}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	setIAPRelatedTestServerClient(t, server)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "offer-codes", "prices",
			"--offer-code-id", "offer-1",
			"--output", "table",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, want := range []string{"USA", "pp-us", "CAN", "FREE"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected table output to contain %q, got %q", want, stdout)
		}
	}
}

func TestIAPOfferCodePricesNextURLPreservesRelationshipsForTableOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/inAppPurchaseOfferCodes/offer-1/prices", "cursor=legacy", "fields[inAppPurchaseOfferPrices]=territory,pricePoint", "include=territory,pricePoint"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":"https://api.appstoreconnect.apple.com/v1/inAppPurchaseOfferCodes/offer-1/prices?cursor=page2"}}`))
	reg.Register(asctest.GET("/v1/inAppPurchaseOfferCodes/offer-1/prices", "cursor=page2", "fields[inAppPurchaseOfferPrices]=territory,pricePoint", "include=territory,pricePoint"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"inAppPurchaseOfferPrices","id":"free-1","relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"pricePoint":{"data":null}}}],"links":{"next":null}}`))

	nextURL := "https://api.appstoreconnect.apple.com/v1/inAppPurchaseOfferCodes/offer-1/prices?cursor=legacy"
	stdout, stderr, err := runCommand(t, []string{
		"iap", "offer-codes", "prices",
		"--next", nextURL,
		"--paginate",
		"--output", "table",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests := len(reg.Requests()); requests != 2 {
		t.Fatalf("expected two paginated requests, got %d", requests)
	}
	for _, want := range []string{"USA", "FREE"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected table output to contain %q, got %q", want, stdout)
		}
	}
}

func TestIAPOfferCodesCreateReturnsCreateFailure(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/inAppPurchaseOfferCodes").JSONBody(`{"data":{"type":"inAppPurchaseOfferCodes","attributes":{"name":"SPRING","customerEligibilities":["NON_SPENDER","ACTIVE_SPENDER","CHURNED_SPENDER"]},"relationships":{"inAppPurchase":{"data":{"type":"inAppPurchases","id":"9000000001"}},"prices":{"data":[{"type":"inAppPurchaseOfferPrices","id":"${local-price-1}"}]}}},"included":[{"type":"inAppPurchaseOfferPrices","id":"${local-price-1}","relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"pricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-us"}}}}]}`),
		asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","title":"Conflict","detail":"duplicate code"}]}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "offer-codes", "create",
			"--iap-id", "9000000001",
			"--name", "SPRING",
			"--prices", "usa:pp-us",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "iap offer-codes create: failed to create") {
		t.Fatalf("expected create failure, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestIAPOfferCodesListFallsBackToNumericIDAfterLookupTimeout(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	lookupTimeout := expireSelectorLookup(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		switch req.URL.Path {
		case "/v1/apps/app-123/inAppPurchasesV2":
			return lookupTimeout(req)
		case "/v2/inAppPurchases/2024/offerCodes":
			if err := req.Context().Err(); err != nil {
				t.Fatalf("expected fresh list context after lookup timeout, got %v", err)
			}
			body := `{"data":[{"type":"inAppPurchaseOfferCodes","id":"offer-timeout-1"}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "offer-codes", "list",
			"--app", "app-123",
			"--iap-id", "2024",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests != 2 {
		t.Fatalf("expected lookup timeout followed by offer-code list fetch, got %d requests", requests)
	}
	if !strings.Contains(stdout, `"id":"offer-timeout-1"`) {
		t.Fatalf("expected fallback list output, got %q", stdout)
	}
}

func TestIAPOfferCodesCreateFallsBackToNumericIDAfterLookupTimeout(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	lookupTimeout := expireSelectorLookup(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		switch req.URL.Path {
		case "/v1/apps/app-123/inAppPurchasesV2":
			return lookupTimeout(req)
		case "/v1/inAppPurchaseOfferCodes":
			if err := req.Context().Err(); err != nil {
				t.Fatalf("expected fresh create context after lookup timeout, got %v", err)
			}
			rawBody, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body error: %v", err)
			}
			var payload map[string]any
			if err := json.Unmarshal(rawBody, &payload); err != nil {
				t.Fatalf("decode request body: %v\nbody=%s", err, string(rawBody))
			}
			relationships := payload["data"].(map[string]any)["relationships"].(map[string]any)
			iapRelationship := relationships["inAppPurchase"].(map[string]any)["data"].(map[string]any)
			if iapRelationship["id"] != "2024" {
				t.Fatalf("expected numeric fallback IAP id 2024, got %#v", iapRelationship["id"])
			}
			body := `{"data":{"type":"inAppPurchaseOfferCodes","id":"offer-timeout-create","attributes":{"name":"SPRING","active":true}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "offer-codes", "create",
			"--app", "app-123",
			"--iap-id", "2024",
			"--name", "SPRING",
			"--prices", "usa:pp-us",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests != 2 {
		t.Fatalf("expected lookup timeout followed by create fetch, got %d requests", requests)
	}
	if !strings.Contains(stdout, `"id":"offer-timeout-create"`) {
		t.Fatalf("expected created offer code output, got %q", stdout)
	}
}

// TestIAPOfferCodesListRejectsMalformedNextURL asserts the usage contract
// (diagnostic on stderr, exit 2) for the rejections shared.ValidateNextURL
// produces.
func TestIAPOfferCodesListRejectsMalformedNextURL(t *testing.T) {
	tests := []struct {
		name    string
		next    string
		wantErr string
	}{
		{
			name:    "foreign host",
			next:    "https://example.com/v2/inAppPurchases/9000000001/offerCodes?cursor=AQ",
			wantErr: "iap offer-codes list: --next must be an App Store Connect URL",
		},
		{
			name:    "invalid scheme",
			next:    "http://api.appstoreconnect.apple.com/v2/inAppPurchases/9000000001/offerCodes?cursor=AQ",
			wantErr: "iap offer-codes list: --next must be an App Store Connect URL",
		},
		{
			name:    "malformed URL",
			next:    malformedNextURL,
			wantErr: "iap offer-codes list: --next must be a valid URL: " + malformedNextURLParseError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExitCode(
				t,
				[]string{"iap", "offer-codes", "list", "--next", test.next},
				test.wantErr,
			)
		})
	}
}

func TestIAPOfferCodesListPaginateFromNextWithoutIAP(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const firstURL = "https://api.appstoreconnect.apple.com/v2/inAppPurchases/9000000001/offerCodes?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v2/inAppPurchases/9000000001/offerCodes?cursor=BQ&limit=200"

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000001/offerCodes", "cursor=AQ", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[{"type":"inAppPurchaseOfferCodes","id":"offer-next-1"}],
			"links":{"next":"`+secondURL+`"}
		}`))
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000001/offerCodes", "cursor=BQ", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[{"type":"inAppPurchaseOfferCodes","id":"offer-next-2"}],
			"links":{"next":""}
		}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"iap", "offer-codes", "list", "--paginate", "--next", firstURL}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"offer-next-1"`) || !strings.Contains(stdout, `"id":"offer-next-2"`) {
		t.Fatalf("expected both paginated offer codes in output, got %q", stdout)
	}
}

func TestIAPOfferCodesListTableOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000001/offerCodes"),
		asctest.JSON(http.StatusOK, `{
			"data":[{"type":"inAppPurchaseOfferCodes","id":"offer-table-1","attributes":{"name":"Spring","active":true}}],
			"links":{"next":""}
		}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"iap", "offer-codes", "list", "--iap-id", "9000000001", "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "offer-table-1") {
		t.Fatalf("expected table output to contain offer code id, got %q", stdout)
	}
}
