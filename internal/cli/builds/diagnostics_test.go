package builds

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestBuildsMissingRequiredInputExposesStructuredDiagnostic(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	d, _, stderrBuf := newBuildsTestDeps(nil)
	err := newBuildsUploadCommand(d).ParseAndRun(context.Background(), nil)
	stderr := stderrBuf.String()
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want flag.ErrHelp contract", err)
	}
	if got, want := err.Error(), "--app"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
	if want := "Error: --app is required (or set ASC_APP_ID)\n"; !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want diagnostic %q", stderr, want)
	}

	diagnostic, ok := shared.DiagnosticFromError(err)
	if !ok {
		t.Fatalf("DiagnosticFromError(%v) did not find metadata", err)
	}
	if diagnostic.Code != shared.DiagnosticRequiredInputMissing || diagnostic.Parameter != "--app" {
		t.Fatalf("diagnostic = %+v, want required_input_missing for --app", diagnostic)
	}
}

func TestBuildsMissingArtifactLeavesDiagnosticParameterEmpty(t *testing.T) {
	d, _, stderrBuf := newBuildsTestDeps(nil)
	err := newBuildsUploadCommand(d).ParseAndRun(context.Background(), []string{"--app", "123456789"})
	stderr := stderrBuf.String()
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want flag.ErrHelp contract", err)
	}
	if want := "Error: --ipa or --pkg is required\n"; !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want diagnostic %q", stderr, want)
	}

	diagnostic, ok := shared.DiagnosticFromError(err)
	if !ok {
		t.Fatalf("DiagnosticFromError(%v) did not find metadata", err)
	}
	if diagnostic.Code != shared.DiagnosticRequiredInputMissing || diagnostic.Parameter != "" {
		t.Fatalf("diagnostic = %+v, want required_input_missing without a parameter", diagnostic)
	}
}

func TestBuildsMissingExpireSelectorLeavesDiagnosticParameterEmpty(t *testing.T) {
	d, _, stderrBuf := newBuildsTestDeps(nil)
	err := newBuildsExpireAllCommand(d).ParseAndRun(context.Background(), []string{"--app", "123456789"})
	stderr := stderrBuf.String()
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want flag.ErrHelp contract", err)
	}
	if want := "Error: --older-than or --keep-latest is required\n"; !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want diagnostic %q", stderr, want)
	}

	diagnostic, ok := shared.DiagnosticFromError(err)
	if !ok {
		t.Fatalf("DiagnosticFromError(%v) did not find metadata", err)
	}
	if diagnostic.Code != shared.DiagnosticRequiredInputMissing || diagnostic.Parameter != "" {
		t.Fatalf("diagnostic = %+v, want required_input_missing without a parameter", diagnostic)
	}
}
