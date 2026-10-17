package signing

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// Library callers such as distribute resolve signing paths from a config file,
// so a read failure must stay unclassified until a command entry point that
// received the flag classifies it.
func TestSigningInputReadFailureIsClassifiedOnlyByCommandEntryPoints(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.p12")
	err := withSigningRunInputData(
		signingRunOptions{IdentityPath: missing, ProfilePath: missing},
		readBoundedSigningRunFile,
		func(_, _, _ []byte) error {
			t.Fatal("operation ran despite a missing identity")
			return nil
		},
	)
	if err == nil {
		t.Fatal("withSigningRunInputData() error = nil, want missing identity failure")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("withSigningRunInputData() error = %v, want fs.ErrNotExist in chain", err)
	}
	if shared.IsValidationError(err) {
		t.Fatalf("library error is already a validation error: %v", err)
	}
	if diagnostic, ok := shared.DiagnosticFromError(err); ok {
		t.Fatalf("library error already carries diagnostic %+v", diagnostic)
	}

	classified := classifySigningInputError(err)
	if got, want := classified.Error(), err.Error(); got != want {
		t.Fatalf("classified message = %q, want %q", got, want)
	}
	if !shared.IsValidationError(classified) {
		t.Fatalf("classified error is not a validation error: %v", classified)
	}
	diagnostic, ok := shared.DiagnosticFromError(classified)
	if !ok || diagnostic.Code != shared.DiagnosticFileNotFound || diagnostic.Parameter != "--identity" {
		t.Fatalf("classified diagnostic = %+v, %t; want file_not_found for --identity", diagnostic, ok)
	}

	unrelated := errors.New("child failed")
	got := classifySigningInputError(unrelated)
	if shared.IsValidationError(got) {
		t.Fatalf("classifySigningInputError(unrelated) = %v, want it unclassified", got)
	}
	if diagnostic, ok := shared.DiagnosticFromError(got); ok {
		t.Fatalf("classifySigningInputError(unrelated) carries diagnostic %+v", diagnostic)
	}
}
