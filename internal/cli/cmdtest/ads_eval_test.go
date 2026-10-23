package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestAdsAuthDiscoverSummarizesMeAndAcls(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_ORG_ID", "987654")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "111")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	log := newRequestLog(2)
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assertAdsEvalBearer(t, req)
		if req.URL.Host != "api.ads.apple.com" {
			t.Fatalf("unexpected auth discovery host %q", req.URL.Host)
		}
		assertAdsEvalNoOrg(t, req)
		assertAdsEvalNoBody(t, req)
		log.Add(req.Method + " " + req.URL.RequestURI())

		switch req.URL.Path {
		case "/v1/me":
			return adsJSONResponse(`{"result":{"userId":"user-1","name":"Ada Example"}}`), nil
		case "/v1/acls":
			return adsJSONResponse(`{"result":{"acls":[{"adAccount":{"id":111,"orgId":987654,"name":"Example Org"},"roles":["Admin"]},{"adAccount":{"id":222,"orgId":123456,"name":"Other Org"},"roles":["ReadOnly"]}]}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	stdout, stderr, err := runAdsEvalCommand(t, "ads", "auth", "discover", "--output", "json")
	if err != nil {
		t.Fatalf("discover error: %v\nstderr: %s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("discover stderr = %q, want empty", stderr)
	}
	if strings.Contains(stdout, `"ACCESS"`) {
		t.Fatalf("discover leaked access token: %s", stdout)
	}

	var result struct {
		AuthSource        string `json:"auth_source"`
		OrgID             string `json:"org_id"`
		OrgIDSource       string `json:"org_id_source"`
		AdAccountID       string `json:"ad_account_id"`
		AdAccountIDSource string `json:"ad_account_id_source"`
		Me                struct {
			ID string `json:"id"`
		} `json:"me"`
		Accounts []struct {
			AdAccountID string   `json:"ad_account_id"`
			OrgID       string   `json:"org_id"`
			Name        string   `json:"name"`
			Roles       []string `json:"roles"`
			Active      bool     `json:"active"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("discover stdout is not JSON: %v\n%s", err, stdout)
	}
	if result.AuthSource != "ASC_ADS_ACCESS_TOKEN" || result.OrgID != "987654" || result.OrgIDSource != "ASC_ADS_ORG_ID" {
		t.Fatalf("discovery context = %+v, want env token/org", result)
	}
	if result.AdAccountID != "111" || result.AdAccountIDSource != "ASC_ADS_AD_ACCOUNT_ID" || result.Me.ID != "user-1" {
		t.Fatalf("ad account/me = %+v, want selected account and stable v1 me", result)
	}
	if len(result.Accounts) != 2 || result.Accounts[0].AdAccountID != "111" || result.Accounts[0].OrgID != "987654" || result.Accounts[0].Name != "Example Org" || !result.Accounts[0].Active {
		t.Fatalf("accounts = %+v, want active ad account 111 first", result.Accounts)
	}
	if got := strings.Join(result.Accounts[0].Roles, ","); got != "Admin" {
		t.Fatalf("roles = %q, want Admin", got)
	}

	requests := strings.Join(log.Snapshot(), "\n")
	for _, want := range []string{"GET /v1/me", "GET /v1/acls"} {
		if !strings.Contains(requests, want) {
			t.Fatalf("requests = %q, missing %q", requests, want)
		}
	}
}

func TestAdsPlatformAdAccountViewUsesOneValueForPathAndContext(t *testing.T) {
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "123")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	seen := []string{}
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		seen = append(seen, req.URL.Path+" "+req.Header.Get("X-AP-Context"))
		return adsJSONResponse(`{"result":{"id":123}}`), nil
	}))

	for _, args := range [][]string{
		{"ads", "ad-accounts", "view", "--output", "json"},
		{"ads", "ad-accounts", "view", "--ad-account", "456", "--output", "json"},
	} {
		stdout, stderr, err := runAdsEvalCommand(t, args...)
		if err != nil || stderr != "" || !json.Valid([]byte(stdout)) {
			t.Fatalf("asc %s stdout=%q stderr=%q error=%v", strings.Join(args, " "), stdout, stderr, err)
		}
	}
	if got, want := strings.Join(seen, "\n"), "/v1/ad-accounts/123 adAccountId=123;\n/v1/ad-accounts/456 adAccountId=456;"; got != want {
		t.Fatalf("requests = %q, want %q", got, want)
	}
}

func TestAdsAuthDiscoverTableShowsUserAndAccounts(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_ORG_ID", "987654")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "111")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	log := newRequestLog(2)
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assertAdsEvalBearer(t, req)
		if req.URL.Host != "api.ads.apple.com" {
			t.Fatalf("unexpected auth discovery host %q", req.URL.Host)
		}
		assertAdsEvalNoOrg(t, req)
		assertAdsEvalNoBody(t, req)
		log.Add(req.Method + " " + req.URL.RequestURI())

		switch req.URL.Path {
		case "/v1/me":
			return adsJSONResponse(`{"result":{"userId":"user-1","name":"Ada Example"}}`), nil
		case "/v1/acls":
			return adsJSONResponse(`{"result":{"acls":[{"adAccount":{"id":111,"orgId":987654,"name":"Example Org"},"roles":["Admin"]}]}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	stdout, stderr, err := runAdsEvalCommand(t, "ads", "auth", "discover")
	if err != nil {
		t.Fatalf("discover table error: %v\nstderr: %s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("discover table stderr = %q, want empty", stderr)
	}
	for _, want := range []string{
		"Auth source: ASC_ADS_ACCESS_TOKEN",
		"User: Ada Example (user-1)",
		"Selected org: 987654 (ASC_ADS_ORG_ID)",
		"111 - Example Org (active)",
		"Roles: Admin",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("discover table stdout = %q, missing %q", stdout, want)
		}
	}
	if requests := strings.Join(log.Snapshot(), "\n"); !strings.Contains(requests, "GET /v1/me") || !strings.Contains(requests, "GET /v1/acls") {
		t.Fatalf("requests = %q, want me and acls lookups", requests)
	}
}

func TestAdsAuthDiscoverAcceptsRealMeFieldsAndSingletonACL(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_ORG_ID", "987654")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "111")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assertAdsEvalBearer(t, req)
		if req.URL.Host != "api.ads.apple.com" {
			t.Fatalf("unexpected auth discovery host %q", req.URL.Host)
		}
		assertAdsEvalNoOrg(t, req)
		assertAdsEvalNoBody(t, req)

		switch req.URL.Path {
		case "/v1/me":
			return adsJSONResponse(`{"result":{"userId":"user-1","orgId":987654}}`), nil
		case "/v1/acls":
			return adsJSONResponse(`{"result":{"acls":[{"adAccount":{"id":111,"orgId":987654,"name":"Example Org"},"roles":["Admin"]}]}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	stdout, stderr, err := runAdsEvalCommand(t, "ads", "auth", "discover")
	if err != nil {
		t.Fatalf("discover table error: %v\nstderr: %s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("discover table stderr = %q, want empty", stderr)
	}
	for _, want := range []string{
		"User: user-1",
		"111 - Example Org (active)",
		"Roles: Admin",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("discover table stdout = %q, missing %q", stdout, want)
		}
	}
}

func TestAdsAuthDiscoverRejectsInvalidOutput(t *testing.T) {
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	stdout, stderr, err := runAdsEvalCommand(t, "ads", "auth", "discover", "--output", "invalid")
	if rootcmd.ExitCodeFromError(err) != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (err=%v)", rootcmd.ExitCodeFromError(err), rootcmd.ExitUsage, err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, `(got "invalid")`) {
		t.Fatalf("stderr = %q, want invalid --output usage error", stderr)
	}
}

func TestAdsAuthDiscoverRejectsInvalidExplicitAdAccountBeforeNetwork(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	stdout, stderr, err := runAdsEvalCommand(t, "ads", "auth", "discover", "--ad-account", "123;orgId=456", "--output", "json")
	if rootcmd.ExitCodeFromError(err) != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (err=%v)", rootcmd.ExitCodeFromError(err), rootcmd.ExitUsage, err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "--ad-account") || !strings.Contains(stderr, "semicolon") {
		t.Fatalf("stderr = %q, want invalid --ad-account usage error", stderr)
	}
}

func TestAdsAuthDiscoverRejectsControlOnlyExplicitAdAccountBeforeNetwork(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	stdout, stderr, err := runAdsEvalCommand(t, "ads", "auth", "discover", "--ad-account", "\n", "--output", "json")
	if rootcmd.ExitCodeFromError(err) != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (err=%v)", rootcmd.ExitCodeFromError(err), rootcmd.ExitUsage, err)
	}
	if stdout != "" || !strings.Contains(stderr, "control characters") {
		t.Fatalf("stdout = %q stderr = %q, want control-character usage error", stdout, stderr)
	}
}

func TestAdsAuthDiscoverRejectsMalformedDiscoveryResponses(t *testing.T) {
	tests := []struct {
		name    string
		meBody  string
		aclBody string
		wantErr string
	}{
		{
			name:    "me",
			meBody:  `{"data":`,
			aclBody: `{"data":[]}`,
			wantErr: "me response parse failed",
		},
		{
			name:    "acls",
			meBody:  `{"data":{"id":"user-1"}}`,
			aclBody: `{"data":`,
			wantErr: "acl response parse failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateAdsGuideEnv(t)
			t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
			installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				assertAdsEvalBearer(t, req)
				if req.URL.Host != "api.ads.apple.com" {
					t.Fatalf("unexpected auth discovery host %q", req.URL.Host)
				}
				assertAdsEvalNoOrg(t, req)
				assertAdsEvalNoBody(t, req)

				switch req.URL.Path {
				case "/v1/me":
					if test.meBody == `{"data":` {
						return adsJSONResponse(`{"result":`), nil
					}
					if test.meBody == `{"data":{"id":"user-1"}}` {
						return adsJSONResponse(`{"result":{"userId":"user-1"}}`), nil
					}
					return adsJSONResponse(strings.Replace(test.meBody, `{"data":`, `{"result":`, 1)), nil
				case "/v1/acls":
					if test.aclBody == `{"data":` {
						return adsJSONResponse(`{"result":`), nil
					}
					if test.aclBody == `{"data":[]}` {
						return adsJSONResponse(`{"result":{"acls":[]}}`), nil
					}
					return adsJSONResponse(strings.Replace(test.aclBody, `{"data":`, `{"result":{"acls":`, 1)), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}))

			stdout, stderr, err := runAdsEvalCommand(t, "ads", "auth", "discover", "--output", "json")
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
			if stdout != "" || stderr != "" {
				t.Fatalf("stdout = %q stderr = %q, want empty output on parse failure", stdout, stderr)
			}
		})
	}
}

func TestAdsAuthDiscoverContinuesWhenOptionalOrgConfigIsInvalid(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "111")
	configPath := writeAdsEvalPayload(t, "config.json", `{"ads":`)
	t.Setenv("ASC_CONFIG_PATH", configPath)

	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assertAdsEvalBearer(t, req)
		if req.URL.Host != "api.ads.apple.com" {
			t.Fatalf("unexpected auth discovery host %q", req.URL.Host)
		}
		assertAdsEvalNoOrg(t, req)
		assertAdsEvalNoBody(t, req)

		switch req.URL.Path {
		case "/v1/me":
			return adsJSONResponse(`{"result":{"userId":"user-1","name":"Ada Example"}}`), nil
		case "/v1/acls":
			return adsJSONResponse(`{"result":{"acls":[{"adAccount":{"id":111,"orgId":987654,"name":"Example Org"}}]}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	stdout, stderr, err := runAdsEvalCommand(t, "ads", "auth", "discover", "--output", "json")
	if err != nil {
		t.Fatalf("discover error: %v\nstderr: %s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("discover stderr = %q, want empty", stderr)
	}
	var result struct {
		OrgID       string `json:"org_id"`
		OrgIDSource string `json:"org_id_source"`
		Accounts    []struct {
			AdAccountID string `json:"ad_account_id"`
			OrgID       string `json:"org_id"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("discover stdout is not JSON: %v\n%s", err, stdout)
	}
	if result.OrgID != "" || result.OrgIDSource != "" {
		t.Fatalf("org context = %+v, want no selected org from invalid config", result)
	}
	if len(result.Accounts) != 1 || result.Accounts[0].AdAccountID != "111" || result.Accounts[0].OrgID != "987654" {
		t.Fatalf("accounts = %+v, want discovered account despite invalid config", result.Accounts)
	}
}

func runAdsEvalCommand(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		root := RootCommand("dev")
		if err := root.Parse(args); err != nil {
			runErr = err
			return
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func writeAdsEvalPayload(t *testing.T, name string, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	return path
}

func assertAdsEvalBearer(t *testing.T, req *http.Request) {
	t.Helper()

	if got := req.Header.Get("Authorization"); got != "Bearer ACCESS" {
		t.Fatalf("Authorization = %q, want Bearer ACCESS", got)
	}
}

func assertAdsEvalNoOrg(t *testing.T, req *http.Request) {
	t.Helper()

	if got := req.Header.Get("X-AP-Context"); got != "" {
		t.Fatalf("X-AP-Context = %q, want empty", got)
	}
}

func assertAdsEvalNoBody(t *testing.T, req *http.Request) {
	t.Helper()

	if req.Body == nil {
		return
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if strings.TrimSpace(string(body)) != "" {
		t.Fatalf("body = %q, want empty", string(body))
	}
	if got := req.Header.Get("Content-Type"); got != "" {
		t.Fatalf("Content-Type = %q, want empty", got)
	}
}
