package cmdtest

import (
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// The unknown-child suggester is exercised here through the public Run entry
// point for the command groups telemetry ranks highest, so a registry change
// that renames a suggested target fails at the CLI surface, not only in the
// table-walking unit test.
func TestUnknownChildSuggestionsAcrossCommandGroups(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name: "versions",
			args: []string{"versions", "current"},
			wantStderr: "Error: unknown command `asc versions current`\n" +
				"Try:\n" +
				"  asc versions list --app APP_ID --latest\n" +
				"  asc versions view --version-id VERSION_ID\n" +
				"For help:\n" +
				"  asc versions --help\n",
		},
		{
			name: "builds",
			args: []string{"builds", "status"},
			wantStderr: "Error: unknown command `asc builds status`\n" +
				"Try:\n" +
				"  asc builds info --app APP_ID --latest\n" +
				"  asc builds wait --app APP_ID --latest\n" +
				"For help:\n" +
				"  asc builds --help\n",
		},
		{
			name: "subscriptions",
			args: []string{"subscriptions", "show"},
			wantStderr: "Error: unknown command `asc subscriptions show`\n" +
				"Try:\n" +
				"  asc subscriptions view --id SUBSCRIPTION_ID\n" +
				"  asc subscriptions list --app APP_ID\n" +
				"For help:\n" +
				"  asc subscriptions --help\n",
		},
		{
			name: "testflight groups",
			args: []string{"testflight", "groups", "create-group"},
			wantStderr: "Error: unknown command `asc testflight groups create-group`\n" +
				"Try:\n" +
				"  asc testflight groups create --app APP_ID --name NAME\n" +
				"For help:\n" +
				"  asc testflight groups --help\n",
		},
		{
			name: "iap",
			args: []string{"iap", "show"},
			wantStderr: "Error: unknown command `asc iap show`\n" +
				"Try:\n" +
				"  asc iap view --id IAP_ID\n" +
				"  asc iap list --app APP_ID\n" +
				"For help:\n" +
				"  asc iap --help\n",
		},
		{
			name: "metadata",
			args: []string{"metadata", "get"},
			wantStderr: "Error: unknown command `asc metadata get`\n" +
				"Try:\n" +
				"  asc metadata pull --app APP_ID --version VERSION --dir ./metadata\n" +
				"  asc apps info view --app APP_ID\n" +
				"For help:\n" +
				"  asc metadata --help\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(test.args, "1.2.3"); code != rootcmd.ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, rootcmd.ExitUsage)
				}
			})

			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if stderr != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
		})
	}
}

func TestUnknownChildSuggesterLeavesGroupHelpUnchanged(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	for _, group := range [][]string{{"versions"}, {"apps"}, {"testflight", "groups"}} {
		t.Run(strings.Join(group, " "), func(t *testing.T) {
			args := append(append([]string{}, group...), "--help")
			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitSuccess {
					t.Fatalf("Run() exit code = %d, want %d", code, rootcmd.ExitSuccess)
				}
			})

			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if !strings.Contains(stdout, "SUBCOMMANDS") {
				t.Fatalf("stdout does not render the group help: %q", stdout)
			}
			for _, banned := range []string{"Try:", "Common tasks:", "For help:", "unknown command"} {
				if strings.Contains(stdout, banned) {
					t.Fatalf("group help leaks the recovery block %q: %q", banned, stdout)
				}
			}
		})
	}
}
