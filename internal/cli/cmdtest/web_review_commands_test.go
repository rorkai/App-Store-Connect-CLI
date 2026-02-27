package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestWebReviewCommandsAreRegisteredAndLegacyRemoved(t *testing.T) {
	root := RootCommand("1.2.3")

	for _, path := range [][]string{
		{"web", "review"},
		{"web", "review", "list"},
		{"web", "review", "show"},
	} {
		if sub := findSubcommand(root, path...); sub == nil {
			t.Fatalf("expected command %q to be registered", strings.Join(path, " "))
		}
	}

	for _, legacyPath := range [][]string{
		{"web", "submissions"},
		{"web", "review", "threads"},
		{"web", "review", "messages"},
		{"web", "review", "rejections"},
		{"web", "review", "draft"},
		{"web", "review", "attachments"},
	} {
		if sub := findSubcommand(root, legacyPath...); sub != nil {
			t.Fatalf("expected legacy command %q to be removed", strings.Join(legacyPath, " "))
		}
	}
}

func TestWebReviewListRequiresApp(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "review", "list"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "--app is required") {
		t.Fatalf("expected missing --app message, got %q", stderr)
	}
}

func TestWebReviewListRejectsUnknownState(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"web", "review", "list",
			"--app", "123456789",
			"--state", "UNRESOLVED_ISSUES,NOT_A_REAL_STATE",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "unsupported value") {
		t.Fatalf("expected unsupported state message, got %q", stderr)
	}
}

func TestWebReviewShowRequiresApp(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "review", "show"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "--app is required") {
		t.Fatalf("expected missing --app message, got %q", stderr)
	}
}

func TestWebReviewShowRequiresAppleIDWhenNoMatchingCache(t *testing.T) {
	t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "file")
	t.Setenv("ASC_WEB_SESSION_CACHE_DIR", t.TempDir())
	t.Setenv("ASC_WEB_SESSION_CACHE", "1")
	t.Setenv("ASC_WEB_PASSWORD", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"web", "review", "show",
			"--app", "123456789",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "--apple-id is required") {
		t.Fatalf("expected missing apple-id message, got %q", stderr)
	}
}

func TestWebReviewShowRejectsInvalidPattern(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"web", "review", "show",
			"--app", "123456789",
			"--pattern", "[",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "--pattern is invalid") {
		t.Fatalf("expected invalid pattern message, got %q", stderr)
	}
}
