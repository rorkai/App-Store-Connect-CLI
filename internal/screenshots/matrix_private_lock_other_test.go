//go:build !windows

package screenshots

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRestoreMatrixPrivateAttemptDirectoryRetainsHandleAfterFailure(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })

	handle, err := lockMatrixPrivateAttemptDirectoryRetained(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = unlockMatrixPrivateAttemptDirectoryRetained(handle, root)
		_ = closeMatrixPrivateAttemptDACLHandle(handle)
	})

	if err := restoreMatrixPrivateAttemptDirectory(handle, nil); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("restore with unavailable root error = %v, want %v", err, os.ErrInvalid)
	}
	if !handle.locked {
		t.Fatal("retained handle closed after failed restoration")
	}
	if err := restoreMatrixPrivateAttemptDirectory(handle, root); err != nil {
		t.Fatalf("retry restoration: %v", err)
	}
	if handle.locked {
		t.Fatal("retained handle remained open after successful restoration")
	}
}

func TestFinalizeMatrixPrivateAttemptDirectoryClosesHandleAfterPersistentFailure(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	handle, err := lockMatrixPrivateAttemptDirectoryRetained(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Chmod(".", 0o700); err != nil {
			t.Errorf("restore private attempt directory mode: %v", err)
		}
	}()

	if err := finalizeMatrixPrivateAttemptDirectory(handle, nil); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("finalize with unavailable root error = %v, want %v", err, os.ErrInvalid)
	}
	if handle.locked {
		t.Fatal("retained directory handle remained open after terminal restoration failure")
	}
}

func TestRetryMatrixPrivateAttemptDirectoryRestoreRecoversTransientFailure(t *testing.T) {
	transientErr := errors.New("transient restore failure")
	attempts := 0
	err := retryMatrixPrivateAttemptDirectoryRestoreWith(nil, nil, func(*matrixPrivateAttemptDACLHandle, *os.Root) error {
		attempts++
		if attempts == 1 {
			return transientErr
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retry restore error = %v, want recovery", err)
	}
	if attempts != 2 {
		t.Fatalf("restore attempts = %d, want 2", attempts)
	}
}

func TestFinalizeMatrixPrivateAttemptDACLHandlePreservesCloseFailure(t *testing.T) {
	closeErr := errors.New("close failed")
	attempts := 0
	err := finalizeMatrixPrivateAttemptDACLHandleWith(nil, func(*matrixPrivateAttemptDACLHandle) error {
		attempts++
		if attempts == 1 {
			return closeErr
		}
		return nil
	})
	if !errors.Is(err, closeErr) {
		t.Fatalf("finalize close error = %v, want %v", err, closeErr)
	}
	if attempts != 2 {
		t.Fatalf("close attempts = %d, want 2", attempts)
	}
}

func TestFinalizeMatrixPrivateAttemptFileClosesHandleAfterPersistentFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.png")
	if err := os.WriteFile(path, []byte("input"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := lockMatrixPrivateAttemptFileRetained(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if err := finalizeMatrixPrivateAttemptFile(handle); err == nil {
		t.Fatal("finalize closed file error = nil, want restoration failure")
	}
	if handle.locked || handle.file != nil {
		t.Fatalf("retained file handle not terminally closed: locked=%t file=%v", handle.locked, handle.file)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
}
