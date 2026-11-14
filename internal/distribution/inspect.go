// Package distribution inspects and prepares local iOS release-testing
// artifacts. It deliberately contains no account, keychain, network, or
// storage-provider operations.
package distribution

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"go.mozilla.org/pkcs7"
	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/deviceset"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/infoplist"
)

const (
	maxArchiveEntries              = 20_000
	maxArchiveMemberNameLen        = 4096
	maxArchiveExpandedBytes uint64 = 16 << 30
	maxProfileBytes                = 16 << 20
	// MaxIPABytes bounds synchronous inspection, hashing, and preparation work.
	MaxIPABytes int64 = 8 << 30
)

type ProfileClass string

const (
	ProfileClassUnknown     ProfileClass = "unknown"
	ProfileClassDevelopment ProfileClass = "development"
	ProfileClassAdHoc       ProfileClass = "ad-hoc"
	ProfileClassEnterprise  ProfileClass = "enterprise"
	ProfileClassAppStore    ProfileClass = "app-store"
)

type App struct {
	BundleID         string `json:"bundleId"`
	Title            string `json:"title"`
	Version          string `json:"version"`
	BuildNumber      string `json:"buildNumber"`
	MinimumOSVersion string `json:"minimumOSVersion,omitempty"`
}

type Artifact struct {
	RelativePath string `json:"relativePath,omitempty"`
	SizeBytes    int64  `json:"sizeBytes"`
	SHA256       string `json:"sha256"`
}

type Signing struct {
	ProfileClass                         ProfileClass              `json:"profileClass"`
	ProfileUUID                          string                    `json:"profileUuid,omitempty"`
	TeamID                               string                    `json:"teamId,omitempty"`
	ExpiresAt                            string                    `json:"expiresAt,omitempty"`
	DeviceCount                          int                       `json:"deviceCount"`
	DeviceSetSHA256                      string                    `json:"deviceSetSha256,omitempty"`
	EmbeddedProfileSHA256                string                    `json:"embeddedProfileSha256,omitempty"`
	ProfileCertificateSHA256Fingerprints []string                  `json:"profileCertificateSha256Fingerprints,omitempty"`
	Devices                              []string                  `json:"devices,omitempty"`
	ProfileIntegrityVerification         CodeSignatureVerification `json:"profileIntegrityVerification"`
	ProfileTrustVerification             CodeSignatureVerification `json:"profileTrustVerification"`
	CodeSignatureVerification            CodeSignatureVerification `json:"codeSignatureVerification"`
}

type Preparation struct {
	MetadataEligible bool     `json:"metadataEligible"`
	Issues           []string `json:"issues"`
}

type CodeSignatureVerificationStatus string

const (
	CodeSignatureVerified    CodeSignatureVerificationStatus = "verified"
	CodeSignatureInvalid     CodeSignatureVerificationStatus = "invalid"
	CodeSignatureNotVerified CodeSignatureVerificationStatus = "not-verified"
)

type CodeSignatureVerification struct {
	Status                              CodeSignatureVerificationStatus `json:"status"`
	Scope                               string                          `json:"scope,omitempty"`
	Reason                              string                          `json:"reason,omitempty"`
	SignerCertificateSHA256Fingerprints []string                        `json:"signerCertificateSha256Fingerprints,omitempty"`
}

type Source struct {
	Channel  string `json:"channel,omitempty"`
	Revision string `json:"revision,omitempty"`
	URL      string `json:"url,omitempty"`
}

type Inspection struct {
	SchemaVersion      string      `json:"schemaVersion"`
	Platform           string      `json:"platform"`
	DistributionMethod string      `json:"distributionMethod"`
	App                App         `json:"app"`
	Artifact           Artifact    `json:"artifact"`
	Signing            Signing     `json:"signing"`
	Preparation        Preparation `json:"preparation"`
	EmbeddedTargets    []string    `json:"embeddedTargets,omitempty"`
}

type InspectOptions struct {
	IncludeDevices bool
	Now            time.Time
}

var (
	duringZIPValidationForTest func(string)
	duringZIPStreamReadForTest func()
)

type infoPlistPayload struct {
	BundleID           string   `plist:"CFBundleIdentifier"`
	Executable         string   `plist:"CFBundleExecutable"`
	DisplayName        string   `plist:"CFBundleDisplayName"`
	Name               string   `plist:"CFBundleName"`
	Version            string   `plist:"CFBundleShortVersionString"`
	Build              string   `plist:"CFBundleVersion"`
	MinimumOS          string   `plist:"MinimumOSVersion"`
	PlatformName       string   `plist:"DTPlatformName"`
	SupportedPlatforms []string `plist:"CFBundleSupportedPlatforms"`
}

type embeddedProfile struct {
	UUID                        string         `plist:"UUID"`
	TeamIdentifier              []string       `plist:"TeamIdentifier"`
	ApplicationIdentifierPrefix []string       `plist:"ApplicationIdentifierPrefix"`
	ProvisionedDevices          []string       `plist:"ProvisionedDevices"`
	ProvisionsAllDevices        bool           `plist:"ProvisionsAllDevices"`
	ExpirationDate              time.Time      `plist:"ExpirationDate"`
	Entitlements                map[string]any `plist:"Entitlements"`
	DeveloperCertificates       [][]byte       `plist:"DeveloperCertificates"`
	Platform                    []string       `plist:"Platform"`
}

type parsedProfile struct {
	embeddedProfile
	BundleID                 string
	Class                    ProfileClass
	EmbeddedProfileSHA256    string
	ProfileTrustVerification CodeSignatureVerification
}

var appleProfileRootFingerprints = map[string]struct{}{
	"b0b1730ecbc7ff4505142c49f1295e6eda6bcaed7e2c68c5be91b5a11001f024": {},
	"c2b9b042dd57830e7d117dac55ac8ae19407d38e41d88f3215bc3a890444a050": {},
	"63343abfb89a6a03ebb57e9b3f5fa7be7c4f5c756f3017b3a8c488c3653e9179": {},
}

// InspectIPAContext validates and reads deterministic metadata from an
// already-open regular IPA file. The file must remain open for the duration of
// the call.
func InspectIPAContext(ctx context.Context, file *os.File, size int64, options InspectOptions) (Inspection, error) {
	if err := contextError(ctx); err != nil {
		return Inspection{}, err
	}
	if file == nil {
		return Inspection{}, fmt.Errorf("IPA file is nil")
	}
	if size < 0 {
		return Inspection{}, fmt.Errorf("IPA size is invalid")
	}
	if size > MaxIPABytes {
		return Inspection{}, fmt.Errorf("IPA size %d bytes exceeds supported limit of %d bytes", size, MaxIPABytes)
	}
	snapshot, digest, cleanup, err := snapshotIPAContext(ctx, file, size)
	if err != nil {
		return Inspection{}, err
	}
	defer cleanup()
	if afterIPASnapshotForTest != nil {
		afterIPASnapshotForTest()
	}
	return inspectSnapshotContext(ctx, snapshot, size, digest, options)
}

func inspectSnapshotContext(ctx context.Context, file *os.File, size int64, digest string, options InspectOptions) (Inspection, error) {
	if err := contextError(ctx); err != nil {
		return Inspection{}, err
	}
	reader, err := zip.NewReader(file, size)
	if err != nil {
		return Inspection{}, fmt.Errorf("open IPA ZIP: %w", err)
	}
	if len(reader.File) > maxArchiveEntries {
		return Inspection{}, fmt.Errorf("IPA contains %d entries; limit is %d", len(reader.File), maxArchiveEntries)
	}

	seen := make(map[string]bool, len(reader.File))
	hasDescendants := make(map[string]struct{}, len(reader.File))
	var infoFiles []*zip.File
	var embeddedTargets []string
	var declaredExpandedBytes uint64
	for _, member := range reader.File {
		if err := validateArchiveMember(member); err != nil {
			return Inspection{}, err
		}
		if member.UncompressedSize64 > maxArchiveExpandedBytes-declaredExpandedBytes {
			return Inspection{}, fmt.Errorf("IPA declared expansion exceeds %d bytes", maxArchiveExpandedBytes)
		}
		declaredExpandedBytes += member.UncompressedSize64
		key := strings.ToLower(strings.TrimSuffix(member.Name, "/"))
		if _, exists := seen[key]; exists {
			return Inspection{}, fmt.Errorf("IPA contains duplicate path %q", member.Name)
		}
		isDirectory := member.FileInfo().IsDir()
		for ancestor := path.Dir(key); ancestor != "."; ancestor = path.Dir(ancestor) {
			if ancestorIsDirectory, exists := seen[ancestor]; exists && !ancestorIsDirectory {
				return Inspection{}, fmt.Errorf("IPA contains file/directory path collision involving %q", member.Name)
			}
		}
		if !isDirectory {
			if _, exists := hasDescendants[key]; exists {
				return Inspection{}, fmt.Errorf("IPA contains file/directory path collision involving %q", member.Name)
			}
		}
		seen[key] = isDirectory
		for ancestor := path.Dir(key); ancestor != "."; ancestor = path.Dir(ancestor) {
			hasDescendants[ancestor] = struct{}{}
		}
		if isMainAppMember(member.Name, "Info.plist") && !member.FileInfo().IsDir() {
			infoFiles = append(infoFiles, member)
		} else if isEmbeddedTargetInfoPlist(member.Name) && !member.FileInfo().IsDir() {
			embeddedTargets = append(embeddedTargets, member.Name)
		}
	}
	var expandedBytes uint64
	for _, member := range reader.File {
		if duringZIPValidationForTest != nil {
			duringZIPValidationForTest(member.Name)
		}
		if err := contextError(ctx); err != nil {
			return Inspection{}, err
		}
		remaining := maxArchiveExpandedBytes - expandedBytes
		streamMember := member
		if member.FileInfo().IsDir() {
			regular := *member
			regular.Name = strings.TrimSuffix(regular.Name, "/")
			regular.SetMode(0o600)
			streamMember = &regular
		}
		opened, err := streamMember.Open()
		if err != nil {
			return Inspection{}, fmt.Errorf("open IPA member %q: %w", member.Name, err)
		}
		written, readErr := copyWithContext(ctx, io.Discard, io.LimitReader(opened, int64(remaining)+1), duringZIPStreamReadForTest)
		closeErr := opened.Close()
		if written < 0 || uint64(written) > remaining {
			return Inspection{}, fmt.Errorf("IPA expanded contents exceed %d bytes", maxArchiveExpandedBytes)
		}
		expandedBytes += uint64(written)
		if readErr != nil {
			return Inspection{}, fmt.Errorf("validate IPA member %q compressed data: %w", member.Name, readErr)
		}
		if closeErr != nil {
			return Inspection{}, fmt.Errorf("close IPA member %q: %w", member.Name, closeErr)
		}
		if uint64(written) != member.UncompressedSize64 {
			return Inspection{}, fmt.Errorf("IPA member %q expanded size does not match its declaration", member.Name)
		}
		if member.FileInfo().IsDir() && written != 0 {
			return Inspection{}, fmt.Errorf("IPA directory member %q contains data", member.Name)
		}
	}
	if len(infoFiles) == 0 {
		return Inspection{}, fmt.Errorf("IPA is missing Payload/*.app/Info.plist")
	}
	if len(infoFiles) != 1 {
		return Inspection{}, fmt.Errorf("IPA contains %d main apps; expected exactly one", len(infoFiles))
	}

	info, err := readInfoPlistContext(ctx, infoFiles[0])
	if err != nil {
		return Inspection{}, err
	}
	if err := validateIOSPlatform(info); err != nil {
		return Inspection{}, err
	}
	app := App{
		BundleID:         strings.TrimSpace(info.BundleID),
		Title:            firstNonempty(info.DisplayName, info.Name),
		Version:          strings.TrimSpace(info.Version),
		BuildNumber:      strings.TrimSpace(info.Build),
		MinimumOSVersion: strings.TrimSpace(info.MinimumOS),
	}
	if err := validateAppMetadata(app); err != nil {
		return Inspection{}, err
	}
	appDir := path.Dir(infoFiles[0].Name)
	profileName := appDir + "/embedded.mobileprovision"
	var profileFile *zip.File
	for _, member := range reader.File {
		if member.Name == profileName && !member.FileInfo().IsDir() {
			profileFile = member
			break
		}
	}

	result := Inspection{
		SchemaVersion:      "1",
		Platform:           "IOS",
		DistributionMethod: "release-testing",
		App:                app,
		Artifact:           Artifact{SizeBytes: size, SHA256: digest},
		Signing: Signing{
			ProfileClass: ProfileClassUnknown,
			ProfileIntegrityVerification: CodeSignatureVerification{
				Status: CodeSignatureNotVerified,
				Reason: "embedded provisioning profile CMS integrity was not verified",
			},
			ProfileTrustVerification: CodeSignatureVerification{
				Status: CodeSignatureNotVerified,
				Reason: "Apple provisioning profile trust chain and issuance were not verified",
			},
			CodeSignatureVerification: CodeSignatureVerification{
				Status: CodeSignatureNotVerified,
				Scope:  CodeSignatureScopeCompleteMainApp,
				Reason: "Mach-O code signature and signer/profile certificate binding were not verified",
			},
		},
		Preparation:     Preparation{Issues: []string{}},
		EmbeddedTargets: canonicalSet(embeddedTargets),
	}

	if profileFile == nil {
		result.Preparation.Issues = append(result.Preparation.Issues, "embedded provisioning profile is missing")
	} else {
		profile, err := readProfileContext(ctx, profileFile, effectiveNow(options.Now))
		if err != nil {
			return Inspection{}, err
		}
		devices := canonicalSet(profile.ProvisionedDevices)
		deviceSet := deviceset.Digest(profile.ProvisionedDevices)
		certificates := certificateFingerprints(profile.DeveloperCertificates)
		result.Signing = Signing{
			ProfileClass:                         profile.Class,
			ProfileUUID:                          strings.TrimSpace(profile.UUID),
			TeamID:                               onlyTrimmed(profile.TeamIdentifier),
			DeviceCount:                          deviceSet.Count,
			DeviceSetSHA256:                      deviceSet.SHA256,
			EmbeddedProfileSHA256:                profile.EmbeddedProfileSHA256,
			ProfileCertificateSHA256Fingerprints: certificates,
			ProfileIntegrityVerification: CodeSignatureVerification{
				Status: CodeSignatureVerified,
				Reason: "CMS signature integrity verified; signer trust was not evaluated",
			},
			ProfileTrustVerification: profile.ProfileTrustVerification,
			CodeSignatureVerification: CodeSignatureVerification{
				Status: CodeSignatureNotVerified,
				Scope:  CodeSignatureScopeCompleteMainApp,
				Reason: "Mach-O code signature and signer/profile certificate binding were not verified",
			},
		}
		if !profile.ExpirationDate.IsZero() {
			result.Signing.ExpiresAt = profile.ExpirationDate.UTC().Format(time.RFC3339)
		}
		if options.IncludeDevices {
			result.Signing.Devices = devices
		}
		result.Preparation.Issues = preparationIssues(result, profile, effectiveNow(options.Now))
		verification := verifyMainAppCodeSignatureContext(ctx, reader.File, appDir, info.Executable, app.BundleID, profile)
		if err := contextError(ctx); err != nil {
			return Inspection{}, err
		}
		result.Signing.CodeSignatureVerification = verification
	}
	result.Preparation.MetadataEligible = len(result.Preparation.Issues) == 0
	return result, nil
}

func validateArchiveMember(member *zip.File) error {
	name := member.Name
	if name == "" || len(name) > maxArchiveMemberNameLen || !utf8.ValidString(name) || strings.Contains(name, `\`) {
		return fmt.Errorf("IPA contains unsafe path %q", name)
	}
	for _, r := range name {
		if r == 0 || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.In(r, unicode.Bidi_Control) {
			return fmt.Errorf("IPA contains unsafe path %q", name)
		}
	}
	trimmed := strings.TrimSuffix(name, "/")
	if trimmed == "" || path.IsAbs(name) || path.Clean(trimmed) != trimmed || strings.HasPrefix(trimmed, "../") || trimmed == ".." {
		return fmt.Errorf("IPA contains non-canonical path %q", name)
	}
	if member.Flags&1 != 0 {
		return fmt.Errorf("IPA contains encrypted member %q", name)
	}
	if member.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("IPA contains symbolic link %q", name)
	}
	if !member.FileInfo().IsDir() && !member.Mode().IsRegular() {
		return fmt.Errorf("IPA contains non-regular member %q", name)
	}
	return nil
}

func validateAppMetadata(app App) error {
	if err := validateBundleIdentifier(app.BundleID); err != nil {
		return err
	}
	for _, field := range []struct {
		name  string
		value string
		limit int
	}{
		{name: "CFBundleIdentifier", value: app.BundleID, limit: 255},
		{name: "app title", value: app.Title, limit: 512},
		{name: "CFBundleShortVersionString", value: app.Version, limit: 64},
		{name: "CFBundleVersion", value: app.BuildNumber, limit: 64},
		{name: "MinimumOSVersion", value: app.MinimumOSVersion, limit: 64},
	} {
		if len(field.value) > field.limit {
			return fmt.Errorf("invalid %s: must be at most %d bytes", field.name, field.limit)
		}
		for _, r := range field.value {
			if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.In(r, unicode.Bidi_Control) {
				return fmt.Errorf("invalid %s: control or formatting characters are not allowed", field.name)
			}
		}
	}
	if err := validateBundleIdentifier(app.BundleID); err != nil {
		return err
	}
	return nil
}

func validateBundleIdentifier(value string) error {
	if value == "" {
		return nil
	}
	if len(value) > 255 {
		return fmt.Errorf("invalid CFBundleIdentifier: must be at most 255 bytes")
	}
	for _, component := range strings.Split(value, ".") {
		if component == "" {
			return fmt.Errorf("invalid CFBundleIdentifier: components must not be empty")
		}
		for index := 0; index < len(component); index++ {
			character := component[index]
			if (character < 'a' || character > 'z') &&
				(character < 'A' || character > 'Z') &&
				(character < '0' || character > '9') &&
				character != '-' {
				return fmt.Errorf("invalid CFBundleIdentifier: only ASCII alphanumeric characters, hyphens, and periods are allowed")
			}
		}
	}
	return nil
}

func validateIOSPlatform(info infoPlistPayload) error {
	platformName := strings.ToLower(strings.TrimSpace(info.PlatformName))
	if platformName == "" && len(info.SupportedPlatforms) == 0 {
		return fmt.Errorf("main app iOS platform metadata is missing")
	}
	if platformName != "" && platformName != "iphoneos" {
		return fmt.Errorf("main app iOS platform metadata has unsupported DTPlatformName")
	}
	for _, platform := range info.SupportedPlatforms {
		if strings.ToLower(strings.TrimSpace(platform)) != "iphoneos" {
			return fmt.Errorf("main app iOS platform metadata contains an unsupported CFBundleSupportedPlatforms value")
		}
	}
	return nil
}

func isMainAppMember(name, base string) bool {
	if path.Base(name) != base {
		return false
	}
	dir := path.Dir(name)
	return strings.HasSuffix(dir, ".app") && path.Dir(dir) == "Payload"
}

func isEmbeddedTargetInfoPlist(name string) bool {
	if path.Base(name) != "Info.plist" || !strings.HasPrefix(name, "Payload/") {
		return false
	}
	dir := path.Dir(name)
	if !strings.HasSuffix(dir, ".appex") && !strings.HasSuffix(dir, ".app") {
		return false
	}
	return len(strings.Split(dir, "/")) > 2
}

func readInfoPlistContext(ctx context.Context, member *zip.File) (infoPlistPayload, error) {
	if err := infoplist.CheckDeclaredSize(member.UncompressedSize64); err != nil {
		return infoPlistPayload{}, fmt.Errorf("read main app Info.plist: %w", err)
	}
	data, err := readZipMemberBoundedContext(ctx, member, infoplist.MaxBytes)
	if err != nil {
		return infoPlistPayload{}, fmt.Errorf("read main app Info.plist: %w", err)
	}
	if err := infoplist.ValidateStructure(data); err != nil {
		return infoPlistPayload{}, fmt.Errorf("decode main app Info.plist: %w", err)
	}
	var result infoPlistPayload
	if _, err := plist.Unmarshal(data, &result); err != nil {
		return infoPlistPayload{}, fmt.Errorf("decode main app Info.plist: %w", err)
	}
	return result, nil
}

func readProfileContext(ctx context.Context, member *zip.File, now time.Time) (parsedProfile, error) {
	if member.UncompressedSize64 > maxProfileBytes {
		return parsedProfile{}, fmt.Errorf("embedded provisioning profile declared size exceeds %d bytes", maxProfileBytes)
	}
	data, err := readZipMemberBoundedContext(ctx, member, maxProfileBytes)
	if err != nil {
		return parsedProfile{}, fmt.Errorf("read embedded provisioning profile: %w", err)
	}
	p7, err := pkcs7.Parse(data)
	if err != nil {
		return parsedProfile{}, fmt.Errorf("embedded provisioning profile is not signed CMS data: %w", err)
	}
	if len(p7.Content) == 0 {
		return parsedProfile{}, fmt.Errorf("embedded provisioning profile CMS content is empty")
	}
	if err := p7.Verify(); err != nil {
		return parsedProfile{}, fmt.Errorf("verify embedded provisioning profile CMS signature integrity: %w", err)
	}
	profileDigest := sha256.Sum256(data)
	trust := verifyAppleProfileTrust(p7, now, appleProfileRootFingerprints)
	if err := infoplist.ValidateStructure(p7.Content); err != nil {
		return parsedProfile{}, fmt.Errorf("validate embedded provisioning profile plist: %w", err)
	}
	var profile embeddedProfile
	if _, err := plist.Unmarshal(p7.Content, &profile); err != nil {
		return parsedProfile{}, fmt.Errorf("decode embedded provisioning profile: %w", err)
	}
	teamID := declaredSingle(profile.TeamIdentifier)
	prefix := declaredSingle(profile.ApplicationIdentifierPrefix)
	if teamID == "" || prefix == "" {
		return parsedProfile{}, fmt.Errorf("embedded provisioning profile must declare exactly one team and application identifier prefix")
	}
	if err := validateTeamIdentifier(teamID); err != nil {
		return parsedProfile{}, err
	}
	if err := validateApplicationIdentifierPrefix(prefix); err != nil {
		return parsedProfile{}, err
	}
	entitlementTeam, _ := profile.Entitlements["com.apple.developer.team-identifier"].(string)
	if strings.TrimSpace(entitlementTeam) != teamID {
		return parsedProfile{}, fmt.Errorf("embedded provisioning profile team identifier does not match its entitlement")
	}
	applicationID, _ := profile.Entitlements["application-identifier"].(string)
	wantPrefix := prefix + "."
	if !strings.HasPrefix(strings.TrimSpace(applicationID), wantPrefix) {
		return parsedProfile{}, fmt.Errorf("embedded provisioning profile application identifier does not match its declared prefix")
	}
	profileBundleID := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(applicationID), wantPrefix))
	if profileBundleID == "" {
		return parsedProfile{}, fmt.Errorf("embedded provisioning profile bundle identifier is empty")
	}
	debugValue, hasDebugValue := profile.Entitlements["get-task-allow"]
	debuggable, debugIsBool := debugValue.(bool)
	class := ProfileClassUnknown
	deviceCount := len(canonicalSet(profile.ProvisionedDevices))
	switch {
	case profile.ProvisionsAllDevices && deviceCount == 0:
		class = ProfileClassEnterprise
	case !profile.ProvisionsAllDevices && deviceCount > 0 && hasDebugValue && debugIsBool && debuggable:
		class = ProfileClassDevelopment
	case !profile.ProvisionsAllDevices && deviceCount > 0 && hasDebugValue && debugIsBool && !debuggable:
		class = ProfileClassAdHoc
	case !profile.ProvisionsAllDevices && deviceCount == 0 && hasDebugValue && debugIsBool && !debuggable:
		class = ProfileClassAppStore
	}
	return parsedProfile{
		embeddedProfile: profile, BundleID: profileBundleID, Class: class,
		EmbeddedProfileSHA256: hex.EncodeToString(profileDigest[:]), ProfileTrustVerification: trust,
	}, nil
}

func verifyAppleProfileTrust(profile *pkcs7.PKCS7, now time.Time, allowedRoots map[string]struct{}) CodeSignatureVerification {
	invalid := func(reason string) CodeSignatureVerification {
		return CodeSignatureVerification{Status: CodeSignatureInvalid, Reason: reason}
	}
	if len(profile.Signers) != 1 {
		return invalid("provisioning profile must contain exactly one CMS signer")
	}
	signer := profile.GetOnlySigner()
	if signer == nil || signer.Subject.CommonName != "Apple iPhone OS Provisioning Profile Signing" ||
		len(signer.Subject.Organization) != 1 || signer.Subject.Organization[0] != "Apple Inc." ||
		signer.Issuer.CommonName != "Apple iPhone Certification Authority" {
		return invalid("provisioning profile CMS signer is not the expected Apple provisioning signer")
	}
	pool := x509.NewCertPool()
	foundPinnedRoot := false
	for _, certificate := range profile.Certificates {
		if !certificate.IsCA || (!bytes.Equal(certificate.RawSubject, certificate.RawIssuer) && certificate.Subject.String() != certificate.Issuer.String()) {
			continue
		}
		sum := sha256.Sum256(certificate.Raw)
		if _, ok := allowedRoots[hex.EncodeToString(sum[:])]; !ok {
			continue
		}
		pool.AddCert(certificate)
		foundPinnedRoot = true
	}
	if !foundPinnedRoot {
		return invalid("provisioning profile chain does not contain a pinned Apple root")
	}
	if err := profile.VerifyWithChainAtTime(pool, now); err != nil {
		return invalid("provisioning profile does not chain to a pinned Apple root")
	}
	return CodeSignatureVerification{Status: CodeSignatureVerified, Reason: "Apple provisioning signer identity and chain verified to a pinned Apple root"}
}

func readZipMemberBoundedContext(ctx context.Context, member *zip.File, limit int64) ([]byte, error) {
	reader, err := member.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	var destination bytes.Buffer
	if limit < 1<<20 {
		destination.Grow(int(limit))
	}
	_, err = copyWithContext(ctx, &destination, io.LimitReader(reader, limit+1), nil)
	if err != nil {
		return nil, err
	}
	data := destination.Bytes()
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("expanded contents exceed %d bytes", limit)
	}
	return data, nil
}

func preparationIssues(result Inspection, profile parsedProfile, now time.Time) []string {
	issues := make([]string, 0)
	if result.App.BundleID == "" {
		issues = append(issues, "CFBundleIdentifier is missing")
	}
	if result.App.Title == "" {
		issues = append(issues, "app title is missing")
	}
	if result.App.Version == "" {
		issues = append(issues, "CFBundleShortVersionString is missing")
	}
	if result.App.BuildNumber == "" {
		issues = append(issues, "CFBundleVersion is missing")
	}
	if profile.Class != ProfileClassAdHoc {
		issues = append(issues, fmt.Sprintf("provisioning profile class is %s; expected ad-hoc", profile.Class))
	}
	if profile.ExpirationDate.IsZero() {
		issues = append(issues, "provisioning profile expiration is missing")
	} else if !profile.ExpirationDate.After(now) {
		issues = append(issues, "provisioning profile is expired")
	}
	if len(canonicalSet(profile.ProvisionedDevices)) == 0 {
		issues = append(issues, "provisioning profile contains no devices")
	}
	if !bundleMatches(profile.BundleID, result.App.BundleID) {
		issues = append(issues, "provisioning profile bundle identifier does not match the app")
	}
	if strings.TrimSpace(profile.UUID) == "" {
		issues = append(issues, "provisioning profile UUID is missing")
	}
	if len(certificateFingerprints(profile.DeveloperCertificates)) == 0 {
		issues = append(issues, "provisioning profile contains no signing certificates")
	}
	if !containsFold(profile.Platform, "iOS") {
		issues = append(issues, "provisioning profile does not include the iOS platform")
	}
	if len(result.EmbeddedTargets) > 0 {
		issues = append(issues, "embedded targets require target-by-target signing validation before preparation")
	}
	return issues
}

func bundleMatches(profileBundleID, appBundleID string) bool {
	profileBundleID = strings.TrimSpace(profileBundleID)
	appBundleID = strings.TrimSpace(appBundleID)
	if profileBundleID == appBundleID {
		return appBundleID != ""
	}
	if profileBundleID == "*" {
		return appBundleID != ""
	}
	if strings.HasSuffix(profileBundleID, ".*") {
		prefix := strings.TrimSuffix(profileBundleID, "*")
		return strings.HasPrefix(appBundleID, prefix) && len(appBundleID) > len(prefix)
	}
	return false
}

func certificateFingerprints(certificates [][]byte) []string {
	result := make([]string, 0, len(certificates))
	for _, data := range certificates {
		if _, err := x509.ParseCertificate(data); err != nil {
			continue
		}
		sum := sha256.Sum256(data)
		result = append(result, hex.EncodeToString(sum[:]))
	}
	return canonicalSet(result)
}

func canonicalSet(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			set[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func onlyTrimmed(values []string) string {
	values = canonicalSet(values)
	if len(values) != 1 {
		return ""
	}
	return values[0]
}

func declaredSingle(values []string) string {
	if len(values) != 1 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func containsFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), want) {
			return true
		}
	}
	return false
}

func firstNonempty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func effectiveNow(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now()
	}
	return now
}
