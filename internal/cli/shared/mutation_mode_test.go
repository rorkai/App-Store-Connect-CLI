package shared

import (
	"errors"
	"flag"
	"testing"
)

func TestRequireConfirmUnlessDryRunCarriesStructuredDiagnostic(t *testing.T) {
	err := DefaultDeps().RequireConfirmUnlessDryRun(false, false)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want flag.ErrHelp", err)
	}
	if err.Error() != "--confirm is required unless --dry-run is set" {
		t.Fatalf("error = %q, want preserved confirmation message", err)
	}

	diagnostic, ok := DiagnosticFromError(err)
	if !ok {
		t.Fatal("expected structured diagnostic")
	}
	if diagnostic.Code != DiagnosticRequiredInputMissing || diagnostic.Parameter != "--confirm" {
		t.Fatalf("diagnostic = %+v, want code %q parameter %q", diagnostic, DiagnosticRequiredInputMissing, "--confirm")
	}
}
