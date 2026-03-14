package config

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestSaveAtSyncsDirectoriesDuringWrite(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.json")

	previousSyncDirFn := syncDirFn
	var synced []string
	syncDirFn = func(path string) error {
		synced = append(synced, path)
		return nil
	}
	t.Cleanup(func() {
		syncDirFn = previousSyncDirFn
	})

	if err := SaveAt(path, &Config{KeyID: "KEY123"}); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	if !containsPath(synced, filepath.Dir(tempDir)) {
		t.Fatalf("expected parent directory sync, got %v", synced)
	}
	configDirSyncs := 0
	for _, syncedPath := range synced {
		if syncedPath == tempDir {
			configDirSyncs++
		}
	}
	if configDirSyncs < 2 {
		t.Fatalf("expected config directory to be synced before and after rename, got %v", synced)
	}
}

func TestSaveAtPropagatesDirectorySyncErrors(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.json")

	syncErr := errors.New("sync failure")
	previousSyncDirFn := syncDirFn
	syncDirFn = func(string) error {
		return syncErr
	}
	t.Cleanup(func() {
		syncDirFn = previousSyncDirFn
	})

	err := SaveAt(path, &Config{KeyID: "KEY123"})
	if err == nil {
		t.Fatal("expected SaveAt() to fail when directory sync fails")
	}
	if !errors.Is(err, syncErr) {
		t.Fatalf("expected sync error, got %v", err)
	}
}

func containsPath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}
