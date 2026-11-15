package cmdtest

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestTestFlightGroupReadsSendRequest(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantPath  string
		wantQuery url.Values
		body      string
		deps      bool // the command is built on shared.Deps: run it on test deps
	}{
		{
			name:     "group builds with fields",
			args:     []string{"testflight", "groups", "builds", "list", "--group-id", "group-1", "--fields", "version,uploadedDate,processingState", "--limit", "5"},
			deps:     true,
			wantPath: "/v1/betaGroups/group-1/builds",
			wantQuery: url.Values{
				"fields[builds]": {"version,uploadedDate,processingState"},
				"limit":          {"5"},
			},
		},
		{
			name:      "group testers paginate keep the page size",
			args:      []string{"testflight", "groups", "testers", "list", "--group-id", "group-1", "--paginate", "--limit", "3"},
			deps:      true,
			wantPath:  "/v1/betaGroups/group-1/betaTesters",
			wantQuery: url.Values{"limit": {"3"}},
		},
		{
			name:      "group builds print Apple's sparse envelope unmodified",
			args:      []string{"testflight", "groups", "builds", "list", "--group-id", "group-1", "--fields", "version", "--limit", "1"},
			deps:      true,
			wantPath:  "/v1/betaGroups/group-1/builds",
			wantQuery: url.Values{"fields[builds]": {"version"}, "limit": {"1"}},
			body:      `{"data":[{"type":"builds","id":"build-1","attributes":{"version":"11"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/betaGroups/group-1/builds"}}`,
		},
		{
			name:     "group testers with fields",
			args:     []string{"testflight", "groups", "testers", "list", "--group-id", "group-1", "--fields", "email,state"},
			deps:     true,
			wantPath: "/v1/betaGroups/group-1/betaTesters",
			wantQuery: url.Values{
				"fields[betaTesters]": {"email,state"},
			},
		},
		{
			name:     "app testers with fields keep included relationships",
			args:     []string{"testflight", "testers", "list", "--app", "app-1", "--fields", "email", "--include", "betaGroups"},
			wantPath: "/v1/betaTesters",
			wantQuery: url.Values{
				"fields[betaTesters]": {"email,betaGroups"},
				"filter[apps]":        {"app-1"},
				"include":             {"betaGroups"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			body := test.body
			if body == "" {
				body = `{"data":[],"links":{}}`
			}
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET(test.wantPath, strings.Split(test.wantQuery.Encode(), "&")...),
				asctest.JSON(http.StatusOK, body))

			args := append(test.args, "--output", "json")
			var stdout, stderr string
			if test.deps {
				var code int
				stdout, stderr, code = runWithDeps(t, stubClient(t, reg), args...)
				if code != cmd.ExitSuccess {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitSuccess, code)
				}
			} else {
				setupAuth(t)
				reg.Install()
				stdout, stderr = captureOutput(t, func() {
					if code := cmd.Run(args, "1.2.3"); code != cmd.ExitSuccess {
						t.Fatalf("expected exit code %d, got %d", cmd.ExitSuccess, code)
					}
				})
			}
			requests := reg.Requests()
			if len(requests) == 0 {
				t.Fatalf("expected an outbound request, stderr %q", stderr)
			}
			captured := requests[len(requests)-1]
			if captured.Method != http.MethodGet || captured.URL.Path != test.wantPath {
				t.Fatalf("request = %s %s, want GET %s", captured.Method, captured.URL.Path, test.wantPath)
			}
			if got := captured.URL.Query().Encode(); got != test.wantQuery.Encode() {
				t.Fatalf("query = %q, want %q", got, test.wantQuery.Encode())
			}
			if got := strings.TrimSpace(stdout); got != body {
				t.Fatalf("stdout = %q, want Apple's body %q", got, body)
			}
		})
	}
}

func TestTestFlightGroupReadsRejectUsage(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/betaGroups/group-1/builds?cursor=AQ"

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "group builds without group id",
			args:    []string{"testflight", "groups", "builds", "list"},
			wantErr: "--group-id is required",
		},
		{
			name:    "group builds fields with next",
			args:    []string{"testflight", "groups", "builds", "list", "--next", nextURL, "--fields", "version"},
			wantErr: "testflight groups builds list: --next cannot be combined with --fields",
		},
		{
			name:    "group builds limit with next",
			args:    []string{"testflight", "groups", "builds", "list", "--next", nextURL, "--limit", "5"},
			wantErr: "--next cannot be combined with --limit",
		},
		{
			name:    "group builds unknown field",
			args:    []string{"testflight", "groups", "builds", "list", "--group-id", "group-1", "--fields", "name"},
			wantErr: "--fields must be one of: version, uploadedDate",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			reg := asctest.NewRegistry(t)

			stdout, stderr, code := runWithDeps(t, stubClient(t, reg), test.args...)
			if code != cmd.ExitUsage {
				t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}
