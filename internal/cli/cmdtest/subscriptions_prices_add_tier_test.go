package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func subscriptionPriceListFixture(pricePointID, territoryID, attrs string) string {
	if strings.TrimSpace(attrs) == "" {
		attrs = "{}"
	}

	territoryRelationship := ""
	if territoryID != "" {
		territoryRelationship = fmt.Sprintf(
			`,"territory":{"data":{"type":"territories","id":%q}}`,
			territoryID,
		)
	}

	return fmt.Sprintf(
		`{"data":[{"type":"subscriptionPrices","id":%q,"attributes":%s,"relationships":{"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":%q}}%s}}],"links":{"next":""}}`,
		"existing-price-1",
		attrs,
		pricePointID,
		territoryRelationship,
	)
}

func TestSubscriptionsPricesAdd_TierAndPricePointMutualExclusion(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "set",
			"--subscription-id", "8000000003",
			"--price-point", "PP",
			"--tier", "5",
			"--territory", "USA",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("expected mutually exclusive error, got %q", stderr)
	}
}

func TestSubscriptionsPricesAdd_TierUsesSubscriptionPricePoints(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var resolvedPricePointID string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/relationships/prices"):
			body := `{"data":[{"type":"subscriptionPrices","id":"existing-price-1"}],"links":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/subscriptions/8000000003/pricePoints"):
			body := `{
				"data":[
					{"type":"subscriptionPricePoints","id":"sub-pp-1","attributes":{"customerPrice":"0.99","proceeds":"0.70"}},
					{"type":"subscriptionPricePoints","id":"sub-pp-2","attributes":{"customerPrice":"1.99","proceeds":"1.40"}},
					{"type":"subscriptionPricePoints","id":"sub-pp-3","attributes":{"customerPrice":"2.99","proceeds":"2.10"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/apps/"):
			t.Fatalf("unexpected app price points request: %s", req.URL.Path)
			return nil, nil
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/prices"):
			body := subscriptionPriceListFixture("sub-pp-1", "USA", "")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/subscriptionPrices"):
			bodyBytes, _ := io.ReadAll(req.Body)
			bodyStr := string(bodyBytes)
			if strings.Contains(bodyStr, "sub-pp-2") {
				resolvedPricePointID = "sub-pp-2"
			}
			resp := `{"data":{"type":"subscriptionPrices","id":"sub-price-1","attributes":{}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(resp)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	t.Setenv("HOME", t.TempDir())
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "set",
			"--subscription-id", "8000000003",
			"--tier", "2",
			"--territory", "USA",
			"--refresh",
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
	if resolvedPricePointID != "sub-pp-2" {
		t.Fatalf("expected tier 2 to resolve sub-pp-2, got %q", resolvedPricePointID)
	}
	if !strings.Contains(stdout, `"id":"sub-price-1"`) {
		t.Fatalf("expected create output, got %q", stdout)
	}
}

func TestSubscriptionsPricesAdd_NormalizesTerritory(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/relationships/prices"):
			body := `{"data":[{"type":"subscriptionPrices","id":"existing-price-1"}],"links":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/subscriptions/8000000003/pricePoints"):
			if got := req.URL.Query().Get("filter[territory]"); got != "FRA" {
				t.Fatalf("expected normalized filter[territory]=FRA, got %q", got)
			}
			body := `{
				"data":[
					{"type":"subscriptionPricePoints","id":"sub-pp-2","attributes":{"customerPrice":"1.99","proceeds":"1.40"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/prices"):
			body := subscriptionPriceListFixture("different-pp", "FRA", "")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/subscriptionPrices"):
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(`{"data":{"type":"subscriptionPrices","id":"sub-price-1","attributes":{}}}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	t.Setenv("HOME", t.TempDir())
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if err := root.Parse([]string{
		"subscriptions", "pricing", "prices", "set",
		"--subscription-id", "8000000003",
		"--tier", "1",
		"--territory", "France",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
}

func TestSubscriptionsPricesAdd_ProbeErrorReturnsWithoutWrite(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/relationships/prices"):
			body := `{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","title":"An unexpected error occurred.","detail":"An unexpected error occurred on the server side."}]}`
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodPatch && strings.Contains(req.URL.Path, "/v1/subscriptions/"):
			t.Fatalf("unexpected PATCH write request after failed probe: %s", req.URL.Path)
			return nil, nil
		case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/subscriptionPrices"):
			t.Fatalf("unexpected POST write request after failed probe: %s", req.URL.Path)
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	t.Setenv("HOME", t.TempDir())
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "set",
			"--subscription-id", "8000000003",
			"--price-point", "PP_ID",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected command to fail when prices probe fails")
		}
		if !strings.Contains(err.Error(), "failed to check existing prices") {
			t.Fatalf("expected probe error message, got %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestSubscriptionsPricesAdd_InitialPriceForwardsAttributes(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/relationships/prices"):
			body := `{"data":[],"links":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodPatch && strings.Contains(req.URL.Path, "/v1/subscriptions/8000000003"):
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("failed to decode patch payload: %v", err)
			}

			includedAny, ok := payload["included"].([]any)
			if !ok || len(includedAny) != 1 {
				t.Fatalf("expected one included resource, got %#v", payload["included"])
			}
			included, ok := includedAny[0].(map[string]any)
			if !ok {
				t.Fatalf("expected included resource object, got %#v", includedAny[0])
			}

			attrs, ok := included["attributes"].(map[string]any)
			if !ok {
				t.Fatalf("expected included attributes object, got %#v", included["attributes"])
			}
			if attrs["startDate"] != "2026-05-01" {
				t.Fatalf("expected startDate 2026-05-01, got %#v", attrs["startDate"])
			}
			if attrs["preserveCurrentPrice"] != true {
				t.Fatalf("expected preserveCurrentPrice true, got %#v", attrs["preserveCurrentPrice"])
			}

			relationships, ok := included["relationships"].(map[string]any)
			if !ok {
				t.Fatalf("expected included relationships object, got %#v", included["relationships"])
			}
			territory, ok := relationships["territory"].(map[string]any)
			if !ok {
				t.Fatalf("expected territory relationship object, got %#v", relationships["territory"])
			}
			territoryData, ok := territory["data"].(map[string]any)
			if !ok {
				t.Fatalf("expected territory.data object, got %#v", territory["data"])
			}
			if territoryData["id"] != "USA" {
				t.Fatalf("expected territory id USA, got %#v", territoryData["id"])
			}

			resp := `{"data":{"type":"subscriptions","id":"8000000003","attributes":{"name":"Monthly","productId":"com.example.sub.monthly"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(resp)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/subscriptionPrices"):
			t.Fatalf("unexpected POST request; initial pricing should use PATCH flow")
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	t.Setenv("HOME", t.TempDir())
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "set",
			"--subscription-id", "8000000003",
			"--price-point", "PP_ID",
			"--territory", "USA",
			"--start-date", "2026-05-01",
			"--preserved",
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
	if !strings.Contains(stdout, `"id":"8000000003"`) {
		t.Fatalf("expected subscription response in stdout, got %q", stdout)
	}
}

func TestSubscriptionsPricesAdd_ExistingPriceForwardsTerritoryOverridePayload(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/relationships/prices"):
			body := `{"data":[{"type":"subscriptionPrices","id":"existing-price-1"}],"links":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/prices"):
			body := subscriptionPriceListFixture("different-pp", "NOR", `{"startDate":"2026-05-01","preserved":true}`)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices":
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("failed to decode post payload: %v", err)
			}

			data, ok := payload["data"].(map[string]any)
			if !ok {
				t.Fatalf("expected data object, got %#v", payload["data"])
			}
			if got := data["type"]; got != "subscriptionPrices" {
				t.Fatalf("expected type subscriptionPrices, got %#v", got)
			}

			attrs, ok := data["attributes"].(map[string]any)
			if !ok {
				t.Fatalf("expected attributes object, got %#v", data["attributes"])
			}
			if attrs["startDate"] != "2026-05-01" {
				t.Fatalf("expected startDate 2026-05-01, got %#v", attrs["startDate"])
			}
			if attrs["preserveCurrentPrice"] != true {
				t.Fatalf("expected preserveCurrentPrice true, got %#v", attrs["preserveCurrentPrice"])
			}

			relationships, ok := data["relationships"].(map[string]any)
			if !ok {
				t.Fatalf("expected relationships object, got %#v", data["relationships"])
			}
			subscriptionRelationship, ok := relationships["subscription"].(map[string]any)
			if !ok {
				t.Fatalf("expected subscription relationship object, got %#v", relationships["subscription"])
			}
			subscription, ok := subscriptionRelationship["data"].(map[string]any)
			if !ok {
				t.Fatalf("expected subscription relationship data object, got %#v", subscriptionRelationship["data"])
			}
			if subscription["id"] != "8000000003" {
				t.Fatalf("expected subscription id 8000000003, got %#v", subscription["id"])
			}
			pricePointRelationship, ok := relationships["subscriptionPricePoint"].(map[string]any)
			if !ok {
				t.Fatalf("expected subscriptionPricePoint relationship object, got %#v", relationships["subscriptionPricePoint"])
			}
			pricePoint, ok := pricePointRelationship["data"].(map[string]any)
			if !ok {
				t.Fatalf("expected subscriptionPricePoint relationship data object, got %#v", pricePointRelationship["data"])
			}
			if pricePoint["id"] != "PP_ID" {
				t.Fatalf("expected price point PP_ID, got %#v", pricePoint["id"])
			}
			territoryRelationship, ok := relationships["territory"].(map[string]any)
			if !ok {
				t.Fatalf("expected territory relationship object, got %#v", relationships["territory"])
			}
			territory, ok := territoryRelationship["data"].(map[string]any)
			if !ok {
				t.Fatalf("expected territory relationship data object, got %#v", territoryRelationship["data"])
			}
			if territory["id"] != "NOR" {
				t.Fatalf("expected territory NOR, got %#v", territory["id"])
			}

			return &http.Response{
				StatusCode: http.StatusCreated,
				Body: io.NopCloser(strings.NewReader(
					`{"data":{"type":"subscriptionPrices","id":"sub-price-1","attributes":{"startDate":"2026-05-01","preserved":true}}}`,
				)),
				Header: http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	t.Setenv("HOME", t.TempDir())
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "set",
			"--subscription-id", "8000000003",
			"--price-point", "PP_ID",
			"--territory", "Norway",
			"--start-date", "2026-05-01",
			"--preserved",
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
	if !strings.Contains(stdout, `"id":"sub-price-1"`) {
		t.Fatalf("expected subscription price response in stdout, got %q", stdout)
	}
}

func TestSubscriptionsPricesAdd_ExistingMatchingPriceSkipsCreate(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var posted bool
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/relationships/prices"):
			body := `{"data":[{"type":"subscriptionPrices","id":"existing-price-1"}],"links":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/prices"):
			query := req.URL.Query()
			if got := query.Get("filter[territory]"); got != "USA" {
				t.Fatalf("expected filter[territory]=USA, got %q", got)
			}
			if got := query.Get("include"); got != "subscriptionPricePoint,territory" {
				t.Fatalf("expected relationship include query, got %q", got)
			}
			body := subscriptionPriceListFixture("PP_ID", "USA", `{"startDate":"2026-05-01","preserved":true}`)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices":
			posted = true
			t.Fatalf("unexpected POST request for already matching price")
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	t.Setenv("HOME", t.TempDir())
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "set",
			"--subscription-id", "8000000003",
			"--price-point", "PP_ID",
			"--territory", "USA",
			"--start-date", "2026-05-01",
			"--preserved",
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
	if posted {
		t.Fatal("did not expect create request")
	}
	if !strings.Contains(stdout, `"id":"existing-price-1"`) {
		t.Fatalf("expected existing subscription price response in stdout, got %q", stdout)
	}
}

func TestSubscriptionsPricesAdd_ForceResavesCompletePriceMatrix(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var patched bool
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/relationships/prices"):
			body := `{"data":[{"type":"subscriptionPrices","id":"existing-price-1"}],"links":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/prices"):
			body := subscriptionPriceListFixture("PP_ID", "USA", `{"planType":"UPFRONT"}`)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionPricePoints/PP_ID/equalizations":
			body := `{"data":[{"type":"subscriptionPricePoints","id":"PP_CAN","attributes":{"customerPrice":"1.49"},"relationships":{"territory":{"data":{"type":"territories","id":"CAN"}}}}],"links":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/subscriptions/8000000003":
			patched = true
			var payload asc.SubscriptionUpdateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode price matrix payload: %v", err)
			}
			if len(payload.Included) != 2 {
				t.Fatalf("expected complete two-territory matrix, got %+v", payload.Included)
			}
			body := `{"data":{"type":"subscriptions","id":"8000000003","attributes":{"state":"READY_TO_SUBMIT"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	t.Setenv("HOME", t.TempDir())
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "set",
			"--subscription-id", "8000000003",
			"--price-point", "PP_ID",
			"--territory", "USA",
			"--force",
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
	if !patched {
		t.Fatal("expected forced atomic price matrix PATCH")
	}
	if !strings.Contains(stdout, `"id":"8000000003"`) {
		t.Fatalf("expected updated subscription response in stdout, got %q", stdout)
	}
}

func TestSubscriptionsPricesAdd_ForceRejectsInvalidBoolean(t *testing.T) {
	assertUsageExit(t, []string{
		"subscriptions", "pricing", "prices", "set",
		"--subscription-id", "8000000003",
		"--price-point", "PP_ID",
		"--force=maybe",
	}, `invalid boolean value "maybe" for -force`)
}

func TestSubscriptionsPricesAdd_TierRequiresTerritory(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "set",
			"--subscription-id", "8000000003",
			"--tier", "5",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--territory is required") {
		t.Fatalf("expected --territory required error, got %q", stderr)
	}
}

func TestSubscriptionsPricesAdd_InvalidPriceValue(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "set",
			"--subscription-id", "8000000003",
			"--price", "abc",
			"--territory", "USA",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--price must be a number") {
		t.Fatalf("expected invalid --price error, got %q", stderr)
	}
}

func TestSubscriptionsPricesAdd_NegativeTier(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "set",
			"--subscription-id", "8000000003",
			"--tier", "-1",
			"--territory", "USA",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--tier must be a positive integer") {
		t.Fatalf("expected invalid --tier error, got %q", stderr)
	}
}

func TestSubscriptionsPricesAddRefreshesContextAfterTierResolution(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_TIMEOUT", "80ms")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var relationshipDeadlineRemaining time.Duration
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/subscriptions/8000000003/pricePoints"):
			time.Sleep(60 * time.Millisecond)
			body := `{
				"data":[
					{"type":"subscriptionPricePoints","id":"sub-pp-1","attributes":{"customerPrice":"0.99","proceeds":"0.70"}},
					{"type":"subscriptionPricePoints","id":"sub-pp-2","attributes":{"customerPrice":"1.99","proceeds":"1.40"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/relationships/prices"):
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected prices relationship request to carry a timeout deadline")
			}
			relationshipDeadlineRemaining = time.Until(deadline)
			if relationshipDeadlineRemaining < 35*time.Millisecond {
				t.Fatalf("expected fresh prices context after tier resolution, got only %v remaining", relationshipDeadlineRemaining)
			}

			body := `{"data":[{"type":"subscriptionPrices","id":"existing-price-1"}],"links":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/prices"):
			resp := subscriptionPriceListFixture("sub-pp-1", "USA", "")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(resp)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/subscriptionPrices"):
			resp := `{"data":{"type":"subscriptionPrices","id":"sub-price-1","attributes":{}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(resp)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	t.Setenv("HOME", t.TempDir())
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "set",
			"--subscription-id", "8000000003",
			"--tier", "2",
			"--territory", "USA",
			"--refresh",
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
	if relationshipDeadlineRemaining == 0 {
		t.Fatal("expected relationship request to run")
	}
	if !strings.Contains(stdout, `"id":"sub-price-1"`) {
		t.Fatalf("expected create output, got %q", stdout)
	}
}
