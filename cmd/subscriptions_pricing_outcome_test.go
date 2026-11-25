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
			case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionPriceTiers":
				if query := req.URL.Query(); query.Get("filter[subscription]") != "8000000001" || query.Get("limit") != "1" {
					t.Errorf("unexpected tier lookup query %s", req.URL.RawQuery)
				}
				fmt.Fprint(w, `{"data":[],"links":{}}`)
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

func TestRunSubscriptionsIntroductoryOffersCreatePartialFailureKeepsTerritoryCause(t *testing.T) {
	const rerunHint = "Hint: App Store Connect rate-limited 1 territory; rerun the same command later. Territories that already have an introductory offer are skipped.\n"
	tests := []struct {
		name        string
		status      int
		body        string
		wantExit    int
		wantOutcome telemetry.OutcomeKind
		wantStderr  string
	}{
		{
			name:        "rate limited territory",
			status:      http.StatusTooManyRequests,
			body:        `{"errors":[{"status":"429","code":"RATE_LIMIT_EXCEEDED","title":"The request rate limit has been reached.","detail":"We've received too many requests for this API."}]}`,
			wantExit:    39,
			wantOutcome: telemetry.OutcomeAPIClientError,
			wantStderr:  "Error: subscriptions introductory-offers create: 1 territory failed\n" + rerunHint,
		},
		{
			name:        "rejected territory",
			status:      http.StatusUnprocessableEntity,
			body:        `{"errors":[{"status":"422","code":"ENTITY_ERROR","title":"invalid","detail":"duplicate territory"}]}`,
			wantExit:    ExitHTTPUnprocessable,
			wantOutcome: telemetry.OutcomeAPIClientError,
			wantStderr:  "Error: subscriptions introductory-offers create: 1 territory failed\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_MAX_RETRIES", "0")
			run := runSubscriptionPricingCommand(
				t, func(w http.ResponseWriter, req *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch {
					case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/subscriptionAvailability":
						fmt.Fprint(w, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1"}}`)
					case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAvailabilities/avail-1/availableTerritories":
						fmt.Fprint(w, `{"data":[{"type":"territories","id":"CAN"},{"type":"territories","id":"USA"}],"links":{}}`)
					case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/introductoryOffers":
						fmt.Fprint(w, `{"data":[],"links":{}}`)
					case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionIntroductoryOffers":
						body, _ := io.ReadAll(req.Body)
						if strings.Contains(string(body), `"id":"CAN"`) {
							w.WriteHeader(test.status)
							fmt.Fprint(w, test.body)
							return
						}
						w.WriteHeader(http.StatusCreated)
						fmt.Fprint(w, `{"data":{"type":"subscriptionIntroductoryOffers","id":"intro-usa"}}`)
					default:
						t.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
						w.WriteHeader(http.StatusInternalServerError)
					}
				},
				"subscriptions", "offers", "introductory", "create",
				"--subscription-id", "8000000001",
				"--all-territories",
				"--offer-duration", "ONE_WEEK",
				"--offer-mode", "FREE_TRIAL",
				"--number-of-periods", "1",
			)

			if run.exitCode != test.wantExit {
				t.Fatalf("exit code = %d, want %d; stderr=%q", run.exitCode, test.wantExit, run.stderr)
			}
			if run.stderr != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", run.stderr, test.wantStderr)
			}
			if !strings.Contains(run.stdout, `"created":1`) || !strings.Contains(run.stdout, `"territory":"CAN"`) {
				t.Fatalf("stdout = %q, want summary with one created offer and the CAN failure", run.stdout)
			}
			if run.context.OutcomeKind != test.wantOutcome || int(run.context.HTTPStatus) != test.status {
				t.Fatalf("telemetry context = %+v, want %s with HTTP %d", run.context, test.wantOutcome, test.status)
			}
		})
	}
}

// monthlyCommitmentPrices renders a subscription prices page with one price per
// territory, keyed by territory ID.
func monthlyCommitmentPrices(prices map[string]string) string {
	var data, included []string
	for territory, price := range prices {
		data = append(data, `{"type":"subscriptionPrices","id":"price-`+territory+`","attributes":{"startDate":"2024-01-01"},"relationships":{"territory":{"data":{"type":"territories","id":"`+territory+`"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-`+territory+`"}}}}`)
		included = append(included, `{"type":"subscriptionPricePoints","id":"pp-`+territory+`","attributes":{"customerPrice":"`+price+`"}}`, `{"type":"territories","id":"`+territory+`"}`)
	}
	return `{"data":[` + strings.Join(data, ",") + `],"included":[` + strings.Join(included, ",") + `],"links":{}}`
}

func TestRunSubscriptionsPricingMonthlyCommitmentEnablePriceChecksAreExpectedNegatives(t *testing.T) {
	tests := []struct {
		name          string
		price         string
		territories   string
		upfront       map[string]string
		pricePoint    string
		wantError     string
		wantHint      string
		wantCode      shared.DiagnosticCode
		wantParameter string
	}{
		{
			name:          "price outside the range for the price territory",
			price:         "15.01",
			territories:   "Norway",
			upfront:       map[string]string{"NOR": "120.00"},
			wantError:     "Error: subscriptions pricing monthly-commitment enable: monthly commitment total 180.12 is outside the allowed range [120.00, 180.00] based on upfront price 120.00",
			wantHint:      "Hint: In NOR, pass a --price from 10.00 to 15.00, so that 12 payments total from the upfront annual price to 1.5 times it. List the prices Apple offers in that range with `asc subscriptions pricing price-points list --subscription-id \"8000000001\" --territory NOR --paginate --min-price 10.00 --max-price 15.00`.",
			wantCode:      shared.DiagnosticInvalidInput,
			wantParameter: "--price",
		},
		{
			name:          "price outside the range for another territory",
			price:         "10.00",
			territories:   "Norway,Germany",
			upfront:       map[string]string{"NOR": "120.00", "DEU": "70.00"},
			wantError:     "Error: subscriptions pricing monthly-commitment enable: monthly price is invalid for DEU: monthly commitment total 120.00 is outside the allowed range [70.00, 105.00] based on upfront price 70.00",
			wantHint:      "Hint: In DEU, pass a --price from 5.84 to 8.75, so that 12 payments total from the upfront annual price to 1.5 times it. List the prices Apple offers in that range with `asc subscriptions pricing price-points list --subscription-id \"8000000001\" --territory DEU --paginate --min-price 5.84 --max-price 8.75`.",
			wantCode:      shared.DiagnosticInvalidInput,
			wantParameter: "--price",
		},
		{
			name:          "territory without an upfront price",
			price:         "10.00",
			territories:   "Norway,Germany",
			upfront:       map[string]string{"NOR": "120.00"},
			wantError:     "Error: subscriptions pricing monthly-commitment enable: current UPFRONT subscription price is missing for DEU",
			wantHint:      "Hint: Monthly-commitment billing needs an UPFRONT price in each territory first. Set one in DEU with `asc subscriptions pricing prices set --subscription-id \"8000000001\" --territory DEU --price \"PRICE\"`, or in every territory from one base price with `asc subscriptions pricing equalize --subscription-id \"8000000001\" --base-price \"PRICE\" --confirm`, then run this command again.",
			wantCode:      shared.DiagnosticStateNotReady,
			wantParameter: "--territories",
		},
		{
			name:          "price that is not a price point",
			price:         "10.00",
			territories:   "Norway",
			upfront:       map[string]string{"NOR": "120.00"},
			pricePoint:    "10.49",
			wantError:     "Error: subscriptions pricing monthly-commitment enable: resolve monthly price for NOR: no price point found matching price 10.00 in this territory",
			wantHint:      "Hint: List the prices Apple offers in NOR with `asc subscriptions pricing price-points list --subscription-id \"8000000001\" --territory NOR --paginate --min-price 10.00 --max-price 15.00` and pass one of them as --price.",
			wantCode:      shared.DiagnosticInvalidInput,
			wantParameter: "--price",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			run := runSubscriptionPricingCommand(
				t, func(w http.ResponseWriter, req *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch {
					case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001":
						fmt.Fprint(w, `{"data":{"type":"subscriptions","id":"8000000001","attributes":{"subscriptionPeriod":"ONE_YEAR"}}}`)
					case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/prices":
						prices := map[string]string{}
						territory := req.URL.Query().Get("filter[territory]")
						if req.URL.Query().Get("filter[planType]") == "UPFRONT" {
							for id, price := range test.upfront {
								if territory == "" || territory == id {
									prices[id] = price
								}
							}
						}
						fmt.Fprint(w, monthlyCommitmentPrices(prices))
					case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/planAvailabilities":
						fmt.Fprint(w, `{"data":[]}`)
					case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/pricePoints":
						fmt.Fprint(w, `{"data":[{"type":"subscriptionPricePoints","id":"pp-monthly","attributes":{"customerPrice":"`+test.pricePoint+`"}}],"links":{}}`)
					default:
						t.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
						w.WriteHeader(http.StatusInternalServerError)
					}
				},
				"subscriptions", "pricing", "monthly-commitment", "enable",
				"--subscription-id", "8000000001",
				"--price", test.price,
				"--price-territory", "Norway",
				"--territories", test.territories,
				"--confirm",
			)

			if run.exitCode != ExitError {
				t.Fatalf("exit code = %d, want %d; stderr=%q", run.exitCode, ExitError, run.stderr)
			}
			if want := test.wantError + "\n" + test.wantHint + "\n"; run.stderr != want {
				t.Fatalf("stderr = %q, want %q", run.stderr, want)
			}
			if run.context.OutcomeKind != telemetry.OutcomeExpectedNegative ||
				run.context.DiagnosticCode != string(test.wantCode) ||
				run.context.FailureParameter != test.wantParameter {
				t.Fatalf("telemetry context = %+v, want expected_negative %s on %s", run.context, test.wantCode, test.wantParameter)
			}
		})
	}
}
