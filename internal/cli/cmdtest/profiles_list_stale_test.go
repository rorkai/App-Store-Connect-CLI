package cmdtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func startProfilesListStaleStub(t *testing.T) *atomic.Int32 {
	t.Helper()
	setupAuth(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests.Add(1)
		if req.Method != http.MethodGet || req.URL.Path != "/v1/profiles" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected", http.StatusInternalServerError)
			return
		}
		if req.URL.Query().Get("cursor") == "2" {
			writeSigningFetchOutputJSON(t, w, http.StatusOK, `{"data":[`+
				staleProfileJSON("invalid-2", "INVALID", "2100-01-01T00:00:00Z")+`],"links":{"self":"https://api.appstoreconnect.apple.com/v1/profiles?cursor=2"},"meta":{"paging":{"total":3,"limit":200}}}`)
			return
		}
		writeSigningFetchOutputJSON(t, w, http.StatusOK, fmt.Sprintf(
			`{"data":[%s,%s],"links":{"self":"https://api.appstoreconnect.apple.com/v1/profiles","next":"https://api.appstoreconnect.apple.com/v1/profiles?cursor=2"},"meta":{"paging":{"total":3,"limit":200}}}`,
			staleProfileJSON("expired-1", "ACTIVE", "2000-01-01T00:00:00Z"),
			staleProfileJSON("fresh-1", "ACTIVE", "2100-01-01T00:00:00Z"),
		))
	}))
	t.Cleanup(server.Close)
	installStaleProfilesClient(t, server)
	return &requests
}

func runProfilesList(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run(append([]string{"profiles", "list"}, args...), "test")
	})
	return code, stdout, stderr
}

func TestProfilesListStaleOnlyIsAComputedViewOverEveryPage(t *testing.T) {
	requests := startProfilesListStaleStub(t)

	code, stdout, stderr := runProfilesList(t, "--stale-only", "--output", "json")
	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d (stderr %q)", code, stderr)
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Links map[string]string `json:"links"`
		Meta  json.RawMessage   `json:"meta"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	ids := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		ids = append(ids, item.ID)
	}
	if got := strings.Join(ids, ","); got != "expired-1,invalid-2" {
		t.Fatalf("stale ids = %q, want expired-1,invalid-2 from both pages", got)
	}
	if payload.Meta != nil {
		t.Fatalf("meta = %s, want dropped for the filtered view", payload.Meta)
	}
	if payload.Links["next"] != "" {
		t.Fatalf("links.next = %q, want empty", payload.Links["next"])
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want 2 pages", got)
	}
}

func TestProfilesListIncludeStaleRejectsJSON(t *testing.T) {
	requests := startProfilesListStaleStub(t)

	code, stdout, stderr := runProfilesList(t, "--include-stale", "--output", "json")
	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
	}
	if stdout != "" || !strings.Contains(stderr, "--include-stale requires --output table or markdown") {
		t.Fatalf("stdout = %q, stderr = %q", stdout, stderr)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("requests = %d, want 0", got)
	}
}

func TestProfilesListIncludeStaleAddsColumnToTable(t *testing.T) {
	startProfilesListStaleStub(t)

	code, stdout, stderr := runProfilesList(t, "--include-stale", "--output", "table", "--paginate")
	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d (stderr %q)", code, stderr)
	}
	if !strings.Contains(stdout, "Stale") || !strings.Contains(stdout, "fresh-1") {
		t.Fatalf("table = %q, want a Stale column with every profile", stdout)
	}
}

func TestProfilesListStaleOnlyRejectsNext(t *testing.T) {
	requests := startProfilesListStaleStub(t)

	code, _, stderr := runProfilesList(t, "--stale-only", "--next", "https://api.appstoreconnect.apple.com/v1/profiles?cursor=2")
	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (stderr %q)", code, rootcmd.ExitUsage, stderr)
	}
	if !strings.Contains(stderr, "--next cannot be combined with --stale-only") {
		t.Fatalf("stderr = %q", stderr)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("requests = %d, want 0", got)
	}
}

func installStaleProfilesClient(t *testing.T, server *httptest.Server) {
	t.Helper()
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	serverTransport := server.Client().Transport
	client, err := asc.NewClientWithHTTPClient(
		os.Getenv("ASC_KEY_ID"),
		os.Getenv("ASC_ISSUER_ID"),
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			cloned := req.Clone(req.Context())
			cloned.URL.Scheme = serverURL.Scheme
			cloned.URL.Host = serverURL.Host
			return serverTransport.RoundTrip(cloned)
		})},
	)
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))
}

func staleProfileJSON(id, state, expiration string) string {
	return fmt.Sprintf(`{"type":"profiles","id":%q,"attributes":{"name":%q,"profileType":"IOS_APP_STORE","profileState":%q,"expirationDate":%q}}`, id, "Profile "+id, state, expiration)
}

func TestProfilesListStaleFlagsRequireStalenessFields(t *testing.T) {
	for _, flagName := range []string{"--stale-only", "--include-stale"} {
		t.Run(flagName, func(t *testing.T) {
			requests := startProfilesListStaleStub(t)
			code, stdout, stderr := runProfilesList(t, flagName, "--fields", "name,profileState", "--output", "table")
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, rootcmd.ExitUsage, stderr)
			}
			if stdout != "" || !strings.Contains(stderr, "require --fields to include profileState and expirationDate") {
				t.Fatalf("stdout = %q, stderr = %q", stdout, stderr)
			}
			if got := requests.Load(); got != 0 {
				t.Fatalf("requests = %d, want 0", got)
			}
		})
	}
}

func TestProfilesListStaleOnlyKeepsOnlyReferencedIncludedResources(t *testing.T) {
	setupAuth(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		writeSigningFetchOutputJSON(t, w, http.StatusOK, `{"data":[
			{"type":"profiles","id":"expired-1","attributes":{"name":"Old","profileType":"IOS_APP_STORE","profileState":"ACTIVE","expirationDate":"2000-01-01T00:00:00Z"},
			 "relationships":{"bundleId":{"data":{"type":"bundleIds","id":"bundle-old"}},"certificates":{"data":[{"type":"certificates","id":"cert-shared"}]}}},
			{"type":"profiles","id":"fresh-1","attributes":{"name":"New","profileType":"IOS_APP_STORE","profileState":"ACTIVE","expirationDate":"2100-01-01T00:00:00Z"},
			 "relationships":{"bundleId":{"data":{"type":"bundleIds","id":"bundle-new"}},"certificates":{"data":[{"type":"certificates","id":"cert-shared"},{"type":"certificates","id":"cert-new"}]}}}
		],"included":[
			{"type":"bundleIds","id":"bundle-old","attributes":{"identifier":"com.old"}},
			{"type":"bundleIds","id":"bundle-new","attributes":{"identifier":"com.new"}},
			{"type":"certificates","id":"cert-shared","attributes":{"name":"Shared"}},
			{"type":"certificates","id":"cert-new","attributes":{"name":"New"}}
		],"links":{"self":"https://api.appstoreconnect.apple.com/v1/profiles"}}`)
	}))
	t.Cleanup(server.Close)
	installStaleProfilesClient(t, server)

	code, stdout, stderr := runProfilesList(t, "--stale-only", "--include", "bundleId,certificates", "--output", "json")
	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d (stderr %q)", code, stderr)
	}
	var payload struct {
		Included []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"included"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	ids := make([]string, 0, len(payload.Included))
	for _, item := range payload.Included {
		ids = append(ids, item.Type+"/"+item.ID)
	}
	if got := strings.Join(ids, ","); got != "bundleIds/bundle-old,certificates/cert-shared" {
		t.Fatalf("included = %q, want only resources the stale profile references", got)
	}
}
