package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestSigningRelationshipsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
		deps    bool // the command is built on shared.Deps: run it on test deps
	}{
		{
			name:    "certificates links pass-type-id missing id",
			args:    []string{"certificates", "links", "pass-type-id"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "profiles links bundle-id missing id",
			args:    []string{"profiles", "links", "bundle-id"},
			wantErr: "Error: --id is required",
			deps:    true,
		},
		{
			name:    "profiles links certificates missing id",
			args:    []string{"profiles", "links", "certificates"},
			wantErr: "Error: --id is required",
			deps:    true,
		},
		{
			name:    "profiles links devices missing id",
			args:    []string{"profiles", "links", "devices"},
			wantErr: "Error: --id is required",
			deps:    true,
		},
		{
			name:    "users visible-apps list missing id",
			args:    []string{"users", "visible-apps", "list"},
			wantErr: "Error: --id is required",
			deps:    true,
		},
		{
			name:    "users visible-apps view missing id",
			args:    []string{"users", "visible-apps", "view"},
			wantErr: "Error: --id is required",
			deps:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr string
			var err error
			if test.deps {
				stdout, stderr, err = runRootWithDeps(t, stubClient(t, asctest.NewRegistry(t)), test.args...)
			} else {
				root := RootCommand("1.2.3")
				root.FlagSet.SetOutput(io.Discard)

				stdout, stderr = captureOutput(t, func() {
					if parseErr := root.Parse(test.args); parseErr != nil {
						t.Fatalf("parse error: %v", parseErr)
					}
					err = root.Run(context.Background())
				})
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", err)
			}

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}
