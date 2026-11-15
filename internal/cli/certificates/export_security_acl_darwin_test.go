//go:build darwin

package certificates

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// applyDarwinTestACL grants another account read access through an extended
// ACL entry, which mode bits do not reflect.
func applyDarwinTestACL(t *testing.T, path string) {
	t.Helper()
	output, err := exec.Command("/bin/chmod", "+a", "everyone allow read", path).CombinedOutput()
	if err != nil {
		t.Skipf("cannot apply test ACL: %v (%s)", err, strings.TrimSpace(string(output)))
	}
}

func darwinFileHasTestACL(t *testing.T, path string) bool {
	t.Helper()
	output, err := exec.Command("/bin/ls", "-le", path).CombinedOutput()
	if err != nil {
		t.Fatalf("ls -le %q: %v (%s)", path, err, strings.TrimSpace(string(output)))
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && strings.HasSuffix(fields[0], ":") {
			return true
		}
	}
	return false
}

func TestValidateCertificateExportProtectedFileRejectsExtendedACL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "push.key")
	if err := os.WriteFile(path, []byte("key material"), 0o600); err != nil {
		t.Fatalf("write protected file: %v", err)
	}
	applyDarwinTestACL(t, path)

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open protected file: %v", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatalf("stat protected file: %v", err)
	}

	err = validateCertificateExportProtectedFile(file, info, "private key")
	if err == nil || !strings.Contains(err.Error(), "ACL") {
		t.Fatalf("validateCertificateExportProtectedFile() error = %v, want extended-ACL rejection", err)
	}
}

func TestPrepareCertificateExportOutputStripsExtendedACL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "staging.p12")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("create staging file: %v", err)
	}
	defer file.Close()
	applyDarwinTestACL(t, path)
	if !darwinFileHasTestACL(t, path) {
		t.Fatal("test ACL was not applied")
	}

	if err := prepareCertificateExportOutput(file); err != nil {
		t.Fatalf("prepareCertificateExportOutput() error = %v, want inherited ACL stripped", err)
	}
	if darwinFileHasTestACL(t, path) {
		t.Fatal("staging file retains its extended ACL after preparation")
	}
}

func TestCreateCertificateExportStagingFileIsOwnerOnlyAtCreation(t *testing.T) {
	directory := t.TempDir()
	output, err := exec.Command("/bin/chmod", "+a", "everyone allow read,file_inherit", directory).CombinedOutput()
	if err != nil {
		t.Skipf("cannot apply inheritable test ACL: %v (%s)", err, strings.TrimSpace(string(output)))
	}

	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("OpenRoot() error = %v", err)
	}
	defer root.Close()

	const name = ".asc-cert-export-staging-security-test"
	file, err := createCertificateExportStagingFile(root, name, 0o600)
	if err != nil {
		t.Fatalf("createCertificateExportStagingFile() error = %v", err)
	}
	defer file.Close()
	defer func() { _ = root.Remove(name) }()

	if darwinFileHasTestACL(t, filepath.Join(directory, name)) {
		t.Fatal("staging file carried an inherited extended ACL after atomic creation")
	}
	// The ordinary writer preparation remains a separate pre-write check and
	// must be harmless after creator-time hardening.
	if err := prepareCertificateExportOutput(file); err != nil {
		t.Fatalf("prepareCertificateExportOutput() error = %v after creator-time hardening", err)
	}
}

func TestRunCertificateExportStripsACLInheritedFromDestinationDirectory(t *testing.T) {
	dir := t.TempDir()
	artifacts := newCertificateExportTestArtifacts(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	certificatePath := filepath.Join(dir, "push.cer")
	privateKeyPath := filepath.Join(dir, "push.key")
	passwordPath := filepath.Join(dir, "password")
	writeCertificateExportTestFiles(t, certificatePath, privateKeyPath, "", passwordPath, artifacts, []byte("password"))

	destination := filepath.Join(dir, "out")
	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatalf("create destination directory: %v", err)
	}
	output, err := exec.Command("/bin/chmod", "+a", "everyone allow read,file_inherit", destination).CombinedOutput()
	if err != nil {
		t.Skipf("cannot apply inheritable test ACL: %v (%s)", err, strings.TrimSpace(string(output)))
	}
	p12Path := filepath.Join(destination, "push.p12")

	if _, err := runCertificateExport(context.TODO(), certificatesTestDeps(nil), certificateExportOptions{
		CertificatePath: certificatePath,
		PrivateKeyPath:  privateKeyPath,
		PasswordPath:    passwordPath,
		P12Out:          p12Path,
	}); err != nil {
		t.Fatalf("runCertificateExport() error = %v, want inherited ACL stripped from the identity", err)
	}
	if darwinFileHasTestACL(t, p12Path) {
		t.Fatal("published identity carries an inherited extended ACL")
	}
}
