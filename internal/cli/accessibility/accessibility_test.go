package accessibility

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestAccessibilityValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name       string
		newCommand func(*shared.Deps) *ffcli.Command
		args       []string
		wantStderr string
	}{
		{name: "list missing app", newCommand: newAccessibilityListCommand, wantStderr: "Error: --app is required (or set ASC_APP_ID)\n"},
		{name: "get missing id", newCommand: newAccessibilityGetCommand, wantStderr: "Error: --id is required\n"},
		{name: "create missing device-family", newCommand: newAccessibilityCreateCommand, args: []string{"--app", "APP_ID"}, wantStderr: "Error: --device-family is required\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd := test.newCommand(&shared.Deps{
				Client: func() (*asc.Client, error) {
					return nil, errors.New("unexpected client request")
				},
				IO: &shared.IO{Out: &stdout, ErrOut: &stderr},
			})
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", err)
			}
			if got := stderr.String(); got != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", got, test.wantStderr)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
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
