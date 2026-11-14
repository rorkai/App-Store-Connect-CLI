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

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestAccessibilityListRejectsNextQueryFlagsBeforeAuth(t *testing.T) {
	t.Cleanup(func() { shared.SetSelectedProfile("") })
	t.Setenv("ASC_APP_ID", "")

	const nextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/accessibilityDeclarations?cursor=next"
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "app before next",
			args:    []string{"accessibility", "list", "--app", "app-2", "--next", nextURL},
			wantErr: "accessibility list: --next cannot be combined with --app",
		},
		{
			name:    "explicit empty app after next",
			args:    []string{"accessibility", "list", "--next", nextURL, "--app", ""},
			wantErr: "accessibility list: --next cannot be combined with --app",
		},
		{
			name:    "device family after next",
			args:    []string{"accessibility", "list", "--next", nextURL, "--device-family", "IPHONE"},
			wantErr: "accessibility list: --next cannot be combined with --device-family",
		},
		{
			name:    "state before next",
			args:    []string{"accessibility", "list", "--state", "PUBLISHED", "--next", nextURL},
			wantErr: "accessibility list: --next cannot be combined with --state",
		},
		{
			name:    "explicit empty fields after next",
			args:    []string{"accessibility", "list", "--next", nextURL, "--fields", ""},
			wantErr: "accessibility list: --next cannot be combined with --fields",
		},
		{
			name:    "explicit zero limit before next",
			args:    []string{"accessibility", "list", "--limit", "0", "--next", nextURL},
			wantErr: "accessibility list: --next cannot be combined with --limit",
		},
		{
			name:    "out of range limit after next",
			args:    []string{"accessibility", "list", "--next", nextURL, "--limit", "201"},
			wantErr: "accessibility list: --next cannot be combined with --limit",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			client := func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			}

			assertUsageExitWithDeps(t, client, test.args, test.wantErr)
			if clientFactoryCalled {
				t.Fatal("client factory ran before --next conflict validation")
			}
		})
	}
}

func TestAccessibilityListInvalidNextPrecedesLimitConflict(t *testing.T) {
	stdout, stderr, code := runWithDeps(t, nil,
		"accessibility", "list",
		"--next", "http://api.appstoreconnect.apple.com/v1/apps/app-1/accessibilityDeclarations?cursor=next",
		"--limit", "201",
	)
	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	// The usage page ffcli renders after a usage error lists every flag, so the
	// precedence assertion is scoped to the diagnostic line itself.
	diagnostic, _, _ := strings.Cut(stderr, "\n")
	if diagnostic != "Error: accessibility list: --next must be an App Store Connect URL" {
		t.Fatalf("stderr diagnostic = %q, want invalid --next error", diagnostic)
	}
	if strings.Contains(diagnostic, "--limit") {
		t.Fatalf("stderr diagnostic = %q, want --next validation to take precedence", diagnostic)
	}
}

func TestAccessibilityListNextOnlyUsesCursorURL(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "env-app")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const nextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/accessibilityDeclarations?cursor=page-2&filter%5Bstate%5D=PUBLISHED&limit=5"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/accessibilityDeclarations" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		query := req.URL.Query()
		if got := query.Get("cursor"); got != "page-2" {
			t.Errorf("cursor = %q, want page-2", got)
		}
		if got := query.Get("filter[state]"); got != "PUBLISHED" {
			t.Errorf("filter[state] = %q, want PUBLISHED", got)
		}
		if got := query.Get("limit"); got != "5" {
			t.Errorf("limit = %q, want 5", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"accessibilityDeclarations","id":"decl-next"}],"links":{"next":""}}`)
	}))
	t.Cleanup(server.Close)

	stdout, stderr, err := runRootWithDeps(t, accessibilityServerClient(t, server), "accessibility", "list", "--next", nextURL, "--output", "json")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, `"id":"decl-next"`) {
		t.Fatalf("stdout = %q, want cursor response", stdout)
	}
}

func TestAccessibilityListFilterOnlyBuildsDocumentedQuery(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/accessibilityDeclarations" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		want := url.Values{
			"fields[accessibilityDeclarations]": {"deviceFamily,state"},
			"filter[deviceFamily]":              {"IPHONE,IPAD"},
			"filter[state]":                     {"DRAFT"},
			"limit":                             {"25"},
		}
		if got := req.URL.Query(); got.Encode() != want.Encode() {
			t.Errorf("query = %q, want %q", got.Encode(), want.Encode())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"accessibilityDeclarations","id":"decl-filtered"}],"links":{"next":""}}`)
	}))
	t.Cleanup(server.Close)

	stdout, stderr, err := runRootWithDeps(t, accessibilityServerClient(t, server),
		"accessibility", "list",
		"--app", "app-1",
		"--device-family", "iphone,ipad",
		"--state", "draft",
		"--fields", "deviceFamily,state",
		"--limit", "25",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, `"id":"decl-filtered"`) {
		t.Fatalf("stdout = %q, want filtered response", stdout)
	}
}

// accessibilityServerClient returns a client factory whose clients send every
// request to server.
func accessibilityServerClient(t *testing.T, server *httptest.Server) func() (*asc.Client, error) {
	t.Helper()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	return stubClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	}))
}
