package assets

import (
	"errors"
	"fmt"
	"io/fs"
	"syscall"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestMediaInputClassificationSkipsStorageFailures(t *testing.T) {
	tests := []struct {
		name         string
		cause        error
		fallback     shared.DiagnosticCode
		wantCode     shared.DiagnosticCode
		wantDetected bool
	}{
		{name: "missing", cause: &fs.PathError{Op: "lstat", Path: "a.png", Err: syscall.ENOENT}, fallback: shared.DiagnosticFileInvalidFormat, wantCode: shared.DiagnosticFileNotFound, wantDetected: true},
		{name: "unreadable", cause: &fs.PathError{Op: "open", Path: "a.png", Err: syscall.EACCES}, fallback: shared.DiagnosticFileInvalidFormat, wantCode: shared.DiagnosticFilePermissionDenied, wantDetected: true},
		{name: "content", cause: errors.New("decode image format: image: unknown format"), fallback: shared.DiagnosticFileInvalidFormat, wantCode: shared.DiagnosticFileInvalidFormat, wantDetected: true},
		{name: "storage path error", cause: &fs.PathError{Op: "read", Path: "a.png", Err: syscall.EIO}, fallback: shared.DiagnosticFileInvalidFormat},
		{name: "storage errno", cause: syscall.EIO, fallback: shared.DiagnosticFileInvalidFormat},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rendered := fmt.Errorf("upload: %w", markMediaInput(test.cause, test.fallback))
			err := ClassifyMediaInputError(rendered, "--path")
			if got, want := err.Error(), rendered.Error(); got != want {
				t.Fatalf("ClassifyMediaInputError().Error() = %q, want %q", got, want)
			}
			if !errors.Is(err, test.cause) {
				t.Fatalf("ClassifyMediaInputError() lost the cause: %v", err)
			}
			diagnostic, ok := shared.DiagnosticFromError(err)
			if !test.wantDetected {
				if ok || shared.IsValidationError(err) {
					t.Fatalf("storage failure classified as %+v (validation=%t)", diagnostic, shared.IsValidationError(err))
				}
				return
			}
			if !shared.IsValidationError(err) || !ok || diagnostic.Code != test.wantCode || diagnostic.Parameter != "--path" {
				t.Fatalf("diagnostic = %+v, %t; want validation %q for --path", diagnostic, ok, test.wantCode)
			}
		})
	}
}
