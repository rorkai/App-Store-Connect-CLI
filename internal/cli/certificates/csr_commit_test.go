package certificates

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestCommitCSRWrites_RestoresKeyWhenSecondWriteFails(t *testing.T) {
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
				if err := os.WriteFile(keyOut, originalKey, 0o640); err != nil {
					t.Fatalf("WriteFile(keyOut) error: %v", err)
				}
			}
			if err := os.WriteFile(csrOut, originalCSR, 0o644); err != nil {
				t.Fatalf("WriteFile(csrOut) error: %v", err)
			}

			keyWrite, err := prepareCSRWrite("--key-out", keyOut, []byte("replacement-key"), 0o600, true)
			if err != nil {
				t.Fatalf("prepareCSRWrite() error: %v", err)
			}
			writeFailure := errors.New("destination rejects replacement")
			writeFile := func(path string, data []byte, mode os.FileMode, force bool) error {
				if path == csrOut {
					return writeFailure
				}
				return writeFileBytesNoSymlink(path, data, mode, force)
			}

			err = commitCSRWrites([]preparedCSRWrite{
				keyWrite,
				{flag: "--csr-out", path: csrOut, data: []byte("replacement-csr"), mode: 0o644, force: true},
			}, writeFile)
			if !errors.Is(err, writeFailure) {
				t.Fatalf("commitCSRWrites() error = %v, want destination failure", err)
			}
			if !strings.Contains(err.Error(), "write --csr-out") {
				t.Errorf("commitCSRWrites() error = %q, want CSR flag", err)
			}

			if keyExists {
				keyContents, err := os.ReadFile(keyOut)
				if err != nil {
					t.Fatalf("ReadFile(keyOut) error: %v", err)
				}
				if string(keyContents) != string(originalKey) {
					t.Errorf("key contents = %q, want original contents", keyContents)
				}
				info, err := os.Stat(keyOut)
				if err != nil {
					t.Fatalf("Stat(keyOut) error: %v", err)
				}
				if got := info.Mode().Perm(); got != 0o640 {
					t.Errorf("key mode = %o, want 640", got)
				}
			} else if _, err := os.Lstat(keyOut); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("new key remained after second write failed: %v", err)
			}
			csrContents, err := os.ReadFile(csrOut)
			if err != nil {
				t.Fatalf("ReadFile(csrOut) error: %v", err)
			}
			if string(csrContents) != string(originalCSR) {
				t.Errorf("CSR contents = %q, want original contents", csrContents)
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

func TestCommitCSRWrites_RollbackPreservesKeyIdentity(t *testing.T) {
	dir := t.TempDir()
	keyOut := filepath.Join(dir, "cert.key")
	keyAlias := filepath.Join(dir, "cert.key.alias")
	csrOut := filepath.Join(dir, "cert.csr")
	originalKey := []byte("original-private-key")
	if err := os.WriteFile(keyOut, originalKey, 0o640); err != nil {
		t.Fatalf("WriteFile(keyOut) error: %v", err)
	}
	if err := os.Link(keyOut, keyAlias); err != nil {
		t.Skipf("temporary volume does not support hard links: %v", err)
	}

	keyWrite, err := prepareCSRWrite("--key-out", keyOut, []byte("replacement-key"), 0o600, true)
	if err != nil {
		t.Fatalf("prepareCSRWrite() error: %v", err)
	}
	writeFailure := errors.New("injected CSR write failure")
	writeFile := func(path string, data []byte, mode os.FileMode, force bool) error {
		if path == csrOut {
			return writeFailure
		}
		return writeFileBytesNoSymlink(path, data, mode, force)
	}

	err = commitCSRWrites([]preparedCSRWrite{
		keyWrite,
		{flag: "--csr-out", path: csrOut, data: []byte("replacement-csr"), mode: 0o644, force: true},
	}, writeFile)
	if !errors.Is(err, writeFailure) {
		t.Fatalf("commitCSRWrites() error = %v, want CSR write failure", err)
	}

	keyContents, err := os.ReadFile(keyOut)
	if err != nil {
		t.Fatalf("ReadFile(keyOut) error: %v", err)
	}
	if string(keyContents) != string(originalKey) {
		t.Errorf("key contents = %q, want original contents", keyContents)
	}
	keyInfo, err := os.Lstat(keyOut)
	if err != nil {
		t.Fatalf("Lstat(keyOut) error: %v", err)
	}
	aliasInfo, err := os.Lstat(keyAlias)
	if err != nil {
		t.Fatalf("Lstat(keyAlias) error: %v", err)
	}
	if !os.SameFile(keyInfo, aliasInfo) {
		t.Error("rollback replaced the key's directory entry instead of restoring the original file")
	}
	if got := keyInfo.Mode().Perm(); got != 0o640 {
		t.Errorf("key mode = %o, want 640", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(dir) error: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("unexpected pair-write artifacts: %v", entries)
	}
}

func TestCommitCSRWrites_ForceReplacesUnreadableKey(t *testing.T) {
	for _, csrFails := range []bool{false, true} {
		name := "csr write succeeds"
		if csrFails {
			name = "csr write fails"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			keyOut := filepath.Join(dir, "cert.key")
			keyAlias := filepath.Join(dir, "cert.key.alias")
			csrOut := filepath.Join(dir, "cert.csr")
			originalKey := []byte("original-private-key")
			if err := os.WriteFile(keyOut, originalKey, 0o640); err != nil {
				t.Fatalf("WriteFile(keyOut) error: %v", err)
			}
			if err := os.Link(keyOut, keyAlias); err != nil {
				t.Skipf("temporary volume does not support hard links: %v", err)
			}
			if err := os.Chmod(keyOut, 0o000); err != nil {
				t.Fatalf("Chmod(keyOut) error: %v", err)
			}
			t.Cleanup(func() {
				_ = os.Chmod(keyOut, 0o600)
				_ = os.Chmod(keyAlias, 0o600)
			})
			if probe, err := os.Open(keyOut); err == nil {
				_ = probe.Close()
				t.Skip("environment does not enforce file read permissions")
			}

			keyWrite, err := prepareCSRWrite("--key-out", keyOut, []byte("replacement-key"), 0o600, true)
			if err != nil {
				t.Fatalf("prepareCSRWrite() error: %v", err)
			}
			writeFailure := errors.New("injected CSR write failure")
			writeFile := func(path string, data []byte, mode os.FileMode, force bool) error {
				if csrFails && path == csrOut {
					return writeFailure
				}
				return writeFileBytesNoSymlink(path, data, mode, force)
			}

			err = commitCSRWrites([]preparedCSRWrite{
				keyWrite,
				{flag: "--csr-out", path: csrOut, data: []byte("replacement-csr"), mode: 0o644, force: true},
			}, writeFile)

			keyInfo, statErr := os.Lstat(keyOut)
			if statErr != nil {
				t.Fatalf("Lstat(keyOut) error: %v", statErr)
			}
			aliasInfo, statErr := os.Lstat(keyAlias)
			if statErr != nil {
				t.Fatalf("Lstat(keyAlias) error: %v", statErr)
			}

			if csrFails {
				if !errors.Is(err, writeFailure) {
					t.Fatalf("commitCSRWrites() error = %v, want CSR write failure", err)
				}
				if !os.SameFile(keyInfo, aliasInfo) {
					t.Error("rollback did not restore the original unreadable key file")
				}
				if got := keyInfo.Mode().Perm(); got != 0o000 {
					t.Errorf("restored key mode = %o, want 000", got)
				}
				if err := os.Chmod(keyOut, 0o600); err != nil {
					t.Fatalf("Chmod(keyOut) error: %v", err)
				}
				keyContents, err := os.ReadFile(keyOut)
				if err != nil {
					t.Fatalf("ReadFile(keyOut) error: %v", err)
				}
				if string(keyContents) != string(originalKey) {
					t.Errorf("key contents = %q, want original contents", keyContents)
				}
			} else {
				if err != nil {
					t.Fatalf("commitCSRWrites() error = %v, want forced replacement of unreadable key", err)
				}
				if os.SameFile(keyInfo, aliasInfo) {
					t.Error("forced replacement kept the original key file")
				}
				keyContents, err := os.ReadFile(keyOut)
				if err != nil {
					t.Fatalf("ReadFile(keyOut) error: %v", err)
				}
				if string(keyContents) != "replacement-key" {
					t.Errorf("key contents = %q, want replacement contents", keyContents)
				}
				csrContents, err := os.ReadFile(csrOut)
				if err != nil {
					t.Fatalf("ReadFile(csrOut) error: %v", err)
				}
				if string(csrContents) != "replacement-csr" {
					t.Errorf("CSR contents = %q, want replacement contents", csrContents)
				}
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("ReadDir(dir) error: %v", err)
			}
			wantEntries := 2
			if !csrFails {
				wantEntries = 3
			}
			if len(entries) != wantEntries {
				t.Errorf("unexpected pair-write artifacts: %v", entries)
			}
		})
	}
}

func TestCommitCSRWrites_ReportsRollbackFailure(t *testing.T) {
	writeFailure := errors.New("CSR write failed")
	rollbackFailure := errors.New("key restore failed")
	calls := 0
	writeFile := func(string, []byte, os.FileMode, bool) error {
		calls++
		switch calls {
		case 1:
			return nil
		case 2:
			return writeFailure
		default:
			return rollbackFailure
		}
	}
	err := commitCSRWrites([]preparedCSRWrite{
		{
			flag:         "--key-out",
			path:         "cert.key",
			data:         []byte("replacement-key"),
			mode:         0o600,
			force:        true,
			original:     []byte("original-key"),
			originalMode: 0o600,
			existed:      true,
		},
		{flag: "--csr-out", path: "cert.csr", data: []byte("replacement-csr"), mode: 0o644, force: true},
	}, writeFile)
	if !errors.Is(err, writeFailure) {
		t.Fatalf("commitCSRWrites() error = %v, want write failure", err)
	}
	if !errors.Is(err, rollbackFailure) {
		t.Fatalf("commitCSRWrites() error = %v, want rollback failure", err)
	}
	for _, text := range []string{"write --csr-out", "rollback failed", "restore --key-out"} {
		if !strings.Contains(err.Error(), text) {
			t.Errorf("commitCSRWrites() error = %q, want %q", err, text)
		}
	}
}

func TestCommitCSRWrites_RemovesPartialNonForceCSR(t *testing.T) {
	dir := t.TempDir()
	keyOut := filepath.Join(dir, "cert.key")
	csrOut := filepath.Join(dir, "cert.csr")
	keyWrite, err := prepareCSRWrite("--key-out", keyOut, []byte("replacement-key"), 0o600, false)
	if err != nil {
		t.Fatalf("prepareCSRWrite() error: %v", err)
	}
	writeFailure := errors.New("injected CSR write failure")
	writeFile := func(path string, data []byte, mode os.FileMode, force bool) error {
		if path != csrOut {
			return writeFileBytesNoSymlink(path, data, mode, force)
		}
		_, err := shared.SafeWriteFileNoSymlink(
			path,
			mode,
			false,
			".asc-csr-*",
			".asc-csr-backup-*",
			func(file *os.File) (int64, error) {
				written, err := file.Write([]byte("partial-csr"))
				if err != nil {
					return int64(written), err
				}
				return int64(written), writeFailure
			},
		)
		return err
	}

	err = commitCSRWrites([]preparedCSRWrite{
		keyWrite,
		{flag: "--csr-out", path: csrOut, data: []byte("replacement-csr"), mode: 0o644},
	}, writeFile)
	if !errors.Is(err, writeFailure) {
		t.Fatalf("commitCSRWrites() error = %v, want CSR write failure", err)
	}
	for _, output := range []string{keyOut, csrOut} {
		if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("pair output remained after partial CSR failure: %q (stat error: %v)", output, err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(dir) error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("pair directory contains failure residue: %v", entries)
	}
}
