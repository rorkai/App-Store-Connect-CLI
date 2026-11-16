package signing

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"go.mozilla.org/pkcs7"
	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/infoplist"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const (
	signingResignManifestSchemaVersion   = 1
	signingResignManifestMaxBytes        = 1 << 20
	signingResignManifestMaxEntries      = 256
	signingResignProfileMaxBytes         = 16 << 20
	signingResignProfileClassDevelopment = "development"
	signingResignProfileClassAdHoc       = "ad-hoc"
	signingResignProfileClassAppStore    = "app-store"
	signingResignProfileClassEnterprise  = "enterprise"
)

type signingResignManifest struct {
	SchemaVersion int                          `json:"schemaVersion"`
	Profiles      []signingResignManifestEntry `json:"profiles"`
}

// signingResignManifestAllowedJSONFields lists the exact field spellings the
// strict manifest schema accepts. encoding/json would otherwise match a
// case-variant alias such as BundleID to the bundleId tag, bypassing
// DisallowUnknownFields.
var signingResignManifestAllowedJSONFields = map[string]struct{}{
	"schemaVersion": {},
	"profiles":      {},
	"bundleId":      {},
	"profilePath":   {},
}

type signingResignManifestEntry struct {
	BundleID    string `json:"bundleId"`
	ProfilePath string `json:"profilePath"`
}

type signingResignProfile struct {
	Data                        []byte
	UUID                        string
	TeamID                      string
	ApplicationIdentifierPrefix string
	BundleID                    string
	Class                       string
	ExpirationDate              time.Time
	Entitlements                map[string]any
	DeveloperCertificates       [][]byte
	SHA256                      string
}

var signingResignAppleProfileRootFingerprints = map[string]struct{}{
	"b0b1730ecbc7ff4505142c49f1295e6eda6bcaed7e2c68c5be91b5a11001f024": {},
	"c2b9b042dd57830e7d117dac55ac8ae19407d38e41d88f3215bc3a890444a050": {},
	"63343abfb89a6a03ebb57e9b3f5fa7be7c4f5c756f3017b3a8c488c3653e9179": {},
}

func readSigningResignManifest(path string) (signingResignManifest, error) {
	if strings.TrimSpace(path) == "" {
		return signingResignManifest{}, fmt.Errorf("profiles manifest path is empty")
	}
	path = filepath.Clean(path)
	data, err := readBoundedSigningRunFile(path, signingResignManifestMaxBytes, false)
	if err != nil {
		return signingResignManifest{}, fmt.Errorf("read profiles manifest failed")
	}
	defer clear(data)
	manifest, err := decodeSigningResignManifest(data)
	if err != nil {
		// Only the manifest's schema/JSON validation is a usage failure. File
		// access remains an operational failure so a missing or unreadable
		// manifest does not masquerade as a malformed command invocation.
		return signingResignManifest{}, signingResignUsage(err)
	}
	return manifest, nil
}

func decodeSigningResignManifest(data []byte) (signingResignManifest, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return signingResignManifest{}, fmt.Errorf("profiles manifest is empty")
	}
	if len(data) > signingResignManifestMaxBytes {
		return signingResignManifest{}, fmt.Errorf("profiles manifest exceeds %d bytes", signingResignManifestMaxBytes)
	}
	if err := validateSigningRunJSONKeys(data, signingResignManifestAllowedJSONFields); err != nil {
		return signingResignManifest{}, fmt.Errorf("profiles manifest contains invalid fields: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest signingResignManifest
	if err := decoder.Decode(&manifest); err != nil {
		return signingResignManifest{}, fmt.Errorf("decode profiles manifest: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return signingResignManifest{}, fmt.Errorf("decode profiles manifest: %w", err)
	}
	if manifest.SchemaVersion != signingResignManifestSchemaVersion {
		return signingResignManifest{}, fmt.Errorf("profiles manifest schemaVersion must be %d", signingResignManifestSchemaVersion)
	}
	if len(manifest.Profiles) == 0 {
		return signingResignManifest{}, fmt.Errorf("profiles manifest must contain at least one profile")
	}
	if len(manifest.Profiles) > signingResignManifestMaxEntries {
		return signingResignManifest{}, fmt.Errorf("profiles manifest contains too many profile entries")
	}
	seenBundleIDs := make(map[string]struct{}, len(manifest.Profiles))
	seenProfilePaths := make(map[string]struct{}, len(manifest.Profiles))
	for index := range manifest.Profiles {
		entry := &manifest.Profiles[index]
		entry.BundleID = strings.TrimSpace(entry.BundleID)
		if err := validateSigningResignBundleID(entry.BundleID); err != nil {
			return signingResignManifest{}, fmt.Errorf("profiles[%d].bundleId: %w", index, err)
		}
		if err := validateSigningResignProfilePath(entry.ProfilePath); err != nil {
			return signingResignManifest{}, fmt.Errorf("profiles[%d].profilePath: %w", index, err)
		}
		bundleKey := strings.ToLower(entry.BundleID)
		if _, exists := seenBundleIDs[bundleKey]; exists {
			return signingResignManifest{}, fmt.Errorf("profiles manifest contains duplicate bundleId %q", entry.BundleID)
		}
		seenBundleIDs[bundleKey] = struct{}{}
		pathKey := strings.ToLower(filepath.ToSlash(filepath.Clean(entry.ProfilePath)))
		if _, exists := seenProfilePaths[pathKey]; exists {
			return signingResignManifest{}, fmt.Errorf("profiles manifest contains duplicate profilePath")
		}
		seenProfilePaths[pathKey] = struct{}{}
	}
	return manifest, nil
}

func validateSigningResignProfilePath(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("profilePath is required")
	}
	if strings.ContainsRune(value, '\\') {
		return fmt.Errorf("profilePath must use relative slash-separated components")
	}
	if err := rootfs.ValidateRelative(value); err != nil {
		return err
	}
	cleaned := filepath.Clean(filepath.FromSlash(value))
	if cleaned == "." || cleaned == ".." || filepath.IsAbs(cleaned) || filepath.VolumeName(cleaned) != "" {
		return fmt.Errorf("profilePath must be relative to the manifest directory")
	}
	return nil
}

func validateSigningResignBundleID(value string) error {
	if value == "" || len(value) > 255 || strings.TrimSpace(value) != value {
		return fmt.Errorf("bundle identifier is missing or too long")
	}
	for _, component := range strings.Split(value, ".") {
		if component == "" || component == "*" {
			return fmt.Errorf("bundle identifier must be an exact non-wildcard value")
		}
		for index := 0; index < len(component); index++ {
			character := component[index]
			if (character < 'a' || character > 'z') &&
				(character < 'A' || character > 'Z') &&
				(character < '0' || character > '9') && character != '-' {
				return fmt.Errorf("bundle identifier contains unsupported characters")
			}
		}
	}
	return nil
}

func readSigningResignProfiles(manifestPath string, manifest signingResignManifest, clock func() time.Time) (map[string]signingResignProfile, error) {
	manifestPath = filepath.Clean(manifestPath)
	root, err := rootfs.New(filepath.Dir(manifestPath))
	if err != nil {
		return nil, fmt.Errorf("open profiles manifest directory: %w", err)
	}
	defer root.Close()
	profiles := make(map[string]signingResignProfile, len(manifest.Profiles))
	for index, entry := range manifest.Profiles {
		file, err := root.OpenFile(filepath.FromSlash(entry.ProfilePath))
		if err != nil {
			return nil, fmt.Errorf("read profile for bundle %s failed", entry.BundleID)
		}
		info, err := file.Stat()
		if err == nil {
			err = validateSigningResignProfileFile(entry.ProfilePath, info)
		}
		if err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("validate profile for bundle %s failed", entry.BundleID)
		}
		data, readErr := io.ReadAll(io.LimitReader(file, signingResignProfileMaxBytes+1))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			return nil, fmt.Errorf("read profile for bundle %s failed", entry.BundleID)
		}
		if len(data) > signingResignProfileMaxBytes {
			clear(data)
			return nil, fmt.Errorf("profiles[%d] for bundle %s exceeds %d bytes", index, entry.BundleID, signingResignProfileMaxBytes)
		}
		profile, err := inspectSigningResignProfile(data, clock())
		if err != nil {
			clear(data)
			return nil, fmt.Errorf("validate profile for bundle %s failed", entry.BundleID)
		}
		if profile.BundleID != entry.BundleID {
			clear(data)
			return nil, fmt.Errorf("profiles[%d] bundle %s does not match its manifest bundle %s", index, profile.BundleID, entry.BundleID)
		}
		if err := validateSigningResignProfileForTarget(profile, entry.BundleID); err != nil {
			clear(data)
			return nil, fmt.Errorf("validate profile for bundle %s failed", entry.BundleID)
		}
		profile.Data = data
		profiles[entry.BundleID] = profile
	}
	return profiles, nil
}

func validateSigningResignProfileForTarget(profile signingResignProfile, bundleID string) error {
	if profile.BundleID != bundleID {
		return fmt.Errorf("profile bundle identifier does not match target")
	}
	if err := validateSigningResignTeamID(profile.TeamID); err != nil {
		return fmt.Errorf("profile team identifier is invalid")
	}
	if err := validateSigningResignTeamID(profile.ApplicationIdentifierPrefix); err != nil {
		return fmt.Errorf("profile application identifier prefix is invalid")
	}
	applicationID, ok := profile.Entitlements["application-identifier"].(string)
	if !ok || applicationID != profile.ApplicationIdentifierPrefix+"."+bundleID {
		return fmt.Errorf("profile application identifier does not match target")
	}
	teamID, ok := profile.Entitlements["com.apple.developer.team-identifier"].(string)
	if !ok || teamID != profile.TeamID {
		return fmt.Errorf("profile team entitlement does not match profile team")
	}
	if alternate, exists := profile.Entitlements["com.apple.application-identifier"]; exists {
		value, ok := alternate.(string)
		if !ok || value != applicationID {
			return fmt.Errorf("profile application identifiers are contradictory")
		}
	}
	return nil
}

func validateSigningResignProfileFile(_ string, info os.FileInfo) error {
	if info == nil || !info.Mode().IsRegular() {
		return fmt.Errorf("profile is not a regular file")
	}
	if err := validateSigningRunInputPermissions("profile", info, false); err != nil {
		return fmt.Errorf("profile ownership or permissions are invalid")
	}
	return nil
}

func inspectSigningResignProfile(data []byte, now time.Time) (signingResignProfile, error) {
	if len(data) == 0 || len(data) > signingResignProfileMaxBytes {
		return signingResignProfile{}, fmt.Errorf("profile size is invalid")
	}
	p7, err := pkcs7.Parse(data)
	if err != nil {
		return signingResignProfile{}, fmt.Errorf("profile is not signed CMS data")
	}
	if len(p7.Content) == 0 {
		return signingResignProfile{}, fmt.Errorf("profile CMS content is empty")
	}
	if err := p7.Verify(); err != nil {
		return signingResignProfile{}, fmt.Errorf("profile CMS integrity verification failed")
	}
	if err := verifySigningResignProfileTrust(p7, now); err != nil {
		return signingResignProfile{}, err
	}
	if err := infoplist.ValidateStructure(p7.Content); err != nil {
		return signingResignProfile{}, fmt.Errorf("profile plist is invalid")
	}
	var payload signingRunMobileProvision
	if _, err := plist.Unmarshal(p7.Content, &payload); err != nil {
		return signingResignProfile{}, fmt.Errorf("decode profile plist")
	}
	payload.UUID = strings.TrimSpace(payload.UUID)
	if !signingRunUUIDPattern.MatchString(payload.UUID) {
		return signingResignProfile{}, fmt.Errorf("profile UUID is missing or invalid")
	}
	if payload.ExpirationDate.IsZero() || !now.Before(payload.ExpirationDate) {
		return signingResignProfile{}, fmt.Errorf("profile is expired")
	}
	if !payload.CreationDate.IsZero() && payload.CreationDate.After(now) {
		return signingResignProfile{}, fmt.Errorf("profile creation date is in the future")
	}
	if !slices.ContainsFunc(payload.Platform, func(value string) bool {
		return strings.EqualFold(strings.TrimSpace(value), "iOS")
	}) {
		return signingResignProfile{}, fmt.Errorf("profile does not target iOS")
	}
	if payload.ProvisionsAllDevices {
		return signingResignProfile{}, fmt.Errorf("enterprise profiles are not supported")
	}
	teamID, err := signingRunTeamID(payload.TeamIdentifier)
	if err != nil {
		return signingResignProfile{}, err
	}
	if err := validateSigningResignTeamID(teamID); err != nil {
		return signingResignProfile{}, err
	}
	if len(payload.ApplicationIdentifierPrefix) != 1 || strings.TrimSpace(payload.ApplicationIdentifierPrefix[0]) == "" {
		return signingResignProfile{}, fmt.Errorf("profile must declare exactly one application identifier prefix")
	}
	prefix := strings.TrimSpace(payload.ApplicationIdentifierPrefix[0])
	if err := validateSigningResignTeamID(prefix); err != nil {
		return signingResignProfile{}, fmt.Errorf("profile application identifier prefix: %w", err)
	}
	if entitlementTeam, ok := payload.Entitlements["com.apple.developer.team-identifier"].(string); !ok || strings.TrimSpace(entitlementTeam) != teamID {
		return signingResignProfile{}, fmt.Errorf("profile entitlement team identifier does not match TeamIdentifier")
	}
	applicationID, ok := payload.Entitlements["application-identifier"].(string)
	if !ok || strings.TrimSpace(applicationID) == "" {
		return signingResignProfile{}, fmt.Errorf("profile application identifier is missing")
	}
	applicationID = strings.TrimSpace(applicationID)
	if strings.ContainsRune(applicationID, '*') || !strings.HasPrefix(applicationID, prefix+".") {
		return signingResignProfile{}, fmt.Errorf("profile application identifier must be an exact value")
	}
	bundleID := strings.TrimPrefix(applicationID, prefix+".")
	if err := validateSigningResignBundleID(bundleID); err != nil {
		return signingResignProfile{}, fmt.Errorf("profile application identifier: %w", err)
	}
	if alternate, exists := payload.Entitlements["com.apple.application-identifier"]; exists {
		value, ok := alternate.(string)
		if !ok || strings.TrimSpace(value) != applicationID {
			return signingResignProfile{}, fmt.Errorf("profile application identifiers are inconsistent")
		}
	}
	debugValue := payload.Entitlements["get-task-allow"]
	debuggable, ok := debugValue.(bool)
	if !ok {
		return signingResignProfile{}, fmt.Errorf("profile get-task-allow entitlement must be a boolean")
	}
	devices := canonicalSigningResignStrings(payload.ProvisionedDevices)
	class := signingResignProfileClassAppStore
	switch {
	case debuggable && len(devices) > 0:
		class = signingResignProfileClassDevelopment
	case !debuggable && len(devices) > 0:
		class = signingResignProfileClassAdHoc
	case debuggable:
		return signingResignProfile{}, fmt.Errorf("profile class is unknown")
	}
	for _, certificateDER := range payload.DeveloperCertificates {
		if _, err := x509.ParseCertificate(certificateDER); err != nil {
			return signingResignProfile{}, fmt.Errorf("profile contains an invalid signing certificate")
		}
	}
	digest := sha256.Sum256(data)
	return signingResignProfile{
		TeamID: teamID, ApplicationIdentifierPrefix: prefix, BundleID: bundleID,
		Class: class, ExpirationDate: payload.ExpirationDate,
		Entitlements: payload.Entitlements, DeveloperCertificates: payload.DeveloperCertificates,
		SHA256: strings.ToUpper(hex.EncodeToString(digest[:])), UUID: strings.TrimSpace(payload.UUID),
	}, nil
}

func verifySigningResignProfileTrust(profile *pkcs7.PKCS7, now time.Time) error {
	if len(profile.Signers) != 1 {
		return fmt.Errorf("profile must contain exactly one CMS signer")
	}
	signer := profile.GetOnlySigner()
	if signer == nil || signer.Subject.CommonName != "Apple iPhone OS Provisioning Profile Signing" ||
		len(signer.Subject.Organization) != 1 || signer.Subject.Organization[0] != "Apple Inc." ||
		signer.Issuer.CommonName != "Apple iPhone Certification Authority" {
		return fmt.Errorf("profile CMS signer is not an accepted Apple provisioning signer")
	}
	pool := x509.NewCertPool()
	foundPinnedRoot := false
	for _, certificate := range profile.Certificates {
		if !certificate.IsCA || (!bytes.Equal(certificate.RawSubject, certificate.RawIssuer) && certificate.Subject.String() != certificate.Issuer.String()) {
			continue
		}
		digest := sha256.Sum256(certificate.Raw)
		if _, ok := signingResignAppleProfileRootFingerprints[hex.EncodeToString(digest[:])]; !ok {
			continue
		}
		pool.AddCert(certificate)
		foundPinnedRoot = true
	}
	if !foundPinnedRoot {
		return fmt.Errorf("profile chain does not contain an accepted Apple root")
	}
	if err := profile.VerifyWithChainAtTime(pool, now); err != nil {
		return fmt.Errorf("profile trust verification failed")
	}
	return nil
}

func validateSigningResignTeamID(value string) error {
	if value == "" || len(value) > 32 {
		return fmt.Errorf("team identifier is invalid")
	}
	for _, character := range value {
		if unicode.IsControl(character) || (character < '0' || character > '9') &&
			(character < 'A' || character > 'Z') && (character < 'a' || character > 'z') {
			return fmt.Errorf("team identifier contains unsupported characters")
		}
	}
	return nil
}

func canonicalSigningResignStrings(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	slices.Sort(result)
	return result
}

func validateSigningResignProfileIdentity(profile signingResignProfile, identity *signingRunIdentity) error {
	if identity == nil || identity.Certificate == nil {
		return fmt.Errorf("signing identity is missing")
	}
	teamID, err := signingRunCertificateTeamID(identity.Certificate)
	if err != nil {
		return err
	}
	if teamID != profile.TeamID {
		return fmt.Errorf("signing identity team does not match profile")
	}
	found := false
	for _, certificateDER := range profile.DeveloperCertificates {
		if bytes.Equal(identity.Certificate.Raw, certificateDER) {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("signing identity certificate is not permitted by profile")
	}
	return nil
}

func validateSigningResignProfileSet(profiles map[string]signingResignProfile, identity *signingRunIdentity) error {
	if len(profiles) == 0 {
		return fmt.Errorf("profiles manifest contains no profiles")
	}
	var teamID, class string
	bundleIDs := make([]string, 0, len(profiles))
	for bundleID := range profiles {
		bundleIDs = append(bundleIDs, bundleID)
	}
	slices.Sort(bundleIDs)
	for _, bundleID := range bundleIDs {
		profile := profiles[bundleID]
		if profile.BundleID != bundleID {
			return fmt.Errorf("profile mapping for bundle %s is inconsistent", bundleID)
		}
		if class == "" {
			class = profile.Class
		} else if class != profile.Class {
			return fmt.Errorf("profiles must use one consistent profile class")
		}
		if teamID == "" {
			teamID = profile.TeamID
		} else if teamID != profile.TeamID {
			return fmt.Errorf("profiles must use one consistent team")
		}
		if err := validateSigningResignProfileIdentity(profile, identity); err != nil {
			return fmt.Errorf("profile for bundle %s: %w", bundleID, err)
		}
	}
	return nil
}

func validateSigningResignManifestTargets(manifest signingResignManifest, targetIDs map[string]struct{}) error {
	if len(manifest.Profiles) != len(targetIDs) {
		return signingResignUsage(fmt.Errorf("profiles manifest must contain exactly one entry for every app-like target"))
	}
	for _, entry := range manifest.Profiles {
		if _, ok := targetIDs[entry.BundleID]; !ok {
			return signingResignUsage(fmt.Errorf("profiles manifest contains an entry for an undiscovered target"))
		}
	}
	return nil
}
