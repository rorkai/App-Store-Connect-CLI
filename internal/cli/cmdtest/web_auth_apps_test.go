package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
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
	}

	usage := cmd.UsageFunc(cmd)
	if !strings.Contains(usage, "interactive terminal") {
		t.Fatalf("expected interactive contract in usage, got %q", usage)
	}
	passwordFlag := "--" + "password"
	if !strings.Contains(usage, passwordFlag) {
		t.Fatalf("expected temporary password compatibility in usage, got %q", usage)
	}
}

func TestWebAppsCreateRequiresAppleIDWhenNoCacheAndNoTTY(t *testing.T) {
	t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "file")
	t.Setenv("ASC_WEB_SESSION_CACHE_DIR", t.TempDir())
	t.Setenv(webPasswordEnvNameForTest(), "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	var stderr string
	withNonTTYStdin(t, func() {
		_, stderr = captureOutput(t, func() {
			if err := root.Parse([]string{
				"web", "apps", "create",
				"--name", "My App",
				"--bundle-id", "com.example.app",
				"--sku", "SKU123",
			}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			runErr = root.Run(context.Background())
		})
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "no cached web session is available") {
		t.Fatalf("expected missing cached-session message, got %q", stderr)
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
