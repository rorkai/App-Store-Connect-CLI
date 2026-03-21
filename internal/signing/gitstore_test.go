package signing

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureInsideDir(t *testing.T) {
	baseDir := t.TempDir()

	tests := []struct {
		name    string
		target  string
		wantErr bool
	}{
		{
			name:   "allows base directory itself",
			target: baseDir,
		},
		{
			name:   "allows child path",
			target: filepath.Join(baseDir, "nested", "file.txt"),
		},
		{
			name:    "rejects parent directory escape",
			target:  filepath.Join(baseDir, "..", "escaped.txt"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EnsureInsideDir(baseDir, tt.target)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("EnsureInsideDir(%q, %q) expected error, got nil", baseDir, tt.target)
				}
				return
			}
			if err != nil {
				t.Fatalf("EnsureInsideDir(%q, %q) unexpected error: %v", baseDir, tt.target, err)
			}
		})
	}
}

func TestGitStoreWriteAndReadEncryptedFileRoundTrip(t *testing.T) {
	store := &GitStore{LocalDir: t.TempDir()}
	relPath := filepath.Join("profiles", "development", "com.example.app.mobileprovision")
	plaintext := []byte("profile-data")
	password := "test-password"

	if err := store.WriteEncryptedFile(relPath, plaintext, password); err != nil {
		t.Fatalf("WriteEncryptedFile: %v", err)
	}

	encryptedPath := filepath.Join(store.LocalDir, relPath+".enc")
	encrypted, err := os.ReadFile(encryptedPath)
	if err != nil {
		t.Fatalf("read encrypted output: %v", err)
	}
	if bytes.Equal(encrypted, plaintext) {
		t.Fatal("encrypted file should not match plaintext bytes")
	}

	got, err := store.ReadEncryptedFile(relPath, password)
	if err != nil {
		t.Fatalf("ReadEncryptedFile: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("decrypted output mismatch: got %q, want %q", got, plaintext)
	}
}

func TestGitStoreWriteEncryptedFileRejectsPathEscape(t *testing.T) {
	store := &GitStore{LocalDir: t.TempDir()}

	err := store.WriteEncryptedFile(filepath.Join("..", "escaped"), []byte("secret"), "test-password")
	if err == nil {
		t.Fatal("expected path escape error, got nil")
	}
	if !strings.Contains(err.Error(), "escapes base directory") {
		t.Fatalf("expected path escape error, got %v", err)
	}
}

func TestGitStoreWriteEncryptedFileRejectsSymlinkedParentDirectory(t *testing.T) {
	store := &GitStore{LocalDir: t.TempDir()}
	outsideDir := t.TempDir()

	if err := os.Symlink(outsideDir, filepath.Join(store.LocalDir, "linked")); err != nil {
		t.Fatalf("create parent symlink: %v", err)
	}

	err := store.WriteEncryptedFile(filepath.Join("linked", "secret"), []byte("secret"), "test-password")
	if err == nil {
		t.Fatal("expected symlink rejection error, got nil")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection error, got %v", err)
	}

	_, statErr := os.Stat(filepath.Join(outsideDir, "secret.enc"))
	if statErr == nil {
		t.Fatal("did not expect write through symlinked parent directory")
	}
	if !os.IsNotExist(statErr) {
		t.Fatalf("stat outside write target: %v", statErr)
	}
}

func TestGitStoreWriteEncryptedFileRejectsSymlinkTarget(t *testing.T) {
	store := &GitStore{LocalDir: t.TempDir()}
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "secret.enc")

	if err := os.WriteFile(outsidePath, []byte("original"), 0o600); err != nil {
		t.Fatalf("write outside target: %v", err)
	}
	if err := os.Symlink(outsidePath, filepath.Join(store.LocalDir, "secret.enc")); err != nil {
		t.Fatalf("create file symlink: %v", err)
	}

	err := store.WriteEncryptedFile("secret", []byte("secret"), "test-password")
	if err == nil {
		t.Fatal("expected symlink rejection error, got nil")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection error, got %v", err)
	}

	got, readErr := os.ReadFile(outsidePath)
	if readErr != nil {
		t.Fatalf("read outside target: %v", readErr)
	}
	if string(got) != "original" {
		t.Fatalf("did not expect write through symlink target, got %q", got)
	}
}

func TestGitStoreReadEncryptedFileRejectsSymlink(t *testing.T) {
	store := &GitStore{LocalDir: t.TempDir()}
	targetDir := t.TempDir()
	password := "test-password"

	encrypted, err := Encrypt([]byte("secret"), password)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	targetPath := filepath.Join(targetDir, "secret.enc")
	if err := os.WriteFile(targetPath, encrypted, 0o600); err != nil {
		t.Fatalf("write target encrypted file: %v", err)
	}

	linkPath := filepath.Join(store.LocalDir, "secret.enc")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	_, err = store.ReadEncryptedFile("secret", password)
	if err == nil {
		t.Fatal("expected symlink rejection error, got nil")
	}
	if !strings.Contains(err.Error(), "refusing to read symlink") {
		t.Fatalf("expected symlink rejection error, got %v", err)
	}
}

func TestGitStoreReadEncryptedFileRejectsSymlinkedParentDirectory(t *testing.T) {
	store := &GitStore{LocalDir: t.TempDir()}
	targetDir := t.TempDir()
	password := "test-password"

	encrypted, err := Encrypt([]byte("secret"), password)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	targetPath := filepath.Join(targetDir, "secret.enc")
	if err := os.WriteFile(targetPath, encrypted, 0o600); err != nil {
		t.Fatalf("write target encrypted file: %v", err)
	}

	if err := os.Symlink(targetDir, filepath.Join(store.LocalDir, "linked")); err != nil {
		t.Fatalf("create parent symlink: %v", err)
	}

	_, err = store.ReadEncryptedFile(filepath.Join("linked", "secret"), password)
	if err == nil {
		t.Fatal("expected symlink rejection error, got nil")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection error, got %v", err)
	}
}

func TestGitStoreListEncryptedFilesSkipsGitDirAndSymlinks(t *testing.T) {
	store := &GitStore{LocalDir: t.TempDir()}

	if err := os.MkdirAll(filepath.Join(store.LocalDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(store.LocalDir, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	write := func(rel string) {
		t.Helper()
		path := filepath.Join(store.LocalDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	write("root.enc")
	write(filepath.Join("nested", "child.enc"))
	write(filepath.Join(".git", "ignored.enc"))

	if err := os.Symlink(filepath.Join(store.LocalDir, "root.enc"), filepath.Join(store.LocalDir, "symlink.enc")); err != nil {
		t.Fatalf("create file symlink: %v", err)
	}
	if err := os.Symlink(filepath.Join(store.LocalDir, "nested"), filepath.Join(store.LocalDir, "linked-dir")); err != nil {
		t.Fatalf("create dir symlink: %v", err)
	}

	got, err := store.ListEncryptedFiles()
	if err != nil {
		t.Fatalf("ListEncryptedFiles: %v", err)
	}

	gotSet := map[string]bool{}
	for _, rel := range got {
		gotSet[filepath.ToSlash(rel)] = true
	}

	if !gotSet["root"] {
		t.Fatalf("expected root file in list, got %v", got)
	}
	if !gotSet["nested/child"] {
		t.Fatalf("expected nested file in list, got %v", got)
	}
	if gotSet[".git/ignored"] {
		t.Fatalf("did not expect .git file in list, got %v", got)
	}
	if gotSet["symlink"] {
		t.Fatalf("did not expect symlink file in list, got %v", got)
	}
	if gotSet["linked-dir/child"] {
		t.Fatalf("did not expect symlinked directory contents in list, got %v", got)
	}
}
