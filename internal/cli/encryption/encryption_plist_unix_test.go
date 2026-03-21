//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package encryption

import (
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestUpdatePlistExemption_RejectsNamedPipe(t *testing.T) {
	pipePath := filepath.Join(t.TempDir(), "Info.plist")
	if err := unix.Mkfifo(pipePath, 0o644); err != nil {
		t.Fatalf("Mkfifo() error: %v", err)
	}

	err := updatePlistExemption(pipePath)
	if err == nil {
		t.Fatal("expected non-regular file error, got nil")
	}
	if !strings.Contains(err.Error(), "refusing to read non-regular file") {
		t.Fatalf("expected non-regular file error, got %v", err)
	}
}
