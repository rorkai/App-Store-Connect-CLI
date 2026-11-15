//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package notarization

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestNotarizationSubmitRejectsNonRegularArchiveWithoutBlocking(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "archive.pkg")
	if err := unix.Mkfifo(archivePath, 0o600); err != nil {
		t.Skipf("mkfifo not supported: %v", err)
	}

	cmd := submitCommand(newNotarizationTestDeps(nil))
	if err := cmd.FlagSet.Parse([]string{"--file", archivePath}); err != nil {
		t.Fatalf("parse command: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Exec(context.Background(), nil)
	}()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "is not a regular file") {
			t.Fatalf("submit error = %v, want non-regular file rejection", err)
		}
	case <-time.After(time.Second):
		t.Fatal("notarization submit blocked opening a non-regular file")
	}
}
