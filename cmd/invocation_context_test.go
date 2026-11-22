package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/itunes"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

func TestRuntimeFailureContextClassifiesItunesHTTPStatus(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		wantKind    telemetry.ErrorKind
		wantOutcome telemetry.OutcomeKind
	}{
		{
			name:        "public unauthorized response",
			statusCode:  http.StatusUnauthorized,
			wantKind:    telemetry.ErrorKindOther,
			wantOutcome: telemetry.OutcomeAPIClientError,
		},
		{
			name:        "public forbidden response",
			statusCode:  http.StatusForbidden,
			wantKind:    telemetry.ErrorKindOther,
			wantOutcome: telemetry.OutcomeAPIClientError,
		},
		{
			name:        "client error",
			statusCode:  http.StatusTooManyRequests,
			wantKind:    telemetry.ErrorKindOther,
			wantOutcome: telemetry.OutcomeAPIClientError,
		},
		{
			name:        "server error",
			statusCode:  http.StatusServiceUnavailable,
			wantKind:    telemetry.ErrorKindAPI5xx,
			wantOutcome: telemetry.OutcomeAPIServerError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.statusCode)
			}))
			defer server.Close()

			client := &itunes.Client{BaseURL: server.URL, HTTPClient: server.Client()}
			_, err := client.SearchApps(context.Background(), "focus", "us", 20)
			if err == nil {
				t.Fatal("expected non-success response error")
			}

			got := runtimeFailureContext(
				invocationAnalysis{shape: telemetry.InvocationShapeLeaf},
				err,
				ExitError,
			)
			if got.ErrorKind != test.wantKind || got.FailureStage != telemetry.FailureStageRequest ||
				got.OutcomeKind != test.wantOutcome || got.HTTPStatus != test.statusCode || !got.PublicStorefront {
				t.Fatalf(
					"runtimeFailureContext() = %+v, want kind=%q stage=%q outcome=%q status=%d public storefront",
					got,
					test.wantKind,
					telemetry.FailureStageRequest,
					test.wantOutcome,
					test.statusCode,
				)
			}
		})
	}
}

func TestRuntimeFailureContextClassifiesLowCardinalityFailures(t *testing.T) {
	analysis := invocationAnalysis{shape: telemetry.InvocationShapeLeaf}
	tests := []struct {
		name        string
		err         error
		exitCode    int
		wantKind    telemetry.ErrorKind
		wantStage   telemetry.FailureStage
		wantOutcome telemetry.OutcomeKind
		wantStatus  int
	}{
		{
			name:        "missing auth is local validation",
			err:         shared.ErrMissingAuth,
			exitCode:    ExitAuth,
			wantKind:    telemetry.ErrorKindOther,
			wantStage:   telemetry.FailureStageValidation,
			wantOutcome: telemetry.OutcomeAuthError,
		},
		{
			name:        "missing Apple web session is a missing-input usage failure",
			err:         fmt.Errorf("web review show failed: %w", &shared.MissingWebSessionError{Message: "no Apple web session is cached"}),
			exitCode:    ExitUsage,
			wantKind:    telemetry.ErrorKindMissingRequired,
			wantStage:   telemetry.FailureStageValidation,
			wantOutcome: telemetry.OutcomeUsageError,
		},
		{
			name:        "invalid Apple Account credentials",
			err:         fmt.Errorf("SRP login failed: %w", webcore.ErrInvalidAppleAccountCredentials),
			exitCode:    ExitError,
			wantKind:    telemetry.ErrorKindOther,
			wantStage:   telemetry.FailureStageExecution,
			wantOutcome: telemetry.OutcomeAuthError,
		},
		{
			name:        "reported validation failure",
			err:         shared.NewValidationReportedError(errors.New("found blocking issues")),
			exitCode:    ExitError,
			wantKind:    telemetry.ErrorKindOther,
			wantStage:   telemetry.FailureStageValidation,
			wantOutcome: telemetry.OutcomeExpectedNegative,
		},
		{
			name:        "reported missing usage failure",
			err:         shared.NewReportedUsageError(shared.UsageErrorMissingRequired, "--territory is required"),
			exitCode:    ExitUsage,
			wantKind:    telemetry.ErrorKindMissingRequired,
			wantStage:   telemetry.FailureStageValidation,
			wantOutcome: telemetry.OutcomeUsageError,
		},
		{
			name:        "reported invalid usage failure",
			err:         shared.NewReportedUsageError(shared.UsageErrorInvalidValue, "invalid value for --territory"),
			exitCode:    ExitUsage,
			wantKind:    telemetry.ErrorKindInvalidValue,
			wantStage:   telemetry.FailureStageValidation,
			wantOutcome: telemetry.OutcomeUsageError,
		},
		{
			name:        "API conflict",
			err:         errors.New("conflict"),
			exitCode:    ExitConflict,
			wantKind:    telemetry.ErrorKindAPIConflict,
			wantStage:   telemetry.FailureStageRequest,
			wantOutcome: telemetry.OutcomeConflict,
		},
		{
			name:        "API server failure",
			err:         errors.New("server failure"),
			exitCode:    ExitHTTPInternalServer,
			wantKind:    telemetry.ErrorKindAPI5xx,
			wantStage:   telemetry.FailureStageRequest,
			wantOutcome: telemetry.OutcomeTransportError,
		},
		{
			name:        "API server failure at upper exit code boundary",
			err:         errors.New("server failure"),
			exitCode:    HTTPStatusToExitCode(599),
			wantKind:    telemetry.ErrorKindAPI5xx,
			wantStage:   telemetry.FailureStageRequest,
			wantOutcome: telemetry.OutcomeTransportError,
		},
		{
			name:        "exact API permission failure",
			err:         &asc.APIError{StatusCode: 403},
			exitCode:    ExitHTTPForbidden,
			wantKind:    telemetry.ErrorKindOther,
			wantStage:   telemetry.FailureStageRequest,
			wantOutcome: telemetry.OutcomeAuthError,
			wantStatus:  403,
		},
		{
			name:        "exact API server failure",
			err:         &asc.APIError{StatusCode: 503},
			exitCode:    ExitHTTPServiceUnavailable,
			wantKind:    telemetry.ErrorKindAPI5xx,
			wantStage:   telemetry.FailureStageRequest,
			wantOutcome: telemetry.OutcomeAPIServerError,
			wantStatus:  503,
		},
		{
			name:        "cancelled command",
			err:         context.Canceled,
			exitCode:    ExitError,
			wantKind:    telemetry.ErrorKindOther,
			wantStage:   telemetry.FailureStageExecution,
			wantOutcome: telemetry.OutcomeCancelled,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := runtimeFailureContext(analysis, test.err, test.exitCode)
			if got.ErrorKind != test.wantKind || got.FailureStage != test.wantStage ||
				got.OutcomeKind != test.wantOutcome || got.HTTPStatus != test.wantStatus {
				t.Fatalf(
					"runtimeFailureContext() = %+v, want kind=%q stage=%q outcome=%q status=%d",
					got,
					test.wantKind,
					test.wantStage,
					test.wantOutcome,
					test.wantStatus,
				)
			}
		})
	}
}

func TestRuntimeFailureContextDoesNotTreatLocalStaplerExitAsAPIStatus(t *testing.T) {
	analysis := invocationAnalysis{shape: telemetry.InvocationShapeLeaf}
	for _, code := range []int{3, 4, 5, 65, 66} {
		t.Run(fmt.Sprintf("exit-%d", code), func(t *testing.T) {
			cause := &localxcode.StaplerCommandError{
				Operation: string(localxcode.StaplerOperationValidate),
				ExitCode:  code,
				Err:       errors.New("local stapler failure"),
			}
			err := shared.NewProcessExitErrorWithCause(code, cause)
			var gotCause *localxcode.StaplerCommandError
			if !errors.As(err, &gotCause) || gotCause != cause {
				t.Fatalf("process exit error = %T %v, want preserved stapler cause", err, err)
			}
			got := runtimeFailureContext(analysis, err, code)
			if got.ErrorKind != telemetry.ErrorKindOther || got.FailureStage != telemetry.FailureStageExecution ||
				got.OutcomeKind != telemetry.OutcomeInternalError || got.HTTPStatus != 0 {
				t.Fatalf("runtimeFailureContext() = %+v, want local execution/internal failure", got)
			}
		})
	}
}

func TestRuntimeFailureContextKeepsInterruptedLocalStaplerExitOutOfAPIBuckets(t *testing.T) {
	analysis := invocationAnalysis{shape: telemetry.InvocationShapeLeaf}
	tests := []struct {
		name        string
		contextErr  error
		wantOutcome telemetry.OutcomeKind
	}{
		{name: "cancellation", contextErr: context.Canceled, wantOutcome: telemetry.OutcomeCancelled},
		{name: "deadline", contextErr: context.DeadlineExceeded, wantOutcome: telemetry.OutcomeTransportError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// A stapler child can return a concrete status in the same window in
			// which the caller's context is canceled, so the local status and the
			// context error travel together.
			cause := errors.Join(&localxcode.StaplerCommandError{
				Operation: string(localxcode.StaplerOperationStaple),
				ExitCode:  66,
				Err:       errors.New("local stapler failure"),
			}, test.contextErr)
			err := shared.NewProcessExitErrorWithCause(66, cause)
			got := runtimeFailureContext(analysis, err, 66)
			if got.ErrorKind != telemetry.ErrorKindOther || got.FailureStage != telemetry.FailureStageExecution || got.HTTPStatus != 0 {
				t.Fatalf("runtimeFailureContext() = %+v, want local execution failure without API classification", got)
			}
			if got.OutcomeKind != test.wantOutcome {
				t.Fatalf("runtimeFailureContext() outcome = %v, want %v", got.OutcomeKind, test.wantOutcome)
			}
		})
	}
}

func TestValidationFailureContextPrefersStructuredDiagnostic(t *testing.T) {
	err := shared.WithDiagnostic(
		shared.NewReportedUsageError(shared.UsageErrorMissingRequired, "--issuer-id is required"),
		shared.DiagnosticConflictingInput,
		"--key-id",
	)

	got := validationFailureContext(
		invocationAnalysis{shape: telemetry.InvocationShapeLeaf},
		err,
	)

	if got.ErrorKind != telemetry.ErrorKindInvalidValue ||
		got.FailureStage != telemetry.FailureStageValidation ||
		got.FailureParameter != "--key-id" ||
		got.DiagnosticCode != string(shared.DiagnosticConflictingInput) {
		t.Fatalf("validationFailureContext() = %+v", got)
	}
}

func TestValidationFailureContextPreservesUsageKindForUnmappedDiagnostic(t *testing.T) {
	err := shared.WithDiagnostic(
		shared.NewReportedUsageError(shared.UsageErrorInvalidValue, "auth login: invalid private key"),
		shared.DiagnosticFileNotFound,
		"--private-key",
	)

	got := validationFailureContext(
		invocationAnalysis{shape: telemetry.InvocationShapeLeaf},
		err,
	)

	if got.ErrorKind != telemetry.ErrorKindInvalidValue ||
		got.FailureParameter != "--private-key" ||
		got.DiagnosticCode != string(shared.DiagnosticFileNotFound) {
		t.Fatalf("validationFailureContext() = %+v", got)
	}
}

func TestValidationFailureContextRetainsLegacyMessageFallback(t *testing.T) {
	err := shared.NewReportedUsageError(shared.UsageErrorInvalidValue, "invalid value for --territory")

	got := validationFailureContext(
		invocationAnalysis{shape: telemetry.InvocationShapeLeaf},
		err,
	)

	if got.FailureParameter != "--territory" || got.DiagnosticCode != "" {
		t.Fatalf("validationFailureContext() = %+v", got)
	}
}

func TestRuntimeFailureContextCarriesStructuredDiagnostic(t *testing.T) {
	err := shared.WithDiagnostic(
		shared.NewValidationError(errors.New("private key could not be loaded")),
		shared.DiagnosticFileNotFound,
		"--private-key",
	)

	got := runtimeFailureContext(
		invocationAnalysis{shape: telemetry.InvocationShapeLeaf},
		err,
		ExitError,
	)

	if got.FailureStage != telemetry.FailureStageValidation ||
		got.OutcomeKind != telemetry.OutcomeExpectedNegative ||
		got.FailureParameter != "--private-key" ||
		got.DiagnosticCode != string(shared.DiagnosticFileNotFound) {
		t.Fatalf("runtimeFailureContext() = %+v", got)
	}
}

func TestPendingWaitEmitsExpectedNegativeStateNotReady(t *testing.T) {
	err := shared.NewPendingError("builds wait: Build is still pending after 50s")
	eventContext := runtimeFailureContext(
		invocationAnalysis{shape: telemetry.InvocationShapeLeaf},
		err,
		ExitCodeFromError(err),
	)

	event, ok := telemetry.BuildEventWithContext("asc builds wait", "1.0.0", 0, ExitPending, eventContext)
	if !ok {
		t.Fatal("BuildEventWithContext() returned no event")
	}
	if event.OutcomeKind != telemetry.OutcomeExpectedNegative ||
		event.DiagnosticCode == nil || *event.DiagnosticCode != string(shared.DiagnosticStateNotReady) {
		t.Fatalf("event outcome=%q diagnostic=%v, want expected_negative with state_not_ready", event.OutcomeKind, event.DiagnosticCode)
	}
}

func TestRuntimeFailureContextKeepsExplainedConflictAsAPIConflict(t *testing.T) {
	// Apple's 409 for a past start date on POST /v1/appPriceSchedules, captured
	// live on 2026-09-26. The pricing command explains it and adds a
	// diagnostic; the event must stay an API conflict with the 409 status.
	body := `{"errors":[{"id":"4e33e336-382f-4c14-b0c6-76e89677419c","status":"409","code":"ENTITY_ERROR.INVALID_START_DATE","title":"There is a problem with the request entity","detail":"Interval can not have a start date in the past","source":{"pointer":"/data/relationships/manualPrices/data/0/attributes/startDate"}}]}`
	err := shared.WithDiagnostic(
		fmt.Errorf("pricing schedule create: --start-date is in the past\n\nApp Store Connect: %w", asc.ParseErrorWithStatus([]byte(body), http.StatusConflict)),
		shared.DiagnosticInvalidInput,
		"--start-date",
	)

	got := runtimeFailureContext(
		invocationAnalysis{shape: telemetry.InvocationShapeLeaf},
		err,
		ExitCodeFromError(err),
	)

	if got.ErrorKind != telemetry.ErrorKindAPIConflict ||
		got.FailureStage != telemetry.FailureStageRequest ||
		got.OutcomeKind != telemetry.OutcomeConflict ||
		got.HTTPStatus != http.StatusConflict ||
		got.FailureParameter != "--start-date" ||
		got.DiagnosticCode != string(shared.DiagnosticInvalidInput) {
		t.Fatalf("runtimeFailureContext() = %+v", got)
	}
}

func TestAnalyzeInvocationPreservesRawTokens(t *testing.T) {
	root := RootCommand("1.0.0")

	got := analyzeInvocation(root, []string{" builds "})

	if got.command != root || got.shape != telemetry.InvocationShapeUnknownChild || got.unknownToken != " builds " {
		t.Fatalf("analyzeInvocation() = %+v, want root unknown child with raw token", got)
	}
}

func TestCommonCommandPathRecoveryDestinationsExist(t *testing.T) {
	root := RootCommand("1.0.0")

	for _, rule := range commonCommandPathRecoveryRules {
		current := root
		for _, part := range rule.destination {
			current = findDirectSubcommand(current, part)
			if current == nil {
				t.Fatalf("recovery destination %q does not resolve to a command", strings.Join(rule.destination, " "))
			}
		}
		if current.Exec == nil {
			t.Fatalf("recovery destination %q is not executable", strings.Join(rule.destination, " "))
		}
	}
}

func TestCommonCommandPathRecoveryRequiresExactUnknownPrefix(t *testing.T) {
	root := RootCommand("1.0.0")
	analysis := invocationAnalysis{shape: telemetry.InvocationShapeUnknownChild}
	tests := [][]string{
		{"builds", "show", "--build-id", "BUILD_ID"},
		{"builds", "get", "--build-id", "BUILD_ID"},
		{"versions", "information", "--version-id", "VERSION_ID"},
		{" versions ", "info", "--version-id", "VERSION_ID"},
		{"reviewsubmissions", "get", "--id", "SUBMISSION_ID"},
		{"testflight", "groups", "build", "list", "--build-id", "BUILD_ID"},
	}

	for _, args := range tests {
		if invalid, suggested, ok := commonCommandPathRecovery(root, analysis, args); ok {
			t.Fatalf("commonCommandPathRecovery(%q) = (%q, %q, true), want no recovery", args, invalid, suggested)
		}
	}
}

func TestCommonCommandPathRecoveryRejectsUnsupportedSuffix(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	root := RootCommand("1.0.0")
	analysis := invocationAnalysis{shape: telemetry.InvocationShapeUnknownChild}
	tests := [][]string{
		{"versions", "info"},
		{"versions", "info", "--version-id", "VERSION_ID", "--include", "unknown"},
		{"versions", "info", "--version-id", "VERSION_ID", "--include", "build", "--include-build"},
		{"versions", "info", "--version-id", "ONE", "--id", "TWO"},
		{"versions", "info", "--version-id", "VERSION_ID", "--output", "yaml"},
		{"versions", "info", "--version-id", "VERSION_ID", "--include-build", "maybe"},
		{"versions", "info", "--version-id", "VERSION_ID", "localizations"},
		{"versions", "info", "--version-id"},
		{"versions", "info", "--version-id", "--include-build"},
		{"versions", "info", "--version-id", ""},
		{"versions", "info", "--version-id="},
		{"versions", "info", "--version-id", "VERSION_ID", "--include-build", ""},
		{"versions", "info", "--version-id", "VERSION_ID", "--include-build="},
		{"versions", "info", "---version-id", "VERSION_ID"},
		{"versions", "info", "--version-id", "VERSION_ID", "--include-build=maybe"},
		{"reviewsubmissions", "list", "--limit=abc"},
		{"reviewsubmissions", "list", "--app", "APP_ID", "--limit", ""},
		{"reviewsubmissions", "list", "--app", "APP_ID", "--limit="},
		{"reviewsubmissions", "list", "--limit", "abc"},
		{"reviewsubmissions", "list", "--app", "APP_ID", "--limit", "201"},
		{"reviewsubmissions", "list", "--app", "APP_ID", "--platform", "ANDROID"},
		{"reviewsubmissions", "list", "--app", "APP_ID", "--state", "UNKNOWN"},
		{"reviewsubmissions", "list", "--next", "http://api.appstoreconnect.apple.com/v1/reviewSubmissions"},
		{"reviewsubmissions", "list", "--next", "https://api.appstoreconnect.apple.com/v1/reviewSubmissions", "--app", "APP_ID"},
		{"reviewsubmissions", "list"},
		{"reviewsubmissions", "list", "--unknown", "VALUE"},
		{"testflight", "groups", "builds", "list", "--app", "APP_ID", "--limit", "201"},
		{"testflight", "groups", "builds", "list", "--next", "http://api.appstoreconnect.apple.com/v1/betaGroups"},
		{"testflight", "groups", "builds", "list", "--app", "APP_ID", "--internal", "--external"},
		{"testflight", "groups", "builds", "list", "--build-id", "BUILD_ID", "--limit", "10"},
		{"testflight", "groups", "builds", "list", "--global", "--app", "APP_ID"},
		{"testflight", "groups", "builds", "list"},
		{"testflight", "groups", "builds", "list", "--"},
	}

	for _, args := range tests {
		if invalid, suggested, ok := commonCommandPathRecovery(root, analysis, args); ok {
			t.Fatalf("commonCommandPathRecovery(%q) = (%q, %q, true), want no recovery", args, invalid, suggested)
		}
	}
}

func TestCommonCommandPathRecoveryAcceptsCompleteDestinationFlags(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	root := RootCommand("1.0.0")
	analysis := invocationAnalysis{shape: telemetry.InvocationShapeUnknownChild}
	tests := [][]string{
		{"versions", "info", "--version-id", "VERSION_ID"},
		{"versions", "info", "--version-id=VERSION_ID"},
		{"versions", "info", "--version-id", "VERSION_ID", "--include-build"},
		{"versions", "info", "--version-id", "VERSION_ID", "--include-build", "true"},
		{"versions", "info", "--version-id", "VERSION_ID", "--include-build", "false"},
		{"versions", "info", "--version-id", "VERSION_ID", "--include", "build", "--include-build=false"},
		{"versions", "info", "--version-id", "VERSION_ID", "--include="},
		{"versions", "info", "--version-id", "VERSION_ID", "--include", ""},
		{"reviewsubmissions", "list", "--app", "APP_ID", "--limit=10"},
		{"reviewsubmissions", "list", "--app", "APP_ID", "--limit", "10"},
		{"reviewsubmissions", "list", "--app", "APP_ID", "--platform="},
		{"reviewsubmissions", "list", "--app", "APP_ID", "--platform", ""},
		{"reviewsubmissions", "list", "--next", "https://api.appstoreconnect.apple.com/v1/reviewSubmissions"},
		{"testflight", "groups", "builds", "list", "--app", "APP_ID", "--limit", "10"},
		{"testflight", "groups", "builds", "list", "--app", "APP_ID", "--next="},
		{"testflight", "groups", "builds", "list", "--app", "APP_ID", "--next", ""},
		{"testflight", "groups", "builds", "list", "--build-id", "BUILD_ID", "--internal"},
		{"testflight", "groups", "builds", "list", "--build-id", "BUILD_ID", "--internal=false", "--external"},
		{"testflight", "groups", "builds", "list", "--global"},
	}

	for _, args := range tests {
		if _, _, ok := commonCommandPathRecovery(root, analysis, args); !ok {
			t.Fatalf("commonCommandPathRecovery(%q) did not recognize complete destination flags", args)
		}
	}
}

func TestCommonCommandPathRecoveryAcceptsTerminalHelp(t *testing.T) {
	root := RootCommand("1.0.0")
	analysis := invocationAnalysis{shape: telemetry.InvocationShapeUnknownChild}
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"versions", "info", "--help"}, "asc versions view --help"},
		{[]string{"versions", "info", "-h"}, "asc versions view -h"},
		{[]string{"reviewsubmissions", "list", "--help"}, "asc review submissions list --help"},
		{[]string{"reviewsubmissions", "list", "-h"}, "asc review submissions list -h"},
		{[]string{"testflight", "groups", "builds", "list", "--help"}, "asc testflight groups list --help"},
		{[]string{"testflight", "groups", "builds", "list", "-h"}, "asc testflight groups list -h"},
	}

	for _, test := range tests {
		_, suggested, ok := commonCommandPathRecovery(root, analysis, test.args)
		if !ok {
			t.Fatalf("commonCommandPathRecovery(%q) did not recognize terminal help", test.args)
		}
		if suggested != test.want {
			t.Fatalf("commonCommandPathRecovery(%q) suggestion = %q, want %q", test.args, suggested, test.want)
		}
	}
}

func TestCommonCommandPathRecoveryRejectsNonTerminalHelp(t *testing.T) {
	root := RootCommand("1.0.0")
	analysis := invocationAnalysis{shape: telemetry.InvocationShapeUnknownChild}
	for _, args := range [][]string{
		{"versions", "info", "--help=true"},
		{"versions", "info", "--help", "--version-id", "VERSION_ID"},
		{"versions", "info", "-h", "false"},
	} {
		if invalid, suggested, ok := commonCommandPathRecovery(root, analysis, args); ok {
			t.Fatalf("commonCommandPathRecovery(%q) = (%q, %q, true), want no recovery", args, invalid, suggested)
		}
	}
}

func TestCommonCommandPathRecoveryUsesAppIDEnvironment(t *testing.T) {
	t.Setenv("ASC_APP_ID", "APP_ID")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "invalid.json"))
	if err := os.WriteFile(os.Getenv("ASC_CONFIG_PATH"), []byte("not json"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	root := RootCommand("1.0.0")
	analysis := invocationAnalysis{shape: telemetry.InvocationShapeUnknownChild}

	for _, args := range [][]string{
		{"reviewsubmissions", "list", "--limit", "10"},
		{"testflight", "groups", "builds", "list", "--limit", "10"},
	} {
		if _, _, ok := commonCommandPathRecovery(root, analysis, args); !ok {
			t.Fatalf("commonCommandPathRecovery(%q) did not honor ASC_APP_ID", args)
		}
	}
}

func TestCommonCommandPathRecoveryUsesConfiguredAppID(t *testing.T) {
	t.Setenv("ASC_APP_ID", "temporary")
	if err := os.Unsetenv("ASC_APP_ID"); err != nil {
		t.Fatalf("Unsetenv() error: %v", err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(`{"app_id":"APP_FROM_CONFIG"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	t.Setenv("ASC_CONFIG_PATH", configPath)

	root := RootCommand("1.0.0")
	analysis := invocationAnalysis{shape: telemetry.InvocationShapeUnknownChild}
	for _, args := range [][]string{
		{"reviewsubmissions", "list", "--limit", "10"},
		{"testflight", "groups", "builds", "list", "--limit", "10"},
	} {
		if _, _, ok := commonCommandPathRecovery(root, analysis, args); !ok {
			t.Fatalf("commonCommandPathRecovery(%q) did not honor configured app ID", args)
		}
	}
}

func TestCommonCommandPathRecoveryOmitsConsumedReportFlags(t *testing.T) {
	root := RootCommand("1.0.0")
	analysis := invocationAnalysis{shape: telemetry.InvocationShapeUnknownChild}
	tests := []struct {
		args []string
		want string
	}{
		{
			args: []string{
				"--report", "junit", "--report-file", "/tmp/junit.xml", "--profile", "Team Profile",
				"versions", "info", "--version-id", "VERSION_ID",
			},
			want: "asc --profile 'Team Profile' versions view --version-id VERSION_ID",
		},
		{
			args: []string{
				"--report=junit", "--report-file=/tmp/junit.xml", "--profile", "report",
				"versions", "info", "--version-id=VERSION_ID",
			},
			want: "asc --profile report versions view --version-id=VERSION_ID",
		},
		{
			args: []string{
				"--profile", "--report", "--report=junit", "--report-file=/tmp/junit.xml",
				"versions", "info", "--version-id", "VERSION_ID",
			},
			want: "asc --profile --report versions view --version-id VERSION_ID",
		},
	}

	for _, test := range tests {
		_, suggested, ok := commonCommandPathRecovery(root, analysis, test.args)
		if !ok {
			t.Fatalf("commonCommandPathRecovery(%q) did not recognize exact command path", test.args)
		}
		if suggested != test.want {
			t.Fatalf("commonCommandPathRecovery(%q) suggestion = %q, want %q", test.args, suggested, test.want)
		}
	}
}

func TestCommonCommandPathRecoveryRendersSuffixForSafeShellCopy(t *testing.T) {
	root := RootCommand("1.0.0")
	analysis := invocationAnalysis{shape: telemetry.InvocationShapeUnknownChild}
	tests := []struct {
		args []string
		want string
	}{
		{
			args: []string{"versions", "info", "--version-id", "VERSION ID; $(not-a-command)"},
			want: "asc versions view --version-id 'VERSION ID; $(not-a-command)'",
		},
		{
			args: []string{"versions", "info", "--id", "VERSION_ID"},
			want: "asc versions view --version-id VERSION_ID",
		},
	}
	for _, test := range tests {
		_, suggested, ok := commonCommandPathRecovery(root, analysis, test.args)
		if !ok {
			t.Fatalf("commonCommandPathRecovery(%q) did not recognize exact command path", test.args)
		}
		if suggested != test.want {
			t.Fatalf("suggested command = %q, want %q", suggested, test.want)
		}
	}
}

func TestRenderSuggestedCommandForWindowsUsesOnlyPortableUnquotedArgs(t *testing.T) {
	got, ok := renderSuggestedCommandForOS([]string{
		"versions", "view", "--version-id", "ABC_123", "--next=https://example.com/v1/builds", "--include=build+app",
	}, "windows")
	if !ok {
		t.Fatal("renderSuggestedCommandForOS() rejected portable Windows arguments")
	}
	want := "asc versions view --version-id ABC_123 --next=https://example.com/v1/builds --include=build+app"
	if got != want {
		t.Fatalf("renderSuggestedCommandForOS() = %q, want %q", got, want)
	}
}

func TestRenderSuggestedCommandForWindowsRejectsShellDependentArgs(t *testing.T) {
	for _, arg := range []string{
		"", "Team Profile", "100%", "bang!", "quote'", `double"quote`, "a&b", "a|b", "a<b", "a>b",
		"caret^", "(group)", "@splat", "a,b", "C:\\path", "line\nbreak", "café",
	} {
		if got, ok := renderSuggestedCommandForOS([]string{"versions", "view", "--version-id", arg}, "windows"); ok {
			t.Fatalf("renderSuggestedCommandForOS(%q) = %q, true; want rejection", arg, got)
		}
	}
}

func TestCommonCommandPathRecoveryForWindowsRequiresPortableArgs(t *testing.T) {
	root := RootCommand("1.0.0")
	analysis := invocationAnalysis{shape: telemetry.InvocationShapeUnknownChild}

	_, suggested, ok := commonCommandPathRecoveryForOS(root, analysis, []string{
		"versions", "info", "--version-id", "VERSION_ID",
	}, "windows")
	if !ok || suggested != "asc versions view --version-id VERSION_ID" {
		t.Fatalf("portable Windows recovery = (%q, %t), want exact semantic retry", suggested, ok)
	}

	for _, value := range []string{"VERSION ID", "", "100%", `C:\\version`} {
		if invalid, suggested, ok := commonCommandPathRecoveryForOS(root, analysis, []string{
			"versions", "info", "--version-id", value,
		}, "windows"); ok {
			t.Fatalf("Windows recovery for %q = (%q, %q, true), want generic fallback", value, invalid, suggested)
		}
	}
}

func TestRenderSuggestedCommandForPOSIXPreservesQuotedArgs(t *testing.T) {
	got, ok := renderSuggestedCommandForOS([]string{"--profile", "Team Profile", "versions", "view", "--include", ""}, "darwin")
	if !ok {
		t.Fatal("renderSuggestedCommandForOS() rejected POSIX quoted arguments")
	}
	want := "asc --profile 'Team Profile' versions view --include ''"
	if got != want {
		t.Fatalf("renderSuggestedCommandForOS() = %q, want %q", got, want)
	}
}

func TestParseFailureContextClassifiesUnknownChildAsOther(t *testing.T) {
	got := parseFailureContext(invocationAnalysis{shape: telemetry.InvocationShapeUnknownChild})

	if got.ErrorKind != telemetry.ErrorKindOther || got.FailureStage != telemetry.FailureStageParse {
		t.Fatalf("parseFailureContext() = %+v, want kind=%q stage=%q", got, telemetry.ErrorKindOther, telemetry.FailureStageParse)
	}
}

func TestRun_GroupUnknownFlagNamesSubcommandsThatAcceptIt(t *testing.T) {
	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"apps", "info", "--app", "123"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	want := "Error: unknown flag `--app` for `asc apps info`\n" +
		"Did you mean a subcommand? These accept `--app`: list, view, edit\n" +
		"For help:\n  asc apps info --help\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}
