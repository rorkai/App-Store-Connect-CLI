package cmd

import (
	"strings"
	"testing"
)

// TestResolveCommandAliasesRewritesOnlyUnknownReadVerbs pins where a guessed
// read verb resolves: `get` to the group's `view`, and `get` or `view` to a
// group's `info` leaf when it has no `view`. Everything a real subcommand or a
// leaf's own arguments already claim is left as written.
func TestResolveCommandAliasesRewritesOnlyUnknownReadVerbs(t *testing.T) {
	tests := []struct {
		name string
		args string
		want string
	}{
		{name: "get on a group with view", args: "versions get --version-id V", want: "versions view --version-id V"},
		{name: "nested group", args: "builds build-beta-detail get --build-id B", want: "builds build-beta-detail view --build-id B"},
		{name: "root flag before the command", args: "--profile p apps get --id 1", want: "--profile p apps view --id 1"},
		{name: "get on a group whose read leaf is info", args: "builds get --build-id B", want: "builds info --build-id B"},
		{name: "view on a group whose read leaf is info", args: "builds view --build-id B", want: "builds info --build-id B"},
		{name: "info that is a group is not a read leaf", args: "app-setup get --app 1", want: "app-setup get --app 1"},
		{name: "group with neither view nor info", args: "review submissions get --id S", want: "review submissions get --id S"},
		{name: "real subcommand", args: "versions list --app 1", want: "versions list --app 1"},
		{name: "a leaf's positional is not a command", args: "api GET /v1/apps/get", want: "api GET /v1/apps/get"},
		{name: "after an unknown flag", args: "versions --bogus get", want: "versions --bogus get"},
		{name: "after the terminator", args: "versions -- get", want: "versions -- get"},
	}
	root := RootCommand("1.0.0")
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(resolveCommandAliases(root, strings.Fields(test.args)), " ")
			if got != test.want {
				t.Fatalf("resolveCommandAliases(%q) = %q, want %q", test.args, got, test.want)
			}
		})
	}
}

// TestRunResolvesGetToTheViewCommand runs a guessed `get` end to end: it must
// reach the `view` command's own validation, not the unknown-command error.
func TestRunResolvesGetToTheViewCommand(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("ASC_APP_ID", "")
	var code int
	_, stderr := captureCommandOutput(t, func() {
		code = Run([]string{"builds", "build-beta-detail", "get"}, "1.0.0")
	})
	if code != ExitUsage {
		t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
	}
	if first, _, _ := strings.Cut(stderr, "\n"); first != "Error: --build-id or --app is required (or set ASC_APP_ID)" {
		t.Fatalf("stderr first line = %q, want the view command's validation error", first)
	}
}
