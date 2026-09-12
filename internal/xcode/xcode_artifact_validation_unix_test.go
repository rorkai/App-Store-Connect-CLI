//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package xcode

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestValidateRejectsNonRegularArtifactsBeforeAltool(t *testing.T) {
	tests := []struct {
		name    string
		create  func(t *testing.T, path string)
		wantErr string
	}{
		{
			name: "FIFO",
			create: func(t *testing.T, path string) {
				t.Helper()
				if err := unix.Mkfifo(path, 0o600); err != nil {
					t.Fatalf("Mkfifo() error: %v", err)
				}
			},
			wantErr: "--pkg must be a regular file",
		},
		{
			name: "Unix socket",
			create: func(t *testing.T, path string) {
				t.Helper()
				listener, err := net.Listen("unix", path)
				if err != nil {
					t.Fatalf("Listen() error: %v", err)
				}
				t.Cleanup(func() { _ = listener.Close() })
			},
			wantErr: "--pkg must be a regular file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "ascx")
			if err != nil {
				t.Fatalf("MkdirTemp() error: %v", err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(tempDir) })
			pkgPath := filepath.Join(tempDir, "Demo.pkg")
			tt.create(t, pkgPath)
			logPath := filepath.Join(tempDir, "commands.log")

			restore := overrideTestEnvironment(t)
			runtimeGOOS = "darwin"
			lookPathFn = func(file string) (string, error) {
				switch file {
				case "xcodebuild", "xcrun":
					return "/usr/bin/" + file, nil
				default:
					return "", exec.ErrNotFound
				}
			}
			commandContextFn = helperCommandContext(t, logPath)
			t.Cleanup(restore)

			_, err = Validate(context.Background(), ValidateOptions{PKGPath: pkgPath})
			if err == nil {
				t.Fatal("expected non-regular artifact error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tt.wantErr)
			}

			logData, readErr := os.ReadFile(logPath)
			if readErr == nil && strings.Contains(string(logData), "|--validate-app|") {
				t.Fatalf("altool validation ran for non-regular artifact: %q", string(logData))
			}
		})
	}
}
