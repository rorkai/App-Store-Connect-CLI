//go:build darwin

package signing

import (
	"context"
	"crypto/x509"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	modernpkcs12 "software.sslmate.com/src/go-pkcs12"
)

func TestConfigurePersistentSigningKeychainLeavesCleanupToCaller(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	configureErr := errors.New("configuration failed")
	deleted := false
	err := configurePersistentSigningKeychain(
		ctx,
		"/tmp/persistent.keychain-db",
		func(runCtx context.Context, _ []byte, _ ...string) ([]byte, []byte, error) {
			cancel()
			return nil, nil, configureErr
		},
	)
	if !errors.Is(err, configureErr) || deleted {
		t.Fatalf("configure error = %v, deleted = %v", err, deleted)
	}
}

func TestPersistentSigningProbeUsesPrivateTemporaryDirectory(t *testing.T) {
	operatorDir := t.TempDir()
	operatorProbe := filepath.Join(operatorDir, "codesign-probe")
	if err := os.WriteFile(operatorProbe, []byte("operator-owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	keychainPath := filepath.Join(operatorDir, "persistent.keychain-db")
	verified := false
	err := withPersistentSigningProbe(
		context.Background(),
		keychainPath,
		strings.Repeat("A", 40),
		createSigningRunTempDir,
		removeSigningRunTempDir,
		func(_ context.Context, probeDir, gotKeychainPath, _ string) error {
			if probeDir == operatorDir || filepath.Dir(probeDir) != filepath.Clean(os.TempDir()) {
				t.Fatalf("probe directory = %q, operator directory = %q", probeDir, operatorDir)
			}
			if gotKeychainPath != keychainPath {
				t.Fatalf("keychain path = %q", gotKeychainPath)
			}
			verified = true
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !verified {
		t.Fatal("codesign probe did not run")
	}
	data, err := os.ReadFile(operatorProbe)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "operator-owned" {
		t.Fatalf("operator probe content = %q", data)
	}
}

func TestSigningKeychainInstallLiveDedicatedKeychain(t *testing.T) {
	if os.Getenv("ASC_SIGNING_KEYCHAIN_INSTALL_LIVE_TEST") != "1" {
		t.Skip("set ASC_SIGNING_KEYCHAIN_INSTALL_LIVE_TEST=1 to exercise a disposable persistent keychain")
	}
	deps := platformSigningKeychainInstallDeps()
	if deps.GOOS != "darwin" || !deps.SecurityAvailable {
		t.Skip("requires a cgo-enabled macOS build")
	}

	before, err := deps.KeychainSearchList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	fixture := newSigningRunFixture(t, signingRunFixtureOptions{codeSigningLeaf: true, selfSignedCodeSigningLeaf: true})
	identity, err := inspectSigningRunIdentity(fixture.identity, []byte(fixture.password), fixture.now)
	if err != nil {
		t.Fatalf("inspect code-signing leaf fixture: %v", err)
	}
	if identity.Certificate == nil || identity.Certificate.IsCA || identity.Certificate.KeyUsage&x509.KeyUsageDigitalSignature == 0 || !slices.Contains(identity.Certificate.ExtKeyUsage, x509.ExtKeyUsageCodeSigning) {
		t.Fatalf("fixture certificate is not a code-signing leaf: %+v", identity.Certificate)
	}
	fixture.identity, err = modernpkcs12.Modern2023.Encode(identity.PrivateKey, identity.Certificate, nil, fixture.password)
	if err != nil {
		t.Fatalf("encode modern PKCS#12 identity: %v", err)
	}

	directory := t.TempDir()
	identityPath := filepath.Join(directory, "App.p12")
	identityPasswordPath := filepath.Join(directory, "identity-password")
	keychainPasswordPath := filepath.Join(directory, "keychain-password")
	trustAnchorPath := filepath.Join(directory, "test-root.cer")
	keychainPath := filepath.Join(directory, "release.keychain-db")
	writePrivateTestFile(t, identityPath, fixture.identity)
	writePrivateTestFile(t, identityPasswordPath, []byte(fixture.password+"\n"))
	writePrivateTestFile(t, keychainPasswordPath, []byte("live-leaf-keychain-secret\r\n"))
	if len(fixture.trustAnchor) > 0 {
		if err := os.WriteFile(trustAnchorPath, fixture.trustAnchor, 0o600); err != nil {
			t.Fatalf("write test trust anchor: %v", err)
		}
	}
	resolvedKeychainPath := canonicalSigningKeychainTestPath(t, keychainPath)

	cleanupCtx := context.Background()
	t.Cleanup(func() {
		if err := deps.SetKeychainSearchList(cleanupCtx, before); err != nil {
			t.Errorf("restore keychain search list: %v", err)
		}
		if err := deps.DeleteKeychain(cleanupCtx, resolvedKeychainPath); err != nil {
			t.Errorf("delete disposable keychain: %v", err)
		}
	})

	createKeychain := func(ctx context.Context, path string, password []byte) (bool, error) {
		return createPersistentSigningKeychain(ctx, path, password)
	}
	importIdentity := func(ctx context.Context, path string, keychainPassword, identityData, identityPassword []byte, expectedSHA1 string) error {
		if err := importPKCS12WithSecurityFramework(path, identityData, []byte("incorrect-password")); err == nil {
			return errors.New("native PKCS#12 import accepted an incorrect password")
		}
		if len(fixture.trustAnchor) > 0 {
			_, stderr, err := runSigningUtility(ctx, nil, "import", trustAnchorPath, "-k", path, "-T", "/usr/bin/codesign")
			if err != nil {
				return utilityFailure("install disposable test trust anchor", stderr, err)
			}
		}
		return importPersistentSigningIdentity(ctx, path, keychainPassword, identityData, identityPassword, expectedSHA1)
	}

	result, err := executeSigningKeychainInstallWith(context.Background(), signingKeychainInstallOptions{
		IdentityPath:              identityPath,
		IdentityPasswordPath:      identityPasswordPath,
		KeychainPath:              keychainPath,
		KeychainPasswordPath:      keychainPasswordPath,
		ExpectedCertificateSHA256: identity.CertificateSHA256,
		AddToSearchList:           true,
	}, signingKeychainInstallDeps{
		GOOS:                      "darwin",
		SecurityAvailable:         true,
		Now:                       func() time.Time { return fixture.now },
		AcquireLock:               acquireSigningKeychainTestLock,
		CreateKeychain:            createKeychain,
		ImportIdentity:            importIdentity,
		KeychainSearchList:        keychainSearchList,
		SetKeychainSearchList:     setKeychainSearchList,
		RemoveKeychainSearchEntry: removeKeychainSearchEntry,
		DeleteKeychain:            deleteSigningRunKeychain,
	})
	if err != nil {
		t.Fatalf("install code-signing leaf identity: %v", err)
	}
	if result.Action != "installed" || !result.SearchListUpdated || result.KeychainPath != resolvedKeychainPath {
		t.Fatalf("result = %+v", result)
	}

	installedList, err := keychainSearchList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(installedList, resolvedKeychainPath) {
		t.Fatalf("installed keychain is missing from search list: %v", installedList)
	}
	if err := setKeychainSearchList(context.Background(), before); err != nil {
		t.Fatalf("restore keychain search list after install: %v", err)
	}
	if err := deleteSigningRunKeychain(context.Background(), resolvedKeychainPath); err != nil {
		t.Fatalf("delete disposable keychain after install: %v", err)
	}
	after, err := keychainSearchList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("keychain search list changed: before=%v after=%v", before, after)
	}
}

func TestValidatePersistentSigningCertificateFingerprintsAllowsCertificateChain(t *testing.T) {
	leaf := strings.Repeat("A", 40)
	chain := []string{strings.Repeat("B", 40), leaf, strings.Repeat("C", 40)}
	if err := validatePersistentSigningCertificateFingerprints(chain, strings.ToLower(leaf)); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePersistentSigningCertificateFingerprintsRequiresLeafExactlyOnce(t *testing.T) {
	leaf := strings.Repeat("A", 40)
	for _, certificates := range [][]string{
		{strings.Repeat("B", 40)},
		{leaf, leaf},
	} {
		if err := validatePersistentSigningCertificateFingerprints(certificates, leaf); err == nil {
			t.Fatalf("certificates = %v, want exact-leaf-count failure", certificates)
		}
	}
}
