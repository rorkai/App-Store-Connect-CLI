package certificates

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunCertificateExportRejectsWeakPrivateKeyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions are not enforced on Windows")
	}
	dir := t.TempDir()
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	certificatePath := filepath.Join(dir, "push.cer")
	privateKeyPath := filepath.Join(dir, "push.key")
	passwordPath := filepath.Join(dir, "password")
	p12Path := filepath.Join(dir, "push.p12")
	writeCertificateExportTestFiles(t, certificatePath, privateKeyPath, "", passwordPath, artifacts, []byte("password"))
	if err := os.Chmod(privateKeyPath, 0o644); err != nil {
		t.Fatalf("chmod private key: %v", err)
	}

	_, err := runCertificateExport(context.TODO(), certificatesTestDeps(nil), certificateExportOptions{
		CertificatePath: certificatePath,
		PrivateKeyPath:  privateKeyPath,
		PasswordPath:    passwordPath,
		P12Out:          p12Path,
	})
	if err == nil || !strings.Contains(err.Error(), "private key permissions") {
		t.Fatalf("runCertificateExport() error = %v, want private-key permission error", err)
	}
	if _, statErr := os.Stat(p12Path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("p12 output exists after permission failure, stat error = %v", statErr)
	}
}

func TestRunCertificateExportRejectsPasswordFileWithWeakPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions are not enforced on Windows")
	}
	dir := t.TempDir()
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	certificatePath := filepath.Join(dir, "push.cer")
	privateKeyPath := filepath.Join(dir, "push.key")
	passwordPath := filepath.Join(dir, "password")
	p12Path := filepath.Join(dir, "push.p12")
	writeCertificateExportTestFiles(t, certificatePath, privateKeyPath, "", passwordPath, artifacts, []byte("password"))
	if err := os.Chmod(passwordPath, 0o644); err != nil {
		t.Fatalf("chmod password: %v", err)
	}

	_, err := runCertificateExport(context.TODO(), certificatesTestDeps(nil), certificateExportOptions{
		CertificatePath: certificatePath,
		PrivateKeyPath:  privateKeyPath,
		PasswordPath:    passwordPath,
		P12Out:          p12Path,
	})
	if err == nil || !strings.Contains(err.Error(), "password permissions") {
		t.Fatalf("runCertificateExport() error = %v, want password permission error", err)
	}
}

func TestRunCertificateExportRejectsSymlinkInputsAndDestination(t *testing.T) {
	dir := t.TempDir()
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	certificateTarget := filepath.Join(dir, "certificate-target.cer")
	certificatePath := filepath.Join(dir, "certificate.cer")
	privateKeyPath := filepath.Join(dir, "push.key")
	passwordPath := filepath.Join(dir, "password")
	p12Path := filepath.Join(dir, "push.p12")
	writeCertificateExportTestFiles(t, certificateTarget, privateKeyPath, "", passwordPath, artifacts, []byte("password"))
	if err := os.Symlink(certificateTarget, certificatePath); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}

	_, err := runCertificateExport(context.TODO(), certificatesTestDeps(nil), certificateExportOptions{
		CertificatePath: certificatePath,
		PrivateKeyPath:  privateKeyPath,
		PasswordPath:    passwordPath,
		P12Out:          p12Path,
	})
	if err == nil || !strings.Contains(err.Error(), "without following symlinks") {
		t.Fatalf("runCertificateExport() error = %v, want symlink input error", err)
	}

	if err := os.Remove(certificatePath); err != nil {
		t.Fatalf("remove certificate symlink: %v", err)
	}
	if err := os.WriteFile(certificatePath, artifacts.certificateDER, 0o644); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	destinationTarget := filepath.Join(dir, "destination-target.p12")
	if err := os.WriteFile(destinationTarget, []byte("unchanged"), 0o600); err != nil {
		t.Fatalf("write destination target: %v", err)
	}
	if err := os.Symlink(destinationTarget, p12Path); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	_, err = runCertificateExport(context.TODO(), certificatesTestDeps(nil), certificateExportOptions{
		CertificatePath: certificatePath,
		PrivateKeyPath:  privateKeyPath,
		PasswordPath:    passwordPath,
		P12Out:          p12Path,
		Force:           true,
	})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("runCertificateExport() error = %v, want symlink destination error", err)
	}
	target, readErr := os.ReadFile(destinationTarget)
	if readErr != nil {
		t.Fatalf("read destination target: %v", readErr)
	}
	if string(target) != "unchanged" {
		t.Fatalf("symlink target changed: %q", target)
	}
}

func TestRunCertificateExportRejectsSymlinkedDestinationParent(t *testing.T) {
	dir := t.TempDir()
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	certificatePath := filepath.Join(dir, "push.cer")
	privateKeyPath := filepath.Join(dir, "push.key")
	passwordPath := filepath.Join(dir, "password")
	writeCertificateExportTestFiles(t, certificatePath, privateKeyPath, "", passwordPath, artifacts, []byte("password"))

	outside := filepath.Join(dir, "outside")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatalf("create outside directory: %v", err)
	}
	linkedParent := filepath.Join(dir, "linked-parent")
	if err := os.Symlink(outside, linkedParent); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	p12Path := filepath.Join(linkedParent, "push.p12")

	_, err := runCertificateExport(context.TODO(), certificatesTestDeps(nil), certificateExportOptions{
		CertificatePath: certificatePath,
		PrivateKeyPath:  privateKeyPath,
		PasswordPath:    passwordPath,
		P12Out:          p12Path,
	})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("runCertificateExport() error = %v, want symlinked-parent error", err)
	}
	if _, statErr := os.Lstat(filepath.Join(outside, "push.p12")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("output was published through symlinked parent, stat error = %v", statErr)
	}
}

func TestCertificateExportPathAllowsUnixTrailingBackslash(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("backslash is a path separator on Windows")
	}
	path := filepath.Join(t.TempDir(), "identity.p12") + `\`
	if _, err := validateCertificateExportOutputPath(certificatesTestDeps(nil), path); err != nil {
		t.Fatalf("validateCertificateExportOutputPath(%q) error = %v, want accepted Unix filename", path, err)
	}
}

func TestRunCertificateExportWritesUnixTrailingBackslashFilename(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("backslash is a path separator on Windows")
	}
	dir := t.TempDir()
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	certificatePath := filepath.Join(dir, "push.cer")
	privateKeyPath := filepath.Join(dir, "push.key")
	passwordPath := filepath.Join(dir, "password")
	writeCertificateExportTestFiles(t, certificatePath, privateKeyPath, "", passwordPath, artifacts, []byte("password"))
	p12Path := filepath.Join(dir, "identity.p12") + `\`

	if _, err := runCertificateExport(context.TODO(), certificatesTestDeps(nil), certificateExportOptions{
		CertificatePath: certificatePath,
		PrivateKeyPath:  privateKeyPath,
		PasswordPath:    passwordPath,
		P12Out:          p12Path,
	}); err != nil {
		t.Fatalf("runCertificateExport() error = %v, want Unix filename ending in backslash", err)
	}
	if _, err := os.Stat(p12Path); err != nil {
		t.Fatalf("Stat(%q) error = %v", p12Path, err)
	}
}

func TestRunCertificateExportRejectsInputOutputCollision(t *testing.T) {
	dir := t.TempDir()
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	certificatePath := filepath.Join(dir, "push.cer")
	privateKeyPath := filepath.Join(dir, "push.key")
	passwordPath := filepath.Join(dir, "password")
	writeCertificateExportTestFiles(t, certificatePath, privateKeyPath, "", passwordPath, artifacts, []byte("password"))

	_, err := runCertificateExport(context.TODO(), certificatesTestDeps(nil), certificateExportOptions{
		CertificatePath: certificatePath,
		PrivateKeyPath:  privateKeyPath,
		PasswordPath:    passwordPath,
		P12Out:          certificatePath,
		Force:           true,
	})
	if err == nil || !strings.Contains(err.Error(), "must differ from input path") {
		t.Fatalf("runCertificateExport() error = %v, want input/output collision error", err)
	}

	hardlinkPath := filepath.Join(dir, "hardlink.p12")
	if err := os.Link(certificatePath, hardlinkPath); err != nil {
		t.Skipf("hardlinks are unavailable: %v", err)
	}
	_, err = runCertificateExport(context.TODO(), certificatesTestDeps(nil), certificateExportOptions{
		CertificatePath: certificatePath,
		PrivateKeyPath:  privateKeyPath,
		PasswordPath:    passwordPath,
		P12Out:          hardlinkPath,
		Force:           true,
	})
	if err == nil || !strings.Contains(err.Error(), "must not resolve to input path") {
		t.Fatalf("runCertificateExport() error = %v, want hardlink collision error", err)
	}
}

func TestParseCertificateExportPrivateKeyFormats(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	rsaDER := x509.MarshalPKCS1PrivateKey(rsaKey)
	if key, err := parseCertificateExportPrivateKey(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: rsaDER})); err != nil {
		t.Fatalf("parse PKCS#1 key error: %v", err)
	} else if _, ok := key.(*rsa.PrivateKey); !ok {
		t.Fatalf("PKCS#1 key type = %T, want *rsa.PrivateKey", key)
	}

	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey(EC) error: %v", err)
	}
	ecDER, err := x509.MarshalECPrivateKey(ecKey)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey() error: %v", err)
	}
	if key, err := parseCertificateExportPrivateKey(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: ecDER})); err != nil {
		t.Fatalf("parse EC key error: %v", err)
	} else if _, ok := key.(*ecdsa.PrivateKey); !ok {
		t.Fatalf("EC key type = %T, want *ecdsa.PrivateKey", key)
	}
}

func TestParseCertificateExportCSRRejectsTrailingDER(t *testing.T) {
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	csrBlock, _ := pem.Decode(artifacts.csr)
	if csrBlock == nil {
		t.Fatal("test CSR is not PEM")
	}
	trailing := append(append([]byte(nil), csrBlock.Bytes...), 0x00)
	if _, err := parseCertificateExportCSR(trailing); err == nil || !strings.Contains(err.Error(), "exactly one DER object") {
		t.Fatalf("parseCertificateExportCSR() error = %v, want trailing-DER error", err)
	}
}

func TestPrepareCertificateExportOutputVerifiesUnixPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions are not enforced on Windows")
	}
	path := filepath.Join(t.TempDir(), "staging.p12")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("create staging file: %v", err)
	}
	defer file.Close()

	if err := prepareCertificateExportOutput(file); err != nil {
		t.Fatalf("prepareCertificateExportOutput() error = %v for an owner-only staging file", err)
	}

	if err := file.Chmod(0o640); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}
	if err := prepareCertificateExportOutput(file); err == nil || !strings.Contains(err.Error(), "permissions") {
		t.Fatalf("prepareCertificateExportOutput() error = %v, want broad-permission rejection", err)
	}
}

func TestRunCertificateExportRejectsParentSwappedForSymlinkAfterPreflight(t *testing.T) {
	dir := t.TempDir()
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	certificatePath := filepath.Join(dir, "push.cer")
	privateKeyPath := filepath.Join(dir, "push.key")
	passwordPath := filepath.Join(dir, "password")
	writeCertificateExportTestFiles(t, certificatePath, privateKeyPath, "", passwordPath, artifacts, []byte("password"))

	outside := filepath.Join(dir, "outside")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatalf("create outside directory: %v", err)
	}
	parent := filepath.Join(dir, "out")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatalf("create parent directory: %v", err)
	}
	p12Path := filepath.Join(parent, "push.p12")

	certificateExportTestHookAfterPreflight = func() {
		certificateExportTestHookAfterPreflight = nil
		if err := os.Rename(parent, parent+".real"); err != nil {
			t.Fatalf("swap parent aside: %v", err)
		}
		if err := os.Symlink(outside, parent); err != nil {
			t.Skipf("symlinks are unavailable: %v", err)
		}
	}
	defer func() { certificateExportTestHookAfterPreflight = nil }()

	_, err := runCertificateExport(context.TODO(), certificatesTestDeps(nil), certificateExportOptions{
		CertificatePath: certificatePath,
		PrivateKeyPath:  privateKeyPath,
		PasswordPath:    passwordPath,
		P12Out:          p12Path,
	})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("runCertificateExport() error = %v, want symlinked-parent refusal after the swap", err)
	}
	if _, statErr := os.Lstat(filepath.Join(outside, "push.p12")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("identity was published through the swapped parent, stat error = %v", statErr)
	}
}

func TestRunCertificateExportRejectsMissingParentUnderSwappedAncestor(t *testing.T) {
	dir := t.TempDir()
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	certificatePath := filepath.Join(dir, "push.cer")
	privateKeyPath := filepath.Join(dir, "push.key")
	passwordPath := filepath.Join(dir, "password")
	writeCertificateExportTestFiles(t, certificatePath, privateKeyPath, "", passwordPath, artifacts, []byte("password"))

	outside := filepath.Join(dir, "outside")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatalf("create outside directory: %v", err)
	}
	ancestor := filepath.Join(dir, "out")
	if err := os.Mkdir(ancestor, 0o700); err != nil {
		t.Fatalf("create ancestor directory: %v", err)
	}
	p12Path := filepath.Join(ancestor, "deep", "push.p12")

	certificateExportTestHookAfterPreflight = func() {
		certificateExportTestHookAfterPreflight = nil
		if err := os.Rename(ancestor, ancestor+".real"); err != nil {
			t.Fatalf("swap ancestor aside: %v", err)
		}
		if err := os.Symlink(outside, ancestor); err != nil {
			t.Skipf("symlinks are unavailable: %v", err)
		}
	}
	defer func() { certificateExportTestHookAfterPreflight = nil }()

	_, err := runCertificateExport(context.TODO(), certificatesTestDeps(nil), certificateExportOptions{
		CertificatePath: certificatePath,
		PrivateKeyPath:  privateKeyPath,
		PasswordPath:    passwordPath,
		P12Out:          p12Path,
	})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("runCertificateExport() error = %v, want symlinked-parent refusal after the swap", err)
	}
	if _, statErr := os.Lstat(filepath.Join(outside, "deep")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("missing parent was created through the swapped ancestor, stat error = %v", statErr)
	}
}

func TestRunCertificateExportPublishesIntoPinnedParentAfterSwap(t *testing.T) {
	dir := t.TempDir()
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	certificatePath := filepath.Join(dir, "push.cer")
	privateKeyPath := filepath.Join(dir, "push.key")
	passwordPath := filepath.Join(dir, "password")
	writeCertificateExportTestFiles(t, certificatePath, privateKeyPath, "", passwordPath, artifacts, []byte("password"))

	outside := filepath.Join(dir, "outside")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatalf("create outside directory: %v", err)
	}
	parent := filepath.Join(dir, "out")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatalf("create parent directory: %v", err)
	}
	p12Path := filepath.Join(parent, "push.p12")

	certificateExportTestHookAfterParentPinned = func() {
		certificateExportTestHookAfterParentPinned = nil
		if err := os.Rename(parent, parent+".real"); err != nil {
			t.Fatalf("swap parent aside: %v", err)
		}
		if err := os.Symlink(outside, parent); err != nil {
			t.Skipf("symlinks are unavailable: %v", err)
		}
	}
	defer func() { certificateExportTestHookAfterParentPinned = nil }()

	if _, err := runCertificateExport(context.TODO(), certificatesTestDeps(nil), certificateExportOptions{
		CertificatePath: certificatePath,
		PrivateKeyPath:  privateKeyPath,
		PasswordPath:    passwordPath,
		P12Out:          p12Path,
	}); err != nil {
		t.Fatalf("runCertificateExport() error = %v, want publication into the pinned parent", err)
	}
	if _, statErr := os.Lstat(filepath.Join(outside, "push.p12")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("identity was published through the swapped symlink, stat error = %v", statErr)
	}
	if _, statErr := os.Lstat(filepath.Join(parent+".real", "push.p12")); statErr != nil {
		t.Fatalf("identity missing from the pinned original parent: %v", statErr)
	}
}

func TestRunCertificateExportCreatesMissingParentsThroughPinnedRoots(t *testing.T) {
	dir := t.TempDir()
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	certificatePath := filepath.Join(dir, "push.cer")
	privateKeyPath := filepath.Join(dir, "push.key")
	passwordPath := filepath.Join(dir, "password")
	writeCertificateExportTestFiles(t, certificatePath, privateKeyPath, "", passwordPath, artifacts, []byte("password"))
	p12Path := filepath.Join(dir, "new1", "new2", "push.p12")

	if _, err := runCertificateExport(context.TODO(), certificatesTestDeps(nil), certificateExportOptions{
		CertificatePath: certificatePath,
		PrivateKeyPath:  privateKeyPath,
		PasswordPath:    passwordPath,
		P12Out:          p12Path,
	}); err != nil {
		t.Fatalf("runCertificateExport() error = %v, want rooted creation of missing parents", err)
	}
	info, err := os.Lstat(p12Path)
	if err != nil {
		t.Fatalf("Lstat(%q) error = %v", p12Path, err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("output mode = %v, want regular file", info.Mode())
	}
}

func TestOpenCertificateExportDestinationParentAllowsDarwinPrivateAliases(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS private directory aliases are not present on this platform")
	}

	for _, alias := range []string{"/tmp", "/var"} {
		t.Run(filepath.Base(alias), func(t *testing.T) {
			output := filepath.Join(alias, "asc-certificate-export-alias-test.p12")
			parent, err := openCertificateExportDestinationParent(certificatesTestDeps(nil), output, false)
			if err != nil {
				t.Fatalf("openCertificateExportDestinationParent(%q) error = %v, want known macOS alias accepted", output, err)
			}
			if parent == nil {
				t.Fatal("openCertificateExportDestinationParent() returned nil root for existing alias parent")
			}
			if err := parent.Close(); err != nil {
				t.Fatalf("close rooted alias parent: %v", err)
			}
		})
	}
}

func TestRunCertificateExportAnchorsDestinationAtDepsWorkingDirectory(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve temporary directory: %v", err)
	}
	realDir := filepath.Join(dir, "real")
	if err := os.Mkdir(realDir, 0o700); err != nil {
		t.Fatalf("create directory: %v", err)
	}
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	writeCertificateExportTestFiles(t, filepath.Join(realDir, "push.cer"), filepath.Join(realDir, "push.key"), "", filepath.Join(realDir, "password"), artifacts, []byte("password"))
	workingDir := filepath.Join(dir, "link")
	if err := os.Symlink(realDir, workingDir); err != nil {
		t.Skipf("create symlink: %v", err)
	}
	options := certificateExportOptions{
		CertificatePath: "push.cer",
		PrivateKeyPath:  "push.key",
		PasswordPath:    "password",
		P12Out:          "push.p12",
	}
	t.Chdir(t.TempDir())

	d := certificatesTestDeps(nil)
	d.Getwd = func() (string, error) { return workingDir, nil }
	if _, err := runCertificateExport(context.TODO(), d, options); err != nil {
		t.Fatalf("runCertificateExport() with relative paths below the deps working directory error = %v, want them read and written there", err)
	}
	if info, err := os.Lstat(filepath.Join(realDir, "push.p12")); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("p12 output = %v, %v; want a regular file", info, err)
	}
}
