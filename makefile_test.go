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

func TestMakeBuildRebuildsBinaryWhenSourceChanges(t *testing.T) {
	workspaceDir := t.TempDir()
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	writeWorkspaceFile := func(path, contents string) {
		t.Helper()
		fullPath := filepath.Join(workspaceDir, path)
		if err := os.WriteFile(fullPath, []byte(contents), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	writeWorkspaceFile("go.mod", "module example.com/makebuildtest\n\ngo 1.24.0\n")
	writeWorkspaceFile("main.go", "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Print(\"first\") }\n")

	runMakeBuild := func() {
		t.Helper()
		cmd := exec.Command("make", "-f", filepath.Join(repoRoot, "Makefile"), "-C", workspaceDir, "build")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("make build failed: %v\n%s", err, output)
		}
	}

	runBinary := func() string {
		t.Helper()
		cmd := exec.Command(filepath.Join(workspaceDir, "asc"))
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("run built binary failed: %v\n%s", err, output)
		}
		return string(output)
	}

	runMakeBuild()
	if got := runBinary(); got != "first" {
		t.Fatalf("expected initial binary output %q, got %q", "first", got)
	}

	writeWorkspaceFile("main.go", "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Print(\"second\") }\n")

	runMakeBuild()
	if got := runBinary(); got != "second" {
		t.Fatalf("expected rebuilt binary output %q, got %q", "second", got)
	}
}
