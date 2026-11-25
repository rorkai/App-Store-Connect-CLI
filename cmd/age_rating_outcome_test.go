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

func TestRunAgeRatingResolutionFailuresAreNotInternalErrors(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		appInfos      string
		wantExit      int
		wantStderr    string
		wantOutcome   telemetry.OutcomeKind
		wantDiagnosis shared.DiagnosticCode
		wantParameter string
	}{
		{
			name:          "view conflicting targets",
			args:          []string{"age-rating", "view", "--app-info-id", "info-1", "--version-id", "ver-1"},
			wantExit:      ExitUsage,
			wantStderr:    "only one of --app-info-id or --version-id is allowed",
			wantOutcome:   telemetry.OutcomeUsageError,
			wantDiagnosis: shared.DiagnosticConflictingInput,
			wantParameter: "--version-id",
		},
		{
			name:          "edit conflicting targets",
			args:          []string{"age-rating", "edit", "--app-info-id", "info-1", "--version-id", "ver-1", "--gambling", "true"},
			wantExit:      ExitUsage,
			wantStderr:    "only one of --app-info-id or --version-id is allowed",
			wantOutcome:   telemetry.OutcomeUsageError,
			wantDiagnosis: shared.DiagnosticConflictingInput,
			wantParameter: "--version-id",
		},
		{
			name:          "ambiguous app info",
			args:          []string{"age-rating", "view", "--app", "100000001"},
			appInfos:      `[{"type":"appInfos","id":"info-1","attributes":{"state":"READY_FOR_DISTRIBUTION"}},{"type":"appInfos","id":"info-2","attributes":{"state":"WAITING_FOR_REVIEW"}}]`,
			wantExit:      ExitUsage,
			wantStderr:    "2 app infos match app \"100000001\"; pass --app-info-id with one of:",
			wantOutcome:   telemetry.OutcomeUsageError,
			wantDiagnosis: shared.DiagnosticConflictingInput,
			wantParameter: "--app-info-id",
		},
		{
			name:          "no current app info",
			args:          []string{"age-rating", "view", "--app", "100000001"},
			appInfos:      `[{"type":"appInfos","id":"info-1","attributes":{"state":"REPLACED_WITH_NEW_INFO"}}]`,
			wantExit:      ExitError,
			wantStderr:    `no current app info found for app "100000001"`,
			wantOutcome:   telemetry.OutcomeExpectedNegative,
			wantDiagnosis: shared.DiagnosticStateNotReady,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetReportFlags(t)
			t.Setenv("ASC_APP_ID", "")

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if test.appInfos == "" || req.Method != http.MethodGet || req.URL.Path != "/v1/apps/100000001/appInfos" {
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"data":%s}`, test.appInfos)
			}))
			defer server.Close()

			client := newHTTPStatusTestClient(t, server.URL)
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
			var gotContext telemetry.EventContext
			emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, eventContext telemetry.EventContext) {
				gotContext = eventContext
			}

			_, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "4.0.0"); code != test.wantExit {
					t.Fatalf("Run() exit code = %d, want %d", code, test.wantExit)
				}
			})

			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if gotContext.OutcomeKind != test.wantOutcome ||
				gotContext.DiagnosticCode != string(test.wantDiagnosis) ||
				gotContext.FailureParameter != test.wantParameter {
				t.Fatalf("telemetry context = %+v, want %s %s %q", gotContext, test.wantOutcome, test.wantDiagnosis, test.wantParameter)
			}
		})
	}
}
