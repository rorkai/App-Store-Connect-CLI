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

func TestAdsAPIRequestRejectsNonAppleURLsBeforeNetwork(t *testing.T) {
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	root := RootCommand("dev")
	if err := root.Parse([]string{"ads", "api", "request", "--path", "https://example.com/v1/campaigns"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	_, stderr := captureOutput(t, func() {
		runErr = root.Run(context.Background())
	})
	if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(stderr, "Apple Ads Platform API v1 URL") {
		t.Fatalf("run error = %v stderr = %q, want Apple host guardrail", runErr, stderr)
	}
}

func TestAdsAPIRequestRejectsUnexpectedArgsBeforeNetwork(t *testing.T) {
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "123456")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	root := RootCommand("dev")
	if err := root.Parse([]string{"ads", "api", "request", "--path", "v1/campaigns", "--output", "json", "unexpected"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	_, stderr := captureOutput(t, func() {
		runErr = root.Run(context.Background())
	})
	if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(stderr, "unexpected argument(s): unexpected") {
		t.Fatalf("run error = %v stderr = %q, want unexpected argument usage error", runErr, stderr)
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

func TestAdsPlatformAPIRequestRejectsAdAccountPathMismatchBeforeNetwork(t *testing.T) {
	for _, path := range []string{"v1/ad-accounts/PATH_ACCOUNT", "https://api.ads.apple.com/v1/ad-accounts/PATH_ACCOUNT"} {
		t.Run(path, func(t *testing.T) {
			isolateAdsGuideEnv(t)
			t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
			t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "CONTEXT_ACCOUNT")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
			installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected token/network request: %s %s", req.Method, req.URL.String())
				return nil, nil
			}))

			stdout, stderr, err := runAdsEvalCommand(t, "ads", "api", "request", "--method", "GET", "--path", path, "--output", "json")
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want usage error", err)
			}
			if stdout != "" || !strings.Contains(stderr, "must match the v1/ad-accounts path ID") {
				t.Fatalf("stdout=%q stderr=%q error=%v, want path/context mismatch before network", stdout, stderr, err)
			}
		})
	}
}

func TestAdsPlatformAPIRequestRejectsInvalidOutputBeforeNetwork(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	stdout, stderr, err := runAdsEvalCommand(t, "ads", "api", "request", "--path", "v1/me", "--output", "invalid")
	if !isUsageClassError(err) || stdout != "" || !strings.Contains(stderr, `(got "invalid")`) {
		t.Fatalf("stdout=%q stderr=%q error=%v, want preflight output error", stdout, stderr, err)
	}
}

func TestAdsPlatformAPIRequestRejectsMultipartUploadBeforeAuthOrNetwork(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "AD_ACCOUNT")
	t.Setenv("ASC_ADS_ORG_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected token/network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	stdout, stderr, err := runAdsEvalCommand(t, "ads", "api", "request", "--method", "POST", "--path", "v1/assets/upload", "--file", filepath.Join(t.TempDir(), "payload.json"))
	if !errors.Is(err, flag.ErrHelp) || stdout != "" {
		t.Fatalf("stdout=%q stderr=%q error=%v, want preflight usage error", stdout, stderr, err)
	}
	for _, want := range []string{"multipart/form-data", "asc ads assets upload"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr=%q, want %q", stderr, want)
		}
	}
}

func TestAdsEndpointRejectsInvalidOutputBeforeReadingBody(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	stdout, stderr, err := runAdsEvalCommand(t, "ads", "ad-accounts", "create", "--file", filepath.Join(t.TempDir(), "does-not-exist.json"), "--output", "invalid")
	if !isUsageClassError(err) || stdout != "" || !strings.Contains(stderr, `(got "invalid")`) {
		t.Fatalf("stdout=%q stderr=%q error=%v, want output preflight before body read", stdout, stderr, err)
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

func TestAdsPlatformAPIRequestRequiresAdAccountBeforeNetwork(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	root := RootCommand("dev")
	if err := root.Parse([]string{"ads", "api", "request", "--path", "v1/campaigns/123"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	_, stderr := captureOutput(t, func() {
		runErr = root.Run(context.Background())
	})
	if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(stderr, "--ad-account is required") {
		t.Fatalf("run error = %v stderr = %q", runErr, stderr)
	}
}

func TestAdsPlatformAPIRequestRejectsAdAccountForContextFreeEndpoint(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	root := RootCommand("dev")
	if err := root.Parse([]string{"ads", "api", "request", "--path", "v1/me", "--ad-account", "AD_ACCOUNT"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	_, stderr := captureOutput(t, func() {
		runErr = root.Run(context.Background())
	})
	if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(stderr, "--ad-account is not supported") {
		t.Fatalf("run error = %v stderr = %q", runErr, stderr)
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

func TestAdsPlatformAdAccountCreateRequiresRiskConfirmationBeforeNetwork(t *testing.T) {
	payloadPath := filepath.Join(t.TempDir(), "ad-account.json")
	if err := os.WriteFile(payloadPath, []byte(`{"name":"Disposable","productFeatures":["APPSTORE_APP_MANUAL"]}`), 0o600); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected token/network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	root := RootCommand("dev")
	if err := root.Parse([]string{"ads", "ad-accounts", "create", "--file", payloadPath}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	_, stderr := captureOutput(t, func() {
		runErr = root.Run(context.Background())
	})
	if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(stderr, "--confirm is required to acknowledge potential Apple Ads spend, billing, delivery, targeting, or access impact") {
		t.Fatalf("run error = %v stderr = %q, want risk confirmation", runErr, stderr)
	}
}

func adsJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
