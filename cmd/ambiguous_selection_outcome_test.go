package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

func TestRunAmbiguousSelectionsAreClassified(t *testing.T) {
	const twoPlatforms = `{"data":[` +
		`{"type":"appStoreVersions","id":"ver-ios","attributes":{"platform":"IOS","versionString":"1.0","appVersionState":"PREPARE_FOR_SUBMISSION","appStoreState":"PREPARE_FOR_SUBMISSION"}},` +
		`{"type":"appStoreVersions","id":"ver-mac","attributes":{"platform":"MAC_OS","versionString":"1.0","appVersionState":"PREPARE_FOR_SUBMISSION","appStoreState":"PREPARE_FOR_SUBMISSION"}}` +
		`],"links":{}}`
	const twoApps = `{"data":[` +
		`{"type":"apps","id":"app-1","attributes":{"name":"Widget","bundleId":"com.example.widget"}},` +
		`{"type":"apps","id":"app-2","attributes":{"name":"Widget","bundleId":"com.example.widget2"}}` +
		`],"links":{}}`
	tests := []struct {
		name          string
		args          []string
		wantStderr    string
		wantParameter string
	}{
		{
			name:          "app name matches several apps",
			args:          []string{"review", "status", "--app", "Widget"},
			wantStderr:    `2 apps match "Widget"; pass --app with one of:`,
			wantParameter: "--app",
		},
		{
			name:          "version string matches several platforms",
			args:          []string{"review", "status", "--app", "123456789", "--version", "1.0"},
			wantStderr:    `pass --platform with one of:`,
			wantParameter: "--platform",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetReportFlags(t)
			t.Setenv("ASC_APP_ID", "")

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/appStoreVersions":
					_, _ = w.Write([]byte(twoPlatforms))
					return
				case req.Method == http.MethodGet && req.URL.Path == "/v1/apps":
					_, _ = w.Write([]byte(twoApps))
					return
				}
				t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
				w.WriteHeader(http.StatusNotFound)
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
				if code := Run(test.args, "4.0.0"); code != ExitError {
					t.Errorf("Run() exit code = %d, want %d", code, ExitError)
				}
			})

			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if gotExitCode != ExitError ||
				gotContext.OutcomeKind != telemetry.OutcomeExpectedNegative ||
				gotContext.FailureStage != telemetry.FailureStageValidation ||
				gotContext.DiagnosticCode != string(shared.DiagnosticInvalidInput) ||
				gotContext.FailureParameter != test.wantParameter {
				t.Fatalf("telemetry exit=%d context=%+v, want expected_negative validation invalid_input on %s", gotExitCode, gotContext, test.wantParameter)
			}
		})
	}
}
