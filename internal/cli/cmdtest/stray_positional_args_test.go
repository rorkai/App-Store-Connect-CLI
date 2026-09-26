package cmdtest

import (
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// TestStrayPositionalOperandsNameTheTokenAndFlag covers the invocations that
// telemetry showed callers reaching for most often: a bare resource ID where a
// flag-only leaf command expects a flag. Every case must fail before auth or
// any request, name the offending token, and exit with the usage code.
//
// Every operand here renders identically in every shell. Quoting and the
// Windows suppression rule belong to strayPositionalHint and are covered
// platform-independently by TestStrayPositionalHintRendersSafelyPerShell.
func TestStrayPositionalOperandsNameTheTokenAndFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantErr  string
		wantHint string
		noHint   bool
	}{
		{
			name:     "apps view",
			args:     []string{"apps", "view", "123"},
			wantErr:  `Error: unexpected argument "123"`,
			wantHint: "Did you mean: asc apps view --id 123",
		},
		{
			name:     "builds list",
			args:     []string{"builds", "list", "33"},
			wantErr:  `Error: unexpected argument "33"`,
			wantHint: "Did you mean: asc builds list --app 33",
		},
		{
			name:     "bundle ids list",
			args:     []string{"bundle-ids", "list", "26"},
			wantErr:  `Error: unexpected argument "26"`,
			wantHint: "Did you mean: asc bundle-ids list --id 26",
		},
		{
			name:     "pricing availability create",
			args:     []string{"pricing", "availability", "create", "37"},
			wantErr:  `Error: unexpected argument "37"`,
			wantHint: "Did you mean: asc pricing availability create --app 37",
		},
		{
			name:    "auth login without a primary identifier flag",
			args:    []string{"auth", "login", "26"},
			wantErr: `Error: unexpected argument "26"`,
			noHint:  true,
		},
		{
			name:    "screenshots list with ambiguous identifier flags",
			args:    []string{"screenshots", "list", "23"},
			wantErr: `Error: unexpected argument "23"`,
			noHint:  true,
		},
		{
			name:    "several stray operands",
			args:    []string{"apps", "view", "123", "456"},
			wantErr: `Error: unexpected arguments "123", "456"`,
			noHint:  true,
		},
		{
			name:    "operand behind the argument terminator",
			args:    []string{"apps", "view", "--id", "app-1", "--", "stray"},
			wantErr: `Error: unexpected argument "stray"`,
			noHint:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Cleanup(func() { shared.SetSelectedProfile("") })
			t.Setenv("ASC_APP_ID", "")

			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(test.args, "1.2.3"); code != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
				}
			})
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, test.wantErr)
			}
			if test.noHint {
				if strings.Contains(stderr, "Did you mean:") {
					t.Fatalf("stderr = %q, want no did-you-mean hint", stderr)
				}
				return
			}
			if !strings.Contains(stderr, test.wantHint) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, test.wantHint)
			}
		})
	}
}

// TestCommandsThatTakeOperandsKeepAcceptingThem pins the exclusions: the
// handful of commands whose positional operands are part of their published
// contract must not reach the stray-operand diagnostic.
func TestCommandsThatTakeOperandsKeepAcceptingThem(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "search", args: []string{"search", "external testers"}},
		{name: "schema", args: []string{"schema", "apps.list"}},
		{name: "docs show", args: []string{"docs", "show", "api-notes"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(test.args, "1.2.3"); code != 0 {
					t.Fatalf("exit code = %d, want 0 (stderr follows)", code)
				}
			})
			if strings.Contains(stderr, "unexpected argument") {
				t.Fatalf("stderr = %q, want no stray-operand diagnostic", stderr)
			}
		})
	}
}

// TestOperandCommandsFailForTheirOwnReasons covers the exclusions whose
// operands cannot be exercised offline: they must still fail on their own
// validation rather than on the stray-operand diagnostic.
func TestOperandCommandsFailForTheirOwnReasons(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "workflow run", args: []string{"workflow", "run", "release", "--file", "missing-workflow.json"}},
		{
			name: "signing run passthrough",
			args: []string{"signing", "run", "--identity", "missing.p12", "--profile", "missing.mobileprovision", "true"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, stderr := captureOutput(t, func() {
				_ = rootcmd.Run(test.args, "1.2.3")
			})
			if strings.Contains(stderr, "unexpected argument") {
				t.Fatalf("stderr = %q, want no stray-operand diagnostic", stderr)
			}
		})
	}
}
