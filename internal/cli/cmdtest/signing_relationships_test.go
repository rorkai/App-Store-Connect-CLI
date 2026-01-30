package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestSigningRelationshipsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "certificates relationships pass-type-id missing id",
			args:    []string{"certificates", "relationships", "pass-type-id"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "profiles relationships bundle-id missing id",
			args:    []string{"profiles", "relationships", "bundle-id"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "profiles relationships certificates missing id",
			args:    []string{"profiles", "relationships", "certificates"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "profiles relationships devices missing id",
			args:    []string{"profiles", "relationships", "devices"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "users visible-apps list missing id",
			args:    []string{"users", "visible-apps", "list"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "users visible-apps get missing id",
			args:    []string{"users", "visible-apps", "get"},
			wantErr: "Error: --id is required",
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
