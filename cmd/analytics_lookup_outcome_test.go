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
		noInstance    bool
		wantExit      int
		wantStderr    string
		wantOutcome   telemetry.OutcomeKind
		wantDiagnosis shared.DiagnosticCode
		wantParameter string
	}{
		{
			name:          "download instance not found",
			args:          []string{"analytics", "download", "--request-id", requestID, "--instance-id", "inst-1"},
			noInstance:    true,
			wantExit:      ExitNotFound,
			wantStderr:    fmt.Sprintf("Error: analytics download: instance %q not found", "inst-1"),
			wantOutcome:   telemetry.OutcomeNotFound,
			wantDiagnosis: shared.DiagnosticResourceNotFound,
			wantParameter: "--instance-id",
		},
		{
			name:          "download no segments yet",
			args:          []string{"analytics", "download", "--request-id", requestID, "--instance-id", "inst-1"},
			segments:      `[]`,
			wantExit:      ExitError,
			wantStderr:    fmt.Sprintf("Error: analytics download: no segments available for instance %q", "inst-1"),
			wantOutcome:   telemetry.OutcomeExpectedNegative,
			wantDiagnosis: shared.DiagnosticStateNotReady,
		},
		{
			name:          "download segment not found",
			args:          []string{"analytics", "download", "--request-id", requestID, "--instance-id", "inst-1", "--segment-id", "seg-missing"},
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
				case req.Method == http.MethodGet && req.URL.Path == "/v1/analyticsReportInstances/inst-1/segments" && test.noInstance:
					w.WriteHeader(http.StatusNotFound)
					// Apple's live response for an unknown instance (GET
					// /v1/analyticsReportInstances/{id}/segments, 2026-11-22).
					fmt.Fprint(w, `{"errors":[{"id":"c9f59569-a500-44b6-beeb-89596c027400","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'analyticsReportInstances' with id 'inst-1'"}]}`)
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

func TestRunAnalyticsInstanceLookupHint(t *testing.T) {
	tests := []struct {
		status  int
		code    string
		detail  string
		exit    int
		outcome telemetry.OutcomeKind
		hint    string
	}{
		{
			status:  http.StatusNotFound,
			code:    "NOT_FOUND",
			detail:  "There is no resource of type 'analyticsReportInstances' with id 'report-1'",
			exit:    ExitNotFound,
			outcome: telemetry.OutcomeNotFound,
			hint:    "Find current instance IDs with `asc analytics reports links --report-id REPORT_ID --paginate --output json` (use `data[].id`) or `asc analytics view --request-id REQUEST_ID --paginate --output json` (use `data[].instances[].id`).",
		},
		{
			status:  http.StatusForbidden,
			code:    "FORBIDDEN",
			detail:  "The API key does not have access to this resource",
			exit:    ExitAuth,
			outcome: telemetry.OutcomeAuthError,
			hint:    "Check that your API key has the right role/permissions for this operation in App Store Connect.",
		},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			resetReportFlags(t)
			t.Chdir(t.TempDir())
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodGet || req.URL.RequestURI() != "/v1/analyticsReportInstances/report-1" {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL)
					http.NotFound(w, req)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.status)
				fmt.Fprintf(w, `{"errors":[{"status":"%d","code":%q,"detail":%q}]}`, test.status, test.code, test.detail)
			}))
			t.Cleanup(server.Close)
			client := newHTTPStatusTestClient(t, server.URL)
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
			var gotContext telemetry.EventContext
			emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, eventContext telemetry.EventContext) {
				gotContext = eventContext
			}
			stdout, stderr := captureCommandOutput(t, func() {
				args := []string{"analytics", "instances", "view", "--instance-id", "report-1", "--output", "json"}
				if code := Run(args, "7.3.1"); code != test.exit {
					t.Errorf("Run() exit code = %d, want %d", code, test.exit)
				}
			})
			if stdout != "" || !strings.Contains(stderr, test.detail) || !strings.HasSuffix(stderr, "\nHint: "+test.hint+"\n") {
				t.Errorf("stdout = %q, stderr = %q, want empty stdout, Apple detail %q and hint %q", stdout, stderr, test.detail, test.hint)
			}
			if gotContext.HTTPStatus != test.status || gotContext.OutcomeKind != test.outcome || gotContext.FailureStage != telemetry.FailureStageRequest {
				t.Errorf("telemetry = %+v, want HTTP %d and outcome %q at request stage", gotContext, test.status, test.outcome)
			}
		})
	}
}
