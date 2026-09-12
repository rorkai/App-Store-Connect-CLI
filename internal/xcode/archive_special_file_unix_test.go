//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package xcode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestReadRegularFileFromRootRejectsFIFOWithoutBlocking(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "Info.plist")
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatalf("Mkfifo() error: %v", err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("OpenRoot() error: %v", err)
	}
	defer root.Close()

	done := make(chan error, 1)
	go func() {
		_, readErr := readRegularFileFromRoot(root, "Info.plist")
		done <- readErr
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("readRegularFileFromRoot() error = %v, want regular-file rejection", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("readRegularFileFromRoot() blocked while opening a FIFO")
	}
}
