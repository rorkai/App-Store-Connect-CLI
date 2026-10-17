//go:build darwin && cgo

package cmd

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bitrise-io/go-pkcs12"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// writeSigningLocalInputIdentity writes a decodable, currently valid PKCS#12
// identity protected by password so later preflight stages can be reached.
func writeSigningLocalInputIdentity(t *testing.T, path, password string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "ASC Local Input Test", OrganizationalUnit: []string{"TEAM123456"}},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	data, err := pkcs12.Encode(rand.Reader, key, certificate, nil, password)
	if err != nil {
		t.Fatalf("encode PKCS#12: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error: %v", path, err)
	}
	return path
}

// writeSigningInputFixture writes a fixture with an exact mode, so permission
// checks see it regardless of the umask.
func writeSigningInputFixture(t *testing.T, path, data string, mode os.FileMode) string {
	t.Helper()
	writeLocalInputFixture(t, path, data)
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("Chmod(%q) error: %v", path, err)
	}
	return path
}

func TestRunSigningLocalInputFileFailuresAreClassified(t *testing.T) {
	dir := t.TempDir()
	missing := func(name string) string { return filepath.Join(dir, "missing-"+name) }
	identity := writeSigningLocalInputIdentity(t, filepath.Join(dir, "valid.p12"), "right")
	junkIdentity := writeSigningInputFixture(t, filepath.Join(dir, "junk.p12"), "junk\n", 0o600)
	exposedIdentity := writeSigningInputFixture(t, filepath.Join(dir, "exposed.p12"), "junk\n", 0o644)
	profile := writeSigningInputFixture(t, filepath.Join(dir, "junk.mobileprovision"), "junk\n", 0o644)
	writableProfile := writeSigningInputFixture(t, filepath.Join(dir, "writable.mobileprovision"), "junk\n", 0o666)
	rightPassword := writeSigningInputFixture(t, filepath.Join(dir, "right-password"), "right\n", 0o600)
	wrongPassword := writeSigningInputFixture(t, filepath.Join(dir, "wrong-password"), "wrong\n", 0o600)
	keychainPassword := writeSigningInputFixture(t, filepath.Join(dir, "keychain-password"), "keychain\n", 0o600)
	keychain := filepath.Join(dir, "release.keychain-db")

	openErr := func(path string) string {
		t.Helper()
		_, err := os.Open(path)
		if err == nil {
			t.Fatalf("os.Open(%q) unexpectedly succeeded", path)
		}
		return err.Error()
	}
	run := func(args ...string) []string {
		return append(append([]string{"signing", "run"}, args...), "--", "true")
	}
	install := func(identityPath, passwordPath, keychainPasswordPath string) []string {
		return []string{
			"signing", "keychain", "install",
			"--identity", identityPath,
			"--identity-password-file", passwordPath,
			"--keychain", keychain,
			"--keychain-password-file", keychainPasswordPath,
			"--confirm",
		}
	}

	tests := []localInputOutcomeCase{
		{
			name:             "signing run missing identity",
			args:             run("--identity", missing("identity.p12"), "--profile", profile),
			wantStderrPrefix: "Error: signing run: read identity: " + openErr(missing("identity.p12")) + "\n",
			wantDiagnosis:    shared.DiagnosticFileNotFound,
			wantParameter:    "--identity",
		},
		{
			name:             "signing run missing profile",
			args:             run("--identity", junkIdentity, "--profile", missing("profile.mobileprovision")),
			wantStderrPrefix: "Error: signing run: read profile: " + openErr(missing("profile.mobileprovision")) + "\n",
			wantDiagnosis:    shared.DiagnosticFileNotFound,
			wantParameter:    "--profile",
		},
		{
			name:             "signing run missing identity password file",
			args:             run("--identity", junkIdentity, "--profile", profile, "--identity-password-file", missing("password")),
			wantStderrPrefix: "Error: signing run: read identity password: " + openErr(missing("password")) + "\n",
			wantDiagnosis:    shared.DiagnosticFileNotFound,
			wantParameter:    "--identity-password-file",
		},
		{
			name:             "signing run identity readable by others",
			args:             run("--identity", exposedIdentity, "--profile", profile),
			wantStderrPrefix: "Error: signing run: read identity: \"" + exposedIdentity + "\" must not be accessible by group or other users\n",
			wantDiagnosis:    shared.DiagnosticFilePermissionsInsecure,
			wantParameter:    "--identity",
		},
		{
			name:             "signing run profile writable by others",
			args:             run("--identity", junkIdentity, "--profile", writableProfile),
			wantStderrPrefix: "Error: signing run: read profile: \"" + writableProfile + "\" must not be writable by group or other users\n",
			wantDiagnosis:    shared.DiagnosticFilePermissionsInsecure,
			wantParameter:    "--profile",
		},
		{
			name:             "signing run undecodable identity",
			args:             run("--identity", junkIdentity, "--profile", profile),
			wantStderrPrefix: "Error: signing run: preflight: decode identity: pkcs12: ",
			wantDiagnosis:    shared.DiagnosticFileInvalidFormat,
			wantParameter:    "--identity",
		},
		{
			name:             "signing run wrong identity password",
			args:             run("--identity", identity, "--profile", profile, "--identity-password-file", wrongPassword),
			wantStderrPrefix: "Error: signing run: preflight: decode identity: pkcs12: decryption password incorrect\n",
			wantDiagnosis:    shared.DiagnosticInvalidInput,
			wantParameter:    "--identity-password-file",
		},
		{
			name:             "signing run undecodable profile",
			args:             run("--identity", identity, "--profile", profile, "--identity-password-file", rightPassword),
			wantStderrPrefix: "Error: signing run: preflight: decode signed provisioning profile: ",
			wantDiagnosis:    shared.DiagnosticFileInvalidFormat,
			wantParameter:    "--profile",
		},
		{
			name:             "signing keychain install missing identity",
			args:             install(missing("identity.p12"), rightPassword, keychainPassword),
			wantStderrPrefix: "Error: signing keychain install: read identity: " + openErr(missing("identity.p12")) + "\n",
			wantDiagnosis:    shared.DiagnosticFileNotFound,
			wantParameter:    "--identity",
		},
		{
			name:             "signing keychain install identity readable by others",
			args:             install(exposedIdentity, rightPassword, keychainPassword),
			wantStderrPrefix: "Error: signing keychain install: read identity: \"" + exposedIdentity + "\" must not be accessible by group or other users\n",
			wantDiagnosis:    shared.DiagnosticFilePermissionsInsecure,
			wantParameter:    "--identity",
		},
		{
			name:             "signing keychain install missing keychain password file",
			args:             install(junkIdentity, rightPassword, missing("keychain-password")),
			wantStderrPrefix: "Error: signing keychain install: read keychain password: " + openErr(missing("keychain-password")) + "\n",
			wantDiagnosis:    shared.DiagnosticFileNotFound,
			wantParameter:    "--keychain-password-file",
		},
		{
			name:             "signing keychain install undecodable identity",
			args:             install(junkIdentity, rightPassword, keychainPassword),
			wantStderrPrefix: "Error: signing keychain install: inspect identity: decode identity: pkcs12: ",
			wantDiagnosis:    shared.DiagnosticFileInvalidFormat,
			wantParameter:    "--identity",
		},
		{
			name:             "signing keychain install wrong identity password",
			args:             install(identity, wrongPassword, keychainPassword),
			wantStderrPrefix: "Error: signing keychain install: inspect identity: decode identity: pkcs12: decryption password incorrect\n",
			wantDiagnosis:    shared.DiagnosticInvalidInput,
			wantParameter:    "--identity-password-file",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runLocalInputOutcomeCase(t, test)
			if _, err := os.Lstat(keychain); !os.IsNotExist(err) {
				t.Fatalf("keychain %q exists after a failed preflight (err=%v)", keychain, err)
			}
		})
	}
}
