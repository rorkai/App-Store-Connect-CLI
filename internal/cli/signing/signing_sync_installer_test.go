package signing

import (
	"bytes"
	"crypto/x509"
	"encoding/asn1"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/artifacts/artifactstest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
	modernpkcs12 "software.sslmate.com/src/go-pkcs12"
)

func TestSigningSyncPullInstallerEncoding(t *testing.T) {
	fixture := newOpenSSLImportFixture(t, 1)
	installer := addOpenSSLInstallerFixture(t, fixture.dir, "developer_id_installer", artifactstest.DeveloperIDInstallerLeaf)
	installerPath := "identities/developer_id_installer/" + signingCertificateSHA256(installer) + ".p12"
	appPath := "identities/distribution/" + fixture.distributionSHA + ".p12"
	passwordFile := openSSLSourcePasswordFile(t, importSourcePassword)
	remote, remotePath := newSigningSyncBareRemote(t)
	if _, stderr, err := runSigningSyncImport(t, "--source", fixture.dir, "--source-password", importSourcePassword,
		"--repo", remote, "--password-file", passwordFile, "--include-installers", "--confirm"); err != nil {
		t.Fatalf("seed store: %v\n%s", err, stderr)
	}
	before := runGitCommand(t, remotePath, "rev-parse", "HEAD")
	for _, source := range []string{"store", "openssl"} {
		for _, encoding := range []string{"default", "legacy", "modern"} {
			t.Run(source+"/"+encoding, func(t *testing.T) {
				outDir := filepath.Join(t.TempDir(), "output")
				args := []string{"--repo", remote, "--password-file", passwordFile, "--output-dir", outDir, "--output", "json"}
				if source == "openssl" {
					args = append(args, "--source-format", "openssl", "--repo", fixture.dir)
				}
				if encoding != "default" {
					args = append(args, "--p12-encoding", encoding)
				}
				stdout, stderr, err := runSigningSyncCommandForStorage(t, syncPullCommand, nil, args)
				if err != nil {
					t.Fatalf("pull: %v\n%s", err, stderr)
				}
				var result asc.SigningSyncResult
				if err := json.Unmarshal([]byte(stdout), &result); err != nil || !result.IdentityPresent {
					t.Fatalf("invalid pull receipt: %s (%v)", stdout, err)
				}
				for path, cert := range map[string]*x509.Certificate{installerPath: installer, appPath: fixture.distribution} {
					data, err := os.ReadFile(filepath.Join(outDir, filepath.FromSlash(path)))
					if err != nil {
						t.Fatal(err)
					}
					key, got, err := modernpkcs12.Decode(data, importSourcePassword)
					if err != nil || !got.Equal(cert) || !publicKeysEqual(key, cert.PublicKey) {
						t.Fatalf("%s lost its key pair: %v", path, err)
					}
					wantOID := asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 1, 3} // 3DES
					if path == appPath || encoding == "modern" {
						wantOID = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 42} // AES-256-CBC
					}
					oid, err := asn1.Marshal(wantOID)
					if err != nil || !bytes.Contains(data, oid) {
						t.Fatalf("%s does not use %s: %v", path, wantOID, err)
					}
				}
			})
		}
	}
	if after := runGitCommand(t, remotePath, "rev-parse", "HEAD"); after != before {
		t.Fatalf("pull changed the encrypted store: %s -> %s", before, after)
	}
}

func TestSigningSyncPullRejectsEncodingWithoutInstallers(t *testing.T) {
	fixture := newOpenSSLImportFixture(t, 1)
	passwordFile := openSSLSourcePasswordFile(t, importSourcePassword)
	remote, _ := newSigningSyncBareRemote(t)
	if _, stderr, err := runSigningSyncImport(t, "--source", fixture.dir, "--source-password", importSourcePassword,
		"--repo", remote, "--password-file", passwordFile, "--confirm"); err != nil {
		t.Fatalf("seed store: %v\n%s", err, stderr)
	}
	emptyRemote, _ := newSigningSyncBareRemote(t)
	for _, test := range []struct {
		name string
		args []string
	}{
		{"app identities in store", []string{"--repo", remote}},
		{"app identities in OpenSSL source", []string{"--repo", fixture.dir, "--source-format", "openssl"}},
		{"empty store", []string{"--repo", emptyRemote}},
	} {
		t.Run(test.name, func(t *testing.T) {
			outDir := filepath.Join(t.TempDir(), "output")
			args := append([]string{"--password-file", passwordFile, "--p12-encoding", "modern", "--output-dir", outDir}, test.args...)
			stdout, _, err := runSigningSyncCommandForStorage(t, syncPullCommand, nil, args)
			if err == nil || err.Error() != "signing sync pull: --p12-encoding requires at least one active installer identity" || stdout != "" {
				t.Fatalf("error=%v stdout=%q; want no receipt and missing installer error", err, stdout)
			}
			if _, err := os.Stat(outDir); !os.IsNotExist(err) {
				t.Fatalf("failed pull created its output directory: %v", err)
			}
		})
	}
}

func TestSigningSyncInstallerRestoreChecksPurposeAndValidity(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	chain := artifactstest.NewTrustChainWithLeaf(t, artifactstest.DeveloperIDInstallerLeaf, now.Add(-time.Hour), now.Add(time.Hour))
	fingerprint := signingCertificateSHA256(chain.Leaf)
	identity, err := normalizeSigningIdentity(&signingIdentity{PrivateKey: chain.LeafKey, Certificate: chain.Leaf, CertificateSHA256: fingerprint}, "fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, category string
		at             time.Time
		wantCount      int
		wantError      string
	}{
		{name: "valid", category: "developer_id_installer", at: now, wantCount: 1},
		{name: "wrong purpose", category: "mac_installer_distribution", at: now, wantError: "installer certificate purpose"},
		{name: "expired", category: "developer_id_installer", at: now.Add(2 * time.Hour)},
		{name: "not yet valid", category: "developer_id_installer", at: now.Add(-2 * time.Hour)},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &signingpkg.GitStore{LocalDir: t.TempDir()}
			path := "identities/" + test.category + "/" + fingerprint + ".p12"
			if err := store.WriteEncryptedFileWithMetadata(path, identity, "fixture-password", signingpkg.EncryptedFileMetadata{
				Kind: installerIdentityKind, Sensitive: true, CertificateSHA256: fingerprint, TeamID: certificateTeamID(chain.Leaf),
			}); err != nil {
				t.Fatal(err)
			}
			d := testSigningDeps()
			d.Now = func() time.Time { return test.at }
			files, err := decryptAndValidateSigningFiles(d, store, []string{path}, "fixture-password")
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("restore error=%v, want %s", err, test.wantError)
				}
				return
			}
			if err != nil || len(files) != test.wantCount {
				t.Fatalf("restored %d identities, want %d; error=%v", len(files), test.wantCount, err)
			}
		})
	}
}
