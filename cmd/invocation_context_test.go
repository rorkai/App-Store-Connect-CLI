package cmd

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

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

func TestAnalyzeInvocationPreservesRawTokens(t *testing.T) {
	root := RootCommand("1.0.0")

	got := analyzeInvocation(root, []string{" builds "})

	if got.command != root || got.shape != telemetry.InvocationShapeUnknownChild || got.unknownToken != " builds " {
		t.Fatalf("analyzeInvocation() = %+v, want root unknown child with raw token", got)
	}
}

func TestParseFailureContextClassifiesUnknownChildAsOther(t *testing.T) {
	got := parseFailureContext(invocationAnalysis{shape: telemetry.InvocationShapeUnknownChild})

	if got.ErrorKind != telemetry.ErrorKindOther || got.FailureStage != telemetry.FailureStageParse {
		t.Fatalf("parseFailureContext() = %+v, want kind=%q stage=%q", got, telemetry.ErrorKindOther, telemetry.FailureStageParse)
	}
}
