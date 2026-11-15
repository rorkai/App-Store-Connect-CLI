package cmd

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

func TestRunListValidationErrorsAreUsageAndSkipASCClient(t *testing.T) {
	tests := []struct {
		name               string
		args               []string
		wantError          string
		wantParameter      string
		wantDiagnosticCode string
	}{
		{
			name:          "categories invalid limit",
			args:          []string{"categories", "list", "--limit", "0"},
			wantError:     "categories list: --limit must be between 1 and 200",
			wantParameter: "--limit",
		},
		{
			name:          "devices invalid status",
			args:          []string{"devices", "list", "--status", "MAYBE"},
			wantError:     "devices list: --status must be one of: ENABLED, DISABLED",
			wantParameter: "--status",
		},
		{
			name:          "app tags invalid visibility",
			args:          []string{"app-tags", "list", "--app", "123", "--visible-in-app-store", "maybe"},
			wantError:     "app-tags list: --visible-in-app-store must be true or false",
			wantParameter: "--visible-in-app-store",
		},
		{
			name:               "app tags territory fields without include",
			args:               []string{"app-tags", "list", "--app", "123456789", "--territory-fields", "currency"},
			wantError:          "--territory-fields requires --include territories",
			wantParameter:      "--territory-fields",
			wantDiagnosticCode: string(shared.DiagnosticConflictingInput),
		},
		{
			name:               "app tags territory limit without include",
			args:               []string{"app-tags", "list", "--app", "123456789", "--territory-limit", "5"},
			wantError:          "--territory-limit requires --include territories",
			wantParameter:      "--territory-limit",
			wantDiagnosticCode: string(shared.DiagnosticConflictingInput),
		},
		{
			name:               "apps invalid limit",
			args:               []string{"apps", "list", "--limit", "201"},
			wantError:          "apps list: --limit must be between 1 and 200",
			wantParameter:      "--limit",
			wantDiagnosticCode: string(shared.DiagnosticInvalidInput),
		},
		{
			name:               "builds invalid limit",
			args:               []string{"builds", "list", "--limit", "201"},
			wantError:          "builds list: --limit must be between 1 and 200",
			wantParameter:      "--limit",
			wantDiagnosticCode: string(shared.DiagnosticInvalidInput),
		},
		{
			name:               "apps next conflict",
			args:               []string{"apps", "list", "--next", "https://api.appstoreconnect.apple.com/v1/apps?cursor=next", "--limit", "50"},
			wantError:          "--next cannot be combined with --limit",
			wantParameter:      "--limit",
			wantDiagnosticCode: string(shared.DiagnosticConflictingInput),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetReportFlags(t)

			clientFactoryCalled := false
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("ASC client factory must not run during list validation")
			}))

			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })

			var gotExitCode int
			var gotContext telemetry.EventContext
			emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
				gotExitCode = exitCode
				gotContext = eventContext
			}

			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "1.0.0"); code != ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
				}
			})

			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.wantError) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantError)
			}
			if gotExitCode != ExitUsage {
				t.Fatalf("telemetry exit code = %d, want %d", gotExitCode, ExitUsage)
			}
			if gotContext.ErrorKind != telemetry.ErrorKindInvalidValue ||
				gotContext.FailureStage != telemetry.FailureStageValidation ||
				gotContext.OutcomeKind != telemetry.OutcomeUsageError ||
				gotContext.FailureParameter != test.wantParameter ||
				gotContext.DiagnosticCode != test.wantDiagnosticCode {
				t.Fatalf("unexpected telemetry context: %+v", gotContext)
			}
			if clientFactoryCalled {
				t.Fatal("ASC client factory ran before list validation")
			}
		})
	}
}
