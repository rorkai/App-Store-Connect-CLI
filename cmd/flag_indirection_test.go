package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

func newIndirectionTestTree() *ffcli.Command {
	root := &ffcli.Command{
		Name:    "asc",
		FlagSet: flag.NewFlagSet("asc", flag.ContinueOnError),
	}
	root.FlagSet.String("profile", "", "")
	root.FlagSet.String("report", "", "")
	root.FlagSet.String("report-file", "", "")
	root.FlagSet.Bool("debug", false, "")

	updateFlags := flag.NewFlagSet("update", flag.ContinueOnError)
	updateFlags.String("whats-new", "", "")
	updateFlags.String("secret", "", "")
	updateFlags.String("output", "", "")
	updateFlags.String("format", "", "")
	updateFlags.String("profile", "", "")
	updateFlags.Bool("confirm", false, "")
	var enabled shared.OptionalBool
	updateFlags.Var(&enabled, "enabled", "")
	updateFlags.Int("limit", 0, "")
	shared.BindOnceCSVFlag(updateFlags, "events", "")
	var tags shared.MultiStringFlag
	updateFlags.Var(&tags, "tag", "")

	group := &ffcli.Command{
		Name:    "localizations",
		FlagSet: flag.NewFlagSet("localizations", flag.ContinueOnError),
		Subcommands: []*ffcli.Command{
			{Name: "update", FlagSet: updateFlags},
		},
	}
	root.Subcommands = []*ffcli.Command{group}
	return root
}

func TestResolveFlagValueIndirectionRewritesArgs(t *testing.T) {
	t.Setenv("ASC_TEST_NOTES", "Fixed crashes")
	t.Setenv("ASC_TEST_SECRET", "hunter2")
	t.Setenv("ASC_TEST_EVENTS", "A,B")
	t.Setenv("ASC_TEST_FORMAT", "json")
	t.Setenv("ASC_TEST_ENABLED", "true")
	notesPath := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(notesPath, []byte("From file\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "separate value on subcommand",
			args: []string{"localizations", "update", "--whats-new", "@env:ASC_TEST_NOTES"},
			want: []string{"localizations", "update", "--whats-new", "Fixed crashes"},
		},
		{
			name: "inline value on subcommand",
			args: []string{"localizations", "update", "--whats-new=@env:ASC_TEST_NOTES"},
			want: []string{"localizations", "update", "--whats-new=Fixed crashes"},
		},
		{
			name: "single dash spelling",
			args: []string{"localizations", "update", "-whats-new", "@file:" + notesPath},
			want: []string{"localizations", "update", "-whats-new", "From file"},
		},
		{
			name: "double at escape",
			args: []string{"localizations", "update", "--whats-new", "@@env:literal"},
			want: []string{"localizations", "update", "--whats-new", "@env:literal"},
		},
		{
			name: "plain at value untouched",
			args: []string{"localizations", "update", "--whats-new", "@handle"},
			want: []string{"localizations", "update", "--whats-new", "@handle"},
		},
		{
			name: "typed and custom values resolve before parsing",
			args: []string{"localizations", "update", "--events", "@env:ASC_TEST_EVENTS", "--tag", "@env:ASC_TEST_SECRET", "--tag", "@@x", "--limit", "@env:ASC_TEST_EVENTS"},
			want: []string{"localizations", "update", "--events", "A,B", "--tag", "hunter2", "--tag", "@x", "--limit", "A,B"},
		},
		{
			name: "explicit-value boolean resolves like any value flag",
			args: []string{"localizations", "update", "--enabled", "@env:ASC_TEST_ENABLED"},
			want: []string{"localizations", "update", "--enabled", "true"},
		},
		{
			name: "boolean flag never consumes an indirect value",
			args: []string{"localizations", "update", "--confirm", "@env:ASC_TEST_SECRET"},
			want: []string{"localizations", "update", "--confirm", "@env:ASC_TEST_SECRET"},
		},
		{
			name: "boolean inline value untouched",
			args: []string{"--debug=@env:ASC_TEST_SECRET", "localizations", "update"},
			want: []string{"--debug=@env:ASC_TEST_SECRET", "localizations", "update"},
		},
		{
			name: "root routing flags excluded",
			args: []string{"--profile", "@env:ASC_TEST_SECRET", "--report=@env:ASC_TEST_SECRET", "--report-file", "@file:" + notesPath, "localizations", "update", "--secret", "@env:ASC_TEST_SECRET"},
			want: []string{"--profile", "@env:ASC_TEST_SECRET", "--report=@env:ASC_TEST_SECRET", "--report-file", "@file:" + notesPath, "localizations", "update", "--secret", "hunter2"},
		},
		{
			name: "output excluded on subcommand",
			args: []string{"localizations", "update", "--output", "@env:ASC_TEST_SECRET", "--secret", "@env:ASC_TEST_SECRET"},
			want: []string{"localizations", "update", "--output", "@env:ASC_TEST_SECRET", "--secret", "hunter2"},
		},
		{
			name: "command-local profile flag resolves",
			args: []string{"localizations", "update", "--profile", "@file:" + notesPath},
			want: []string{"localizations", "update", "--profile", "From file"},
		},
		{
			name: "command-local format flag resolves",
			args: []string{"localizations", "update", "--format", "@env:ASC_TEST_FORMAT"},
			want: []string{"localizations", "update", "--format", "json"},
		},
		{
			name: "terminator stops rewriting",
			args: []string{"localizations", "update", "--secret", "@env:ASC_TEST_SECRET", "--", "--whats-new", "@env:ASC_TEST_NOTES"},
			want: []string{"localizations", "update", "--secret", "hunter2", "--", "--whats-new", "@env:ASC_TEST_NOTES"},
		},
		{
			name: "malformed flag prefix stops rewriting",
			args: []string{"localizations", "update", "---secret=@env:ASC_TEST_SECRET", "--whats-new", "@env:ASC_TEST_NOTES"},
			want: []string{"localizations", "update", "---secret=@env:ASC_TEST_SECRET", "--whats-new", "@env:ASC_TEST_NOTES"},
		},
		{
			name: "unknown flag stops rewriting",
			args: []string{"localizations", "update", "--secret", "@env:ASC_TEST_SECRET", "--nope", "--whats-new", "@env:ASC_TEST_NOTES"},
			want: []string{"localizations", "update", "--secret", "hunter2", "--nope", "--whats-new", "@env:ASC_TEST_NOTES"},
		},
		{
			name: "positional payload untouched and later known flags still resolve",
			args: []string{"localizations", "update", "@env:ASC_TEST_NOTES", "--whats-new", "@env:ASC_TEST_NOTES"},
			want: []string{"localizations", "update", "@env:ASC_TEST_NOTES", "--whats-new", "Fixed crashes"},
		},
		{
			name: "group without leaf leaves values",
			args: []string{"localizations", "--whats-new", "@env:ASC_TEST_NOTES"},
			want: []string{"localizations", "--whats-new", "@env:ASC_TEST_NOTES"},
		},
		{
			name: "trailing flag without value",
			args: []string{"localizations", "update", "--whats-new"},
			want: []string{"localizations", "update", "--whats-new"},
		},
		{
			name: "empty tokens preserved",
			args: []string{"", "localizations", "update", "--whats-new", "@env:ASC_TEST_NOTES", ""},
			want: []string{"", "localizations", "update", "--whats-new", "Fixed crashes", ""},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original := append([]string(nil), test.args...)
			got, err := resolveFlagValueIndirection(newIndirectionTestTree(), test.args)
			if err != nil {
				t.Fatalf("resolveFlagValueIndirection() error = %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("resolveFlagValueIndirection() = %q, want %q", got, test.want)
			}
			if !reflect.DeepEqual(test.args, original) {
				t.Fatalf("input args mutated to %q", test.args)
			}
		})
	}
}

func TestResolveFlagValueIndirectionReportsFlagOnError(t *testing.T) {
	os.Unsetenv("ASC_TEST_INDIRECT_UNSET")
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "separate value",
			args:    []string{"localizations", "update", "--secret", "@env:ASC_TEST_INDIRECT_UNSET"},
			wantErr: "--secret: environment variable ASC_TEST_INDIRECT_UNSET is not set",
		},
		{
			name:    "inline value",
			args:    []string{"localizations", "update", "--whats-new=@file:" + filepath.Join(t.TempDir(), "missing.txt")},
			wantErr: "--whats-new: cannot read file ",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveFlagValueIndirection(newIndirectionTestTree(), test.args)
			if err == nil {
				t.Fatalf("resolveFlagValueIndirection() = %q, want error", got)
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), test.wantErr)
			}
		})
	}
}

func TestRunIndirectionFailureIsUsageErrorWithoutValueInTelemetry(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	os.Unsetenv("ASC_TEST_INDIRECT_UNSET")

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var gotContext telemetry.EventContext
	var gotExit int
	var gotCommand string
	emitTelemetry = func(commandName string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
		gotCommand = commandName
		gotExit = exitCode
		gotContext = eventContext
	}

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"localizations", "update", "--id", "loc-1", "--whats-new", "@env:ASC_TEST_INDIRECT_UNSET"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	want := "Error: --whats-new: environment variable ASC_TEST_INDIRECT_UNSET is not set\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	if gotExit != ExitUsage || gotCommand != "asc localizations update" {
		t.Fatalf("telemetry = (%q, %d), want (asc localizations update, %d)", gotCommand, gotExit, ExitUsage)
	}
	if gotContext.ErrorKind != telemetry.ErrorKindInvalidValue ||
		gotContext.FailureStage != telemetry.FailureStageValidation ||
		gotContext.OutcomeKind != telemetry.OutcomeUsageError ||
		gotContext.FailureParameter != "--whats-new" {
		t.Fatalf("telemetry context = %+v, want invalid-value validation failure on --whats-new", gotContext)
	}
}

func TestRunIndirectionResolvedValueNeverReachesTelemetry(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	const secret = "s3cr3t-webhook-value-9f1c"
	t.Setenv("ASC_TEST_WEBHOOK_SECRET", secret)

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var recorded []string
	emitTelemetry = func(commandName string, versionInfo string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
		recorded = append(recorded, commandName, versionInfo, fmt.Sprintf("%+v", eventContext))
		if event, ok := telemetry.BuildEventWithContext(commandName, versionInfo, 0, exitCode, eventContext); ok {
			recorded = append(recorded, fmt.Sprintf("%+v", event))
		}
	}

	// The secret resolves, then the command fails validation on another flag,
	// so the run exercises the telemetry path after a successful rewrite.
	_, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"webhooks", "create", "--secret", "@env:ASC_TEST_WEBHOOK_SECRET"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if strings.Contains(stderr, secret) {
		t.Fatalf("stderr leaked the resolved value: %q", stderr)
	}
	if len(recorded) == 0 {
		t.Fatal("expected telemetry to be emitted")
	}
	for _, item := range recorded {
		if strings.Contains(item, secret) {
			t.Fatalf("telemetry leaked the resolved value: %q", item)
		}
	}
}

func TestRunHelpWinsOverUnresolvableIndirectValue(t *testing.T) {
	resetReportFlags(t)
	os.Unsetenv("ASC_TEST_INDIRECT_UNSET")
	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"localizations", "update", "--whats-new", "@env:ASC_TEST_INDIRECT_UNSET", "--help"}, "1.0.0"); code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})
	if !strings.Contains(stdout, "USAGE") || !strings.Contains(stdout, "--whats-new") {
		t.Fatalf("stdout = %q, want command help", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestResolveFlagValueIndirectionScopesRootSelectorsToRootFlagSet(t *testing.T) {
	t.Setenv("ASC_TEST_PROFILE_PATH", "/tmp/asc-test/dev.mobileprovision")

	root := rootCommandForArgs("1.0.0", []string{"signing", "run"})
	commandLocal := []string{"signing", "run", "--profile", "@env:ASC_TEST_PROFILE_PATH"}
	got, err := resolveFlagValueIndirection(root, commandLocal)
	if err != nil {
		t.Fatalf("resolveFlagValueIndirection() error = %v", err)
	}
	want := []string{"signing", "run", "--profile", "/tmp/asc-test/dev.mobileprovision"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveFlagValueIndirection() = %q, want %q", got, want)
	}

	rootLevel := []string{"--profile", "@env:ASC_TEST_PROFILE_PATH", "apps", "list"}
	got, err = resolveFlagValueIndirection(rootCommandForArgs("1.0.0", rootLevel), rootLevel)
	if err != nil {
		t.Fatalf("resolveFlagValueIndirection() error = %v", err)
	}
	if !reflect.DeepEqual(got, rootLevel) {
		t.Fatalf("resolveFlagValueIndirection() = %q, want the root selector untouched %q", got, rootLevel)
	}
}

func TestRunHelpWinsOverResolvableIndirectTypedValue(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_TEST_INDIRECT_LIMIT", "5")
	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"apps", "list", "--limit", "@env:ASC_TEST_INDIRECT_LIMIT", "--help"}, "1.0.0"); code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})
	if !strings.Contains(stdout, "USAGE") || !strings.Contains(stdout, "--limit") {
		t.Fatalf("stdout = %q, want command help", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestRunHelpDropsIndirectValuesWithoutResolvingThem(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	const secret = "s3cr3t-not-a-number-4b7d"
	t.Setenv("ASC_TEST_INDIRECT_LIMIT_BAD", secret)
	os.Unsetenv("ASC_TEST_INDIRECT_UNSET")
	missing := filepath.Join(t.TempDir(), "never-read.txt")

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "resolvable typed value",
			args: []string{"apps", "list", "--limit", "@env:ASC_TEST_INDIRECT_LIMIT_BAD", "--help"},
		},
		{
			name: "unset environment variable",
			args: []string{"apps", "list", "--limit", "@env:ASC_TEST_INDIRECT_UNSET", "--help"},
		},
		{
			name: "missing file is never opened",
			args: []string{"apps", "list", "--next=@file:" + missing, "--help"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "1.0.0"); code != ExitSuccess {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
				}
			})
			if !strings.Contains(stdout, "USAGE") {
				t.Fatalf("stdout = %q, want command help", stdout)
			}
			if strings.Contains(stdout, secret) || strings.Contains(stderr, secret) {
				t.Fatalf("help leaked the resolved value: stdout = %q, stderr = %q", stdout, stderr)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
		})
	}
}

func TestDropIndirectFlagValuesKeepsEverythingElse(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "separate and inline indirect values dropped",
			args: []string{"localizations", "update", "--secret", "@env:X", "--whats-new=@file:/tmp/x", "--help"},
			want: []string{"localizations", "update", "--help"},
		},
		{
			name: "escape form dropped too",
			args: []string{"localizations", "update", "--whats-new", "@@literal", "--help"},
			want: []string{"localizations", "update", "--help"},
		},
		{
			name: "plain values and booleans kept",
			args: []string{"localizations", "update", "--secret", "plain", "--confirm", "--help"},
			want: []string{"localizations", "update", "--secret", "plain", "--confirm", "--help"},
		},
		{
			name: "excluded and unknown flags kept verbatim",
			args: []string{"--profile", "@env:X", "localizations", "update", "--output", "@env:X", "--nope", "@env:X", "--help"},
			want: []string{"--profile", "@env:X", "localizations", "update", "--output", "@env:X", "--nope", "@env:X", "--help"},
		},
		{
			name: "malformed flag prefix kept so bad flag syntax still reports",
			args: []string{"localizations", "update", "---secret=@env:X", "--help"},
			want: []string{"localizations", "update", "---secret=@env:X", "--help"},
		},
		{
			name: "terminator and positionals kept",
			args: []string{"localizations", "update", "@env:X", "--help", "--", "@env:X"},
			want: []string{"localizations", "update", "@env:X", "--help", "--", "@env:X"},
		},
		{
			name: "trailing flag without a value kept",
			args: []string{"localizations", "update", "--secret"},
			want: []string{"localizations", "update", "--secret"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := dropIndirectFlagValues(newIndirectionTestTree(), test.args)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("dropIndirectFlagValues() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRunHelpLeavesLiteralValueValidationUnchanged(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	// Help precedence covers indirection, not flag parsing at large: a literal
	// value keeps the behavior it had before this feature existed.
	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"apps", "list", "--limit=not-a-number", "--help"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "for flag -limit") {
		t.Fatalf("stderr = %q, want the invalid-value error for --limit", stderr)
	}
}

func TestRunHelpKeepsBadFlagSyntaxAuthoritative(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_TEST_INDIRECT_LIMIT", "5")
	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"apps", "list", "---limit=@env:ASC_TEST_INDIRECT_LIMIT", "--help"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "bad flag syntax") {
		t.Fatalf("stderr = %q, want the bad flag syntax error", stderr)
	}
	if strings.Contains(stderr, "=5") {
		t.Fatalf("stderr = %q, want the raw token, not a resolved value", stderr)
	}
}

func TestResolveFlagValueIndirectionResolvesFlagsAfterAPositional(t *testing.T) {
	// `asc search` takes the query as a positional and re-parses its own
	// flags afterwards, so the walk must not stop at the first positional:
	// stopping would hand the command a literal `@env:NAME` string.
	t.Setenv("ASC_TEST_SEARCH_LIMIT", "2")
	args := []string{"search", "upload a build", "--limit", "@env:ASC_TEST_SEARCH_LIMIT"}
	got, err := resolveFlagValueIndirection(rootCommandForArgs("1.0.0", args), args)
	if err != nil {
		t.Fatalf("resolveFlagValueIndirection() error = %v", err)
	}
	want := []string{"search", "upload a build", "--limit", "2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveFlagValueIndirection() = %q, want %q", got, want)
	}
}
