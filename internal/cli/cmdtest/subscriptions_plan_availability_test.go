package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestSubscriptionsPricingPlanAvailabilityHelp(t *testing.T) {
	root := RootCommand("1.2.3")

	pricing := findSubcommand(root, "subscriptions", "pricing")
	if pricing == nil {
		t.Fatal("expected subscriptions pricing command")
		return
	}
	if usage := pricing.UsageFunc(pricing); !strings.Contains(usage, "plan-availability") {
		t.Fatalf("expected pricing help to list plan-availability, got %q", usage)
	}

	group := findSubcommand(root, "subscriptions", "pricing", "plan-availability")
	if group == nil {
		t.Fatal("expected plan-availability command group")
		return
	}
	groupUsage := group.UsageFunc(group)
	for _, want := range []string{"show", "set"} {
		if !strings.Contains(groupUsage, want) {
			t.Fatalf("expected plan-availability help to list %q, got %q", want, groupUsage)
		}
	}

	setCmd := findSubcommand(root, "subscriptions", "pricing", "plan-availability", "set")
	if setCmd == nil {
		t.Fatal("expected plan-availability set command")
		return
	}
	setUsage := setCmd.UsageFunc(setCmd)
	for _, want := range []string{"--territories", "--available-in-new-territories", "--confirm", "--plan-type"} {
		if !strings.Contains(setUsage, want) {
			t.Fatalf("expected plan-availability set help to document %q, got %q", want, setUsage)
		}
	}
}

func TestSubscriptionsPricingPlanAvailabilityValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "show missing subscription",
			args:    []string{"subscriptions", "pricing", "plan-availability", "show"},
			wantErr: "--subscription-id is required",
		},
		{
			name:    "show rejects positional arguments",
			args:    []string{"subscriptions", "pricing", "plan-availability", "show", "sub-1"},
			wantErr: `unexpected argument "sub-1"`,
		},
		{
			name:    "set missing subscription",
			args:    []string{"subscriptions", "pricing", "plan-availability", "set", "--plan-type", "UPFRONT", "--territories", "USA", "--confirm"},
			wantErr: "--subscription-id is required",
		},
		{
			name:    "set missing plan type",
			args:    []string{"subscriptions", "pricing", "plan-availability", "set", "--subscription-id", "sub-1", "--territories", "USA", "--confirm"},
			wantErr: "--plan-type is required",
		},
		{
			name:    "set invalid plan type",
			args:    []string{"subscriptions", "pricing", "plan-availability", "set", "--subscription-id", "sub-1", "--plan-type", "annual", "--territories", "USA", "--confirm"},
			wantErr: "--plan-type must be one of: MONTHLY, UPFRONT",
		},
		{
			name:    "set missing territories",
			args:    []string{"subscriptions", "pricing", "plan-availability", "set", "--subscription-id", "sub-1", "--plan-type", "UPFRONT", "--confirm"},
			wantErr: "--territories is required",
		},
		{
			name:    "set rejects empty territory list",
			args:    []string{"subscriptions", "pricing", "plan-availability", "set", "--subscription-id", "sub-1", "--plan-type", "UPFRONT", "--territories", " ", "--confirm"},
			wantErr: "asc web subscriptions availability remove-from-sale",
		},
		{
			name:    "set missing confirm",
			args:    []string{"subscriptions", "pricing", "plan-availability", "set", "--subscription-id", "sub-1", "--plan-type", "UPFRONT", "--territories", "USA"},
			wantErr: "--confirm is required",
		},
		{
			name:    "set rejects confirm false after another boolean",
			args:    []string{"subscriptions", "pricing", "plan-availability", "set", "--subscription-id", "sub-1", "--plan-type", "UPFRONT", "--territories", "USA", "--available-in-new-territories", "--confirm", "false"},
			wantErr: "--confirm",
		},
		{
			name:    "set rejects available in new territories for monthly",
			args:    []string{"subscriptions", "pricing", "plan-availability", "set", "--subscription-id", "sub-1", "--plan-type", "MONTHLY", "--territories", "Norway", "--available-in-new-territories", "--confirm"},
			wantErr: "--available-in-new-territories is not supported for MONTHLY plan availability",
		},
		{
			name:    "set rejects unmappable territory",
			args:    []string{"subscriptions", "pricing", "plan-availability", "set", "--subscription-id", "sub-1", "--plan-type", "UPFRONT", "--territories", "show", "--confirm"},
			wantErr: "territory \"show\" could not be mapped",
		},
		{
			name:    "set rejects repeated territories flag",
			args:    []string{"subscriptions", "pricing", "plan-availability", "set", "--subscription-id", "sub-1", "--plan-type", "UPFRONT", "--territories", "USA", "--territories", "CAN", "--confirm"},
			wantErr: "--territories specified multiple times",
		},
		{
			name:    "set rejects monthly territories that are all excluded",
			args:    []string{"subscriptions", "pricing", "plan-availability", "set", "--subscription-id", "sub-1", "--plan-type", "MONTHLY", "--territories", "United States,Singapore", "--confirm"},
			wantErr: "no eligible monthly-commitment territories remain after excluding USA and Singapore",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}

func TestSubscriptionsPricingPlanAvailabilitySetRejectsConfirmFalseAfterBoolean(t *testing.T) {
	setupAuth(t)

	var patched bool
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPatch {
			patched = true
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "plan-availability", "set",
			"--subscription-id", "8000000001",
			"--plan-type", "UPFRONT",
			"--territories", "USA",
			"--available-in-new-territories",
			"--confirm", "false",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil || !strings.Contains(err.Error(), "--confirm") {
			t.Fatalf("expected a --confirm rejection, got %v", err)
		}
	})
	if patched {
		t.Fatal("must not PATCH when --confirm false is supplied")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestSubscriptionsPricingPlanAvailabilityShowIncludesTerritories(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/planAvailabilities" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
		if got := req.URL.Query().Get("include"); got != "availableTerritories" {
			t.Fatalf("expected include=availableTerritories, got %q", got)
		}
		if got := req.URL.Query().Get("limit[availableTerritories]"); got != "50" {
			t.Fatalf("expected limit[availableTerritories]=50, got %q", got)
		}
		body := `{
			"data":[{
				"type":"subscriptionPlanAvailabilities","id":"plan-upfront",
				"attributes":{"planType":"UPFRONT","availableInNewTerritories":true},
				"relationships":{"availableTerritories":{"data":[{"type":"territories","id":"USA"}]}}
			}],
			"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}]
		}`
		return jsonResponse(http.StatusOK, body)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "plan-availability", "show",
			"--subscription-id", "8000000001",
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
			ID            string `json:"id"`
			Relationships struct {
				AvailableTerritories struct {
					Data []struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"availableTerritories"`
			} `json:"relationships"`
		} `json:"data"`
		Included []struct {
			ID string `json:"id"`
		} `json:"included"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("expected valid JSON output, got parse error: %v; stdout=%q", err, stdout)
	}
	if len(got.Data) != 1 || got.Data[0].ID != "plan-upfront" {
		t.Fatalf("expected the plan availability envelope, got %#v", got.Data)
	}
	if len(got.Data[0].Relationships.AvailableTerritories.Data) != 1 {
		t.Fatalf("expected territory linkages in the envelope, got %q", stdout)
	}
	if len(got.Included) != 1 || got.Included[0].ID != "USA" {
		t.Fatalf("expected included territories preserved, got %q", stdout)
	}
}

func TestSubscriptionsPricingPlanAvailabilitySetAppliesTerritoryDiff(t *testing.T) {
	setupAuth(t)

	var patched bool
	var readbacks int
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/subscriptions/8000000001/planAvailabilities" && req.Method == http.MethodGet:
			body := `{"data":[
				{"type":"subscriptionPlanAvailabilities","id":"plan-upfront","attributes":{"planType":"UPFRONT","availableInNewTerritories":false}},
				{"type":"subscriptionPlanAvailabilities","id":"plan-monthly","attributes":{"planType":"MONTHLY","availableInNewTerritories":false}}
			]}`
			return jsonResponse(http.StatusOK, body)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities/plan-upfront/relationships/availableTerritories" && req.Method == http.MethodGet:
			if got := req.URL.Query().Get("limit"); got != "200" {
				t.Fatalf("expected territory relationship limit 200, got %q", got)
			}
			if !patched {
				return jsonResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"DEU"}],"links":{"next":""}}`)
			}
			readbacks++
			return jsonResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"CAN"}],"links":{"next":""}}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities/plan-upfront" && req.Method == http.MethodPatch:
			var payload asc.SubscriptionPlanAvailabilityUpdateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if payload.Data.ID != "plan-upfront" {
				t.Fatalf("expected plan-upfront in payload, got %q", payload.Data.ID)
			}
			if payload.Data.Attributes == nil || payload.Data.Attributes.AvailableInNewTerritories == nil || !*payload.Data.Attributes.AvailableInNewTerritories {
				t.Fatalf("expected availableInNewTerritories=true in payload, got %#v", payload.Data.Attributes)
			}
			territories := payload.Data.Relationships.AvailableTerritories.Data
			if len(territories) != 2 || territories[0].ID != "CAN" || territories[1].ID != "USA" {
				t.Fatalf("expected the complete desired territory set in stable order, got %#v", territories)
			}
			patched = true
			return jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionPlanAvailabilities","id":"plan-upfront","attributes":{"planType":"UPFRONT","availableInNewTerritories":true}}}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities/plan-upfront" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionPlanAvailabilities","id":"plan-upfront","attributes":{"planType":"UPFRONT","availableInNewTerritories":true}}}`)
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
			"subscriptions", "pricing", "plan-availability", "set",
			"--subscription-id", "8000000001",
			"--plan-type", "UPFRONT",
			"--territories", "United States,Canada",
			"--available-in-new-territories",
			"--confirm",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q stdout=%q", runErr, stderr, stdout)
	}
	if !patched {
		t.Fatal("expected a PATCH to plan-upfront")
	}
	if readbacks != 1 {
		t.Fatalf("expected exactly one territory readback after the write, got %d", readbacks)
	}

	var got asc.SubscriptionPlanAvailabilitySetResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("expected valid JSON receipt, got parse error: %v; stdout=%q", err, stdout)
	}
	if got.PlanAvailabilityID != "plan-upfront" || got.PlanType != "UPFRONT" {
		t.Fatalf("unexpected receipt target: %#v", got)
	}
	if !got.Changed || got.Created {
		t.Fatalf("expected changed=true created=false, got %#v", got)
	}
	if strings.Join(got.AddedTerritories, ",") != "CAN" {
		t.Fatalf("expected CAN added, got %#v", got.AddedTerritories)
	}
	if strings.Join(got.RemovedTerritories, ",") != "DEU" {
		t.Fatalf("expected DEU removed, got %#v", got.RemovedTerritories)
	}
	if strings.Join(got.UnchangedTerritories, ",") != "USA" {
		t.Fatalf("expected USA unchanged, got %#v", got.UnchangedTerritories)
	}
	if strings.Join(got.AvailableTerritories, ",") != "CAN,USA" {
		t.Fatalf("expected the verified territory set, got %#v", got.AvailableTerritories)
	}
}

func TestSubscriptionsPricingPlanAvailabilitySetIsNoOpWhenCurrent(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/subscriptions/8000000001/planAvailabilities" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPlanAvailabilities","id":"plan-upfront","attributes":{"planType":"UPFRONT","availableInNewTerritories":false}}]}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities/plan-upfront/relationships/availableTerritories" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"territories","id":"CAN"},{"type":"territories","id":"USA"}],"links":{"next":""}}`)
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
			"subscriptions", "pricing", "plan-availability", "set",
			"--subscription-id", "8000000001",
			"--plan-type", "UPFRONT",
			"--territories", "Canada,United States",
			"--confirm",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q stdout=%q", runErr, stderr, stdout)
	}

	var got asc.SubscriptionPlanAvailabilitySetResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("expected valid JSON receipt, got parse error: %v; stdout=%q", err, stdout)
	}
	if got.Changed {
		t.Fatalf("expected changed=false for an already-current territory set, got %#v", got)
	}
	if len(got.AddedTerritories) != 0 || len(got.RemovedTerritories) != 0 {
		t.Fatalf("expected an empty diff, got %#v", got)
	}
	if strings.Join(got.UnchangedTerritories, ",") != "CAN,USA" {
		t.Fatalf("expected both territories unchanged, got %#v", got.UnchangedTerritories)
	}
}

func TestSubscriptionsPricingPlanAvailabilitySetCreatesMissingPlan(t *testing.T) {
	setupAuth(t)

	var created bool
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/subscriptions/8000000001/planAvailabilities" && req.Method == http.MethodGet:
			if created {
				return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPlanAvailabilities","id":"plan-monthly","attributes":{"planType":"MONTHLY","availableInNewTerritories":false}}]}`)
			}
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities" && req.Method == http.MethodPost:
			var payload asc.SubscriptionPlanAvailabilityCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if payload.Data.Attributes.PlanType != asc.SubscriptionPlanTypeMonthly {
				t.Fatalf("expected MONTHLY planType, got %q", payload.Data.Attributes.PlanType)
			}
			if payload.Data.Attributes.AvailableInNewTerritories != nil {
				t.Fatalf("expected no availableInNewTerritories for MONTHLY, got %#v", payload.Data.Attributes.AvailableInNewTerritories)
			}
			territories := payload.Data.Relationships.AvailableTerritories.Data
			if len(territories) != 1 || territories[0].ID != "NOR" {
				t.Fatalf("expected NOR territory, got %#v", territories)
			}
			created = true
			return jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionPlanAvailabilities","id":"plan-monthly","attributes":{"planType":"MONTHLY","availableInNewTerritories":false}}}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities/plan-monthly/relationships/availableTerritories" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"territories","id":"NOR"}],"links":{"next":""}}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities/plan-monthly" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionPlanAvailabilities","id":"plan-monthly","attributes":{"planType":"MONTHLY","availableInNewTerritories":false}}}`)
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
			"subscriptions", "pricing", "plan-availability", "set",
			"--subscription-id", "8000000001",
			"--plan-type", "MONTHLY",
			"--territories", "Norway,United States",
			"--confirm",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q stdout=%q", runErr, stderr, stdout)
	}
	if !created {
		t.Fatal("expected a POST creating the missing plan availability")
	}
	if !strings.Contains(stderr, "Warning: monthly-commitment billing is unavailable in USA") {
		t.Fatalf("expected the excluded-territory warning on stderr, got %q", stderr)
	}

	var got asc.SubscriptionPlanAvailabilitySetResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("expected valid JSON receipt, got parse error: %v; stdout=%q", err, stdout)
	}
	if !got.Created || !got.Changed {
		t.Fatalf("expected created=true changed=true, got %#v", got)
	}
	if strings.Join(got.AddedTerritories, ",") != "NOR" {
		t.Fatalf("expected NOR added, got %#v", got.AddedTerritories)
	}
	if strings.Join(got.ExcludedTerritories, ",") != "USA" {
		t.Fatalf("expected USA reported as excluded, got %#v", got.ExcludedTerritories)
	}
}

func TestSubscriptionsPricingPlanAvailabilitySetFailsWhenReadbackDiffers(t *testing.T) {
	setupAuth(t)

	var patched bool
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/subscriptions/8000000001/planAvailabilities" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPlanAvailabilities","id":"plan-upfront","attributes":{"planType":"UPFRONT","availableInNewTerritories":false}}]}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities/plan-upfront/relationships/availableTerritories" && req.Method == http.MethodGet:
			if !patched {
				return jsonResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"}],"links":{"next":""}}`)
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"}],"links":{"next":""}}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities/plan-upfront" && req.Method == http.MethodPatch:
			patched = true
			return jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionPlanAvailabilities","id":"plan-upfront","attributes":{"planType":"UPFRONT"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "plan-availability", "set",
			"--subscription-id", "8000000001",
			"--plan-type", "UPFRONT",
			"--territories", "United States,Canada",
			"--confirm",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil || !strings.Contains(err.Error(), "CAN") {
			t.Fatalf("expected a readback verification failure naming CAN, got %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout on verification failure, got %q", stdout)
	}
}

func TestSubscriptionsPricingPlanAvailabilitySetRendersTableReceipt(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/subscriptions/8000000001/planAvailabilities" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPlanAvailabilities","id":"plan-upfront","attributes":{"planType":"UPFRONT","availableInNewTerritories":false}}]}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities/plan-upfront/relationships/availableTerritories" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"}],"links":{"next":""}}`)
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
			"subscriptions", "pricing", "plan-availability", "set",
			"--subscription-id", "8000000001",
			"--plan-type", "UPFRONT",
			"--territories", "United States",
			"--confirm",
			"--output", "table",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q stdout=%q", runErr, stderr, stdout)
	}
	if strings.HasPrefix(strings.TrimSpace(stdout), "{") {
		t.Fatalf("expected a rendered table receipt, got JSON: %q", stdout)
	}
	for _, want := range []string{"Plan Availability", "Plan Type", "Changed", "unchanged", "USA"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected table output to contain %q, got %q", want, stdout)
		}
	}
}

func TestSubscriptionsPricingPlanAvailabilitySetOmitsMonthlyWarningWithoutConfirm(t *testing.T) {
	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"subscriptions", "pricing", "plan-availability", "set",
			"--subscription-id", "sub-1",
			"--plan-type", "MONTHLY",
			"--territories", "United States,Norway",
		}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected --confirm to be required, got %q", stderr)
	}
	if strings.Contains(stderr, "Warning:") {
		t.Fatalf("rejected invocation warned about a write it never attempted: %q", stderr)
	}
}

func TestSubscriptionsPricingPlanAvailabilitySetFailsWhenAttributeReadbackDiffers(t *testing.T) {
	setupAuth(t)

	var patched bool
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/subscriptions/8000000001/planAvailabilities" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPlanAvailabilities","id":"plan-upfront","attributes":{"planType":"UPFRONT","availableInNewTerritories":false}}]}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities/plan-upfront/relationships/availableTerritories" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"}],"links":{"next":""}}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities/plan-upfront" && req.Method == http.MethodPatch:
			patched = true
			return jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionPlanAvailabilities","id":"plan-upfront","attributes":{"planType":"UPFRONT","availableInNewTerritories":true}}}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities/plan-upfront" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionPlanAvailabilities","id":"plan-upfront","attributes":{"planType":"UPFRONT","availableInNewTerritories":false}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "plan-availability", "set",
			"--subscription-id", "8000000001",
			"--plan-type", "UPFRONT",
			"--territories", "United States",
			"--available-in-new-territories",
			"--confirm",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil || !strings.Contains(err.Error(), "availableInNewTerritories") {
			t.Fatalf("expected availableInNewTerritories readback failure, got %v", err)
		}
	})
	if !patched {
		t.Fatal("expected a PATCH before attribute readback")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on verification failure, got %q", stdout)
	}
}

func TestSubscriptionsPricingPlanAvailabilityShowTableIncludesTerritories(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/planAvailabilities" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
		body := `{
			"data":[{
				"type":"subscriptionPlanAvailabilities","id":"plan-upfront",
				"attributes":{"planType":"UPFRONT","availableInNewTerritories":true},
				"relationships":{
					"availableTerritories":{
						"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"CAN"}],
						"meta":{"paging":{"total":2,"limit":50}}
					}
				}
			}]
		}`
		return jsonResponse(http.StatusOK, body)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "plan-availability", "show",
			"--subscription-id", "8000000001",
			"--output", "table",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q stdout=%q", runErr, stderr, stdout)
	}
	if strings.Contains(stderr, "Warning:") {
		t.Fatalf("did not expect a truncated-include warning, got %q", stderr)
	}
	for _, want := range []string{"Plan Type", "Territories", "USA", "CAN"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected table output to contain %q, got %q", want, stdout)
		}
	}
}

func TestSubscriptionsPricingPlanAvailabilityShowWarnsWhenIncludedTerritoriesAreCapped(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/planAvailabilities" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
		body := `{
			"data":[{
				"type":"subscriptionPlanAvailabilities","id":"plan-upfront",
				"attributes":{"planType":"UPFRONT","availableInNewTerritories":true},
				"relationships":{
					"availableTerritories":{
						"data":[{"type":"territories","id":"USA"}],
						"meta":{"paging":{"total":175,"limit":50}}
					}
				}
			}]
		}`
		return jsonResponse(http.StatusOK, body)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "plan-availability", "show",
			"--subscription-id", "8000000001",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q stdout=%q", runErr, stderr, stdout)
	}
	if !strings.Contains(stderr, "plan-upfront") || !strings.Contains(stderr, "175") {
		t.Fatalf("expected a truncated-include warning naming the plan and total, got %q", stderr)
	}
	if strings.Contains(stdout, `"subscriptionId"`) {
		t.Fatalf("expected Apple's unmodified envelope on stdout, got %q", stdout)
	}
}

func TestSubscriptionsPricingPlanAvailabilitySetAcceptsSpacedBooleanAfterConfirm(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/subscriptions/8000000001/planAvailabilities" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPlanAvailabilities","id":"plan-upfront","attributes":{"planType":"UPFRONT","availableInNewTerritories":false}}]}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities/plan-upfront/relationships/availableTerritories" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"}],"links":{"next":""}}`)
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
			"subscriptions", "pricing", "plan-availability", "set",
			"--subscription-id", "8000000001",
			"--plan-type", "UPFRONT",
			"--territories", "USA",
			"--confirm",
			"--available-in-new-territories", "false",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q stdout=%q", runErr, stderr, stdout)
	}

	var got asc.SubscriptionPlanAvailabilitySetResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("expected valid JSON receipt, got parse error: %v; stdout=%q", err, stdout)
	}
	if got.Changed {
		t.Fatalf("expected a no-op for the current USA set, got %#v", got)
	}
}

func TestSubscriptionsPricingPlanAvailabilitySetAcceptsTerritoriesAfterSpacedBoolean(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/subscriptions/8000000001/planAvailabilities" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPlanAvailabilities","id":"plan-upfront","attributes":{"planType":"UPFRONT","availableInNewTerritories":false}}]}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities/plan-upfront/relationships/availableTerritories" && req.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"}],"links":{"next":""}}`)
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
			"subscriptions", "pricing", "plan-availability", "set",
			"--subscription-id", "8000000001",
			"--plan-type", "UPFRONT",
			"--available-in-new-territories", "false",
			"--territories", "USA",
			"--confirm",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q stdout=%q", runErr, stderr, stdout)
	}

	var got asc.SubscriptionPlanAvailabilitySetResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("expected valid JSON receipt, got parse error: %v; stdout=%q", err, stdout)
	}
	if got.Changed {
		t.Fatalf("expected a no-op for the current USA set, got %#v", got)
	}
}

func TestSubscriptionsPricingMonthlyCommitmentListTableOmitsTerritoriesColumn(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/planAvailabilities" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
		return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPlanAvailabilities","id":"plan-monthly","attributes":{"planType":"MONTHLY","availableInNewTerritories":false}}]}`)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "monthly-commitment", "list",
			"--subscription-id", "8000000001",
			"--output", "table",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q stdout=%q", runErr, stderr, stdout)
	}
	if strings.Count(stdout, "Territories") != 1 || !strings.Contains(stdout, "Available In New Territories") {
		t.Fatalf("monthly-commitment list table should keep the original three columns, got %q", stdout)
	}
	if !strings.Contains(stdout, "plan-monthly") {
		t.Fatalf("expected the plan ID in the table, got %q", stdout)
	}
}
