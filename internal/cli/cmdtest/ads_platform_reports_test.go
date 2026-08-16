package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAdsPlatformAppCampaignReportRequest(t *testing.T) {
	requestJSON := `{
  "pagination": {"offset": 0, "pageSize": 20},
  "filters": [{"field": "campaignId", "operator": "EQUALS", "value": ["444555666"]}],
  "groupBy": ["countryOrRegion"],
  "timeRange": {"start": "2025-01-01", "end": "2025-01-31", "timeZone": "ORTZ", "granularity": "DAILY"}
}`
	var expectedBody map[string]any
	if err := json.Unmarshal([]byte(requestJSON), &expectedBody); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		command string
		path    string
	}{
		{command: "ad-groups", path: "/v1/reports/apps/adgroups/query"},
		{command: "ads", path: "/v1/reports/apps/ads/query"},
		{command: "campaigns", path: "/v1/reports/apps/campaigns/query"},
		{command: "keywords", path: "/v1/reports/apps/keywords/query"},
		{command: "search-terms", path: "/v1/reports/apps/searchterms/query"},
	}
	for _, test := range tests {
		t.Run(test.command, func(t *testing.T) {
			isolateAdsGuideEnv(t)
			t.Setenv("ASC_ADS_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
			t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "AD_ACCOUNT")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
			payloadPath := filepath.Join(t.TempDir(), "report.json")
			if err := os.WriteFile(payloadPath, []byte(requestJSON), 0o600); err != nil {
				t.Fatal(err)
			}

			installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost || req.URL.Host != "api.ads.apple.com" || req.URL.Path != test.path || req.URL.RawQuery != "" {
					t.Fatalf("request = %s %s", req.Method, req.URL.String())
				}
				if got := req.Header.Get("X-AP-Context"); got != "adAccountId=AD_ACCOUNT;" {
					t.Fatalf("X-AP-Context = %q", got)
				}
				var body map[string]any
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				if !reflect.DeepEqual(body, expectedBody) {
					t.Fatalf("request body = %#v, want %#v", body, expectedBody)
				}
				return adsJSONResponse(200, `{"result":{"row":[{"campaignId":"123"}]},"pagination":{"totalResults":1}}`), nil
			}))

			root := RootCommand("dev")
			if err := root.Parse([]string{"ads", "reports", "apps", test.command, "--file", payloadPath, "--output", "json"}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			stdout, stderr := captureOutput(t, func() {
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})
			if stderr != "" {
				t.Fatalf("stderr = %q", stderr)
			}
			var output map[string]any
			if err := json.Unmarshal([]byte(stdout), &output); err != nil {
				t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
			}
			if _, ok := output["pagination"]; !ok {
				t.Fatalf("stdout omitted raw pagination envelope: %s", stdout)
			}
		})
	}
}

func TestAdsPlatformSearchTermPopularityUsesRuntimeSortKey(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "AD_ACCOUNT")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	requestJSON := `{
  "timeRange": {"start": "2026-08-02", "end": "2026-08-08", "timeZone": "UTC", "granularity": "WEEKLY_SUN_SAT"},
  "sorting": [{"field": "rankInGenre", "sortOrder": "ASC"}],
  "pagination": {"offset": 0, "pageSize": 20}
}`
	payloadPath := filepath.Join(t.TempDir(), "query.json")
	if err := os.WriteFile(payloadPath, []byte(requestJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Host != "api.ads.apple.com" || req.URL.Path != "/v1/insights/apps/search-term-popularity/query" {
			t.Fatalf("request = %s %s", req.Method, req.URL.String())
		}
		var body struct {
			Sorting []map[string]json.RawMessage `json:"sorting"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(body.Sorting) != 1 || string(body.Sorting[0]["sortOrder"]) != `"ASC"` {
			t.Fatalf("sorting = %#v, want sortOrder ASC", body.Sorting)
		}
		if _, ok := body.Sorting[0]["order"]; ok {
			t.Fatalf("sorting sent documentation-only order key: %#v", body.Sorting)
		}
		return adsJSONResponse(200, `{"result":[],"pagination":{"totalResults":0}}`), nil
	}))

	root := RootCommand("dev")
	if err := root.Parse([]string{"ads", "insights", "search-term-popularity", "find", "--file", payloadPath, "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureOutput(t, func() {
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	if !json.Valid([]byte(stdout)) {
		t.Fatalf("stdout is not JSON: %s", stdout)
	}
}

func TestAdsPlatformChangeHistoryDetailRequest(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "AD_ACCOUNT")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	requests := 0
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/change-history/Campaign.444555666.txn_abc123def456" {
			t.Fatalf("request = %s %s", req.Method, req.URL.String())
		}
		if got := req.URL.Query().Get("limit"); got != "2" {
			t.Fatalf("limit = %q", got)
		}
		switch got := req.URL.Query().Get("offset"); got {
		case "0":
			return adsJSONResponse(200, `{"dataType":"ChangeDetail","pagination":{"totalCount":3,"offset":0,"pageSize":2},"result":[{"detailId":"Campaign.444555666.txn_abc123def456","details":[{"transactionId":"txn_abc123def456","changes":[{"field":"name"},{"field":"status"}]}]}]}`), nil
		case "2":
			return adsJSONResponse(200, `{"dataType":"ChangeDetail","pagination":{"totalCount":3,"offset":2,"pageSize":2},"result":[{"detailId":"Campaign.444555666.txn_abc123def456","details":[{"transactionId":"txn_abc123def456","changes":[{"field":"dailyBudget"}]}]}]}`), nil
		default:
			t.Fatalf("offset = %q", got)
		}
		return nil, nil
	}))

	root := RootCommand("dev")
	if err := root.Parse([]string{"ads", "change-history", "view", "--detail-id", "Campaign.444555666.txn_abc123def456", "--limit", "2", "--paginate", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureOutput(t, func() {
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	if requests != 2 {
		t.Fatalf("request count = %d, want 2", requests)
	}
	var response struct {
		Result []struct {
			Details []struct {
				Changes []json.RawMessage `json:"changes"`
			} `json:"details"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if len(response.Result) != 1 || len(response.Result[0].Details) != 1 || len(response.Result[0].Details[0].Changes) != 3 {
		t.Fatalf("stdout did not aggregate nested changes: %s", stdout)
	}
}

func TestAdsPlatformRecommendationApplyRequiresConfirmBeforeFileAuthOrNetwork(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	root := RootCommand("dev")
	if err := root.Parse([]string{"ads", "recommendations", "daily-budgets", "apply", "--file", filepath.Join(t.TempDir(), "missing.json")}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	_, stderr := captureOutput(t, func() {
		runErr = root.Run(context.Background())
	})
	if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("run error = %v stderr = %q, want confirm usage error", runErr, stderr)
	}
}

func TestAdsPlatformReportRequiresFileBeforeAuthOrNetwork(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	root := RootCommand("dev")
	if err := root.Parse([]string{"ads", "insights", "impression-share", "find"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	_, stderr := captureOutput(t, func() {
		runErr = root.Run(context.Background())
	})
	if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(stderr, "--file is required") {
		t.Fatalf("run error = %v stderr = %q, want file usage error", runErr, stderr)
	}
}
