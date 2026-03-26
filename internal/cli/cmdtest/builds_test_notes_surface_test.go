package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestRootHelpHidesBetaBuildLocalizationsRoot(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if strings.Contains(stderr, "  beta-build-localizations:") {
		t.Fatalf("expected root help to hide beta-build-localizations, got %q", stderr)
	}
}

func TestBuildsTestNotesHelpShowsViewNotGet(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "test-notes"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "view") {
		t.Fatalf("expected builds test-notes help to contain view, got %q", stderr)
	}
	if !strings.Contains(stderr, `asc builds test-notes list --build-id "BUILD_ID"`) {
		t.Fatalf("expected builds test-notes help to show build-id examples, got %q", stderr)
	}
	if strings.Contains(stderr, `asc builds test-notes view --id "LOCALIZATION_ID"`) {
		t.Fatalf("expected builds test-notes help to avoid canonical --id examples, got %q", stderr)
	}
	if strings.Contains(stderr, "\n  get ") || strings.Contains(stderr, "\n  get\t") {
		t.Fatalf("expected builds test-notes help to hide get alias, got %q", stderr)
	}
}

func TestRemovedBuildsTestNotesGetShowsGuidance(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "test-notes", "get", "--id", "loc-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if strings.Contains(stderr, "Unknown command: get") {
		t.Fatalf("expected targeted removal guidance, got %q", stderr)
	}
	if !strings.Contains(stderr, "Error: `asc builds test-notes get` was removed. Use `asc builds test-notes view` instead.") {
		t.Fatalf("expected removal guidance, got %q", stderr)
	}
}
