package cmd

import (
	"flag"
	"strings"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

// TestUnknownFlagSuggestionsResolveOnTargetCommand walks the whole command tree
// and asserts that every suggestion for every synonym key, and for a set of
// generic typos, names a flag the target command actually defines.
func TestUnknownFlagSuggestionsResolveOnTargetCommand(t *testing.T) {
	root := RootCommand("1.0.0")
	inputs := make([]string, 0, len(flagSynonyms)+8)
	for key := range flagSynonyms {
		inputs = append(inputs, key)
	}
	inputs = append(inputs, "ap", "appp", "versn", "grup", "buildid", "outpt", "idd", "ipaa")

	walkCommandTree(root, func(path string, command *ffcli.Command) {
		if command.FlagSet == nil {
			return
		}
		for _, input := range inputs {
			suggestions := unknownFlagSuggestions(command.FlagSet, "--"+input, unknownFlagSuggestionOptions{allowSelectorFallback: true})
			if len(suggestions) > maxUnknownFlagSuggestions {
				t.Fatalf("%s --%s returned %d suggestions, want at most %d", path, input, len(suggestions), maxUnknownFlagSuggestions)
			}
			seen := make(map[string]struct{}, len(suggestions))
			for _, suggestion := range suggestions {
				if suggestion == input {
					t.Fatalf("%s --%s suggested itself", path, input)
				}
				if _, repeated := seen[suggestion]; repeated {
					t.Fatalf("%s --%s repeated suggestion --%s", path, input, suggestion)
				}
				seen[suggestion] = struct{}{}
				if command.FlagSet.Lookup(suggestion) == nil {
					t.Fatalf("%s --%s suggested undefined flag --%s", path, input, suggestion)
				}
			}
		}
	})
}

// TestFlagSynonymTargetsExistSomewhere guards the curated table against typos:
// every synonym target must be a real flag on at least one command, and no key
// may map to itself.
func TestFlagSynonymTargetsExistSomewhere(t *testing.T) {
	root := RootCommand("1.0.0")
	defined := make(map[string]struct{})
	walkCommandTree(root, func(_ string, command *ffcli.Command) {
		if command.FlagSet == nil {
			return
		}
		command.FlagSet.VisitAll(func(item *flag.Flag) {
			defined[item.Name] = struct{}{}
		})
	})

	for key, targets := range flagSynonyms {
		if len(targets) == 0 {
			t.Fatalf("synonym %q has no targets", key)
		}
		for _, target := range targets {
			if target == key {
				t.Fatalf("synonym %q maps to itself", key)
			}
			if _, ok := defined[target]; !ok {
				t.Fatalf("synonym %q target --%s is not defined by any command", key, target)
			}
		}
	}
}

// TestRun_UnknownFlagSuggestionsPreserveTelemetryParameter keeps the usage exit
// code and the failure parameter unchanged: the unknown flag token still
// reaches the event context, and the telemetry allowlist still reduces it to a
// known flag name with no value attached.
func TestRun_UnknownFlagSuggestionsPreserveTelemetryParameter(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantContext   string
		wantParameter string
	}{
		{
			name:          "allowlisted name",
			args:          []string{"submit", "status", "--app", "SECRET_VALUE"},
			wantContext:   "--app",
			wantParameter: "--app",
		},
		{
			name:          "unrecognized name is dropped",
			args:          []string{"builds", "groups", "list", "--totally-made-up=SECRET_VALUE"},
			wantContext:   "--totally-made-up=SECRET_VALUE",
			wantParameter: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetReportFlags(t)
			original := emitTelemetry
			t.Cleanup(func() { emitTelemetry = original })
			var calls int
			var gotExit int
			var gotContext telemetry.EventContext
			emitTelemetry = func(_, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
				calls++
				gotExit = exitCode
				gotContext = eventContext
			}

			_, _ = captureCommandOutput(t, func() {
				if code := Run(test.args, "1.0.0"); code != ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
				}
			})

			if calls != 1 || gotExit != ExitUsage {
				t.Fatalf("telemetry calls = %d exit = %d, want 1 and %d", calls, gotExit, ExitUsage)
			}
			if gotContext.ErrorKind != telemetry.ErrorKindUnknownFlag ||
				gotContext.FailureStage != telemetry.FailureStageParse ||
				gotContext.OutcomeKind != telemetry.OutcomeUsageError {
				t.Fatalf("unexpected telemetry context: %+v", gotContext)
			}
			if gotContext.FailureParameter != test.wantContext {
				t.Fatalf("context failure parameter = %q, want %q", gotContext.FailureParameter, test.wantContext)
			}

			event, ok := telemetry.BuildEventWithContext("asc builds list", "1.0.0", 0, ExitUsage, gotContext)
			if !ok {
				t.Fatal("BuildEventWithContext() returned no event")
			}
			switch {
			case test.wantParameter == "":
				if event.FailureParameter != nil {
					t.Fatalf("failure parameter = %q, want nil", *event.FailureParameter)
				}
			case event.FailureParameter == nil || *event.FailureParameter != test.wantParameter:
				t.Fatalf("failure parameter = %v, want %q", event.FailureParameter, test.wantParameter)
			}
			if event.FailureParameter != nil && strings.Contains(*event.FailureParameter, "SECRET_VALUE") {
				t.Fatalf("failure parameter leaked a value: %q", *event.FailureParameter)
			}
		})
	}
}

func walkCommandTree(command *ffcli.Command, visit func(path string, command *ffcli.Command)) {
	var walk func(path string, node *ffcli.Command)
	walk = func(path string, node *ffcli.Command) {
		if node == nil {
			return
		}
		current := strings.TrimSpace(path + " " + node.Name)
		visit(current, node)
		for _, sub := range node.Subcommands {
			walk(current, sub)
		}
	}
	walk("", command)
}

// TestUnknownFlagSuggestionRankingTiers pins the tier order on a synthetic flag
// set: a curated synonym outranks a nearer name, the nearest name answers a
// typo, and the identifier-selector fallback only fires when nothing else did.
func TestUnknownFlagSuggestionRankingTiers(t *testing.T) {
	tests := []struct {
		name     string
		flags    func(*flag.FlagSet)
		unknown  string
		fallback bool
		want     []string
	}{
		{
			name: "synonym outranks nearer name",
			flags: func(fs *flag.FlagSet) {
				fs.String("group-id", "", "Beta group ID")
				fs.String("groups", "", "Comma-separated group names")
			},
			unknown: "--group",
			want:    []string{"group-id", "groups"},
		},
		{
			name: "typo resolves to the nearest name",
			flags: func(fs *flag.FlagSet) {
				fs.String("app", "", "App Store Connect app ID")
				fs.String("output", "", "Output format")
			},
			unknown: "--ap",
			want:    []string{"app"},
		},
		{
			name: "selector fallback answers an invented identifier",
			flags: func(fs *flag.FlagSet) {
				fs.String("build-id", "", "Build ID whose groups should be listed")
				fs.String("output", "", "Output format")
			},
			unknown:  "--app",
			fallback: true,
			want:     []string{"build-id"},
		},
		{
			name: "selector fallback stays out of the way of a real match",
			flags: func(fs *flag.FlagSet) {
				fs.String("app-store-version-id", "", "App Store version ID")
				fs.String("id", "", "Submission ID")
			},
			unknown:  "--version-id",
			fallback: true,
			want:     []string{"app-store-version-id", "id"},
		},
		{
			name: "selector fallback skips sparse-field and paging flags",
			flags: func(fs *flag.FlagSet) {
				fs.String("fields", "", "Sparse fields for subscriptions")
				fs.String("version-fields", "", "Sparse fields for included subscriptionVersions")
				fs.String("next", "", "Fetch next page using a links.next URL")
				fs.Int("limit", 0, "Maximum results per page (1-200)")
			},
			unknown:  "--app",
			fallback: true,
			want:     nil,
		},
		{
			name: "a path-valued output flag answers an unknown path flag",
			flags: func(fs *flag.FlagSet) {
				fs.String("output", "", "Output CSV file path (required)")
			},
			unknown: "--path",
			want:    []string{"output"},
		},
		{
			name: "a path-first output flag answers an unknown path flag",
			flags: func(fs *flag.FlagSet) {
				fs.String("output", "", "Path for the newly re-signed IPA (required)")
				fs.String("ipa", "", "Path to the IPA to re-sign (required)")
			},
			unknown: "--path",
			want:    []string{"ipa", "output"},
		},
		{
			name: "a format-only output flag is not a path destination",
			flags: func(fs *flag.FlagSet) {
				fs.String("output", "", "Output format: json, table, markdown (default: json)")
				fs.String("platform", "", "Filter by platform")
			},
			unknown: "--path",
			want:    nil,
		},
		{
			name: "selector fallback stays silent when another input is required",
			flags: func(fs *flag.FlagSet) {
				fs.String("org", "", "Apple Ads organization ID (or ASC_ADS_ORG_ID env)")
				fs.String("ad-group", "", "ad group (required)")
				fs.String("campaign", "", "campaign (required)")
			},
			unknown:  "--id",
			fallback: true,
			want:     nil,
		},
		{
			name: "an identifier inside a filename template is not a selector",
			flags: func(fs *flag.FlagSet) {
				fs.String("template", "", "Name pattern such as AuthKey_<KEY_ID>.p8")
			},
			unknown:  "--app",
			fallback: true,
			want:     nil,
		},
		{
			name: "deprecated flags are never suggested",
			flags: func(fs *flag.FlagSet) {
				fs.String("group-id", "", "Deprecated: use --id")
			},
			unknown: "--group",
			want:    nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			flags := flag.NewFlagSet("asc test", flag.ContinueOnError)
			test.flags(flags)
			got := unknownFlagSuggestions(flags, test.unknown, unknownFlagSuggestionOptions{
				allowSelectorFallback: test.fallback,
			})
			if strings.Join(got, ",") != strings.Join(test.want, ",") {
				t.Fatalf("suggestions = %v, want %v", got, test.want)
			}
		})
	}
}

// TestRemovedFlagGuidanceSuggestionsFollowTheRule keeps the removed-alias
// message authoritative: a rule that names a replacement prints no competing
// suggestion, and a note-only rule - which names nothing to type instead - gets
// the generic suggestions after its message.
func TestRemovedFlagGuidanceSuggestionsFollowTheRule(t *testing.T) {
	flags := flag.NewFlagSet("asc test", flag.ContinueOnError)
	flags.String("build-id", "", "Build ID")
	flags.String("build-number", "", "CFBundleVersion")

	if got := removedFlagGuidanceSuggestions(
		removedFlagRule{flag: "build", replacement: "`--build-id`"},
		flags,
		"--build",
	); got != nil {
		t.Fatalf("replacement rule suggestions = %v, want none", got)
	}

	got := removedFlagGuidanceSuggestions(
		removedFlagRule{flag: "build", note: "builds are selected by identifier"},
		flags,
		"--build",
	)
	if strings.Join(got, ",") != "build-id,build-number" {
		t.Fatalf("note-only rule suggestions = %v, want [build-id build-number]", got)
	}

	excluded := removedFlagGuidanceSuggestions(
		removedFlagRule{flag: "build", note: "use `--build-id` only through `asc builds wait --build-id`"},
		flags,
		"--build",
	)
	if strings.Join(excluded, ",") != "build-number" {
		t.Fatalf("suggestions = %v, want only [build-number]", excluded)
	}
}

// TestConditionalSynonymTargetsExistSomewhere holds the help-text-dependent
// synonyms to the same standard as the plain table: every target is a real flag
// somewhere in the CLI, and it is only offered when its own help text agrees.
func TestConditionalSynonymTargetsExistSomewhere(t *testing.T) {
	root := RootCommand("1.0.0")
	usages := make(map[string][]string)
	walkCommandTree(root, func(_ string, command *ffcli.Command) {
		if command.FlagSet == nil {
			return
		}
		command.FlagSet.VisitAll(func(item *flag.Flag) {
			usages[item.Name] = append(usages[item.Name], item.Usage)
		})
	})

	for key, conditionals := range conditionalSynonyms {
		for _, conditional := range conditionals {
			if conditional.target == key {
				t.Fatalf("conditional synonym %q maps to itself", key)
			}
			seen, ok := usages[conditional.target]
			if !ok {
				t.Fatalf("conditional synonym %q target --%s is not defined by any command", key, conditional.target)
			}
			matched := false
			for _, usage := range seen {
				if conditional.when(usage) {
					matched = true
					break
				}
			}
			if !matched {
				t.Fatalf("conditional synonym %q target --%s never satisfies its own condition", key, conditional.target)
			}
		}
	}
}
