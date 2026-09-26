package cmd

import (
	"flag"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

func newHoistTestRoot() *ffcli.Command {
	rootFlags := flag.NewFlagSet("asc", flag.ContinueOnError)
	rootFlags.String("profile", "", "Use named authentication profile")
	rootFlags.Bool("debug", false, "Enable debug logging")

	listFlags := flag.NewFlagSet("list", flag.ContinueOnError)
	listFlags.String("output", "", "Output format")
	listFlags.Bool("paginate", false, "Fetch every page")

	viewFlags := flag.NewFlagSet("view", flag.ContinueOnError)
	viewFlags.String("id", "", "Resource identifier")

	runFlags := flag.NewFlagSet("run", flag.ContinueOnError)
	runFlags.String("profile", "", "Path to the provisioning profile")

	searchFlags := flag.NewFlagSet("search", flag.ContinueOnError)
	searchFlags.String("output", "", "Output format")
	searchFlags.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:    "asc",
		FlagSet: rootFlags,
		Subcommands: []*ffcli.Command{
			{
				Name:    "apps",
				FlagSet: flag.NewFlagSet("apps", flag.ContinueOnError),
				Subcommands: []*ffcli.Command{
					{Name: "list", FlagSet: listFlags},
					{Name: "view", FlagSet: viewFlags},
					{
						Name:    "public",
						FlagSet: flag.NewFlagSet("public", flag.ContinueOnError),
						Subcommands: []*ffcli.Command{
							{Name: "search", FlagSet: flag.NewFlagSet("public search", flag.ContinueOnError)},
						},
					},
				},
			},
			{
				Name:    "signing",
				FlagSet: flag.NewFlagSet("signing", flag.ContinueOnError),
				Subcommands: []*ffcli.Command{
					{Name: "run", FlagSet: runFlags},
				},
			},
			{Name: "search", FlagSet: searchFlags},
		},
	}
}

func TestHoistRootProfileFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "leaf command separate value",
			args: []string{"apps", "list", "--profile", "staging"},
			want: []string{"--profile=staging", "apps", "list"},
		},
		{
			name: "leaf command inline value",
			args: []string{"apps", "list", "--profile=staging"},
			want: []string{"--profile=staging", "apps", "list"},
		},
		{
			name: "between command flags",
			args: []string{"apps", "list", "--profile", "staging", "--output", "json"},
			want: []string{"--profile=staging", "apps", "list", "--output", "json"},
		},
		{
			name: "group command",
			args: []string{"apps", "--profile", "staging", "list"},
			want: []string{"--profile=staging", "apps", "list"},
		},
		{
			name: "already at root",
			args: []string{"--profile", "staging", "apps", "list"},
			want: []string{"--profile", "staging", "apps", "list"},
		},
		{
			name: "preserves left to right precedence",
			args: []string{"--profile", "prod", "apps", "list", "--profile", "staging"},
			want: []string{"--profile", "prod", "--profile=staging", "apps", "list"},
		},
		{
			name: "keeps other root flags before the command",
			args: []string{"--debug", "apps", "list", "--profile", "staging"},
			want: []string{"--debug", "--profile=staging", "apps", "list"},
		},
		{
			name: "command owned profile flag is untouched",
			args: []string{"signing", "run", "--profile", "app.mobileprovision", "--", "child"},
			want: []string{"signing", "run", "--profile", "app.mobileprovision", "--", "child"},
		},
		{
			name: "value naming a subcommand is not consumed",
			args: []string{"apps", "--profile", "list"},
			want: []string{"apps", "--profile", "list"},
		},
		{
			name: "missing value is left in place",
			args: []string{"apps", "list", "--profile"},
			want: []string{"apps", "list", "--profile"},
		},
		{
			name: "earlier selector still hoists when a later one has no value",
			args: []string{"apps", "list", "--profile=staging", "--profile"},
			want: []string{"--profile=staging", "apps", "list", "--profile"},
		},
		{
			name: "stops at the terminator",
			args: []string{"search", "--", "--profile", "staging"},
			want: []string{"search", "--", "--profile", "staging"},
		},
		{
			name: "stops at a positional argument",
			args: []string{"search", "upload a build", "--profile", "staging"},
			want: []string{"search", "upload a build", "--profile", "staging"},
		},
		{
			name: "stops at an empty positional argument",
			args: []string{"search", "", "--profile", "staging"},
			want: []string{"search", "", "--profile", "staging"},
		},
		{
			name: "stops at an unknown flag",
			args: []string{"apps", "list", "--bogus", "--profile", "staging"},
			want: []string{"apps", "list", "--bogus", "--profile", "staging"},
		},
		{
			name: "stops at a malformed flag spelling",
			args: []string{"apps", "list", "---profile", "staging"},
			want: []string{"apps", "list", "---profile", "staging"},
		},
		{
			name: "boolean command flag keeps its following token",
			args: []string{"apps", "list", "--paginate", "--profile", "staging"},
			want: []string{"--profile=staging", "apps", "list", "--paginate"},
		},
		{
			name: "spaced boolean value does not end the walk",
			args: []string{"apps", "list", "--paginate", "false", "--profile", "staging"},
			want: []string{"--profile=staging", "apps", "list", "--paginate", "false"},
		},
		{
			name: "positional payload command keeps its spaced boolean value",
			args: []string{"search", "--pretty", "false", "--profile", "staging"},
			want: []string{"search", "--pretty", "false", "--profile", "staging"},
		},
		{
			name: "no profile flag leaves args untouched",
			args: []string{"apps", "list", "--output", "json"},
			want: []string{"apps", "list", "--output", "json"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := hoistRootProfileFlag(newHoistTestRoot(), slices.Clone(test.args))
			if !slices.Equal(got, test.want) {
				t.Fatalf("hoistRootProfileFlag(%q) = %q, want %q", test.args, got, test.want)
			}
		})
	}
}

func TestMarkLeadingSearchFlagTerminator(t *testing.T) {
	root := newHoistTestRoot()
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "leading search terminator",
			args: []string{"search", "--", "apps", "--profile", "staging"},
			want: []string{"search", shared.FlagTerminatorSentinel, "apps", "--profile", "staging"},
		},
		{
			name: "terminator after query is already retained",
			args: []string{"search", "apps", "--", "--profile", "staging"},
			want: []string{"search", "apps", "--", "--profile", "staging"},
		},
		{
			name: "terminator used as flag value",
			args: []string{"search", "--output", "--", "apps"},
			want: []string{"search", "--output", "--", "apps"},
		},
		{
			name: "other command",
			args: []string{"apps", "list", "--", "--profile", "staging"},
			want: []string{"apps", "list", "--", "--profile", "staging"},
		},
		{
			name: "nested search command",
			args: []string{"apps", "public", "search", "--", "--term"},
			want: []string{"apps", "public", "search", "--", "--term"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := markLeadingSearchFlagTerminator(root, tt.args)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("markLeadingSearchFlagTerminator() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHoistRootProfileFlagWithoutRootBinding(t *testing.T) {
	root := newHoistTestRoot()
	root.FlagSet = flag.NewFlagSet("asc", flag.ContinueOnError)

	args := []string{"apps", "list", "--profile", "staging"}
	got := hoistRootProfileFlag(root, slices.Clone(args))
	if !slices.Equal(got, args) {
		t.Fatalf("hoistRootProfileFlag = %q, want %q", got, args)
	}
}

func TestRun_ProfileAfterPositionalEmitsOneSanitizedUsageEvent(t *testing.T) {
	for _, args := range [][]string{
		{"search", "upload a build", "--profile", "staging"},
		{"search", "upload a build", "--profile=staging"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			resetReportFlags(t)
			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })

			var calls int
			var gotCommand string
			var gotExitCode int
			var gotContext telemetry.EventContext
			emitTelemetry = func(command string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
				calls++
				gotCommand = command
				gotExitCode = exitCode
				gotContext = eventContext
			}

			captureCommandOutput(t, func() {
				if code := Run(args, "1.2.3"); code != ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
				}
			})

			if calls != 1 || gotCommand != "asc search" || gotExitCode != ExitUsage {
				t.Fatalf("telemetry calls/command/exit = %d/%q/%d, want 1/asc search/%d", calls, gotCommand, gotExitCode, ExitUsage)
			}
			if gotContext.InvocationShape != telemetry.InvocationShapeLeaf ||
				gotContext.ErrorKind != telemetry.ErrorKindOther ||
				gotContext.FailureStage != telemetry.FailureStageValidation ||
				gotContext.OutcomeKind != telemetry.OutcomeUsageError ||
				gotContext.FailureParameter != "--profile" || gotContext.DiagnosticCode != "" {
				t.Fatalf("unexpected telemetry context: %+v", gotContext)
			}
		})
	}
}

func TestRun_ProfileAfterPositionalPreservesEarlierDiagnostics(t *testing.T) {
	t.Run("unknown flag", func(t *testing.T) {
		resetReportFlags(t)
		_, stderr := captureCommandOutput(t, func() {
			if code := Run([]string{"--bogus", "query", "--profile", "staging"}, "1.2.3"); code != ExitUsage {
				t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
			}
		})
		if !strings.Contains(stderr, "unknown flag `--bogus`") ||
			strings.Contains(stderr, "must appear before positional arguments") {
			t.Fatalf("stderr = %q, want the unknown-flag diagnostic", stderr)
		}
	})

	t.Run("explicit help", func(t *testing.T) {
		resetReportFlags(t)
		stdout, stderr := captureCommandOutput(t, func() {
			if code := Run([]string{"--help", "query", "--profile", "staging"}, "1.2.3"); code != ExitSuccess {
				t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
			}
		})
		if stdout == "" || stderr != "" {
			t.Fatalf("stdout/stderr = %q/%q, want help on stdout only", stdout, stderr)
		}
	})

	t.Run("invalid earlier flag value", func(t *testing.T) {
		resetReportFlags(t)
		_, stderr := captureCommandOutput(t, func() {
			if code := Run([]string{"search", "--pretty=maybe", "query", "--profile", "staging"}, "1.2.3"); code != ExitUsage {
				t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
			}
		})
		if !strings.Contains(stderr, "invalid boolean value") ||
			strings.Contains(stderr, "must appear before positional arguments") {
			t.Fatalf("stderr = %q, want the invalid-value diagnostic", stderr)
		}
	})

	t.Run("invalid interspersed search flag value", func(t *testing.T) {
		resetReportFlags(t)
		_, stderr := captureCommandOutput(t, func() {
			if code := Run([]string{"search", "query", "--limit", "nope", "--profile", "staging"}, "1.2.3"); code != ExitUsage {
				t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
			}
		})
		if !strings.Contains(stderr, `invalid value "nope" for --limit`) ||
			strings.Contains(stderr, "must appear before positional arguments") {
			t.Fatalf("stderr = %q, want the invalid-limit diagnostic", stderr)
		}
	})

	t.Run("unknown interspersed schema flag", func(t *testing.T) {
		resetReportFlags(t)
		_, stderr := captureCommandOutput(t, func() {
			if code := Run([]string{"schema", "apps", "--bogus", "--profile", "staging"}, "1.2.3"); code != ExitUsage {
				t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
			}
		})
		if !strings.Contains(stderr, "flag provided but not defined: -bogus") ||
			strings.Contains(stderr, "must appear before positional arguments") {
			t.Fatalf("stderr = %q, want the unknown-schema-flag diagnostic", stderr)
		}
	})

	for _, args := range [][]string{
		{"bogus", "--profile", "staging"},
		{"apps", "bogus", "--profile", "staging"},
	} {
		t.Run("unknown command "+strings.Join(args, " "), func(t *testing.T) {
			resetReportFlags(t)
			_, stderr := captureCommandOutput(t, func() {
				if code := Run(args, "1.2.3"); code != ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
				}
			})
			if !strings.Contains(stderr, "unknown command") || !strings.Contains(stderr, "bogus") ||
				strings.Contains(stderr, "must appear before positional arguments") {
				t.Fatalf("stderr = %q, want the unknown-command diagnostic", stderr)
			}
		})
	}
}

func TestRun_ProfileAfterPositionalWritesRequestedJUnitReport(t *testing.T) {
	resetReportFlags(t)
	reportPath := t.TempDir() + "/usage.xml"
	_, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{
			"--report", "junit",
			"--report-file", reportPath,
			"search", "query", "--profile", "staging",
		}, "1.2.3"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if !strings.Contains(stderr, "must appear before positional arguments") {
		t.Fatalf("stderr = %q, want the profile-placement diagnostic", stderr)
	}
	report, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", reportPath, err)
	}
	if !strings.Contains(string(report), "--profile") ||
		!strings.Contains(string(report), "must appear before positional arguments") {
		t.Fatalf("JUnit report does not contain the placement error: %s", report)
	}
}

// TestCommandOwnedProfileFlagInventory pins the commands that define their own
// `profile` flag. Those keep it, so a new command-local `--profile` must be a
// deliberate decision rather than a silent change to credential selection.
func TestCommandOwnedProfileFlagInventory(t *testing.T) {
	// Both commands take a provisioning-profile file path as --profile. They
	// are local-only commands that never read credentials, so the local flag
	// shadowing the root credential selector is deliberate.
	want := map[string]struct{}{
		"asc signing run":        {},
		"asc xcode signing plan": {},
	}

	got := map[string]struct{}{}
	var walk func(command *ffcli.Command, path []string)
	walk = func(command *ffcli.Command, path []string) {
		if command == nil {
			return
		}
		commandPath := append(append([]string{}, path...), command.Name)
		if len(commandPath) > 1 && command.FlagSet != nil {
			if command.FlagSet.Lookup(rootProfileFlagName) != nil {
				got[strings.Join(commandPath, " ")] = struct{}{}
			}
		}
		for _, subcommand := range command.Subcommands {
			walk(subcommand, commandPath)
		}
	}
	walk(RootCommand("test"), nil)

	for command := range want {
		if _, ok := got[command]; !ok {
			t.Errorf("%s no longer defines its own --profile flag", command)
		}
	}
	for command := range got {
		if _, ok := want[command]; !ok {
			t.Errorf("%s defines a command-local --profile flag; root --profile is the credential selector", command)
		}
	}
}
