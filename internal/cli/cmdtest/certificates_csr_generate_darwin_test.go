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

func TestCertificatesCSRGenerate_RejectsMountAliasEquivalentOutputsBeforeWriting(t *testing.T) {
	physicalParent, err := os.MkdirTemp("/private/tmp", "asc-csr-mount-alias-")
	if err != nil {
		t.Fatalf("MkdirTemp() error: %v", err)
	}
	t.Cleanup(func() {
		entries, readErr := os.ReadDir(physicalParent)
		if readErr == nil {
			for _, entry := range entries {
				if removeErr := os.Remove(filepath.Join(physicalParent, entry.Name())); removeErr != nil {
					t.Errorf("remove mount-alias test artifact %q: %v", entry.Name(), removeErr)
				}
			}
		}
		if removeErr := os.Remove(physicalParent); removeErr != nil {
			t.Errorf("remove mount-alias test directory: %v", removeErr)
		}
	})

	aliasParent := filepath.Join("/System/Volumes/Data", physicalParent)
	physicalInfo, err := os.Stat(physicalParent)
	if err != nil {
		t.Fatalf("Stat(physicalParent) error: %v", err)
	}
	aliasInfo, err := os.Stat(aliasParent)
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("macOS data-volume alias is unavailable")
	}
	if err != nil {
		t.Fatalf("Stat(aliasParent) error: %v", err)
	}
	if !os.SameFile(physicalInfo, aliasInfo) {
		t.Skip("macOS data-volume paths do not alias the same directory")
	}
	physicalResolved, err := filepath.EvalSymlinks(physicalParent)
	if err != nil {
		t.Fatalf("EvalSymlinks(physicalParent) error: %v", err)
	}
	aliasResolved, err := filepath.EvalSymlinks(aliasParent)
	if err != nil {
		t.Fatalf("EvalSymlinks(aliasParent) error: %v", err)
	}
	if physicalResolved == aliasResolved {
		t.Skip("mount aliases are already canonicalized as symlinks")
	}

	for _, force := range []bool{false, true} {
		name := "without force"
		if force {
			name = "with force"
		}
		t.Run(name, func(t *testing.T) {
			keyOut := filepath.Join(physicalParent, strings.ReplaceAll(name, " ", "-"))
			csrOut := filepath.Join(aliasParent, strings.ReplaceAll(name, " ", "-"))
			args := []string{
				"certificates", "csr", "generate",
				"--key-out", keyOut,
				"--csr-out", csrOut,
				"--output", "json",
			}
			if force {
				args = append(args, "--force")
			}

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			var runErr error
			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse command: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Errorf("expected usage error, got %v", runErr)
			}
			const expected = "--key-out and --csr-out must be different paths"
			if runErr == nil || runErr.Error() != expected {
				t.Errorf("error = %v, want %q", runErr, expected)
			}
			if stdout != "" {
				t.Errorf("expected empty stdout, got %q", stdout)
			}
			entries, err := os.ReadDir(physicalParent)
			if err != nil {
				t.Fatalf("ReadDir(physicalParent) error: %v", err)
			}
			if len(entries) != 0 {
				t.Errorf("outputs were created before mount aliases were rejected: %v", entries)
			}
		})
	}
}
