package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildsTestNotesUpdateRejectsLocalizationIDWithBuildSelector(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "test-notes", "update",
			"--localization-id", "loc-1",
			"--build-id", "build-1",
			"--whats-new", "test",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --localization-id combined with --build-id, got %v", runErr)
	}
	if !strings.Contains(stderr, "--localization-id cannot be combined with build selectors or --locale") {
		t.Fatalf("expected conflict stderr, got %q", stderr)
	}
}

func TestBuildsTestNotesUpdateRejectsBuildWithoutLocale(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "test-notes", "update",
			"--build-id", "build-1",
			"--whats-new", "test",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --build-id set without --locale, got %v", runErr)
	}
	if !strings.Contains(stderr, "either --localization-id or (--locale and a build selector) is required") {
		t.Fatalf("expected missing-locale stderr, got %q", stderr)
	}
}

func TestBuildsTestNotesUpdateRejectsLocaleWithoutBuild(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "test-notes", "update",
			"--locale", "en-US",
			"--whats-new", "test",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --locale set without --build, got %v", runErr)
	}
	if !strings.Contains(stderr, "either --localization-id or (--locale and a build selector) is required") {
		t.Fatalf("expected missing-build stderr, got %q", stderr)
	}
}

func TestBuildsTestNotesUpdateRejectsInvalidLocale(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "test-notes", "update",
			"--build-id", "build-1",
			"--locale", "!!!bad!!!",
			"--whats-new", "test",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected invalid locale error")
	}
	if !strings.Contains(runErr.Error(), "invalid locale") {
		t.Fatalf("expected invalid locale error, got %v", runErr)
	}
}

func TestBetaGroupsAddTestersRejectsNoTesterOrEmail(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"testflight", "beta-groups", "add-testers",
			"--group", "group-1",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when neither --tester nor --email, got %v", runErr)
	}
	if !strings.Contains(stderr, "--tester or --email is required") {
		t.Fatalf("expected tester/email required stderr, got %q", stderr)
	}
}

func TestBuildsListRejectsInvalidLimit(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "list",
			"--app", "123456789",
			"--limit", "999",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected limit validation error")
	}
	if !strings.Contains(runErr.Error(), "--limit must be between 1 and 200") {
		t.Fatalf("expected limit range error, got %v", runErr)
	}
}
