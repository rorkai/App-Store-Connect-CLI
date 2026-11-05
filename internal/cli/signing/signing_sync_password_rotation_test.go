package signing

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
	modernpkcs12 "software.sslmate.com/src/go-pkcs12"
)

func TestSigningSyncRotatePasswordFlagsAreRegistered(t *testing.T) {
	command := syncRotatePasswordCommand()
	for _, name := range []string{"repo", "password-file", "new-password-file", "branch", "confirm"} {
		flagDefinition := command.FlagSet.Lookup(name)
		if flagDefinition == nil {
			t.Fatalf("missing --%s flag", name)
		}
	}
}

func TestSigningSyncRotatePasswordRequiresConfirmBeforeSecretReads(t *testing.T) {
	command := syncRotatePasswordCommand()
	if err := command.Parse([]string{
		"--repo", "file:///repository-that-must-not-be-cloned",
		"--password-file", filepath.Join(t.TempDir(), "missing-current"),
		"--new-password-file", filepath.Join(t.TempDir(), "missing-new"),
	}); err != nil {
		t.Fatal(err)
	}
	err := command.Run(context.Background())
	if err == nil || err.Error() != "--confirm is required to rotate the signing sync password" {
		t.Fatalf("error = %v, want confirmation usage error", err)
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want usage error", err)
	}
}

func TestSigningSyncRotatePasswordRejectsEqualPasswordsBeforeClone(t *testing.T) {
	currentPasswordFile := filepath.Join(t.TempDir(), "current")
	newPasswordFile := filepath.Join(t.TempDir(), "next")
	writePrivateTestFile(t, currentPasswordFile, []byte("same-password\n"))
	writePrivateTestFile(t, newPasswordFile, []byte("same-password\n"))

	command := syncRotatePasswordCommand()
	if err := command.Parse([]string{
		"--repo", "file:///repository-that-must-not-be-cloned",
		"--password-file", currentPasswordFile,
		"--new-password-file", newPasswordFile,
		"--confirm",
	}); err != nil {
		t.Fatal(err)
	}
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		runErr = command.Run(context.Background())
	})
	if runErr == nil || runErr.Error() != "current and new signing sync passwords must differ" {
		t.Fatalf("error = %v, want equal-password usage error", runErr)
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("error = %v, want usage error", runErr)
	}
	if stdout != "" || strings.Contains(stderr, "Cloning signing repo") {
		t.Fatalf("unexpected output before validation: stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestSigningSyncRotatePasswordReencryptsCompleteRepositoryAndIdentity(t *testing.T) {
	const oldPassword = "old-repository-password"
	const newPassword = "new-repository-password"

	remoteURL, remotePath := newSigningSyncBareRemote(t)
	seedStore := &signingpkg.GitStore{
		RepoURL:  remoteURL,
		LocalDir: filepath.Join(t.TempDir(), "seed-clone"),
		Branch:   "main",
	}
	if err := seedStore.Clone(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = seedStore.Cleanup() })

	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 91)
	identity := &signingIdentity{
		PrivateKey:        key,
		Certificate:       certificate,
		CertificateSHA256: certificateSHA256(certificate),
	}
	artifacts, err := prepareSigningIdentityArtifacts(identity, oldPassword, "com.example.app", "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	profilePath, profileContent := bindTestSigningIdentityArtifacts(t, artifacts, certificate, key, "com.example.app", "IOS_APP_ADHOC", "profile-rotation")
	certificatePath := filepath.Join("certs", "distribution", "certificate.cer")
	if err := seedStore.WriteEncryptedFile(certificatePath, certificate.Raw, oldPassword); err != nil {
		t.Fatal(err)
	}
	if err := seedStore.WriteEncryptedFile(profilePath, profileContent, oldPassword); err != nil {
		t.Fatal(err)
	}
	if err := writeOrReuseSigningIdentityArtifacts(seedStore, artifacts, oldPassword); err != nil {
		t.Fatal(err)
	}
	if err := seedStore.CommitAndPush(context.Background(), "seed encrypted signing assets"); err != nil {
		t.Fatal(err)
	}

	currentPasswordFile := filepath.Join(t.TempDir(), "current")
	newPasswordFile := filepath.Join(t.TempDir(), "next")
	writePrivateTestFile(t, currentPasswordFile, []byte(oldPassword+"\n"))
	writePrivateTestFile(t, newPasswordFile, []byte(newPassword+"\n"))

	command := syncRotatePasswordCommand()
	if err := command.Parse([]string{
		"--repo", remoteURL,
		"--password-file", currentPasswordFile,
		"--new-password-file", newPasswordFile,
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatal(err)
	}
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		runErr = command.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("rotate-password error = %v\nstderr=%s", runErr, stderr)
	}
	var result SyncResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output %q: %v", stdout, err)
	}
	if result.Operation != "rotate-password" || result.RepoURL != remoteURL || !result.IdentityPresent {
		t.Fatalf("rotation result = %#v", result)
	}
	if result.Storage == nil || *result.Storage != (asc.SigningSyncStorage{Kind: "git", Location: remoteURL, Branch: "main"}) {
		t.Fatalf("rotation storage = %#v, want git storage with the branch", result.Storage)
	}
	if len(result.Files) != 4 || len(result.SensitiveFiles) != 1 || result.SensitiveFiles[0] != artifacts.IdentityPath {
		t.Fatalf("rotation files = %#v sensitive = %#v", result.Files, result.SensitiveFiles)
	}

	verifyStore := &signingpkg.GitStore{
		RepoURL:  remoteURL,
		LocalDir: filepath.Join(t.TempDir(), "verify-clone"),
		Branch:   "main",
	}
	if err := verifyStore.Clone(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = verifyStore.Cleanup() })
	encryptedFiles, err := verifyStore.ListEncryptedFiles()
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := prepareDecryptedSigningFiles(verifyStore, encryptedFiles, newPassword, t.TempDir())
	if err != nil {
		t.Fatalf("new password rejected: %v", err)
	}
	if _, _, err := verifyStore.ReadEncryptedFileWithMetadata(certificatePath, oldPassword); err == nil {
		t.Fatal("old password still decrypts a rotated artifact")
	}
	legacyCertificate, certificateMetadata, err := verifyStore.ReadEncryptedFileWithMetadata(certificatePath, newPassword)
	if err != nil {
		t.Fatal(err)
	}
	if string(legacyCertificate) != string(certificate.Raw) || certificateMetadata.Version != 0 {
		t.Fatalf("legacy artifact changed format or content: metadata=%#v", certificateMetadata)
	}

	var identityPlaintext []byte
	for _, file := range decrypted {
		if file.RelativePath == artifacts.IdentityPath {
			identityPlaintext = file.Plaintext
			break
		}
	}
	if len(identityPlaintext) == 0 {
		t.Fatal("rotated identity is missing")
	}
	if _, _, err := modernpkcs12.Decode(identityPlaintext, newPassword); err != nil {
		t.Fatalf("rotated identity does not use new password: %v", err)
	}
	if _, _, err := modernpkcs12.Decode(identityPlaintext, oldPassword); err == nil {
		t.Fatal("rotated identity still uses old password")
	}

	if got := strings.TrimSpace(runGitCommand(t, remotePath, "rev-list", "--count", "main")); got != "3" {
		t.Fatalf("commit count = %s, want seed README, encrypted assets, and one rotation commit", got)
	}
}

func TestSigningSyncRotatePasswordDoesNotPublishWhenOneArtifactIsCorrupt(t *testing.T) {
	const oldPassword = "old-repository-password"
	const newPassword = "new-repository-password"

	remoteURL, _ := newSigningSyncBareRemote(t)
	seedStore := &signingpkg.GitStore{RepoURL: remoteURL, LocalDir: filepath.Join(t.TempDir(), "seed-clone"), Branch: "main"}
	if err := seedStore.Clone(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if err := seedStore.WriteEncryptedFile(filepath.Join("certs", "distribution", "valid.cer"), []byte("valid"), oldPassword); err != nil {
		t.Fatal(err)
	}
	corruptPath := filepath.Join(seedStore.LocalDir, "profiles", "appstore", "corrupt.mobileprovision.enc")
	if err := os.MkdirAll(filepath.Dir(corruptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corruptPath, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := seedStore.CommitAndPush(context.Background(), "seed corrupt signing assets"); err != nil {
		t.Fatal(err)
	}
	if err := seedStore.Cleanup(); err != nil {
		t.Fatal(err)
	}

	currentPasswordFile := filepath.Join(t.TempDir(), "current")
	newPasswordFile := filepath.Join(t.TempDir(), "next")
	writePrivateTestFile(t, currentPasswordFile, []byte(oldPassword))
	writePrivateTestFile(t, newPasswordFile, []byte(newPassword))
	command := syncRotatePasswordCommand()
	if err := command.Parse([]string{
		"--repo", remoteURL,
		"--password-file", currentPasswordFile,
		"--new-password-file", newPasswordFile,
		"--confirm",
	}); err != nil {
		t.Fatal(err)
	}
	if err := command.Run(context.Background()); err == nil || !strings.Contains(err.Error(), "decrypt profiles/appstore/corrupt.mobileprovision") {
		t.Fatalf("error = %v, want corrupt-artifact failure", err)
	}

	verifyDir := t.TempDir()
	runGitCommand(t, verifyDir, "clone", remoteURL, "clone")
	if got := strings.TrimSpace(runGitCommand(t, filepath.Join(verifyDir, "clone"), "rev-list", "--count", "HEAD")); got != "2" {
		t.Fatalf("commit count = %s, rotation published despite corrupt input", got)
	}
}
