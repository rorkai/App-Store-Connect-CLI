package cmdtest

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestUsersListQuerySurfaceEmitsSupportedParameters(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/users" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		gotQuery = req.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"users","id":"user-query"}],"links":{"next":""}}`)
	}))
	t.Cleanup(server.Close)

	stdout, stderr, runErr := runRootWithDeps(t, usersListTestClient(t, server),
		"users", "list",
		"--email", "user@example.com",
		"--role", "developer, app_manager",
		"--visible-app", "app-1,app-2",
		"--sort", "-lastName",
		"--fields", "username,lastName,visibleApps",
		"--app-fields", "name,bundleId",
		"--include", "visibleApps",
		"--visible-apps-limit", "25",
		"--limit", "10",
		"--output", "json",
	)

	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, `"id":"user-query"`) {
		t.Fatalf("stdout = %q, want user response", stdout)
	}

	want := url.Values{
		"filter[username]":    {"user@example.com"},
		"filter[roles]":       {"DEVELOPER,APP_MANAGER"},
		"filter[visibleApps]": {"app-1,app-2"},
		"sort":                {"-lastName"},
		"fields[users]":       {"username,lastName,visibleApps"},
		"fields[apps]":        {"name,bundleId"},
		"include":             {"visibleApps"},
		"limit[visibleApps]":  {"25"},
		"limit":               {"10"},
	}
	if gotQuery.Encode() != want.Encode() {
		t.Fatalf("query = %q, want %q", gotQuery.Encode(), want.Encode())
	}
}

func TestUsersListQuerySurfaceAcceptsCommaSeparatedSort(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	var gotQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/users" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		gotQuery = req.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"users","id":"user-sort"}],"links":{"next":""}}`)
	}))
	t.Cleanup(server.Close)

	stdout, stderr, runErr := runRootWithDeps(t, usersListTestClient(t, server), "users", "list", "--sort", " username, -lastName ", "--output", "json")

	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q", runErr, stderr)
	}
	if !strings.Contains(stdout, `"id":"user-sort"`) {
		t.Fatalf("stdout = %q, want user response", stdout)
	}

	want := url.Values{"sort": {"username,-lastName"}}
	if gotQuery.Encode() != want.Encode() {
		t.Fatalf("query = %q, want %q", gotQuery.Encode(), want.Encode())
	}
}

func TestUsersListRejectsNextQueryFlagConflictsBeforeClient(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/users?cursor=next"
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "email", args: []string{"--email", "user@example.com"}, want: "--next cannot be combined with --email"},
		{name: "role", args: []string{"--role", "ADMIN"}, want: "--next cannot be combined with --role"},
		{name: "visible app", args: []string{"--visible-app", "app-1"}, want: "--next cannot be combined with --visible-app"},
		{name: "sort", args: []string{"--sort", "username"}, want: "--next cannot be combined with --sort"},
		{name: "fields", args: []string{"--fields", "username"}, want: "--next cannot be combined with --fields"},
		{name: "app fields", args: []string{"--app-fields", "name"}, want: "--next cannot be combined with --app-fields"},
		{name: "include", args: []string{"--include", "visibleApps"}, want: "--next cannot be combined with --include"},
		{name: "visible apps limit", args: []string{"--visible-apps-limit", "25"}, want: "--next cannot be combined with --visible-apps-limit"},
		{name: "limit", args: []string{"--limit", "10"}, want: "--next cannot be combined with --limit"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			client := func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			}

			args := append([]string{"users", "list", "--next", nextURL}, test.args...)
			assertUsageExitWithDeps(t, client, args, "users list: "+test.want)
			if clientFactoryCalled {
				t.Fatal("client factory ran before --next conflict validation")
			}
		})
	}
}

func TestUsersListRejectsVisibleAppsLimitWithoutInclude(t *testing.T) {
	clientFactoryCalled := false
	client := func() (*asc.Client, error) {
		clientFactoryCalled = true
		return nil, errors.New("client factory must not run during validation")
	}

	assertUsageExitWithDeps(t, client, []string{"users", "list", "--visible-apps-limit", "25"}, "users list: --visible-apps-limit requires --include visibleApps")
	if clientFactoryCalled {
		t.Fatal("client factory ran before relationship limit validation")
	}
}

func TestUsersListRejectsInvalidQueryValuesBeforeClient(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "sort", args: []string{"--sort", "createdDate"}, want: "users list: --sort must be one of"},
		{name: "sort mixed", args: []string{"--sort", "username,createdDate"}, want: "users list: --sort must be one of"},
		{name: "sort empty", args: []string{"--sort", ""}, want: "users list: --sort must not be empty"},
		{name: "user fields", args: []string{"--fields", "createdDate"}, want: "users list: --fields must be one of"},
		{name: "user fields separators only", args: []string{"--fields", ","}, want: "users list: --fields must not be empty"},
		{name: "app fields", args: []string{"--app-fields", "createdDate"}, want: "users list: --app-fields must be one of"},
		{name: "app fields whitespace", args: []string{"--app-fields", " \t"}, want: "users list: --app-fields must not be empty"},
		{name: "include", args: []string{"--include", "apps"}, want: "users list: --include must be one of"},
		{name: "include separators only", args: []string{"--include", ","}, want: "users list: --include must not be empty"},
		{name: "visible app empty", args: []string{"--visible-app", ""}, want: "users list: --visible-app must not be empty"},
		{name: "visible app separators only", args: []string{"--visible-app", ","}, want: "users list: --visible-app must not be empty"},
		{name: "visible apps limit zero", args: []string{"--visible-apps-limit", "0"}, want: "users list: --visible-apps-limit must be between 1 and 50"},
		{name: "visible apps limit", args: []string{"--visible-apps-limit", "51"}, want: "users list: --visible-apps-limit must be between 1 and 50"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			client := func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			}

			assertUsageExitWithDeps(t, client, append([]string{"users", "list"}, test.args...), test.want)
			if clientFactoryCalled {
				t.Fatal("client factory ran before query validation")
			}
		})
	}
}

func usersListTestClient(t *testing.T, server *httptest.Server) func() (*asc.Client, error) {
	t.Helper()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme+"://"+req.URL.Host != asc.BaseURL {
			t.Fatalf("request origin = %s://%s, want %s", req.URL.Scheme, req.URL.Host, asc.BaseURL)
		}
		if authorization := req.Header.Get("Authorization"); !strings.HasPrefix(authorization, "Bearer ") {
			t.Fatalf("Authorization = %q, want Bearer token", authorization)
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
	return func() (*asc.Client, error) {
		return client, nil
	}
}
