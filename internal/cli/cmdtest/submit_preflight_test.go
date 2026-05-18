package cmdtest

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestSubmitPreflightCommandIsRemoved(t *testing.T) {
	root := RootCommand("1.2.3")
	cmd := findSubcommand(root, "submit", "preflight")
	if cmd == nil {
		t.Fatal("expected removed submit preflight shim to remain for migration guidance")
		return
	}
	if !strings.Contains(cmd.ShortHelp, "removed") {
		t.Fatalf("expected removed submit preflight shim, got short help %q", cmd.ShortHelp)
	}
}

func TestSubmitHelpNoLongerMentionsDeprecatedCompatibilityPaths(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"submit"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		_ = root.Run(context.Background())
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Submission lifecycle tools; use `publish appstore --submit` to ship.") {
		t.Fatalf("expected submit help, got %q", stderr)
	}
	if strings.Contains(stderr, "submit preflight") {
		t.Fatalf("expected submit help to stop mentioning submit preflight, got %q", stderr)
	}
}
