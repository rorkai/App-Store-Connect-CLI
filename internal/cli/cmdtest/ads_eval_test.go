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

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestAdsAgentReadOnlyEvalWorkflow(t *testing.T) {
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_ORG_ID", "987654")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	reportPayload := writeAdsEvalPayload(t, "report.json", `{
		"startTime": "2026-05-01",
		"endTime": "2026-05-31",
		"returnRowTotals": true,
		"selector": {
			"orderBy": [
				{"field": "impressions", "sortOrder": "DESCENDING"}
			],
			"pagination": {"offset": 0, "limit": 100}
		}
	}`)

	log := newRequestLog(5)
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assertAdsEvalBearer(t, req)
		log.Add(req.Method + " " + req.URL.RequestURI())

		switch req.URL.Path {
		case "/api/v5/me":
			assertAdsEvalNoOrg(t, req)
			assertAdsEvalNoBody(t, req)
			return adsJSONResponse(200, `{"data":{"id":"user-1"}}`), nil
		case "/api/v5/acls":
			assertAdsEvalNoOrg(t, req)
			assertAdsEvalNoBody(t, req)
			return adsJSONResponse(200, `{"data":[{"orgId":987654}]}`), nil
		case "/api/v5/campaigns":
			assertAdsEvalOrg(t, req)
			if req.Method != http.MethodGet {
				t.Fatalf("campaigns method = %s, want GET", req.Method)
			}
			if got := req.URL.Query().Get("limit"); got != "1" {
				t.Fatalf("campaigns limit = %q, want 1", got)
			}
			assertAdsEvalNoBody(t, req)
			return adsJSONResponse(200, `{"data":[{"id":12345}],"pagination":{"itemsPerPage":1,"startIndex":0,"totalResults":1}}`), nil
		case "/api/v5/reports/campaigns":
			assertAdsEvalOrg(t, req)
			if req.Method != http.MethodPost {
				t.Fatalf("reports method = %s, want POST", req.Method)
			}
			if got := req.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", got)
			}
			body := readAdsEvalJSONBody(t, req)
			selector, ok := body["selector"].(map[string]any)
			if !ok {
				t.Fatalf("report body selector = %#v, want object", body["selector"])
			}
			if _, ok := selector["orderBy"].([]any); !ok {
				t.Fatalf("report body selector.orderBy = %#v, want array", selector["orderBy"])
			}
			return adsJSONResponse(200, `{"data":{"reportingDataResponse":{"row":[{"metadata":{"campaignId":12345}}]}}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	for _, args := range [][]string{
		{"ads", "me", "--output", "json"},
		{"ads", "me", "view", "--output", "json"},
		{"ads", "acls", "--output", "json"},
		{"ads", "campaigns", "--limit", "1", "--output", "json"},
		{"ads", "reports", "campaigns", "--file", reportPayload, "--output", "json"},
		{"ads", "api", "request", "--method", "GET", "--path", "v5/me", "--output", "json"},
	} {
		stdout, stderr, err := runAdsEvalCommand(t, args...)
		if err != nil {
			t.Fatalf("asc %s error: %v\nstderr: %s", strings.Join(args, " "), err, stderr)
		}
		if stderr != "" {
			t.Fatalf("asc %s stderr = %q, want empty", strings.Join(args, " "), stderr)
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
			t.Fatalf("asc %s stdout is not JSON: %v\n%s", strings.Join(args, " "), err, stdout)
		}
	}

	joined := strings.Join(log.Snapshot(), "\n")
	for _, want := range []string{
		"GET /api/v5/me",
		"GET /api/v5/acls",
		"GET /api/v5/campaigns?limit=1",
		"POST /api/v5/reports/campaigns",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("requests = %q, missing %q", joined, want)
		}
	}
}

func TestAdsAuthDiscoverSummarizesMeAndAcls(t *testing.T) {
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_ORG_ID", "987654")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	log := newRequestLog(2)
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assertAdsEvalBearer(t, req)
		assertAdsEvalNoOrg(t, req)
		assertAdsEvalNoBody(t, req)
		log.Add(req.Method + " " + req.URL.RequestURI())

		switch req.URL.Path {
		case "/api/v5/me":
			return adsJSONResponse(200, `{"data":{"id":"user-1","name":"Ada Example"}}`), nil
		case "/api/v5/acls":
			return adsJSONResponse(200, `{"data":[{"orgId":987654,"orgName":"Example Org","roleNames":["Admin"]},{"orgId":123456,"name":"Other Org","roles":["ReadOnly"]}]}`), nil
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
		AuthSource  string `json:"auth_source"`
		OrgID       string `json:"org_id"`
		OrgIDSource string `json:"org_id_source"`
		Me          struct {
			ID string `json:"id"`
		} `json:"me"`
		Accounts []struct {
			OrgID  string   `json:"org_id"`
			Name   string   `json:"name"`
			Roles  []string `json:"roles"`
			Active bool     `json:"active"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("discover stdout is not JSON: %v\n%s", err, stdout)
	}
	if result.AuthSource != "ASC_ADS_ACCESS_TOKEN" || result.OrgID != "987654" || result.OrgIDSource != "ASC_ADS_ORG_ID" {
		t.Fatalf("discovery context = %+v, want env token/org", result)
	}
	if result.Me.ID != "user-1" {
		t.Fatalf("me.id = %q, want user-1", result.Me.ID)
	}
	if len(result.Accounts) != 2 || result.Accounts[0].OrgID != "987654" || result.Accounts[0].Name != "Example Org" || !result.Accounts[0].Active {
		t.Fatalf("accounts = %+v, want active Example Org first", result.Accounts)
	}
	if got := strings.Join(result.Accounts[0].Roles, ","); got != "Admin" {
		t.Fatalf("roles = %q, want Admin", got)
	}

	requests := strings.Join(log.Snapshot(), "\n")
	for _, want := range []string{"GET /api/v5/me", "GET /api/v5/acls"} {
		if !strings.Contains(requests, want) {
			t.Fatalf("requests = %q, missing %q", requests, want)
		}
	}
}

func TestAdsAuthDiscoverTableShowsUserAndAccounts(t *testing.T) {
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_ORG_ID", "987654")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	log := newRequestLog(2)
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assertAdsEvalBearer(t, req)
		assertAdsEvalNoOrg(t, req)
		assertAdsEvalNoBody(t, req)
		log.Add(req.Method + " " + req.URL.RequestURI())

		switch req.URL.Path {
		case "/api/v5/me":
			return adsJSONResponse(200, `{"data":{"id":"user-1","name":"Ada Example"}}`), nil
		case "/api/v5/acls":
			return adsJSONResponse(200, `{"data":[{"orgId":987654,"orgName":"Example Org","roleNames":["Admin"]}]}`), nil
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
		"987654 - Example Org (active)",
		"Roles: Admin",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("discover table stdout = %q, missing %q", stdout, want)
		}
	}
	if requests := strings.Join(log.Snapshot(), "\n"); !strings.Contains(requests, "GET /api/v5/me") || !strings.Contains(requests, "GET /api/v5/acls") {
		t.Fatalf("requests = %q, want me and acls lookups", requests)
	}
}

func TestAdsAuthDiscoverAcceptsRealMeFieldsAndSingletonACL(t *testing.T) {
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_ORG_ID", "987654")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assertAdsEvalBearer(t, req)
		assertAdsEvalNoOrg(t, req)
		assertAdsEvalNoBody(t, req)

		switch req.URL.Path {
		case "/api/v5/me":
			return adsJSONResponse(200, `{"data":{"userId":"user-1","parentOrgId":987654}}`), nil
		case "/api/v5/acls":
			return adsJSONResponse(200, `{"data":{"orgId":987654,"orgName":"Example Org","roleNames":["Admin"]}}`), nil
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
		"987654 - Example Org (active)",
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
	if !strings.Contains(stderr, "unsupported format: invalid") {
		t.Fatalf("stderr = %q, want invalid --output usage error", stderr)
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
			t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
			installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				assertAdsEvalBearer(t, req)
				assertAdsEvalNoOrg(t, req)
				assertAdsEvalNoBody(t, req)

				switch req.URL.Path {
				case "/api/v5/me":
					return adsJSONResponse(200, test.meBody), nil
				case "/api/v5/acls":
					return adsJSONResponse(200, test.aclBody), nil
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
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	configPath := writeAdsEvalPayload(t, "config.json", `{"ads":`)
	t.Setenv("ASC_CONFIG_PATH", configPath)

	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assertAdsEvalBearer(t, req)
		assertAdsEvalNoOrg(t, req)
		assertAdsEvalNoBody(t, req)

		switch req.URL.Path {
		case "/api/v5/me":
			return adsJSONResponse(200, `{"data":{"id":"user-1","name":"Ada Example"}}`), nil
		case "/api/v5/acls":
			return adsJSONResponse(200, `{"data":[{"orgId":987654,"orgName":"Example Org"}]}`), nil
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
			OrgID string `json:"org_id"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("discover stdout is not JSON: %v\n%s", err, stdout)
	}
	if result.OrgID != "" || result.OrgIDSource != "" {
		t.Fatalf("org context = %+v, want no selected org from invalid config", result)
	}
	if len(result.Accounts) != 1 || result.Accounts[0].OrgID != "987654" {
		t.Fatalf("accounts = %+v, want discovered account despite invalid config", result.Accounts)
	}
}

func TestAdsAgentMutationEvalWorkflow(t *testing.T) {
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_ORG_ID", "987654")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	campaignCreate := writeAdsEvalPayload(t, "campaign-create.json", `{"name":"ASC CLI Eval Campaign","status":"PAUSED"}`)
	campaignUpdate := writeAdsEvalPayload(t, "campaign-update.json", `{"campaign":{"status":"PAUSED"}}`)
	keywords := writeAdsEvalPayload(t, "keywords.json", `[{"text":"example keyword","matchType":"EXACT","status":"PAUSED"}]`)
	keywordIDs := writeAdsEvalPayload(t, "keyword-ids.json", `[111111111]`)

	log := newRequestLog(4)
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assertAdsEvalBearer(t, req)
		assertAdsEvalOrg(t, req)
		log.Add(req.Method + " " + req.URL.RequestURI())

		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/api/v5/campaigns":
			body := readAdsEvalJSONBody(t, req)
			if got := body["name"]; got != "ASC CLI Eval Campaign" {
				t.Fatalf("campaign create name = %#v, want ASC CLI Eval Campaign", got)
			}
			return adsJSONResponse(200, `{"data":{"id":1001}}`), nil
		case req.Method == http.MethodPut && req.URL.Path == "/api/v5/campaigns/1001":
			body := readAdsEvalJSONBody(t, req)
			campaign, ok := body["campaign"].(map[string]any)
			if !ok {
				t.Fatalf("campaign update body = %#v, want campaign envelope", body)
			}
			if got := campaign["status"]; got != "PAUSED" {
				t.Fatalf("campaign update status = %#v, want PAUSED", got)
			}
			return adsJSONResponse(200, `{"data":{"id":1001,"status":"PAUSED"}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/api/v5/campaigns/1001/adgroups/2002/targetingkeywords/bulk":
			items := readAdsEvalJSONArrayBody(t, req)
			if len(items) != 1 {
				t.Fatalf("keyword create body length = %d, want 1", len(items))
			}
			return adsJSONResponse(200, `{"data":[{"id":111111111}]}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/api/v5/campaigns/1001/adgroups/2002/targetingkeywords/delete/bulk":
			items := readAdsEvalJSONArrayBody(t, req)
			if len(items) != 1 || items[0] != float64(111111111) {
				t.Fatalf("keyword delete body = %#v, want [111111111]", items)
			}
			return adsJSONResponse(200, `{"data":[111111111]}`), nil
		case req.Method == http.MethodDelete && req.URL.Path == "/api/v5/campaigns/1001":
			assertAdsEvalNoBody(t, req)
			return adsJSONResponse(204, ``), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	for _, args := range [][]string{
		{"ads", "campaigns", "create", "--file", campaignCreate, "--output", "json"},
		{"ads", "campaigns", "update", "--campaign", "1001", "--file", campaignUpdate, "--output", "json"},
		{"ads", "targeting-keywords", "create-bulk", "--campaign", "1001", "--ad-group", "2002", "--file", keywords, "--output", "json"},
		{"ads", "targeting-keywords", "delete-bulk", "--campaign", "1001", "--ad-group", "2002", "--file", keywordIDs, "--confirm", "--output", "json"},
		{"ads", "campaigns", "delete", "--campaign", "1001", "--confirm", "--output", "json"},
	} {
		stdout, stderr, err := runAdsEvalCommand(t, args...)
		if err != nil {
			t.Fatalf("asc %s error: %v\nstderr: %s", strings.Join(args, " "), err, stderr)
		}
		if stderr != "" {
			t.Fatalf("asc %s stderr = %q, want empty", strings.Join(args, " "), stderr)
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
			t.Fatalf("asc %s stdout is not JSON: %v\n%s", strings.Join(args, " "), err, stdout)
		}
	}

	joined := strings.Join(log.Snapshot(), "\n")
	for _, want := range []string{
		"POST /api/v5/campaigns",
		"PUT /api/v5/campaigns/1001",
		"POST /api/v5/campaigns/1001/adgroups/2002/targetingkeywords/bulk",
		"POST /api/v5/campaigns/1001/adgroups/2002/targetingkeywords/delete/bulk",
		"DELETE /api/v5/campaigns/1001",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("requests = %q, missing %q", joined, want)
		}
	}
}

func TestAdsAgentEvalRejectsArrayPayloadMistakesBeforeNetwork(t *testing.T) {
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_ORG_ID", "987654")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	objectPayload := writeAdsEvalPayload(t, "keyword-object.json", `{"text":"not an array"}`)
	_, _, err := runAdsEvalCommand(
		t,
		"ads", "targeting-keywords", "create-bulk",
		"--campaign", "1001",
		"--ad-group", "2002",
		"--file", objectPayload,
		"--output", "json",
	)
	if err == nil || !strings.Contains(err.Error(), "payload must be a JSON array") {
		t.Fatalf("error = %v, want JSON array validation", err)
	}
}

func TestAdsAgentRawAPIEvalRequiresConfirmAndAcceptsAppleURL(t *testing.T) {
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_ORG_ID", "987654")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	log := newRequestLog(1)
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assertAdsEvalBearer(t, req)
		assertAdsEvalOrg(t, req)
		log.Add(req.Method + " " + req.URL.RequestURI())
		if req.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", req.Method)
		}
		if got := req.URL.RawQuery; got != "audit=true" {
			t.Fatalf("query = %q, want audit=true", got)
		}
		assertAdsEvalNoBody(t, req)
		return adsJSONResponse(204, ``), nil
	}))

	_, stderr, err := runAdsEvalCommand(
		t,
		"ads", "api", "request",
		"--method", "DELETE",
		"--path", "https://api.searchads.apple.com/api/v5/campaigns/1001?audit=true",
		"--output", "json",
	)
	if !errors.Is(err, flag.ErrHelp) || !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("error = %v stderr = %q, want confirm usage error", err, stderr)
	}
	if got := len(log.Snapshot()); got != 0 {
		t.Fatalf("requests before confirm = %d, want 0", got)
	}

	stdout, stderr, err := runAdsEvalCommand(
		t,
		"ads", "api", "request",
		"--method", "DELETE",
		"--path", "https://api.searchads.apple.com/api/v5/campaigns/1001?audit=true",
		"--confirm",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("confirmed raw delete error: %v\nstderr: %s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var parsed struct {
		Data any `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if parsed.Data != nil {
		t.Fatalf("data = %#v, want nil", parsed.Data)
	}
	requests := log.Snapshot()
	if len(requests) != 1 || requests[0] != "DELETE /api/v5/campaigns/1001?audit=true" {
		t.Fatalf("requests = %#v", requests)
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

func assertAdsEvalOrg(t *testing.T, req *http.Request) {
	t.Helper()

	want := "orgId=987654"
	if got := req.Header.Get("X-AP-Context"); got != want {
		t.Fatalf("X-AP-Context = %q, want %s", got, want)
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

func readAdsEvalJSONBody(t *testing.T, req *http.Request) map[string]any {
	t.Helper()

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("body is not a JSON object: %v\n%s", err, body)
	}
	return parsed
}

func readAdsEvalJSONArrayBody(t *testing.T, req *http.Request) []any {
	t.Helper()

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var parsed []any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("body is not a JSON array: %v\n%s", err, body)
	}
	return parsed
}
