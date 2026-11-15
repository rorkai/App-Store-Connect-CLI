package subscriptions

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestSubscriptionsMissingRequiredInputExposesStructuredDiagnostic(t *testing.T) {
	d, stderrBuf := newSubscriptionsTestDeps()
	err := newSubscriptionsVersionLocalizationsViewCommand(d).ParseAndRun(context.Background(), nil)
	stderr := stderrBuf.String()
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want flag.ErrHelp contract", err)
	}
	if got, want := err.Error(), "--id"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
	if want := "Error: --id is required\n"; !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want diagnostic %q", stderr, want)
	}

	diagnostic, ok := shared.DiagnosticFromError(err)
	if !ok {
		t.Fatalf("DiagnosticFromError(%v) did not find metadata", err)
	}
	if diagnostic.Code != shared.DiagnosticRequiredInputMissing || diagnostic.Parameter != "--id" {
		t.Fatalf("diagnostic = %+v, want required_input_missing for --id", diagnostic)
	}
}

func TestSubscriptionsGroupUpdateMissingReferenceNameExposesStructuredDiagnostic(t *testing.T) {
	d, stderrBuf := newSubscriptionsTestDeps()
	err := newSubscriptionsGroupsUpdateCommand(d).ParseAndRun(context.Background(), []string{"--id", "group-1"})
	stderr := stderrBuf.String()
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want flag.ErrHelp contract", err)
	}
	if got, want := err.Error(), "--reference-name"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
	if want := "Error: at least one update flag is required (--reference-name)\n"; !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want diagnostic %q", stderr, want)
	}

	diagnostic, ok := shared.DiagnosticFromError(err)
	if !ok {
		t.Fatalf("DiagnosticFromError(%v) did not find metadata", err)
	}
	if diagnostic.Code != shared.DiagnosticRequiredInputMissing || diagnostic.Parameter != "--reference-name" {
		t.Fatalf("diagnostic = %+v, want required_input_missing for --reference-name", diagnostic)
	}
}

func TestSubscriptionsConflictingInputExposesStructuredDiagnostic(t *testing.T) {
	for _, test := range []struct {
		name          string
		args          []string
		wantParameter string
		wantStderr    string
	}{
		{
			name:          "name",
			args:          []string{"--id", "localization-1", "--name", "Premium", "--clear-name"},
			wantParameter: "--name",
			wantStderr:    "Error: --name cannot be used with --clear-name\n",
		},
		{
			name:          "custom app name",
			args:          []string{"--id", "localization-1", "--custom-app-name", "Premium", "--clear-custom-app-name"},
			wantParameter: "--custom-app-name",
			wantStderr:    "Error: --custom-app-name cannot be used with --clear-custom-app-name\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			d, stderrBuf := newSubscriptionsTestDeps()
			err := newSubscriptionsGroupsVersionLocalizationsUpdateCommand(d).ParseAndRun(context.Background(), test.args)
			stderr := stderrBuf.String()
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want flag.ErrHelp contract", err)
			}
			if got, want := err.Error(), flag.ErrHelp.Error(); got != want {
				t.Fatalf("error = %q, want %q", got, want)
			}
			if got := shared.ClassifyUsageError(err); got != shared.UsageErrorInvalidValue {
				t.Fatalf("usage classification = %q, want %q", got, shared.UsageErrorInvalidValue)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want diagnostic %q", stderr, test.wantStderr)
			}

			diagnostic, ok := shared.DiagnosticFromError(err)
			if !ok {
				t.Fatalf("DiagnosticFromError(%v) did not find metadata", err)
			}
			if diagnostic.Code != shared.DiagnosticConflictingInput || diagnostic.Parameter != test.wantParameter {
				t.Fatalf("diagnostic = %+v, want conflicting_input for %s", diagnostic, test.wantParameter)
			}
		})
	}
}

func TestSubscriptionsAlternativeSelectorsLeaveDiagnosticParameterEmpty(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	tests := []struct {
		name    string
		command func() interface {
			ParseAndRun(context.Context, []string) error
		}
		depsCommand func(*shared.Deps) interface {
			ParseAndRun(context.Context, []string) error
		}
		wantMessage string
	}{
		{
			name: "subscription list",
			depsCommand: func(d *shared.Deps) interface {
				ParseAndRun(context.Context, []string) error
			} {
				return newSubscriptionsListCommand(d)
			},
			wantMessage: "Error: --group-id or --app is required (or set ASC_APP_ID)\n",
		},
		{
			name: "availability view",
			depsCommand: func(d *shared.Deps) interface {
				ParseAndRun(context.Context, []string) error
			} {
				return newSubscriptionsAvailabilityViewCommand(d)
			},
			wantMessage: "Error: --availability-id or --subscription-id is required\n",
		},
		{
			name: "availability territories",
			depsCommand: func(d *shared.Deps) interface {
				ParseAndRun(context.Context, []string) error
			} {
				return newSubscriptionsAvailabilityAvailableTerritoriesCommand(d)
			},
			wantMessage: "Error: --availability-id or --subscription-id is required\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var err error
			var stderr string
			if test.depsCommand != nil {
				d, stderrBuf := newSubscriptionsTestDeps()
				err = test.depsCommand(d).ParseAndRun(context.Background(), nil)
				stderr = stderrBuf.String()
			} else {
				stderr = captureSubscriptionsDiagnosticStderr(t, func() {
					err = test.command().ParseAndRun(context.Background(), nil)
				})
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want flag.ErrHelp contract", err)
			}
			if !strings.Contains(stderr, test.wantMessage) {
				t.Fatalf("stderr = %q, want diagnostic %q", stderr, test.wantMessage)
			}
			diagnostic, ok := shared.DiagnosticFromError(err)
			if !ok {
				t.Fatalf("DiagnosticFromError(%v) did not find metadata", err)
			}
			if diagnostic.Code != shared.DiagnosticRequiredInputMissing || diagnostic.Parameter != "" {
				t.Fatalf("diagnostic = %+v, want required_input_missing without a parameter", diagnostic)
			}
		})
	}
}

func TestSubscriptionsNumberOfPeriodsDistinguishesMissingFromInvalid(t *testing.T) {
	tests := []struct {
		name    string
		command func() interface {
			ParseAndRun(context.Context, []string) error
		}
		args []string
	}{
		{
			name: "introductory offer",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				d, _ := newSubscriptionsTestDeps()
				return newSubscriptionsIntroductoryOffersCreateCommand(d)
			},
			args: []string{"--subscription-id", "sub-1", "--offer-duration", "ONE_MONTH", "--offer-mode", "FREE_TRIAL"},
		},
		{
			name: "offer code",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				d, _ := newSubscriptionsTestDeps()
				return newSubscriptionsOfferCodesCreateCommand(d)
			},
			args: []string{"--subscription-id", "sub-1", "--name", "Spring", "--offer-eligibility", "STACK_WITH_INTRO_OFFERS", "--customer-eligibilities", "NEW", "--offer-duration", "ONE_MONTH", "--offer-mode", "FREE_TRIAL"},
		},
		{
			name: "promotional offer",
			command: func() interface {
				ParseAndRun(context.Context, []string) error
			} {
				d, _ := newSubscriptionsTestDeps()
				return newSubscriptionsPromotionalOffersCreateCommand(d)
			},
			args: []string{"--subscription-id", "sub-1", "--offer-code", "SPRING", "--name", "Spring", "--offer-duration", "ONE_MONTH", "--offer-mode", "FREE_TRIAL"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, input := range []struct {
				name     string
				periods  []string
				wantCode shared.DiagnosticCode
			}{
				{name: "missing", wantCode: shared.DiagnosticRequiredInputMissing},
				{name: "nonpositive", periods: []string{"--number-of-periods", "-1"}, wantCode: shared.DiagnosticInvalidInput},
			} {
				t.Run(input.name, func(t *testing.T) {
					args := append(append([]string{}, test.args...), input.periods...)
					err := test.command().ParseAndRun(context.Background(), args)
					if !errors.Is(err, flag.ErrHelp) {
						t.Fatalf("error = %v, want flag.ErrHelp contract", err)
					}
					diagnostic, ok := shared.DiagnosticFromError(err)
					if !ok {
						t.Fatalf("DiagnosticFromError(%v) did not find metadata", err)
					}
					if diagnostic.Code != input.wantCode || diagnostic.Parameter != "--number-of-periods" {
						t.Fatalf("diagnostic = %+v, want code %q parameter --number-of-periods", diagnostic, input.wantCode)
					}
					wantKind := shared.UsageErrorMissingRequired
					if input.name == "nonpositive" {
						wantKind = shared.UsageErrorInvalidValue
					}
					if got := shared.ClassifyUsageError(err); got != wantKind {
						t.Fatalf("usage classification = %q, want %q", got, wantKind)
					}
				})
			}
		})
	}
}

func captureSubscriptionsDiagnosticStderr(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	os.Stderr = writer
	defer func() {
		os.Stderr = original
		_ = reader.Close()
		_ = writer.Close()
	}()

	fn()
	_ = writer.Close()
	os.Stderr = original
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return string(data)
}
