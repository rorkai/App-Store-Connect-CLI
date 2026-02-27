//go:build darwin

package screenshots

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestFormatMacOSWindowLookupError_SwiftMissingIncludesInstallHint(t *testing.T) {
	err := &exec.Error{Name: "swift", Err: exec.ErrNotFound}

	got := formatMacOSWindowLookupError("com.example.app", err, nil)
	if got == nil {
		t.Fatal("expected error")
	}

	msg := got.Error()
	if !strings.Contains(msg, "swift not found in PATH") {
		t.Fatalf("expected missing swift message, got %q", msg)
	}
	if !strings.Contains(msg, "xcode-select --install") {
		t.Fatalf("expected CLT install hint, got %q", msg)
	}
}

func TestFormatMacOSWindowLookupError_MissingCLTIncludesInstallHint(t *testing.T) {
	err := &exec.ExitError{
		Stderr: []byte("xcrun: error: invalid active developer path (/Library/Developer/CommandLineTools), missing xcrun"),
	}

	got := formatMacOSWindowLookupError("com.example.app", err, nil)
	if got == nil {
		t.Fatal("expected error")
	}

	msg := got.Error()
	if !strings.Contains(msg, "invalid active developer path") {
		t.Fatalf("expected CLT failure details, got %q", msg)
	}
	if !strings.Contains(msg, "xcode-select --install") {
		t.Fatalf("expected CLT install hint, got %q", msg)
	}
}

func TestFormatMacOSWindowLookupError_AppNotRunningNoCLTHint(t *testing.T) {
	err := errors.New("exit status 1")
	out := []byte("app not running: com.example.app")

	got := formatMacOSWindowLookupError("com.example.app", err, out)
	if got == nil {
		t.Fatal("expected error")
	}

	msg := got.Error()
	if !strings.Contains(msg, "app not running") {
		t.Fatalf("expected runtime details in message, got %q", msg)
	}
	if strings.Contains(msg, "xcode-select --install") {
		t.Fatalf("did not expect CLT install hint for app runtime error, got %q", msg)
	}
}
