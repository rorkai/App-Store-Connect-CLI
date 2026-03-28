package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMakeCleanRemovesReleaseDirectory(t *testing.T) {
	workspaceDir := t.TempDir()
	releaseDir := filepath.Join(workspaceDir, "release")
	staleArtifact := filepath.Join(releaseDir, "stale-artifact")
	if err := os.MkdirAll(releaseDir, 0o755); err != nil {
		t.Fatalf("mkdir release dir: %v", err)
	}
	if err := os.WriteFile(staleArtifact, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale artifact: %v", err)
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	cmd := exec.Command("make", "-f", filepath.Join(repoRoot, "Makefile"), "-C", workspaceDir, "clean")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make clean failed: %v\n%s", err, output)
	}

	if _, err := os.Stat(staleArtifact); !os.IsNotExist(err) {
		t.Fatalf("expected make clean to remove %s, stat err=%v\n%s", staleArtifact, err, output)
	}
}
