//go:build unix

package web

import (
	"syscall"
	"testing"
)

func makeWebReviewBodyFIFO(t *testing.T, path string) {
	t.Helper()
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
}
