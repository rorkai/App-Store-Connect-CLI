package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

func TestRunXcodeCloudWaitCompletionOutcome(t *testing.T) {
	statusArgs := []string{"xcode-cloud", "status", "--run-id", "run-1", "--wait", "--poll-interval", "1ms", "--output", "json"}
	runArgs := []string{"xcode-cloud", "run", "--workflow-id", "wf-1", "--git-reference-id", "ref-1", "--wait", "--poll-interval", "1ms", "--output", "json"}
	stateNotReady := string(shared.DiagnosticStateNotReady)

	tests := []struct {
		name           string
		args           []string
		progress       string
		completion     string
		wantExit       int
		wantStderr     string
		wantResult     bool
		wantOutcome    telemetry.OutcomeKind
		wantDiagnostic string
	}{
		{name: "status failed", args: statusArgs, progress: "COMPLETE", completion: "FAILED", wantExit: ExitError, wantStderr: "build run run-1 completed with status: FAILED", wantResult: true, wantOutcome: telemetry.OutcomeExpectedNegative, wantDiagnostic: stateNotReady},
		{name: "status errored", args: statusArgs, progress: "COMPLETE", completion: "ERRORED", wantExit: ExitError, wantStderr: "build run run-1 completed with status: ERRORED", wantResult: true, wantOutcome: telemetry.OutcomeExpectedNegative, wantDiagnostic: stateNotReady},
		{name: "status canceled", args: statusArgs, progress: "COMPLETE", completion: "CANCELED", wantExit: ExitError, wantStderr: "build run run-1 completed with status: CANCELED", wantResult: true, wantOutcome: telemetry.OutcomeExpectedNegative, wantDiagnostic: stateNotReady},
		{name: "status skipped", args: statusArgs, progress: "COMPLETE", completion: "SKIPPED", wantExit: ExitError, wantStderr: "build run run-1 completed with status: SKIPPED", wantResult: true, wantOutcome: telemetry.OutcomeExpectedNegative, wantDiagnostic: stateNotReady},
		{name: "run failed", args: runArgs, progress: "COMPLETE", completion: "FAILED", wantExit: ExitError, wantStderr: "build run run-1 completed with status: FAILED", wantResult: true, wantOutcome: telemetry.OutcomeExpectedNegative, wantDiagnostic: stateNotReady},
		{name: "status succeeded", args: statusArgs, progress: "COMPLETE", completion: "SUCCEEDED", wantExit: ExitSuccess, wantResult: true},
		{name: "status timeout", args: append(append([]string{}, statusArgs...), "--timeout", "50ms"), progress: "RUNNING", wantExit: ExitError, wantStderr: "timed out waiting for build run run-1", wantOutcome: telemetry.OutcomeTransportError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetReportFlags(t)
			t.Setenv("ASC_APP_ID", "")

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case req.Method == http.MethodPost && req.URL.Path == "/v1/ciBuildRuns":
					w.WriteHeader(http.StatusCreated)
					fmt.Fprint(w, `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"number":7,"executionProgress":"PENDING"}}}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/ciBuildRuns/run-1":
					fmt.Fprintf(w, `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"number":7,"executionProgress":%q,"completionStatus":%q}}}`, test.progress, test.completion)
				default:
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			client := newHTTPStatusTestClient(t, server.URL)
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
			var gotExitCode int
			var gotContext telemetry.EventContext
			emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
				gotExitCode = exitCode
				gotContext = eventContext
			}

			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "5.15.0"); code != test.wantExit {
					t.Fatalf("Run() exit code = %d, want %d", code, test.wantExit)
				}
			})

			if test.wantResult {
				var result asc.XcodeCloudStatusResult
				if err := json.Unmarshal([]byte(stdout), &result); err != nil {
					t.Fatalf("stdout = %q, want JSON status result: %v", stdout, err)
				}
				if result.BuildRunID != "run-1" || result.CompletionStatus != test.completion {
					t.Fatalf("unexpected status result: %+v", result)
				}
			} else if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if test.wantStderr == "" {
				if strings.Contains(stderr, "Error:") {
					t.Fatalf("stderr = %q, want no error", stderr)
				}
			} else if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if gotExitCode != test.wantExit ||
				gotContext.OutcomeKind != test.wantOutcome ||
				gotContext.DiagnosticCode != test.wantDiagnostic {
				t.Fatalf("unexpected telemetry: exit=%d context=%+v", gotExitCode, gotContext)
			}
		})
	}
}
