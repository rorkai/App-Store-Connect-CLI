package cmdtest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestSubscriptionsPricingMonthlyCommitmentHelp(t *testing.T) {
	root := RootCommand("1.2.3")

	cmd := findSubcommand(root, "subscriptions", "pricing")
	if cmd == nil {
		t.Fatal("expected subscriptions pricing command")
		return
	}
	usage := cmd.UsageFunc(cmd)
	if !strings.Contains(usage, "monthly-commitment") {
		t.Fatalf("expected pricing help to mention monthly-commitment, got %q", usage)
	}

	monthlyCmd := findSubcommand(root, "subscriptions", "pricing", "monthly-commitment")
	if monthlyCmd == nil {
		t.Fatal("expected monthly-commitment command")
		return
	}
	monthlyUsage := monthlyCmd.UsageFunc(monthlyCmd)
	if !strings.Contains(monthlyUsage, "App Store Connect API 4.4") {
		t.Fatalf("expected monthly-commitment help to mention App Store Connect API 4.4, got %q", monthlyUsage)
	}
}

func TestSubscriptionsPricingMonthlyCommitmentValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "enable missing subscription",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "enable", "--price", "9.99", "--price-territory", "Norway", "--territories", "Norway"},
			wantErr: "--subscription-id is required",
		},
		{
			name:    "enable rejects excluded price territory",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "enable", "--subscription-id", "sub-1", "--price", "9.99", "--price-territory", "United States", "--territories", "Norway"},
			wantErr: "--price-territory cannot be USA or Singapore",
		},
		{
			name:    "enable rejects excluded price territory with mixed flag order",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "enable", "--territories", "Norway", "--price-territory", "USA", "--subscription-id", "sub-1", "--price", "9.99"},
			wantErr: "--price-territory cannot be USA or Singapore",
		},
		{
			name:    "enable rejects available in new territories",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "enable", "--subscription-id", "sub-1", "--price", "9.99", "--price-territory", "Norway", "--territories", "Norway", "--available-in-new-territories"},
			wantErr: "--available-in-new-territories is not supported for MONTHLY plan availability",
		},
		{
			name:    "disable missing territories",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "disable", "--subscription-id", "sub-1"},
			wantErr: "--territories is required",
		},
		{
			name:    "disable treats subcommand name as flag value",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "disable", "--subscription-id", "sub-1", "--territories", "list"},
			wantErr: "territory \"list\" could not be mapped",
		},
		{
			name:    "list missing subscription",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "list"},
			wantErr: "--subscription-id is required",
		},
		{
			name:    "list invalid plan type",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "list", "--subscription-id", "sub-1", "--plan-type", "annual"},
			wantErr: "--plan-type must be one of: MONTHLY, UPFRONT",
		},
		{
			name:    "list empty plan type",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "list", "--subscription-id", "sub-1", "--plan-type", ""},
			wantErr: "invalid value for --plan-type: cannot be empty",
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

func TestSubscriptionsPricingMonthlyCommitmentUsageExitCodes(t *testing.T) {
	binaryPath := buildASCBlackBoxBinary(t)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "subcommand flag before subcommand returns usage",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "--subscription-id", "sub-1", "enable", "--price", "9.99", "--price-territory", "Norway", "--territories", "Norway"},
			wantErr: "flag provided but not defined: -subscription-id",
		},
		{
			name:    "mixed flag order invalid price territory returns usage",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "enable", "--territories", "Norway", "--price-territory", "USA", "--subscription-id", "sub-1", "--price", "9.99"},
			wantErr: "--price-territory cannot be USA or Singapore",
		},
		{
			name:    "flag value matching subcommand returns usage when invalid",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "disable", "--subscription-id", "sub-1", "--territories", "list"},
			wantErr: "territory \"list\" could not be mapped",
		},
		{
			name:    "list invalid plan type returns usage",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "list", "--subscription-id", "sub-1", "--plan-type", "annual"},
			wantErr: "--plan-type must be one of: MONTHLY, UPFRONT",
		},
		{
			name:    "list empty plan type returns usage",
			args:    []string{"subscriptions", "pricing", "monthly-commitment", "list", "--subscription-id", "sub-1", "--plan-type", ""},
			wantErr: "invalid value for --plan-type: cannot be empty",
		},
		{
			name:    "availability edit invalid billing mode returns usage",
			args:    []string{"subscriptions", "pricing", "availability", "edit", "--subscription-id", "sub-1", "--territories", "Norway", "--billing-mode", "list"},
			wantErr: "--billing-mode must be one of: upfront, monthly-commitment",
		},
		{
			name:    "availability edit rejects available in new territories for monthly",
			args:    []string{"subscriptions", "pricing", "availability", "edit", "--subscription-id", "sub-1", "--territories", "Norway", "--billing-mode", "monthly-commitment", "--available-in-new-territories"},
			wantErr: "--available-in-new-territories is not supported for MONTHLY plan availability",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, test.args...)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("expected process exit error, got %v", err)
			}
			if exitErr.ExitCode() != 2 {
				t.Fatalf("expected exit code 2, got %d", exitErr.ExitCode())
			}
			if stdout.String() != "" {
				t.Fatalf("expected empty stdout, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr.String())
			}
		})
	}
}

func TestSubscriptionsPricingMonthlyCommitmentListFiltersPlanType(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/subscriptions/8000000001/planAvailabilities" || req.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
		if got := req.URL.Query().Get("filter[planType]"); got != "" {
			t.Fatalf("expected no server-side planType filter, got %q", got)
		}
		body := `{"data":[
			{"type":"subscriptionPlanAvailabilities","id":"plan-monthly","attributes":{"planType":"MONTHLY","availableInNewTerritories":true}},
			{"type":"subscriptionPlanAvailabilities","id":"plan-upfront","attributes":{"planType":"UPFRONT","availableInNewTerritories":false}}
		]}`
		return jsonResponse(http.StatusOK, body)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "monthly-commitment", "list",
			"--subscription-id", "8000000001",
			"--plan-type", "UPFRONT",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q stdout=%q", runErr, stderr, stdout)
	}
	var got struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v; stdout=%q", err, stdout)
	}
	if len(got.Data) != 1 || got.Data[0].ID != "plan-upfront" {
		t.Fatalf("expected only plan-upfront, got %#v", got.Data)
	}
}

func TestSubscriptionsPricingMonthlyCommitmentDisableFiltersExcludedTerritories(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/subscriptions/8000000001/planAvailabilities" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPlanAvailabilities","id":"plan-1","attributes":{"planType":"MONTHLY","availableInNewTerritories":true}}]}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities/plan-1" && req.Method == http.MethodPatch:
			var payload asc.SubscriptionPlanAvailabilityUpdateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if len(payload.Data.Relationships.AvailableTerritories.Data) != 0 {
				t.Fatalf("expected empty territory list, got %#v", payload.Data.Relationships.AvailableTerritories.Data)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionPlanAvailabilities","id":"plan-1","attributes":{"planType":"MONTHLY","availableInNewTerritories":true}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "monthly-commitment", "disable",
			"--subscription-id", "8000000001",
			"--territories", "United States,Norway,Singapore",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q stdout=%q", runErr, stderr, stdout)
	}

	if !strings.Contains(stdout, `"id":"plan-1"`) {
		t.Fatalf("expected plan availability response, got %q", stdout)
	}
	if !strings.Contains(stderr, "Warning: monthly-commitment billing is unavailable in USA,SGP") {
		t.Fatalf("expected excluded territory warning, got %q", stderr)
	}
}

func TestSubscriptionsPricingMonthlyCommitmentEnableRejectsPriceOutsideRange(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/subscriptions/8000000001" && req.Method == http.MethodGet:
			body := `{"data":{"type":"subscriptions","id":"8000000001","attributes":{"name":"Yearly","productId":"com.example.yearly","subscriptionPeriod":"ONE_YEAR","state":"APPROVED"}}}`
			return jsonResponse(http.StatusOK, body)
		case req.URL.Path == "/v1/subscriptions/8000000001/prices" && req.Method == http.MethodGet:
			if got := req.URL.Query().Get("filter[territory]"); got != "NOR" {
				t.Fatalf("expected price territory NOR, got %q", got)
			}
			if got := req.URL.Query().Get("filter[planType]"); got != "UPFRONT" {
				t.Fatalf("expected upfront planType filter, got %q", got)
			}
			body := `{
				"data":[{
					"type":"subscriptionPrices","id":"price-1",
					"attributes":{"startDate":"2024-01-01"},
					"relationships":{
						"territory":{"data":{"type":"territories","id":"NOR"}},
						"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-1"}}
					}
				}],
				"included":[
					{"type":"subscriptionPricePoints","id":"pp-1","attributes":{"customerPrice":"120.00"}},
					{"type":"territories","id":"NOR","attributes":{"currency":"NOK"}}
				],
				"links":{"next":""}
			}`
			return jsonResponse(http.StatusOK, body)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "monthly-commitment", "enable",
			"--subscription-id", "8000000001",
			"--price", "15.01",
			"--price-territory", "Norway",
			"--territories", "Norway,Germany",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil || !strings.Contains(err.Error(), "monthly commitment total 180.12 is outside the allowed range [120.00, 180.00]") {
			t.Fatalf("expected pricing range rejection, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestSubscriptionsPricingMonthlyCommitmentEnableCreatesMonthlyPrices(t *testing.T) {
	setupAuth(t)

	var postedPlanType string
	var mutationOrder []string
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/subscriptions/8000000001" && req.Method == http.MethodGet:
			body := `{"data":{"type":"subscriptions","id":"8000000001","attributes":{"name":"Yearly","productId":"com.example.yearly","subscriptionPeriod":"ONE_YEAR","state":"APPROVED"}}}`
			return jsonResponse(http.StatusOK, body)
		case req.URL.Path == "/v1/subscriptions/8000000001/prices" && req.Method == http.MethodGet:
			query := req.URL.Query()
			switch query.Get("filter[planType]") {
			case "UPFRONT":
				body := `{
					"data":[{
						"type":"subscriptionPrices","id":"price-upfront",
						"attributes":{"planType":"UPFRONT","startDate":"2024-01-01"},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"NOR"}},
							"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-upfront"}}
						}
					}],
					"included":[
						{"type":"subscriptionPricePoints","id":"pp-upfront","attributes":{"customerPrice":"120.00"}},
						{"type":"territories","id":"NOR","attributes":{"currency":"NOK"}}
					],
					"links":{"next":""}
				}`
				return jsonResponse(http.StatusOK, body)
			case "MONTHLY":
				return jsonResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`)
			default:
				t.Fatalf("unexpected prices query: %q", req.URL.RawQuery)
				return nil, nil
			}
		case req.URL.Path == "/v1/subscriptions/8000000001/pricePoints" && req.Method == http.MethodGet:
			body := `{"data":[{"type":"subscriptionPricePoints","id":"pp-monthly","attributes":{"customerPrice":"10.00","proceeds":"7.00"}}],"links":{"next":""}}`
			return jsonResponse(http.StatusOK, body)
		case req.URL.Path == "/v1/subscriptionPrices" && req.Method == http.MethodPost:
			mutationOrder = append(mutationOrder, "price")
			var payload asc.SubscriptionPriceCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode create price payload: %v", err)
			}
			if payload.Data.Attributes == nil || payload.Data.Attributes.PlanType != asc.SubscriptionPlanTypeMonthly {
				t.Fatalf("expected planType MONTHLY, got %#v", payload.Data.Attributes)
			}
			if payload.Data.Relationships == nil || payload.Data.Relationships.Territory == nil || payload.Data.Relationships.Territory.Data.ID != "NOR" {
				t.Fatalf("expected NOR territory, got %#v", payload.Data.Relationships)
			}
			if payload.Data.Relationships.SubscriptionPricePoint == nil || payload.Data.Relationships.SubscriptionPricePoint.Data.ID != "pp-monthly" {
				t.Fatalf("expected pp-monthly price point, got %#v", payload.Data.Relationships)
			}
			return jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionPrices","id":"price-monthly","attributes":{"planType":"MONTHLY"}}}`)
		case req.URL.Path == "/v1/subscriptions/8000000001/planAvailabilities" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities" && req.Method == http.MethodPost:
			mutationOrder = append(mutationOrder, "availability")
			var payload asc.SubscriptionPlanAvailabilityCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode plan availability payload: %v", err)
			}
			postedPlanType = string(payload.Data.Attributes.PlanType)
			if payload.Data.Attributes.AvailableInNewTerritories != nil {
				t.Fatalf("MONTHLY create must omit availableInNewTerritories, got %#v", payload.Data.Attributes)
			}
			return jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionPlanAvailabilities","id":"plan-1","attributes":{"planType":"MONTHLY","availableInNewTerritories":false}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "monthly-commitment", "enable",
			"--subscription-id", "8000000001",
			"--price", "10.00",
			"--price-territory", "Norway",
			"--territories", "Norway",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q stdout=%q", runErr, stderr, stdout)
	}
	if postedPlanType != string(asc.SubscriptionPlanTypeMonthly) {
		t.Fatalf("expected plan availability planType MONTHLY, got %q", postedPlanType)
	}
	if got := strings.Join(mutationOrder, ","); got != "availability,price" {
		t.Fatalf("expected availability before price creation, got %q", got)
	}
	if !strings.Contains(stdout, `"id":"plan-1"`) {
		t.Fatalf("expected plan availability response, got %q", stdout)
	}
}

func TestSubscriptionsPricingAvailabilityEditMonthlyCommitmentOmitsAvailableInNewTerritories(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/subscriptionPlanAvailabilities" || req.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		var payload asc.SubscriptionPlanAvailabilityCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode plan availability payload: %v", err)
		}
		if payload.Data.Attributes.PlanType != asc.SubscriptionPlanTypeMonthly {
			t.Fatalf("expected MONTHLY plan type, got %#v", payload.Data.Attributes)
		}
		if payload.Data.Attributes.AvailableInNewTerritories != nil {
			t.Fatalf("MONTHLY create must omit availableInNewTerritories, got %#v", payload.Data.Attributes)
		}
		return jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionPlanAvailabilities","id":"plan-1","attributes":{"planType":"MONTHLY"}}}`)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if err := root.Parse([]string{
		"subscriptions", "pricing", "availability", "edit",
		"--subscription-id", "8000000001",
		"--billing-mode", "monthly-commitment",
		"--territories", "Norway",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
}

func TestSubscriptionsPricingMonthlyCommitmentEnableSkipsEquivalentMonthlyPrice(t *testing.T) {
	setupAuth(t)

	subscriptionPricePosts := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/subscriptions/8000000001" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":{"type":"subscriptions","id":"8000000001","attributes":{"subscriptionPeriod":"ONE_YEAR"}}}`)
		case req.URL.Path == "/v1/subscriptions/8000000001/prices" && req.Method == http.MethodGet:
			switch req.URL.Query().Get("filter[planType]") {
			case "UPFRONT":
				return jsonResponse(http.StatusOK, `{
					"data":[{
						"type":"subscriptionPrices","id":"price-upfront",
						"attributes":{"startDate":"2024-01-01"},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"NOR"}},
							"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-upfront"}}
						}
					}],
					"included":[
						{"type":"subscriptionPricePoints","id":"pp-upfront","attributes":{"customerPrice":"120.00"}},
						{"type":"territories","id":"NOR","attributes":{"currency":"NOK"}}
					],
					"links":{"next":""}
				}`)
			case "MONTHLY":
				return jsonResponse(http.StatusOK, `{
					"data":[{
						"type":"subscriptionPrices","id":"price-monthly",
						"attributes":{"startDate":"2024-01-01"},
						"relationships":{
							"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-existing"}}
						}
					}],
					"included":[
						{"type":"subscriptionPricePoints","id":"pp-existing","attributes":{"customerPrice":"10.00"}}
					],
					"links":{"next":""}
				}`)
			default:
				t.Fatalf("unexpected prices query: %q", req.URL.RawQuery)
				return nil, nil
			}
		case req.URL.Path == "/v1/subscriptions/8000000001/pricePoints" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"pp-resolved","attributes":{"customerPrice":"10.00"}}],"links":{"next":""}}`)
		case req.URL.Path == "/v1/subscriptionPrices" && req.Method == http.MethodPost:
			subscriptionPricePosts++
			return jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionPrices","id":"price-created"}}`)
		case req.URL.Path == "/v1/subscriptions/8000000001/planAvailabilities" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities" && req.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionPlanAvailabilities","id":"plan-1","attributes":{"planType":"MONTHLY"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if err := root.Parse([]string{
		"subscriptions", "pricing", "monthly-commitment", "enable",
		"--subscription-id", "8000000001",
		"--price", "10.00",
		"--price-territory", "Norway",
		"--territories", "Norway",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
	if subscriptionPricePosts != 0 {
		t.Fatalf("expected equivalent current MONTHLY price to be reused, got %d create request(s)", subscriptionPricePosts)
	}
}

func TestSubscriptionsPricingMonthlyCommitmentEnableOmitsPlanTypeOnUpdate(t *testing.T) {
	setupAuth(t)

	sentPlanType := false
	sentAvailableInNewTerritories := false
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/subscriptions/8000000001" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":{"type":"subscriptions","id":"8000000001","attributes":{"subscriptionPeriod":"ONE_YEAR"}}}`)
		case req.URL.Path == "/v1/subscriptions/8000000001/prices" && req.Method == http.MethodGet:
			switch req.URL.Query().Get("filter[planType]") {
			case "UPFRONT":
				return jsonResponse(http.StatusOK, `{
					"data":[{
						"type":"subscriptionPrices","id":"price-upfront",
						"attributes":{"startDate":"2024-01-01"},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"NOR"}},
							"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-upfront"}}
						}
					}],
					"included":[
						{"type":"subscriptionPricePoints","id":"pp-upfront","attributes":{"customerPrice":"120.00"}},
						{"type":"territories","id":"NOR","attributes":{"currency":"NOK"}}
					],
					"links":{"next":""}
				}`)
			case "MONTHLY":
				return jsonResponse(http.StatusOK, `{
					"data":[{
						"type":"subscriptionPrices","id":"price-monthly",
						"attributes":{"startDate":"2024-01-01"},
						"relationships":{
							"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-monthly"}}
						}
					}],
					"included":[
						{"type":"subscriptionPricePoints","id":"pp-monthly","attributes":{"customerPrice":"10.00"}}
					],
					"links":{"next":""}
				}`)
			default:
				t.Fatalf("unexpected prices query: %q", req.URL.RawQuery)
				return nil, nil
			}
		case req.URL.Path == "/v1/subscriptions/8000000001/pricePoints" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"pp-monthly","attributes":{"customerPrice":"10.00"}}],"links":{"next":""}}`)
		case req.URL.Path == "/v1/subscriptions/8000000001/planAvailabilities" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPlanAvailabilities","id":"plan-1","attributes":{"planType":"MONTHLY"}}]}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities/plan-1" && req.Method == http.MethodPatch:
			var payload struct {
				Data struct {
					Attributes map[string]any `json:"attributes"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode update payload: %v", err)
			}
			_, sentPlanType = payload.Data.Attributes["planType"]
			_, sentAvailableInNewTerritories = payload.Data.Attributes["availableInNewTerritories"]
			return jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionPlanAvailabilities","id":"plan-1","attributes":{"planType":"MONTHLY"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if err := root.Parse([]string{
		"subscriptions", "pricing", "monthly-commitment", "enable",
		"--subscription-id", "8000000001",
		"--price", "10.00",
		"--price-territory", "Norway",
		"--territories", "Norway",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
	if sentPlanType {
		t.Fatal("update payload must not include create-only planType")
	}
	if sentAvailableInNewTerritories {
		t.Fatal("MONTHLY update payload must not include availableInNewTerritories")
	}
}
