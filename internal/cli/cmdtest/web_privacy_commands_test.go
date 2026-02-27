package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestWebPrivacyCommandsAreRegistered(t *testing.T) {
	root := RootCommand("1.2.3")

	for _, path := range [][]string{
		{"web", "privacy"},
		{"web", "privacy", "catalog"},
		{"web", "privacy", "pull"},
		{"web", "privacy", "plan"},
		{"web", "privacy", "apply"},
		{"web", "privacy", "publish"},
	} {
		if sub := findSubcommand(root, path...); sub == nil {
			t.Fatalf("expected command %q to be registered", strings.Join(path, " "))
		}
	}
}

func TestWebPrivacyPullRequiresApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "privacy", "pull"}); err != nil {
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

func TestWebPrivacyCatalogRejectsPositionalArgs(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "privacy", "catalog", "extra"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "does not accept positional arguments") {
		t.Fatalf("expected positional args usage message, got %q", stderr)
	}
}

func TestWebPrivacyPlanRequiresFile(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "privacy", "plan", "--app", "123456789"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "--file is required") {
		t.Fatalf("expected missing --file message, got %q", stderr)
	}
}

func TestWebPrivacyApplyRequiresConfirmWhenAllowDeletesSet(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"web", "privacy", "apply",
			"--app", "123456789",
			"--file", "privacy.json",
			"--allow-deletes",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "--confirm is required when --allow-deletes is set") {
		t.Fatalf("expected missing --confirm message, got %q", stderr)
	}
}

func TestWebPrivacyPublishRequiresConfirm(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"web", "privacy", "publish",
			"--app", "123456789",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected missing --confirm message, got %q", stderr)
	}
}
