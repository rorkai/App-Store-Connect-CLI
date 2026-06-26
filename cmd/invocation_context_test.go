package cmd

import (
	"errors"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

func TestRuntimeFailureContextClassifiesLowCardinalityFailures(t *testing.T) {
	analysis := invocationAnalysis{shape: telemetry.InvocationShapeLeaf}
	tests := []struct {
		name      string
		err       error
		exitCode  int
		wantKind  telemetry.ErrorKind
		wantStage telemetry.FailureStage
	}{
		{
			name:      "missing auth is local validation",
			err:       shared.ErrMissingAuth,
			exitCode:  ExitAuth,
			wantKind:  telemetry.ErrorKindOther,
			wantStage: telemetry.FailureStageValidation,
		},
		{
			name:      "API conflict",
			err:       errors.New("conflict"),
			exitCode:  ExitConflict,
			wantKind:  telemetry.ErrorKindAPIConflict,
			wantStage: telemetry.FailureStageRequest,
		},
		{
			name:      "API server failure",
			err:       errors.New("server failure"),
			exitCode:  ExitHTTPInternalServer,
			wantKind:  telemetry.ErrorKindAPI5xx,
			wantStage: telemetry.FailureStageRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := runtimeFailureContext(analysis, test.err, test.exitCode)
			if got.ErrorKind != test.wantKind || got.FailureStage != test.wantStage {
				t.Fatalf("runtimeFailureContext() = %+v, want kind=%q stage=%q", got, test.wantKind, test.wantStage)
			}
		})
	}
}
