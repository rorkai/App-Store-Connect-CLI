package web

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebXcodeCloudScmCommandHierarchy(t *testing.T) {
	cmd := WebXcodeCloudCommand()
	scm := findSub(cmd, "scm")
	if scm == nil {
		t.Fatal("expected scm subcommand")
	}
	providers := findSub(scm, "providers")
	if providers == nil {
		t.Fatal("expected providers subcommand")
	}
	list := findSub(providers, "list")
	if list == nil {
		t.Fatal("expected providers list subcommand")
	}
	status := findSub(scm, "connection-status")
	if status == nil {
		t.Fatal("expected connection-status subcommand")
	}
	if scm.UsageFunc == nil || providers.UsageFunc == nil || list.UsageFunc == nil || status.UsageFunc == nil {
		t.Fatal("expected UsageFunc on every SCM command")
	}
	if list.FlagSet.Lookup("output") == nil || list.FlagSet.Lookup("pretty") == nil {
		t.Fatal("expected output flags on providers list")
	}
	if status.FlagSet.Lookup("scm-provider-id") == nil {
		t.Fatal("expected --scm-provider-id on connection-status")
	}
}

func TestWebXcodeCloudScmProvidersListPreservesRawJSONAndHumanRows(t *testing.T) {
	const response = `[{"id":"provider-1","provider":"github","provider_display_name":"GitHub","is_registered":true,"is_user_connected":false,"future_field":{"keep":true}}]`

	for _, format := range []string{"json", "table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			stubCIScmSession(t, func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/ci/api/teams/team-uuid/scm-providers-v2" {
					t.Fatalf("path = %q, want /ci/api/teams/team-uuid/scm-providers-v2", req.URL.Path)
				}
				if req.URL.RawQuery != "" {
					t.Fatalf("query = %q, want empty", req.URL.RawQuery)
				}
				if req.Body != nil {
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("read request body: %v", err)
					}
					if len(body) != 0 {
						t.Fatalf("request body = %q, want empty", body)
					}
				}
				return ciScmHTTPResponse(req, response), nil
			})

			cmd := webXcodeCloudScmProvidersListCommand()
			if err := cmd.FlagSet.Parse([]string{"--output", format}); err != nil {
				t.Fatalf("parse: %v", err)
			}
			stdout, stderr := captureOutput(t, func() {
				if err := cmd.Exec(context.Background(), nil); err != nil {
					t.Fatalf("Exec() error = %v", err)
				}
			})
			if stderr != "" {
				t.Fatalf("unexpected stderr = %q", stderr)
			}
			for _, want := range []string{"provider-1", "github", "GitHub", "true", "false"} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("%s output missing %q: %q", format, want, stdout)
				}
			}
			if format == "json" {
				for _, want := range []string{`"provider_display_name":"GitHub"`, `"future_field":{"keep":true}`, `"is_user_connected":false`} {
					if !strings.Contains(stdout, want) {
						t.Fatalf("raw JSON output missing %q: %q", want, stdout)
					}
				}
			} else {
				for _, want := range []string{"ID", "Provider", "Name", "Registered", "Connected"} {
					if !strings.Contains(stdout, want) {
						t.Fatalf("%s output missing header %q: %q", format, want, stdout)
					}
				}
			}
		})
	}
}

func TestWebXcodeCloudScmConnectionStatusPreservesRawJSON(t *testing.T) {
	const response = `{"status":"success","future_status_field":"keep","error":null}`
	stubCIScmSession(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/ci/api/teams/team-uuid/scm-providers/provider-1/connection-v2" {
			t.Fatalf("path = %q, want /ci/api/teams/team-uuid/scm-providers/provider-1/connection-v2", req.URL.Path)
		}
		if req.URL.RawQuery != "" {
			t.Fatalf("query = %q, want empty", req.URL.RawQuery)
		}
		return ciScmHTTPResponse(req, response), nil
	})

	cmd := webXcodeCloudScmConnectionStatusCommand()
	if err := cmd.FlagSet.Parse([]string{"--scm-provider-id", "provider-1", "--output", "json"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("Exec() error = %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("unexpected stderr = %q", stderr)
	}
	for _, want := range []string{`"status":"success"`, `"future_status_field":"keep"`, `"error":null`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("status output missing %q: %q", want, stdout)
		}
	}
}

func TestWebXcodeCloudScmValidatesBeforeSession(t *testing.T) {
	original := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = original })
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		t.Fatal("web session must not be resolved for invalid SCM arguments")
		return nil, "", nil
	}

	tests := []struct {
		name string
		cmd  func() *ffcli.Command
		args []string
		want string
	}{
		{
			name: "connection status missing provider",
			cmd:  webXcodeCloudScmConnectionStatusCommand,
			want: "--scm-provider-id is required",
		},
		{
			name: "providers invalid output combination",
			cmd:  webXcodeCloudScmProvidersListCommand,
			args: []string{"--output", "table", "--pretty"},
			want: "--pretty is only valid with JSON output",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.cmd()
			if err := cmd.FlagSet.Parse(tt.args); err != nil {
				t.Fatalf("parse: %v", err)
			}
			var runErr error
			_, stderr := captureOutput(t, func() { runErr = cmd.Exec(context.Background(), cmd.FlagSet.Args()) })
			if runErr == nil {
				t.Fatalf("error = nil, stderr = %q, want %q", stderr, tt.want)
			}
			if !errors.Is(runErr, flag.ErrHelp) && !strings.Contains(runErr.Error(), tt.want) {
				t.Fatalf("error = %v, stderr = %q, want %q", runErr, stderr, tt.want)
			}
			if !strings.Contains(stderr+runErr.Error(), tt.want) {
				t.Fatalf("error/stderr missing %q: error=%v stderr=%q", tt.want, runErr, stderr)
			}
		})
	}
}

func TestWebXcodeCloudScmRequiresPublicProviderIDBeforeHTTP(t *testing.T) {
	original := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = original })
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("HTTP must not run without a public provider ID")
				return nil, nil
			})},
		}, "cache", nil
	}

	cmd := webXcodeCloudScmProvidersListCommand()
	if err := cmd.FlagSet.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "session has no public provider ID") {
		t.Fatalf("error = %v, want public provider ID error", err)
	}
}

func TestWebXcodeCloudScmReadFlagsHaveNoMutationInputs(t *testing.T) {
	for _, tc := range []struct {
		name string
		cmd  *ffcli.Command
	}{
		{name: "providers list", cmd: webXcodeCloudScmProvidersListCommand()},
		{name: "connection status", cmd: webXcodeCloudScmConnectionStatusCommand()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, forbidden := range []string{"confirm", "repository-id", "product-id", "oauth", "pat"} {
				if tc.cmd.FlagSet.Lookup(forbidden) != nil {
					t.Fatalf("unexpected mutation flag --%s", forbidden)
				}
			}
		})
	}
}

func stubCIScmSession(t *testing.T, transport func(*http.Request) (*http.Response, error)) {
	t.Helper()
	original := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = original })
	resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			PublicProviderID: "team-uuid",
			Client:           &http.Client{Transport: roundTripFunc(transport)},
		}, "cache", nil
	}
}

func ciScmHTTPResponse(req *http.Request, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}
