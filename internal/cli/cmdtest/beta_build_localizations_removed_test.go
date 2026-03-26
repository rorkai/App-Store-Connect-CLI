package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestRemovedBetaBuildLocalizationsRootShowsGuidance(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"beta-build-localizations"}); err != nil {
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
	if strings.Contains(stderr, "Unknown command: beta-build-localizations") {
		t.Fatalf("expected targeted removal guidance, got %q", stderr)
	}
	if !strings.Contains(stderr, "Error: `asc beta-build-localizations` was removed. Use `asc builds test-notes` instead.") {
		t.Fatalf("expected removal guidance, got %q", stderr)
	}
}

func TestRemovedBetaBuildLocalizationsCommandsShowGuidance(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "list command",
			args:    []string{"beta-build-localizations", "list", "--build", "BUILD_ID"},
			wantErr: "Error: `asc beta-build-localizations list` was removed. Use `asc builds test-notes list` instead.",
		},
		{
			name:    "get command",
			args:    []string{"beta-build-localizations", "get", "--id", "LOC_ID"},
			wantErr: "Error: `asc beta-build-localizations get` was removed. Use `asc builds test-notes view` instead.",
		},
		{
			name:    "create command",
			args:    []string{"beta-build-localizations", "create", "--build", "BUILD_ID", "--locale", "en-US", "--whats-new", "notes"},
			wantErr: "Error: `asc beta-build-localizations create` was removed. Use `asc builds test-notes create` instead.",
		},
		{
			name:    "update command",
			args:    []string{"beta-build-localizations", "update", "--id", "LOC_ID", "--whats-new", "notes"},
			wantErr: "Error: `asc beta-build-localizations update` was removed. Use `asc builds test-notes update` instead.",
		},
		{
			name:    "delete command",
			args:    []string{"beta-build-localizations", "delete", "--id", "LOC_ID", "--confirm"},
			wantErr: "Error: `asc beta-build-localizations delete` was removed. Use `asc builds test-notes delete` instead.",
		},
		{
			name:    "build get command",
			args:    []string{"beta-build-localizations", "build", "get", "--id", "LOC_ID"},
			wantErr: "Error: `asc beta-build-localizations build get` was removed. No canonical replacement exists yet.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}
