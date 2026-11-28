package signing

import (
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/artifacts/artifactstest"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
)

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
