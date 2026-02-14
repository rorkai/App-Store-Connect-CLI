package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenExistingNoFollowBestEffort_RejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	_, err := openExistingNoFollowBestEffort(link, os.Open)
	if err == nil {
		t.Fatal("expected error when opening symlink path, got nil")
	}
	if !strings.Contains(err.Error(), "refusing to follow symlink") {
		t.Fatalf("expected symlink refusal error, got %v", err)
	}
}

func TestOpenNewFileNoFollowBestEffort_RejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	_, err := openNewFileNoFollowBestEffort(link, 0o600, openNewFileWithExcl)
	if err == nil {
		t.Fatal("expected error when creating through symlink path, got nil")
	}
	if !strings.Contains(err.Error(), "refusing to follow symlink") {
		t.Fatalf("expected symlink refusal error, got %v", err)
	}
}

func TestOpenExistingNoFollowBestEffort_DetectsPathSwap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.txt")
	if err := os.WriteFile(path, []byte("expected"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	other := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(other, []byte("other"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	file, err := openExistingNoFollowBestEffort(path, func(string) (*os.File, error) {
		return os.Open(other)
	})
	if file != nil {
		_ = file.Close()
	}
	if err == nil {
		t.Fatal("expected mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "file changed during open") {
		t.Fatalf("expected path swap error, got %v", err)
	}
}

func TestOpenExistingNoFollowBestEffort_AllowsRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	file, err := openExistingNoFollowBestEffort(path, os.Open)
	if err != nil {
		t.Fatalf("openExistingNoFollowBestEffort() error = %v", err)
	}
	defer file.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("file data = %q, want %q", string(data), "hello")
	}
}

func TestOpenNewFileNoFollowBestEffort_CreatesRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.txt")

	file, err := openNewFileNoFollowBestEffort(path, 0o600, openNewFileWithExcl)
	if err != nil {
		t.Fatalf("openNewFileNoFollowBestEffort() error = %v", err)
	}
	if _, err := file.Write([]byte("ok")); err != nil {
		file.Close()
		t.Fatalf("Write() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "ok" {
		t.Fatalf("file data = %q, want %q", string(data), "ok")
	}
}

func openNewFileWithExcl(path string, perm os.FileMode) (*os.File, error) {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	return os.OpenFile(path, flags, perm)
}
