package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestAppEventsValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "localizations screenshots list missing localization id",
			args:    []string{"app-events", "localizations", "screenshots", "list"},
			wantErr: "Error: --localization-id is required",
		},
		{
			name:    "localizations video-clips list missing localization id",
			args:    []string{"app-events", "localizations", "video-clips", "list"},
			wantErr: "Error: --localization-id is required",
		},
		{
			name:    "localizations screenshots relationships missing localization id",
			args:    []string{"app-events", "localizations", "screenshots-relationships"},
			wantErr: "Error: --localization-id is required",
		},
		{
			name:    "localizations video-clips relationships missing localization id",
			args:    []string{"app-events", "localizations", "video-clips-relationships"},
			wantErr: "Error: --localization-id is required",
		},
		{
			name:    "relationships missing event id",
			args:    []string{"app-events", "relationships"},
			wantErr: "Error: --event-id is required",
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
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}
