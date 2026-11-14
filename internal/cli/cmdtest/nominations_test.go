package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestNominationsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "nominations list missing status",
			args:    []string{"nominations", "list"},
			wantErr: "--status is required",
		},
		{
			name:    "nominations create missing app",
			args:    []string{"nominations", "create", "--name", "Launch", "--type", "APP_LAUNCH", "--description", "desc", "--submitted=false", "--publish-start-date", "2026-02-01T08:00:00Z"},
			wantErr: "--app is required",
		},
		{
			name:    "nominations create missing name",
			args:    []string{"nominations", "create", "--app", "APP_ID", "--type", "APP_LAUNCH", "--description", "desc", "--submitted=false", "--publish-start-date", "2026-02-01T08:00:00Z"},
			wantErr: "--name is required",
		},
		{
			name:    "nominations create missing type",
			args:    []string{"nominations", "create", "--app", "APP_ID", "--name", "Launch", "--description", "desc", "--submitted=false", "--publish-start-date", "2026-02-01T08:00:00Z"},
			wantErr: "--type is required",
		},
		{
			name:    "nominations create invalid type",
			args:    []string{"nominations", "create", "--app", "APP_ID", "--name", "Launch", "--type", "INVALID", "--description", "desc", "--submitted=false", "--publish-start-date", "2026-02-01T08:00:00Z"},
			wantErr: "--type must be one of",
		},
		{
			name:    "nominations create missing submitted",
			args:    []string{"nominations", "create", "--app", "APP_ID", "--name", "Launch", "--type", "APP_LAUNCH", "--description", "desc", "--publish-start-date", "2026-02-01T08:00:00Z"},
			wantErr: "--submitted is required",
		},
		{
			name:    "nominations create invalid publish date",
			args:    []string{"nominations", "create", "--app", "APP_ID", "--name", "Launch", "--type", "APP_LAUNCH", "--description", "desc", "--submitted=false", "--publish-start-date", "2026-02-01"},
			wantErr: "--publish-start-date must be in RFC3339 format",
		},
		{
			name:    "nominations update missing id",
			args:    []string{"nominations", "update", "--name", "Updated"},
			wantErr: "--id is required",
		},
		{
			name:    "nominations update missing updates",
			args:    []string{"nominations", "update", "--id", "NOM_ID"},
			wantErr: "at least one update flag is required",
		},
		{
			name:    "nominations update invalid type",
			args:    []string{"nominations", "update", "--id", "NOM_ID", "--type", "INVALID", "--submitted=false"},
			wantErr: "--type must be one of",
		},
		{
			name:    "nominations update invalid publish date",
			args:    []string{"nominations", "update", "--id", "NOM_ID", "--publish-start-date", "2026-02-01", "--submitted=false"},
			wantErr: "--publish-start-date must be in RFC3339 format",
		},
		{
			name:    "nominations update missing submitted or archived",
			args:    []string{"nominations", "update", "--id", "NOM_ID", "--notes", "Updated"},
			wantErr: "--submitted or --archived is required by Apple's live API",
		},
		{
			name:    "nominations delete missing id",
			args:    []string{"nominations", "delete", "--confirm"},
			wantErr: "--id is required",
		},
		{
			name:    "nominations delete missing confirm",
			args:    []string{"nominations", "delete", "--id", "NOM_ID"},
			wantErr: "--confirm is required to delete",
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
