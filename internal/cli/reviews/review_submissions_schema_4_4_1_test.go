package reviews

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestReviewSubmissionsUpdateValidatesSchemaAndConfirmationBeforeAuth(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		want      string
		wantCode  shared.DiagnosticCode
		wantParam string
	}{
		{name: "missing update", args: []string{"--id", "sub-1"}, want: "at least one update flag is required (--platform, --submitted, --canceled, --clear-platform, --clear-submitted, --clear-canceled)", wantCode: shared.DiagnosticRequiredInputMissing},
		{name: "invalid platform", args: []string{"--id", "sub-1", "--platform", "NOPE"}, want: "--platform must be one of", wantCode: shared.DiagnosticInvalidInput, wantParam: "--platform"},
		{name: "platform clear conflict", args: []string{"--id", "sub-1", "--platform", "IOS", "--clear-platform"}, want: "--platform cannot be combined with --clear-platform", wantCode: shared.DiagnosticConflictingInput},
		{name: "submitted clear conflict", args: []string{"--id", "sub-1", "--submitted=false", "--clear-submitted"}, want: "--submitted cannot be combined with --clear-submitted", wantCode: shared.DiagnosticConflictingInput},
		{name: "canceled clear conflict", args: []string{"--id", "sub-1", "--canceled=false", "--clear-canceled"}, want: "--canceled cannot be combined with --clear-canceled", wantCode: shared.DiagnosticConflictingInput},
		{name: "contradictory transitions", args: []string{"--id", "sub-1", "--submitted=true", "--canceled=true", "--confirm"}, want: "--submitted=true cannot be combined with --canceled=true", wantCode: shared.DiagnosticConflictingInput},
		{name: "submission requires confirmation", args: []string{"--id", "sub-1", "--submitted=true"}, want: "--confirm is required when --submitted=true", wantCode: shared.DiagnosticRequiredInputMissing, wantParam: "--confirm"},
		{name: "cancellation requires confirmation", args: []string{"--id", "sub-1", "--canceled=true"}, want: "--confirm is required when --canceled=true", wantCode: shared.DiagnosticRequiredInputMissing, wantParam: "--confirm"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			d, _, stderrBuffer := newReviewTestDeps(func() (*asc.Client, error) {
				factoryCalled = true
				return nil, errors.New("poison client factory called")
			})

			err := newReviewSubmissionsUpdateCommand(d).ParseAndRun(context.Background(), test.args)
			stderr := stderrBuffer.String()
			if err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(stderr, "Error: "+test.want) {
				t.Fatalf("error = %v, stderr = %q, want usage error printing %q", err, stderr, test.want)
			}
			diagnostic, ok := shared.DiagnosticFromError(err)
			if !ok {
				t.Fatal("expected structured diagnostic")
			}
			if diagnostic.Code != test.wantCode || diagnostic.Parameter != test.wantParam {
				t.Fatalf("diagnostic = %+v, want code %q parameter %q", diagnostic, test.wantCode, test.wantParam)
			}
			if factoryCalled {
				t.Fatal("client factory called before update validation")
			}
		})
	}
}

func TestReviewItemsRemovedTrueRequiresConfirmationBeforeAuth(t *testing.T) {
	tests := []struct {
		name    string
		command func(d *shared.Deps) error
	}{
		{name: "flat", command: func(d *shared.Deps) error {
			return newReviewItemsUpdateCommand(d).ParseAndRun(context.Background(), []string{"--id", "item-1", "--removed=true"})
		}},
		{name: "nested", command: func(d *shared.Deps) error {
			return reviewItemsUpdateCommand(d, "update", "review items update", "usage", "example").ParseAndRun(context.Background(), []string{"--id", "item-1", "--removed=true"})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			d, _, _ := newReviewTestDeps(func() (*asc.Client, error) {
				factoryCalled = true
				return nil, errors.New("poison client factory called")
			})

			err := test.command(d)
			if err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "--confirm is required when --removed=true") {
				t.Fatalf("error = %v, want removal confirmation usage error", err)
			}
			diagnostic, ok := shared.DiagnosticFromError(err)
			if !ok {
				t.Fatal("expected structured diagnostic")
			}
			if diagnostic.Code != shared.DiagnosticRequiredInputMissing || diagnostic.Parameter != "--confirm" {
				t.Fatalf("diagnostic = %+v, want code %q parameter %q", diagnostic, shared.DiagnosticRequiredInputMissing, "--confirm")
			}
			if factoryCalled {
				t.Fatal("client factory called before removal confirmation")
			}
		})
	}
}
