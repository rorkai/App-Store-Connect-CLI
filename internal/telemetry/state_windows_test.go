//go:build windows

package telemetry

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestReplaceStateFileWaitsForConcurrentReader(t *testing.T) {
	setTelemetryTestHome(t)

	if _, err := EnsureInstallID(); err != nil {
		t.Fatalf("EnsureInstallID() error = %v", err)
	}
	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}
	reader := openStateFileWithoutDeleteSharing(t, path)

	replacement, err := os.CreateTemp(filepath.Dir(path), stateFileName+".*.tmp")
	if err != nil {
		t.Fatalf("create replacement state: %v", err)
	}
	replacementPath := replacement.Name()
	t.Cleanup(func() { _ = os.Remove(replacementPath) })
	want := []byte("{\"disabled\":true}\n")
	if _, err := replacement.Write(want); err != nil {
		t.Fatalf("write replacement state: %v", err)
	}
	if err := replacement.Close(); err != nil {
		t.Fatalf("close replacement state: %v", err)
	}
	if err := os.Rename(replacementPath, path); !isRetryableStateReplaceError(err) {
		t.Fatalf("rename with non-delete-sharing reader error = %v, want retryable error", err)
	}

	replaceDone := make(chan error, 1)
	go func() {
		replaceDone <- replaceStateFile(replacementPath, path, lockTimeout)
	}()

	select {
	case err := <-replaceDone:
		t.Fatalf("replaceStateFile() completed while reader was open: %v", err)
	case <-time.After(3 * lockPollInterval):
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close state reader: %v", err)
	}
	select {
	case err := <-replaceDone:
		if err != nil {
			t.Fatalf("replaceStateFile() after reader close error = %v", err)
		}
	case <-time.After(lockTimeout + time.Second):
		t.Fatal("replaceStateFile() did not finish after reader closed")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replaced state: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("replaced state = %q, want %q", got, want)
	}
}

func TestEnsureInstallIDDoesNotWaitForStateReader(t *testing.T) {
	setTelemetryTestHome(t)

	if err := SetEnabled(false); err != nil {
		t.Fatalf("SetEnabled(false) error = %v", err)
	}
	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}
	reader := openStateFileWithoutDeleteSharing(t, path)

	start := time.Now()
	if _, err := ensureInstallID(0); err == nil {
		t.Fatal("ensureInstallID(0) succeeded while state reader blocked replacement")
	}
	if elapsed := time.Since(start); elapsed >= 500*time.Millisecond {
		t.Fatalf("ensureInstallID(0) elapsed = %s, want replacement contention skipped before 500ms", elapsed)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close state reader: %v", err)
	}
}

func TestStateLockIdentityHandleAllowsRename(t *testing.T) {
	setTelemetryTestHome(t)

	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}
	lockPath := path + ".lock"
	if err := os.MkdirAll(lockPath, 0o700); err != nil {
		t.Fatalf("create state lock directory: %v", err)
	}
	identityHandle, err := openStateLockForStat(lockPath)
	if err != nil {
		t.Fatalf("openStateLockForStat() error = %v", err)
	}
	defer identityHandle.Close()

	renamedPath := lockPath + ".renamed"
	if err := os.Rename(lockPath, renamedPath); err != nil {
		t.Fatalf("rename state lock with identity handle open: %v", err)
	}
	info, err := identityHandle.Stat()
	if err != nil {
		t.Fatalf("stat renamed state lock through identity handle: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("identity handle no longer describes the state lock directory")
	}
}

func openStateFileWithoutDeleteSharing(t *testing.T, path string) *os.File {
	t.Helper()

	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatalf("encode state path: %v", err)
	}
	handle, err := windows.CreateFile(
		pathPointer,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatalf("open state without delete sharing: %v", err)
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		_ = windows.CloseHandle(handle)
		t.Fatal("wrap state reader handle")
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}
