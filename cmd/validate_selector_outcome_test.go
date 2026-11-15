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

func TestRunValidateSelectorFailuresAreClassified(t *testing.T) {
	const (
		listing   = `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0","appVersionState":"PREPARE_FOR_SUBMISSION"}}],"links":{}}`
		ambiguous = `{"data":[{"type":"appStoreVersions","id":"ver-ios","attributes":{"platform":"IOS","versionString":"1.0"}},{"type":"appStoreVersions","id":"ver-mac","attributes":{"platform":"MAC_OS","versionString":"1.0"}}],"links":{}}`
		otherApp  = `{"data":{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0"},"relationships":{"app":{"data":{"type":"apps","id":"app-2"}}}}}`
	)
	tests := []struct {
		name          string
		args          []string
		filtered      string
		wantCode      int
		wantStderr    string
		wantOutcome   telemetry.OutcomeKind
		wantDiagnosis string
		wantParameter string
	}{
		{
			name:          "explicit version not found",
			args:          []string{"--version", "9.9"},
			filtered:      `{"data":[],"links":{}}`,
			wantCode:      ExitError,
			wantStderr:    `app store version not found for version "9.9"`,
			wantOutcome:   telemetry.OutcomeExpectedNegative,
			wantDiagnosis: "resource_not_found",
			wantParameter: "--version",
		},
		{
			name:          "explicit version ambiguous",
			args:          []string{"--version", "1.0"},
			filtered:      ambiguous,
			wantCode:      ExitUsage,
			wantStderr:    `Error: validate: 2 app store versions match version "1.0"; pass --platform with one of:`,
			wantOutcome:   telemetry.OutcomeUsageError,
			wantDiagnosis: "invalid_input",
			wantParameter: "--platform",
		},
		{
			name:          "version id from another app",
			args:          []string{"--version-id", "ver-1"},
			wantCode:      ExitError,
			wantStderr:    `version "ver-1" belongs to app "app-2", not "app-1"`,
			wantOutcome:   telemetry.OutcomeExpectedNegative,
			wantDiagnosis: "invalid_input",
			wantParameter: "--version-id",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetReportFlags(t)
			t.Setenv("ASC_APP_ID", "")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case req.URL.Path == "/v1/apps/app-1/appStoreVersions" && req.URL.Query().Get("filter[versionString]") != "":
					fmt.Fprint(w, test.filtered)
				case req.URL.Path == "/v1/apps/app-1/appStoreVersions":
					fmt.Fprint(w, listing)
				case req.URL.Path == "/v1/appStoreVersions/ver-1":
					fmt.Fprint(w, otherApp)
				default:
					t.Errorf("unexpected request %s %s", req.Method, req.URL.String())
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			client := newHTTPStatusTestClient(t, server.URL)
			d := shared.DefaultDeps()
			d.Client = func() (*asc.Client, error) { return client, nil }

			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
			var gotContext telemetry.EventContext
			emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, eventContext telemetry.EventContext) {
				gotContext = eventContext
			}

			_, stderr := captureCommandOutput(t, func() {
				args := append([]string{"validate", "--app", "app-1"}, test.args...)
				if code := RunWithDeps(args, "4.0.0", d); code != test.wantCode {
					t.Errorf("Run() exit code = %d, want %d", code, test.wantCode)
				}
			})

			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if gotContext.OutcomeKind != test.wantOutcome ||
				gotContext.DiagnosticCode != test.wantDiagnosis ||
				gotContext.FailureParameter != test.wantParameter {
				t.Fatalf("telemetry context = %+v, want %s %s for %s", gotContext, test.wantOutcome, test.wantDiagnosis, test.wantParameter)
			}
		})
	}
}
