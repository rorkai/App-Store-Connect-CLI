package shared

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSafeWriteFileNoSymlink_RemovesNewFileAfterWriteFailure(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "output.pem")
	writeFailure := errors.New("injected write failure")

	_, err := SafeWriteFileNoSymlink(
		destination,
		0o600,
		false,
		".test-write-*",
		".test-backup-*",
		func(file *os.File) (int64, error) {
			written, err := file.Write([]byte("partial"))
			if err != nil {
				return int64(written), err
			}
			return int64(written), writeFailure
		},
	)
	if !errors.Is(err, writeFailure) {
		t.Fatalf("SafeWriteFileNoSymlink() error = %v, want write failure", err)
	}
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial destination remained after write failure: %v", err)
	}
}

func TestWriteNewFileNoSymlink_RemovesNewFileAfterSyncFailure(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "output.pem")
	syncFailure := errors.New("injected sync failure")

	_, err := writeNewFileNoSymlink(
		destination,
		0o600,
		func(file *os.File) (int64, error) {
			written, err := file.Write([]byte("complete but not durable"))
			return int64(written), err
		},
		func(*os.File) error {
			return syncFailure
		},
	)
	if !errors.Is(err, syncFailure) {
		t.Fatalf("writeNewFileNoSymlink() error = %v, want sync failure", err)
	}
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination remained after sync failure: %v", err)
	}
}

func TestWriteNewFileNoSymlink_DoesNotRemoveChangedDestination(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not permit replacing the pathname while its original file is open")
	}
	destination := filepath.Join(t.TempDir(), "output.pem")
	writeFailure := errors.New("injected write failure")
	replacement := []byte("concurrent replacement")

	_, err := writeNewFileNoSymlink(
		destination,
		0o600,
		func(file *os.File) (int64, error) {
			written, err := file.Write([]byte("partial"))
			if err != nil {
				return int64(written), err
			}
			if err := os.Remove(destination); err != nil {
				return int64(written), err
			}
			if err := os.WriteFile(destination, replacement, 0o600); err != nil {
				return int64(written), err
			}
			return int64(written), writeFailure
		},
		func(file *os.File) error {
			return file.Sync()
		},
	)
	if !errors.Is(err, writeFailure) {
		t.Fatalf("writeNewFileNoSymlink() error = %v, want write failure", err)
	}
	if !strings.Contains(err.Error(), "refusing to remove changed output") {
		t.Fatalf("writeNewFileNoSymlink() error = %q, want changed-output diagnostic", err)
	}
	contents, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("ReadFile(destination) error: %v", err)
	}
	if string(contents) != string(replacement) {
		t.Fatalf("destination contents = %q, want concurrent replacement", contents)
	}
}
