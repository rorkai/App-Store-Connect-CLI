package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/handlertest"
)

func TestPricingAvailabilityRemoveFromSaleUpdatesAndVerifiesTerritories(t *testing.T) {
	setupAuth(t)
	fixture := handlertest.New(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var mu sync.Mutex
	states := map[string]bool{"USA": true, "FRA": false}
	var patches atomic.Int32

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return singlePlatformAppStoreVersionsResponse(), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appAvailabilityV2":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":true}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v2/appAvailabilities/availability-1/territoryAvailabilities":
			mu.Lock()
			defer mu.Unlock()
			return territoryAvailabilityResponse(fixture, states), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/territoryAvailabilities/ta-usa":
			var payload struct {
				Data struct {
					ID         string `json:"id"`
					Attributes struct {
						Available *bool `json:"available"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				return nil, fixture.Errorf("decode PATCH: %w", err)
			}
			if payload.Data.ID != "ta-usa" || payload.Data.Attributes.Available == nil || *payload.Data.Attributes.Available {
				return nil, fixture.Errorf("unexpected PATCH payload: %+v", payload)
			}
			patches.Add(1)
			mu.Lock()
			states["USA"] = false
			mu.Unlock()
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"territoryAvailabilities","id":"ta-usa","attributes":{"available":false}}}`), nil
		default:
			return nil, fixture.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"pricing", "availability", "remove-from-sale", "--app", "app-1", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if got := patches.Load(); got != 1 {
		t.Fatalf("PATCH count = %d, want 1", got)
	}
	var result struct {
		AppID                          string   `json:"appId"`
		AvailabilityID                 string   `json:"availabilityId"`
		Status                         string   `json:"status"`
		AvailableInNewTerritories      bool     `json:"availableInNewTerritories"`
		TotalTerritories               int      `json:"totalTerritories"`
		UpdatedTerritories             int      `json:"updatedTerritories"`
		AlreadyUnavailableTerritories  int      `json:"alreadyUnavailableTerritories"`
		VerifiedUnavailableTerritories int      `json:"verifiedUnavailableTerritories"`
		FailedTerritories              []string `json:"failedTerritories"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode result %q: %v", stdout, err)
	}
	if result.AppID != "app-1" || result.AvailabilityID != "availability-1" || result.Status != "removedFromSale" {
		t.Fatalf("unexpected identity result: %+v", result)
	}
	if !result.AvailableInNewTerritories {
		t.Fatalf("expected preserved new-territory policy, got %+v", result)
	}
	if result.TotalTerritories != 2 || result.UpdatedTerritories != 1 || result.AlreadyUnavailableTerritories != 1 || result.VerifiedUnavailableTerritories != 2 || len(result.FailedTerritories) != 0 {
		t.Fatalf("unexpected counts: %+v", result)
	}
	if !strings.Contains(stderr, "preserved availableInNewTerritories=true") {
		t.Fatalf("expected preserved-policy caveat, got %q", stderr)
	}
}

func TestPricingAvailabilityRemoveFromSaleRequiresConfirmWithUsageExit(t *testing.T) {
	stdout, stderr := captureOutput(t, func() {
		if code := rootcmd.Run([]string{"pricing", "availability", "remove-from-sale", "--app", "app-1"}, "1.2.3"); code != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected confirmation diagnostic, got %q", stderr)
	}
}

func TestPricingAvailabilityRemoveFromSaleOutputFormats(t *testing.T) {
	setupAuth(t)
	fixture := handlertest.New(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return singlePlatformAppStoreVersionsResponse(), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appAvailabilityV2":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":false}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v2/appAvailabilities/availability-1/territoryAvailabilities":
			return territoryAvailabilityResponse(fixture, map[string]bool{"USA": false, "FRA": false}), nil
		default:
			return nil, fixture.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})

	tests := []struct {
		format string
		want   string
	}{
		{format: "table", want: "Availability ID"},
		{format: "markdown", want: "| Field"},
	}
	for _, test := range tests {
		t.Run(test.format, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, _ := captureOutput(t, func() {
				if err := root.Parse([]string{"pricing", "availability", "remove-from-sale", "--app", "app-1", "--confirm", "--output", test.format}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})
			if !strings.Contains(stdout, test.want) {
				t.Fatalf("%s output missing %q: %s", test.format, test.want, stdout)
			}
			if !strings.Contains(stdout, "Platform listings verified") {
				t.Fatalf("%s output missing platform verification field: %s", test.format, stdout)
			}
		})
	}
}

func TestPricingAvailabilityRemoveFromSaleNoOp(t *testing.T) {
	setupAuth(t)
	fixture := handlertest.New(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	var patches atomic.Int32

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return singlePlatformAppStoreVersionsResponse(), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appAvailabilityV2":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":false}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v2/appAvailabilities/availability-1/territoryAvailabilities":
			return territoryAvailabilityResponse(fixture, map[string]bool{"USA": false, "FRA": false}), nil
		case req.Method == http.MethodPatch:
			patches.Add(1)
			return nil, fixture.Errorf("no-op availability should not be patched: %s", req.URL.Path)
		default:
			return nil, fixture.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"pricing", "availability", "remove-from-sale", "--app", "app-1", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if got := patches.Load(); got != 0 {
		t.Fatalf("PATCH count = %d, want 0", got)
	}
	if !strings.Contains(stdout, `"updatedTerritories":0`) || !strings.Contains(stdout, `"alreadyUnavailableTerritories":2`) {
		t.Fatalf("unexpected no-op result: %s", stdout)
	}
}

func TestPricingAvailabilityRemoveFromSaleContinuesAfterPartialFailure(t *testing.T) {
	setupAuth(t)
	fixture := handlertest.New(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var mu sync.Mutex
	states := map[string]bool{"USA": true, "FRA": true}
	var patches atomic.Int32
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return singlePlatformAppStoreVersionsResponse(), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appAvailabilityV2":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":false}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v2/appAvailabilities/availability-1/territoryAvailabilities":
			mu.Lock()
			defer mu.Unlock()
			return territoryAvailabilityResponse(fixture, states), nil
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/territoryAvailabilities/ta-"):
			patches.Add(1)
			territory := strings.ToUpper(strings.TrimPrefix(req.URL.Path, "/v1/territoryAvailabilities/ta-"))
			if territory == "FRA" {
				return jsonHTTPResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR","title":"Invalid"}]}`), nil
			}
			mu.Lock()
			states[territory] = false
			mu.Unlock()
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"territoryAvailabilities","id":"ta-usa","attributes":{"available":false}}}`), nil
		default:
			return nil, fixture.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"pricing", "availability", "remove-from-sale", "--app", "app-1", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected partial failure")
		}
		if !strings.Contains(err.Error(), "updated 1, skipped 0, failed 1 (FRA)") {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitError {
			t.Fatalf("exit code = %d, want %d", got, rootcmd.ExitError)
		}
	})
	if got := patches.Load(); got != 2 {
		t.Fatalf("PATCH count = %d, want 2", got)
	}
	var result struct {
		Status                         string   `json:"status"`
		TotalTerritories               int      `json:"totalTerritories"`
		UpdatedTerritories             int      `json:"updatedTerritories"`
		VerifiedUnavailableTerritories int      `json:"verifiedUnavailableTerritories"`
		FailedTerritories              []string `json:"failedTerritories"`
		RemovedPlatformListings        []struct {
			Platform string `json:"platform"`
		} `json:"removedPlatformListings"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode partial-failure result %q: %v", stdout, err)
	}
	if result.Status != "partialFailure" ||
		result.TotalTerritories != 2 ||
		result.UpdatedTerritories != 1 ||
		result.VerifiedUnavailableTerritories != 1 ||
		len(result.FailedTerritories) != 1 ||
		result.FailedTerritories[0] != "FRA" {
		t.Fatalf("unexpected partial-failure result: %+v", result)
	}
	if len(result.RemovedPlatformListings) != 0 {
		t.Fatalf("partial failure claimed removed platform listings: %+v", result.RemovedPlatformListings)
	}
}

func TestPricingAvailabilityRemoveFromSaleFinalReadbackIncludesInitiallyUnavailableTerritories(t *testing.T) {
	setupAuth(t)
	fixture := handlertest.New(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	var territoryReads atomic.Int32
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return singlePlatformAppStoreVersionsResponse(), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appAvailabilityV2":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":false}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v2/appAvailabilities/availability-1/territoryAvailabilities":
			if territoryReads.Add(1) == 1 {
				return territoryAvailabilityResponse(fixture, map[string]bool{"USA": true, "FRA": false}), nil
			}
			return territoryAvailabilityResponse(fixture, map[string]bool{"USA": false, "FRA": true}), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/territoryAvailabilities/ta-usa":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"territoryAvailabilities","id":"ta-usa","attributes":{"available":false}}}`), nil
		default:
			return nil, fixture.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"pricing", "availability", "remove-from-sale", "--app", "app-1", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected final-readback failure")
		}
		if !strings.Contains(err.Error(), "updated 1, skipped 1, failed 1 (FRA)") || !strings.Contains(err.Error(), "state changed during verification") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func territoryAvailabilityResponse(fixture *handlertest.Asserter, states map[string]bool) *http.Response {
	territories := []string{"USA", "FRA"}
	data := make([]map[string]any, 0, len(territories))
	for _, territory := range territories {
		data = append(data, map[string]any{
			"type":       "territoryAvailabilities",
			"id":         "ta-" + strings.ToLower(territory),
			"attributes": map[string]any{"available": states[territory]},
			"relationships": map[string]any{
				"territory": map[string]any{"data": map[string]any{"type": "territories", "id": territory}},
			},
		})
	}
	body, err := json.Marshal(map[string]any{"data": data, "links": map[string]string{"next": ""}})
	if err != nil {
		return fixture.Response("marshal territory response: %v", err)
	}
	return jsonHTTPResponse(http.StatusOK, string(body))
}

func singlePlatformAppStoreVersionsResponse() *http.Response {
	return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-ios","attributes":{"platform":"IOS","versionString":"1.0","appStoreState":"READY_FOR_SALE","appVersionState":"READY_FOR_DISTRIBUTION","createdDate":"2026-01-01T00:00:00Z"}}],"links":{}}`)
}

func multiPlatformAppStoreVersionsResponse() *http.Response {
	return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-ios","attributes":{"platform":"IOS","versionString":"4.1.0","appStoreState":"READY_FOR_SALE","appVersionState":"READY_FOR_DISTRIBUTION","createdDate":"2026-07-14T00:00:00Z"}},{"type":"appStoreVersions","id":"ver-vision","attributes":{"platform":"VISION_OS","versionString":"1.3.1","appStoreState":"PREORDER_READY_FOR_SALE","appVersionState":"PREORDER_READY_FOR_SALE","createdDate":"2024-07-06T00:00:00Z"}}],"links":{}}`)
}

func unknownPlatformStateAppStoreVersionsResponse() *http.Response {
	return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-ios","attributes":{"platform":"IOS","versionString":"4.1.0","appStoreState":"FUTURE_SALE_STATE","createdDate":"2026-07-14T00:00:00Z"}}],"links":{}}`)
}

func TestPricingAvailabilityRemoveFromSaleRequiresAllPlatformsWhenMultiplePlatformsLive(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var patches atomic.Int32
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return multiPlatformAppStoreVersionsResponse(), nil
		case req.Method == http.MethodPatch:
			patches.Add(1)
			return nil, fmt.Errorf("unexpected PATCH: %s", req.URL.String())
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})

	stdout, stderr := captureOutput(t, func() {
		if code := rootcmd.Run([]string{"pricing", "availability", "remove-from-sale", "--app", "app-1", "--confirm"}, "1.2.3"); code != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
		}
	})
	if got := patches.Load(); got != 0 {
		t.Fatalf("PATCH count = %d, want 0", got)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, want := range []string{"IOS 4.1.0", "VISION_OS 1.3.1", "--all-platforms", "App Store Connect support"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing %q, got %q", want, stderr)
		}
	}
}

func TestPricingAvailabilityRemoveFromSaleFailsClosedWhenPlatformsCannotBeVerified(t *testing.T) {
	tests := []struct {
		name     string
		response func() *http.Response
		wantErr  []string
	}{
		{
			name: "platform lookup fails",
			response: func() *http.Response {
				return jsonHTTPResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden"}]}`)
			},
			wantErr: []string{"could not verify live platform listings", "--all-platforms", "Forbidden"},
		},
		{
			name:     "platform state is unknown",
			response: unknownPlatformStateAppStoreVersionsResponse,
			wantErr:  []string{"could not verify live platform listings", "FUTURE_SALE_STATE", "--all-platforms"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })

			var requests atomic.Int32
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests.Add(1)
				if req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions" {
					return test.response(), nil
				}
				return nil, fmt.Errorf("unexpected request after failed platform verification: %s %s", req.Method, req.URL.String())
			})

			stdout, _, err := runRootCommand(t, []string{"pricing", "availability", "remove-from-sale", "--app", "app-1", "--confirm"})
			if err == nil {
				t.Fatal("expected platform verification failure")
			}
			for _, want := range test.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error missing %q: %v", want, err)
				}
			}
			if stdout != "" {
				t.Fatalf("expected no stdout before mutation, got %q", stdout)
			}
			if got := requests.Load(); got != 1 {
				t.Fatalf("request count = %d, want only the platform lookup", got)
			}
		})
	}
}

func TestPricingAvailabilityRemoveFromSaleAllPlatformsAcknowledged(t *testing.T) {
	setupAuth(t)
	fixture := handlertest.New(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var mu sync.Mutex
	states := map[string]bool{"USA": true, "FRA": false}
	var patches atomic.Int32
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return multiPlatformAppStoreVersionsResponse(), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appAvailabilityV2":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":false}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v2/appAvailabilities/availability-1/territoryAvailabilities":
			mu.Lock()
			defer mu.Unlock()
			return territoryAvailabilityResponse(fixture, states), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/territoryAvailabilities/ta-usa":
			patches.Add(1)
			mu.Lock()
			states["USA"] = false
			mu.Unlock()
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"territoryAvailabilities","id":"ta-usa","attributes":{"available":false}}}`), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"pricing", "availability", "remove-from-sale", "--app", "app-1", "--confirm", "--all-platforms", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if got := patches.Load(); got != 1 {
		t.Fatalf("PATCH count = %d, want 1", got)
	}
	var result struct {
		Status                    string `json:"status"`
		PlatformListingsVerified  bool   `json:"platformListingsVerified"`
		PlatformListingsVerifyErr string `json:"platformListingsVerificationError"`
		RemovedPlatformListings   []struct {
			Platform      string `json:"platform"`
			VersionString string `json:"versionString"`
			Live          bool   `json:"live"`
		} `json:"removedPlatformListings"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v (%q)", err, stdout)
	}
	if result.Status != "removedFromSale" || !result.PlatformListingsVerified || result.PlatformListingsVerifyErr != "" {
		t.Fatalf("status = %q, want removedFromSale", result.Status)
	}
	if len(result.RemovedPlatformListings) != 2 {
		t.Fatalf("removedPlatformListings = %d, want 2", len(result.RemovedPlatformListings))
	}
	wantListings := []struct {
		platform string
		version  string
		live     bool
	}{
		{platform: "IOS", version: "4.1.0", live: true},
		{platform: "VISION_OS", version: "1.3.1", live: true},
	}
	for i, want := range wantListings {
		got := result.RemovedPlatformListings[i]
		if got.Platform != want.platform || got.VersionString != want.version || got.Live != want.live {
			t.Fatalf("unexpected removed listing at index %d: %+v", i, got)
		}
	}
}

func TestPricingAvailabilityRemoveFromSaleAllPlatformsAcknowledgedReportsUnverifiedListings(t *testing.T) {
	setupAuth(t)
	fixture := handlertest.New(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	states := map[string]bool{"USA": true}
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return unknownPlatformStateAppStoreVersionsResponse(), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appAvailabilityV2":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":false}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v2/appAvailabilities/availability-1/territoryAvailabilities":
			return territoryAvailabilityResponse(fixture, states), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/territoryAvailabilities/ta-usa":
			states["USA"] = false
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"territoryAvailabilities","id":"ta-usa","attributes":{"available":false}}}`), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"pricing", "availability", "remove-from-sale", "--app", "app-1", "--confirm", "--all-platforms", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	var result struct {
		Status                   string `json:"status"`
		PlatformListingsVerified bool   `json:"platformListingsVerified"`
		VerificationError        string `json:"platformListingsVerificationError"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v (%q)", err, stdout)
	}
	if result.Status != "removedFromSale" || result.PlatformListingsVerified || !strings.Contains(result.VerificationError, "FUTURE_SALE_STATE") {
		t.Fatalf("unexpected verification receipt: %+v", result)
	}
}

func TestPricingAvailabilityPlatformsPreservesUnverifiableListingVisibility(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions" {
			return unknownPlatformStateAppStoreVersionsResponse(), nil
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"pricing", "availability", "platforms", "--app", "app-1", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	var result struct {
		Platforms []struct {
			Platform   string `json:"platform"`
			State      string `json:"state"`
			Live       bool   `json:"live"`
			StateKnown bool   `json:"stateKnown"`
		} `json:"platforms"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v (%q)", err, stdout)
	}
	if len(result.Platforms) != 1 || result.Platforms[0].Platform != "IOS" || result.Platforms[0].State != "FUTURE_SALE_STATE" || result.Platforms[0].Live || result.Platforms[0].StateKnown {
		t.Fatalf("unexpected unverifiable platform listing: %+v", result.Platforms)
	}
}

func TestPricingAvailabilityPlatformsPrefersLiveListingOverNewerNonLive(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-rejected","attributes":{"platform":"IOS","versionString":"2.2.1","appStoreState":"REJECTED","appVersionState":"REJECTED","createdDate":"2026-02-01T00:00:00Z"}},{"type":"appStoreVersions","id":"ver-live","attributes":{"platform":"IOS","versionString":"2.2.0","appStoreState":"READY_FOR_SALE","appVersionState":"READY_FOR_DISTRIBUTION","createdDate":"2025-09-29T00:00:00Z"}},{"type":"appStoreVersions","id":"ver-mac","attributes":{"platform":"MAC_OS","versionString":"1.0","appStoreState":"PREPARE_FOR_SUBMISSION","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2025-01-01T00:00:00Z"}}],"links":{}}`), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"pricing", "availability", "platforms", "--app", "app-1", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	var result struct {
		AppID     string `json:"appId"`
		Platforms []struct {
			Platform      string `json:"platform"`
			VersionString string `json:"versionString"`
			State         string `json:"state"`
			Live          bool   `json:"live"`
		} `json:"platforms"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v (%q)", err, stdout)
	}
	if len(result.Platforms) != 2 {
		t.Fatalf("platforms = %d, want 2", len(result.Platforms))
	}
	ios := result.Platforms[0]
	if ios.Platform != "IOS" || ios.VersionString != "2.2.0" || !ios.Live || ios.State != "READY_FOR_DISTRIBUTION" {
		t.Fatalf("unexpected iOS listing: %+v", ios)
	}
	mac := result.Platforms[1]
	if mac.Platform != "MAC_OS" || mac.VersionString != "1.0" || mac.Live {
		t.Fatalf("unexpected macOS listing: %+v", mac)
	}
}
