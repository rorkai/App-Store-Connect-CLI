package telemetry

import (
	"os"
	"path/filepath"
	"testing"
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

func statMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}
