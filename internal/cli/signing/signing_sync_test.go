package signing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSigningSyncCommandLongHelpUsesOutputDirExample(t *testing.T) {
	cmd := SigningSyncCommand()
	if !strings.Contains(cmd.LongHelp, "--output-dir ./signing") {
		t.Fatalf("expected long help to document --output-dir, got %q", cmd.LongHelp)
	}
	if strings.Contains(cmd.LongHelp, "--output ./signing") {
		t.Fatalf("expected long help to avoid --output path example, got %q", cmd.LongHelp)
	}
}

func TestSanitizeRepoURLForOutputRedactsCredentials(t *testing.T) {
	raw := "https://token:secret@example.com/org/repo.git?access_token=abc123"
	got := sanitizeRepoURLForOutput(raw)

	if strings.Contains(got, "token:secret@") || strings.Contains(got, "secret") || strings.Contains(got, "abc123") {
		t.Fatalf("expected credentials to be redacted, got %q", got)
	}
	if !strings.Contains(got, "%5BREDACTED%5D") {
		t.Fatalf("expected sanitized marker, got %q", got)
	}
}

func TestSigningCommandLongHelpUsesOutputDirForSyncPull(t *testing.T) {
	cmd := SigningCommand()
	if !strings.Contains(cmd.LongHelp, "asc signing sync pull --repo git@github.com:team/certs.git --output-dir ./signing") {
		t.Fatalf("expected top-level help to use --output-dir for sync pull, got %q", cmd.LongHelp)
	}
	if strings.Contains(cmd.LongHelp, "asc signing sync pull --repo git@github.com:team/certs.git --output ./signing") {
		t.Fatalf("expected top-level help to avoid --output for sync pull, got %q", cmd.LongHelp)
	}
}

func TestSigningSyncCommandLongHelpPullExampleOmitsUnsupportedFlags(t *testing.T) {
	cmd := SigningSyncCommand()
	if strings.Contains(cmd.LongHelp, "asc signing sync pull --bundle-id") {
		t.Fatalf("expected pull example to omit --bundle-id, got %q", cmd.LongHelp)
	}
	if strings.Contains(cmd.LongHelp, "asc signing sync pull --profile-type") {
		t.Fatalf("expected pull example to omit --profile-type, got %q", cmd.LongHelp)
	}
}

func TestWriteDecryptedOutputFileWritesPlaintext(t *testing.T) {
	outDir := t.TempDir()
	relPath := filepath.Join("profiles", "appstore", "app.mobileprovision")
	plaintext := []byte("profile-data")

	if err := writeDecryptedOutputFile(outDir, relPath, plaintext); err != nil {
		t.Fatalf("writeDecryptedOutputFile: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(outDir, relPath))
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if string(got) != string(plaintext) {
		t.Fatalf("output mismatch: got %q, want %q", got, plaintext)
	}
}

func TestWriteDecryptedOutputFileRejectsSymlinkTarget(t *testing.T) {
	outDir := t.TempDir()
	targetDir := t.TempDir()
	relPath := filepath.Join("profiles", "appstore", "app.mobileprovision")
	destPath := filepath.Join(outDir, relPath)
	targetPath := filepath.Join(targetDir, "app.mobileprovision")

	if err := os.WriteFile(targetPath, []byte("original"), 0o600); err != nil {
		t.Fatalf("write target file: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		t.Fatalf("mkdir output parent: %v", err)
	}
	if err := os.Symlink(targetPath, destPath); err != nil {
		t.Fatalf("create destination symlink: %v", err)
	}

	err := writeDecryptedOutputFile(outDir, relPath, []byte("updated"))
	if err == nil {
		t.Fatal("expected symlink rejection error, got nil")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection error, got %v", err)
	}

	got, readErr := os.ReadFile(targetPath)
	if readErr != nil {
		t.Fatalf("read target file: %v", readErr)
	}
	if string(got) != "original" {
		t.Fatalf("did not expect write through symlink target, got %q", got)
	}
}
