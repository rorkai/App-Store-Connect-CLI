package cmd

import (
	"flag"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// Rendered task maps shared with the older unknown-command tests so a table
// edit fails in one place instead of drifting silently across files.
const (
	buildsTaskHintBlock = "Common tasks:\n" +
		"  list builds          asc builds list --app APP_ID\n" +
		"  latest build         asc builds info --app APP_ID --latest\n" +
		"  next build number    asc builds next-build-number --app APP_ID\n" +
		"  upload a build       asc builds upload --app APP_ID --ipa IPA_PATH\n" +
		"  wait for processing  asc builds wait --app APP_ID --latest\n"

	versionsTaskHintBlock = "Common tasks:\n" +
		"  list versions      asc versions list --app APP_ID\n" +
		"  view a version     asc versions view --version-id VERSION_ID\n" +
		"  create a version   asc versions create --app APP_ID --version VERSION\n" +
		"  attach a build     asc versions attach-build --version-id VERSION_ID --build-id BUILD_ID\n" +
		"  release a version  asc versions release --version-id VERSION_ID --confirm\n"
)

func TestRun_UnknownChildAppendsCuratedTaskHints(t *testing.T) {
	resetReportFlags(t)

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name: "listed group without a near match",
			args: []string{"builds", "qqqqq"},
			wantStderr: "Error: unknown command `asc builds qqqqq`\n" +
				buildsTaskHintBlock +
				"For help:\n" +
				"  asc builds --help\n",
		},
		{
			name: "listed nested group without a near match",
			args: []string{"testflight", "groups", "qqqqq"},
			wantStderr: "Error: unknown command `asc testflight groups qqqqq`\n" +
				"Common tasks:\n" +
				"  list groups     asc testflight groups list --app APP_ID\n" +
				"  view a group    asc testflight groups view --id GROUP_ID\n" +
				"  create a group  asc testflight groups create --app APP_ID --name NAME\n" +
				"  add testers     asc testflight groups add-testers --group GROUP_ID --email EMAIL\n" +
				"For help:\n" +
				"  asc testflight groups --help\n",
		},
		{
			name: "listed group with a near match keeps the typo suggestion only",
			args: []string{"builds", "lsit"},
			wantStderr: "Error: unknown command `asc builds lsit`\n" +
				"Try:\n" +
				"  asc builds list\n" +
				"For help:\n" +
				"  asc builds --help\n",
		},
		{
			name: "unlisted group is unchanged",
			args: []string{"xcode-cloud", "workflows", "qqqqq"},
			wantStderr: "Error: unknown command `asc xcode-cloud workflows qqqqq`\n" +
				"For help:\n" +
				"  asc xcode-cloud workflows --help\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "1.0.0"); code != ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
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

func TestUnknownChildTaskHintsCoverTheDocumentedGroups(t *testing.T) {
	wantGroups := []string{
		"asc apps",
		"asc auth",
		"asc builds",
		"asc iap",
		"asc review",
		"asc subscriptions",
		"asc testflight",
		"asc testflight groups",
		"asc versions",
	}

	if len(unknownChildTaskHints) != len(wantGroups) {
		t.Fatalf("task hint groups = %d, want %d", len(unknownChildTaskHints), len(wantGroups))
	}
	for _, group := range wantGroups {
		hints, ok := unknownChildTaskHints[group]
		if !ok {
			t.Fatalf("missing task hints for %q", group)
		}
		if len(hints) < 3 || len(hints) > 5 {
			t.Fatalf("%q task hints = %d, want between 3 and 5", group, len(hints))
		}
		for _, hint := range hints {
			if strings.TrimSpace(hint.task) == "" || strings.TrimSpace(hint.command) == "" {
				t.Fatalf("%q has an empty task hint: %+v", group, hint)
			}
		}
	}
}

// Every curated invocation must stay copy-paste valid: it has to resolve to a
// real leaf command under its group and use only long-form flags that command
// actually defines.
func TestUnknownChildTaskHintsResolveToRealCommands(t *testing.T) {
	root := RootCommand("1.0.0")

	for group, hints := range unknownChildTaskHints {
		groupPath := strings.Fields(group)
		if len(groupPath) < 2 || groupPath[0] != "asc" {
			t.Fatalf("task hint key %q must be a full `asc ...` group path", group)
		}
		if resolveCommandPath(root, groupPath[1:]) == nil {
			t.Fatalf("task hint group %q does not resolve to a command", group)
		}

		for _, hint := range hints {
			if !strings.HasPrefix(hint.command+" ", group+" ") {
				t.Fatalf("%q hint %q must stay inside its group", group, hint.command)
			}
			assertHintInvocationResolves(t, root, group, hint.command)
		}
	}
}

// assertHintInvocationResolves fails unless invocation is a copy-paste valid
// `asc ...` call: it resolves to a real leaf command, every token is shell-safe,
// and every flag is a long-form flag that command defines. label names the
// table entry in failures.
func assertHintInvocationResolves(t *testing.T, root *ffcli.Command, label, invocation string) {
	t.Helper()

	tokens := strings.Fields(invocation)
	if len(tokens) == 0 || tokens[0] != "asc" {
		t.Fatalf("%q hint %q must start with `asc`", label, invocation)
	}
	commandPath := []string{}
	flagTokens := []string{}
	for _, token := range tokens[1:] {
		if strings.HasPrefix(token, "-") {
			flagTokens = append(flagTokens, token)
			continue
		}
		if len(flagTokens) == 0 {
			commandPath = append(commandPath, token)
		}
	}

	command := resolveCommandPath(root, commandPath)
	if command == nil {
		t.Fatalf("%q hint %q does not resolve to a command", label, invocation)
	}
	if len(command.Subcommands) > 0 {
		t.Fatalf("%q hint %q resolves to a group, not a runnable command", label, invocation)
	}
	for _, token := range tokens {
		if !isShellSafeHintToken(token) {
			t.Fatalf("%q hint %q has shell-unsafe token %q", label, invocation, token)
		}
	}
	for _, token := range flagTokens {
		if !strings.HasPrefix(token, "--") {
			t.Fatalf("%q hint %q uses the short flag %q", label, invocation, token)
		}
		name, _, _ := strings.Cut(strings.TrimPrefix(token, "--"), "=")
		if lookupTaskHintFlag(command.FlagSet, name) == nil {
			t.Fatalf("%q hint %q uses undefined flag --%s", label, invocation, name)
		}
	}
}

// isShellSafeHintToken mirrors the unquoted-safe character set that
// shellSafeCommandArg uses for the nearest-match suggester. A hint that needs
// shell quoting is not copy-pasteable, and angle-bracket placeholders in
// particular would be read as redirections.
func isShellSafeHintToken(token string) bool {
	if token == "" {
		return false
	}
	return strings.IndexFunc(token, func(r rune) bool {
		isASCIILetterOrDigit := (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9')
		return !isASCIILetterOrDigit && !strings.ContainsRune("_@%+=:,./-", r)
	}) == -1
}

func lookupTaskHintFlag(flagSet *flag.FlagSet, name string) *flag.Flag {
	if flagSet == nil {
		return nil
	}
	return flagSet.Lookup(name)
}
