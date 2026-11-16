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

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// webReviewAppScopedCommands lists every app-scoped `asc web review` command
// with the extra flags it needs to get past local validation, so each one is
// held to the same --app / ASC_APP_ID resolution as `web review subscriptions
// list` and the other app-scoped commands.
var webReviewAppScopedCommands = []struct {
	name string
	cmd  func(*shared.Deps) *ffcli.Command
	args []string
}{
	{name: "list", cmd: newWebReviewListCommand},
	{name: "threads", cmd: newWebReviewThreadsCommand},
	{name: "show", cmd: newWebReviewShowCommand},
	{name: "drafts create", cmd: newWebReviewDraftCreateCommand, args: []string{"--thread-id", "thread-1", "--message", "Updated the demo account.", "--confirm"}},
	{name: "drafts update", cmd: newWebReviewDraftUpdateCommand, args: []string{"--thread-id", "thread-1", "--draft-id", "draft-1", "--message", "Updated the demo account.", "--confirm"}},
	{name: "drafts delete", cmd: newWebReviewDraftDeleteCommand, args: []string{"--thread-id", "thread-1", "--draft-id", "draft-1", "--confirm"}},
	{name: "subscriptions attach", cmd: newWebReviewSubscriptionsAttachCommand, args: []string{"--subscription-id", "sub-1", "--confirm"}},
	{name: "subscriptions attach-group", cmd: newWebReviewSubscriptionsAttachGroupCommand, args: []string{"--group-id", "group-1", "--confirm"}},
	{name: "subscriptions remove", cmd: newWebReviewSubscriptionsRemoveCommand, args: []string{"--subscription-id", "sub-1", "--confirm"}},
	{name: "subscriptions remove-group", cmd: newWebReviewSubscriptionsRemoveGroupCommand, args: []string{"--group-id", "group-1", "--confirm"}},
	{name: "iaps attach", cmd: newWebReviewIAPsAttachCommand, args: []string{"--iap-id", "iap-1", "--confirm"}},
}

// stubWebReviewAppScopedSession records the path of every web request and
// answers each one with a 404, so a command stops at its first app-scoped read
// without any network access.
func stubWebReviewAppScopedSession(t *testing.T) *[]string {
	t.Helper()

	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	paths := []string{}
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					paths = append(paths, req.URL.Path)
					return &http.Response{
						StatusCode: http.StatusNotFound,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not found"}]}`)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}
	return &paths
}

func TestWebReviewAppScopedCommandsResolveAppFromEnv(t *testing.T) {
	for _, test := range webReviewAppScopedCommands {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "123456789")
			paths := stubWebReviewAppScopedSession(t)

			d, depsStdout, depsStderr := newWebTestDeps()

			cmd := test.cmd(d)
			if err := cmd.FlagSet.Parse(append([]string{"--output", "json"}, test.args...)); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			var runErr error
			_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
				runErr = cmd.Exec(context.Background(), nil)
			})

			if errors.Is(runErr, flag.ErrHelp) || strings.Contains(stderr, "--app is required") {
				t.Fatalf("expected ASC_APP_ID to satisfy --app, got err=%v stderr=%q", runErr, stderr)
			}
			if len(*paths) == 0 || !strings.Contains((*paths)[0], "/apps/123456789/") {
				t.Fatalf("expected the first request to be scoped to ASC_APP_ID app 123456789, got %#v (err=%v)", *paths, runErr)
			}
		})
	}
}

func TestWebReviewAppScopedCommandsExplicitAppWinsOverEnv(t *testing.T) {
	for _, test := range webReviewAppScopedCommands {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "999999999")
			paths := stubWebReviewAppScopedSession(t)

			d, depsStdout, depsStderr := newWebTestDeps()

			cmd := test.cmd(d)
			if err := cmd.FlagSet.Parse(append([]string{"--app", "123456789", "--output", "json"}, test.args...)); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			_, _ = runWithWebTestDeps(depsStdout, depsStderr, func() {
				_ = cmd.Exec(context.Background(), nil)
			})

			if len(*paths) == 0 || !strings.Contains((*paths)[0], "/apps/123456789/") {
				t.Fatalf("expected explicit --app to win over ASC_APP_ID, got %#v", *paths)
			}
		})
	}
}

func TestWebReviewAppScopedCommandsMissingAppNamesEnvFallback(t *testing.T) {
	for _, test := range webReviewAppScopedCommands {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			paths := stubWebReviewAppScopedSession(t)

			d, depsStdout, depsStderr := newWebTestDeps()

			cmd := test.cmd(d)
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			var runErr error
			stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
				runErr = cmd.Exec(context.Background(), nil)
			})

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected a usage error, got %v", runErr)
			}
			if want := "Error: --app is required (or set ASC_APP_ID)\n"; stderr != want {
				t.Fatalf("expected stderr %q, got %q", want, stderr)
			}
			if stdout != "" || len(*paths) != 0 {
				t.Fatalf("expected no output and no requests, got stdout=%q paths=%#v", stdout, *paths)
			}
		})
	}
}

func TestWebReviewAppScopedCommandsHelpNamesEnvFallback(t *testing.T) {
	for _, test := range webReviewAppScopedCommands {
		t.Run(test.name, func(t *testing.T) {
			d, _, _ := newWebTestDeps()
			appFlag := test.cmd(d).FlagSet.Lookup("app")
			if appFlag == nil {
				t.Fatal("expected an --app flag")
			}
			if !strings.Contains(appFlag.Usage, "(or ASC_APP_ID env)") {
				t.Fatalf("expected --app help to name the ASC_APP_ID fallback, got %q", appFlag.Usage)
			}
		})
	}
}
