package cmdtest

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// TestRemovedFlagAliasesNameTheirReplacement locks the 5.3.2 unknown-flag
// hint: a flag that was a compatibility alias in 4.x and was removed in 5.0.0
// still fails as a usage error (exit 2, nothing on stdout, no HTTP), but the
// diagnostic says it was removed and names the replacement instead of the
// generic edit-distance suggestion.
func TestRemovedFlagAliasesNameTheirReplacement(t *testing.T) {
	setupUsageExitCodeEnv(t)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("removed aliases must fail before HTTP: %s %s", req.Method, req.URL.String())
		return nil, errors.New("unexpected request")
	}))

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "localizations list --version-id",
			args: []string{"localizations", "list", "--version-id", "PRIVATE_VALUE"},
			want: "Error: `--version-id` was removed in 5.0.0; use `--version` (see migrate-to-5-0)\nFor help:\n  asc localizations list --help\n",
		},
		{
			name: "versions view --id",
			args: []string{"versions", "view", "--id=PRIVATE_VALUE"},
			want: "Error: `--id` was removed in 5.0.0; use `--version-id` (see migrate-to-5-0)\nFor help:\n  asc versions view --help\n",
		},
		{
			name: "apps view --app",
			args: []string{"apps", "view", "--app", "PRIVATE_VALUE"},
			want: "Error: `--app` was removed in 5.0.0; use `--id` (see migrate-to-5-0)\nFor help:\n  asc apps view --help\n",
		},
		{
			name: "review submit --build",
			args: []string{"review", "submit", "--app", "app-1", "--version-id", "version-1", "--build", "PRIVATE_VALUE"},
			want: "Error: `--build` was removed in 5.0.0; use `--build-id` or `--build-number` (see migrate-to-5-0)\nFor help:\n  asc review submit --help\n",
		},
		{
			name: "testflight groups view --group-id",
			args: []string{"testflight", "groups", "view", "--group-id", "PRIVATE_VALUE"},
			want: "Error: `--group-id` was removed in 5.0.0; use `--id` (see migrate-to-5-0)\nFor help:\n  asc testflight groups view --help\n",
		},
		{
			name: "subscriptions view --subscription-id",
			args: []string{"subscriptions", "view", "--subscription-id", "PRIVATE_VALUE"},
			want: "Error: `--subscription-id` was removed in 5.0.0; use `--id` (see migrate-to-5-0)\nFor help:\n  asc subscriptions view --help\n",
		},
		{
			name: "builds info --build",
			args: []string{"builds", "info", "--build", "PRIVATE_VALUE"},
			want: "Error: `--build` was removed in 5.0.0; use `--build-id` or `--build-number` (see migrate-to-5-0)\nFor help:\n  asc builds info --help\n",
		},
		{
			name: "release stage --build",
			args: []string{"release", "stage", "--app", "app-1", "--build", "PRIVATE_VALUE"},
			want: "Error: `--build` was removed in 5.0.0; use `--build-id` (see migrate-to-5-0)\nFor help:\n  asc release stage --help\n",
		},
		{
			name: "single-dash spelling",
			args: []string{"builds", "info", "-build", "PRIVATE_VALUE"},
			want: "Error: `--build` was removed in 5.0.0; use `--build-id` or `--build-number` (see migrate-to-5-0)\nFor help:\n  asc builds info --help\n",
		},
		{
			name: "pre-orders enable --available-in-new-territories has no replacement",
			args: []string{"pre-orders", "enable", "--app", "app-1", "--territory", "USA", "--release-date", "2026-06-01", "--available-in-new-territories", "true"},
			want: "Error: `--available-in-new-territories` was removed in 5.0.0; pre-orders are enabled by patching territory availabilities (see migrate-to-5-0)\nFor help:\n  asc pre-orders enable --help\n",
		},
		{
			name: "web --two-factor-code matches a historical web command",
			args: []string{"web", "agreements", "status", "--two-factor-code", "PRIVATE_VALUE"},
			want: "Error: `--two-factor-code` was removed in 5.0.0; use `--two-factor-code-command` or `ASC_WEB_2FA_CODE_COMMAND` (see migrate-to-5-0)\nFor help:\n  asc web agreements status --help\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(test.args, "5.3.2")
			})
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
