package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
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
	if !strings.Contains(stderr, "skipping duplicate search") {
		t.Fatalf("expected offline duplicate search note, got %q", stderr)
	}
	if !strings.Contains(stderr, "test dry run") {
		t.Fatalf("expected issue title in dry-run output, got %q", stderr)
	}
}

func TestSnitchDryRunConfirmNoToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"snitch", "--dry-run", "--confirm", "test dry run confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err != nil {
			t.Fatalf("expected no error for dry-run confirm, got %v", err)
		}
	})

	if !strings.Contains(stderr, "Dry run: would create issue") {
		t.Fatalf("expected dry-run output, got %q", stderr)
	}
	if !strings.Contains(stderr, "skipping duplicate search") {
		t.Fatalf("expected offline duplicate search note, got %q", stderr)
	}
	if !strings.Contains(stderr, "test dry run confirm") {
		t.Fatalf("expected issue title in dry-run output, got %q", stderr)
	}
	if strings.Contains(stderr, "GITHUB_TOKEN or GH_TOKEN is required") {
		t.Fatalf("did not expect token requirement in dry-run confirm output, got %q", stderr)
	}
}

func TestSnitchPreviewWithoutConfirmNoToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"snitch", "preview", "without", "confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err != nil {
			t.Fatalf("expected preview mode without confirm, got %v", err)
		}
	})

	if !strings.Contains(stderr, "Preview only: rerun with --confirm to create issue") {
		t.Fatalf("expected preview-only message, got %q", stderr)
	}
	if !strings.Contains(stderr, "preview without confirm") {
		t.Fatalf("expected full multi-word description, got %q", stderr)
	}
}

func TestSnitchRejectsTrailingSnitchFlagsAfterDescription(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"snitch", "misordered description", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "flags must appear before the description") {
		t.Fatalf("expected ordering validation error, got %q", stderr)
	}
	if !strings.Contains(stderr, "--confirm") {
		t.Fatalf("expected offending flag name in error, got %q", stderr)
	}
}

func TestSnitchLocalLog(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("os.Chdir restore error: %v", err)
		}
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir temp dir error: %v", err)
	}

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

func TestSnitchDryRunLocalDoesNotWriteLog(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("os.Chdir restore error: %v", err)
		}
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir temp dir error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"snitch", "--dry-run", "--local", "local dry run entry"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected no error for dry-run local, got %v", err)
		}
	})

	if !strings.Contains(stderr, "Dry run: would create issue") {
		t.Fatalf("expected dry-run output, got %q", stderr)
	}
	if strings.Contains(stderr, "Friction logged") {
		t.Fatalf("did not expect local log write output, got %q", stderr)
	}

	logPath := filepath.Join(".asc", "snitch.log")
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("expected no local log file, got err=%v", err)
	}
}

func TestSnitchConfirmNoTokenReturnsError(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"snitch", "--confirm", "test without token"}); err != nil {
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
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("os.Chdir restore error: %v", err)
		}
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir temp dir error: %v", err)
	}

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

func TestSnitchFlushRejectsPositionalArgs(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("os.Chdir restore error: %v", err)
		}
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir temp dir error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"snitch", "flush", "/tmp/snitch.log"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "snitch flush does not accept positional arguments") {
		t.Fatalf("expected flush positional argument error, got %q", stderr)
	}
	if !strings.Contains(stderr, "--file PATH") {
		t.Fatalf("expected --file guidance, got %q", stderr)
	}
}

func TestSnitchFlushFormatsEntries(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("os.Chdir restore error: %v", err)
		}
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir temp dir error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"snitch", "--local", "--repro", `asc status --app "com.example.app"`, "status command needs bundle ID support"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	flushRoot := RootCommand("1.2.3")
	flushRoot.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := flushRoot.Parse([]string{"snitch", "flush"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := flushRoot.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected no stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "[1] bug: status command needs bundle ID support") {
		t.Fatalf("expected formatted flush output, got %q", stdout)
	}
	if !strings.Contains(stdout, `asc status --app "com.example.app"`) {
		t.Fatalf("expected reproduction details in flush output, got %q", stdout)
	}
}
