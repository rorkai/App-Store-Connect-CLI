package auth

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPrivateKeyReadsRejectSpecialFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("/dev/null is not a POSIX special file on Windows")
	}
	info, err := os.Lstat(os.DevNull)
	if err != nil {
		t.Fatalf("Lstat(%q) error: %v", os.DevNull, err)
	}
	if info.Mode().IsRegular() {
		t.Skipf("%s is a regular file on this platform", os.DevNull)
	}

	tests := []struct {
		name string
		call func() error
	}{
		{name: "validate", call: func() error { return ValidateKeyFile(os.DevNull) }},
		{name: "load", call: func() error { _, err := LoadPrivateKey(os.DevNull); return err }},
		{name: "storage comparison read", call: func() error { _, err := loadPrivateKeyPEMForStorage(os.DevNull); return err }},
		{name: "migration existing path", call: func() error {
			_, _, err := migrationPrivateKeyPath(Credential{Name: "special", PrivateKeyPath: os.DevNull}, t.TempDir(), "special")
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.call()
			if err == nil {
				t.Fatal("expected special-file rejection, got nil")
			}
			if !strings.Contains(err.Error(), "not a regular file") {
				t.Fatalf("error = %v, want regular-file rejection", err)
			}
		})
	}
}

func TestValidateKeyFilePreservesDirectoryErrorClassification(t *testing.T) {
	err := ValidateKeyFile(t.TempDir())
	if err == nil {
		t.Fatal("expected directory rejection, got nil")
	}
	if kind, ok := PrivateKeyErrorKindOf(err); !ok || kind != PrivateKeyInvalidFormat {
		t.Fatalf("PrivateKeyErrorKindOf(%v) = %q, %t; want %q, true", err, kind, ok, PrivateKeyInvalidFormat)
	}
	if !strings.Contains(err.Error(), "private key path is a directory") {
		t.Fatalf("error = %v, want directory diagnostic", err)
	}
}

func TestPrivateKeyReadsRejectSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.p8")
	link := filepath.Join(dir, "AuthKey.p8")
	writeECDSAPEM(t, target, 0o600, true)
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	tests := []struct {
		name string
		call func() error
	}{
		{name: "validate", call: func() error { return ValidateKeyFile(link) }},
		{name: "load", call: func() error { _, err := LoadPrivateKey(link); return err }},
		{name: "storage comparison read", call: func() error { _, err := loadPrivateKeyPEMForStorage(link); return err }},
		{name: "migration existing path", call: func() error {
			_, _, err := migrationPrivateKeyPath(Credential{Name: "linked", PrivateKeyPath: link}, filepath.Join(dir, "exports"), "linked")
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.call()
			if err == nil {
				t.Fatal("expected symlink rejection, got nil")
			}
			if !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("error = %v, want symlink rejection", err)
			}
		})
	}
}

func TestPrivateKeyReadsRejectSymlinkedParent(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "target")
	if err := os.Mkdir(targetDir, 0o700); err != nil {
		t.Fatalf("Mkdir(target) error: %v", err)
	}
	target := filepath.Join(targetDir, "AuthKey.p8")
	writeECDSAPEM(t, target, 0o600, true)
	linkDir := filepath.Join(dir, "linked")
	if err := os.Symlink(targetDir, linkDir); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	keyPath := filepath.Join(linkDir, "AuthKey.p8")

	tests := []struct {
		name string
		call func() error
	}{
		{name: "validate", call: func() error { return ValidateKeyFile(keyPath) }},
		{name: "load", call: func() error { _, err := LoadPrivateKey(keyPath); return err }},
		{name: "storage comparison read", call: func() error { _, err := loadPrivateKeyPEMForStorage(keyPath); return err }},
		{name: "migration existing path", call: func() error {
			_, _, err := migrationPrivateKeyPath(Credential{Name: "linked", PrivateKeyPath: keyPath}, filepath.Join(dir, "exports"), "linked")
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.call()
			if err == nil {
				t.Fatal("expected symlinked-parent rejection, got nil")
			}
			if !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("error = %v, want symlink rejection", err)
			}
		})
	}
}

func TestMigrationScanIgnoresSymlinkedFastlaneFiles(t *testing.T) {
	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	if err := os.Mkdir(fastlaneDir, 0o755); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}
	target := filepath.Join(root, "outside-Appfile")
	if err := os.WriteFile(target, []byte(`app_identifier "com.example.app"`), 0o600); err != nil {
		t.Fatalf("WriteFile(target) error: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(fastlaneDir, "Appfile")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	signals := scanMigrationSignals(root)
	if len(signals.detectedFiles) != 0 {
		t.Fatalf("detectedFiles = %#v, want no symlinked files", signals.detectedFiles)
	}
	if signals.appIdentifier != "" {
		t.Fatalf("appIdentifier = %q, want empty for symlinked Appfile", signals.appIdentifier)
	}
}
