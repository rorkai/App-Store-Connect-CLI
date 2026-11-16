package shared

import (
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteStreamToFileRejectsSymlinkedParent(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	output := filepath.Join(root, "output")
	if err := os.Mkdir(output, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	link := filepath.Join(output, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	destination := filepath.Join(link, "artifact.bin")
	if _, err := WriteStreamToFile(destination, bytes.NewBufferString("payload")); err == nil {
		t.Fatal("WriteStreamToFile() succeeded through a symlinked parent")
	}
	if _, err := os.Stat(filepath.Join(outside, "artifact.bin")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outside destination stat error = %v, want os.ErrNotExist", err)
	}
}

func TestDecompressGzipFileRejectsSymlinkedOutputParent(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.gz")
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte("payload")); err != nil {
		t.Fatalf("gzip.Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("gzip.Close() error = %v", err)
	}
	if err := os.WriteFile(source, compressed.Bytes(), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	outside := t.TempDir()
	output := filepath.Join(root, "output")
	if err := os.Mkdir(output, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	link := filepath.Join(output, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	destination := filepath.Join(link, "artifact.txt")
	if _, err := DefaultDeps().DecompressGzipFile(source, destination); err == nil {
		t.Fatal("DefaultDeps().DecompressGzipFile() succeeded through a symlinked output parent")
	}
	if _, err := os.Stat(filepath.Join(outside, "artifact.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outside destination stat error = %v, want os.ErrNotExist", err)
	}
}

func TestDecompressGzipFileRejectsSymlinkedSourceParent(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	source := filepath.Join(outside, "source.gz")
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte("payload")); err != nil {
		t.Fatalf("gzip.Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("gzip.Close() error = %v", err)
	}
	if err := os.WriteFile(source, compressed.Bytes(), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	link := filepath.Join(root, "source-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	destination := filepath.Join(root, "artifact.txt")
	if _, err := DefaultDeps().DecompressGzipFile(filepath.Join(link, "source.gz"), destination); err == nil {
		t.Fatal("DefaultDeps().DecompressGzipFile() read through a symlinked source parent")
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination stat error = %v, want os.ErrNotExist", err)
	}
}
