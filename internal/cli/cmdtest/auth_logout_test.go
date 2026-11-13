package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"path/filepath"
	"strings"
	"testing"

	authsvc "github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	authcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/auth"
)

// cmdtest runs with ASC_BYPASS_KEYCHAIN=1, so every logout prints this note.
const authLogoutBypassNote = "Note: ASC_BYPASS_KEYCHAIN is set, so auth logout changes config files only and leaves keychain entries untouched.\n"

type authLogoutCalls struct {
	names   []string
	all     int
	options []authsvc.RemoveOptions
}

func stubAuthLogoutRemovers(t *testing.T) *authLogoutCalls {
	t.Helper()
	// The retained-global-config warning reads the global config, so keep it
	// away from the developer's home directory.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	calls := &authLogoutCalls{}
	restore := authcmd.SetLogoutCredentialRemovers(
		func(name string, opts authsvc.RemoveOptions) error {
			calls.names = append(calls.names, name)
			calls.options = append(calls.options, opts)
			return nil
		},
		func(opts authsvc.RemoveOptions) error {
			calls.all++
			calls.options = append(calls.options, opts)
			return nil
		},
	)
	t.Cleanup(restore)
	return calls
}

func TestAuthLogoutUsageErrorsRemoveNothing(t *testing.T) {
	const missingConfirm = "--confirm is required to remove stored credentials"
	for _, tt := range []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "implicit all without confirm", args: []string{"auth", "logout"}, wantErr: missingConfirm},
		{name: "explicit all without confirm", args: []string{"auth", "logout", "--all"}, wantErr: missingConfirm},
		{name: "named without confirm", args: []string{"auth", "logout", "--name", "demo"}, wantErr: missingConfirm},
		{name: "blank name", args: []string{"auth", "logout", "--name", "   "}, wantErr: "--name cannot be blank"},
		{name: "explicit false confirm", args: []string{"auth", "logout", "--all", "--confirm=false"}, wantErr: "--confirm must be true when specified"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := stubAuthLogoutRemovers(t)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(tt.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("run error = %v, want flag.ErrHelp", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, tt.wantErr) {
				t.Fatalf("expected %q in stderr, got %q", tt.wantErr, stderr)
			}
			if strings.Contains(stderr, "Warning") || strings.Contains(stderr, "5.0.0") {
				t.Fatalf("stderr still carries deprecation wording: %q", stderr)
			}
			if len(calls.names) != 0 || calls.all != 0 {
				t.Fatalf("expected no removal, got %+v", calls)
			}
		})
	}
}
