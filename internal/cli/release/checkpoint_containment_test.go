package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveCheckpointDoesNotWriteThroughPredictableTemporarySymlink(t *testing.T) {
	d := releaseTestDeps()
	dir := t.TempDir()
	sentinelPath := filepath.Join(t.TempDir(), "sentinel.txt")
	writeContainmentFile(t, sentinelPath, "original")

	checkpointPath := filepath.Join(dir, "release-checkpoint.json")
	if err := os.Symlink(sentinelPath, checkpointPath+".tmp"); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	checkpoint := runCheckpoint{AppID: "123", Version: "1.0.0", Completed: map[string]bool{stepEnsureVersion: true}}
	if err := saveCheckpoint(d, checkpointPath, checkpoint); err != nil {
		t.Fatalf("saveCheckpoint(d, ) error = %v", err)
	}

	if got := readContainmentFile(t, sentinelPath); got != "original" {
		t.Fatalf("sentinel content = %q, want %q", got, "original")
	}
	loaded, err := loadCheckpoint(releaseTestDeps(), checkpointPath)
	if err != nil {
		t.Fatalf("loadCheckpoint() error = %v", err)
	}
	if loaded == nil || loaded.AppID != "123" {
		t.Fatalf("loadCheckpoint() = %#v, want persisted checkpoint", loaded)
	}
}

func TestSaveCheckpointRefusesSymlinkedDestination(t *testing.T) {
	d := releaseTestDeps()
	dir := t.TempDir()
	sentinelPath := filepath.Join(t.TempDir(), "sentinel.txt")
	writeContainmentFile(t, sentinelPath, "original")

	checkpointPath := filepath.Join(dir, "release-checkpoint.json")
	if err := os.Symlink(sentinelPath, checkpointPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	err := saveCheckpoint(d, checkpointPath, runCheckpoint{AppID: "123", Completed: map[string]bool{}})
	if err == nil {
		t.Fatal("saveCheckpoint(d, ) error = nil, want symlink rejection")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("saveCheckpoint(d, ) error = %v, want symlink rejection", err)
	}
	if got := readContainmentFile(t, sentinelPath); got != "original" {
		t.Fatalf("sentinel content = %q, want %q", got, "original")
	}
}

func TestSaveCheckpointRefusesSymlinkedRepositoryDirectoryComponent(t *testing.T) {
	d := releaseTestDeps()
	workDir := t.TempDir()
	t.Chdir(workDir)

	externalDir := t.TempDir()
	checkpointDir := defaultStageCheckpointPath("123", "1.0.0", "456", "IOS")
	if err := os.MkdirAll(filepath.Dir(filepath.Dir(checkpointDir)), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.Symlink(externalDir, filepath.Dir(checkpointDir)); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	err := saveCheckpoint(d, checkpointDir, runCheckpoint{AppID: "123", Completed: map[string]bool{}})
	if err == nil {
		t.Fatal("saveCheckpoint(d, ) error = nil, want symlink rejection")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("saveCheckpoint(d, ) error = %v, want symlink rejection", err)
	}
	entries, readErr := os.ReadDir(externalDir)
	if readErr != nil {
		t.Fatalf("ReadDir() error = %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("checkpoint escaped into the symlink target: %v", entries)
	}
}

func TestSaveCheckpointAllowsOperatorSelectedExternalCheckpointRoot(t *testing.T) {
	d := releaseTestDeps()
	workDir := t.TempDir()
	t.Chdir(workDir)

	// An explicitly selected checkpoint directory outside the working directory
	// is the operator's trusted root, even when reached through a symlink.
	realDir := t.TempDir()
	linkedDir := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(realDir, linkedDir); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	checkpointPath := filepath.Join(linkedDir, "release-checkpoint.json")
	if err := saveCheckpoint(d, checkpointPath, runCheckpoint{AppID: "123", Completed: map[string]bool{}}); err != nil {
		t.Fatalf("saveCheckpoint(d, ) error = %v", err)
	}
	if got := readContainmentFile(t, filepath.Join(realDir, "release-checkpoint.json")); !strings.Contains(got, `"appId": "123"`) {
		t.Fatalf("checkpoint content = %q", got)
	}
}

func TestLoadCheckpointRefusesSymlinkedCheckpoint(t *testing.T) {
	dir := t.TempDir()
	externalPath := filepath.Join(t.TempDir(), "external.json")
	writeContainmentFile(t, externalPath, `{"appId":"external"}`)

	checkpointPath := filepath.Join(dir, "release-checkpoint.json")
	if err := os.Symlink(externalPath, checkpointPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	loaded, err := loadCheckpoint(releaseTestDeps(), checkpointPath)
	if err == nil {
		t.Fatalf("loadCheckpoint() error = nil, want symlink rejection (got %#v)", loaded)
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("loadCheckpoint() error = %v, want symlink rejection", err)
	}
}

func TestSaveAndLoadCheckpointRoundTripsOrdinaryFile(t *testing.T) {
	d := releaseTestDeps()
	checkpointPath := filepath.Join(t.TempDir(), "nested", "release-checkpoint.json")
	checkpoint := runCheckpoint{
		AppID:     "123",
		Version:   "1.2.3",
		BuildID:   "456",
		Platform:  "IOS",
		Completed: map[string]bool{stepEnsureVersion: true},
	}

	if err := saveCheckpoint(d, checkpointPath, checkpoint); err != nil {
		t.Fatalf("saveCheckpoint(d, ) error = %v", err)
	}
	if err := saveCheckpoint(d, checkpointPath, checkpoint); err != nil {
		t.Fatalf("saveCheckpoint(d, ) overwrite error = %v", err)
	}

	loaded, err := loadCheckpoint(releaseTestDeps(), checkpointPath)
	if err != nil {
		t.Fatalf("loadCheckpoint() error = %v", err)
	}
	if loaded == nil || loaded.BuildID != "456" || !loaded.Completed[stepEnsureVersion] {
		t.Fatalf("loadCheckpoint() = %#v, want round-tripped checkpoint", loaded)
	}

	info, err := os.Lstat(checkpointPath)
	if err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("checkpoint mode = %v, want 0600", info.Mode().Perm())
	}
	entries, err := os.ReadDir(filepath.Dir(checkpointPath))
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, entry := range entries {
		if entry.Name() != "release-checkpoint.json" {
			t.Fatalf("unexpected leftover file %q in checkpoint directory", entry.Name())
		}
	}
}

func writeContainmentFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func readContainmentFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return string(data)
}
