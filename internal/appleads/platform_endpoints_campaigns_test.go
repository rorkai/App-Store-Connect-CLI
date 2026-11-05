package appleads

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestPlatformEndpointCommandPathsAreUnique(t *testing.T) {
	seen := map[string]string{}
	for _, spec := range PlatformEndpointSpecs() {
		command := strings.Join(spec.CommandPath, " ")
		if previous, ok := seen[command]; ok {
			t.Fatalf("duplicate platform command %q from %q and %q", command, previous, spec.Name)
		}
		seen[command] = spec.Name
	}
	for _, command := range [][]string{{"targeting-keywords", "delete-bulk"}, {"negative-keywords", "delete-bulk"}} {
		if _, ok := PlatformEndpointByCommandPath(command...); ok {
			t.Fatalf("Platform API v1 must not register %q", strings.Join(command, " "))
		}
	}
}

func TestPlatformCampaignIdentifiersAndSharedBudgetContexts(t *testing.T) {
	for _, command := range [][]string{
		{"campaigns", "view"},
		{"ad-groups", "view"},
		{"ads", "view"},
		{"targeting-keywords", "view"},
		{"negative-keywords", "view"},
		{"budget-orders", "view"},
	} {
		spec, ok := PlatformEndpointByCommandPath(command...)
		if !ok || len(spec.PathParams) != 1 || spec.PathParams[0].Type != ParamString {
			t.Errorf("%q must use one generic string ID: %+v", strings.Join(command, " "), spec.PathParams)
		}
	}
	locales, ok := PlatformEndpointByCommandPath("apps", "locales", "find")
	if !ok || len(locales.PathParams) != 1 || locales.PathParams[0].Name != "adamId" || locales.PathParams[0].Type != ParamInt {
		t.Fatalf("apps locales find adamId contract = %+v", locales.PathParams)
	}

	wants := map[string]ContextKind{
		"budget-orders create": ContextNone,
		"budget-orders update": ContextNone,
		"budget-orders delete": ContextNone,
		"budget-orders view":   ContextAdAccountOptional,
		"budget-orders find":   ContextAdAccountOptional,
	}
	for command, want := range wants {
		spec, ok := PlatformEndpointByCommandPath(strings.Fields(command)...)
		if !ok || spec.Context != want {
			t.Errorf("%q context = %d, want %d", command, spec.Context, want)
		}
	}
}

func TestPlatformCampaignCLIOverridesPreserveSDKFixtureContracts(t *testing.T) {
	for _, command := range []string{"targeting-keywords find", "negative-keywords find"} {
		spec, ok := PlatformEndpointByCommandPath(strings.Fields(command)...)
		if !ok {
			t.Fatalf("missing %q", command)
		}
		if !spec.BodyOptional {
			t.Fatalf("%q BodyOptional = false, want the independent SDK fixture contract", command)
		}
		if !spec.CLIRequiresBody {
			t.Fatalf("%q CLIRequiresBody = false, want the documented CLI selector requirement", command)
		}
		if spec.BodyFileExample != "query.json" {
			t.Fatalf("%q BodyFileExample = %q, want query.json", command, spec.BodyFileExample)
		}
	}

	campaign, ok := PlatformEndpointByCommandPath("campaigns", "create")
	if !ok {
		t.Fatal("missing campaigns create")
	}
	if campaign.RequiresConfirm {
		t.Fatal("campaign creation must remain non-destructive in fixture confirmation metadata")
	}
	if !campaign.RiskConfirm || campaign.RiskConfirmBodyField != "status" || campaign.RiskConfirmBodyValue != "PAUSED" {
		t.Fatalf("campaign risk metadata = %+v, want separate paused spend acknowledgement", campaign)
	}

	budget, ok := PlatformEndpointByCommandPath("budget-orders", "create")
	if !ok {
		t.Fatal("missing budget-orders create")
	}
	if budget.RequiresConfirm {
		t.Fatal("budget creation must remain non-destructive in fixture confirmation metadata")
	}
	if !budget.RiskConfirm || budget.RiskConfirmBodyField != "" {
		t.Fatalf("budget risk metadata = %+v, want unconditional spend acknowledgement", budget)
	}
}

func TestPlatformCampaignUpdateAndBudgetUpdateCarryRiskMetadata(t *testing.T) {
	campaign, ok := PlatformEndpointByCommandPath("campaigns", "update")
	if !ok {
		t.Fatal("missing campaigns update")
	}
	if !campaign.RiskConfirm || campaign.RiskConfirmBodyField != "status" || campaign.RiskConfirmBodyValue != "PAUSED" {
		t.Fatalf("campaign update risk metadata = %+v, want paused-only safe update", campaign)
	}
	if got, want := campaign.RiskConfirmAllowedBodyFields, []string{"name", "status"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("campaign update safe fields = %v, want %v", got, want)
	}

	budget, ok := PlatformEndpointByCommandPath("budget-orders", "update")
	if !ok {
		t.Fatal("missing budget-orders update")
	}
	if !budget.RiskConfirm || budget.RiskConfirmBodyField != "" {
		t.Fatalf("budget update risk metadata = %+v, want unconditional spend acknowledgement", budget)
	}
}

func TestPlatformSpendRiskMutationMetadata(t *testing.T) {
	wants := []string{
		"ad-groups create",
		"ad-groups update",
		"ads create",
		"ads update",
		"budget-orders create",
		"budget-orders update",
		"campaigns create",
		"campaigns update",
		"targeting-keywords create",
		"targeting-keywords update",
		"targeting-keywords create-bulk",
		"targeting-keywords update-bulk",
		"negative-keywords create",
		"negative-keywords update",
		"negative-keywords create-bulk",
		"negative-keywords update-bulk",
		"ad-accounts create",
	}
	for _, command := range wants {
		spec, ok := PlatformEndpointByCommandPath(strings.Fields(command)...)
		if !ok {
			t.Fatalf("missing %q", command)
		}
		if !spec.RiskConfirm {
			t.Errorf("%q RiskConfirm = false, want spend-risk confirmation", command)
		}
	}
}

func TestPlatformGeoSearchSupportsPagePagination(t *testing.T) {
	spec, ok := PlatformEndpointByCommandPath("geo", "search")
	if !ok {
		t.Fatal("missing geo search endpoint")
	}
	if !spec.SupportsPaginate {
		t.Fatal("geo search must support --paginate")
	}
	if param := findQueryParam(spec, "pageSize"); param.Name != "pageSize" || param.Flag != "page-size" {
		t.Fatalf("geo page size parameter = %+v, want pageSize/page-size", param)
	}
	if param := findQueryParam(spec, "offset"); param.Name != "offset" || param.Flag != "offset" {
		t.Fatalf("geo offset parameter = %+v, want offset/offset", param)
	}
}

func TestPlatformCampaignSpecsRouteRepresentativeHTTPContracts(t *testing.T) {
	type requestRecord struct {
		method  string
		host    string
		path    string
		query   string
		body    string
		context string
	}
	var requests []requestRecord
	client, err := NewClient(
		Credentials{AccessToken: "ACCESS", AdAccountID: "account-string"},
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var body []byte
			if req.Body != nil {
				var readErr error
				body, readErr = io.ReadAll(req.Body)
				if readErr != nil {
					t.Fatal(readErr)
				}
			}
			requests = append(requests, requestRecord{
				method:  req.Method,
				host:    req.URL.Host,
				path:    req.URL.Path,
				query:   req.URL.RawQuery,
				body:    string(body),
				context: req.Header.Get("X-AP-Context"),
			})
			return jsonResponse(http.StatusOK, `{}`), nil
		})}),
	)
	if err != nil {
		t.Fatal(err)
	}

	campaign, _ := PlatformEndpointByCommandPath("campaigns", "update")
	if _, err := client.Do(context.Background(), campaign, map[string]string{"id": "campaign-string"}, nil, []byte(`{"status":"PAUSED"}`)); err != nil {
		t.Fatal(err)
	}
	geo, _ := PlatformEndpointByCommandPath("geo", "search")
	if _, err := client.Do(context.Background(), geo, nil, url.Values{"supplySource": {"APPSTORE"}, "query": {"San Francisco"}}, nil); err != nil {
		t.Fatal(err)
	}
	createBudget, _ := PlatformEndpointByCommandPath("budget-orders", "create")
	if _, err := client.Do(context.Background(), createBudget, nil, nil, []byte(`{"name":"FY27"}`)); err != nil {
		t.Fatal(err)
	}
	viewBudget, _ := PlatformEndpointByCommandPath("budget-orders", "view")
	if _, err := client.Do(context.Background(), viewBudget, map[string]string{"id": "budget-string"}, nil, nil); err != nil {
		t.Fatal(err)
	}

	wants := []requestRecord{
		{method: "PUT", host: "api.ads.apple.com", path: "/v1/campaigns/campaign-string", body: `{"status":"PAUSED"}`, context: "adAccountId=account-string;"},
		{method: "GET", host: "api.ads.apple.com", path: "/v1/search/geo", query: "query=San+Francisco&supplySource=APPSTORE", context: "adAccountId=account-string;"},
		{method: "POST", host: "api.ads.apple.com", path: "/v1/shared-budgets", body: `{"name":"FY27"}`},
		{method: "GET", host: "api.ads.apple.com", path: "/v1/shared-budgets/budget-string", context: "adAccountId=account-string;"},
	}
	if len(requests) != len(wants) {
		t.Fatalf("request count = %d, want %d", len(requests), len(wants))
	}
	for i := range wants {
		if requests[i] != wants[i] {
			t.Errorf("request[%d] = %+v, want %+v", i, requests[i], wants[i])
		}
	}

	clientWithoutAccount, err := NewClient(
		Credentials{AccessToken: "ACCESS"},
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("X-AP-Context"); got != "" {
				t.Errorf("optional shared-budget context = %q, want omitted", got)
			}
			return jsonResponse(http.StatusOK, `{}`), nil
		})}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clientWithoutAccount.Do(context.Background(), viewBudget, map[string]string{"id": "budget-string"}, nil, nil); err != nil {
		t.Fatal(err)
	}
}
