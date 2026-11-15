package validate

import (
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestValidationFailuresExposeStructuredDiagnostics(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	d, _, _ := validateTestDeps()

	tests := []struct {
		name    string
		command func() interface {
			ParseAndRun(context.Context, []string) error
		}
		args      []string
		wantError string
		wantCode  shared.DiagnosticCode
		wantParam string
	}{
		{
			name: "testflight missing build",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return newValidateTestFlightCommand(d)
			},
			args:      []string{"--app", "app-1"},
			wantError: "--build-id",
			wantCode:  shared.DiagnosticRequiredInputMissing,
			wantParam: "--build-id",
		},
		{
			name: "validate conflicting version selectors",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return NewValidateCommand(d)
			},
			args:      []string{"--app", "app-1", "--version", "1.0", "--version-id", "version-1"},
			wantError: "--version and --version-id are mutually exclusive",
			wantCode:  shared.DiagnosticConflictingInput,
			wantParam: "--version-id",
		},
		{
			name: "validate apple id requires deep",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				return NewValidateCommand(d)
			},
			args:      []string{"--app", "app-1", "--version-id", "version-1", "--apple-id", "person@example.com"},
			wantError: "--apple-id requires --deep",
			wantCode:  shared.DiagnosticInvalidInput,
			wantParam: "--apple-id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.command().ParseAndRun(context.Background(), test.args)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want flag.ErrHelp contract", err)
			}
			if got := err.Error(); got != test.wantError {
				t.Fatalf("error = %q, want %q", got, test.wantError)
			}
			diagnostic, ok := shared.DiagnosticFromError(err)
			if !ok {
				t.Fatalf("DiagnosticFromError(%v) did not find metadata", err)
			}
			if diagnostic.Code != test.wantCode || diagnostic.Parameter != test.wantParam {
				t.Fatalf("diagnostic = %+v, want code %q parameter %q", diagnostic, test.wantCode, test.wantParam)
			}
		})
	}
}
