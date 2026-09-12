//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package secureopen

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestOpenExistingRegularFileNoFollowRejectsUnixSpecialFiles(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, path string)
	}{
		{
			name: "FIFO",
			prepare: func(t *testing.T, path string) {
				t.Helper()
				if err := unix.Mkfifo(path, 0o600); err != nil {
					t.Fatalf("Mkfifo() error: %v", err)
				}
			},
		},
		{
			name: "Unix socket",
			prepare: func(t *testing.T, path string) {
				t.Helper()
				listener, err := net.Listen("unix", path)
				if err != nil {
					t.Fatalf("Listen() error: %v", err)
				}
				t.Cleanup(func() { _ = listener.Close() })
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixtureDir, err := os.MkdirTemp("", "ascsf")
			if err != nil {
				t.Fatalf("MkdirTemp() error: %v", err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(fixtureDir) })
			path := filepath.Join(fixtureDir, "artifact")
			tt.prepare(t, path)

			file, _, err := OpenExistingRegularFileNoFollow(path, "artifact", "--artifact")
			if file != nil {
				file.Close()
			}
			if err == nil {
				t.Fatal("expected special-file error, got nil")
			}
			if !strings.Contains(err.Error(), "--artifact must be a regular file") {
				t.Fatalf("error = %v, want regular-file rejection", err)
			}
		})
	}
}

func TestOpenExistingRegularFileNoFollowRejectsDevice(t *testing.T) {
	info, err := os.Lstat("/dev/null")
	if err != nil {
		t.Skipf("/dev/null unavailable: %v", err)
	}
	if info.Mode().IsRegular() {
		t.Skip("/dev/null is unexpectedly a regular file")
	}

	file, _, err := OpenExistingRegularFileNoFollow("/dev/null", "device", "--artifact")
	if file != nil {
		file.Close()
	}
	if err == nil {
		t.Fatal("expected device rejection, got nil")
	}
	if !strings.Contains(err.Error(), "--artifact must be a regular file") {
		t.Fatalf("error = %v, want regular-file rejection", err)
	}
}
