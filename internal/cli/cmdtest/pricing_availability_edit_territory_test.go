package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func TestPricingAvailabilityEditNormalizesTerritories(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var patchedMu sync.Mutex
	patched := map[string]bool{}
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appAvailabilityV2":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":false}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v2/appAvailabilities/availability-1/territoryAvailabilities":
			patchedMu.Lock()
			usAvailable := patched["ta-us"]
			frAvailable := patched["ta-fr"]
			patchedMu.Unlock()
			return jsonHTTPResponse(http.StatusOK, fmt.Sprintf(`{
				"data":[
					{"type":"territoryAvailabilities","id":"ta-us","attributes":{"available":%t},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}},
					{"type":"territoryAvailabilities","id":"ta-fr","attributes":{"available":%t},"relationships":{"territory":{"data":{"type":"territories","id":"FRA"}}}}
				],
				"links":{"next":""}
			}`, usAvailable, frAvailable)), nil
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/territoryAvailabilities/"):
			patchedMu.Lock()
			patched[strings.TrimPrefix(req.URL.Path, "/v1/territoryAvailabilities/")] = true
			patchedMu.Unlock()
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"territoryAvailabilities","id":"patched","attributes":{"available":true}}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"pricing", "availability", "edit",
			"--app", "app-1",
			"--territory", "US,France",
			"--available", "true",
			"--available-in-new-territories", "false",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" && !strings.Contains(stderr, "Updating availability for") {
		t.Fatalf("unexpected stderr %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected command output")
	}
	patchedMu.Lock()
	defer patchedMu.Unlock()
	if !patched["ta-us"] || !patched["ta-fr"] {
		t.Fatalf("expected normalized territories to be patched, got %#v", patched)
	}
}

func TestPricingAvailabilityEditSkipsNoOpWithoutNewTerritoriesGuard(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	patches := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appAvailabilityV2":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":true}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v2/appAvailabilities/availability-1/territoryAvailabilities":
			return jsonHTTPResponse(http.StatusOK, `{
				"data":[
					{"type":"territoryAvailabilities","id":"ta-us","attributes":{"available":true},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}
				],
				"links":{"next":""}
			}`), nil
		case req.Method == http.MethodPatch:
			patches++
			t.Fatalf("no-op territory should not be patched: %s", req.URL.Path)
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"pricing", "availability", "edit",
			"--app", "app-1",
			"--territory", "US",
			"--available", "true",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if patches != 0 {
		t.Fatalf("expected no PATCH requests, got %d", patches)
	}
	if stdout == "" {
		t.Fatal("expected command output")
	}
	if !strings.Contains(stderr, "Updated 0 territories; 1 already matched") {
		t.Fatalf("expected no-op summary, got stderr %q", stderr)
	}
}

func TestPricingAvailabilityEditContinuesAfterFailureAndVerifiesFinalState(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var mu sync.Mutex
	states := map[string]bool{"USA": false, "FRA": false, "DEU": false}
	patched := map[string]int{}

	territoryResponse := func() *http.Response {
		mu.Lock()
		defer mu.Unlock()
		data := make([]map[string]any, 0, 3)
		for _, territoryID := range []string{"USA", "FRA", "DEU"} {
			data = append(data, map[string]any{
				"type":       "territoryAvailabilities",
				"id":         "ta-" + strings.ToLower(territoryID),
				"attributes": map[string]any{"available": states[territoryID]},
				"relationships": map[string]any{
					"territory": map[string]any{"data": map[string]any{"type": "territories", "id": territoryID}},
				},
			})
		}
		body, err := json.Marshal(map[string]any{"data": data, "links": map[string]string{"next": ""}})
		if err != nil {
			t.Fatalf("marshal territory response: %v", err)
		}
		return jsonHTTPResponse(http.StatusOK, string(body))
	}

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appAvailabilityV2":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":false}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v2/appAvailabilities/availability-1/territoryAvailabilities":
			return territoryResponse(), nil
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/territoryAvailabilities/ta-"):
			territoryID := strings.ToUpper(strings.TrimPrefix(req.URL.Path, "/v1/territoryAvailabilities/ta-"))
			mu.Lock()
			patched[territoryID]++
			if territoryID != "FRA" {
				states[territoryID] = true
			}
			mu.Unlock()
			if territoryID == "FRA" {
				return jsonHTTPResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR","title":"Invalid"}]}`), nil
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"territoryAvailabilities","id":"patched","attributes":{"available":true}}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{
			"pricing", "availability", "edit",
			"--app", "app-1",
			"--all-territories",
			"--available", "true",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected partial failure")
		}
		if !strings.Contains(err.Error(), "updated 2, skipped 0, failed 1 (FRA)") {
			t.Fatalf("expected verified partial-failure summary, got %v", err)
		}
	})

	mu.Lock()
	defer mu.Unlock()
	for _, territoryID := range []string{"USA", "FRA", "DEU"} {
		if patched[territoryID] != 1 {
			t.Fatalf("expected one PATCH for %s, got %#v", territoryID, patched)
		}
	}
}
