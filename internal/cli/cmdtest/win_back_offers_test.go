package cmdtest

import (
	"context"
	"errors"
	"flag"
	"path/filepath"
	"strings"
	"testing"
)

func TestWinBackOffersValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "list missing subscription",
			args:    []string{"subscriptions", "offers", "win-back", "list"},
			wantErr: "Error: --subscription-id is required",
		},
		{
			name:    "get missing id",
			args:    []string{"subscriptions", "offers", "win-back", "get"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "create missing subscription",
			args:    []string{"subscriptions", "offers", "win-back", "create"},
			wantErr: "Error: --subscription-id is required",
		},
		{
			name:    "create missing reference-name",
			args:    []string{"subscriptions", "offers", "win-back", "create", "--subscription-id", "SUB_ID"},
			wantErr: "Error: --reference-name is required",
		},
		{
			name:    "update missing id",
			args:    []string{"subscriptions", "offers", "win-back", "update", "--priority", "NORMAL"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "update missing updates",
			args:    []string{"subscriptions", "offers", "win-back", "update", "--id", "OFFER_ID"},
			wantErr: "Error: at least one update flag is required",
		},
		{
			name:    "delete missing id",
			args:    []string{"subscriptions", "offers", "win-back", "delete", "--confirm"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "delete missing confirm",
			args:    []string{"subscriptions", "offers", "win-back", "delete", "--id", "OFFER_ID"},
			wantErr: "Error: --confirm is required",
		},
		{
			name:    "prices missing id",
			args:    []string{"subscriptions", "offers", "win-back", "prices"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "prices relationships missing id",
			args:    []string{"subscriptions", "offers", "win-back", "prices-links"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "relationships missing subscription",
			args:    []string{"subscriptions", "offers", "win-back", "links"},
			wantErr: "Error: --subscription-id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")

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
