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
			args:          []string{"review", "submissions-get", "--app", "SECRET_VALUE"},
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
			name: "an artifact-agnostic path flag does not answer --ipa",
			flags: func(fs *flag.FlagSet) {
				fs.String("path", "", "Path to the metadata directory (required)")
			},
			unknown: "--ipa",
			want:    nil,
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
			name: "identifier spelling skips sparse-field flags",
			flags: func(fs *flag.FlagSet) {
				fs.String("bundle-id-fields", "", "Sparse fields for included bundleIds")
				fs.String("id", "", "Profile ID (required)")
			},
			unknown:  "--bundle-id",
			fallback: true,
			want:     []string{"id"},
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

// TestRun_FilterExpressionNamesTheMappedFlags covers the REST filter
// spellings callers guess (`--filter[app]=X`, `--filter app:X`,
// `--filter "app=X,version=7"`). The error names the command with the flags
// those keys map to on it, and with values only when every key and value was
// read exactly; otherwise it names the flags with placeholders. A filter whose
// keys map to no flag on the command is left to the usual unknown-flag answer.
// On commands that filter builds, `version` is Apple's build number but asc's
// marketing version, so it never carries a value and the error names both
// flags.
func TestRun_FilterExpressionNamesTheMappedFlags(t *testing.T) {
	const buildVersionNote = "Apple's filter[version] on builds is the build number; for that, use --build-number BUILD_NUMBER."
	tests := []struct {
		args     []string
		command  string
		flag     string
		wantTry  string
		wantNote string
	}{
		{args: []string{"builds", "list", "--filter", "app:1234567890", "--sort", "-uploadedDate"}, command: "asc builds list", flag: "--filter", wantTry: "asc builds list --app 1234567890"},
		{args: []string{"builds", "list", "--filter[app]=1234567890", "--output", "json"}, command: "asc builds list", flag: "--filter[app]", wantTry: "asc builds list --app 1234567890"},
		{args: []string{"builds", "list", "--filter[app]", "1234567890"}, command: "asc builds list", flag: "--filter[app]", wantTry: "asc builds list --app 1234567890"},
		{args: []string{"builds", "list", "--filter=app=com.example.app"}, command: "asc builds list", flag: "--filter", wantTry: "asc builds list --app com.example.app"},
		{args: []string{"builds", "list", "--filter[app]=My App"}, command: "asc builds list", flag: "--filter[app]", wantTry: "asc builds list --app 'My App'"},
		{args: []string{"builds", "list", "--filter[APP]=1234567890"}, command: "asc builds list", flag: "--filter[APP]", wantTry: "asc builds list --app 1234567890"},
		{args: []string{"builds", "list", "--filter", "App=1234567890"}, command: "asc builds list", flag: "--filter", wantTry: "asc builds list --app 1234567890"},
		{args: []string{"builds", "list", "--filter", "PROCESSINGSTATE=VALID"}, command: "asc builds list", flag: "--filter", wantTry: "asc builds list --processing-state VALID"},
		{args: []string{"versions", "list", "--filter[app]=1234567890"}, command: "asc versions list", flag: "--filter[app]", wantTry: "asc versions list --app 1234567890"},
		{args: []string{"builds", "list", "--filter", "app=1234567890,version=7"}, command: "asc builds list", flag: "--filter", wantTry: "asc builds list --app 1234567890 --version VERSION", wantNote: buildVersionNote},
		{args: []string{"builds", "list", "--filter", "filter[app]=1234567890&filter[version]=7"}, command: "asc builds list", flag: "--filter", wantTry: "asc builds list --app 1234567890 --version VERSION", wantNote: buildVersionNote},
		{args: []string{"builds", "count", "--filter", "app=1234567890,version=7"}, command: "asc builds count", flag: "--filter", wantTry: "asc builds count --app 1234567890 --version VERSION", wantNote: buildVersionNote},
		{args: []string{"builds", "list", "--filter[app]=1234567890", "--filter[processingState]=VALID"}, command: "asc builds list", flag: "--filter[app]", wantTry: "asc builds list --app 1234567890 --processing-state VALID"},
		{args: []string{"builds", "list", "--filter", "processingState:VALID"}, command: "asc builds list", flag: "--filter", wantTry: "asc builds list --processing-state VALID"},
		{args: []string{"versions", "list", "--filter", "app=1234567890,version=1.0"}, command: "asc versions list", flag: "--filter", wantTry: "asc versions list --app 1234567890 --version 1.0"},
		{args: []string{"versions", "list", "--filter", "app=1234567890,platform=IOS"}, command: "asc versions list", flag: "--filter", wantTry: "asc versions list --app 1234567890 --platform IOS"},
		// A key with no exact flag, a piece without a key, a repeated key, a
		// value holding another expression, or a missing value: no value is
		// guessed, so the mapped flags are named with placeholders.
		{args: []string{"builds", "list", "--filter", "app=1234567890,preReleaseVersion.version=7"}, command: "asc builds list", flag: "--filter", wantTry: "asc builds list --app APP_ID"},
		{args: []string{"builds", "list", "--filter", "app=1234567890,version=7,1.1"}, command: "asc builds list", flag: "--filter", wantTry: "asc builds list --app APP_ID --version VERSION", wantNote: buildVersionNote},
		{args: []string{"builds", "list", "--filter", "app=1234567890,app=987654321"}, command: "asc builds list", flag: "--filter", wantTry: "asc builds list --app APP_ID"},
		{args: []string{"builds", "list", "--filter[app]=1234567890,version=7"}, command: "asc builds list", flag: "--filter[app]", wantTry: "asc builds list --app APP_ID"},
		{args: []string{"builds", "list", "--filter", "app:"}, command: "asc builds list", flag: "--filter", wantTry: "asc builds list --app APP_ID"},
		{args: []string{"builds", "list", "--filter", "exclude-expired=true"}, command: "asc builds list", flag: "--filter"},
		{args: []string{"builds", "list", "--filter", "limit=5"}, command: "asc builds list", flag: "--filter"},
		{args: []string{"builds", "groups", "list", "--filter[app]=1234567890"}, command: "asc builds groups list", flag: "--filter[app]"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			resetReportFlags(t)
			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "1.0.0"); code != ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
				}
			})
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			prefix := "Error: unknown flag `" + test.flag + "` for `" + test.command + "`\n"
			if test.wantTry == "" {
				if !strings.HasPrefix(stderr, prefix) || strings.Contains(stderr, "Try:\n  "+test.command) {
					t.Fatalf("stderr = %q, want the unknown flag without a command line to try", stderr)
				}
				return
			}
			want := prefix + "Try:\n  " + test.wantTry + "\n"
			if test.wantNote != "" {
				want += test.wantNote + "\n"
			}
			want += "For help:\n  " + test.command + " --help\n"
			if stderr != want {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
		})
	}
}
