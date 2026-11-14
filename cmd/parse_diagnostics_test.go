package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

func TestRunKnownFlagParseFailuresAreConcise(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "invalid value",
			args: []string{"builds", "list", "--limit", "nope"},
			want: `invalid value "nope" for flag --limit: parse error`,
		},
		{
			name: "missing value",
			args: []string{"builds", "list", "--limit"},
			want: "flag needs an argument: --limit",
		},
		{
			name: "invalid boolean",
			args: []string{"builds", "list", "--paginate", "nope"},
			want: `invalid boolean value "nope" for --paginate: parse error`,
		},
		{
			name: "invalid boolean with equals",
			args: []string{"builds", "list", "--paginate=nope"},
			want: `invalid boolean value "nope" for --paginate: parse error`,
		},
		{
			name: "flag typed with one dash",
			args: []string{"builds", "list", "-limit", "nope"},
			want: `invalid value "nope" for flag --limit: parse error`,
		},
		{
			name: "value that looks like a parse message",
			args: []string{"builds", "list", "--limit", "1 for flag -limit: x"},
			want: `invalid value "1 for flag -limit: x" for flag --limit: parse error`,
		},
		{
			name: "value with a quote and a dash",
			args: []string{"builds", "list", "--limit", `a" for -x`},
			want: `invalid value "a\" for -x" for flag --limit: parse error`,
		},
		{
			name: "dash value",
			args: []string{"builds", "list", "--limit", "-x"},
			want: `invalid value "-x" for flag --limit: parse error`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetReportFlags(t)
			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "1.0.0"); code != ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
				}
			})

			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			want := "Error: " + test.want + "\nFor help:\n  asc builds list --help\n"
			if stderr != want {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
			if strings.Contains(stderr, "DESCRIPTION") || strings.Contains(stderr, "USAGE") || strings.Contains(stderr, "FLAGS") {
				t.Fatalf("parse failure dumped command help: %q", stderr)
			}
		})
	}
}

func TestRunRootFlagParseFailurePointsToRootHelp(t *testing.T) {
	resetReportFlags(t)
	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"--strict-auth=not-a-bool", "builds", "list"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "asc --help") {
		t.Fatalf("stderr = %q, want root help guidance", stderr)
	}
	if strings.Contains(stderr, "asc builds list --help") {
		t.Fatalf("stderr = %q, must not point to leaf help for a root flag", stderr)
	}
}

func TestRunIntermediateFlagParseFailurePointsToOwningGroupHelp(t *testing.T) {
	resetReportFlags(t)
	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"reviews", "--limit", "nope", "view"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "asc reviews --help") {
		t.Fatalf("stderr = %q, want reviews group help guidance", stderr)
	}
	if strings.Contains(stderr, "asc reviews view --help") {
		t.Fatalf("stderr = %q, must not point to leaf help for a group flag", stderr)
	}
}

func TestRunRepeatedFlagNameParseFailurePointsToFailingLeaf(t *testing.T) {
	resetReportFlags(t)
	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"--profile", "work", "signing", "run", "--profile"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "asc signing run --help") {
		t.Fatalf("stderr = %q, want signing run help guidance", stderr)
	}
	if strings.Contains(stderr, "\n  asc --help") {
		t.Fatalf("stderr = %q, must not point to root help for the leaf flag", stderr)
	}
}

func TestRunEarlierRepeatedFlagParseFailureKeepsEarlierOwner(t *testing.T) {
	resetReportFlags(t)
	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"--version=maybe", "builds", "list", "--version", "1.2.3"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "asc --help") {
		t.Fatalf("stderr = %q, want root help guidance", stderr)
	}
	if strings.Contains(stderr, "asc builds list --help") {
		t.Fatalf("stderr = %q, later flag must not replace failing root owner", stderr)
	}
}

func TestRunRootFlagParseFailureIgnoresFlagMarkerInsideInvalidValue(t *testing.T) {
	resetReportFlags(t)
	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"--strict-auth=foo for -other", "builds", "list"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "asc --help") {
		t.Fatalf("stderr = %q, want root help guidance", stderr)
	}
	if strings.Contains(stderr, "asc builds list --help") {
		t.Fatalf("stderr = %q, must not point to leaf help for a root flag", stderr)
	}
}

func TestRunRootFlagParseFailureUsesRightmostMixedMarker(t *testing.T) {
	resetReportFlags(t)
	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"--strict-auth=foo for flag -limit: x", "builds", "list"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "asc --help") {
		t.Fatalf("stderr = %q, want root help guidance", stderr)
	}
	if strings.Contains(stderr, "asc builds list --help") {
		t.Fatalf("stderr = %q, embedded marker must not replace the diagnostic suffix", stderr)
	}
}

func TestRunRepeatedCustomFlagReasonDoesNotReplaceParserSuffix(t *testing.T) {
	resetReportFlags(t)
	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{
			"reviews", "--stars", "1", "--stars", "2 for flag -version: x", "view",
		}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "asc reviews --help") {
		t.Fatalf("stderr = %q, want reviews group help guidance", stderr)
	}
	if strings.Contains(stderr, "asc reviews view --help") {
		t.Fatalf("stderr = %q, reason text must not replace the parser suffix", stderr)
	}
}

func TestRunRepeatedFlagValuePointsToFailingLeaf(t *testing.T) {
	resetReportFlags(t)
	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{
			"reviews", "--stars", "1", "list", "--stars", "2", "--stars", "1",
		}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "asc reviews list --help") {
		t.Fatalf("stderr = %q, want reviews list help guidance", stderr)
	}
	if strings.Contains(stderr, "\n  asc reviews --help") {
		t.Fatalf("stderr = %q, earlier equal value must not replace the failing leaf owner", stderr)
	}
}

func TestRunParseFailurePreservesJSONAndJUnitContracts(t *testing.T) {
	resetReportFlags(t)
	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"builds", "list", "--output", "json", "--limit", "nope"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("JSON parse failure wrote stdout = %q, want empty", stdout)
	}
	if !strings.HasPrefix(stderr, "Error: invalid value") || !strings.Contains(stderr, "asc builds list --help") {
		t.Fatalf("unexpected JSON parse diagnostic: %q", stderr)
	}

	resetReportFlags(t)
	reportPath := filepath.Join(t.TempDir(), "parse.xml")
	stdout, stderr = captureCommandOutput(t, func() {
		if code := Run([]string{
			"--report", "junit", "--report-file", reportPath,
			"builds", "list", "--limit", "nope",
		}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("JUnit parse failure wrote stdout = %q, want empty", stdout)
	}
	if !strings.HasPrefix(stderr, "Error: invalid value") || !strings.Contains(stderr, "asc builds list --help") {
		t.Fatalf("unexpected JUnit parse diagnostic: %q", stderr)
	}
	if _, err := os.Stat(reportPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("generic parse failures must preserve the no-report contract, stat error = %v", err)
	}
}

func TestRunExplicitHelpStillUsesStdout(t *testing.T) {
	resetReportFlags(t)
	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"builds", "list", "--help"}, "1.0.0"); code != ExitSuccess {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	})
	if !strings.Contains(stdout, "DESCRIPTION") || !strings.Contains(stdout, "--paginate") {
		t.Fatalf("explicit help stdout = %q, want full command help", stdout)
	}
	if stderr != "" {
		t.Fatalf("explicit help stderr = %q, want empty", stderr)
	}
}

func TestRunKnownFlagParseFailureDoesNotChangeUsageClassification(t *testing.T) {
	resetReportFlags(t)
	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var gotExit int
	emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
		gotExit = exitCode
		if eventContext.FailureStage != telemetry.FailureStageParse || eventContext.OutcomeKind != telemetry.OutcomeUsageError {
			t.Errorf("telemetry context = %+v, want parse usage failure", eventContext)
		}
	}

	_, _ = captureCommandOutput(t, func() {
		if code := Run([]string{"builds", "list", "--limit", "nope"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if gotExit != ExitUsage {
		t.Fatalf("telemetry exit code = %d, want %d", gotExit, ExitUsage)
	}
}
