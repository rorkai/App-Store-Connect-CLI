package validate

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestValidationFailuresExposeStructuredDiagnostics(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		command func(*shared.Deps) interface {
			ParseAndRun(context.Context, []string) error
		}
		args       []string
		wantError  string
		wantStderr string
		wantCode   shared.DiagnosticCode
		wantParam  string
	}{
		{
			name: "testflight missing build",
			command: func(d *shared.Deps) interface {
				ParseAndRun(context.Context, []string) error
			} {
				return newValidateTestFlightCommand(d)
			},
			args:       []string{"--app", "app-1"},
			wantError:  "--build-id",
			wantStderr: "Error: --build-id is required\n",
			wantCode:   shared.DiagnosticRequiredInputMissing,
			wantParam:  "--build-id",
		},
		{
			name: "validate conflicting version selectors",
			command: func(d *shared.Deps) interface {
				ParseAndRun(context.Context, []string) error
			} {
				return NewValidateCommand(d)
			},
			args:       []string{"--app", "app-1", "--version", "1.0", "--version-id", "version-1"},
			wantError:  "--version and --version-id are mutually exclusive",
			wantStderr: "Error: --version and --version-id are mutually exclusive\n",
			wantCode:   shared.DiagnosticConflictingInput,
			wantParam:  "--version-id",
		},
		{
			name: "validate apple id requires deep",
			command: func(d *shared.Deps) interface {
				ParseAndRun(context.Context, []string) error
			} {
				return NewValidateCommand(d)
			},
			args:       []string{"--app", "app-1", "--version-id", "version-1", "--apple-id", "person@example.com"},
			wantError:  "--apple-id requires --deep",
			wantStderr: "Error: --apple-id requires --deep\n",
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--apple-id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d, stdout, stderr := validateTestDeps()
			err := test.command(d).ParseAndRun(context.Background(), test.args)
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
			if stdout.String() != "" || !strings.HasPrefix(stderr.String(), test.wantStderr) {
				t.Fatalf("stdout = %q, stderr = %q, want empty stdout and stderr starting with %q", stdout.String(), stderr.String(), test.wantStderr)
			}
		})
	}
}
