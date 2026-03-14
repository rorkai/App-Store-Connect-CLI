package cmdtest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSubscriptionsPricingEqualizeValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing subscription id",
			args:    []string{"subscriptions", "pricing", "equalize", "--base-price", "3.49"},
			wantErr: "Error: --subscription-id is required",
		},
		{
			name:    "missing base price",
			args:    []string{"subscriptions", "pricing", "equalize", "--subscription-id", "sub-1"},
			wantErr: "Error: --base-price is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestSubscriptionsPricingEqualize_RequiresConfirmUnlessDryRun(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "equalize",
			"--subscription-id", "sub-1",
			"--base-price", "0.99",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required unless --dry-run is set") {
		t.Fatalf("expected confirm usage error, got %q", stderr)
	}
}

func TestSubscriptionsPricingEqualize_RejectsOutOfRangeWorkers(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "equalize",
			"--subscription-id", "sub-1",
			"--base-price", "0.99",
			"--dry-run",
			"--workers", "0",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestSubscriptionsPricingEqualize_DryRunMatchesBasePriceNumerically(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	basePricePointID := testSubscriptionPricePointID("USA")
	canPricePointID := testSubscriptionPricePointID("CAN")

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/pricePoints":
			if got := req.URL.Query().Get("filter[territory]"); got != "USA" {
				t.Fatalf("expected filter[territory]=USA, got %q", got)
			}
			body := `{"data":[{"type":"subscriptionPricePoints","id":"` + basePricePointID + `","attributes":{"customerPrice":"3.50"}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionPricePoints/"+basePricePointID+"/equalizations":
			if got := req.URL.Query().Get("include"); got != "territory" {
				t.Fatalf("expected include=territory, got %q", got)
			}
			if got := req.URL.Query().Get("fields[subscriptionPricePoints]"); got != "customerPrice,territory" {
				t.Fatalf("expected fields[subscriptionPricePoints]=customerPrice,territory, got %q", got)
			}
			body := `{"data":[{"type":"subscriptionPricePoints","id":"` + canPricePointID + `","attributes":{"customerPrice":"4.49"},"relationships":{"territory":{"data":{"type":"territories","id":"CAN"}}}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "equalize",
			"--subscription-id", "sub-1",
			"--base-price", "3.5",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var result struct {
		Total       int `json:"total"`
		Territories []struct {
			Territory string `json:"territory"`
			Price     string `json:"price"`
		} `json:"territories"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON result: %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("expected total 2 including base territory, got %d", result.Total)
	}
	if len(result.Territories) != 2 {
		t.Fatalf("expected 2 territories, got %d", len(result.Territories))
	}
	if result.Territories[0].Territory != "USA" || result.Territories[0].Price != "3.5" {
		t.Fatalf("expected base territory to be included first, got %+v", result.Territories[0])
	}
	if result.Territories[1].Territory != "CAN" {
		t.Fatalf("expected CAN equalization, got %+v", result.Territories[1])
	}
}

func TestSubscriptionsPricingEqualize_DryRunUsesTerritoryRelationshipForOpaquePricePointIDs(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	basePricePointID := testSubscriptionPricePointID("USA")

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/pricePoints":
			body := `{"data":[{"type":"subscriptionPricePoints","id":"` + basePricePointID + `","attributes":{"customerPrice":"3.50"}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionPricePoints/"+basePricePointID+"/equalizations":
			body := `{"data":[{"type":"subscriptionPricePoints","id":"opaque-eq-1","attributes":{"customerPrice":"4.49"},"relationships":{"territory":{"data":{"type":"territories","id":"CAN"}}}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "equalize",
			"--subscription-id", "sub-1",
			"--base-price", "3.5",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var result struct {
		Territories []struct {
			Territory string `json:"territory"`
			Price     string `json:"price"`
		} `json:"territories"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON result: %v", err)
	}
	if len(result.Territories) != 2 || result.Territories[1].Territory != "CAN" {
		t.Fatalf("expected CAN territory from relationships, got %+v", result.Territories)
	}
}

func TestSubscriptionsPricingEqualize_ApplyFailsWhenAvailabilityIsMissing(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	basePricePointID := testSubscriptionPricePointID("USA")
	canPricePointID := testSubscriptionPricePointID("CAN")

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/pricePoints":
			body := `{"data":[{"type":"subscriptionPricePoints","id":"` + basePricePointID + `","attributes":{"customerPrice":"0.99"}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionPricePoints/"+basePricePointID+"/equalizations":
			body := `{"data":[{"type":"subscriptionPricePoints","id":"` + canPricePointID + `","attributes":{"customerPrice":"1.29"},"relationships":{"territory":{"data":{"type":"territories","id":"CAN"}}}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/subscriptionAvailability":
			return jsonHTTPResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"not found","detail":"missing"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "equalize",
			"--subscription-id", "sub-1",
			"--base-price", "0.99",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected availability preflight to fail")
	}
	if stdout != "" {
		t.Fatalf("expected no stdout on availability preflight failure, got %q", stdout)
	}
	if !strings.Contains(runErr.Error(), "equalize only updates prices and will not change sale availability") {
		t.Fatalf("expected availability guidance in error, got %v", runErr)
	}
}

func TestSubscriptionsPricingEqualize_ApplyFailsWhenAvailabilityDoesNotCoverAllTerritories(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	basePricePointID := testSubscriptionPricePointID("USA")
	canPricePointID := testSubscriptionPricePointID("CAN")

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/pricePoints":
			body := `{"data":[{"type":"subscriptionPricePoints","id":"` + basePricePointID + `","attributes":{"customerPrice":"0.99"}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionPricePoints/"+basePricePointID+"/equalizations":
			body := `{"data":[{"type":"subscriptionPricePoints","id":"` + canPricePointID + `","attributes":{"customerPrice":"1.29"},"relationships":{"territory":{"data":{"type":"territories","id":"CAN"}}}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/subscriptionAvailability":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":false}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAvailabilities/avail-1/availableTerritories":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"}],"links":{}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "equalize",
			"--subscription-id", "sub-1",
			"--base-price", "0.99",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected incomplete availability preflight to fail")
	}
	if stdout != "" {
		t.Fatalf("expected no stdout on incomplete availability preflight failure, got %q", stdout)
	}
	if !strings.Contains(runErr.Error(), "missing 1 equalized territory (CAN)") {
		t.Fatalf("expected missing territory guidance in error, got %v", runErr)
	}
}

func TestSubscriptionsPricingEqualize_InitialPriceUsesPatchThenCreatesRemainingTerritories(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	basePricePointID := testSubscriptionPricePointID("USA")
	canPricePointID := testSubscriptionPricePointID("CAN")

	steps := make([]string, 0, 4)
	patchCount := 0
	postCount := 0

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/pricePoints":
			body := `{"data":[{"type":"subscriptionPricePoints","id":"` + basePricePointID + `","attributes":{"customerPrice":"0.99"}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionPricePoints/"+basePricePointID+"/equalizations":
			body := `{"data":[{"type":"subscriptionPricePoints","id":"` + canPricePointID + `","attributes":{"customerPrice":"1.29"},"relationships":{"territory":{"data":{"type":"territories","id":"CAN"}}}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/subscriptionAvailability":
			steps = append(steps, "availability")
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":false}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAvailabilities/avail-1/availableTerritories":
			steps = append(steps, "territories")
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"CAN"}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/relationships/prices":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/subscriptions/sub-1":
			steps = append(steps, "patch")
			patchCount++
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptions","id":"sub-1"}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices":
			steps = append(steps, "price")
			postCount++
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll() error: %v", err)
			}
			if !strings.Contains(string(body), `"id":"CAN"`) {
				t.Fatalf("expected CAN territory in request body, got %s", string(body))
			}
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionPrices","id":"price-1"}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "equalize",
			"--subscription-id", "sub-1",
			"--base-price", "0.99",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if patchCount != 1 {
		t.Fatalf("expected one initial PATCH, got %d", patchCount)
	}
	if postCount != 1 {
		t.Fatalf("expected one follow-up POST, got %d", postCount)
	}
	if strings.Join(steps, ",") != "availability,territories,patch,price" {
		t.Fatalf("expected availability validation before pricing, got %v", steps)
	}

	var result struct {
		Total     int `json:"total"`
		Succeeded int `json:"succeeded"`
		Failed    int `json:"failed"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON result: %v", err)
	}
	if result.Total != 2 || result.Succeeded != 2 || result.Failed != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestSubscriptionsPricingEqualize_FailedInitialPriceStopsBeforePostingRemainingTerritories(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	basePricePointID := testSubscriptionPricePointID("USA")
	canPricePointID := testSubscriptionPricePointID("CAN")

	patchCount := 0
	postCount := 0

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/pricePoints":
			body := `{"data":[{"type":"subscriptionPricePoints","id":"` + basePricePointID + `","attributes":{"customerPrice":"0.99"}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionPricePoints/"+basePricePointID+"/equalizations":
			body := `{"data":[{"type":"subscriptionPricePoints","id":"` + canPricePointID + `","attributes":{"customerPrice":"1.29"},"relationships":{"territory":{"data":{"type":"territories","id":"CAN"}}}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/subscriptionAvailability":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":false}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAvailabilities/avail-1/availableTerritories":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"CAN"}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/relationships/prices":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/subscriptions/sub-1":
			patchCount++
			return jsonHTTPResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","title":"unprocessable","detail":"failed initial price"}]}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices":
			postCount++
			t.Fatalf("unexpected price create after failed initial patch")
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "equalize",
			"--subscription-id", "sub-1",
			"--base-price", "0.99",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected command to fail")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}
	if patchCount != 1 {
		t.Fatalf("expected one initial PATCH attempt, got %d", patchCount)
	}
	if postCount != 0 {
		t.Fatalf("expected no follow-up POSTs after failed initial PATCH, got %d", postCount)
	}

	var result struct {
		Total     int `json:"total"`
		Succeeded int `json:"succeeded"`
		Failed    int `json:"failed"`
		Failures  []struct {
			Territory string `json:"territory"`
		} `json:"failures"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON result: %v", err)
	}
	if result.Total != 2 || result.Succeeded != 0 || result.Failed != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(result.Failures) != 1 || result.Failures[0].Territory != "USA" {
		t.Fatalf("expected USA initial price failure, got %+v", result.Failures)
	}
}

func TestSubscriptionsPricingEqualize_ReturnsReportedErrorWhenAnyTerritoryFails(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	basePricePointID := testSubscriptionPricePointID("USA")
	canPricePointID := testSubscriptionPricePointID("CAN")

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/pricePoints":
			body := `{"data":[{"type":"subscriptionPricePoints","id":"` + basePricePointID + `","attributes":{"customerPrice":"0.99"}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionPricePoints/"+basePricePointID+"/equalizations":
			body := `{"data":[{"type":"subscriptionPricePoints","id":"` + canPricePointID + `","attributes":{"customerPrice":"1.29"},"relationships":{"territory":{"data":{"type":"territories","id":"CAN"}}}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/subscriptionAvailability":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":true}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAvailabilities/avail-1/availableTerritories":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"CAN"}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/relationships/prices":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionPrices","id":"price-existing"}],"links":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll() error: %v", err)
			}
			if strings.Contains(string(body), `"id":"USA"`) {
				return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionPrices","id":"price-usa"}}`), nil
			}
			return jsonHTTPResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","title":"unprocessable","detail":"bad territory"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "equalize",
			"--subscription-id", "sub-1",
			"--base-price", "0.99",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatalf("expected command to fail")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}

	var result struct {
		Total     int `json:"total"`
		Succeeded int `json:"succeeded"`
		Failed    int `json:"failed"`
		Failures  []struct {
			Territory string `json:"territory"`
		} `json:"failures"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON result: %v", err)
	}
	if result.Total != 2 || result.Succeeded != 1 || result.Failed != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(result.Failures) != 1 || result.Failures[0].Territory != "CAN" {
		t.Fatalf("expected CAN failure, got %+v", result.Failures)
	}
}

func testSubscriptionPricePointID(territory string) string {
	payload, err := json.Marshal(map[string]string{
		"s": "sub-1",
		"t": territory,
		"p": "100010",
	})
	if err != nil {
		panic(err)
	}
	return strings.TrimRight(base64.StdEncoding.EncodeToString(payload), "=")
}

func jsonHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}
