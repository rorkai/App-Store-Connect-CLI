//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package cmdtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestCertificatesCSRGenerate_ForceRejectsNamedPipeOutputsBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	keyOut := filepath.Join(dir, "cert.key")
	if err := os.WriteFile(keyOut, []byte("original-private-key"), 0o600); err != nil {
		t.Fatalf("WriteFile(keyOut) error: %v", err)
	}
	originalKeyInfo, err := os.Stat(keyOut)
	if err != nil {
		t.Fatalf("Stat(keyOut) error: %v", err)
	}
	csrOut := filepath.Join(dir, "cert.csr")
	if err := unix.Mkfifo(csrOut, 0o600); err != nil {
		t.Fatalf("Mkfifo(csrOut) error: %v", err)
	}

	stdout, _, runErr := runRootWithDeps(t, nil,
		"certificates", "csr", "generate",
		"--key-out", keyOut,
		"--csr-out", csrOut,
		"--force",
		"--output", "json",
	)

	if runErr == nil || !strings.Contains(runErr.Error(), "write --csr-out") ||
		!strings.Contains(runErr.Error(), csrOut) ||
		!strings.Contains(runErr.Error(), "is not a regular file") {
		t.Fatalf("run error = %q, want owning flag, path, and non-regular rejection", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty output", stdout)
	}
	fifoInfo, err := os.Lstat(csrOut)
	if err != nil || fifoInfo.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("Lstat(csrOut) = (%v, %v), want preserved named pipe", fifoInfo, err)
	}
	currentKeyInfo, err := os.Stat(keyOut)
	if err != nil {
		t.Fatalf("Stat(keyOut) after failure error: %v", err)
	}
	if !os.SameFile(originalKeyInfo, currentKeyInfo) {
		t.Error("key inode changed before deterministic CSR destination failure")
	}
	keyContents, err := os.ReadFile(keyOut)
	if err != nil || string(keyContents) != "original-private-key" {
		t.Fatalf("ReadFile(keyOut) = (%q, %v), want original key", keyContents, err)
	}
}
