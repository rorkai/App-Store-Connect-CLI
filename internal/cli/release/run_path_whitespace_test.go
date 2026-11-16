package release

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckpointRootPreservesOperatorPathWhitespace(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantName string
	}{
		{
			name:     "leading whitespace in relative path",
			path:     " release-checkpoint.json",
			wantName: " release-checkpoint.json",
		},
		{
			name:     "trailing whitespace in absolute path",
			path:     filepath.Join(t.TempDir(), "release-checkpoint.json "),
			wantName: "release-checkpoint.json ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, gotName, err := checkpointRoot(releaseTestDeps(), tt.path)
			if err != nil {
				t.Fatalf("checkpointRoot(%q) error = %v", tt.path, err)
			}
			if gotName != tt.wantName {
				t.Fatalf("checkpointRoot(%q) name = %q, want %q", tt.path, gotName, tt.wantName)
			}
		})
	}
}

func TestSaveAndLoadCheckpointPreserveOperatorPathWhitespace(t *testing.T) {
	d := releaseTestDeps()
	checkpointPath := filepath.Join(t.TempDir(), " release-checkpoint.json ")
	checkpoint := runCheckpoint{
		AppID:     "123",
		Version:   "1.2.3",
		BuildID:   "456",
		Platform:  "IOS",
		Completed: map[string]bool{stepEnsureVersion: true},
	}

	if err := saveCheckpoint(d, checkpointPath, checkpoint); err != nil {
		t.Fatalf("saveCheckpoint(d, %q) error = %v", checkpointPath, err)
	}
	if _, err := os.Lstat(checkpointPath); err != nil {
		t.Fatalf("Lstat(%q) error = %v", checkpointPath, err)
	}

	loaded, err := loadCheckpoint(releaseTestDeps(), checkpointPath)
	if err != nil {
		t.Fatalf("loadCheckpoint(%q) error = %v", checkpointPath, err)
	}
	if loaded == nil || loaded.AppID != checkpoint.AppID {
		t.Fatalf("loadCheckpoint(%q) = %#v, want app %q", checkpointPath, loaded, checkpoint.AppID)
	}
}

func TestCheckpointRootResolvesAgainstDepsWorkingDirectory(t *testing.T) {
	processDir, depsDir := t.TempDir(), t.TempDir()
	t.Chdir(processDir)
	d := releaseTestDeps()
	d.Getwd = func() (string, error) { return depsDir, nil }

	root, name, err := checkpointRoot(d, filepath.Join("checkpoints", "run.json"))
	if err != nil {
		t.Fatalf("checkpointRoot() error = %v", err)
	}
	defer root.Close()
	wantRoot, err := filepath.EvalSymlinks(depsDir)
	if err != nil {
		t.Fatalf("resolve deps directory: %v", err)
	}
	gotRoot, err := filepath.EvalSymlinks(root.Path())
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}
	if gotRoot != wantRoot || name != filepath.Join("checkpoints", "run.json") {
		t.Fatalf("checkpointRoot() = (%q, %q), want (%q, %q)", gotRoot, name, wantRoot, filepath.Join("checkpoints", "run.json"))
	}
}
