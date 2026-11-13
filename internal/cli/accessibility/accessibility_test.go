package accessibility

import (
	"context"
	"errors"
	"flag"
	"testing"
)

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
