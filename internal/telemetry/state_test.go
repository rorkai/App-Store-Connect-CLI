package telemetry

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestReadStatusIsEnabledByDefault(t *testing.T) {
	setTelemetryTestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "")
	t.Setenv("DO_NOT_TRACK", "")

	status, err := ReadStatus()
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}
	if !status.Enabled {
		t.Fatalf("expected telemetry enabled by default, got %+v", status)
	}
	if status.Reason != "" {
		t.Fatalf("default status reason = %q, want empty", status.Reason)
	}
}

func TestReadStatusRejectsOversizedState(t *testing.T) {
	setTelemetryTestHome(t)
	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create state directory: %v", err)
	}
	data := []byte(`{"updated_at":"` + strings.Repeat("x", 70*1024) + `"}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write oversized state: %v", err)
	}

	if _, err := ReadStatus(); err == nil {
		t.Fatal("expected oversized telemetry state to be rejected")
	}
}

func TestInstallIDCreateReuseAndReset(t *testing.T) {
	setTelemetryTestHome(t)
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
	if runtime.GOOS != "windows" {
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("state file permissions = %o, want 0600", got)
		}
		if dirMode := statMode(t, filepath.Dir(path)); dirMode != 0o700 {
			t.Fatalf("state dir permissions = %o, want 0700", dirMode)
		}
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
	setTelemetryTestHome(t)

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
	setTelemetryTestHome(t)

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

func TestStaleLegacyLockDirectoryIsMigrated(t *testing.T) {
	setTelemetryTestHome(t)

	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}
	lockPath := path + ".lock"
	if err := os.MkdirAll(lockPath, 0o700); err != nil {
		t.Fatalf("create legacy lock directory: %v", err)
	}
	staleTime := time.Now().Add(-legacyLockStaleAge - time.Second)
	if err := os.Chtimes(lockPath, staleTime, staleTime); err != nil {
		t.Fatalf("age legacy lock directory: %v", err)
	}

	if _, err := EnsureInstallID(); err != nil {
		t.Fatalf("EnsureInstallID() with stale legacy lock error = %v", err)
	}
	info, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("stat migrated lock: %v", err)
	}
	if info.IsDir() {
		t.Fatal("legacy lock directory was not replaced with a lock file")
	}
}

func TestRecentLegacyLockDirectoryIsPreserved(t *testing.T) {
	setTelemetryTestHome(t)

	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}
	lockPath := path + ".lock"
	if err := os.MkdirAll(lockPath, 0o700); err != nil {
		t.Fatalf("create legacy lock directory: %v", err)
	}

	unlock, err := lockState(path, 0)
	if err == nil {
		unlock()
		t.Fatal("acquired a recent legacy lock directory")
	}
	info, statErr := os.Stat(lockPath)
	if statErr != nil {
		t.Fatalf("stat legacy lock directory: %v", statErr)
	}
	if !info.IsDir() {
		t.Fatal("recent legacy lock directory was replaced")
	}
}

func TestLegacyLockMigrationPreservesReplacementDirectory(t *testing.T) {
	setTelemetryTestHome(t)

	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}
	lockPath := path + ".lock"
	if err := os.MkdirAll(lockPath, 0o700); err != nil {
		t.Fatalf("create stale legacy lock directory: %v", err)
	}
	staleInfo, err := statStateLock(lockPath)
	if err != nil {
		t.Fatalf("stat stale legacy lock directory: %v", err)
	}
	replacementPath := lockPath + ".replacement"
	if err := os.Mkdir(replacementPath, 0o700); err != nil {
		t.Fatalf("create replacement legacy lock directory: %v", err)
	}
	stalePath := lockPath + ".stale"
	if err := os.Rename(lockPath, stalePath); err != nil {
		t.Fatalf("preserve stale legacy lock directory: %v", err)
	}
	if err := os.Rename(replacementPath, lockPath); err != nil {
		t.Fatalf("install replacement legacy lock directory: %v", err)
	}

	migrated, err := migrateLegacyStateLockDirectory(lockPath, staleInfo)
	if err != nil {
		t.Fatalf("migrateLegacyStateLockDirectory() error = %v", err)
	}
	if migrated {
		t.Fatal("replacement legacy lock directory was migrated")
	}
	info, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("stat replacement legacy lock directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("replacement legacy lock directory was not preserved")
	}
	if _, err := os.Stat(filepath.Join(lockPath, ".asc-migrating")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("migration marker was not removed: %v", err)
	}
}

func TestStaleLegacyMigrationMarkerIsRecovered(t *testing.T) {
	setTelemetryTestHome(t)

	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}
	lockPath := path + ".lock"
	if err := os.MkdirAll(lockPath, 0o700); err != nil {
		t.Fatalf("create legacy lock directory: %v", err)
	}
	markerPath := filepath.Join(lockPath, ".asc-migrating")
	if err := os.WriteFile(markerPath, nil, 0o600); err != nil {
		t.Fatalf("create stale migration marker: %v", err)
	}
	staleTime := time.Now().Add(-legacyLockStaleAge - time.Second)
	if err := os.Chtimes(markerPath, staleTime, staleTime); err != nil {
		t.Fatalf("age migration marker: %v", err)
	}
	if err := os.Chtimes(lockPath, staleTime, staleTime); err != nil {
		t.Fatalf("age legacy lock directory: %v", err)
	}

	if _, err := EnsureInstallID(); err != nil {
		t.Fatalf("EnsureInstallID() with stale migration marker error = %v", err)
	}
	info, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("stat migrated lock: %v", err)
	}
	if info.IsDir() {
		t.Fatal("legacy lock directory with stale marker was not migrated")
	}
}

func TestAgedLockStillPreservesMutualExclusion(t *testing.T) {
	setTelemetryTestHome(t)

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
	setTelemetryTestHome(t)
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
	setTelemetryTestHome(t)
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

func setTelemetryTestHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

func statMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}
