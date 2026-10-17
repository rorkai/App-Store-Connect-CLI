package shots

import (
	"errors"
	"fmt"
	"io/fs"
	"syscall"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

func TestClassifyFrameErrorLeavesStorageFailuresUnclassified(t *testing.T) {
	tests := []struct {
		name          string
		cause         error
		wantCode      shared.DiagnosticCode
		wantUnchanged bool
	}{
		{name: "missing", cause: &fs.PathError{Op: "lstat", Path: "in.png", Err: syscall.ENOENT}, wantCode: shared.DiagnosticFileNotFound},
		{name: "unreadable", cause: &fs.PathError{Op: "open", Path: "in.png", Err: syscall.EACCES}, wantCode: shared.DiagnosticFilePermissionDenied},
		{name: "not a regular file", cause: errors.New(`expected regular file: "in.png"`), wantCode: shared.DiagnosticFileInvalidFormat},
		{name: "storage path error", cause: &fs.PathError{Op: "read", Path: "in.png", Err: syscall.EIO}, wantUnchanged: true},
		{name: "storage errno", cause: syscall.EIO, wantUnchanged: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rendered := fmt.Errorf("screenshots frame: %w", &screenshots.FrameInputError{
				Input: screenshots.FrameInputScreenshot,
				Err:   fmt.Errorf("read input screenshot: %w", test.cause),
			})
			err := classifyFrameError(rendered)
			if got, want := err.Error(), rendered.Error(); got != want {
				t.Fatalf("classifyFrameError().Error() = %q, want %q", got, want)
			}
			diagnostic, ok := shared.DiagnosticFromError(err)
			if test.wantUnchanged {
				if ok || shared.IsValidationError(err) {
					t.Fatalf("classifyFrameError() = %+v (validation=%t), want unclassified", diagnostic, shared.IsValidationError(err))
				}
				return
			}
			if !ok || diagnostic.Code != test.wantCode || diagnostic.Parameter != "--input" {
				t.Fatalf("classifyFrameError() diagnostic = %+v, %t; want %q for --input", diagnostic, ok, test.wantCode)
			}
		})
	}
}
