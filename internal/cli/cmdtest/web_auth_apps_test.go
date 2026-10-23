package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	webcli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/web"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func webPasswordEnvNameForTest() string {
	return strings.Join([]string{"ASC", "WEB", "PASSWORD"}, "_")
}

func TestWebAuthStatusWithoutCacheReturnsUnauthenticated(t *testing.T) {
	t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "file")
	t.Setenv("ASC_WEB_SESSION_CACHE_DIR", t.TempDir())
	t.Setenv("ASC_WEB_SESSION_CACHE", "1")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "auth", "status", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"authenticated":false`) {
		t.Fatalf("expected authenticated=false output, got %q", stdout)
	}
}

func TestWebAuthLoginRequiresPasswordSource(t *testing.T) {
	t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "file")
	t.Setenv("ASC_WEB_SESSION_CACHE_DIR", t.TempDir())
	t.Setenv(webPasswordEnvNameForTest(), "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "auth", "login", "--apple-id", "user@example.com"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "password is required") {
		t.Fatalf("expected password-required message, got %q", stderr)
	}
}

func TestWebAppsCreateHelpMentionsInteractiveContract(t *testing.T) {
	root := RootCommand("1.2.3")
	cmd := findSubcommand(root, "web", "apps", "create")
	if cmd == nil {
		t.Fatal("expected web apps create command")
		return
	}

	usage := cmd.UsageFunc(cmd)
	if !strings.Contains(usage, "interactive terminal") {
		t.Fatalf("expected interactive contract in usage, got %q", usage)
	}
}

func TestWebAppsCreateMissingFieldsWithControllingTTYReturnsUsageError(t *testing.T) {
	t.Cleanup(webcli.SetControllingTTYForTesting(func() (*os.File, error) {
		return os.Open(os.DevNull)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "apps", "create", "--name", "My App"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "missing required flags: --bundle-id, --sku") {
		t.Fatalf("expected missing-flags usage error, got %q", stderr)
	}
}

func TestWebAuthLogoutMutuallyExclusiveFlags(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "auth", "logout", "--all", "--apple-id", "user@example.com"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("expected mutually-exclusive validation error, got %q", stderr)
	}
}

func TestWebAuthLoginExplicitKeychainBackendWithBypassFailsBeforeSignIn(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_WEB_SESSION_CACHE", "1")
	t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "keychain")
	t.Setenv("ASC_WEB_SESSION_CACHE_DIR", t.TempDir())
	t.Setenv(webPasswordEnvNameForTest(), "secret")

	loginCalls := 0
	t.Cleanup(webcli.SetWebLogin(func(context.Context, webcore.LoginCredentials) (*webcore.AuthSession, error) {
		loginCalls++
		return nil, errors.New("unexpected sign-in")
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "auth", "login", "--apple-id", "user@example.com"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, runErr)
	}
	if loginCalls != 0 {
		t.Fatalf("login calls = %d, want 0", loginCalls)
	}
	if !strings.Contains(stderr, "disabled by ASC_BYPASS_KEYCHAIN") {
		t.Fatalf("expected keychain bypass conflict message, got %q", stderr)
	}
}
