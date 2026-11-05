package cmdtest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/handlertest"
)

func webAppsCreateAccessArgs(extra ...string) []string {
	args := []string{
		"web", "apps", "create",
		"--name", "My App",
		"--bundle-id", "com.example.app",
		"--sku", "SKU123",
		"--output", "json",
	}
	return append(args, extra...)
}

func TestWebAppsCreateAccessValidationFailsBeforeHTTP(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		http.Error(w, "no HTTP expected", http.StatusInternalServerError)
	}))
	defer server.Close()
	setWebAppsCreateASCClient(t, server)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "limited without user", args: webAppsCreateAccessArgs("--access", "limited"), wantErr: "--access limited requires at least one --user"},
		{name: "invalid access", args: webAppsCreateAccessArgs("--access", "team"), wantErr: "--access must be full or limited"},
		{name: "user without access", args: webAppsCreateAccessArgs("--user", "user-1"), wantErr: "--user requires --access limited"},
		{name: "user with full access", args: webAppsCreateAccessArgs("--access", "full", "--user", "user-1"), wantErr: "--user requires --access limited"},
		{name: "access without name", args: []string{"web", "apps", "create", "--access", "full", "--output", "json"}, wantErr: "missing required flags"},
		{name: "blank user", args: webAppsCreateAccessArgs("--access", "limited", "--user", "user-1", "--user", ""), wantErr: `value cannot be empty`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertUsageExit(t, tt.args, tt.wantErr)
		})
	}
	if requests != 0 {
		t.Fatalf("expected no HTTP, got %d requests", requests)
	}
}

func TestWebAppsCreateUnknownUserFailsBeforeCreate(t *testing.T) {
	fixture := handlertest.New(t)
	var (
		mu       sync.Mutex
		requests []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mu.Lock()
		requests = append(requests, req.Method+" "+req.URL.Path)
		mu.Unlock()
		if req.Method == http.MethodGet && req.URL.Path == "/v1/users/missing-user" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist"}]}`))
			return
		}
		fixture.Respond(w, "unexpected request: %s %s", req.Method, req.URL.Path)
	}))
	defer server.Close()
	setWebAppsCreateASCClient(t, server)

	assertUsageExit(t, webAppsCreateAccessArgs(
		"--access", "limited",
		"--user", "missing-user",
	), `unknown user ID "missing-user"`)

	mu.Lock()
	got := append([]string(nil), requests...)
	mu.Unlock()
	if len(got) != 1 || got[0] != "GET /v1/users/missing-user" {
		t.Fatalf("requests = %v, want only GET /v1/users/missing-user", got)
	}
}

func TestWebAppsCreateAllAppsVisibleUserFailsBeforeCreate(t *testing.T) {
	fixture := handlertest.New(t)
	var (
		mu       sync.Mutex
		requests []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mu.Lock()
		requests = append(requests, req.Method+" "+req.URL.Path)
		mu.Unlock()
		if req.Method == http.MethodGet && req.URL.Path == "/v1/users/full-user" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"type":"users","id":"full-user","attributes":{"username":"full@example.com","allAppsVisible":true}}}`))
			return
		}
		fixture.Respond(w, "unexpected request: %s %s", req.Method, req.URL.Path)
	}))
	defer server.Close()
	setWebAppsCreateASCClient(t, server)

	assertUsageExit(t, webAppsCreateAccessArgs(
		"--access", "limited",
		"--user", "full-user",
	), `user ID "full-user" has access to all apps`)

	mu.Lock()
	got := append([]string(nil), requests...)
	mu.Unlock()
	if len(got) != 1 || got[0] != "GET /v1/users/full-user" {
		t.Fatalf("requests = %v, want only GET /v1/users/full-user", got)
	}
}

func setWebAppsCreateASCClient(t *testing.T, server *httptest.Server) {
	t.Helper()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "file")
	t.Setenv("ASC_WEB_SESSION_CACHE_DIR", t.TempDir())

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme+"://"+req.URL.Host != asc.BaseURL {
			t.Fatalf("request origin = %s://%s, want %s", req.URL.Scheme, req.URL.Host, asc.BaseURL)
		}
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	})
	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath)
	client, err := asc.NewClientWithHTTPClient("TEST_KEY", "TEST_ISSUER", keyPath, &http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))
}
