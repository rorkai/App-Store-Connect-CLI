package certificates

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
