package accessibility

import (
	"context"
	"errors"
	"flag"
	"testing"
)

func TestAccessibilityCommandShape(t *testing.T) {
	cmd := AccessibilityCommand()
	if cmd == nil {
		t.Fatal("expected accessibility command")
		return
	}
	if cmd.Name != "accessibility" {
		t.Fatalf("unexpected command name: %q", cmd.Name)
	}
	if len(cmd.Subcommands) != 5 {
		t.Fatalf("expected 5 subcommands, got %d", len(cmd.Subcommands))
	}
	if got := AccessibilityCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
}

func TestAccessibilityValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	t.Run("list missing app", func(t *testing.T) {
		cmd := AccessibilityListCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("get missing id", func(t *testing.T) {
		cmd := AccessibilityGetCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("create missing device-family", func(t *testing.T) {
		cmd := AccessibilityCreateCommand()
		if err := cmd.FlagSet.Parse([]string{"--app", "APP_ID"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("update missing update flags", func(t *testing.T) {
		cmd := AccessibilityUpdateCommand()
		if err := cmd.FlagSet.Parse([]string{"--id", "DECLARATION_ID"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := cmd.Exec(context.Background(), nil)
		if err == nil || errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected non-ErrHelp error, got %v", err)
		}
	})
}

func TestAccessibilityHelpers(t *testing.T) {
	if got, err := normalizeAccessibilityDeviceFamily("iphone"); err != nil || got != "IPHONE" {
		t.Fatalf("expected IPHONE, got %q err=%v", got, err)
	}
	if _, err := normalizeAccessibilityDeviceFamily("bad"); err == nil {
		t.Fatal("expected error for invalid device family")
	}
	if _, err := normalizeAccessibilityStates([]string{"BAD"}); err == nil {
		t.Fatal("expected error for invalid state")
	}
	if _, err := normalizeAccessibilityDeclarationFields("invalidField"); err == nil {
		t.Fatal("expected error for invalid fields")
	}
	if _, err := buildAccessibilityDeclarationCreateAttributes("IPHONE", map[string]string{
		"supports-voiceover": "not-a-bool",
	}); err == nil {
		t.Fatal("expected bool parsing error")
	}
}
