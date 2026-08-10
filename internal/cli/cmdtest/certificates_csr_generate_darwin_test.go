//go:build darwin

package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestCertificatesCSRGenerate_ForceRestoresKeyWhenCSRDestinationRejectsReplacement(t *testing.T) {
	for _, keyExists := range []bool{false, true} {
		name := "new key"
		if keyExists {
			name = "existing key"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			keyOut := filepath.Join(dir, "cert.key")
			csrOut := filepath.Join(dir, "cert.csr")
			originalKey := []byte("original-private-key")
			originalCSR := []byte("original-csr")
			if keyExists {
				if err := os.WriteFile(keyOut, originalKey, 0o600); err != nil {
					t.Fatalf("WriteFile(keyOut) error: %v", err)
				}
			}
			if err := os.WriteFile(csrOut, originalCSR, 0o644); err != nil {
				t.Fatalf("WriteFile(csrOut) error: %v", err)
			}
			if err := unix.Chflags(csrOut, unix.UF_IMMUTABLE); err != nil {
				t.Fatalf("mark CSR destination immutable: %v", err)
			}
			defer func() {
				if err := unix.Chflags(csrOut, 0); err != nil {
					t.Errorf("restore CSR destination flags: %v", err)
				}
			}()

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			if err := root.Parse([]string{
				"certificates", "csr", "generate",
				"--key-out", keyOut,
				"--csr-out", csrOut,
				"--force",
				"--output", "json",
			}); err != nil {
				t.Fatalf("parse command: %v", err)
			}

			var runErr error
			stdout, _ := captureOutput(t, func() {
				runErr = root.Run(context.Background())
			})
			if runErr == nil {
				t.Fatal("expected CSR replacement failure")
			}
			if errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected runtime error, got usage error: %v", runErr)
			}
			if !strings.Contains(runErr.Error(), "write --csr-out") {
				t.Fatalf("expected CSR write error, got %v", runErr)
			}
			if stdout != "" {
				t.Errorf("expected empty stdout, got %q", stdout)
			}

			if keyExists {
				keyContents, err := os.ReadFile(keyOut)
				if err != nil {
					t.Fatalf("ReadFile(keyOut) error: %v", err)
				}
				if string(keyContents) != string(originalKey) {
					t.Errorf("existing key changed after CSR replacement failure")
				}
			} else if _, err := os.Lstat(keyOut); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("new key remained after CSR replacement failure: %v", err)
			}

			csrContents, err := os.ReadFile(csrOut)
			if err != nil {
				t.Fatalf("ReadFile(csrOut) error: %v", err)
			}
			if string(csrContents) != string(originalCSR) {
				t.Errorf("CSR destination changed after rejected replacement")
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("ReadDir(dir) error: %v", err)
			}
			wantEntries := 1
			if keyExists {
				wantEntries = 2
			}
			if len(entries) != wantEntries {
				t.Errorf("unexpected pair-write artifacts: %v", entries)
			}
		})
	}
}
