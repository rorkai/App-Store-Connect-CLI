package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestGameCenterEnabledVersionsListValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "enabled-versions", "list"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --app is required (or set ASC_APP_ID)") {
		t.Fatalf("expected missing app error, got %q", stderr)
	}
}

func TestGameCenterEnabledVersionsCompatibleVersionsValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "enabled-versions", "compatible-versions"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --id is required") {
		t.Fatalf("expected missing id error, got %q", stderr)
	}
}

func TestGameCenterEnabledVersionsListLimitValidation(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "enabled-versions", "list", "--app", "APP_ID", "--limit", "300"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestGameCenterEnabledVersionsCompatibleVersionsLimitValidation(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "enabled-versions", "compatible-versions", "--id", "ENABLED_VERSION_ID", "--limit", "300"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestGameCenterEnabledVersionsOutputErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "enabled-versions list unsupported output",
			args: []string{"game-center", "enabled-versions", "list", "--app", "APP_ID", "--output", "yaml"},
		},
		{
			name: "enabled-versions list pretty with table",
			args: []string{"game-center", "enabled-versions", "list", "--app", "APP_ID", "--output", "table", "--pretty"},
		},
		{
			name: "enabled-versions list pretty with markdown",
			args: []string{"game-center", "enabled-versions", "list", "--app", "APP_ID", "--output", "markdown", "--pretty"},
		},
		{
			name: "enabled-versions compatible unsupported output",
			args: []string{"game-center", "enabled-versions", "compatible-versions", "--id", "ENABLED_VERSION_ID", "--output", "yaml"},
		},
		{
			name: "enabled-versions compatible pretty with table",
			args: []string{"game-center", "enabled-versions", "compatible-versions", "--id", "ENABLED_VERSION_ID", "--output", "table", "--pretty"},
		},
		{
			name: "enabled-versions compatible pretty with markdown",
			args: []string{"game-center", "enabled-versions", "compatible-versions", "--id", "ENABLED_VERSION_ID", "--output", "markdown", "--pretty"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected non-help error, got %v", err)
				}
			})

			_ = stdout
			_ = stderr
		})
	}
}
