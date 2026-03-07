package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"testing"
)

func TestSnitchMissingDescription(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"snitch"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "description is required") {
		t.Fatalf("expected 'description is required' error, got %q", stderr)
	}
}

func TestSnitchInvalidSeverity(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"snitch", "--severity", "critical", "test issue"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--severity must be one of") {
		t.Fatalf("expected severity validation error, got %q", stderr)
	}
}

func TestSnitchDryRunNoToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"snitch", "--dry-run", "test dry run"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err != nil {
			t.Fatalf("expected no error for dry-run, got %v", err)
		}
	})

	if !strings.Contains(stderr, "Dry run: would create issue") {
		t.Fatalf("expected dry-run output, got %q", stderr)
	}
	if !strings.Contains(stderr, "test dry run") {
		t.Fatalf("expected issue title in dry-run output, got %q", stderr)
	}
}

func TestSnitchLocalLog(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"snitch", "--local", "local test entry"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "Friction logged") {
		t.Fatalf("expected friction logged message, got %q", stderr)
	}
}

func TestSnitchNoTokenReturnsError(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"snitch", "test without token"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error when no token is set")
		}
		if !strings.Contains(err.Error(), "GITHUB_TOKEN or GH_TOKEN is required") {
			t.Fatalf("expected token error, got: %v", err)
		}
	})
}

func TestSnitchFlushNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"snitch", "flush"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "No local snitch entries found") {
		t.Fatalf("expected no entries message, got %q", stderr)
	}
}
