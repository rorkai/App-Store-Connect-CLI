package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type adsRoundTripFunc func(*http.Request) (*http.Response, error)

func (f adsRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestAdsRejectsInvalidInputBeforeNetwork(t *testing.T) {
	dir := t.TempDir()
	missingPayload := filepath.Join(dir, "missing.json")
	payloadPath := filepath.Join(dir, "ad-account.json")
	if err := os.WriteFile(payloadPath, []byte(`{"name":"Disposable","productFeatures":["APPSTORE_APP_MANUAL"]}`), 0o600); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	const riskConfirmErr = "Error: --confirm is required to acknowledge potential Apple Ads spend, billing, delivery, targeting, or access impact"

	tests := []struct {
		name      string
		adAccount string
		noToken   bool
		args      []string
		wantErr   string
	}{
		{
			name:    "api request non-Apple URL",
			args:    []string{"ads", "api", "request", "--path", "https://example.com/v1/campaigns"},
			wantErr: "Error: --path must be an Apple Ads Platform API v1 URL",
		},
		{
			name:      "api request unexpected args",
			adAccount: "123456",
			args:      []string{"ads", "api", "request", "--path", "v1/campaigns", "--output", "json", "unexpected"},
			wantErr:   `Error: unexpected argument "unexpected"`,
		},
		{
			name:      "api request ad account path mismatch",
			adAccount: "CONTEXT_ACCOUNT",
			args:      []string{"ads", "api", "request", "--method", "GET", "--path", "v1/ad-accounts/PATH_ACCOUNT", "--output", "json"},
			wantErr:   `Error: --ad-account "CONTEXT_ACCOUNT" must match the v1/ad-accounts path ID "PATH_ACCOUNT"`,
		},
		{
			name:      "api request absolute URL ad account path mismatch",
			adAccount: "CONTEXT_ACCOUNT",
			args:      []string{"ads", "api", "request", "--method", "GET", "--path", "https://api.ads.apple.com/v1/ad-accounts/PATH_ACCOUNT", "--output", "json"},
			wantErr:   `Error: --ad-account "CONTEXT_ACCOUNT" must match the v1/ad-accounts path ID "PATH_ACCOUNT"`,
		},
		{
			name:    "api request invalid output",
			args:    []string{"ads", "api", "request", "--path", "v1/me", "--output", "invalid"},
			wantErr: `Error: --output must be one of: json (got "invalid")`,
		},
		{
			name:      "api request multipart upload",
			adAccount: "AD_ACCOUNT",
			args:      []string{"ads", "api", "request", "--method", "POST", "--path", "v1/assets/upload", "--file", missingPayload},
			wantErr:   "Error: v1/assets/upload requires multipart/form-data; use `asc ads assets upload --file IMAGE --brand BRAND_ID --ad-account AD_ACCOUNT_ID` instead of the JSON raw request command",
		},
		{
			name:    "endpoint invalid output before reading body",
			args:    []string{"ads", "ad-accounts", "create", "--file", missingPayload, "--output", "invalid"},
			wantErr: `Error: --output must be one of: json (got "invalid")`,
		},
		{
			name:    "api request missing ad account",
			args:    []string{"ads", "api", "request", "--path", "v1/campaigns/123"},
			wantErr: "Error: --ad-account is required (or set ASC_ADS_AD_ACCOUNT_ID or an Ads profile ad_account_id)",
		},
		{
			name:    "api request ad account for context-free endpoint",
			args:    []string{"ads", "api", "request", "--path", "v1/me", "--ad-account", "AD_ACCOUNT"},
			wantErr: "Error: --ad-account is not supported for this context-free endpoint",
		},
		{
			name:    "ad account create without risk confirmation",
			args:    []string{"ads", "ad-accounts", "create", "--file", payloadPath},
			wantErr: riskConfirmErr,
		},
		{
			name:    "auth discover invalid output",
			args:    []string{"ads", "auth", "discover", "--output", "invalid"},
			wantErr: `Error: --output must be one of: table, json (got "invalid")`,
		},
		{
			name:    "auth discover ad account with semicolon",
			args:    []string{"ads", "auth", "discover", "--ad-account", "123;orgId=456", "--output", "json"},
			wantErr: "Error: ads auth discover: --ad-account: invalid ad account ID: semicolons are not allowed",
		},
		{
			name:    "auth discover control-only ad account",
			args:    []string{"ads", "auth", "discover", "--ad-account", "\n", "--output", "json"},
			wantErr: "Error: ads auth discover: --ad-account: invalid ad account ID: control characters are not allowed",
		},
		{
			name:    "recommendation apply without confirm",
			noToken: true,
			args:    []string{"ads", "recommendations", "daily-budgets", "apply", "--file", missingPayload},
			wantErr: riskConfirmErr,
		},
		{
			name:    "report without file",
			noToken: true,
			args:    []string{"ads", "insights", "impression-share", "find"},
			wantErr: "Error: --file is required",
		},
		{
			name:    "auth token without confirm",
			args:    []string{"ads", "auth", "token", "--output", "json"},
			wantErr: "Error: --confirm is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateAdsGuideEnv(t)
			t.Setenv("ASC_ADS_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
			t.Setenv("ASC_ADS_AD_ACCOUNT_ID", test.adAccount)
			if !test.noToken {
				t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
			}
			installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
				return nil, nil
			}))

			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}

func TestAdsPlatformAPIRequestUsesV1HostAndAdAccountContext(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "123")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Host != "api.ads.apple.com" || req.URL.Path != "/v1/ad-accounts/123" {
			t.Fatalf("request = %s %s", req.Method, req.URL.String())
		}
		if got := req.Header.Get("Authorization"); got != "Bearer ACCESS" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := req.Header.Get("X-AP-Context"); got != "adAccountId=123;" {
			t.Fatalf("X-AP-Context = %q", got)
		}
		return adsJSONResponse(`{"result":{"id":"123"}}`), nil
	}))

	root := RootCommand("dev")
	if err := root.Parse([]string{"ads", "api", "request", "--path", "v1/ad-accounts/123", "--output", "json"}); err != nil {
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
	var output struct {
		Result struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if output.Result.ID != "123" {
		t.Fatalf("result = %+v, want preserved v1 result envelope with id 123", output.Result)
	}
}

func TestAdsPlatformAPIRequestOmitsContextForMe(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "AD_ACCOUNT")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "api.ads.apple.com" || req.URL.Path != "/v1/me" {
			t.Fatalf("request URL = %s", req.URL.String())
		}
		if got := req.Header.Get("X-AP-Context"); got != "" {
			t.Fatalf("X-AP-Context = %q, want empty", got)
		}
		return adsJSONResponse(`{"result":{"userId":"1"}}`), nil
	}))

	root := RootCommand("dev")
	if err := root.Parse([]string{"ads", "api", "request", "--path", "v1/me", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	_, stderr := captureOutput(t, func() {
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestAdsPlatformAPIRequestRequiresConfirmForKnownImpactMutations(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "daily budget apply",
			args: []string{"ads", "api", "request", "--method", "POST", "--path", "v1/recommendations/daily-budgets/apply", "--ad-account", "AD_ACCOUNT"},
		},
		{
			name: "daily budget dismiss",
			args: []string{"ads", "api", "request", "--method", "POST", "--path", "v1/recommendations/daily-budgets/dismiss", "--ad-account", "AD_ACCOUNT"},
		},
		{
			name: "target CPA apply",
			args: []string{"ads", "api", "request", "--method", "POST", "--path", "v1/recommendations/target-cpas/apply", "--ad-account", "AD_ACCOUNT"},
		},
		{
			name: "target CPA dismiss",
			args: []string{"ads", "api", "request", "--method", "POST", "--path", "v1/recommendations/target-cpas/dismiss", "--ad-account", "AD_ACCOUNT"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateAdsGuideEnv(t)
			t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
			t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "AD_ACCOUNT")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
			installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
				return nil, nil
			}))

			root := RootCommand("dev")
			if err := root.Parse(tt.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			var runErr error
			_, stderr := captureOutput(t, func() {
				runErr = root.Run(context.Background())
			})
			want := "--confirm is required to acknowledge potential Apple Ads spend, billing, delivery, targeting, or access impact"
			if !errors.Is(runErr, flag.ErrHelp) || runErr.Error() != want || !strings.Contains(stderr, want) {
				t.Fatalf("run error = %v stderr = %q", runErr, stderr)
			}
		})
	}
}

func TestAdsPlatformAPIRequestRequiresConfirmForDelegationReplacement(t *testing.T) {
	tempDir := t.TempDir()
	payloadPath := filepath.Join(tempDir, "delegations.json")
	if err := os.WriteFile(payloadPath, []byte(`{"delegations":[{"resourceId":"RESOURCE","resourceType":"CONTENT_PROVIDER"}]}`), 0o600); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "AD_ACCOUNT")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	root := RootCommand("dev")
	if err := root.Parse([]string{
		"ads", "api", "request",
		"--method", "PUT",
		"--path", "v1/ad-accounts/AD_ACCOUNT",
		"--file", payloadPath,
		"--ad-account", "AD_ACCOUNT",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	_, stderr := captureOutput(t, func() {
		runErr = root.Run(context.Background())
	})
	if !errors.Is(runErr, flag.ErrHelp) || runErr.Error() != "--confirm is required" || !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("run error = %v stderr = %q", runErr, stderr)
	}
}

func TestAdsPlatformAPIRequestDefersBodyDependentConfirmationUntilAfterPayload(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantMethod string
		wantPath   string
	}{
		{
			name:       "paused campaign create",
			method:     http.MethodPost,
			path:       "v1/campaigns",
			body:       `{"status":"PAUSED"}`,
			wantMethod: http.MethodPost,
			wantPath:   "/v1/campaigns",
		},
		{
			name:       "name-only ad-account update",
			method:     http.MethodPut,
			path:       "v1/ad-accounts/AD_ACCOUNT",
			body:       `{"name":"Renamed"}`,
			wantMethod: http.MethodPut,
			wantPath:   "/v1/ad-accounts/AD_ACCOUNT",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payloadPath := writeAdsEvalPayload(t, "payload.json", test.body)
			isolateAdsGuideEnv(t)
			t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
			t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "AD_ACCOUNT")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
			installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != test.wantMethod || req.URL.Path != test.wantPath {
					t.Fatalf("request = %s %s, want %s %s", req.Method, req.URL.Path, test.wantMethod, test.wantPath)
				}
				return adsJSONResponse(`{"data":{"id":"1"}}`), nil
			}))

			stdout, stderr, err := runAdsEvalCommand(
				t,
				"ads", "api", "request",
				"--method", test.method,
				"--path", test.path,
				"--file", payloadPath,
				"--ad-account", "AD_ACCOUNT",
				"--output", "json",
			)
			if err != nil {
				t.Fatalf("run error = %v, want body-dependent confirmation after payload read", err)
			}
			if stderr != "" || !strings.Contains(stdout, `"data"`) {
				t.Fatalf("stdout=%q stderr=%q, want successful raw response", stdout, stderr)
			}
		})
	}
}

func TestAdsPlatformAPIRequestAllowsKnownReadOnlyPostWithoutConfirmation(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "AD_ACCOUNT")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/campaigns/query" {
			t.Fatalf("request = %s %s, want POST /v1/campaigns/query", req.Method, req.URL.Path)
		}
		return adsJSONResponse(`{"data":[]}`), nil
	}))

	stdout, stderr, err := runAdsEvalCommand(
		t,
		"ads", "api", "request",
		"--method", http.MethodPost,
		"--path", "v1/campaigns/query",
		"--ad-account", "AD_ACCOUNT",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("run error = %v, want known read-only POST to remain confirmation-free", err)
	}
	if stderr != "" || !strings.Contains(stdout, `"data"`) {
		t.Fatalf("stdout=%q stderr=%q, want successful raw response", stdout, stderr)
	}
}

func TestAdsPlatformAPIRequestKeepsUnconditionalConfirmationBeforePayloadRead(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	missingPayload := filepath.Join(t.TempDir(), "missing.json")
	stdout, stderr, err := runAdsEvalCommand(
		t,
		"ads", "api", "request",
		"--method", http.MethodPost,
		"--path", "v1/ad-accounts",
		"--file", missingPayload,
		"--output", "json",
	)
	if !errors.Is(err, flag.ErrHelp) || !strings.Contains(stderr, "--confirm is required to acknowledge potential Apple Ads spend, billing, delivery, targeting, or access impact") {
		t.Fatalf("stdout=%q stderr=%q error=%v, want unconditional confirmation before payload read", stdout, stderr, err)
	}
}

func adsJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
