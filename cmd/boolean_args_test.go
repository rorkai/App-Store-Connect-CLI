package cmd

import (
	"flag"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestNormalizeSpacedBooleanFlags(t *testing.T) {
	root := &ffcli.Command{
		Name:    "asc",
		FlagSet: flag.NewFlagSet("asc", flag.ContinueOnError),
	}
	commandFlags := flag.NewFlagSet("import", flag.ContinueOnError)
	commandFlags.Bool("confirm", false, "")
	commandFlags.Bool("dry-run", false, "")
	commandFlags.Bool("continue-on-error", true, "")
	commandFlags.Bool("invite", false, "")
	commandFlags.Bool("removed", false, "")
	commandFlags.Bool("replace", false, "")
	commandFlags.String("input", "", "")
	importCommand := &ffcli.Command{
		Name:    "import",
		FlagSet: commandFlags,
	}
	stringSiblingFlags := flag.NewFlagSet("push", flag.ContinueOnError)
	stringSiblingFlags.String("continue-on-error", "", "")
	root.Subcommands = []*ffcli.Command{
		importCommand,
		{Name: "push", FlagSet: stringSiblingFlags},
	}

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "explicit false before another flag",
			args: []string{"import", "--confirm", "false", "--dry-run"},
			want: []string{"import", "--confirm=false", "--dry-run"},
		},
		{
			name: "all boolean flags are normalized",
			args: []string{"import", "--confirm", "--continue-on-error", "false", "--dry-run", "true"},
			want: []string{"import", "--confirm", "--continue-on-error=false", "--dry-run=true"},
		},
		{
			name: "trailing explicit false cannot enable confirmation",
			args: []string{"import", "--confirm", "false"},
			want: []string{"import", "--confirm=false"},
		},
		{
			name: "invalid spaced boolean reaches flag validation",
			args: []string{"import", "--confirm", "maybe"},
			want: []string{"import", "--confirm=maybe"},
		},
		{
			name: "modifier false cannot hide later dry run",
			args: []string{"import", "--confirm", "--invite", "false", "--dry-run"},
			want: []string{"import", "--confirm", "--invite=false", "--dry-run"},
		},
		{
			name: "destructive false values are normalized",
			args: []string{"import", "--replace", "false", "--removed", "false"},
			want: []string{"import", "--replace=false", "--removed=false"},
		},
		{
			name: "non-boolean values are untouched",
			args: []string{"import", "--input", "false", "--confirm"},
			want: []string{"import", "--input", "false", "--confirm"},
		},
		{
			name: "equals syntax is untouched",
			args: []string{"import", "--confirm=false", "--dry-run=true"},
			want: []string{"import", "--confirm=false", "--dry-run=true"},
		},
		{
			name: "positional boolean is untouched",
			args: []string{"import", "false"},
			want: []string{"import", "false"},
		},
		{
			name: "mixed sibling flag kinds use active command",
			args: []string{"import", "--continue-on-error", "false"},
			want: []string{"import", "--continue-on-error=false"},
		},
		{
			name: "flag terminator stops normalization",
			args: []string{"import", "--", "--confirm", "false"},
			want: []string{"import", "--", "--confirm", "false"},
		},
		{
			name: "flag-looking string value is untouched",
			args: []string{"import", "--input", "--confirm", "false"},
			want: []string{"import", "--input", "--confirm", "false"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := normalizeSpacedBooleanFlags(root, test.args)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("normalizeSpacedBooleanFlags() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestNormalizeSpacedBooleanFlagsPreservesPositionalCommandPayloads(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "snitch true description",
			args: []string{"snitch", "--dry-run", "true"},
		},
		{
			name: "snitch false description",
			args: []string{"snitch", "--dry-run", "false"},
		},
		{
			name: "search true query",
			args: []string{"search", "--pretty", "true"},
		},
		{
			name: "search false query",
			args: []string{"search", "--pretty", "false"},
		},
		{
			name: "schema true query",
			args: []string{"schema", "--pretty", "true"},
		},
		{
			name: "schema false query",
			args: []string{"schema", "--pretty", "false"},
		},
		{
			name: "workflow true name",
			args: []string{"workflow", "run", "--dry-run", "true"},
		},
		{
			name: "workflow false name",
			args: []string{"workflow", "run", "--dry-run", "false"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := rootCommandForArgs("1.0.0", shared.DefaultDeps(), test.args)
			got := normalizeSpacedBooleanFlags(root, test.args)
			if !reflect.DeepEqual(got, test.args) {
				t.Fatalf("normalizeSpacedBooleanFlags() = %#v, want legacy positional args %#v", got, test.args)
			}
		})
	}
}

func TestRunPreservesBooleanSnitchDescriptions(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	for _, description := range []string{"true", "false"} {
		t.Run(description, func(t *testing.T) {
			_, stderr := captureCommandOutput(t, func() {
				args := []string{"snitch", "--dry-run", description}
				if code := Run(args, "1.0.0"); code != ExitSuccess {
					t.Fatalf("Run(%v) exit code = %d, want success", args, code)
				}
			})
			if !strings.Contains(stderr, "Title: "+description) {
				t.Fatalf("stderr = %q, want positional description %q", stderr, description)
			}
		})
	}
}

func TestRunSpacedFalseNeverEnablesBatchConfirmation(t *testing.T) {
	resetReportFlags(t)
	missing := filepath.Join(t.TempDir(), "missing-input")
	tests := [][]string{
		{
			"subscriptions", "pricing", "prices", "import",
			"--subscription-id", "6000000001", "--input", missing,
			"--confirm", "false",
		},
		{
			"subscriptions", "offers", "introductory", "import",
			"--subscription-id", "6000000001", "--input", missing,
			"--confirm", "false",
		},
		{
			"testflight", "testers", "import",
			"--app", "123456789", "--input", missing,
			"--confirm", "false",
		},
		{
			"reviews", "respond-batch",
			"--app", "123456789", "--file", missing,
			"--confirm", "false",
		},
		{
			"migrate", "import",
			"--app", "123456789", "--version-id", "VERSION_1", "--fastlane-dir", missing,
			"--confirm", "false",
		},
	}

	for _, args := range tests {
		_, stderr := captureCommandOutput(t, func() {
			if code := Run(args, "1.0.0"); code != ExitUsage {
				t.Fatalf("Run(%v) exit code = %d, want usage", args, code)
			}
		})
		if !strings.Contains(stderr, "--confirm is required unless --dry-run is set") {
			t.Fatalf("Run(%v) stderr = %q, want confirmation gate", args, stderr)
		}
		if strings.Contains(stderr, "missing-input") {
			t.Fatalf("Run(%v) read the input before rejecting spaced false: %q", args, stderr)
		}
	}
}

func TestRunNormalizesSpacedBooleansOnActiveCommandPath(t *testing.T) {
	resetReportFlags(t)
	tests := [][]string{
		{
			"subscriptions", "offers", "introductory", "import",
			"--confirm", "--continue-on-error", "false", "--help",
		},
		{
			"migrate", "import",
			"--confirm", "false", "--dry-run", "true", "--help",
		},
	}

	for _, args := range tests {
		stdout, _ := captureCommandOutput(t, func() {
			if code := Run(args, "1.0.0"); code != ExitSuccess {
				t.Fatalf("Run(%v) exit code = %d, want success", args, code)
			}
		})
		if !strings.Contains(stdout, "USAGE") {
			t.Fatalf("Run(%v) stdout = %q, want command help", args, stdout)
		}
	}
}

func TestRunNormalizesSpacedRootBooleanBeforeLazyCommandDiscovery(t *testing.T) {
	resetReportFlags(t)
	stdout, _ := captureCommandOutput(t, func() {
		args := []string{"--version", "false", "builds", "list", "--help"}
		if code := Run(args, "1.0.0"); code != ExitSuccess {
			t.Fatalf("Run(%v) exit code = %d, want success", args, code)
		}
	})
	if !strings.Contains(stdout, "asc builds list") {
		t.Fatalf("stdout = %q, want builds list help", stdout)
	}
}

func TestRunVersionRootFlagNeverDispatchesTrailingCommand(t *testing.T) {
	resetReportFlags(t)
	stdout, stderr := captureCommandOutput(t, func() {
		args := []string{"--version", "true", "builds", "list"}
		if code := Run(args, "9.8.7-test"); code != ExitSuccess {
			t.Fatalf("Run(%v) exit code = %d, want success", args, code)
		}
	})
	if stdout != "9.8.7-test\n" {
		t.Fatalf("stdout = %q, want version only", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no nested-command output", stderr)
	}
}

func TestRequestedHelpArgsRecognizesPositionalPayloadCommands(t *testing.T) {
	resetReportFlags(t)
	tests := []struct {
		name     string
		args     []string
		wantArgs []string
		wantHelp string
	}{
		{
			name:     "signing run help after positional operand",
			args:     []string{"signing", "run", "release", "--help"},
			wantArgs: []string{"signing", "run", "--help", "release"},
			wantHelp: "asc signing run",
		},
		{
			name:     "workflow run help after positional operand",
			args:     []string{"workflow", "run", "release", "--help"},
			wantArgs: []string{"workflow", "run", "--help", "release"},
			wantHelp: "asc workflow run",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := rootCommandForArgs("1.0.0", shared.DefaultDeps(), test.args)
			gotArgs, gotOK := requestedHelpArgs(root, test.args)
			if !gotOK {
				t.Fatalf("requestedHelpArgs(%v) ok = false, want true", test.args)
			}
			if !reflect.DeepEqual(gotArgs, test.wantArgs) {
				t.Fatalf("requestedHelpArgs(%v) = %#v, want %#v", test.args, gotArgs, test.wantArgs)
			}

			var code int
			stdout, stderr := captureCommandOutput(t, func() {
				code = Run(test.args, "1.0.0")
			})
			if code != ExitSuccess {
				t.Fatalf("Run(%v) exit code = %d, want %d (stderr=%q)", test.args, code, ExitSuccess, stderr)
			}
			if !strings.Contains(stdout, test.wantHelp) {
				t.Fatalf("Run(%v) stdout = %q, want %q", test.args, stdout, test.wantHelp)
			}
			if stderr != "" {
				t.Fatalf("Run(%v) stderr = %q, want empty", test.args, stderr)
			}
		})
	}
}
