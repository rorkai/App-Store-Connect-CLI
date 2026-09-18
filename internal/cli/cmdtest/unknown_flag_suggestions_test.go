package cmdtest

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// TestUnknownFlagSuggestsTheIntendedFlag covers the unknown flag spellings
// telemetry reports most often. Each case asserts the complete stderr, so a
// suggestion cannot regress into a usage dump or a wrong flag, and the
// invocation still fails as a usage error before any HTTP request.
func TestUnknownFlagSuggestsTheIntendedFlag(t *testing.T) {
	setupUsageExitCodeEnv(t)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unknown flags must fail before HTTP: %s %s", req.Method, req.URL.String())
		return nil, errors.New("unexpected request")
	}))

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "submit status --app",
			args: []string{"submit", "status", "--app", "PRIVATE_VALUE"},
			want: "Error: unknown flag `--app` for `asc submit status`\n" +
				"Try:\n  --id\n" +
				"For help:\n  asc submit status --help\n",
		},
		{
			name: "subscriptions list --group",
			args: []string{"subscriptions", "list", "--group", "PRIVATE_VALUE"},
			want: "Error: unknown flag `--group` for `asc subscriptions list`\n" +
				"Try:\n  --group-id\n" +
				"For help:\n  asc subscriptions list --help\n",
		},
		{
			name: "builds groups list --app",
			args: []string{"builds", "groups", "list", "--app", "PRIVATE_VALUE"},
			want: "Error: unknown flag `--app` for `asc builds groups list`\n" +
				"Try:\n  --build-id\n" +
				"For help:\n  asc builds groups list --help\n",
		},
		{
			name: "bundle-ids capabilities list --id",
			args: []string{"bundle-ids", "capabilities", "list", "--id", "PRIVATE_VALUE"},
			want: "Error: unknown flag `--id` for `asc bundle-ids capabilities list`\n" +
				"Try:\n  --bundle\n" +
				"For help:\n  asc bundle-ids capabilities list --help\n",
		},
		{
			name: "pricing availability territory-availabilities --app",
			args: []string{"pricing", "availability", "territory-availabilities", "--app", "PRIVATE_VALUE"},
			want: "Error: unknown flag `--app` for `asc pricing availability territory-availabilities`\n" +
				"Try:\n  --availability\n" +
				"For help:\n  asc pricing availability territory-availabilities --help\n",
		},
		{
			name: "builds upload --file",
			args: []string{"builds", "upload", "--app", "app-1", "--file", "PRIVATE_VALUE"},
			want: "Error: unknown flag `--file` for `asc builds upload`\n" +
				"Try:\n  --ipa\n  --pkg\n" +
				"For help:\n  asc builds upload --help\n",
		},
		{
			name: "review details-get --version-id",
			args: []string{"review", "details-get", "--version-id", "PRIVATE_VALUE"},
			want: "Error: unknown flag `--version-id` for `asc review details-get`\n" +
				"Try:\n  --id\n" +
				"For help:\n  asc review details-get --help\n",
		},
		{
			name: "subscriptions versions view --version-id",
			args: []string{"subscriptions", "versions", "view", "--version-id", "PRIVATE_VALUE"},
			want: "Error: unknown flag `--version-id` for `asc subscriptions versions view`\n" +
				"Try:\n  --id\n" +
				"For help:\n  asc subscriptions versions view --help\n",
		},
		{
			name: "profiles list --bundle-id",
			args: []string{"profiles", "list", "--bundle-id", "PRIVATE_VALUE"},
			want: "Error: unknown flag `--bundle-id` for `asc profiles list`\n" +
				"Try:\n  --bundle-id-fields\n  --id\n" +
				"For help:\n  asc profiles list --help\n",
		},
		{
			name: "single-dash spelling",
			args: []string{"submit", "status", "-app", "PRIVATE_VALUE"},
			want: "Error: unknown flag `-app` for `asc submit status`\n" +
				"Try:\n  --id\n" +
				"For help:\n  asc submit status --help\n",
		},
		{
			name: "inline value is never echoed",
			args: []string{"subscriptions", "list", "--group=PRIVATE_VALUE"},
			want: "Error: unknown flag `--group` for `asc subscriptions list`\n" +
				"Try:\n  --group-id\n" +
				"For help:\n  asc subscriptions list --help\n",
		},
		{
			name: "a path-valued --output answers --path",
			args: []string{"testflight", "testers", "export", "--app", "app-1", "--path", "PRIVATE_VALUE"},
			want: "Error: unknown flag `--path` for `asc testflight testers export`\n" +
				"Try:\n  --output\n" +
				"For help:\n  asc testflight testers export --help\n",
		},
		{
			name: "signing resign --path names both path flags",
			args: []string{"signing", "resign", "--ipa", "in.ipa", "--path", "PRIVATE_VALUE"},
			want: "Error: unknown flag `--path` for `asc signing resign`\n" +
				"Try:\n  --ipa\n  --output\n" +
				"For help:\n  asc signing resign --help\n",
		},
		{
			name: "a format-only --output does not answer --path",
			args: []string{"apps", "list", "--path", "PRIVATE_VALUE"},
			want: "Error: unknown flag `--path` for `asc apps list`\n" +
				"For help:\n  asc apps list --help\n",
		},
		{
			name: "no comparable flag leaves the Try block out",
			args: []string{"categories", "list", "--app", "PRIVATE_VALUE"},
			want: "Error: unknown flag `--app` for `asc categories list`\n" +
				"For help:\n  asc categories list --help\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(test.args, "5.6.0")
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
				t.Fatalf("stderr leaked the flag value: %q", stderr)
			}
		})
	}
}

// TestUnknownFlagSuggestionsLeaveHelpUnchanged keeps the suggestion block on
// the failure path only: explicit help still renders the command's own flags on
// stdout with a success exit code and no recovery block.
func TestUnknownFlagSuggestionsLeaveHelpUnchanged(t *testing.T) {
	setupUsageExitCodeEnv(t)

	for _, command := range [][]string{
		{"submit", "status"},
		{"builds", "groups", "list"},
		{"categories", "list"},
	} {
		t.Run(strings.Join(command, " "), func(t *testing.T) {
			args := append(append([]string{}, command...), "--help")
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(args, "5.6.0")
			})
			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if !strings.Contains(stdout, "--output") {
				t.Fatalf("stdout = %q, want the command's flag list", stdout)
			}
			if strings.Contains(stdout, "Try:") || strings.Contains(stdout, "unknown flag") {
				t.Fatalf("help output gained a recovery block: %q", stdout)
			}
		})
	}
}
