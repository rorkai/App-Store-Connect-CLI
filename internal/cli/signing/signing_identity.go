package signing

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	pkcs12 "github.com/bitrise-io/go-pkcs12"
	"github.com/google/uuid"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	signingpkg "github.com/rudrankriyam/App-Store-Connect-CLI/internal/signing"
	"go.mozilla.org/pkcs7"
	"howett.net/plist"
	modernpkcs12 "software.sslmate.com/src/go-pkcs12"
)

type signingIdentity struct {
	PrivateKey        any
	Certificate       *x509.Certificate
	CertificateSHA256 string
	RequestedSHA256   string
}

type signingIdentityArtifacts struct {
	IdentityPath     string
	IdentityData     []byte
	IdentityMetadata signingpkg.EncryptedFileMetadata
	BindingPath      string
	BindingData      []byte
	BindingMetadata  signingpkg.EncryptedFileMetadata
}

type identityContextBinding struct {
	CertificateSHA256 string `json:"certificateSha256"`
	TeamID            string `json:"teamId"`
	BundleID          string `json:"bundleId"`
	ProfileType       string `json:"profileType"`
	ProfileResourceID string `json:"profileResourceId"`
	ProfileUUID       string `json:"profileUuid"`
	ProfilePath       string `json:"profilePath"`
	ProfileSHA256     string `json:"profileSha256"`
}

const maxProtectedSecretFileSize int64 = 32 << 20

func loadPrivateSigningKey(path string) (*signingIdentity, error) {
	data, err := readProtectedSecretFile(path, "private key")
	if err != nil {
		return nil, err
	}
	block, rest := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("private key is not PEM encoded")
	}
	if next, trailing := pem.Decode(rest); next != nil || len(bytes.TrimSpace(trailing)) != 0 {
		return nil, fmt.Errorf("private key file must contain exactly one private key PEM block and no other data")
	}
	if len(bytes.TrimSpace(rest)) != 0 {
		return nil, fmt.Errorf("private key file must contain exactly one private key PEM block and no other data")
	}
	key, err := parseSigningPrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return &signingIdentity{PrivateKey: key}, nil
}

func parseSigningPrivateKey(der []byte) (any, error) {
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		if err := validateSigningPrivateKeyType(key); err != nil {
			return nil, err
		}
		return key, nil
	}
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("private key must be an RSA or EC private key")
}

func validateSigningPrivateKeyType(key any) error {
	switch key.(type) {
	case *rsa.PrivateKey, *ecdsa.PrivateKey:
		return nil
	default:
		return fmt.Errorf("private key must be RSA or EC")
	}
}

func loadPKCS12Identity(path, password, fingerprint string) (*signingIdentity, error) {
	data, err := readProtectedSecretFile(path, "PKCS#12 identity")
	if err != nil {
		return nil, err
	}
	privateKeys, certificates, err := pkcs12.DecodeAll(data, password)
	if err != nil {
		return nil, fmt.Errorf("decode PKCS#12 identity: password or file is invalid")
	}
	candidates := make([]signingIdentity, 0, len(privateKeys))
	for _, privateKey := range privateKeys {
		if err := validateSigningPrivateKeyType(privateKey); err != nil {
			continue
		}
		for _, certificate := range certificates {
			if publicKeysEqual(privateKey, certificate.PublicKey) {
				candidates = append(candidates, signingIdentity{
					PrivateKey:        privateKey,
					Certificate:       certificate,
					CertificateSHA256: signingCertificateSHA256(certificate),
				})
			}
		}
	}
	return selectSigningIdentity(candidates, fingerprint)
}

func selectSigningIdentity(candidates []signingIdentity, fingerprint string) (*signingIdentity, error) {
	requested, err := normalizeCertificateFingerprint(fingerprint)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("PKCS#12 contains no usable RSA or EC private identity")
	}
	if requested == "" {
		if len(candidates) > 1 {
			return nil, fmt.Errorf("PKCS#12 contains multiple private identities; use --identity-sha256 to select one")
		}
		selected := candidates[0]
		return &selected, nil
	}
	for _, candidate := range candidates {
		if strings.EqualFold(candidate.CertificateSHA256, requested) {
			selected := candidate
			return &selected, nil
		}
	}
	return nil, fmt.Errorf("no PKCS#12 private identity matches --identity-sha256")
}

func normalizeCertificateFingerprint(raw string) (string, error) {
	normalized := strings.ToUpper(strings.NewReplacer(":", "", " ", "", "-", "").Replace(strings.TrimSpace(raw)))
	if normalized == "" {
		return "", nil
	}
	decoded, err := hex.DecodeString(normalized)
	if err != nil || len(decoded) != sha256.Size {
		return "", fmt.Errorf("--identity-sha256 must be a 64-character SHA-256 certificate fingerprint")
	}
	return normalized, nil
}

func normalizeSigningIdentity(identity *signingIdentity, password string) ([]byte, error) {
	if identity == nil || identity.PrivateKey == nil || identity.Certificate == nil {
		return nil, fmt.Errorf("signing identity is incomplete")
	}
	if password == "" {
		return nil, fmt.Errorf("repository encryption password is empty")
	}
	if !publicKeysEqual(identity.PrivateKey, identity.Certificate.PublicKey) {
		return nil, fmt.Errorf("private key does not match signing certificate")
	}
	encoded, err := modernpkcs12.Modern2023.WithRand(cryptorand.Reader).Encode(identity.PrivateKey, identity.Certificate, nil, password)
	if err != nil {
		return nil, fmt.Errorf("normalize signing identity: %w", err)
	}
	return encoded, nil
}

func signingPublicKey(privateKey any) (any, error) {
	signer, ok := privateKey.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("private key does not expose a public key")
	}
	return signer.Public(), nil
}

func publicKeysEqual(privateKey, publicKey any) bool {
	derived, err := signingPublicKey(privateKey)
	if err != nil {
		return false
	}
	derivedDER, err := x509.MarshalPKIXPublicKey(derived)
	if err != nil {
		return false
	}
	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return false
	}
	return bytes.Equal(derivedDER, publicDER)
}

func signingCertificateSHA256(certificate *x509.Certificate) string {
	if certificate == nil {
		return ""
	}
	sum := sha256.Sum256(certificate.Raw)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func readProtectedSecretFile(path, label string) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("%s path is empty", label)
	}
	file, err := shared.OpenExistingNoFollow(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s file does not exist", label)
		}
		return nil, fmt.Errorf("open %s file without following symlinks: %w", label, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect %s file: %w", label, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s file must be regular", label)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%s file permissions must be 0600 or more restrictive", label)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxProtectedSecretFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("read %s file: %w", label, err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%s file is empty", label)
	}
	if int64(len(data)) > maxProtectedSecretFileSize {
		return nil, fmt.Errorf("%s file exceeds the 32 MiB size limit", label)
	}
	return data, nil
}

func validateIdentityCertificate(identity *signingIdentity, certificateDER []byte, now time.Time) error {
	certificate, err := x509.ParseCertificate(certificateDER)
	if err != nil {
		return fmt.Errorf("invalid certificate returned by App Store Connect")
	}
	if now.Before(certificate.NotBefore) || !now.Before(certificate.NotAfter) {
		return fmt.Errorf("matching signing certificate is not currently valid")
	}
	if !publicKeysEqual(identity.PrivateKey, certificate.PublicKey) {
		return fmt.Errorf("private key does not match the selected profile certificate")
	}
	if identity.Certificate != nil && !identity.Certificate.Equal(certificate) {
		return fmt.Errorf("PKCS#12 certificate does not match the selected profile certificate")
	}
	identity.Certificate = certificate
	identity.CertificateSHA256 = signingCertificateSHA256(certificate)
	return nil
}

func identityCertificateFilter(identity *signingIdentity) func(asc.Resource[asc.CertificateAttributes]) bool {
	if identity == nil {
		return nil
	}
	return func(certificate asc.Resource[asc.CertificateAttributes]) bool {
		if !usableIdentityCertificateResource(certificate, time.Now()) {
			return false
		}
		der, err := base64.StdEncoding.DecodeString(strings.TrimSpace(certificate.Attributes.CertificateContent))
		if err != nil || len(der) == 0 {
			return false
		}
		parsed, err := x509.ParseCertificate(der)
		if err != nil || !publicKeysEqual(identity.PrivateKey, parsed.PublicKey) {
			return false
		}
		if identity.RequestedSHA256 != "" && !strings.EqualFold(signingCertificateSHA256(parsed), identity.RequestedSHA256) {
			return false
		}
		return identity.Certificate == nil || identity.Certificate.Equal(parsed)
	}
}

type identityMobileProvision struct {
	TeamIdentifier              []string       `plist:"TeamIdentifier"`
	ApplicationIdentifierPrefix []string       `plist:"ApplicationIdentifierPrefix"`
	ExpirationDate              time.Time      `plist:"ExpirationDate"`
	DeveloperCertificates       [][]byte       `plist:"DeveloperCertificates"`
	Entitlements                map[string]any `plist:"Entitlements"`
	UUID                        string         `plist:"UUID"`
	ProvisionedDevices          []string       `plist:"ProvisionedDevices"`
	ProvisionsAllDevices        bool           `plist:"ProvisionsAllDevices"`
}

func validateIdentityForResolvedAssets(identity *signingIdentity, profile *asc.ProfileResponse, certificates *asc.CertificatesResponse, bundleID, profileType string, now time.Time) error {
	if identity == nil || profile == nil || certificates == nil {
		return fmt.Errorf("resolved signing assets are incomplete")
	}
	matched := false
	for _, certificate := range certificates.Data {
		if !usableIdentityCertificateResource(certificate, now) {
			continue
		}
		der, err := base64.StdEncoding.DecodeString(strings.TrimSpace(certificate.Attributes.CertificateContent))
		if err != nil || len(der) == 0 {
			continue
		}
		if validateIdentityCertificate(identity, der, now) == nil {
			matched = true
			break
		}
	}
	if !matched {
		return fmt.Errorf("local private key does not match an active certificate associated with the selected profile")
	}
	if strings.TrimSpace(profile.Data.Attributes.ProfileType) == "" || !strings.EqualFold(strings.TrimSpace(profile.Data.Attributes.ProfileType), strings.TrimSpace(profileType)) {
		return fmt.Errorf("selected profile type does not match --profile-type")
	}
	if profile.Data.Attributes.ProfileState != asc.ProfileStateActive {
		return fmt.Errorf("selected profile is not active")
	}

	profileDER, err := base64.StdEncoding.DecodeString(strings.TrimSpace(profile.Data.Attributes.ProfileContent))
	if err != nil || len(profileDER) == 0 {
		return fmt.Errorf("selected profile content is unavailable")
	}
	parsedProfile, err := parseIdentityMobileProvision(profileDER)
	if err != nil {
		return fmt.Errorf("selected profile is invalid: %w", err)
	}
	if !parsedProfile.ExpirationDate.After(now) {
		return fmt.Errorf("selected profile is expired")
	}
	if !identityProfileTypeMatches(parsedProfile, profileType) {
		return fmt.Errorf("selected profile distribution type does not match --profile-type")
	}
	teamID := certificateTeamID(identity.Certificate)
	if teamID == "" {
		return fmt.Errorf("matching certificate does not identify an Apple Developer team")
	}
	if !containsFold(parsedProfile.TeamIdentifier, teamID) {
		return fmt.Errorf("selected profile team does not match the signing certificate")
	}
	applicationIdentifier, _ := parsedProfile.Entitlements["application-identifier"].(string)
	if applicationIdentifier == "" {
		applicationIdentifier, _ = parsedProfile.Entitlements["com.apple.application-identifier"].(string)
	}
	if strings.TrimSpace(applicationIdentifier) == "" {
		return fmt.Errorf("selected profile has no application identifier entitlement")
	}
	if strings.TrimSpace(bundleID) == "" {
		return fmt.Errorf("selected profile bundle identifier does not match --bundle-id")
	}
	prefixMatches := false
	for _, prefix := range parsedProfile.ApplicationIdentifierPrefix {
		if trimmed := strings.TrimSpace(prefix); trimmed != "" && applicationIdentifier == trimmed+"."+bundleID {
			prefixMatches = true
			break
		}
	}
	if !prefixMatches {
		if len(parsedProfile.ApplicationIdentifierPrefix) == 0 {
			return fmt.Errorf("selected profile has no application identifier prefix")
		}
		return fmt.Errorf("selected profile bundle identifier does not match --bundle-id and its declared application identifier prefixes")
	}
	for _, embeddedDER := range parsedProfile.DeveloperCertificates {
		if bytes.Equal(embeddedDER, identity.Certificate.Raw) {
			return nil
		}
	}
	return fmt.Errorf("selected profile does not contain the matching signing certificate")
}

func usableIdentityCertificateResource(certificate asc.Resource[asc.CertificateAttributes], now time.Time) bool {
	if certificate.Attributes.Activated != nil && !*certificate.Attributes.Activated {
		return false
	}
	der, err := base64.StdEncoding.DecodeString(strings.TrimSpace(certificate.Attributes.CertificateContent))
	if err != nil || len(der) == 0 {
		return false
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil || now.Before(parsed.NotBefore) || !now.Before(parsed.NotAfter) {
		return false
	}
	expiration := strings.TrimSpace(certificate.Attributes.ExpirationDate)
	if expiration == "" {
		return true
	}
	expiresAt, err := time.Parse(time.RFC3339, expiration)
	return err == nil && expiresAt.After(now)
}

func preflightIdentityForProfileCreate(identity *signingIdentity, plan profileCreatePlan, repositoryPassword string, now time.Time) error {
	if identity == nil || len(plan.Certificates) != 1 {
		return fmt.Errorf("profile creation requires exactly one matching local signing identity")
	}
	certificateDER, err := base64.StdEncoding.DecodeString(strings.TrimSpace(plan.Certificates[0].Attributes.CertificateContent))
	if err != nil || len(certificateDER) == 0 {
		return fmt.Errorf("matching App Store Connect certificate content is unavailable")
	}
	if err := validateIdentityCertificate(identity, certificateDER, now); err != nil {
		return err
	}
	if certificateTeamID(identity.Certificate) == "" {
		return fmt.Errorf("matching certificate does not identify an Apple Developer team")
	}
	if _, err := normalizeSigningIdentity(identity, repositoryPassword); err != nil {
		return err
	}
	return nil
}

func parseIdentityMobileProvision(data []byte) (*identityMobileProvision, error) {
	signed, err := pkcs7.Parse(data)
	if err != nil || len(signed.Content) == 0 {
		return nil, fmt.Errorf("profile is not signed CMS content")
	}
	if err := signed.Verify(); err != nil {
		return nil, fmt.Errorf("verify profile CMS signature: %w", err)
	}
	var profile identityMobileProvision
	if _, err := plist.Unmarshal(signed.Content, &profile); err != nil {
		return nil, fmt.Errorf("decode embedded plist: %w", err)
	}
	return &profile, nil
}

func writeOrReuseSigningIdentity(store *signingpkg.GitStore, relPath string, normalized []byte, password string, metadata signingpkg.EncryptedFileMetadata) error {
	exists, err := preflightSigningArtifact(store, relPath, normalized, password, metadata, func(existing, wanted []byte) bool {
		return samePKCS12Identity(existing, wanted, password)
	})
	if err != nil || exists {
		return err
	}
	return store.WriteEncryptedFileWithMetadata(relPath, normalized, password, metadata)
}

func sameIdentityMetadata(existing, wanted signingpkg.EncryptedFileMetadata) bool {
	existingPath := strings.ReplaceAll(filepath.ToSlash(existing.RelativePath), `\`, "/")
	wantedPath := strings.ReplaceAll(filepath.ToSlash(wanted.RelativePath), `\`, "/")
	return existing.Version == 1 && existing.Kind == wanted.Kind &&
		existingPath == wantedPath && existing.Sensitive == wanted.Sensitive &&
		existing.CertificateSHA256 == wanted.CertificateSHA256 && existing.TeamID == wanted.TeamID &&
		existing.BundleID == wanted.BundleID && existing.ProfileType == wanted.ProfileType &&
		existing.ProfileResourceID == wanted.ProfileResourceID && existing.ProfileUUID == wanted.ProfileUUID && existing.ProfilePath == wanted.ProfilePath &&
		existing.ProfileSHA256 == wanted.ProfileSHA256
}

func samePKCS12Identity(existing, wanted []byte, password string) bool {
	existingKey, existingCertificate, err := pkcs12.Decode(existing, password)
	if err != nil {
		return false
	}
	wantedKey, wantedCertificate, err := pkcs12.Decode(wanted, password)
	if err != nil {
		return false
	}
	return existingCertificate.Equal(wantedCertificate) && publicKeysEqual(existingKey, wantedCertificate.PublicKey) &&
		publicKeysEqual(wantedKey, existingCertificate.PublicKey)
}

func certificateTeamID(certificate *x509.Certificate) string {
	if certificate == nil || len(certificate.Subject.OrganizationalUnit) == 0 {
		return ""
	}
	return strings.TrimSpace(certificate.Subject.OrganizationalUnit[0])
}

func containsFold(values []string, wanted string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(wanted)) {
			return true
		}
	}
	return false
}

func prepareSigningIdentityArtifacts(identity *signingIdentity, password, bundleID, profileType string) (*signingIdentityArtifacts, error) {
	normalized, err := normalizeSigningIdentity(identity, password)
	if err != nil {
		return nil, err
	}
	teamID := certificateTeamID(identity.Certificate)
	category := certDirectoryName(profileType)
	identityPath := filepath.Join("identities", category, identity.CertificateSHA256+".p12")
	identityMetadata := signingpkg.EncryptedFileMetadata{
		Kind:              "pkcs12-identity",
		Sensitive:         true,
		CertificateSHA256: identity.CertificateSHA256,
		TeamID:            teamID,
	}
	bindingDigest := sha256.Sum256([]byte(strings.Join([]string{teamID, bundleID, profileType}, "\x00")))
	bindingPath := filepath.Join("identity-contexts", strings.ToUpper(hex.EncodeToString(bindingDigest[:]))+".json")
	return &signingIdentityArtifacts{
		IdentityPath:     identityPath,
		IdentityData:     normalized,
		IdentityMetadata: identityMetadata,
		BindingPath:      bindingPath,
		BindingMetadata: signingpkg.EncryptedFileMetadata{
			Kind:              "identity-context",
			CertificateSHA256: identity.CertificateSHA256,
			TeamID:            teamID,
			BundleID:          bundleID,
			ProfileType:       profileType,
		},
	}, nil
}

func preflightSigningIdentityArtifacts(store *signingpkg.GitStore, artifacts *signingIdentityArtifacts, password string) error {
	if _, err := preflightSigningArtifact(store, artifacts.IdentityPath, artifacts.IdentityData, password, artifacts.IdentityMetadata, func(existing, wanted []byte) bool {
		return samePKCS12Identity(existing, wanted, password)
	}); err != nil {
		return err
	}
	_, err := preflightSigningArtifact(store, artifacts.BindingPath, artifacts.BindingData, password, artifacts.BindingMetadata, bytes.Equal)
	return err
}

func preflightSigningIdentityArtifactsForContextUpdate(store *signingpkg.GitStore, artifacts *signingIdentityArtifacts, password string) error {
	if _, err := preflightSigningArtifact(store, artifacts.IdentityPath, artifacts.IdentityData, password, artifacts.IdentityMetadata, func(existing, wanted []byte) bool {
		return samePKCS12Identity(existing, wanted, password)
	}); err != nil {
		return err
	}
	existing, metadata, err := store.ReadEncryptedFileWithMetadata(artifacts.BindingPath, password)
	if errors.Is(err, os.ErrNotExist) {
		return store.CheckWriteEncryptedFile(artifacts.BindingPath)
	}
	if err != nil {
		return fmt.Errorf("existing identity context cannot be authenticated before profile creation")
	}
	var binding identityContextBinding
	if json.Unmarshal(existing, &binding) != nil || !sameIdentityContextScope(metadata, artifacts.BindingMetadata) ||
		binding.TeamID != artifacts.BindingMetadata.TeamID ||
		binding.BundleID != artifacts.BindingMetadata.BundleID || binding.ProfileType != artifacts.BindingMetadata.ProfileType {
		return fmt.Errorf("existing identity context has a different authenticated scope; explicitly migrate it before creating a profile")
	}
	return store.CheckWriteEncryptedFile(artifacts.BindingPath)
}

func bindSigningIdentityProfile(artifacts *signingIdentityArtifacts, profile *asc.ProfileResponse, profilePath string, profileContent []byte) error {
	if artifacts == nil || profile == nil || strings.TrimSpace(profile.Data.ID) == "" || strings.TrimSpace(profilePath) == "" || len(profileContent) == 0 {
		return fmt.Errorf("identity context profile binding is incomplete")
	}
	digest := sha256.Sum256(profileContent)
	parsedProfile, err := parseIdentityMobileProvision(profileContent)
	if err != nil {
		return fmt.Errorf("identity context profile has no verified UUID: %w", err)
	}
	signedUUID, err := normalizeIdentityProfileUUID(parsedProfile.UUID)
	if err != nil {
		return fmt.Errorf("identity context profile has no valid verified UUID: %w", err)
	}
	if rawAPIUUID := strings.TrimSpace(profile.Data.Attributes.UUID); rawAPIUUID != "" {
		apiUUID, err := normalizeIdentityProfileUUID(rawAPIUUID)
		if err != nil {
			return fmt.Errorf("profile UUID returned by App Store Connect is invalid: %w", err)
		}
		if apiUUID != signedUUID {
			return fmt.Errorf("profile UUID returned by App Store Connect does not match signed profile UUID")
		}
	}
	binding := identityContextBinding{
		CertificateSHA256: artifacts.IdentityMetadata.CertificateSHA256,
		TeamID:            artifacts.IdentityMetadata.TeamID,
		BundleID:          artifacts.BindingMetadata.BundleID,
		ProfileType:       artifacts.BindingMetadata.ProfileType,
		ProfileResourceID: profile.Data.ID,
		ProfileUUID:       signedUUID,
		ProfilePath:       filepath.ToSlash(profilePath),
		ProfileSHA256:     strings.ToUpper(hex.EncodeToString(digest[:])),
	}
	data, err := json.Marshal(binding)
	if err != nil {
		return fmt.Errorf("encode identity context binding: %w", err)
	}
	artifacts.BindingData = data
	artifacts.BindingMetadata.ProfileResourceID = binding.ProfileResourceID
	artifacts.BindingMetadata.ProfileUUID = binding.ProfileUUID
	artifacts.BindingMetadata.ProfilePath = binding.ProfilePath
	artifacts.BindingMetadata.ProfileSHA256 = binding.ProfileSHA256
	return nil
}

func normalizeIdentityProfileUUID(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("profile UUID is empty")
	}
	parsed, err := uuid.Parse(trimmed)
	if err != nil || !strings.EqualFold(trimmed, parsed.String()) {
		return "", fmt.Errorf("profile UUID %q is not a canonical UUID", trimmed)
	}
	return parsed.String(), nil
}

func signingAssetRepositoryPaths(certificates []asc.Resource[asc.CertificateAttributes], profileType, profileName, profileFallback string, artifacts *signingIdentityArtifacts) []string {
	paths := make([]string, 0, len(certificates)+3)
	certDir := certDirectoryName(profileType)
	for _, certificate := range certificates {
		paths = append(paths, filepath.Join("certs", certDir, safeFileName(certificate.Attributes.SerialNumber, certificate.ID)+".cer"))
	}
	paths = append(paths, filepath.Join("profiles", profileDirectoryName(profileType), safeFileName(profileName, profileFallback)+".mobileprovision"))
	if artifacts != nil {
		paths = append(paths, artifacts.IdentityPath, artifacts.BindingPath)
	}
	return paths
}

func preflightSigningAssetDestinations(store *signingpkg.GitStore, plan profileCreatePlan, profileType string) error {
	certDir := certDirectoryName(profileType)
	for _, certificate := range plan.Certificates {
		relPath := filepath.Join("certs", certDir, safeFileName(certificate.Attributes.SerialNumber, certificate.ID)+".cer")
		if err := store.CheckWriteEncryptedFile(relPath); err != nil {
			return fmt.Errorf("preflight certificate destination: %w", err)
		}
	}
	profileRelPath := filepath.Join("profiles", profileDirectoryName(profileType), safeFileName(plan.ProfileName, "profile")+".mobileprovision")
	if err := store.CheckWriteEncryptedFile(profileRelPath); err != nil {
		return fmt.Errorf("preflight profile destination: %w", err)
	}
	return nil
}

func writeOrReuseSigningIdentityArtifacts(store *signingpkg.GitStore, artifacts *signingIdentityArtifacts, password string) error {
	if err := writeOrReuseSigningIdentity(store, artifacts.IdentityPath, artifacts.IdentityData, password, artifacts.IdentityMetadata); err != nil {
		return err
	}
	existing, metadata, err := store.ReadEncryptedFileWithMetadata(artifacts.BindingPath, password)
	if err == nil && sameIdentityMetadata(metadata, withRelativePath(artifacts.BindingMetadata, artifacts.BindingPath)) && bytes.Equal(existing, artifacts.BindingData) {
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("identity context cannot be verified for replacement")
	}
	if err == nil && !sameIdentityContextScope(metadata, artifacts.BindingMetadata) {
		return fmt.Errorf("identity context has a different authenticated scope; explicitly migrate it")
	}
	return store.ReplaceEncryptedFileWithMetadata(artifacts.BindingPath, artifacts.BindingData, password, artifacts.BindingMetadata)
}

func withRelativePath(metadata signingpkg.EncryptedFileMetadata, path string) signingpkg.EncryptedFileMetadata {
	metadata.RelativePath = path
	return metadata
}

func sameIdentityContextScope(existing, wanted signingpkg.EncryptedFileMetadata) bool {
	return existing.Version == 1 && existing.Kind == "identity-context" && !existing.Sensitive &&
		existing.TeamID == wanted.TeamID &&
		existing.BundleID == wanted.BundleID && existing.ProfileType == wanted.ProfileType
}

func preflightSigningArtifact(store *signingpkg.GitStore, relPath string, wanted []byte, password string, metadata signingpkg.EncryptedFileMetadata, samePlaintext func([]byte, []byte) bool) (bool, error) {
	existing, existingMetadata, err := store.ReadEncryptedFileWithMetadata(relPath, password)
	if errors.Is(err, os.ErrNotExist) {
		if err := store.CheckNewEncryptedFile(relPath); err != nil {
			return false, err
		}
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("signing identity artifact already exists and cannot be verified; use a different branch or explicitly migrate the existing artifact")
	}
	metadata.RelativePath = relPath
	if !sameIdentityMetadata(existingMetadata, metadata) || !samePlaintext(existing, wanted) {
		return false, fmt.Errorf("signing identity artifact already exists with different authenticated content; use a different branch or explicitly migrate the existing artifact")
	}
	return true, nil
}
