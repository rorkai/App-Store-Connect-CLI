package cmd

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

type subscriptionPricingRun struct {
	exitCode int
	stdout   string
	stderr   string
	context  telemetry.EventContext
}

func runSubscriptionPricingCommand(t *testing.T, handler http.HandlerFunc, args ...string) subscriptionPricingRun {
	t.Helper()
	resetReportFlags(t)
	t.Setenv("ASC_APP_ID", "")

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := newHTTPStatusTestClient(t, server.URL)
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var run subscriptionPricingRun
	emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, eventContext telemetry.EventContext) {
		run.context = eventContext
	}

	run.stdout, run.stderr = captureCommandOutput(t, func() {
		run.exitCode = Run(args, "4.0.0")
	})
	return run
}

const equalizeTestPricePointUSA = "pp-usa"

func serveEqualizePreflight(w http.ResponseWriter, req *http.Request, availableTerritories string) bool {
	switch {
	case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/subscriptionAvailability":
		fmt.Fprint(w, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1"}}`)
	case req.Method == http.MethodGet && req.URL.Path == "/v1/territories":
		fmt.Fprint(w, `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"CAN"}],"links":{}}`)
	case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAvailabilities/avail-1/availableTerritories":
		fmt.Fprint(w, availableTerritories)
	case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/pricePoints":
		fmt.Fprint(w, `{"data":[{"type":"subscriptionPricePoints","id":"`+equalizeTestPricePointUSA+`","attributes":{"customerPrice":"0.99"}}],"links":{}}`)
	default:
		return false
	}
	return true
}

const equalizeAllTerritoriesAvailable = `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"CAN"}],"links":{}}`

func TestRunSubscriptionsPricingEqualizePartialFailureKeepsConflictCause(t *testing.T) {
	run := runSubscriptionPricingCommand(
		t, func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if serveEqualizePreflight(w, req, equalizeAllTerritoriesAvailable) {
				return
			}
			switch {
			case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionPricePoints/"+equalizeTestPricePointUSA+"/equalizations":
				fmt.Fprint(w, `{"data":[{"type":"subscriptionPricePoints","id":"pp-can","attributes":{"customerPrice":"1.29"},"relationships":{"territory":{"data":{"type":"territories","id":"CAN"}}}}],"links":{}}`)
			case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/relationships/prices":
				fmt.Fprint(w, `{"data":[{"type":"subscriptionPrices","id":"price-existing"}],"links":{}}`)
			case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/prices":
				fmt.Fprint(w, `{"data":[{"type":"subscriptionPrices","id":"price-usa","attributes":{"startDate":"2025-01-01"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"`+equalizeTestPricePointUSA+`"}}}}],"included":[{"type":"subscriptionPricePoints","id":"`+equalizeTestPricePointUSA+`","attributes":{"customerPrice":"0.99"}},{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],"links":{}}`)
			case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices":
				body, _ := io.ReadAll(req.Body)
				if strings.Contains(string(body), `"id":"CAN"`) {
					w.WriteHeader(http.StatusConflict)
					fmt.Fprint(w, `{"errors":[{"status":"409","code":"ENTITY_ERROR","title":"conflict","detail":"price change already scheduled"}]}`)
					return
				}
				w.WriteHeader(http.StatusCreated)
				fmt.Fprint(w, `{"data":{"type":"subscriptionPrices","id":"price-usa"}}`)
			default:
				t.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
				w.WriteHeader(http.StatusInternalServerError)
			}
		},
		"subscriptions", "pricing", "equalize",
		"--subscription-id", "8000000001",
		"--base-price", "0.99",
		"--auto-start-date=false",
		"--confirm",
		"--output", "table",
	)

	if run.exitCode != ExitConflict {
		t.Fatalf("exit code = %d, want %d; stderr=%q", run.exitCode, ExitConflict, run.stderr)
	}
	if !strings.Contains(run.stderr, "Done: 1 succeeded, 1 failed") {
		t.Fatalf("stderr = %q, want summary line", run.stderr)
	}
	if !strings.Contains(run.stdout, "price change already scheduled") {
		t.Fatalf("stdout = %q, want failures table with API detail", run.stdout)
	}
	if run.context.OutcomeKind != telemetry.OutcomeConflict || run.context.HTTPStatus != http.StatusConflict {
		t.Fatalf("telemetry context = %+v, want conflict outcome with HTTP 409", run.context)
	}
}

func TestRunSubscriptionsPricingEqualizeMissingAvailabilityIsStateNotReady(t *testing.T) {
	run := runSubscriptionPricingCommand(
		t, func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if serveEqualizePreflight(w, req, `{"data":[{"type":"territories","id":"USA"}],"links":{}}`) {
				return
			}
			t.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		},
		"subscriptions", "pricing", "equalize",
		"--subscription-id", "8000000001",
		"--base-price", "0.99",
		"--dry-run",
	)

	if run.exitCode != ExitError {
		t.Fatalf("exit code = %d, want %d; stderr=%q", run.exitCode, ExitError, run.stderr)
	}
	if !strings.Contains(run.stderr, "Error: equalize: subscription availability is missing 1 equalized territory (CAN)") ||
		!strings.Contains(run.stderr, "asc subscriptions pricing availability edit") {
		t.Fatalf("stderr = %q, want missing territory guidance", run.stderr)
	}
	if run.context.OutcomeKind != telemetry.OutcomeExpectedNegative ||
		run.context.DiagnosticCode != string(shared.DiagnosticStateNotReady) ||
		run.context.FailureParameter != "--subscription-id" {
		t.Fatalf("telemetry context = %+v, want expected_negative state_not_ready on --subscription-id", run.context)
	}
}

func TestRunSubscriptionsPricingEqualizeUnknownBasePriceIsInvalidInput(t *testing.T) {
	run := runSubscriptionPricingCommand(
		t, func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if serveEqualizePreflight(w, req, equalizeAllTerritoriesAvailable) {
				return
			}
			t.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		},
		"subscriptions", "pricing", "equalize",
		"--subscription-id", "8000000001",
		"--base-price", "3.48",
		"--dry-run",
	)

	if run.exitCode != ExitError {
		t.Fatalf("exit code = %d, want %d; stderr=%q", run.exitCode, ExitError, run.stderr)
	}
	wantHint := "asc subscriptions pricing price-points list --subscription-id \"8000000001\" --territory USA --paginate"
	if !strings.Contains(run.stderr, "Error: equalize: no price point found for USA 3.48") || !strings.Contains(run.stderr, wantHint) {
		t.Fatalf("stderr = %q, want price point error with %q", run.stderr, wantHint)
	}
	if run.context.OutcomeKind != telemetry.OutcomeExpectedNegative ||
		run.context.DiagnosticCode != string(shared.DiagnosticInvalidInput) ||
		run.context.FailureParameter != "--base-price" {
		t.Fatalf("telemetry context = %+v, want expected_negative invalid_input on --base-price", run.context)
	}
}

func TestRunSubscriptionsPricesImportRowFailureClassification(t *testing.T) {
	tests := []struct {
		name        string
		csv         string
		wantExit    int
		wantOutcome telemetry.OutcomeKind
		wantCode    string
		wantParam   string
	}{
		{
			name:        "input errors only",
			csv:         "territory,price\nUSA,$0.99\nUSA,3.48\n",
			wantExit:    ExitError,
			wantOutcome: telemetry.OutcomeExpectedNegative,
			wantCode:    string(shared.DiagnosticInvalidInput),
			wantParam:   "--input",
		},
		{
			name:        "api conflict after input error",
			csv:         "territory,price\nUSA,$0.99\nUSA,0.99\n",
			wantExit:    ExitConflict,
			wantOutcome: telemetry.OutcomeConflict,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			csvPath := filepath.Join(dir, "prices.csv")
			if err := os.WriteFile(csvPath, []byte(test.csv), 0o600); err != nil {
				t.Fatalf("WriteFile() error: %v", err)
			}

			run := runSubscriptionPricingCommand(
				t, func(w http.ResponseWriter, req *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch {
					case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/prices":
						fmt.Fprint(w, `{"data":[],"links":{}}`)
					case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/pricePoints":
						fmt.Fprint(w, `{"data":[{"type":"subscriptionPricePoints","id":"pp-usa","attributes":{"customerPrice":"0.99"}}],"links":{}}`)
					case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices":
						w.WriteHeader(http.StatusConflict)
						fmt.Fprint(w, `{"errors":[{"status":"409","code":"ENTITY_ERROR","title":"conflict","detail":"price change already scheduled"}]}`)
					default:
						t.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
						w.WriteHeader(http.StatusInternalServerError)
					}
				},
				"subscriptions", "pricing", "prices", "import",
				"--subscription-id", "8000000001",
				"--input", csvPath,
				"--confirm",
				"--output", "table",
			)

			if run.exitCode != test.wantExit {
				t.Fatalf("exit code = %d, want %d; stderr=%q", run.exitCode, test.wantExit, run.stderr)
			}
			if !strings.Contains(run.stdout, `price "$0.99" is not a valid numeric value`) {
				t.Fatalf("stdout = %q, want row failure table", run.stdout)
			}
			if run.context.OutcomeKind != test.wantOutcome ||
				run.context.DiagnosticCode != test.wantCode ||
				run.context.FailureParameter != test.wantParam {
				t.Fatalf("telemetry context = %+v, want outcome %s code %q param %q", run.context, test.wantOutcome, test.wantCode, test.wantParam)
			}
		})
	}
}
