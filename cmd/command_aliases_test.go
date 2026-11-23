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
		{name: "group with neither view nor info", args: "review submissions frobnicate --id S", want: "review submissions frobnicate --id S"},
		{name: "real subcommand", args: "versions list --app 1", want: "versions list --app 1"},
		{name: "a leaf's positional is not a command", args: "api GET /v1/apps/get", want: "api GET /v1/apps/get"},
		{name: "after an unknown flag", args: "versions --bogus get", want: "versions --bogus get"},
		{name: "after the terminator", args: "versions -- get", want: "versions -- get"},
		{name: "API resource name at the root", args: "app-infos list --app 1", want: "apps info list --app 1"},
		{name: "path alias then read verb", args: "app-info get --app 1", want: "apps info view --app 1"},
		{name: "nested path alias moves across groups", args: "apps versions list --app 1", want: "versions list --app 1"},
		{name: "path alias after a root flag", args: "--profile p testflight builds list --app 1", want: "--profile p builds list --app 1"},
		{name: "path alias within its group", args: "testflight beta-groups list --app 1", want: "testflight groups list --app 1"},
		{name: "path alias to a group's own child", args: "versions localizations list --version-id V", want: "localizations list --version-id V"},
		{name: "removed v1 group to its v2 replacement", args: "subscriptions localizations list --subscription-id S", want: "subscriptions versions localizations list --subscription-id S"},
		{name: "split hyphenated verb", args: "review details create --version-id V", want: "review details-create --version-id V"},
		{name: "split hyphenated read verb", args: "review details view --version-id V", want: "review details-get --version-id V"},
		{name: "split hyphenated list verb", args: "review attachments ls --review-detail X", want: "review attachments-list --review-detail X"},
		{name: "verb after a real subgroup", args: "review submissions get --id S", want: "review submissions-get --id S"},
		{name: "verb after a real subgroup that has it", args: "review submissions list --app A", want: "review submissions list --app A"},
		{name: "verb split before its noun", args: "builds beta-groups add --build-id B --group G", want: "builds add-groups --build-id B --group G"},
		{name: "group given its view's flags", args: "apps info --app A --output json", want: "apps info view --app A --output json"},
		{name: "relationships group given its view's flags", args: "apps info relationships --app A --output json", want: "apps info relationships view --app A --output json"},
		{name: "app-setup availability given its view's flags", args: "app-setup availability --app A", want: "app-setup availability view --app A"},
		{name: "collection group given its list's flags", args: "versions --app A --output json", want: "versions list --app A --output json"},
		{name: "group without a curated leaf", args: "subscriptions --app A", want: "subscriptions --app A"},
		{name: "group given a flag view lacks", args: "apps info --app A --bogus X", want: "apps info --app A --bogus X"},
		{name: "group given flags and a positional", args: "apps info --app A extra", want: "apps info --app A extra"},
		{name: "no hyphenated leaf", args: "review details frobnicate", want: "review details frobnicate"},
		{name: "ls on a group with list", args: "builds ls --app A", want: "builds list --app A"},
		{name: "API resource name in camel case", args: "builds betaGroups list --build-id B", want: "builds groups list --build-id B"},
		{name: "group-scoped add to the build-scoped verb", args: "testflight groups builds add --group-id G --build-id B", want: "builds add-groups --group-id G --build-id B"},
		{name: "short name of a nested offer group", args: "subscriptions intro-offers list --subscription-id S", want: "subscriptions offers introductory list --subscription-id S"},
		{name: "long name of a nested offer group", args: "subscriptions introductory-offers list --subscription-id S", want: "subscriptions offers introductory list --subscription-id S"},
		{name: "subscription prices one level up", args: "subscriptions prices list --subscription-id S", want: "subscriptions pricing prices list --subscription-id S"},
		{name: "subscription availability one level up", args: "subscriptions availability view --subscription-id S", want: "subscriptions pricing availability view --subscription-id S"},
		{name: "app availability under apps", args: "apps availability view --app A", want: "pricing availability view --app A"},
		{name: "API resource name for build uploads", args: "build-uploads list --app A", want: "builds uploads list --app A"},
		{name: "group builds given its list's flags", args: "testflight groups builds --group G", want: "testflight groups builds list --group G"},
		{name: "help names the command", args: "help builds list", want: "builds list --help"},
		{name: "bare help", args: "help", want: "--help"},
		{name: "help after a terminator stays a positional", args: "help builds -- x", want: "builds --help -- x"},
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

// TestRunResolvesGuessedCommands runs guessed commands end to end: each must
// reach the canonical command, including one in another top-level command,
// which the lazily built root has to materialize after the rewrite.
func TestRunResolvesGuessedCommands(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantCode  int
		wantFirst string
	}{
		{name: "get runs view", args: []string{"builds", "build-beta-detail", "get"}, wantCode: ExitUsage, wantFirst: "Error: --build-id or --app is required (or set ASC_APP_ID)"},
		{name: "path alias into another top-level command", args: []string{"testflight", "builds", "list"}, wantCode: ExitUsage, wantFirst: "Error: --app is required (or set ASC_APP_ID)"},
		{name: "path alias then get", args: []string{"app-info", "get"}, wantCode: ExitUsage, wantFirst: "Error: --app or --info-id is required (or set ASC_APP_ID)"},
		{name: "alias after a spaced root boolean", args: []string{"--debug", "false", "apps", "get"}, wantCode: ExitUsage, wantFirst: "Error: --id is required"},
		{name: "spaced boolean after an alias", args: []string{"builds", "build-beta-detail", "get", "--pretty", "false"}, wantCode: ExitUsage, wantFirst: "Error: --build-id or --app is required (or set ASC_APP_ID)"},
		{name: "group flags with a spaced boolean", args: []string{"builds", "--paginate", "true"}, wantCode: ExitUsage, wantFirst: "Error: --app is required (or set ASC_APP_ID)"},
		{name: "help names a leaf", args: []string{"help", "builds", "list"}, wantCode: ExitSuccess, wantFirst: "DESCRIPTION"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetReportFlags(t)
			t.Setenv("ASC_APP_ID", "")
			var code int
			stdout, stderr := captureCommandOutput(t, func() {
				code = Run(test.args, "1.0.0")
			})
			if code != test.wantCode {
				t.Fatalf("Run() exit code = %d, want %d; stderr %q", code, test.wantCode, stderr)
			}
			output := stderr
			if test.wantCode == ExitSuccess {
				output = stdout
				if !strings.Contains(stdout, "asc builds list [flags]") {
					t.Fatalf("stdout = %q, want builds list help", stdout)
				}
			}
			if first, _, _ := strings.Cut(output, "\n"); first != test.wantFirst {
				t.Fatalf("first line = %q, want %q", first, test.wantFirst)
			}
		})
	}
}

// TestCommandPathAliasesResolveToRealCommands guards the path-alias table: each
// key sits on a real group without shadowing one of its subcommands, and each
// target is a real command.
func TestCommandPathAliasesResolveToRealCommands(t *testing.T) {
	root := RootCommand("1.0.0")
	for group, aliases := range commandPathAliases {
		groupCommand := resolveCommandPath(root, strings.Fields(strings.TrimPrefix(group, "asc")))
		if groupCommand == nil {
			t.Fatalf("path alias group %q does not resolve", group)
		}
		for token, target := range aliases {
			if findDirectSubcommand(groupCommand, token) != nil {
				t.Fatalf("%s %s shadows a real subcommand", group, token)
			}
			if resolveCommandPath(root, target) == nil {
				t.Fatalf("%s %s targets %q, which does not resolve", group, token, target)
			}
		}
	}
}
