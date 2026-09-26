package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSubscriptionsPricingDeriveValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "missing source subscription",
			args: []string{
				"subscriptions", "pricing", "derive",
				"--target-subscription-id", "8000000002",
				"--multiplier", "10",
				"--dry-run",
			},
			wantErr: "--source-subscription-id is required",
		},
		{
			name: "missing target subscription",
			args: []string{
				"subscriptions", "pricing", "derive",
				"--source-subscription-id", "8000000001",
				"--multiplier", "10",
				"--dry-run",
			},
			wantErr: "--target-subscription-id is required",
		},
		{
			name: "missing multiplier",
			args: []string{
				"subscriptions", "pricing", "derive",
				"--source-subscription-id", "8000000001",
				"--target-subscription-id", "8000000002",
				"--dry-run",
			},
			wantErr: "--multiplier is required",
		},
		{
			name: "zero multiplier",
			args: []string{
				"subscriptions", "pricing", "derive",
				"--source-subscription-id", "8000000001",
				"--target-subscription-id", "8000000002",
				"--multiplier", "0",
				"--dry-run",
			},
			wantErr: "--multiplier must be a positive decimal",
		},
		{
			name: "negative multiplier",
			args: []string{
				"subscriptions", "pricing", "derive",
				"--source-subscription-id", "8000000001",
				"--target-subscription-id", "8000000002",
				"--multiplier", "-2.5",
				"--dry-run",
			},
			wantErr: "--multiplier must be a positive decimal",
		},
		{
			name: "non finite multiplier",
			args: []string{
				"subscriptions", "pricing", "derive",
				"--source-subscription-id", "8000000001",
				"--target-subscription-id", "8000000002",
				"--multiplier", "NaN",
				"--dry-run",
			},
			wantErr: "--multiplier must be a positive decimal",
		},
		{
			name: "fraction syntax is not a decimal",
			args: []string{
				"subscriptions", "pricing", "derive",
				"--source-subscription-id", "8000000001",
				"--target-subscription-id", "8000000002",
				"--multiplier", "1/2",
				"--dry-run",
			},
			wantErr: "--multiplier must be a positive decimal",
		},
		{
			name: "invalid rounding mode",
			args: []string{
				"subscriptions", "pricing", "derive",
				"--source-subscription-id", "8000000001",
				"--target-subscription-id", "8000000002",
				"--multiplier", "10",
				"--round", "bankers",
				"--dry-run",
			},
			wantErr: "--round must be one of: exact, nearest, up, down",
		},
		{
			name: "invalid territory",
			args: []string{
				"subscriptions", "pricing", "derive",
				"--source-subscription-id", "8000000001",
				"--target-subscription-id", "8000000002",
				"--multiplier", "10",
				"--territory", "Atlantis",
				"--dry-run",
			},
			wantErr: "invalid --territory",
		},
		{
			name: "explicit empty territory",
			args: []string{
				"subscriptions", "pricing", "derive",
				"--source-subscription-id", "8000000001",
				"--target-subscription-id", "8000000002",
				"--multiplier", "10",
				"--territory", "",
				"--dry-run",
			},
			wantErr: "invalid --territory: cannot be empty",
		},
		{
			name: "same literal subscriptions",
			args: []string{
				"subscriptions", "pricing", "derive",
				"--source-subscription-id", "8000000001",
				"--target-subscription-id", "8000000001",
				"--multiplier", "10",
				"--dry-run",
			},
			wantErr: "source and target subscriptions must be different",
		},
		{
			name: "workers below range",
			args: []string{
				"subscriptions", "pricing", "derive",
				"--source-subscription-id", "8000000001",
				"--target-subscription-id", "8000000002",
				"--multiplier", "10",
				"--workers", "0",
				"--dry-run",
			},
			wantErr: "--workers must be between 1 and 32",
		},
		{
			name: "workers above range",
			args: []string{
				"subscriptions", "pricing", "derive",
				"--source-subscription-id", "8000000001",
				"--target-subscription-id", "8000000002",
				"--multiplier", "10",
				"--workers", "33",
				"--dry-run",
			},
			wantErr: "--workers must be between 1 and 32",
		},
		{
			name: "dry run and confirm conflict",
			args: []string{
				"subscriptions", "pricing", "derive",
				"--source-subscription-id", "8000000001",
				"--target-subscription-id", "8000000002",
				"--multiplier", "10",
				"--dry-run",
				"--confirm",
			},
			wantErr: "--dry-run cannot be combined with --confirm",
		},
		{
			name: "requires confirm",
			args: []string{
				"subscriptions", "pricing", "derive",
				"--source-subscription-id", "8000000001",
				"--target-subscription-id", "8000000002",
				"--multiplier", "10",
			},
			wantErr: "--confirm is required unless --dry-run is set",
		},
		{
			name: "rejects positional arguments",
			args: []string{
				"subscriptions", "pricing", "derive", "extra",
				"--source-subscription-id", "8000000001",
				"--target-subscription-id", "8000000002",
				"--multiplier", "10",
				"--dry-run",
			},
			wantErr: `unexpected argument "extra"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}

func TestSubscriptionsPricingDeriveRequiresAppForLookupSelectorBeforeAuth(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_CONFIG_PATH", t.TempDir()+"/missing.json")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	assertUsageExit(t, []string{
		"subscriptions", "pricing", "derive",
		"--source-subscription-id", "com.example.monthly",
		"--target-subscription-id", "8000000002",
		"--multiplier", "10",
		"--dry-run",
	}, "--app is required (or set ASC_APP_ID) when --source-subscription-id is a product ID or name")
}

func TestSubscriptionsPricingDeriveHelpRegistersNewSurface(t *testing.T) {
	root := RootCommand("1.2.3")
	derive := findCommand(root, "subscriptions", "pricing", "derive")
	if derive == nil {
		t.Fatal("subscriptions pricing derive command is not registered")
	}

	for _, name := range []string{
		"source-subscription-id", "target-subscription-id", "app", "multiplier",
		"round", "territory", "start-date", "preserved", "auto-start-date",
		"dry-run", "confirm", "workers",
	} {
		flagValue := derive.FlagSet.Lookup(name)
		if flagValue == nil {
			t.Fatalf("flag --%s is not registered", name)
		}
	}
}

func TestSubscriptionsPricingDeriveBooleanFlagExitCodes(t *testing.T) {
	for _, test := range []struct {
		name       string
		flag       string
		wantStderr string
	}{
		{name: "invalid dry run", flag: "--dry-run=maybe", wantStderr: `invalid boolean value "maybe" for -dry-run`},
		{name: "invalid confirm", flag: "--confirm=maybe", wantStderr: `invalid boolean value "maybe" for -confirm`},
		{name: "invalid preserved", flag: "--preserved=maybe", wantStderr: `invalid boolean value "maybe" for -preserved`},
		{name: "invalid auto start date", flag: "--auto-start-date=maybe", wantStderr: `invalid boolean value "maybe" for -auto-start-date`},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, []string{
				"subscriptions", "pricing", "derive",
				"--source-subscription-id", "8000000001",
				"--target-subscription-id", "8000000002",
				"--multiplier", "10",
				test.flag,
			}, test.wantStderr)
		})
	}
}

func TestSubscriptionsPricingDeriveDryRunResolvesUnevenTerritoryLadders(t *testing.T) {
	setupAuth(t)

	const sourceID = "8000000001"
	const targetID = "8000000002"
	mutationRequests := 0

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+sourceID+"/prices":
			if got := req.URL.Query().Get("filter[planType]"); got != "UPFRONT" {
				t.Fatalf("source prices plan type = %q, want UPFRONT", got)
			}
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricesFixture([]deriveHTTPPrice{
				{priceID: "source-price-swe", pricePointID: "source-pp-swe", territory: "SWE", currency: "SEK", customerPrice: "9"},
				{priceID: "source-price-usa", pricePointID: "source-pp-usa", territory: "USA", currency: "USD", customerPrice: "0.99"},
			}))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+targetID+"/prices":
			if got := req.URL.Query().Get("filter[planType]"); got != "UPFRONT" {
				t.Fatalf("target prices plan type = %q, want UPFRONT", got)
			}
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricesFixture([]deriveHTTPPrice{
				{priceID: "target-price-swe", pricePointID: "target-current-swe", territory: "SWE", currency: "SEK", customerPrice: "129"},
				{priceID: "target-price-usa", pricePointID: "target-current-usa", territory: "USA", currency: "USD", customerPrice: "12.99"},
			}))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+targetID+"/pricePoints":
			territory := req.URL.Query().Get("filter[territory]")
			if req.URL.Query().Get("fields[subscriptionPricePoints]") != "customerPrice,territory" || req.URL.Query().Get("include") != "territory" || req.URL.Query().Get("limit") != "8000" {
				t.Fatalf("unexpected target price-point query: %s", req.URL.RawQuery)
			}
			if territory != "SWE,USA" {
				t.Fatalf("target price-point territory filter = %q, want SWE,USA", territory)
			}
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricePointsFixture([]deriveHTTPPricePoint{
				{id: "target-pp-swe-89", territory: "SWE", customerPrice: "89"},
				{id: "target-pp-swe-99", territory: "SWE", customerPrice: "99"},
				{id: "target-pp-usa-899", territory: "USA", customerPrice: "8.99"},
				{id: "target-pp-usa-999", territory: "USA", customerPrice: "9.99"},
			}))
		case req.Method == http.MethodPost || req.Method == http.MethodPatch || req.Method == http.MethodDelete:
			mutationRequests++
			t.Fatalf("dry-run made mutation request: %s %s", req.Method, req.URL.String())
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "derive",
			"--source-subscription-id", sourceID,
			"--target-subscription-id", targetID,
			"--multiplier", "10",
			"--round", "nearest",
			"--workers", "1",
			"--auto-start-date=false",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if mutationRequests != 0 {
		t.Fatalf("dry-run mutation requests = %d, want 0", mutationRequests)
	}
	if !strings.Contains(stderr, "Resolving target price points for 2 territories") {
		t.Fatalf("missing progress diagnostic, stderr=%q", stderr)
	}

	var result struct {
		SourceSubscriptionID string `json:"sourceSubscriptionId"`
		TargetSubscriptionID string `json:"targetSubscriptionId"`
		Multiplier           string `json:"multiplier"`
		Rounding             string `json:"rounding"`
		DryRun               bool   `json:"dryRun"`
		Summary              struct {
			Total      int `json:"total"`
			Planned    int `json:"planned"`
			Noop       int `json:"noop"`
			Unresolved int `json:"unresolved"`
		} `json:"summary"`
		Rows []struct {
			Territory          string `json:"territory"`
			Currency           string `json:"currency"`
			SourcePrice        string `json:"sourcePrice"`
			DesiredPrice       string `json:"desiredPrice"`
			CurrentTargetPrice string `json:"currentTargetPrice"`
			TargetPrice        string `json:"targetPrice"`
			TargetPricePointID string `json:"targetPricePointId"`
			RequestedMultiple  string `json:"requestedMultiple"`
			AchievedMultiple   string `json:"achievedMultiple"`
			Action             string `json:"action"`
			Status             string `json:"status"`
		} `json:"rows"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON output: %v\nstdout=%s", err, stdout)
	}
	if result.SourceSubscriptionID != sourceID || result.TargetSubscriptionID != targetID {
		t.Fatalf("unexpected subscription ids: %+v", result)
	}
	if result.Multiplier != "10" || result.Rounding != "nearest" || !result.DryRun {
		t.Fatalf("unexpected derivation metadata: %+v", result)
	}
	if result.Summary.Total != 2 || result.Summary.Planned != 2 || result.Summary.Noop != 0 || result.Summary.Unresolved != 0 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
	if len(result.Rows) != 2 || result.Rows[0].Territory != "SWE" || result.Rows[1].Territory != "USA" {
		t.Fatalf("rows are not deterministically sorted: %+v", result.Rows)
	}
	swe := result.Rows[0]
	if swe.Currency != "SEK" || swe.SourcePrice != "9" || swe.DesiredPrice != "90" || swe.CurrentTargetPrice != "129" || swe.TargetPrice != "89" {
		t.Fatalf("unexpected SWE derivation: %+v", swe)
	}
	if swe.TargetPricePointID != "target-pp-swe-89" || swe.RequestedMultiple != "10" || swe.AchievedMultiple != "9.888889" || swe.Action != "update" || swe.Status != "planned" {
		t.Fatalf("unexpected SWE resolution metadata: %+v", swe)
	}
}

func TestSubscriptionsPricingDeriveTerritoryDryRunPlansMissingCurrentTargetPrice(t *testing.T) {
	setupAuth(t)

	const sourceID = "8000000001"
	const targetID = "8000000002"
	targetPriceReads := 0

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+sourceID+"/prices":
			if got := req.URL.Query().Get("filter[territory]"); got != "SWE" {
				t.Fatalf("source territory filter = %q, want SWE", got)
			}
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricesFixture([]deriveHTTPPrice{
				{priceID: "source-price-swe", pricePointID: "source-pp-swe", territory: "SWE", currency: "SEK", customerPrice: "9"},
			}))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+targetID+"/prices":
			targetPriceReads++
			switch got := req.URL.Query().Get("filter[territory]"); got {
			case "SWE":
				return jsonResponse(http.StatusOK, subscriptionPriceDerivePricesFixture(nil))
			case "":
				return jsonResponse(http.StatusOK, subscriptionPriceDerivePricesFixture([]deriveHTTPPrice{
					{priceID: "target-price-usa", pricePointID: "target-pp-usa", territory: "USA", currency: "USD", customerPrice: "9.99"},
				}))
			default:
				t.Fatalf("target territory filter = %q, want SWE or empty global preflight", got)
				return nil, nil
			}
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+targetID+"/pricePoints":
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricePointsFixture([]deriveHTTPPricePoint{
				{id: "target-pp-swe-89", territory: "SWE", customerPrice: "89"},
				{id: "target-pp-swe-99", territory: "SWE", customerPrice: "99"},
			}))
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "derive",
			"--source-subscription-id", sourceID,
			"--target-subscription-id", targetID,
			"--multiplier", "10",
			"--round", "nearest",
			"--territory", "SWE",
			"--workers", "1",
			"--auto-start-date=false",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if !strings.Contains(stderr, "Fetching current source and target subscription prices") ||
		!strings.Contains(stderr, "Resolving target price points for 1 territory") {
		t.Fatalf("missing dry-run progress diagnostics, stderr=%q", stderr)
	}
	if targetPriceReads != 2 {
		t.Fatalf("target price reads = %d, want scoped read plus global pricing preflight", targetPriceReads)
	}

	var result struct {
		Summary struct {
			Planned    int `json:"planned"`
			Unresolved int `json:"unresolved"`
		} `json:"summary"`
		Rows []struct {
			Territory          string `json:"territory"`
			CurrentTargetPrice string `json:"currentTargetPrice"`
			TargetPrice        string `json:"targetPrice"`
			Action             string `json:"action"`
			Status             string `json:"status"`
		} `json:"rows"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON output: %v\nstdout=%s", err, stdout)
	}
	if result.Summary.Planned != 1 || result.Summary.Unresolved != 0 || len(result.Rows) != 1 {
		t.Fatalf("unexpected missing-target-price plan: %+v", result)
	}
	row := result.Rows[0]
	if row.Territory != "SWE" || row.CurrentTargetPrice != "" || row.TargetPrice != "89" || row.Action != "update" || row.Status != "planned" {
		t.Fatalf("unexpected missing-target-price row: %+v", row)
	}
}

func TestSubscriptionsPricingDeriveTerritoryDryRunRejectsEntirelyUnpricedTarget(t *testing.T) {
	setupAuth(t)

	const sourceID = "8000000001"
	const targetID = "8000000002"
	targetPriceReads := 0

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+sourceID+"/prices":
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricesFixture([]deriveHTTPPrice{
				{priceID: "source-price-swe", pricePointID: "source-pp-swe", territory: "SWE", currency: "SEK", customerPrice: "9"},
			}))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+targetID+"/prices":
			targetPriceReads++
			territoryFilter := req.URL.Query().Get("filter[territory]")
			if territoryFilter != "SWE" && territoryFilter != "" {
				t.Fatalf("target territory filter = %q, want SWE or empty global preflight", territoryFilter)
			}
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricesFixture(nil))
		default:
			t.Fatalf("unpriced target should fail before request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "derive",
			"--source-subscription-id", sourceID,
			"--target-subscription-id", targetID,
			"--multiplier", "10",
			"--territory", "SWE",
			"--auto-start-date=false",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "target subscription has no current UPFRONT prices; initialize its pricing before deriving changes") {
		t.Fatalf("expected unpriced target error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty output before planning", stdout)
	}
	if targetPriceReads != 2 {
		t.Fatalf("target price reads = %d, want scoped read plus global pricing preflight", targetPriceReads)
	}
}

func TestSubscriptionsPricingDeriveUsesTargetPriceEffectiveOnStartDateForNoopDetection(t *testing.T) {
	setupAuth(t)

	const sourceID = "8000000001"
	const targetID = "8000000002"
	const startDate = "2099-01-02"
	mutationRequests := 0

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+sourceID+"/prices":
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricesFixture([]deriveHTTPPrice{
				{priceID: "source-price-swe", pricePointID: "source-pp-swe", territory: "SWE", currency: "SEK", customerPrice: "9"},
			}))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+targetID+"/prices":
			return jsonResponse(http.StatusOK, `{"data":[
				{"type":"subscriptionPrices","id":"target-current","attributes":{"preserved":false,"planType":"UPFRONT"},"relationships":{"territory":{"data":{"type":"territories","id":"SWE"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"target-pp-89"}}}},
				{"type":"subscriptionPrices","id":"target-future","attributes":{"startDate":"2099-01-02","preserved":false,"planType":"UPFRONT"},"relationships":{"territory":{"data":{"type":"territories","id":"SWE"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"target-pp-99"}}}}
			],"included":[
				{"type":"subscriptionPricePoints","id":"target-pp-89","attributes":{"customerPrice":"89"}},
				{"type":"subscriptionPricePoints","id":"target-pp-99","attributes":{"customerPrice":"99"}},
				{"type":"territories","id":"SWE","attributes":{"currency":"SEK"}}
			],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+targetID+"/pricePoints":
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricePointsFixture([]deriveHTTPPricePoint{
				{id: "target-pp-89", territory: "SWE", customerPrice: "89"},
				{id: "target-pp-99", territory: "SWE", customerPrice: "99"},
			}))
		case req.Method == http.MethodPost || req.Method == http.MethodPatch || req.Method == http.MethodDelete:
			mutationRequests++
			t.Fatalf("dry-run made mutation request: %s %s", req.Method, req.URL.String())
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "derive",
			"--source-subscription-id", sourceID,
			"--target-subscription-id", targetID,
			"--multiplier", "10",
			"--round", "nearest",
			"--workers", "1",
			"--start-date", startDate,
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if mutationRequests != 0 {
		t.Fatalf("mutation requests = %d, want 0", mutationRequests)
	}

	var result struct {
		StartDate string `json:"startDate"`
		Rows      []struct {
			CurrentTargetPointID string `json:"currentTargetPricePointId"`
			TargetPricePointID   string `json:"targetPricePointId"`
			Status               string `json:"status"`
		} `json:"rows"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON output: %v\nstdout=%s", err, stdout)
	}
	if result.StartDate != startDate || len(result.Rows) != 1 {
		t.Fatalf("unexpected scheduled plan: %+v", result)
	}
	row := result.Rows[0]
	if row.CurrentTargetPointID != "target-pp-99" || row.TargetPricePointID != "target-pp-89" || row.Status != "planned" {
		t.Fatalf("expected future target price to prevent a false noop, got %+v", row)
	}
}

func TestSubscriptionsPricingDeriveApplyCreatesAndVerifiesTargetPrice(t *testing.T) {
	setupAuth(t)

	const sourceID = "8000000001"
	const targetID = "8000000002"
	startDate := time.Now().UTC().AddDate(0, 0, 7).Format("2006-01-02")
	targetPriceReads := 0
	postRequests := 0

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+sourceID+"/prices":
			if got := req.URL.Query().Get("filter[territory]"); got != "SWE" {
				t.Fatalf("source territory filter = %q, want SWE", got)
			}
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricesFixture([]deriveHTTPPrice{
				{priceID: "source-price-swe", pricePointID: "source-pp-swe", territory: "SWE", currency: "SEK", customerPrice: "9"},
			}))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+targetID+"/prices":
			if got := req.URL.Query().Get("filter[territory]"); got != "SWE" {
				t.Fatalf("target territory filter = %q, want SWE", got)
			}
			targetPriceReads++
			if targetPriceReads == 1 {
				return jsonResponse(http.StatusOK, subscriptionPriceDerivePricesFixture([]deriveHTTPPrice{
					{priceID: "target-current-swe", pricePointID: "target-pp-swe-129", territory: "SWE", currency: "SEK", customerPrice: "129"},
				}))
			}
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricesFixture([]deriveHTTPPrice{
				{priceID: "target-derived-swe", pricePointID: "target-pp-swe-89", territory: "SWE", currency: "SEK", customerPrice: "89"},
			}))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+targetID+"/pricePoints":
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricePointsFixture([]deriveHTTPPricePoint{
				{id: "target-pp-swe-89", territory: "SWE", customerPrice: "89"},
				{id: "target-pp-swe-99", territory: "SWE", customerPrice: "99"},
			}))
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices":
			postRequests++
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read mutation body: %v", err)
			}
			var payload struct {
				Data struct {
					Type       string `json:"type"`
					Attributes struct {
						StartDate            string `json:"startDate"`
						PreserveCurrentPrice *bool  `json:"preserveCurrentPrice"`
						PlanType             string `json:"planType"`
					} `json:"attributes"`
					Relationships struct {
						Subscription struct {
							Data struct {
								ID string `json:"id"`
							} `json:"data"`
						} `json:"subscription"`
						Territory struct {
							Data struct {
								ID string `json:"id"`
							} `json:"data"`
						} `json:"territory"`
						PricePoint struct {
							Data struct {
								ID string `json:"id"`
							} `json:"data"`
						} `json:"subscriptionPricePoint"`
					} `json:"relationships"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("parse mutation body: %v\nbody=%s", err, body)
			}
			if payload.Data.Type != "subscriptionPrices" || payload.Data.Relationships.Subscription.Data.ID != targetID {
				t.Fatalf("unexpected mutation target: %+v", payload)
			}
			if payload.Data.Relationships.Territory.Data.ID != "SWE" || payload.Data.Relationships.PricePoint.Data.ID != "target-pp-swe-89" {
				t.Fatalf("unexpected mutation relationships: %+v", payload.Data.Relationships)
			}
			if payload.Data.Attributes.PlanType != "UPFRONT" || payload.Data.Attributes.StartDate != startDate || payload.Data.Attributes.PreserveCurrentPrice == nil || !*payload.Data.Attributes.PreserveCurrentPrice {
				t.Fatalf("unexpected mutation attributes: %+v", payload.Data.Attributes)
			}
			return jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionPrices","id":"target-derived-swe","attributes":{"planType":"UPFRONT"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "derive",
			"--source-subscription-id", sourceID,
			"--target-subscription-id", targetID,
			"--multiplier", "10",
			"--round", "nearest",
			"--territory", "Sweden",
			"--workers", "1",
			"--start-date", startDate,
			"--preserved",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if postRequests != 1 {
		t.Fatalf("POST requests = %d, want 1", postRequests)
	}
	if targetPriceReads != 2 {
		t.Fatalf("target price reads = %d, want planning + verification", targetPriceReads)
	}
	if !strings.Contains(stderr, "Setting 1 derived territory price") || !strings.Contains(stderr, "Verifying derived target prices") {
		t.Fatalf("missing apply diagnostics, stderr=%q", stderr)
	}

	var result struct {
		Summary struct {
			Applied  int `json:"applied"`
			Verified int `json:"verified"`
			Failed   int `json:"failed"`
		} `json:"summary"`
		Rows []struct {
			Territory string `json:"territory"`
			Status    string `json:"status"`
		} `json:"rows"`
		Verification struct {
			Status   string `json:"status"`
			Verified int    `json:"verified"`
			Failed   int    `json:"failed"`
		} `json:"verification"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON output: %v\nstdout=%s", err, stdout)
	}
	if result.Summary.Applied != 1 || result.Summary.Verified != 1 || result.Summary.Failed != 0 {
		t.Fatalf("unexpected apply summary: %+v", result.Summary)
	}
	if len(result.Rows) != 1 || result.Rows[0].Territory != "SWE" || result.Rows[0].Status != "verified" {
		t.Fatalf("unexpected apply rows: %+v", result.Rows)
	}
	if result.Verification.Status != "completed" || result.Verification.Verified != 1 || result.Verification.Failed != 0 {
		t.Fatalf("unexpected verification: %+v", result.Verification)
	}
}

func TestSubscriptionsPricingDeriveReportsMutationFailuresInStructuredOutput(t *testing.T) {
	setupAuth(t)

	const sourceID = "8000000001"
	const targetID = "8000000002"
	postRequests := 0
	targetPriceReads := 0

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+sourceID+"/prices":
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricesFixture([]deriveHTTPPrice{
				{priceID: "source-price-swe", pricePointID: "source-pp-swe", territory: "SWE", currency: "SEK", customerPrice: "9"},
			}))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+targetID+"/prices":
			targetPriceReads++
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricesFixture([]deriveHTTPPrice{
				{priceID: "target-current-swe", pricePointID: "target-pp-swe-129", territory: "SWE", currency: "SEK", customerPrice: "129"},
			}))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+targetID+"/pricePoints":
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricePointsFixture([]deriveHTTPPricePoint{
				{id: "target-pp-swe-89", territory: "SWE", customerPrice: "89"},
				{id: "target-pp-swe-99", territory: "SWE", customerPrice: "99"},
			}))
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices":
			postRequests++
			return jsonResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR","title":"The request is invalid","detail":"test rejection"}]}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "derive",
			"--source-subscription-id", sourceID,
			"--target-subscription-id", targetID,
			"--multiplier", "10",
			"--round", "nearest",
			"--workers", "1",
			"--auto-start-date=false",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "failed before verification") {
		t.Fatalf("expected reported mutation failure, got %v", runErr)
	}
	if postRequests != 1 {
		t.Fatalf("POST requests = %d, want 1", postRequests)
	}
	if targetPriceReads != 2 {
		t.Fatalf("target price reads = %d, want planning + failure reconciliation", targetPriceReads)
	}

	var result struct {
		Summary struct {
			Failed int `json:"failed"`
		} `json:"summary"`
		Rows []struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		} `json:"rows"`
		Verification struct {
			Status string `json:"status"`
			Failed int    `json:"failed"`
		} `json:"verification"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON output: %v\nstdout=%s", err, stdout)
	}
	if result.Summary.Failed != 1 || len(result.Rows) != 1 || result.Rows[0].Status != "failed" || !strings.Contains(result.Rows[0].Error, "test rejection") {
		t.Fatalf("unexpected structured failure result: %+v", result)
	}
	if result.Verification.Status != "failed" || result.Verification.Failed != 1 {
		t.Fatalf("unexpected failure verification summary: %+v", result.Verification)
	}
}

func TestSubscriptionsPricingDeriveApplyFailsClosedWhenExactPriceIsMissing(t *testing.T) {
	setupAuth(t)

	const sourceID = "8000000001"
	const targetID = "8000000002"
	mutationRequests := 0
	targetPriceReads := 0

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+sourceID+"/prices":
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricesFixture([]deriveHTTPPrice{
				{priceID: "source-price-swe", pricePointID: "source-pp-swe", territory: "SWE", currency: "SEK", customerPrice: "9"},
			}))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+targetID+"/prices":
			targetPriceReads++
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricesFixture([]deriveHTTPPrice{
				{priceID: "target-current-swe", pricePointID: "target-pp-swe-129", territory: "SWE", currency: "SEK", customerPrice: "129"},
			}))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+targetID+"/pricePoints":
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricePointsFixture([]deriveHTTPPricePoint{
				{id: "target-pp-swe-89", territory: "SWE", customerPrice: "89"},
				{id: "target-pp-swe-99", territory: "SWE", customerPrice: "99"},
			}))
		case req.Method == http.MethodPost || req.Method == http.MethodPatch || req.Method == http.MethodDelete:
			mutationRequests++
			t.Fatalf("fail-closed planning made mutation request: %s %s", req.Method, req.URL.String())
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "derive",
			"--source-subscription-id", sourceID,
			"--target-subscription-id", targetID,
			"--multiplier", "10",
			"--round", "exact",
			"--workers", "1",
			"--auto-start-date=false",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "could not be resolved; no changes were applied") {
		t.Fatalf("expected fail-closed reported error, got %v", runErr)
	}
	if mutationRequests != 0 {
		t.Fatalf("mutation requests = %d, want 0", mutationRequests)
	}
	if targetPriceReads != 1 {
		t.Fatalf("target price reads = %d, want planning read only", targetPriceReads)
	}
	var result struct {
		Summary struct {
			Unresolved int `json:"unresolved"`
		} `json:"summary"`
		Rows []struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		} `json:"rows"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON output: %v\nstdout=%s", err, stdout)
	}
	if result.Summary.Unresolved != 1 || len(result.Rows) != 1 || result.Rows[0].Status != "unresolved" || !strings.Contains(result.Rows[0].Error, "no exact target price point for 90") {
		t.Fatalf("unexpected unresolved plan: %+v", result)
	}
}

func TestSubscriptionsPricingDeriveApplySkipsMatchingTargetPrice(t *testing.T) {
	setupAuth(t)

	const sourceID = "8000000001"
	const targetID = "8000000002"
	targetPriceReads := 0
	mutationRequests := 0

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+sourceID+"/prices":
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricesFixture([]deriveHTTPPrice{
				{priceID: "source-price-swe", pricePointID: "source-pp-swe", territory: "SWE", currency: "SEK", customerPrice: "9"},
			}))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+targetID+"/prices":
			targetPriceReads++
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricesFixture([]deriveHTTPPrice{
				{priceID: "target-current-swe", pricePointID: "target-pp-swe-89", territory: "SWE", currency: "SEK", customerPrice: "89"},
			}))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/"+targetID+"/pricePoints":
			return jsonResponse(http.StatusOK, subscriptionPriceDerivePricePointsFixture([]deriveHTTPPricePoint{
				{id: "target-pp-swe-89", territory: "SWE", customerPrice: "89"},
				{id: "target-pp-swe-99", territory: "SWE", customerPrice: "99"},
			}))
		case req.Method == http.MethodPost || req.Method == http.MethodPatch || req.Method == http.MethodDelete:
			mutationRequests++
			t.Fatalf("noop plan made mutation request: %s %s", req.Method, req.URL.String())
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "derive",
			"--source-subscription-id", sourceID,
			"--target-subscription-id", targetID,
			"--multiplier", "10",
			"--round", "nearest",
			"--workers", "1",
			"--auto-start-date=false",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if mutationRequests != 0 {
		t.Fatalf("mutation requests = %d, want 0", mutationRequests)
	}
	if targetPriceReads != 1 {
		t.Fatalf("target price reads = %d, want planning read only for a noop", targetPriceReads)
	}
	var result struct {
		Summary struct {
			Noop    int `json:"noop"`
			Applied int `json:"applied"`
		} `json:"summary"`
		Rows []struct {
			Status string `json:"status"`
		} `json:"rows"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON output: %v\nstdout=%s", err, stdout)
	}
	if result.Summary.Noop != 1 || result.Summary.Applied != 0 || len(result.Rows) != 1 || result.Rows[0].Status != "noop" {
		t.Fatalf("unexpected noop result: %+v", result)
	}
}

type deriveHTTPPrice struct {
	priceID       string
	pricePointID  string
	territory     string
	currency      string
	customerPrice string
}

type deriveHTTPPricePoint struct {
	id            string
	territory     string
	customerPrice string
}

func subscriptionPriceDerivePricePointsFixture(pricePoints []deriveHTTPPricePoint) string {
	data := make([]string, 0, len(pricePoints))
	for _, pricePoint := range pricePoints {
		data = append(data, `{"type":"subscriptionPricePoints","id":"`+pricePoint.id+`","attributes":{"customerPrice":"`+pricePoint.customerPrice+`"},"relationships":{"territory":{"data":{"type":"territories","id":"`+pricePoint.territory+`"}}}}`)
	}
	return `{"data":[` + strings.Join(data, ",") + `],"links":{}}`
}

func subscriptionPriceDerivePricesFixture(prices []deriveHTTPPrice) string {
	data := make([]string, 0, len(prices))
	included := make([]string, 0, len(prices)*2)
	seenTerritories := make(map[string]struct{})
	for _, price := range prices {
		data = append(data, `{"type":"subscriptionPrices","id":"`+price.priceID+`","attributes":{"preserved":false,"planType":"UPFRONT"},"relationships":{"territory":{"data":{"type":"territories","id":"`+price.territory+`"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"`+price.pricePointID+`"}}}}`)
		included = append(included, `{"type":"subscriptionPricePoints","id":"`+price.pricePointID+`","attributes":{"customerPrice":"`+price.customerPrice+`"}}`)
		if _, ok := seenTerritories[price.territory]; !ok {
			seenTerritories[price.territory] = struct{}{}
			included = append(included, `{"type":"territories","id":"`+price.territory+`","attributes":{"currency":"`+price.currency+`"}}`)
		}
	}
	return `{"data":[` + strings.Join(data, ",") + `],"included":[` + strings.Join(included, ",") + `],"links":{}}`
}
