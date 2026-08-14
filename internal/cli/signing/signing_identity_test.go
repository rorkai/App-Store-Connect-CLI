package signing

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	pkcs12 "github.com/bitrise-io/go-pkcs12"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
	"go.mozilla.org/pkcs7"
	"howett.net/plist"
)

func TestLoadPrivateSigningKeyAcceptsRSAAndEC(t *testing.T) {
	tests := []struct {
		name string
		key  any
	}{
		{name: "RSA", key: mustRSAKey(t)},
		{name: "EC", key: mustECKey(t)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "identity-key.pem")
			der, err := x509.MarshalPKCS8PrivateKey(tt.key)
			if err != nil {
				t.Fatalf("marshal private key: %v", err)
			}
			writePrivateTestFile(t, path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))

			identity, err := loadPrivateSigningKey(path)
			if err != nil {
				t.Fatalf("loadPrivateSigningKey() error = %v", err)
			}
			if identity.PrivateKey == nil {
				t.Fatal("identity has no private key")
			}
		})
	}
}

func TestLoadPrivateSigningKeyRejectsSymlinkAndPermissiveMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation and POSIX permission bits are not portable to Windows")
	}
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key.pem")
	der, err := x509.MarshalPKCS8PrivateKey(mustECKey(t))
	if err != nil {
		t.Fatal(err)
	}
	writePrivateTestFile(t, keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))

	symlink := filepath.Join(dir, "linked.pem")
	if err := os.Symlink(keyPath, symlink); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPrivateSigningKey(symlink); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink error = %v", err)
	}

	if err := os.Chmod(keyPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPrivateSigningKey(keyPath); err == nil || !strings.Contains(err.Error(), "permissions") {
		t.Fatalf("permission error = %v", err)
	}
}

func TestLoadPKCS12IdentityRoundTripAndFingerprintSelection(t *testing.T) {
	firstKey := mustECKey(t)
	firstCert := mustSigningCertificate(t, firstKey, 1)
	secondKey := mustRSAKey(t)
	secondCert := mustSigningCertificate(t, secondKey, 2)
	firstP12, err := pkcs12.Encode(rand.Reader, firstKey, firstCert, nil, "source-password")
	if err != nil {
		t.Fatalf("encode first identity: %v", err)
	}

	path := filepath.Join(t.TempDir(), "identity.p12")
	writePrivateTestFile(t, path, firstP12)
	identity, err := loadPKCS12Identity(path, "source-password", "")
	if err != nil {
		t.Fatalf("loadPKCS12Identity() error = %v", err)
	}
	wantFingerprint := certificateSHA256(firstCert)
	if identity.CertificateSHA256 != wantFingerprint {
		t.Fatalf("fingerprint = %q, want %q", identity.CertificateSHA256, wantFingerprint)
	}

	candidates := []signingIdentity{
		{PrivateKey: firstKey, Certificate: firstCert, CertificateSHA256: certificateSHA256(firstCert)},
		{PrivateKey: secondKey, Certificate: secondCert, CertificateSHA256: certificateSHA256(secondCert)},
	}
	if _, err := selectSigningIdentity(candidates, ""); err == nil || !strings.Contains(err.Error(), "--identity-sha256") {
		t.Fatalf("multi-identity error = %v", err)
	}
	selected, err := selectSigningIdentity(candidates, certificateSHA256(secondCert))
	if err != nil {
		t.Fatalf("selectSigningIdentity() error = %v", err)
	}
	if selected.CertificateSHA256 != certificateSHA256(secondCert) {
		t.Fatalf("selected fingerprint = %q", selected.CertificateSHA256)
	}
}

func TestNormalizeSigningIdentityProducesOnePasswordProtectedIdentity(t *testing.T) {
	key := mustRSAKey(t)
	cert := mustSigningCertificate(t, key, 3)
	identity := &signingIdentity{PrivateKey: key, Certificate: cert, CertificateSHA256: certificateSHA256(cert)}

	normalized, err := normalizeSigningIdentity(identity, "new-password")
	if err != nil {
		t.Fatalf("normalizeSigningIdentity() error = %v", err)
	}
	decodedKey, decodedCert, err := pkcs12.Decode(normalized, "new-password")
	if err != nil {
		t.Fatalf("decode normalized identity: %v", err)
	}
	if !publicKeysEqual(decodedKey, cert.PublicKey) || !decodedCert.Equal(cert) {
		t.Fatal("normalized identity does not contain the selected key/certificate pair")
	}
	if _, _, err := pkcs12.Decode(normalized, "wrong-password"); err == nil {
		t.Fatal("normalized identity was not password protected")
	}
}

func TestNormalizeSigningIdentityUsesModernPKCS12Algorithms(t *testing.T) {
	key := mustECKey(t)
	cert := mustSigningCertificate(t, key, 15)
	normalized, err := normalizeSigningIdentity(&signingIdentity{PrivateKey: key, Certificate: cert}, "high-entropy-repository-password")
	if err != nil {
		t.Fatal(err)
	}
	for name, oid := range map[string][]byte{
		"PBES2":       {0x06, 0x09, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0x0d, 0x01, 0x05, 0x0d},
		"PBKDF2":      {0x06, 0x09, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0x0d, 0x01, 0x05, 0x0c},
		"AES-256-CBC": {0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x01, 0x2a},
		"SHA-256":     {0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x02, 0x01},
	} {
		if !bytes.Contains(normalized, oid) {
			t.Errorf("normalized PKCS#12 does not declare %s", name)
		}
	}
}

func TestValidateIdentityForResolvedAssetsChecksProfileCertificateTeamAndBundle(t *testing.T) {
	key := mustECKey(t)
	cert := mustSigningCertificate(t, key, 4)
	profilePlist := mustSignedProfile(t, cert, key, "TEAM123", "TEAM123.com.example.app", time.Now().Add(24*time.Hour))
	profile := &asc.ProfileResponse{Data: asc.Resource[asc.ProfileAttributes]{
		ID: "profile-1",
		Attributes: asc.ProfileAttributes{
			ProfileContent: base64.StdEncoding.EncodeToString(profilePlist),
			ProfileType:    "IOS_APP_ADHOC",
			ProfileState:   asc.ProfileStateActive,
		},
	}}
	certificates := &asc.CertificatesResponse{Data: []asc.Resource[asc.CertificateAttributes]{
		identityCertificateResource(cert),
	}}
	identity := &signingIdentity{PrivateKey: key}

	if err := validateIdentityForResolvedAssets(identity, profile, certificates, "com.example.app", "IOS_APP_ADHOC", time.Now()); err != nil {
		t.Fatalf("validateIdentityForResolvedAssets() error = %v", err)
	}
	if identity.CertificateSHA256 != certificateSHA256(cert) {
		t.Fatalf("identity fingerprint = %q", identity.CertificateSHA256)
	}

	profile.Data.Attributes.ProfileContent = base64.StdEncoding.EncodeToString(mustSignedProfile(t, cert, key, "OTHERTEAM", "OTHERTEAM.com.example.app", time.Now().Add(24*time.Hour)))
	identity = &signingIdentity{PrivateKey: key}
	if err := validateIdentityForResolvedAssets(identity, profile, certificates, "com.example.app", "IOS_APP_ADHOC", time.Now()); err == nil || !strings.Contains(err.Error(), "team") {
		t.Fatalf("team mismatch error = %v", err)
	}
}

func TestValidateIdentityForResolvedAssetsRequiresExactBundleAndProfileType(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 9)
	certificates := &asc.CertificatesResponse{Data: []asc.Resource[asc.CertificateAttributes]{
		identityCertificateResource(certificate),
	}}
	tests := []struct {
		name              string
		applicationID     string
		profileType       string
		wantErrorContains string
	}{
		{name: "missing application identifier", applicationID: "", profileType: "IOS_APP_ADHOC", wantErrorContains: "application identifier"},
		{name: "wildcard application identifier", applicationID: "TEAM123.*", profileType: "IOS_APP_ADHOC", wantErrorContains: "bundle identifier"},
		{name: "different bundle", applicationID: "TEAM123.com.other.app", profileType: "IOS_APP_ADHOC", wantErrorContains: "bundle identifier"},
		{name: "missing profile type", applicationID: "TEAM123.com.example.app", profileType: "", wantErrorContains: "profile type"},
		{name: "different profile type", applicationID: "TEAM123.com.example.app", profileType: "IOS_APP_STORE", wantErrorContains: "profile type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := &asc.ProfileResponse{Data: asc.Resource[asc.ProfileAttributes]{
				ID: "profile-1",
				Attributes: asc.ProfileAttributes{
					ProfileContent: base64.StdEncoding.EncodeToString(mustSignedProfile(t, certificate, key, "TEAM123", tt.applicationID, time.Now().Add(time.Hour))),
					ProfileType:    tt.profileType,
					ProfileState:   asc.ProfileStateActive,
				},
			}}
			identity := &signingIdentity{PrivateKey: key}
			err := validateIdentityForResolvedAssets(identity, profile, certificates, "com.example.app", "IOS_APP_ADHOC", time.Now())
			if err == nil || !strings.Contains(err.Error(), tt.wantErrorContains) {
				t.Fatalf("error = %v, want %q", err, tt.wantErrorContains)
			}
		})
	}
}

func TestValidateIdentityForResolvedAssetsRequiresActiveProfileState(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 23)
	profile := &asc.ProfileResponse{Data: asc.Resource[asc.ProfileAttributes]{Attributes: asc.ProfileAttributes{
		ProfileContent: base64.StdEncoding.EncodeToString(mustSignedProfile(t, certificate, key, "TEAM123", "TEAM123.com.example.app", time.Now().Add(time.Hour))),
		ProfileType:    "IOS_APP_ADHOC",
	}}}
	certificates := &asc.CertificatesResponse{Data: []asc.Resource[asc.CertificateAttributes]{identityCertificateResource(certificate)}}
	if err := validateIdentityForResolvedAssets(&signingIdentity{PrivateKey: key}, profile, certificates, "com.example.app", "IOS_APP_ADHOC", time.Now()); err == nil || !strings.Contains(err.Error(), "not active") {
		t.Fatalf("inactive profile state error = %v", err)
	}
}

func TestValidateIdentityForResolvedAssetsUsesApplicationIdentifierPrefixNotTeamID(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 14)
	profilePlist, err := plist.Marshal(map[string]any{
		"TeamIdentifier":              []string{"TEAM123"},
		"ApplicationIdentifierPrefix": []string{"SEED456"},
		"ExpirationDate":              time.Now().Add(time.Hour),
		"DeveloperCertificates":       [][]byte{certificate.Raw},
		"Entitlements": map[string]any{
			"application-identifier": "SEED456.com.example.app",
			"get-task-allow":         false,
		},
		"ProvisionedDevices": []string{"DEVICE1"},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	profile := &asc.ProfileResponse{Data: asc.Resource[asc.ProfileAttributes]{
		Attributes: asc.ProfileAttributes{
			ProfileContent: base64.StdEncoding.EncodeToString(mustSignedCMS(t, profilePlist, certificate, key)),
			ProfileType:    "IOS_APP_ADHOC",
			ProfileState:   asc.ProfileStateActive,
		},
	}}
	certificates := &asc.CertificatesResponse{Data: []asc.Resource[asc.CertificateAttributes]{
		identityCertificateResource(certificate),
	}}
	if err := validateIdentityForResolvedAssets(&signingIdentity{PrivateKey: key}, profile, certificates, "com.example.app", "IOS_APP_ADHOC", time.Now()); err != nil {
		t.Fatalf("legacy application identifier prefix rejected: %v", err)
	}
}

func TestValidateIdentityForResolvedAssetsAcceptsMatchingSecondApplicationPrefix(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 25)
	profilePlist, err := plist.Marshal(map[string]any{
		"TeamIdentifier": []string{"TEAM123"}, "ApplicationIdentifierPrefix": []string{"OLD111", "SEED456"},
		"ExpirationDate": time.Now().Add(time.Hour), "DeveloperCertificates": [][]byte{certificate.Raw},
		"Entitlements":       map[string]any{"application-identifier": "SEED456.com.example.app", "get-task-allow": false},
		"ProvisionedDevices": []string{"DEVICE1"},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	profile := &asc.ProfileResponse{Data: asc.Resource[asc.ProfileAttributes]{Attributes: asc.ProfileAttributes{
		ProfileContent: base64.StdEncoding.EncodeToString(mustSignedCMS(t, profilePlist, certificate, key)), ProfileType: "IOS_APP_ADHOC", ProfileState: asc.ProfileStateActive,
	}}}
	certificates := &asc.CertificatesResponse{Data: []asc.Resource[asc.CertificateAttributes]{identityCertificateResource(certificate)}}
	if err := validateIdentityForResolvedAssets(&signingIdentity{PrivateKey: key}, profile, certificates, "com.example.app", "IOS_APP_ADHOC", time.Now()); err != nil {
		t.Fatalf("matching second application prefix rejected: %v", err)
	}
}

func TestValidateIdentityForResolvedAssetsRejectsSignedProfileDistributionTypeMismatch(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 36)
	profilePlist, err := plist.Marshal(map[string]any{
		"TeamIdentifier":              []string{"TEAM123"},
		"ApplicationIdentifierPrefix": []string{"TEAM123"},
		"ExpirationDate":              time.Now().Add(time.Hour),
		"DeveloperCertificates":       [][]byte{certificate.Raw},
		"Entitlements": map[string]any{
			"application-identifier": "TEAM123.com.example.app",
			"get-task-allow":         false,
		},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	profile := &asc.ProfileResponse{Data: asc.Resource[asc.ProfileAttributes]{Attributes: asc.ProfileAttributes{
		ProfileContent: base64.StdEncoding.EncodeToString(mustSignedCMS(t, profilePlist, certificate, key)),
		ProfileType:    "IOS_APP_ADHOC",
		ProfileState:   asc.ProfileStateActive,
	}}}
	certificates := &asc.CertificatesResponse{Data: []asc.Resource[asc.CertificateAttributes]{identityCertificateResource(certificate)}}
	err = validateIdentityForResolvedAssets(&signingIdentity{PrivateKey: key}, profile, certificates, "com.example.app", "IOS_APP_ADHOC", time.Now())
	if err == nil || !strings.Contains(err.Error(), "distribution type") {
		t.Fatalf("signed profile distribution mismatch error = %v", err)
	}
}

func TestReadProtectedSecretFileRejectsOversize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversize-secret")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxProtectedSecretFileSize + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := readProtectedSecretFile(path, "private key"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversize error = %v", err)
	}
}

func TestParseIdentityMobileProvisionRequiresValidCMSSignature(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 6)
	unsigned := mustProfilePlist(t, certificate, "TEAM123", "TEAM123.com.example.app", time.Now().Add(time.Hour))
	if _, err := parseIdentityMobileProvision(unsigned); err == nil || !strings.Contains(err.Error(), "signed CMS") {
		t.Fatalf("unsigned profile error = %v", err)
	}

	signed := mustSignedCMS(t, unsigned, certificate, key)
	tampered := append([]byte(nil), signed...)
	tampered[len(tampered)-1] ^= 0xff
	if _, err := parseIdentityMobileProvision(tampered); err == nil {
		t.Fatal("tampered CMS profile was accepted")
	}
}

func TestWriteOrReuseSigningIdentityIsIdempotentAndRejectsReplacement(t *testing.T) {
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 7)
	identity := &signingIdentity{PrivateKey: key, Certificate: certificate, CertificateSHA256: certificateSHA256(certificate)}
	first, err := normalizeSigningIdentity(identity, "repository-password")
	if err != nil {
		t.Fatal(err)
	}
	second, err := normalizeSigningIdentity(identity, "repository-password")
	if err != nil {
		t.Fatal(err)
	}
	relPath := filepath.Join("identities", "distribution", identity.CertificateSHA256+".p12")
	metadata := signingpkg.EncryptedFileMetadata{
		Kind:              "pkcs12-identity",
		Sensitive:         true,
		CertificateSHA256: identity.CertificateSHA256,
		TeamID:            "TEAM123",
	}
	if err := writeOrReuseSigningIdentity(store, relPath, first, "repository-password", metadata); err != nil {
		t.Fatalf("first write error = %v", err)
	}
	if err := writeOrReuseSigningIdentity(store, relPath, second, "repository-password", metadata); err != nil {
		t.Fatalf("idempotent write error = %v", err)
	}

	replacementKey := mustECKey(t)
	replacementCertificate := mustSigningCertificate(t, replacementKey, 8)
	replacement, err := normalizeSigningIdentity(&signingIdentity{PrivateKey: replacementKey, Certificate: replacementCertificate}, "repository-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeOrReuseSigningIdentity(store, relPath, replacement, "repository-password", metadata); err == nil || !strings.Contains(err.Error(), "explicitly migrate") {
		t.Fatalf("replacement error = %v", err)
	}
}

func TestSameIdentityMetadataCanonicalizesWindowsAndSlashPaths(t *testing.T) {
	existing := signingpkg.EncryptedFileMetadata{
		Version: 1, Kind: "pkcs12-identity", Sensitive: true,
		RelativePath: `identities/distribution/ABC.p12`, CertificateSHA256: "ABC", TeamID: "TEAM123",
	}
	wanted := existing
	wanted.RelativePath = `identities\distribution\ABC.p12`
	if !sameIdentityMetadata(existing, wanted) {
		t.Fatalf("cross-platform metadata paths differ: existing=%#v wanted=%#v", existing, wanted)
	}
}

func TestSigningIdentityArtifactsReuseOneIdentityAcrossContexts(t *testing.T) {
	store := &signingpkg.GitStore{LocalDir: t.TempDir()}
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 17)
	identity := &signingIdentity{PrivateKey: key, Certificate: certificate, CertificateSHA256: certificateSHA256(certificate)}

	first, err := prepareSigningIdentityArtifacts(identity, "repository-password", "com.example.first", "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	bindTestSigningIdentityArtifacts(t, first, certificate, key, "com.example.first", "IOS_APP_ADHOC", "profile-first")
	if err := writeOrReuseSigningIdentityArtifacts(store, first, "repository-password"); err != nil {
		t.Fatal(err)
	}
	second, err := prepareSigningIdentityArtifacts(identity, "repository-password", "com.example.second", "IOS_APP_STORE")
	if err != nil {
		t.Fatal(err)
	}
	bindTestSigningIdentityArtifacts(t, second, certificate, key, "com.example.second", "IOS_APP_STORE", "profile-second")
	if first.IdentityPath != second.IdentityPath || first.BindingPath == second.BindingPath {
		t.Fatalf("first=%#v second=%#v", first, second)
	}
	if err := writeOrReuseSigningIdentityArtifacts(store, second, "repository-password"); err != nil {
		t.Fatalf("cross-context reuse failed: %v", err)
	}
	if err := writeOrReuseSigningIdentityArtifacts(store, second, "repository-password"); err != nil {
		t.Fatalf("idempotent cross-context reuse failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.LocalDir, first.IdentityPath+".enc")); err != nil {
		t.Fatal(err)
	}
	for _, binding := range []string{first.BindingPath, second.BindingPath} {
		if _, err := os.Stat(filepath.Join(store.LocalDir, binding+".enc")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPrepareSigningIdentityArtifactsUsesDevelopmentCategory(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 18)
	identity := &signingIdentity{PrivateKey: key, Certificate: certificate, CertificateSHA256: certificateSHA256(certificate)}
	artifacts, err := prepareSigningIdentityArtifacts(identity, "repository-password", "com.example.app", "IOS_APP_DEVELOPMENT")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(filepath.ToSlash(artifacts.IdentityPath), "identities/development/") {
		t.Fatalf("identity path = %q", artifacts.IdentityPath)
	}
}

func TestBindSigningIdentityProfileRejectsAPIAndSignedUUIDMismatch(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 26)
	identity := &signingIdentity{PrivateKey: key, Certificate: certificate, CertificateSHA256: certificateSHA256(certificate)}
	artifacts, err := prepareSigningIdentityArtifacts(identity, "password", "com.example.app", "IOS_APP_ADHOC")
	if err != nil {
		t.Fatal(err)
	}
	profilePlist, err := plist.Marshal(map[string]any{
		"UUID": "SIGNED-UUID", "TeamIdentifier": []string{"TEAM123"}, "ApplicationIdentifierPrefix": []string{"TEAM123"},
		"ExpirationDate": time.Now().Add(time.Hour), "DeveloperCertificates": [][]byte{certificate.Raw},
		"ProvisionedDevices": []string{"DEVICE1"}, "Entitlements": map[string]any{"application-identifier": "TEAM123.com.example.app", "get-task-allow": false},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	profileContent := mustSignedCMS(t, profilePlist, certificate, key)
	profile := &asc.ProfileResponse{Data: asc.Resource[asc.ProfileAttributes]{ID: "resource-id", Attributes: asc.ProfileAttributes{UUID: "API-UUID"}}}
	if err := bindSigningIdentityProfile(artifacts, profile, "profiles/adhoc/profile.mobileprovision", profileContent); err == nil || !strings.Contains(err.Error(), "UUID") {
		t.Fatalf("UUID mismatch error = %v", err)
	}
}

func TestIdentityCertificateFilterRejectsMismatchedPrivateKey(t *testing.T) {
	firstKey := mustECKey(t)
	secondKey := mustECKey(t)
	certificate := mustSigningCertificate(t, firstKey, 5)
	active := true
	resource := asc.Resource[asc.CertificateAttributes]{Attributes: asc.CertificateAttributes{
		CertificateContent: base64.StdEncoding.EncodeToString(certificate.Raw), Activated: &active, ExpirationDate: time.Now().Add(time.Hour).Format(time.RFC3339),
	}}
	if identityCertificateFilter(&signingIdentity{PrivateKey: firstKey})(resource) != true {
		t.Fatal("matching private key was rejected")
	}
	if identityCertificateFilter(&signingIdentity{PrivateKey: secondKey})(resource) != false {
		t.Fatal("mismatched private key was accepted")
	}
}

func TestIdentityCertificateFilterRejectsInactiveOrAPIExpiredCertificate(t *testing.T) {
	key := mustECKey(t)
	certificate := mustSigningCertificate(t, key, 16)
	active := true
	inactive := false
	base := asc.CertificateAttributes{CertificateContent: base64.StdEncoding.EncodeToString(certificate.Raw), Activated: &active, ExpirationDate: time.Now().Add(time.Hour).Format(time.RFC3339)}
	filter := identityCertificateFilter(&signingIdentity{PrivateKey: key})
	if !filter(asc.Resource[asc.CertificateAttributes]{Attributes: base}) {
		t.Fatal("active unexpired certificate rejected")
	}
	base.Activated = nil
	if !filter(asc.Resource[asc.CertificateAttributes]{Attributes: base}) {
		t.Fatal("unexpired certificate with omitted activated state rejected")
	}
	base.Activated = &inactive
	if filter(asc.Resource[asc.CertificateAttributes]{Attributes: base}) {
		t.Fatal("inactive certificate accepted")
	}
	base.Activated = &active
	base.ExpirationDate = time.Now().Add(-time.Hour).Format(time.RFC3339)
	if filter(asc.Resource[asc.CertificateAttributes]{Attributes: base}) {
		t.Fatal("API-expired certificate accepted")
	}
}

func TestIdentityCertificateFilterFallsBackToEncodedValidity(t *testing.T) {
	key := mustECKey(t)
	now := time.Now()
	current := mustSigningCertificateWithValidity(t, key, 30, now.Add(-time.Hour), now.Add(time.Hour))
	expired := mustSigningCertificateWithValidity(t, key, 31, now.Add(-2*time.Hour), now.Add(-time.Hour))
	notYetValid := mustSigningCertificateWithValidity(t, key, 32, now.Add(time.Hour), now.Add(2*time.Hour))
	filter := identityCertificateFilter(&signingIdentity{PrivateKey: key})

	resource := func(content string) asc.Resource[asc.CertificateAttributes] {
		return asc.Resource[asc.CertificateAttributes]{Attributes: asc.CertificateAttributes{
			CertificateContent: content,
		}}
	}

	if !filter(resource(base64.StdEncoding.EncodeToString(current.Raw))) {
		t.Fatal("current certificate with omitted expirationDate rejected")
	}
	if filter(resource(base64.StdEncoding.EncodeToString(expired.Raw))) {
		t.Fatal("expired encoded certificate accepted")
	}
	if filter(resource(base64.StdEncoding.EncodeToString(notYetValid.Raw))) {
		t.Fatal("not-yet-valid encoded certificate accepted")
	}
	if filter(resource(base64.StdEncoding.EncodeToString([]byte("not DER")))) {
		t.Fatal("malformed encoded certificate accepted")
	}
}

func TestIdentityCertificateFilterUsesFingerprintToDisambiguateSameKey(t *testing.T) {
	key := mustECKey(t)
	first := mustSigningCertificate(t, key, 21)
	second := mustSigningCertificate(t, key, 22)
	active := true
	resource := func(certificate *x509.Certificate) asc.Resource[asc.CertificateAttributes] {
		return asc.Resource[asc.CertificateAttributes]{Attributes: asc.CertificateAttributes{
			CertificateContent: base64.StdEncoding.EncodeToString(certificate.Raw), Activated: &active, ExpirationDate: time.Now().Add(time.Hour).Format(time.RFC3339),
		}}
	}
	filter := identityCertificateFilter(&signingIdentity{PrivateKey: key, RequestedSHA256: certificateSHA256(second)})
	if filter(resource(first)) || !filter(resource(second)) {
		t.Fatal("fingerprint did not deterministically select the requested same-key certificate")
	}
}

func TestLoadPrivateSigningKeyRejectsAmbiguousOrTrailingData(t *testing.T) {
	key := mustECKey(t)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	block := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	for name, data := range map[string][]byte{
		"multiple keys":    append(append([]byte(nil), block...), block...),
		"trailing garbage": append(append([]byte(nil), block...), []byte("not pem")...),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "key.pem")
			writePrivateTestFile(t, path, data)
			if _, err := loadPrivateSigningKey(path); err == nil || !strings.Contains(err.Error(), "exactly one") {
				t.Fatalf("loadPrivateSigningKey() error = %v", err)
			}
		})
	}
}

func TestPKCS12DecodeErrorDoesNotLeakPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.p12")
	writePrivateTestFile(t, path, []byte("not a PKCS#12"))
	secret := "CANARY-IDENTITY-PASSWORD"
	_, err := loadPKCS12Identity(path, secret, "")
	if err == nil {
		t.Fatal("expected decode error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked password: %v", err)
	}
}

func identityCertificateResource(certificate *x509.Certificate) asc.Resource[asc.CertificateAttributes] {
	return asc.Resource[asc.CertificateAttributes]{ID: "cert-1", Attributes: asc.CertificateAttributes{
		CertificateContent: base64.StdEncoding.EncodeToString(certificate.Raw),
		ExpirationDate:     certificate.NotAfter.Format(time.RFC3339),
	}}
}

func bindTestSigningIdentityArtifacts(t *testing.T, artifacts *signingIdentityArtifacts, certificate *x509.Certificate, privateKey any, bundleID, profileType, profileID string) (string, []byte) {
	t.Helper()
	return bindTestSigningIdentityArtifactsWithSigner(t, artifacts, certificate, certificate, privateKey, bundleID, profileType, profileID)
}

func bindTestSigningIdentityArtifactsWithSigner(t *testing.T, artifacts *signingIdentityArtifacts, embeddedCertificate, signerCertificate *x509.Certificate, signerPrivateKey any, bundleID, profileType, profileID string) (string, []byte) {
	t.Helper()
	profilePlist, err := plist.Marshal(map[string]any{
		"UUID": profileID, "TeamIdentifier": []string{"TEAM123"}, "ApplicationIdentifierPrefix": []string{"TEAM123"},
		"ExpirationDate": time.Now().Add(time.Hour), "DeveloperCertificates": [][]byte{embeddedCertificate.Raw},
		"ProvisionedDevices": []string{"DEVICE1"},
		"Entitlements":       map[string]any{"application-identifier": "TEAM123." + bundleID, "get-task-allow": false},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	profileContent := mustSignedCMS(t, profilePlist, signerCertificate, signerPrivateKey)
	profile := &asc.ProfileResponse{Data: asc.Resource[asc.ProfileAttributes]{ID: profileID, Attributes: asc.ProfileAttributes{Name: profileID, UUID: profileID}}}
	profilePath := filepath.Join("profiles", profileDirectoryName(profileType), profileID+".mobileprovision")
	if err := bindSigningIdentityProfile(artifacts, profile, profilePath, profileContent); err != nil {
		t.Fatal(err)
	}
	return profilePath, profileContent
}

func mustProfilePlist(t *testing.T, certificate *x509.Certificate, teamID, applicationIdentifier string, expiration time.Time) []byte {
	t.Helper()
	data, err := plist.Marshal(map[string]any{
		"UUID":                        "profile-1",
		"TeamIdentifier":              []string{teamID},
		"ApplicationIdentifierPrefix": []string{teamID},
		"ExpirationDate":              expiration,
		"DeveloperCertificates":       [][]byte{certificate.Raw},
		"Entitlements": map[string]any{
			"application-identifier": applicationIdentifier,
			"get-task-allow":         false,
		},
		"ProvisionedDevices": []string{"DEVICE1"},
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustSignedProfile(t *testing.T, certificate *x509.Certificate, privateKey any, teamID, applicationIdentifier string, expiration time.Time) []byte {
	t.Helper()
	return mustSignedCMS(t, mustProfilePlist(t, certificate, teamID, applicationIdentifier, expiration), certificate, privateKey)
}

func mustSignedCMS(t *testing.T, content []byte, certificate *x509.Certificate, privateKey any) []byte {
	t.Helper()
	signed, err := pkcs7.NewSignedData(content)
	if err != nil {
		t.Fatal(err)
	}
	if err := signed.AddSigner(certificate, privateKey, pkcs7.SignerInfoConfig{}); err != nil {
		t.Fatal(err)
	}
	data, err := signed.Finish()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func mustECKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func mustSigningCertificate(t *testing.T, privateKey any, serial int64) *x509.Certificate {
	t.Helper()
	return mustSigningCertificateWithValidity(t, privateKey, serial, time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
}

func mustSigningCertificateWithValidity(t *testing.T, privateKey any, serial int64, notBefore, notAfter time.Time) *x509.Certificate {
	t.Helper()
	publicKey, err := signingPublicKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject: pkix.Name{
			CommonName:         "Apple Distribution: Test",
			OrganizationalUnit: []string{"TEAM123"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func certificateSHA256(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func writePrivateTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
