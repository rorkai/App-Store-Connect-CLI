package cmdtest

import (
	"errors"
	"flag"
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

func customPagesQueryTestClient(t *testing.T, handler http.HandlerFunc) func() (*asc.Client, error) {
	t.Helper()

	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return stubServerClient(t, server)
}

func TestCustomPagesListEmitsQuerySurface(t *testing.T) {
	client := customPagesQueryTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/appCustomProductPages" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		want := url.Values{
			"filter[visible]":                      {"false,true"},
			"fields[appCustomProductPages]":        {"name,visible,app,appCustomProductPageVersions"},
			"fields[apps]":                         {"name,bundleId"},
			"fields[appCustomProductPageVersions]": {"version,state"},
			"include":                              {"app,appCustomProductPageVersions"},
			"limit[appCustomProductPageVersions]":  {"25"},
			"limit":                                {"10"},
		}
		if got := req.URL.Query(); got.Encode() != want.Encode() {
			t.Errorf("query = %q, want %q", got.Encode(), want.Encode())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"appCustomProductPages","id":"page-1"}],"links":{"next":""}}`)
	})

	args := []string{
		"product-pages", "custom-pages", "list", "--app", "app-1",
		"--visible", "false,true",
		"--fields", "name,visible,app",
		"--app-fields", "name,bundleId",
		"--version-fields", "version,state",
		"--include", "app,appCustomProductPageVersions",
		"--versions-limit", "25",
		"--limit", "10",
		"--output", "json",
	}
	stdout, stderr, err := runRootWithDeps(t, client, args...)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, `"id":"page-1"`) {
		t.Fatalf("stdout = %q, want page-1", stdout)
	}
}

func TestCustomPagesListRejectsInvalidQueryFlagsBeforeAuth(t *testing.T) {
	next := []string{"--next", "https://api.appstoreconnect.apple.com/v1/apps/app-1/appCustomProductPages?cursor=next"}
	app := []string{"--app", "app-1"}
	tests := []struct {
		name string
		base []string
		args []string
		want string
	}{
		{name: "next with visible", base: next, args: []string{"--visible", "true"}, want: "--next cannot be combined with --visible"},
		{name: "next with fields", base: next, args: []string{"--fields", "name"}, want: "--next cannot be combined with --fields"},
		{name: "next with app fields", base: next, args: []string{"--app-fields", "name"}, want: "--next cannot be combined with --app-fields"},
		{name: "next with version fields", base: next, args: []string{"--version-fields", "version"}, want: "--next cannot be combined with --version-fields"},
		{name: "next with include", base: next, args: []string{"--include", "app"}, want: "--next cannot be combined with --include"},
		{name: "next with versions limit", base: next, args: []string{"--versions-limit", "10"}, want: "--next cannot be combined with --versions-limit"},
		{name: "visible", base: app, args: []string{"--visible", "maybe"}, want: "--visible must be true or false"},
		{name: "page fields", base: app, args: []string{"--fields", "name,invalid"}, want: "--fields must be one of"},
		{name: "app fields", base: app, args: []string{"--app-fields", "invalid"}, want: "--app-fields must be one of"},
		{name: "version fields", base: app, args: []string{"--version-fields", "invalid"}, want: "--version-fields must be one of"},
		{name: "include", base: app, args: []string{"--include", "invalid"}, want: "--include must be one of"},
		{name: "versions limit explicitly zero", base: app, args: []string{"--versions-limit", "0"}, want: "--versions-limit must be between 1 and 50"},
		{name: "versions limit", base: app, args: []string{"--versions-limit", "51"}, want: "--versions-limit must be between 1 and 50"},
		{name: "visible empty", base: app, args: []string{"--visible", ""}, want: "--visible must not be empty"},
		{name: "visible delimiters", base: app, args: []string{"--visible", ","}, want: "--visible must not be empty"},
		{name: "fields empty", base: app, args: []string{"--fields", ""}, want: "--fields must not be empty"},
		{name: "fields delimiters", base: app, args: []string{"--fields", ","}, want: "--fields must not be empty"},
		{name: "app fields whitespace", base: app, args: []string{"--app-fields", " \t"}, want: "--app-fields must not be empty"},
		{name: "app fields delimiters", base: app, args: []string{"--app-fields", ","}, want: "--app-fields must not be empty"},
		{name: "version fields empty", base: app, args: []string{"--version-fields", ""}, want: "--version-fields must not be empty"},
		{name: "version fields delimiters", base: app, args: []string{"--version-fields", ","}, want: "--version-fields must not be empty"},
		{name: "include whitespace", base: app, args: []string{"--include", " \t"}, want: "--include must not be empty"},
		{name: "include delimiters", base: app, args: []string{"--include", ","}, want: "--include must not be empty"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			client := func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			}

			args := append(append([]string{"product-pages", "custom-pages", "list"}, test.base...), test.args...)
			assertUsageExitWithDeps(t, client, args, "custom-pages list: "+test.want)
			if clientFactoryCalled {
				t.Fatal("client factory ran before query validation")
			}
		})
	}
}

func TestCustomPagesListRelationshipFlagsRequireIncludes(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		parameter string
		want      string
	}{
		{
			name:      "app fields",
			args:      []string{"--app-fields", "name"},
			parameter: "--app-fields",
			want:      "custom-pages list: --app-fields requires --include app",
		},
		{
			name:      "version fields",
			args:      []string{"--include", "app", "--version-fields", "version"},
			parameter: "--version-fields",
			want:      "custom-pages list: --version-fields requires --include appCustomProductPageVersions",
		},
		{
			name:      "versions limit",
			args:      []string{"--include", "app", "--versions-limit", "10"},
			parameter: "--versions-limit",
			want:      "custom-pages list: --versions-limit requires --include appCustomProductPageVersions",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			client := func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during relationship validation")
			}

			args := append([]string{"product-pages", "custom-pages", "list", "--app", "app-1"}, test.args...)
			stdout, stderr, runErr := runRootWithDeps(t, client, args...)
			if runErr == nil || !shared.IsReportedUsageError(runErr) || errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("run error = %v, want reported usage error without flag.ErrHelp", runErr)
			}
			if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", got, rootcmd.ExitUsage)
			}
			diagnostic, ok := shared.DiagnosticFromError(runErr)
			if !ok {
				t.Fatal("run error is missing validation diagnostic")
			}
			if diagnostic.Code != shared.DiagnosticInvalidInput || diagnostic.Parameter != test.parameter {
				t.Fatalf("diagnostic = %+v, want code %q parameter %q", diagnostic, shared.DiagnosticInvalidInput, test.parameter)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			wantStderr := "Error: " + test.want + "\n"
			if stderr != wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, wantStderr)
			}
			if clientFactoryCalled {
				t.Fatal("client factory ran before relationship validation")
			}
		})
	}
}

func TestCustomPagesListAllowsNextWithOrWithoutApp(t *testing.T) {
	tests := []struct {
		name     string
		appArgs  []string
		rawQuery string
	}{
		{name: "without app", rawQuery: "cursor=next"},
		{name: "with app keeps the opaque next query", appArgs: []string{"--app", "app-compat"}, rawQuery: "cursor=opaque%2Btoken&limit=17"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nextURL := "https://api.appstoreconnect.apple.com/v1/apps/app-1/appCustomProductPages?" + test.rawQuery
			client := customPagesQueryTestClient(t, func(w http.ResponseWriter, req *http.Request) {
				if req.URL.Path != "/v1/apps/app-1/appCustomProductPages" || req.URL.RawQuery != test.rawQuery {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"data":[{"type":"appCustomProductPages","id":"page-next"}],"links":{"next":""}}`)
			})

			args := append([]string{"product-pages", "custom-pages", "list"}, test.appArgs...)
			args = append(args, "--next", nextURL, "--output", "json")
			stdout, stderr, runErr := runRootWithDeps(t, client, args...)
			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if !strings.Contains(stdout, `"id":"page-next"`) {
				t.Fatalf("stdout = %q, want page-next", stdout)
			}
		})
	}
}
