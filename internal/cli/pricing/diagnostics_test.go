package pricing

import (
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestPricingScheduleCreateInvalidInputExposesStructuredDiagnostics(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name       string
		args       []string
		wantError  string
		wantStderr string
		wantUsage  bool
		wantCode   shared.DiagnosticCode
		wantParam  string
	}{
		{
			name:       "no price selection",
			args:       []string{"--app", "APP", "--base-territory", "USA", "--start-date", "2024-03-01"},
			wantStderr: "Error: one of --price-point, --tier, --price, or --free is required\n",
			wantUsage:  true,
			wantCode:   shared.DiagnosticRequiredInputMissing,
			wantParam:  "",
		},
		{
			name:       "conflicting price selection",
			args:       []string{"--app", "APP", "--price-point", "PP", "--price", "0.99", "--base-territory", "USA", "--start-date", "2024-03-01"},
			wantStderr: "Error: --price-point, --tier, --price, and --free are mutually exclusive\n",
			wantUsage:  true,
			wantCode:   shared.DiagnosticConflictingInput,
			wantParam:  "",
		},
		{
			name:       "negative tier",
			args:       []string{"--app", "APP", "--tier", "-1", "--base-territory", "USA", "--start-date", "2024-03-01"},
			wantStderr: "Error: --tier must be a positive integer\n",
			wantUsage:  true,
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--tier",
		},
		{
			name:       "non numeric price",
			args:       []string{"--app", "APP", "--price", "abc", "--base-territory", "USA", "--start-date", "2024-03-01"},
			wantStderr: "Error: --price must be a number\n",
			wantUsage:  true,
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--price",
		},
		{
			name:       "unmappable base territory",
			args:       []string{"--app", "APP", "--price-point", "PP", "--base-territory", "Neverland", "--start-date", "2024-03-01"},
			wantError:  "territory \"Neverland\" could not be mapped to an App Store Connect territory ID",
			wantStderr: "Error: territory \"Neverland\" could not be mapped to an App Store Connect territory ID\n",
			wantUsage:  true,
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--base-territory",
		},
		{
			name:       "malformed start date",
			args:       []string{"--app", "APP", "--price-point", "PP", "--base-territory", "USA", "--start-date", "03-01-2024"},
			wantError:  "--start-date must be in YYYY-MM-DD format",
			wantStderr: "Error: --start-date must be in YYYY-MM-DD format\n",
			wantUsage:  true,
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--start-date",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := PricingScheduleCreateCommand()
			if err := command.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}

			var err error
			stderr := capturePricingStderr(t, func() {
				err = command.Exec(context.Background(), nil)
			})
			if err == nil {
				t.Fatal("expected error")
			}
			if test.wantError != "" && err.Error() != test.wantError {
				t.Fatalf("error = %q, want %q", err, test.wantError)
			}
			if test.wantStderr != "" && stderr != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if got := errors.Is(err, flag.ErrHelp); got != test.wantUsage {
				t.Fatalf("errors.Is(err, flag.ErrHelp) = %t, want %t", got, test.wantUsage)
			}

			diagnostic, ok := shared.DiagnosticFromError(err)
			if !ok {
				t.Fatalf("DiagnosticFromError(%v) found no metadata", err)
			}
			if diagnostic.Code != test.wantCode || diagnostic.Parameter != test.wantParam {
				t.Fatalf("diagnostic = %+v, want code %q parameter %q", diagnostic, test.wantCode, test.wantParam)
			}
		})
	}
}
