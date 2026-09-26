//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package web

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestBackupSessionFileRejectsFIFO(t *testing.T) {
	fifoPath := filepath.Join(t.TempDir(), "cache.fifo")
	if err := unix.Mkfifo(fifoPath, 0o600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}

	writerDone := make(chan error, 1)
	go func() {
		file, err := os.OpenFile(fifoPath, os.O_RDWR, 0)
		if err != nil {
			writerDone <- err
			return
		}
		_, writeErr := file.Write([]byte("must not be read as a backup"))
		closeErr := file.Close()
		writerDone <- errors.Join(writeErr, closeErr)
	}()

	_, err := backupSessionFile(fifoPath)
	if !errors.Is(err, errUnsafeSessionCacheFile) {
		t.Fatalf("backupSessionFile() error = %v, want errUnsafeSessionCacheFile", err)
	}
	if err := <-writerDone; err != nil {
		t.Fatalf("FIFO writer error = %v", err)
	}
}

func TestPersistSessionIgnoresPreexistingFIFOTemporaryFile(t *testing.T) {
	withFileSessionCache(t)
	key := webSessionCacheKey("user@example.com")
	sessionPath, err := webSessionFilePath(key)
	if err != nil {
		t.Fatalf("webSessionFilePath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(cache) error = %v", err)
	}
	tmpPath := sessionPath + ".tmp"
	if err := unix.Mkfifo(tmpPath, 0o600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}

	if err := PersistSession(newTestAuthSession(t)); err != nil {
		t.Fatalf("PersistSession() error = %v, want pre-existing FIFO ignored", err)
	}
	info, err := os.Stat(tmpPath)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v, want FIFO preserved", tmpPath, err)
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("temp path mode = %v, want FIFO preserved", info.Mode())
	}
}
