package cmd

import (
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

func TestRunAnalyticsLookupFailureOutcome(t *testing.T) {
	const requestID = "11111111-2222-3333-4444-555555555555"
	const segmentURL = "https://example.com/segment.csv.gz"

	tests := []struct {
		name          string
		args          []string
		instances     string
		segments      string
		wantExit      int
		wantStderr    string
		wantOutcome   telemetry.OutcomeKind
		wantDiagnosis shared.DiagnosticCode
		wantParameter string
	}{
		{
			name:          "download instance not found",
			args:          []string{"analytics", "download", "--request-id", requestID, "--instance-id", "inst-1"},
			instances:     `[{"type":"analyticsReportInstances","id":"inst-other"}]`,
			wantExit:      ExitNotFound,
			wantStderr:    fmt.Sprintf("Error: analytics download: instance %q not found for request %q", "inst-1", requestID),
			wantOutcome:   telemetry.OutcomeNotFound,
			wantDiagnosis: shared.DiagnosticResourceNotFound,
			wantParameter: "--instance-id",
		},
		{
			name:          "download no segments yet",
			args:          []string{"analytics", "download", "--request-id", requestID, "--instance-id", "inst-1"},
			instances:     `[{"type":"analyticsReportInstances","id":"inst-1"}]`,
			segments:      `[]`,
			wantExit:      ExitError,
			wantStderr:    fmt.Sprintf("Error: analytics download: no segments available for instance %q", "inst-1"),
			wantOutcome:   telemetry.OutcomeExpectedNegative,
			wantDiagnosis: shared.DiagnosticStateNotReady,
		},
		{
			name:          "download segment not found",
			args:          []string{"analytics", "download", "--request-id", requestID, "--instance-id", "inst-1", "--segment-id", "seg-missing"},
			instances:     `[{"type":"analyticsReportInstances","id":"inst-1"}]`,
			segments:      fmt.Sprintf(`[{"type":"analyticsReportSegments","id":"seg-1","attributes":{"url":%q}}]`, segmentURL),
			wantExit:      ExitNotFound,
			wantStderr:    fmt.Sprintf("Error: analytics download: segment %q not found for instance %q", "seg-missing", "inst-1"),
			wantOutcome:   telemetry.OutcomeNotFound,
			wantDiagnosis: shared.DiagnosticResourceNotFound,
			wantParameter: "--segment-id",
		},
		{
			name:          "download segment URL not ready",
			args:          []string{"analytics", "download", "--request-id", requestID, "--instance-id", "inst-1"},
			instances:     `[{"type":"analyticsReportInstances","id":"inst-1"}]`,
			segments:      `[{"type":"analyticsReportSegments","id":"seg-1","attributes":{"url":""}}]`,
			wantExit:      ExitError,
			wantStderr:    "Error: analytics download: segment download URL is empty",
			wantOutcome:   telemetry.OutcomeExpectedNegative,
			wantDiagnosis: shared.DiagnosticStateNotReady,
		},
		{
			name:          "view instance not found",
			args:          []string{"analytics", "view", "--request-id", requestID, "--instance-id", "inst-1"},
			instances:     `[{"type":"analyticsReportInstances","id":"inst-other"}]`,
			wantExit:      ExitNotFound,
			wantStderr:    fmt.Sprintf("Error: analytics view: instance %q not found for request %q", "inst-1", requestID),
			wantOutcome:   telemetry.OutcomeNotFound,
			wantDiagnosis: shared.DiagnosticResourceNotFound,
			wantParameter: "--instance-id",
		},
		{
			name:          "view processing date has no instances",
			args:          []string{"analytics", "view", "--request-id", requestID, "--processing-date", "2026-01-15", "--paginate"},
			instances:     `[]`,
			wantExit:      ExitNotFound,
			wantStderr:    fmt.Sprintf("Error: analytics view: no instances found for processing date %q", "2026-01-15"),
			wantOutcome:   telemetry.OutcomeNotFound,
			wantDiagnosis: shared.DiagnosticResourceNotFound,
			wantParameter: "--processing-date",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetReportFlags(t)
			t.Chdir(t.TempDir())

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/analyticsReportRequests/"+requestID+"/reports":
					fmt.Fprint(w, `{"data":[{"type":"analyticsReports","id":"report-1","attributes":{"name":"App Downloads"}}],"links":{}}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/analyticsReports/report-1/instances":
					fmt.Fprintf(w, `{"data":%s,"links":{}}`, test.instances)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/analyticsReportInstances/inst-1/segments" && test.segments != "":
					fmt.Fprintf(w, `{"data":%s,"links":{}}`, test.segments)
				default:
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
					w.WriteHeader(http.StatusInternalServerError)
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
					t.Errorf("Run() exit code = %d, want %d", code, test.wantExit)
				}
			})

			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if strings.TrimSpace(stderr) != test.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if gotExitCode != test.wantExit ||
				gotContext.OutcomeKind != test.wantOutcome ||
				gotContext.DiagnosticCode != string(test.wantDiagnosis) ||
				gotContext.FailureParameter != test.wantParameter {
				t.Fatalf("telemetry exit=%d outcome=%q diagnostic=%q parameter=%q, want exit=%d outcome=%q diagnostic=%q parameter=%q",
					gotExitCode, gotContext.OutcomeKind, gotContext.DiagnosticCode, gotContext.FailureParameter,
					test.wantExit, test.wantOutcome, test.wantDiagnosis, test.wantParameter)
			}
		})
	}
}
