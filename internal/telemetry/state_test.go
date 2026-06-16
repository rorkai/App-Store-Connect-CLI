package telemetry

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestInstallIDCreateReuseAndReset(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")

	first, err := EnsureInstallID()
	if err != nil {
		t.Fatalf("EnsureInstallID() error = %v", err)
	}
	if first == "" {
		t.Fatal("expected install ID")
	}

	second, err := EnsureInstallID()
	if err != nil {
		t.Fatalf("EnsureInstallID() second error = %v", err)
	}
	if second != first {
		t.Fatalf("expected reused install ID %q, got %q", first, second)
	}

	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat telemetry state: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("state file permissions = %o, want 0600", got)
	}
	if dirMode := statMode(t, filepath.Dir(path)); dirMode != 0o700 {
		t.Fatalf("state dir permissions = %o, want 0700", dirMode)
	}

	reset, err := ResetInstallID()
	if err != nil {
		t.Fatalf("ResetInstallID() error = %v", err)
	}
	if reset == "" || reset == first {
		t.Fatalf("expected new install ID, got %q after %q", reset, first)
	}
}

func TestEnsureInstallIDDoesNotRewriteUnchangedState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, err := EnsureInstallID(); err != nil {
		t.Fatalf("EnsureInstallID() error = %v", err)
	}
	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}
	firstInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat telemetry state: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	if _, err := EnsureInstallID(); err != nil {
		t.Fatalf("EnsureInstallID() second error = %v", err)
	}
	secondInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat telemetry state after reuse: %v", err)
	}
	if !secondInfo.ModTime().Equal(firstInfo.ModTime()) {
		t.Fatalf("state modification time changed from %v to %v", firstInfo.ModTime(), secondInfo.ModTime())
	}
}

func TestExistingUnlockedLockFileIsReusable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}
	lockPath := path + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		t.Fatalf("create state directory: %v", err)
	}
	if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
		t.Fatalf("create unlocked lock file: %v", err)
	}

	if _, err := EnsureInstallID(); err != nil {
		t.Fatalf("EnsureInstallID() with existing lock file error = %v", err)
	}
}

func TestAgedLockStillPreservesMutualExclusion(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}
	unlockFirst, err := lockState(path, lockTimeout)
	if err != nil {
		t.Fatalf("lockState() first error = %v", err)
	}
	defer unlockFirst()

	lockPath := path + ".lock"
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(lockPath, oldTime, oldTime); err != nil {
		t.Fatalf("age held lock: %v", err)
	}

	unlockSecond, err := lockState(path, 0)
	if err == nil {
		unlockSecond()
		t.Fatal("second caller acquired an aged lock while it was still held")
	}
}

func TestReadStatusHonorsOptOuts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")

	if err := SetEnabled(false); err != nil {
		t.Fatalf("SetEnabled(false) error = %v", err)
	}
	status, err := ReadStatus()
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}
	if status.Enabled || status.Reason != "state" {
		t.Fatalf("expected state-disabled status, got %+v", status)
	}

	if err := SetEnabled(true); err != nil {
		t.Fatalf("SetEnabled(true) error = %v", err)
	}
	t.Setenv("DO_NOT_TRACK", "1")
	status, err = ReadStatus()
	if err != nil {
		t.Fatalf("ReadStatus() with env error = %v", err)
	}
	if status.Enabled || status.Reason != "DO_NOT_TRACK" {
		t.Fatalf("expected DO_NOT_TRACK-disabled status, got %+v", status)
	}
}

func TestConcurrentStateUpdatesPreserveOptOutAndInstallID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")

	var wg sync.WaitGroup
	errs := make(chan error, 100)
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, err := EnsureInstallID()
			errs <- err
		}()
		go func() {
			defer wg.Done()
			errs <- SetEnabled(false)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent state update failed: %v", err)
		}
	}

	status, err := ReadStatus()
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}
	if status.Enabled || status.Reason != "state" {
		t.Fatalf("expected opt-out to survive concurrent updates, got %+v", status)
	}
	if status.InstallID == "" {
		t.Fatalf("expected install ID to survive concurrent updates, got %+v", status)
	}
}

func statMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}
