package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilePermissionsTooPermissiveForOS(t *testing.T) {
	tests := []struct {
		name string
		mode os.FileMode
		goos string
		want bool
	}{
		{name: "unix secure", mode: 0o600, goos: "darwin", want: false},
		{name: "unix insecure", mode: 0o644, goos: "darwin", want: true},
		{name: "windows secure", mode: 0o600, goos: "windows", want: false},
		{name: "windows insecure", mode: 0o644, goos: "windows", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filePermissionsTooPermissiveForOS(tt.mode, tt.goos); got != tt.want {
				t.Fatalf("filePermissionsTooPermissiveForOS(%#o, %q) = %v, want %v", tt.mode.Perm(), tt.goos, got, tt.want)
			}
		})
	}
}

func TestValidateKeyFileForOSWindowsSkipsUnixPermissionCheck(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")

	writeECDSAPEM(t, keyPath, 0o644, true)

	if err := validateKeyFileForOS(keyPath, "windows"); err != nil {
		t.Fatalf("expected Windows validation to ignore Unix permission bits, got %v", err)
	}
}
