package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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

		territoryIDs := make([]string, 0, 2)
		for index, resource := range included {
			relationships := resource.(map[string]any)["relationships"].(map[string]any)
			territory := relationships["territory"].(map[string]any)["data"].(map[string]any)
			territoryIDs = append(territoryIDs, territory["id"].(string))
			if index == 1 {
				if _, exists := relationships["pricePoint"]; exists {
					t.Fatalf("expected free territory to omit pricePoint relationship")
				}
			}
		}
		if !slices.Equal(territoryIDs, []string{"USA", "JPN"}) {
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

func TestIAPOfferCodesCreateReturnsCreateFailure(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseOfferCodes" {
			t.Fatalf("expected path /v1/inAppPurchaseOfferCodes, got %s", req.URL.Path)
		}
		body := `{"errors":[{"status":"409","title":"Conflict","detail":"duplicate code"}]}`
		return &http.Response{
			StatusCode: http.StatusConflict,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

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
	t.Setenv("ASC_TIMEOUT", "10ms")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		switch req.URL.Path {
		case "/v1/apps/app-123/inAppPurchasesV2":
			<-req.Context().Done()
			return nil, req.Context().Err()
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
	t.Setenv("ASC_TIMEOUT", "10ms")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		switch req.URL.Path {
		case "/v1/apps/app-123/inAppPurchasesV2":
			<-req.Context().Done()
			return nil, req.Context().Err()
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

func TestIAPOfferCodesListRejectsInvalidNextURL(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "offer-codes", "list",
			"--next", "https://example.com/v2/inAppPurchases/9000000001/offerCodes?cursor=AQ",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "iap offer-codes list: --next must be an App Store Connect URL") {
		t.Fatalf("expected invalid --next error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestIAPOfferCodesListRejectsMalformedNextURL(t *testing.T) {
	tests := []struct {
		name    string
		next    string
		wantErr string
	}{
		{
			name:    "invalid scheme",
			next:    "http://api.appstoreconnect.apple.com/v2/inAppPurchases/9000000001/offerCodes?cursor=AQ",
			wantErr: "iap offer-codes list: --next must be an App Store Connect URL",
		},
		{
			name:    "malformed URL",
			next:    "https://api.appstoreconnect.apple.com/%zz",
			wantErr: "iap offer-codes list: --next must be a valid URL:",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"iap", "offer-codes", "list",
					"--next", test.next,
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(runErr.Error(), test.wantErr) {
				t.Fatalf("expected error %q, got %v", test.wantErr, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestIAPOfferCodesListOutputErrors(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/inAppPurchases/9000000001/offerCodes" {
			t.Fatalf("expected path /v2/inAppPurchases/9000000001/offerCodes, got %s", req.URL.Path)
		}
		body := `{"data":[{"type":"inAppPurchaseOfferCodes","id":"offer-1"}],"links":{"next":""}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unsupported output",
			args:    []string{"iap", "offer-codes", "list", "--iap-id", "9000000001", "--output", "yaml"},
			wantErr: "unsupported format: yaml",
		},
		{
			name:    "pretty with table",
			args:    []string{"iap", "offer-codes", "list", "--iap-id", "9000000001", "--output", "table", "--pretty"},
			wantErr: "--pretty is only valid with JSON output",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected help error, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestIAPOfferCodesListPaginateFromNextWithoutIAP(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const firstURL = "https://api.appstoreconnect.apple.com/v2/inAppPurchases/9000000001/offerCodes?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v2/inAppPurchases/9000000001/offerCodes?cursor=BQ&limit=200"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.String() != firstURL {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			body := `{
				"data":[{"type":"inAppPurchaseOfferCodes","id":"offer-next-1"}],
				"links":{"next":"` + secondURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != secondURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{
				"data":[{"type":"inAppPurchaseOfferCodes","id":"offer-next-2"}],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

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

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/inAppPurchases/9000000001/offerCodes" {
			t.Fatalf("expected path /v2/inAppPurchases/9000000001/offerCodes, got %s", req.URL.Path)
		}
		body := `{
			"data":[{"type":"inAppPurchaseOfferCodes","id":"offer-table-1","attributes":{"name":"Spring","active":true}}],
			"links":{"next":""}
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

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
