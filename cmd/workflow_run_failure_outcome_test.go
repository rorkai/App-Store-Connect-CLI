package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

func TestRunWorkflowRunFailuresAreClassified(t *testing.T) {
	const twoWorkflows = `{"workflows":{"fail":{"steps":["echo hi","false"]},"slow":{"steps":[{"run":"sleep 5","timeout":"100ms"}]}}}`

	tests := []struct {
		name           string
		file           string
		noFile         bool
		args           []string
		resumeFrom     string
		wantStderr     string
		rejectStderr   string
		wantResult     bool
		wantOutcome    telemetry.OutcomeKind
		wantDiagnostic shared.DiagnosticCode
		wantParameter  string
	}{
		{
			name:           "failing run step",
			file:           twoWorkflows,
			args:           []string{"fail"},
			wantStderr:     "hi\nError: workflow: fail step 2: exit status 1\n",
			wantResult:     true,
			wantOutcome:    telemetry.OutcomeExpectedNegative,
			wantDiagnostic: shared.DiagnosticDependencyFailed,
		},
		{
			name:           "failing run step after retries",
			file:           `{"workflows":{"flaky":{"steps":[{"run":"false","retry":{"max_attempts":2,"delay":"1ms"}}]}}}`,
			args:           []string{"flaky"},
			wantStderr:     "Error: workflow: flaky step 1: exit status 1\n",
			wantResult:     true,
			wantOutcome:    telemetry.OutcomeExpectedNegative,
			wantDiagnostic: shared.DiagnosticDependencyFailed,
		},
		{
			name:           "run step with invalid declared outputs",
			file:           `{"workflows":{"out":{"steps":[{"name":"s","run":"echo notjson","outputs":{"X":"$.x"}}]}}}`,
			args:           []string{"out"},
			wantStderr:     "Error: workflow: out step 1: extract outputs: parse command stdout as JSON:",
			wantResult:     true,
			wantOutcome:    telemetry.OutcomeExpectedNegative,
			wantDiagnostic: shared.DiagnosticDependencyFailed,
		},
		{
			name:           "failing before_all hook",
			file:           `{"before_all":"false","workflows":{"w":{"steps":["echo hi"]}}}`,
			args:           []string{"w"},
			wantStderr:     "Error: workflow: before_all hook failed: exit status 1\n",
			wantResult:     true,
			wantOutcome:    telemetry.OutcomeExpectedNegative,
			wantDiagnostic: shared.DiagnosticDependencyFailed,
		},
		{
			name:           "failing after_all hook",
			file:           `{"after_all":"false","workflows":{"w":{"steps":["true"]}}}`,
			args:           []string{"w"},
			wantStderr:     "Error: workflow: after_all hook failed: exit status 1\n",
			wantResult:     true,
			wantOutcome:    telemetry.OutcomeExpectedNegative,
			wantDiagnostic: shared.DiagnosticDependencyFailed,
		},
		{
			name:           "run step timeout without retry",
			file:           twoWorkflows,
			args:           []string{"slow"},
			wantStderr:     "Error: workflow: slow step 1: step timed out after 100ms\n",
			rejectStderr:   "ASC_TIMEOUT",
			wantResult:     true,
			wantOutcome:    telemetry.OutcomeExpectedNegative,
			wantDiagnostic: shared.DiagnosticDependencyFailed,
		},
		{
			name:           "unknown workflow",
			file:           twoWorkflows,
			args:           []string{"nope"},
			wantStderr:     "Error: workflow run: workflow: unknown workflow \"nope\"\n",
			wantOutcome:    telemetry.OutcomeExpectedNegative,
			wantDiagnostic: shared.DiagnosticResourceNotFound,
		},
		{
			name:           "private workflow",
			file:           `{"workflows":{"helper":{"private":true,"steps":["echo hi"]}}}`,
			args:           []string{"helper"},
			wantStderr:     "Error: workflow run: workflow: \"helper\" is private and cannot be run directly\n",
			wantOutcome:    telemetry.OutcomeExpectedNegative,
			wantDiagnostic: shared.DiagnosticInvalidInput,
		},
		{
			name:           "unknown resume run",
			file:           twoWorkflows,
			args:           []string{"fail", "--resume", "fail-20260101T000000Z-deadbeef"},
			wantStderr:     "Error: workflow: resume run \"fail-20260101T000000Z-deadbeef\" not found\n",
			wantResult:     true,
			wantOutcome:    telemetry.OutcomeExpectedNegative,
			wantDiagnostic: shared.DiagnosticResourceNotFound,
			wantParameter:  "--resume",
		},
		{
			name:           "terminal resume run",
			file:           twoWorkflows,
			args:           []string{"slow"},
			resumeFrom:     "slow",
			wantStderr:     "cannot be resumed",
			wantResult:     true,
			wantOutcome:    telemetry.OutcomeExpectedNegative,
			wantDiagnostic: shared.DiagnosticInvalidInput,
			wantParameter:  "--resume",
		},
		{
			name:           "missing workflow file",
			noFile:         true,
			args:           []string{"fail"},
			wantStderr:     "Error: workflow run: read workflow: open ",
			wantOutcome:    telemetry.OutcomeExpectedNegative,
			wantDiagnostic: shared.DiagnosticFileNotFound,
			wantParameter:  "--file",
		},
		{
			name:           "malformed workflow file",
			file:           `{"workflows":`,
			args:           []string{"fail"},
			wantStderr:     "Error: workflow run: parse workflow JSON: unexpected EOF\n",
			wantOutcome:    telemetry.OutcomeExpectedNegative,
			wantDiagnostic: shared.DiagnosticFileInvalidFormat,
			wantParameter:  "--file",
		},
		{
			name:           "invalid workflow definition",
			file:           `{"workflows":{"a":{"steps":[{"workflow":"missing"}]}}}`,
			args:           []string{"a"},
			wantStderr:     "Error: workflow run: workflow \"a\" step 1 references unknown workflow \"missing\"\n",
			wantOutcome:    telemetry.OutcomeExpectedNegative,
			wantDiagnostic: shared.DiagnosticFileInvalidFormat,
			wantParameter:  "--file",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetReportFlags(t)
			dir := t.TempDir()
			t.Setenv("HOME", dir)
			path := filepath.Join(dir, ".asc", "workflow.json")
			if !test.noFile {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(test.file), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			args := append([]string{"workflow", "run", "--file", path}, test.args...)
			if test.resumeFrom != "" {
				priorStdout, _ := captureCommandOutput(t, func() {
					Run([]string{"workflow", "run", "--file", path, test.resumeFrom}, "5.15.0")
				})
				var prior struct {
					RunID string `json:"run_id"`
				}
				if err := json.Unmarshal([]byte(priorStdout), &prior); err != nil || prior.RunID == "" {
					t.Fatalf("prior run stdout = %q, want run_id: %v", priorStdout, err)
				}
				args = append(args, "--resume", prior.RunID)
			}

			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
			var gotExitCode int
			var gotContext telemetry.EventContext
			emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
				gotExitCode = exitCode
				gotContext = eventContext
			}

			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(args, "5.15.0"); code != ExitError {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitError)
				}
			})

			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if test.rejectStderr != "" && strings.Contains(stderr, test.rejectStderr) {
				t.Fatalf("stderr = %q, must not contain %q", stderr, test.rejectStderr)
			}
			if test.wantResult {
				var result struct {
					Status string `json:"status"`
					Error  string `json:"error"`
				}
				if err := json.Unmarshal([]byte(stdout), &result); err != nil {
					t.Fatalf("stdout = %q, want JSON run result: %v", stdout, err)
				}
				if result.Status != "error" || result.Error == "" {
					t.Fatalf("unexpected run result: %+v", result)
				}
			} else if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if gotExitCode != ExitError ||
				gotContext.OutcomeKind != test.wantOutcome ||
				gotContext.DiagnosticCode != string(test.wantDiagnostic) ||
				gotContext.FailureParameter != test.wantParameter {
				t.Fatalf("unexpected telemetry: exit=%d context=%+v", gotExitCode, gotContext)
			}
		})
	}
}
