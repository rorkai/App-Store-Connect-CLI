package cmdtest

import (
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// TestRemovedFlagAliasesNameTheirReplacement locks the 5.3.2 unknown-flag
// hint: a flag that was a compatibility alias in 4.x and was removed in 5.0.0
// still fails as a usage error (exit 2, nothing on stdout, no HTTP), but the
// diagnostic says it was removed and names the replacement instead of the
// generic edit-distance suggestion.
func TestRemovedFlagAliasesNameTheirReplacement(t *testing.T) {
	setupUsageExitCodeEnv(t)
	asctest.NewRegistry(t).Install()

	tests := []struct {
		name string
		args []string
		want string
		deps bool // the command is built on shared.Deps: run it on test deps
	}{
		{
			name: "single-dash spelling",
			args: []string{"web", "agreements", "status", "-two-factor-code", "PRIVATE_VALUE"},
			want: "Error: `--two-factor-code` was removed in 5.0.0; use `--two-factor-code-command` or `ASC_WEB_2FA_CODE_COMMAND` (see migrate-to-5-0)\nFor help:\n  asc web agreements status --help\n",
			deps: true,
		},
		{
			name: "pre-orders enable --available-in-new-territories has no replacement",
			args: []string{"pre-orders", "enable", "--app", "app-1", "--territory", "USA", "--release-date", "2026-06-01", "--available-in-new-territories", "true"},
			want: "Error: `--available-in-new-territories` was removed in 5.0.0; pre-orders are enabled by patching territory availabilities (see migrate-to-5-0)\nFor help:\n  asc pre-orders enable --help\n",
			deps: true,
		},
		{
			name: "web --two-factor-code matches a historical web command",
			args: []string{"web", "agreements", "status", "--two-factor-code", "PRIVATE_VALUE"},
			want: "Error: `--two-factor-code` was removed in 5.0.0; use `--two-factor-code-command` or `ASC_WEB_2FA_CODE_COMMAND` (see migrate-to-5-0)\nFor help:\n  asc web agreements status --help\n",
			deps: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var code int
			var stdout, stderr string
			if test.deps {
				stdout, stderr, code = runWithDeps(t, nil, test.args...)
			} else {
				stdout, stderr = captureOutput(t, func() {
					code = rootcmd.Run(test.args, "5.3.2")
				})
			}
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, rootcmd.ExitUsage, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if stderr != test.want {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
			if strings.Contains(stderr, "PRIVATE_VALUE") {
				t.Fatalf("stderr leaked the removed flag value: %q", stderr)
			}
		})
	}
}

// TestRemovedFlagHintIsScopedToTheExactCommand keeps the hint from firing on
// commands where the same spelling was never an alias: `--id` is canonical on
// `asc apps view`, and an unrelated command with a typo keeps the generic
// unknown-flag suggestion.
func TestRemovedFlagHintIsScopedToTheExactCommand(t *testing.T) {
	setupUsageExitCodeEnv(t)

	stdout, stderr := captureOutput(t, func() {
		if code := rootcmd.Run([]string{"versions", "attach-build", "--version-id", "version-1", "--buid-id", "build-1"}, "5.3.2"); code != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "Error: unknown flag `--buid-id` for `asc versions attach-build`") {
		t.Fatalf("stderr = %q, want generic unknown-flag diagnostic", stderr)
	}
	if strings.Contains(stderr, "was removed in 5.0.0") {
		t.Fatalf("stderr = %q, must not describe a typo as a removed alias", stderr)
	}
}
