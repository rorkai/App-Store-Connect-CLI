package cmdtest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const retentionOfferResponse = `{"data":{"type":"retentionOffers","id":"ro-1","attributes":{"referenceName":"winter","offerId":"winter_one_month"}},"links":{"self":"https://api.appstoreconnect.apple.com/v1/retentionOffers/ro-1"}}`

const nullWaitResponse = `{"data":{"type":"retentionOffers","id":"ro-1","attributes":{"customerEligibilityPaidSubscriptionDurationInMonths":36,"customerEligibilityWaitBetweenOffersInMonths":null}},"links":{"self":"https://api.appstoreconnect.apple.com/v1/retentionOffers/ro-1"}}`

const retentionOffersResponse = `{"data":[{"type":"retentionOffers","id":"ro-1","attributes":{"referenceName":"winter"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/subscriptions/6446671421/retentionOffers"}}`

const retentionLinkagesResponse = `{"data":[{"type":"retentionOffers","id":"ro-1"}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/subscriptions/6446671421/relationships/retentionOffers"}}`

func TestSubscriptionsRetentionOffersRequestWiring(t *testing.T) {
	const sub = "6446671421"
	tests := []struct {
		name       string
		args       []string
		method     string
		path       string
		query      url.Values
		body       string
		status     int
		response   string
		wantOutput string
	}{
		{
			name:   "list sends sparse fields, includes, and limits",
			args:   []string{"list", "--subscription-id", sub, "--fields", "referenceName,prices", "--include", "prices,subscription", "--price-fields", "subscriptionPricePoint", "--subscription-fields", "name", "--prices-limit", "10", "--limit", "5"},
			method: http.MethodGet,
			path:   "/v1/subscriptions/" + sub + "/retentionOffers",
			query: url.Values{
				"fields[retentionOffers]":      {"referenceName,prices"},
				"include":                      {"prices,subscription"},
				"fields[retentionOfferPrices]": {"subscriptionPricePoint"},
				"fields[subscriptions]":        {"name"},
				"limit[prices]":                {"10"},
				"limit":                        {"5"},
			},
			status: http.StatusOK, response: retentionOffersResponse, wantOutput: retentionOffersResponse,
		},
		{
			name:   "presented list reads retention message offers",
			args:   []string{"presented", "list", "--subscription-id", sub},
			method: http.MethodGet,
			path:   "/v1/subscriptions/" + sub + "/retentionMessageOffers",
			status: http.StatusOK, response: retentionOffersResponse, wantOutput: retentionOffersResponse,
		},
		{
			name:   "links reads retention offer linkages",
			args:   []string{"links", "--subscription-id", sub, "--limit", "5"},
			method: http.MethodGet,
			path:   "/v1/subscriptions/" + sub + "/relationships/retentionOffers",
			query:  url.Values{"limit": {"5"}},
			status: http.StatusOK, response: retentionLinkagesResponse, wantOutput: retentionLinkagesResponse,
		},
		{
			name:   "presented links reads retention message offer linkages",
			args:   []string{"presented", "links", "--subscription-id", sub},
			method: http.MethodGet,
			path:   "/v1/subscriptions/" + sub + "/relationships/retentionMessageOffers",
			status: http.StatusOK, response: retentionLinkagesResponse, wantOutput: retentionLinkagesResponse,
		},
		{
			name:   "view sends includes",
			args:   []string{"view", "--id", "ro-1", "--include", "subscription"},
			method: http.MethodGet,
			path:   "/v1/retentionOffers/ro-1",
			query:  url.Values{"include": {"subscription"}},
			status: http.StatusOK, response: retentionOfferResponse, wantOutput: retentionOfferResponse,
		},
		{
			name: "create inlines price point prices without territories",
			args: []string{
				"create", "--subscription-id", sub, "--reference-name", "winter", "--offer-id", "winter_one_month",
				"--duration", "one_month", "--offer-mode", "PAY_UP_FRONT", "--period-count", "1",
				"--eligibility-paid-months", "3", "--eligibility-wait-months", "6", "--target-subscription-plan-type", "UPFRONT",
				"--price", "PP-USA,PP-FRA",
			},
			method: http.MethodPost,
			path:   "/v1/retentionOffers",
			body: `{"data":{"type":"retentionOffers","attributes":{"referenceName":"winter","offerId":"winter_one_month","duration":"ONE_MONTH","offerMode":"PAY_UP_FRONT","periodCount":1,"customerEligibilityPaidSubscriptionDurationInMonths":3,"customerEligibilityWaitBetweenOffersInMonths":6,"targetSubscriptionPlanType":"UPFRONT"},
				"relationships":{"subscription":{"data":{"type":"subscriptions","id":"6446671421"}},"prices":{"data":[{"type":"retentionOfferPrices","id":"${price-1}"},{"type":"retentionOfferPrices","id":"${price-2}"}]}}},
				"included":[
					{"type":"retentionOfferPrices","id":"${price-1}","relationships":{"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"PP-USA"}}}},
					{"type":"retentionOfferPrices","id":"${price-2}","relationships":{"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"PP-FRA"}}}}]}`,
			status: http.StatusCreated, response: retentionOfferResponse, wantOutput: retentionOfferResponse,
		},
		{
			name:   "update sets paid and wait months",
			args:   []string{"update", "--id", "ro-1", "--eligibility-paid-months", "36", "--eligibility-wait-months", "4"},
			method: http.MethodPatch,
			path:   "/v1/retentionOffers/ro-1",
			body:   `{"data":{"type":"retentionOffers","id":"ro-1","attributes":{"customerEligibilityPaidSubscriptionDurationInMonths":36,"customerEligibilityWaitBetweenOffersInMonths":4}}}`,
			status: http.StatusOK, response: nullWaitResponse, wantOutput: nullWaitResponse,
		},
		{
			name:   "delete prints a receipt",
			args:   []string{"delete", "--id", "ro-1", "--confirm"},
			method: http.MethodDelete,
			path:   "/v1/retentionOffers/ro-1",
			status: http.StatusNoContent, wantOutput: `{"id":"ro-1","deleted":true}`,
		},
		{
			name:   "prices list sends territory filter and included fields",
			args:   []string{"prices", "list", "--id", "ro-1", "--territory", "US,France", "--include", "subscriptionPricePoint", "--price-point-fields", "customerPrice"},
			method: http.MethodGet,
			path:   "/v1/retentionOffers/ro-1/prices",
			query: url.Values{
				"filter[territory]":               {"USA,FRA"},
				"include":                         {"subscriptionPricePoint"},
				"fields[subscriptionPricePoints]": {"customerPrice"},
			},
			status: http.StatusOK, response: `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/retentionOffers/ro-1/prices"}}`,
			wantOutput: `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/retentionOffers/ro-1/prices"}}`,
		},
		{
			name:   "prices links reads price linkages",
			args:   []string{"prices", "links", "--id", "ro-1"},
			method: http.MethodGet,
			path:   "/v1/retentionOffers/ro-1/relationships/prices",
			status: http.StatusOK, response: `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/retentionOffers/ro-1/relationships/prices"}}`,
			wantOutput: `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/retentionOffers/ro-1/relationships/prices"}}`,
		},
		{
			name:   "presented add posts offer linkages",
			args:   []string{"presented", "add", "--subscription-id", sub, "--retention-offer-id", "ro-1,ro-2"},
			method: http.MethodPost,
			path:   "/v1/subscriptions/" + sub + "/relationships/retentionMessageOffers",
			body:   `{"data":[{"type":"retentionOffers","id":"ro-1"},{"type":"retentionOffers","id":"ro-2"}]}`,
			status: http.StatusNoContent, wantOutput: `{"subscriptionId":"6446671421","retentionOfferIds":["ro-1","ro-2"],"action":"added"}`,
		},
		{
			name:   "presented remove deletes offer linkages",
			args:   []string{"presented", "remove", "--subscription-id", sub, "--retention-offer-id", "ro-1", "--confirm"},
			method: http.MethodDelete,
			path:   "/v1/subscriptions/" + sub + "/relationships/retentionMessageOffers",
			body:   `{"data":[{"type":"retentionOffers","id":"ro-1"}]}`,
			status: http.StatusNoContent, wantOutput: `{"subscriptionId":"6446671421","retentionOfferIds":["ro-1"],"action":"removed"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requests++
				if req.Method != test.method || req.URL.Path != test.path {
					t.Errorf("request = %s %s, want %s %s", req.Method, req.URL.Path, test.method, test.path)
				}
				if got := req.URL.Query(); !reflect.DeepEqual(got, test.query) && (len(got) != 0 || len(test.query) != 0) {
					t.Errorf("query = %v, want %v", got, test.query)
				}
				body, _ := io.ReadAll(req.Body)
				assertRetentionJSON(t, "request body", string(body), test.body)
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.response)
			}))
			t.Cleanup(server.Close)
			client := newWinBackOfferTestClient(t, server)
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

			args := append([]string{"subscriptions", "offers", "retention"}, test.args...)
			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitSuccess {
					t.Errorf("exit code = %d, want %d", code, rootcmd.ExitSuccess)
				}
			})
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
			if requests != 1 {
				t.Errorf("requests = %d, want 1", requests)
			}
			assertRetentionJSON(t, "stdout", stdout, test.wantOutput)
		})
	}
}

func TestSubscriptionsRetentionOffersUsageErrors(t *testing.T) {
	create := []string{
		"create", "--subscription-id", "6446671421", "--reference-name", "winter", "--offer-id", "winter_one_month",
		"--duration", "ONE_MONTH", "--offer-mode", "PAY_UP_FRONT", "--period-count", "1", "--eligibility-paid-months", "3",
	}
	with := func(base []string, extra ...string) []string {
		return append(append([]string(nil), base...), extra...)
	}
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"create requires a price point", create, "--price is required"},
		{"create rejects undocumented paid months", with(create, "--price", "PP", "--eligibility-paid-months", "25"), "--eligibility-paid-months must be 1-24, 36, 48, or 60"},
		{"create rejects wait months below two", with(create, "--price", "PP", "--eligibility-wait-months", "1"), "--eligibility-wait-months must be between 2 and 24"},
		{"create rejects unknown duration", with(create, "--price", "PP", "--duration", "TEN_DAYS"), "--duration must be one of"},
		{"update requires a change", []string{"update", "--id", "ro-1"}, "at least one of --eligibility-paid-months"},
		{"delete requires confirm", []string{"delete", "--id", "ro-1"}, "--confirm is required"},
		{"presented remove requires confirm", []string{"presented", "remove", "--subscription-id", "6446671421", "--retention-offer-id", "ro-1"}, "--confirm is required"},
		{"list prices limit requires prices include", []string{"list", "--subscription-id", "6446671421", "--prices-limit", "5"}, "--prices-limit requires --include prices"},
		{"list price fields require prices include", []string{"list", "--subscription-id", "6446671421", "--price-fields", "territory"}, "--price-fields requires --include prices"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			assertWinBackOfferUsageBeforeClient(t, append([]string{"subscriptions", "offers", "retention"}, test.args...), test.wantErr)
		})
	}
}

func assertRetentionJSON(t *testing.T, label, got, want string) {
	t.Helper()
	if got == "" && want == "" {
		return
	}
	var gotValue, wantValue any
	if err := json.Unmarshal([]byte(got), &gotValue); err != nil {
		t.Fatalf("%s is not JSON: %v; %q", label, err, got)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("want %s is not JSON: %v", label, err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Errorf("%s = %s, want %s", label, got, fmt.Sprint(want))
	}
}
