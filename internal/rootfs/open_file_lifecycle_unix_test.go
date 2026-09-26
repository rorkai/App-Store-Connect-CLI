//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package rootfs

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenFileDoesNotRetainTrustedRootDescriptors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte("input"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	countOpenDescriptors := func() int {
		directory, err := os.Open("/dev/fd")
		if err != nil {
			t.Fatalf("Open(/dev/fd) error: %v", err)
		}
		defer directory.Close()
		entries, err := directory.Readdirnames(-1)
		if err != nil {
			t.Fatalf("Readdirnames(/dev/fd) error: %v", err)
		}
		return len(entries)
	}

	before := countOpenDescriptors()
	for i := 0; i < 256; i++ {
		file, err := OpenFile(path)
		if err != nil {
			t.Fatalf("OpenFile() iteration %d error: %v", i, err)
		}
		data, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("ReadAll() iteration %d error: %v", i, err)
		}
		if string(data) != "input" {
			t.Fatalf("ReadAll() iteration %d = %q, want input", i, data)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("Close() iteration %d error: %v", i, err)
		}
	}
	after := countOpenDescriptors()
	if delta := after - before; delta > 8 {
		t.Fatalf("open descriptor count grew by %d after closing returned files (before=%d, after=%d)", delta, before, after)
	}
}

func TestOpenFilePreservesNotExistError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.txt")
	_, err := OpenFile(path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("OpenFile() error = %v, want errors.Is(..., os.ErrNotExist)", err)
	}
	if !os.IsNotExist(err) {
		t.Fatalf("OpenFile() error = %v, want os.IsNotExist(...)", err)
	}
}
