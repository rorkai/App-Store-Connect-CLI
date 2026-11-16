package snitch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteLocalLogRefusesSymlinkedLogAndLeavesModeIntact(t *testing.T) {
	workDir := t.TempDir()
	t.Chdir(workDir)

	sentinelPath := filepath.Join(t.TempDir(), "sentinel.txt")
	if err := os.WriteFile(sentinelPath, []byte("original"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workDir, ".asc"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.Symlink(sentinelPath, filepath.Join(workDir, ".asc", "snitch.log")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	err := writeLocalLog(newSnitchTestDeps(), LogEntry{Description: "friction", Severity: "bug", Timestamp: time.Now().UTC()})
	if err == nil {
		t.Fatal("writeLocalLog() error = nil, want symlink rejection")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("writeLocalLog() error = %v, want symlink rejection", err)
	}

	data, readErr := os.ReadFile(sentinelPath)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	if string(data) != "original" {
		t.Fatalf("sentinel content = %q, want %q", data, "original")
	}
	info, statErr := os.Lstat(sentinelPath)
	if statErr != nil {
		t.Fatalf("Lstat() error = %v", statErr)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("sentinel mode = %v, want 0644 (writeLocalLog must not chmod its target)", info.Mode().Perm())
	}
}

func TestWriteLocalLogRefusesSymlinkedASCDirectory(t *testing.T) {
	workDir := t.TempDir()
	t.Chdir(workDir)

	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(workDir, ".asc")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	err := writeLocalLog(newSnitchTestDeps(), LogEntry{Description: "friction", Severity: "bug", Timestamp: time.Now().UTC()})
	if err == nil {
		t.Fatal("writeLocalLog() error = nil, want symlink rejection")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("writeLocalLog() error = %v, want symlink rejection", err)
	}
	if _, statErr := os.Stat(filepath.Join(external, "snitch.log")); statErr == nil {
		t.Fatal("log escaped through a symlinked .asc directory")
	}
}

func TestWriteLocalLogAppendsEntriesWithSecureMode(t *testing.T) {
	workDir := t.TempDir()
	t.Chdir(workDir)

	first := LogEntry{Description: "first", Severity: "bug", Timestamp: time.Now().UTC()}
	second := LogEntry{Description: "second", Severity: "friction", Timestamp: time.Now().UTC()}
	if err := writeLocalLog(newSnitchTestDeps(), first); err != nil {
		t.Fatalf("writeLocalLog() error = %v", err)
	}
	if err := writeLocalLog(newSnitchTestDeps(), second); err != nil {
		t.Fatalf("writeLocalLog() error = %v", err)
	}

	logPath := filepath.Join(workDir, ".asc", "snitch.log")
	info, err := os.Lstat(logPath)
	if err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("log mode = %v, want 0600", info.Mode().Perm())
	}

	entries, err := readLocalLog(newSnitchTestDeps(), logPath)
	if err != nil {
		t.Fatalf("readLocalLog() error = %v", err)
	}
	if len(entries) != 2 || entries[0].Description != "first" || entries[1].Description != "second" {
		t.Fatalf("readLocalLog() = %#v, want two appended entries", entries)
	}
}

func TestReadLocalLogRefusesSymlinkedLog(t *testing.T) {
	dir := t.TempDir()
	externalPath := filepath.Join(t.TempDir(), "external.log")
	if err := os.WriteFile(externalPath, []byte(`{"description":"external"}`+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	logPath := filepath.Join(dir, "snitch.log")
	if err := os.Symlink(externalPath, logPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	entries, err := readLocalLog(newSnitchTestDeps(), logPath)
	if err == nil {
		t.Fatalf("readLocalLog() error = nil, want symlink rejection (got %#v)", entries)
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("readLocalLog() error = %v, want symlink rejection", err)
	}
}

func TestReadLocalLogPreservesWhitespaceInPath(t *testing.T) {
	dir := t.TempDir()
	exactPath := filepath.Join(dir, " snitch.log ")
	trimmedPath := filepath.Join(dir, "snitch.log")

	writeLogEntry := func(path, description string) {
		t.Helper()
		data := []byte(`{"description":"` + description + `","severity":"bug"}` + "\n")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", path, err)
		}
	}
	writeLogEntry(exactPath, "exact whitespace path")
	writeLogEntry(trimmedPath, "trimmed sibling")

	entries, err := readLocalLog(newSnitchTestDeps(), exactPath)
	if err != nil {
		t.Fatalf("readLocalLog(%q) error = %v", exactPath, err)
	}
	if len(entries) != 1 || entries[0].Description != "exact whitespace path" {
		t.Fatalf("readLocalLog(%q) = %#v, want exact whitespace-bearing file", exactPath, entries)
	}
}

func TestSnitchFlushPreservesWhitespaceInFileFlag(t *testing.T) {
	dir := t.TempDir()
	exactPath := filepath.Join(dir, " snitch.log ")
	trimmedPath := filepath.Join(dir, "snitch.log")

	if err := os.WriteFile(exactPath, []byte(`{"description":"exact whitespace path","severity":"bug"}`+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", exactPath, err)
	}
	if err := os.WriteFile(trimmedPath, []byte(`{"description":"trimmed sibling","severity":"bug"}`+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", trimmedPath, err)
	}

	stdout, _, err := runSnitchCommand(t, "1.2.3", "flush", "--file", exactPath)
	if err != nil {
		t.Fatalf("snitch flush --file %q error = %v", exactPath, err)
	}
	if !strings.Contains(stdout, "exact whitespace path") || strings.Contains(stdout, "trimmed sibling") {
		t.Fatalf("snitch flush output = %q, want exact whitespace-bearing file only", stdout)
	}
}
