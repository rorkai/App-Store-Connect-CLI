package cmd

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

func TestRun_UnknownChildOffersSynonymSuggestions(t *testing.T) {
	resetReportFlags(t)

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name: "versions latest maps to the canonical list and view forms",
			args: []string{"versions", "latest"},
			wantStderr: "Error: unknown command `asc versions latest`\n" +
				"Try:\n" +
				"  asc versions list --app APP_ID --latest\n" +
				"  asc versions view --version-id VERSION_ID\n" +
				"For help:\n" +
				"  asc versions --help\n",
		},
		{
			name: "apps get no longer dumps the group usage",
			args: []string{"apps", "get"},
			wantStderr: "Error: unknown command `asc apps get`\n" +
				"Try:\n" +
				"  asc apps view --id APP_ID\n" +
				"  asc apps info view --app APP_ID\n" +
				"For help:\n" +
				"  asc apps --help\n",
		},
		{
			name: "age-rating set no longer dumps the group usage",
			args: []string{"age-rating", "set"},
			wantStderr: "Error: unknown command `asc age-rating set`\n" +
				"Try:\n" +
				"  asc age-rating edit --app APP_ID --kids-age-band KIDS_AGE_BAND\n" +
				"  asc age-rating view --app APP_ID\n" +
				"For help:\n" +
				"  asc age-rating --help\n",
		},
		{
			name: "auth whoami",
			args: []string{"auth", "whoami"},
			wantStderr: "Error: unknown command `asc auth whoami`\n" +
				"Try:\n" +
				"  asc auth status\n" +
				"  asc auth doctor\n" +
				"For help:\n" +
				"  asc auth --help\n",
		},
		{
			name: "agreements accept points at the web session command",
			args: []string{"agreements", "accept"},
			wantStderr: "Error: unknown command `asc agreements accept`\n" +
				"Try:\n" +
				"  asc web agreements accept --agreement-id AGREEMENT_ID --confirm\n" +
				"  asc web agreements status\n" +
				"For help:\n" +
				"  asc agreements --help\n",
		},
		{
			name: "synonyms rank before near matches and the merged list caps at three",
			args: []string{"apps", "search"},
			wantStderr: "Error: unknown command `asc apps search`\n" +
				"Try:\n" +
				"  asc apps list --name NAME\n" +
				"  asc apps list --bundle-id BUNDLE_ID\n" +
				"  asc apps search-keywords\n" +
				"For help:\n" +
				"  asc apps --help\n",
		},
		{
			name: "a near match already covered by a synonym is not repeated",
			args: []string{"review", "submit-for-review"},
			wantStderr: "Error: unknown command `asc review submit-for-review`\n" +
				"Try:\n" +
				"  asc review submit --app APP_ID --version VERSION --build-id BUILD_ID --confirm\n" +
				"For help:\n" +
				"  asc review --help\n",
		},
		{
			name: "synonym lookup ignores case and surrounding whitespace",
			args: []string{"versions", " Latest "},
			wantStderr: "Error: unknown command `asc versions  Latest `\n" +
				"Try:\n" +
				"  asc versions list --app APP_ID --latest\n" +
				"  asc versions view --version-id VERSION_ID\n" +
				"For help:\n" +
				"  asc versions --help\n",
		},
		{
			name: "substring matches recover a partially remembered name",
			args: []string{"versions", "phased"},
			wantStderr: "Error: unknown command `asc versions phased`\n" +
				"Try:\n" +
				"  asc versions phased-release\n" +
				"For help:\n" +
				"  asc versions --help\n",
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

func TestUnknownChildSynonymsCoverTheTelemetryGroups(t *testing.T) {
	wantGroups := []string{
		"asc age-rating",
		"asc agreements",
		"asc apps",
		"asc auth",
		"asc builds",
		"asc iap",
		"asc localizations",
		"asc metadata",
		"asc review",
		"asc screenshots",
		"asc subscriptions",
		"asc subscriptions groups",
		"asc subscriptions review screenshots",
		"asc testflight",
		"asc testflight groups",
		"asc versions",
	}

	if len(unknownChildSynonyms) != len(wantGroups) {
		t.Fatalf("synonym groups = %d, want %d", len(unknownChildSynonyms), len(wantGroups))
	}
	for _, group := range wantGroups {
		synonyms, ok := unknownChildSynonyms[group]
		if !ok {
			t.Fatalf("missing synonyms for %q", group)
		}
		if len(synonyms) == 0 {
			t.Fatalf("%q has no synonyms", group)
		}
		for token, invocations := range synonyms {
			if token != strings.ToLower(strings.TrimSpace(token)) || token == "" {
				t.Fatalf("%q synonym key %q must be a trimmed lowercase token", group, token)
			}
			if len(invocations) == 0 || len(invocations) > maxUnknownChildSuggestions {
				t.Fatalf("%q synonym %q has %d invocations, want 1..%d", group, token, len(invocations), maxUnknownChildSuggestions)
			}
		}
	}
}

// A synonym must never name a child the group actually has: the real command
// wins the dispatch and the synonym would be dead. Every target must also be a
// copy-paste valid invocation of a real leaf command anywhere in the tree,
// using only long-form flags that command defines.
func TestUnknownChildSynonymsResolveToRealCommands(t *testing.T) {
	root := RootCommand("1.0.0")

	for group, synonyms := range unknownChildSynonyms {
		groupPath := strings.Fields(group)
		if len(groupPath) < 2 || groupPath[0] != "asc" {
			t.Fatalf("synonym key %q must be a full `asc ...` group path", group)
		}
		groupCommand := resolveCommandPath(root, groupPath[1:])
		if groupCommand == nil {
			t.Fatalf("synonym group %q does not resolve to a command", group)
		}
		for token, invocations := range synonyms {
			if findDirectSubcommand(groupCommand, token) != nil {
				t.Fatalf("%q synonym %q shadows a real subcommand", group, token)
			}
			for _, invocation := range invocations {
				assertHintInvocationResolves(t, root, group+" "+token, invocation)
			}
		}
	}
}

func TestRun_UnknownChildTelemetryNeverCarriesThePositionalToken(t *testing.T) {
	resetReportFlags(t)
	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })

	tests := []struct {
		name        string
		args        []string
		token       string
		wantCommand string
	}{
		{name: "synonym", args: []string{"versions", "latest"}, token: "latest", wantCommand: "asc versions"},
		{name: "former legacy child", args: []string{"auth", "whoami"}, token: "whoami", wantCommand: "asc auth"},
		{name: "near match", args: []string{"builds", "lsit"}, token: "lsit", wantCommand: "asc builds"},
		{name: "no suggestion", args: []string{"builds", "qqqqq"}, token: "qqqqq", wantCommand: "asc builds"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls int
			var gotCommand string
			var gotContext telemetry.EventContext
			emitTelemetry = func(command, _ string, _ time.Duration, _ int, context telemetry.EventContext) {
				calls++
				gotCommand = command
				gotContext = context
			}

			_, _ = captureCommandOutput(t, func() {
				if code := Run(test.args, "1.0.0"); code != ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
				}
			})

			if calls != 1 || gotCommand != test.wantCommand {
				t.Fatalf("telemetry calls=%d command=%q, want 1 call for %q", calls, gotCommand, test.wantCommand)
			}
			if gotContext.InvocationShape != telemetry.InvocationShapeUnknownChild || gotContext.FailureParameter != "" {
				t.Fatalf("unexpected telemetry context: %+v", gotContext)
			}
			if rendered := fmt.Sprintf("%+v", gotContext); strings.Contains(rendered, test.token) {
				t.Fatalf("telemetry context leaks the positional token %q: %s", test.token, rendered)
			}
		})
	}
}

func TestRun_UnknownChildGenericAliasesReachUncuratedGroups(t *testing.T) {
	resetReportFlags(t)

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name: "get reaches view and list in a group the curated table never mentions",
			args: []string{"accessibility", "get"},
			wantStderr: "Error: unknown command `asc accessibility get`\n" +
				"Try:\n" +
				"  asc accessibility view\n" +
				"  asc accessibility list\n" +
				"For help:\n" +
				"  asc accessibility --help\n",
		},
		{
			name: "set falls through to update in a group without an edit child",
			args: []string{"accessibility", "set"},
			wantStderr: "Error: unknown command `asc accessibility set`\n" +
				"Try:\n" +
				"  asc accessibility update\n" +
				"For help:\n" +
				"  asc accessibility --help\n",
		},
		{
			name: "add reaches register where that is the group's spelling",
			args: []string{"devices", "add"},
			wantStderr: "Error: unknown command `asc devices add`\n" +
				"Try:\n" +
				"  asc devices register\n" +
				"For help:\n" +
				"  asc devices --help\n",
		},
		{
			name: "rm reaches revoke for certificates",
			args: []string{"certificates", "rm"},
			wantStderr: "Error: unknown command `asc certificates rm`\n" +
				"Try:\n" +
				"  asc certificates revoke\n" +
				"For help:\n" +
				"  asc certificates --help\n",
		},
		{
			name: "rm reaches delete for profiles",
			args: []string{"profiles", "rm"},
			wantStderr: "Error: unknown command `asc profiles rm`\n" +
				"Try:\n" +
				"  asc profiles delete\n" +
				"For help:\n" +
				"  asc profiles --help\n",
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

// A generic alias must stay a verb no group registers as the target it names:
// a real child wins the dispatch, and a target no command is named would make
// the alias dead weight.
func TestGenericChildAliasesStayCleanAndReachable(t *testing.T) {
	root := RootCommand("1.0.0")
	registered := map[string]struct{}{}
	var walk func(command *ffcli.Command)
	walk = func(command *ffcli.Command) {
		for _, subcommand := range command.Subcommands {
			if subcommand == nil {
				continue
			}
			registered[subcommand.Name] = struct{}{}
			walk(subcommand)
		}
	}
	walk(root)

	for alias, children := range genericChildAliases {
		if alias == "" || alias != strings.ToLower(strings.TrimSpace(alias)) {
			t.Fatalf("alias %q must be a trimmed lowercase token", alias)
		}
		if len(children) == 0 || len(children) > maxUnknownChildSuggestions {
			t.Fatalf("alias %q has %d targets, want 1..%d", alias, len(children), maxUnknownChildSuggestions)
		}
		for _, child := range children {
			if child == alias {
				t.Fatalf("alias %q names itself", alias)
			}
			if _, ok := registered[child]; !ok {
				t.Fatalf("alias %q targets %q, which no registered command is named", alias, child)
			}
		}
	}
}

// The curated table answers a token it knows on its own: a generic alias must
// not append a flagless spelling of a target the curated entry already covers.
func TestUnknownChildCuratedSynonymsOutrankGenericAliases(t *testing.T) {
	root := RootCommand("1.0.0")
	apps := resolveCommandPath(root, []string{"apps"})
	if apps == nil {
		t.Fatal("apps group does not resolve")
	}

	got := unknownChildSuggestions(apps, "asc apps", "get")
	want := unknownChildSynonyms["asc apps"]["get"]
	if !slices.Equal(got, want) {
		t.Fatalf("unknownChildSuggestions() = %v, want the curated entries %v", got, want)
	}
}

// invocationParts splits an `asc ...` invocation into its command path and the
// long-form flag names it passes. Values are ignored: a curated suggestion is
// checked for shape, not for plausible placeholders.
func invocationParts(invocation string) (path []string, flags []string) {
	for _, token := range strings.Fields(invocation) {
		if token == "asc" && len(path) == 0 && len(flags) == 0 {
			continue
		}
		if strings.HasPrefix(token, "--") {
			name, _, _ := strings.Cut(strings.TrimPrefix(token, "--"), "=")
			flags = append(flags, name)
			continue
		}
		if strings.HasPrefix(token, "-") || len(flags) > 0 {
			continue
		}
		path = append(path, token)
	}
	return path, flags
}

// documentedExamples collects the `asc <path> ...` example lines for path from
// the help text of the command itself and of its ancestors, which is where
// several groups document their leaves.
func documentedExamples(root *ffcli.Command, path []string) []string {
	examples := []string{}
	for depth := 0; depth <= len(path); depth++ {
		command := resolveCommandPath(root, path[:depth])
		if command == nil {
			continue
		}
		for _, line := range strings.Split(command.LongHelp, "\n") {
			line = strings.TrimSpace(strings.ReplaceAll(line, `"`, ""))
			if !strings.HasPrefix(line, "asc ") {
				continue
			}
			examplePath, _ := invocationParts(line)
			if slices.Equal(examplePath, path) {
				examples = append(examples, line)
			}
		}
	}
	return examples
}

// synonymsVerifiedAgainstValidation lists the curated suggestions whose
// runnability was confirmed by reading the target command's own validation
// rather than its examples, because every documented example of that command
// also passes flags it does not require. Each entry records what was checked.
var synonymsVerifiedAgainstValidation = map[string]string{
	// subscriptions.go: list rejects only --group-id together with --app, so
	// --app alone lists across groups; every --app example adds --paginate.
	"asc subscriptions list --app APP_ID": "list accepts --app on its own",
	// age_rating.go: edit requires only a selector (--id or --app), so a single
	// declaration flag is enough; every example sets two or more.
	"asc age-rating edit --app APP_ID --kids-age-band KIDS_AGE_BAND": "edit requires only a selector",
}

// A curated suggestion must carry every flag some documented example of the
// same command carries, so it can never drop a flag that command requires. The
// `--app APP_ID` form of `asc localizations list`, for one, exits 2 asking for
// `--type app-info`, and only the command's own examples record that.
func TestUnknownChildSynonymsMirrorDocumentedExamples(t *testing.T) {
	root := RootCommand("1.0.0")
	unusedExceptions := map[string]struct{}{}
	for invocation := range synonymsVerifiedAgainstValidation {
		unusedExceptions[invocation] = struct{}{}
	}

	for group, synonyms := range unknownChildSynonyms {
		for token, invocations := range synonyms {
			for _, invocation := range invocations {
				if _, ok := synonymsVerifiedAgainstValidation[invocation]; ok {
					delete(unusedExceptions, invocation)
					continue
				}
				path, flags := invocationParts(invocation)
				examples := documentedExamples(root, path)
				if len(examples) == 0 {
					t.Errorf("%q synonym %q suggests %q, which no help text documents", group, token, invocation)
					continue
				}
				matched := false
				for _, example := range examples {
					_, exampleFlags := invocationParts(example)
					covered := true
					for _, flag := range exampleFlags {
						if !slices.Contains(flags, flag) {
							covered = false
							break
						}
					}
					if covered {
						matched = true
						break
					}
				}
				if !matched {
					t.Errorf("%q synonym %q suggests %q, which carries fewer flags than every documented example: %v", group, token, invocation, examples)
				}
			}
		}
	}

	for invocation := range unusedExceptions {
		t.Errorf("synonymsVerifiedAgainstValidation lists %q, which the table no longer suggests", invocation)
	}
}
