package signing

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	nativepkcs12 "golang.org/x/crypto/pkcs12"
	modernpkcs12 "software.sslmate.com/src/go-pkcs12"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/artifacts/artifactstest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func runOpenSSLSourcePull(t *testing.T, args ...string) (asc.SigningSyncResult, string, error) {
	t.Helper()
	t.Setenv(signingSyncPasswordEnvVar, "")
	d, depsStdout, depsStderr := newSigningTestDeps()
	cmd := syncPullCommand(d)
	cmd.FlagSet.SetOutput(io.Discard)
	var runErr error
	if err := cmd.Parse(append([]string{"--source-format", "openssl", "--output", "json"}, args...)); err != nil {
		t.Fatalf("parse: %v", err)
	}
	runErr = cmd.Run(context.Background())
	stdout := depsStdout.String()
	stderr := depsStderr.String()
	if strings.Contains(stdout+stderr, importSourcePassword) {
		t.Fatalf("password leaked to output:\nstdout=%s\nstderr=%s", stdout, stderr)
	}
	var result asc.SigningSyncResult
	if runErr == nil {
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("decode result: %v\n%s", err, stdout)
		}
	} else if stdout != "" {
		t.Fatalf("failed pull emitted a receipt: %s", stdout)
	}
	return result, stderr, runErr
}

func openSSLSourcePasswordFile(t *testing.T, password string) string {
	t.Helper()
	passwordFile := filepath.Join(t.TempDir(), "password")
	writePrivateTestFile(t, passwordFile, []byte(password))
	return passwordFile
}

func TestSigningSyncPullOpenSSLSourceWritesUsableIdentity(t *testing.T) {
	fixture := newOpenSSLImportFixture(t, 1)
	if err := os.RemoveAll(filepath.Join(fixture.dir, "certs", "development")); err != nil {
		t.Fatal(err)
	}
	installer := addOpenSSLInstallerFixture(t, fixture.dir, "developer_id_installer", artifactstest.DeveloperIDInstallerLeaf)
	installerPath := "identities/developer_id_installer/" + signingCertificateSHA256(installer) + ".p12"
	outDir := filepath.Join(t.TempDir(), "pulled")

	result, stderr, err := runOpenSSLSourcePull(t, "--repo", fixture.dir, "--password-file", openSSLSourcePasswordFile(t, importSourcePassword), "--output-dir", outDir)
	if err != nil {
		t.Fatalf("pull: %v\nstderr=%s", err, stderr)
	}
	identityPath := "identities/distribution/" + fixture.distributionSHA + ".p12"
	if result.SourceFormat != "openssl" || result.RepoURL != fixture.dir || !result.IdentityPresent || !reflect.DeepEqual(result.SensitiveFiles, []string{installerPath, identityPath}) {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Skipped) != 0 || strings.Contains(stderr, "Warning: skipping") {
		t.Fatalf("active installer was skipped: %+v, stderr=%s", result.Skipped, stderr)
	}
	for _, relPath := range result.Files {
		info, err := os.Stat(filepath.Join(outDir, filepath.FromSlash(relPath)))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("%s: mode %v, error %v; want 0600", relPath, info, err)
		}
	}
	for path, wanted := range map[string]*x509.Certificate{identityPath: fixture.distribution, installerPath: installer} {
		data, err := os.ReadFile(filepath.Join(outDir, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		key, certificate, err := modernpkcs12.Decode(data, importSourcePassword)
		if err != nil || !certificate.Equal(wanted) || !publicKeysEqual(key, wanted.PublicKey) {
			t.Fatalf("pulled identity %s lost its source key pair: %v", path, err)
		}
		if path == installerPath {
			key, certificate, err = nativepkcs12.Decode(data, importSourcePassword)
			if err != nil || !certificate.Equal(wanted) || !publicKeysEqual(key, wanted.PublicKey) {
				t.Fatalf("restored installer is not compatible with native PKCS#12 import: %v", err)
			}
		}
	}

	t.Run("a certificate no valid profile links is reported", func(t *testing.T) {
		fixture := newOpenSSLImportFixture(t, 1)
		result, stderr, err := runOpenSSLSourcePull(t, "--repo", fixture.dir, "--password-file", openSSLSourcePasswordFile(t, importSourcePassword),
			"--output-dir", filepath.Join(t.TempDir(), "pulled"))
		if err != nil {
			t.Fatalf("pull: %v\nstderr=%s", err, stderr)
		}
		want := []asc.SigningSyncSkippedEntry{{Path: "certs/development/CERT2.p12", Reason: "no currently valid profile links this private key; only the certificate is written"}}
		if !reflect.DeepEqual(result.Skipped, want) || !strings.Contains(stderr, "Warning: skipping certs/development/CERT2.p12") {
			t.Fatalf("skipped = %+v, stderr = %q", result.Skipped, stderr)
		}
		if !reflect.DeepEqual(result.SensitiveFiles, []string{"identities/distribution/" + fixture.distributionSHA + ".p12"}) {
			t.Fatalf("sensitive files = %v", result.SensitiveFiles)
		}
	})
}

func TestSigningSyncPullOpenSSLSourceSelectsBundle(t *testing.T) {
	fixture := newOpenSSLImportFixture(t, 2)
	result, stderr, err := runOpenSSLSourcePull(t, "--repo", fixture.dir, "--password-file", openSSLSourcePasswordFile(t, importSourcePassword),
		"--output-dir", filepath.Join(t.TempDir(), "pulled"), "--bundle-id", "com.example.app1", "--profile-type", "IOS_APP_STORE")
	if err != nil {
		t.Fatalf("pull: %v\nstderr=%s", err, stderr)
	}
	var profiles []string
	for _, relPath := range result.Files {
		if strings.HasPrefix(relPath, "profiles/") {
			profiles = append(profiles, relPath)
		}
	}
	if result.BundleID != "com.example.app1" || !reflect.DeepEqual(profiles, []string{"profiles/appstore/App Store com.example.app1.mobileprovision"}) || !result.IdentityPresent {
		t.Fatalf("result = %+v, profiles = %v", result, profiles)
	}
}

func TestSigningSyncPullOpenSSLSourceFailuresWriteNothing(t *testing.T) {
	fixture := newOpenSSLImportFixture(t, 1)
	protectedKey := newOpenSSLImportFixture(t, 1)
	key := mustECKey(t)
	protected, err := modernpkcs12.Legacy.Encode(key, mustSigningCertificate(t, key, 303), nil, "key-password")
	if err != nil {
		t.Fatal(err)
	}
	writePrivateTestFile(t, filepath.Join(protectedKey.dir, "certs", "development", "CERT2.p12"), encryptOpenSSLV2(t, protected))
	tests := []struct {
		name    string
		args    []string
		usage   bool
		wantErr string
	}{
		{
			name: "wrong password", args: []string{"--password-file", openSSLSourcePasswordFile(t, "not-the-password")},
			wantErr: "signing sync pull: decrypt certs/development/CERT2.cer: wrong password or not an OpenSSL-encrypted signing file",
		},
		{
			name: "password-protected private key", args: []string{"--repo", protectedKey.dir},
			wantErr: "signing sync pull: decrypt certs/development/CERT2.p12: private key must be PEM or a PKCS#12 file without a password",
		},
		{
			name: "other storage", args: []string{"--storage", "object", "--object-bucket", "bucket"}, usage: true,
			wantErr: "--source-format openssl requires --storage git",
		},
		{
			name: "storage option", args: []string{"--prefix", "asc"}, usage: true,
			wantErr: "--prefix requires --storage gitlab-secure-files or --storage aws-secrets-manager",
		},
		{
			name: "branch with a local checkout", args: []string{"--branch", "main"}, usage: true,
			wantErr: "--branch is only supported with a Git URL source",
		},
		{
			name: "unknown format", args: []string{"--source-format", "zip"}, usage: true,
			wantErr: "--source-format must be openssl",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outDir := filepath.Join(t.TempDir(), "pulled")
			args := append([]string{"--repo", fixture.dir, "--output-dir", outDir, "--password-file", openSSLSourcePasswordFile(t, importSourcePassword)}, test.args...)
			_, _, err := runOpenSSLSourcePull(t, args...)
			if err == nil || err.Error() != test.wantErr || errors.Is(err, flag.ErrHelp) != test.usage {
				t.Fatalf("error = %v, want %q (usage error %v)", err, test.wantErr, test.usage)
			}
			if _, err := os.Stat(outDir); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed pull created %s: %v", outDir, err)
			}
		})
	}
}
