package distribution

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"howett.net/plist"
)

var (
	// ErrPrivatePublishLinkExpired means an immutable private install lease can
	// no longer be verified and must be replaced by a newly authorized run.
	ErrPrivatePublishLinkExpired = errors.New("private publication link expired")
	// ErrPrivatePublishProfileExpired means the signed payload is no longer
	// usable for publication and must be rebuilt under a new plan.
	ErrPrivatePublishProfileExpired = errors.New("private publication profile expired")
	// ErrPrivatePublishConflict means exact immutable local or remote evidence
	// conflicts with the saved publication intent. Retrying cannot repair it.
	ErrPrivatePublishConflict = errors.New("private publication intent conflict")
	// ErrImmutableObjectConflict means an existing object at an exact key does
	// not match the expected immutable content identity.
	ErrImmutableObjectConflict = errors.New("immutable object conflict")
	// ErrVerificationContentConflict means a successful fetch returned bytes or
	// representation metadata that conflict with the immutable expected object.
	// Retrying cannot make that exact destination safe to reuse.
	ErrVerificationContentConflict = errors.New("published object content conflict")
)

const (
	ContentTypeIPA      = "application/octet-stream"
	ContentTypeManifest = "application/xml"
	ContentTypeHTML     = "text/html; charset=utf-8"
	maxLinkLifetime     = 7 * 24 * time.Hour
	maxDescriptorBytes  = 1 << 20
)

var ErrObjectVerificationMismatch = errors.New("published object verification mismatch")

type Access string

const (
	AccessPrivate Access = "private"
	AccessPublic  Access = "public"
)

type PreparedDescriptor struct {
	SchemaVersion      string           `json:"schemaVersion"`
	Platform           string           `json:"platform"`
	DistributionMethod string           `json:"distributionMethod"`
	App                PreparedApp      `json:"app"`
	Artifact           PreparedArtifact `json:"artifact"`
	Signing            PreparedSigning  `json:"signing"`
	EmbeddedTargets    []string         `json:"embeddedTargets,omitempty"`
}

type PreparedApp struct {
	BundleID         string `json:"bundleId"`
	Title            string `json:"title"`
	Version          string `json:"version"`
	BuildNumber      string `json:"buildNumber"`
	MinimumOSVersion string `json:"minimumOSVersion,omitempty"`
}

type PreparedArtifact struct {
	RelativePath string `json:"relativePath"`
	SHA256       string `json:"sha256"`
	SizeBytes    int64  `json:"sizeBytes"`
}

type PreparedSigning struct {
	ProfileClass                         string                            `json:"profileClass"`
	ProfileUUID                          string                            `json:"profileUuid"`
	EmbeddedProfileSHA256                string                            `json:"embeddedProfileSha256"`
	TeamID                               string                            `json:"teamId"`
	ExpiresAt                            string                            `json:"expiresAt"`
	DeviceCount                          int                               `json:"deviceCount"`
	DeviceSetSHA256                      string                            `json:"deviceSetSha256"`
	ProfileCertificateSHA256Fingerprints []string                          `json:"profileCertificateSha256Fingerprints"`
	CodeSignatureVerification            PreparedCodeSignatureVerification `json:"codeSignatureVerification"`
	ProfileIntegrityVerification         PreparedCodeSignatureVerification `json:"profileIntegrityVerification"`
	ProfileTrustVerification             PreparedCodeSignatureVerification `json:"profileTrustVerification"`
}

type PreparedCodeSignatureVerification struct {
	Status                              string   `json:"status"`
	Reason                              string   `json:"reason,omitempty"`
	Scope                               string   `json:"scope,omitempty"`
	SignerCertificateSHA256Fingerprints []string `json:"signerCertificateSha256Fingerprints,omitempty"`
}

func (signing *PreparedSigning) UnmarshalJSON(data []byte) error {
	type plain PreparedSigning
	var wire struct {
		plain
		LegacyCertificateSHA256Fingerprints []string `json:"certificateSha256Fingerprints"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*signing = PreparedSigning(wire.plain)
	if len(signing.ProfileCertificateSHA256Fingerprints) > 0 && len(wire.LegacyCertificateSHA256Fingerprints) > 0 && !slices.Equal(signing.ProfileCertificateSHA256Fingerprints, wire.LegacyCertificateSHA256Fingerprints) {
		return fmt.Errorf("signing profileCertificateSha256Fingerprints conflicts with certificateSha256Fingerprints")
	}
	if len(signing.ProfileCertificateSHA256Fingerprints) == 0 {
		signing.ProfileCertificateSHA256Fingerprints = wire.LegacyCertificateSHA256Fingerprints
	}
	return nil
}

type PreparedBundle struct {
	Descriptor       PreparedDescriptor
	DescriptorSHA256 string
	DescriptorSize   int64
	IPA              *os.File
	IPASHA256        string
	IPASize          int64
}

func LoadPreparedBundle(bundleDir string) (*PreparedBundle, error) {
	trimmed := strings.TrimSpace(bundleDir)
	if trimmed == "" {
		return nil, fmt.Errorf("bundle directory is required")
	}
	root, err := rootfs.New(trimmed)
	if err != nil {
		return nil, fmt.Errorf("open prepared bundle: %w", err)
	}
	defer root.Close()
	return LoadPreparedBundleContext(context.Background(), root)
}

// LoadPreparedBundleContext reads a prepared bundle through an already-pinned
// root. The caller owns root and must retain it for the complete publication
// workflow so a lexical path replacement cannot select a different bundle.
func LoadPreparedBundleContext(ctx context.Context, root rootfs.Root) (*PreparedBundle, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	descriptorFile, err := root.OpenFile("bundle.json")
	if err != nil {
		return nil, fmt.Errorf("read bundle.json: %w", err)
	}
	descriptorInfo, err := descriptorFile.Stat()
	if err != nil {
		_ = descriptorFile.Close()
		return nil, fmt.Errorf("stat bundle.json: %w", err)
	}
	if descriptorInfo.Size() > maxDescriptorBytes {
		_ = descriptorFile.Close()
		return nil, fmt.Errorf("bundle.json exceeds %d bytes", maxDescriptorBytes)
	}
	if !descriptorInfo.Mode().IsRegular() {
		_ = descriptorFile.Close()
		return nil, fmt.Errorf("bundle.json is not a regular file")
	}
	descriptorBytes, err := io.ReadAll(io.LimitReader(descriptorFile, maxDescriptorBytes+1))
	if err != nil || len(descriptorBytes) > maxDescriptorBytes {
		_ = descriptorFile.Close()
		return nil, fmt.Errorf("read bounded bundle.json")
	}
	descriptorAfterRead, err := descriptorFile.Stat()
	_ = descriptorFile.Close()
	if err != nil || !stablePreparedFile(descriptorInfo, descriptorAfterRead) || int64(len(descriptorBytes)) != descriptorInfo.Size() {
		return nil, fmt.Errorf("bundle.json changed while it was read")
	}
	descriptorDigest := sha256.Sum256(descriptorBytes)
	descriptorSHA := hex.EncodeToString(descriptorDigest[:])
	var descriptor PreparedDescriptor
	decoder := json.NewDecoder(strings.NewReader(string(descriptorBytes)))
	if err := decoder.Decode(&descriptor); err != nil {
		return nil, fmt.Errorf("decode bundle.json: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("decode bundle.json: trailing content")
	}
	if err := validateDescriptor(descriptor); err != nil {
		return nil, err
	}
	artifactPath := descriptor.Artifact.RelativePath
	if artifactPath != "payload/app.ipa" {
		return nil, fmt.Errorf("bundle.json artifact.relativePath must be payload/app.ipa")
	}
	ipa, err := root.OpenFile(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("open payload/app.ipa: %w", err)
	}
	valid := false
	defer func() {
		if !valid {
			_ = ipa.Close()
		}
	}()
	info, err := ipa.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat payload/app.ipa: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("payload/app.ipa is not a regular file")
	}
	if info.Size() != descriptor.Artifact.SizeBytes {
		return nil, fmt.Errorf("payload/app.ipa size %d does not match bundle.json %d", info.Size(), descriptor.Artifact.SizeBytes)
	}
	if info.Size() > MaxIPABytes {
		return nil, fmt.Errorf("payload/app.ipa exceeds %d bytes", MaxIPABytes)
	}
	digest := sha256.New()
	written, err := copyWithContext(ctx, digest, io.LimitReader(ipa, MaxIPABytes+1), nil)
	if err != nil {
		return nil, fmt.Errorf("hash payload/app.ipa: %w", err)
	}
	if written != info.Size() {
		return nil, fmt.Errorf("payload/app.ipa size changed while hashing: read %d of %d bytes", written, info.Size())
	}
	infoAfterHash, err := ipa.Stat()
	if err != nil || !stablePreparedFile(info, infoAfterHash) {
		return nil, fmt.Errorf("payload/app.ipa changed while it was hashed")
	}
	actualSHA := hex.EncodeToString(digest.Sum(nil))
	if actualSHA != strings.ToLower(descriptor.Artifact.SHA256) {
		return nil, fmt.Errorf("payload/app.ipa SHA-256 %s does not match bundle.json %s", actualSHA, descriptor.Artifact.SHA256)
	}
	if _, err := ipa.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind payload/app.ipa: %w", err)
	}
	valid = true
	return &PreparedBundle{
		Descriptor: descriptor, DescriptorSHA256: descriptorSHA, DescriptorSize: descriptorInfo.Size(),
		IPA: ipa, IPASHA256: actualSHA, IPASize: info.Size(),
	}, nil
}

func validateDescriptor(descriptor PreparedDescriptor) error {
	if descriptor.SchemaVersion != "1" {
		return fmt.Errorf("bundle.json schemaVersion %q is not supported", descriptor.SchemaVersion)
	}
	if descriptor.Platform != "IOS" {
		return fmt.Errorf("bundle.json platform must be IOS")
	}
	if descriptor.DistributionMethod != "release-testing" {
		return fmt.Errorf("bundle.json distributionMethod must be release-testing")
	}
	if strings.TrimSpace(descriptor.App.BundleID) == "" || strings.TrimSpace(descriptor.App.Title) == "" || strings.TrimSpace(descriptor.App.Version) == "" || strings.TrimSpace(descriptor.App.BuildNumber) == "" {
		return fmt.Errorf("bundle.json app requires bundleId, title, version, and buildNumber")
	}
	for field, value := range map[string]string{"bundleId": descriptor.App.BundleID, "title": descriptor.App.Title, "version": descriptor.App.Version, "buildNumber": descriptor.App.BuildNumber} {
		if containsUnsafeText(value) {
			return fmt.Errorf("bundle.json app.%s contains control or bidi characters", field)
		}
		if len([]rune(value)) > 512 {
			return fmt.Errorf("bundle.json app.%s exceeds 512 characters", field)
		}
	}
	digest, err := hex.DecodeString(descriptor.Artifact.SHA256)
	if err != nil || len(digest) != sha256.Size {
		return fmt.Errorf("bundle.json artifact.sha256 must be a 64-character hexadecimal SHA-256")
	}
	if descriptor.Artifact.SizeBytes <= 0 {
		return fmt.Errorf("bundle.json artifact.sizeBytes must be positive")
	}
	if descriptor.Artifact.SizeBytes > MaxIPABytes {
		return fmt.Errorf("bundle.json artifact.sizeBytes exceeds %d", MaxIPABytes)
	}
	if len(descriptor.EmbeddedTargets) > 0 {
		return fmt.Errorf("bundle.json contains embedded targets that this publisher cannot validate")
	}
	if descriptor.Signing.ProfileClass != "ad-hoc" {
		return fmt.Errorf("bundle.json signing.profileClass must be ad-hoc")
	}
	if strings.TrimSpace(descriptor.Signing.ProfileUUID) == "" || strings.TrimSpace(descriptor.Signing.TeamID) == "" || strings.TrimSpace(descriptor.Signing.ExpiresAt) == "" {
		return fmt.Errorf("bundle.json signing requires profileUuid, teamId, and expiresAt")
	}
	if descriptor.Signing.DeviceCount <= 0 || !isCanonicalDigest(descriptor.Signing.DeviceSetSHA256) || !isCanonicalDigest(descriptor.Signing.EmbeddedProfileSHA256) || len(descriptor.Signing.ProfileCertificateSHA256Fingerprints) == 0 {
		return fmt.Errorf("bundle.json signing requires a non-empty device set, embedded profile digest, and profile certificate fingerprints")
	}
	if descriptor.Signing.CodeSignatureVerification.Status != "verified" {
		return fmt.Errorf("bundle.json signing.codeSignatureVerification.status must be verified")
	}
	if descriptor.Signing.CodeSignatureVerification.Scope != "complete-main-app-code-resources-entitlements-and-profile-certificate-binding" {
		return fmt.Errorf("bundle.json signing.codeSignatureVerification.scope must be complete-main-app-code-resources-entitlements-and-profile-certificate-binding")
	}
	if len(descriptor.Signing.CodeSignatureVerification.SignerCertificateSHA256Fingerprints) == 0 {
		return fmt.Errorf("bundle.json signing.codeSignatureVerification requires signerCertificateSha256Fingerprints")
	}
	profileFingerprints := make(map[string]struct{}, len(descriptor.Signing.ProfileCertificateSHA256Fingerprints))
	for _, fingerprint := range descriptor.Signing.ProfileCertificateSHA256Fingerprints {
		if !isCanonicalDigest(fingerprint) {
			return fmt.Errorf("bundle.json signing.profileCertificateSha256Fingerprints must contain canonical SHA-256 values")
		}
		profileFingerprints[fingerprint] = struct{}{}
	}
	for _, fingerprint := range descriptor.Signing.CodeSignatureVerification.SignerCertificateSHA256Fingerprints {
		if !isCanonicalDigest(fingerprint) {
			return fmt.Errorf("bundle.json signing.codeSignatureVerification.signerCertificateSha256Fingerprints must contain canonical SHA-256 values")
		}
		if _, ok := profileFingerprints[fingerprint]; !ok {
			return fmt.Errorf("bundle.json signer certificate fingerprint is not present in the provisioning profile")
		}
	}
	if descriptor.Signing.ProfileIntegrityVerification.Status != "verified" {
		return fmt.Errorf("bundle.json signing.profileIntegrityVerification.status must be verified")
	}
	if descriptor.Signing.ProfileTrustVerification.Status != "verified" {
		return fmt.Errorf("bundle.json signing.profileTrustVerification.status must be verified")
	}
	expiresAt, err := time.Parse(time.RFC3339, descriptor.Signing.ExpiresAt)
	if err != nil {
		return fmt.Errorf("bundle.json signing.expiresAt must be RFC3339: %w", err)
	}
	if !expiresAt.After(time.Now()) {
		return fmt.Errorf("bundle.json signing profile is expired")
	}
	if descriptor.Artifact.RelativePath != "payload/app.ipa" {
		return fmt.Errorf("bundle.json artifact.relativePath must be payload/app.ipa")
	}
	return nil
}

type PutObject struct {
	Key         string
	Body        io.Reader
	SHA256      string
	SizeBytes   int64
	ContentType string
}

type StoredObject struct {
	Key         string `json:"key"`
	SHA256      string `json:"sha256"`
	SizeBytes   int64  `json:"sizeBytes"`
	ContentType string `json:"contentType"`
	Status      string `json:"status"`
	entityTag   string
}

type ObjectStore interface {
	Ensure(context.Context, PutObject) (StoredObject, error)
	PresignGet(context.Context, string, time.Duration) (string, error)
}

type CorruptObjectReplacer interface {
	ReplaceCorrupt(context.Context, PutObject) (StoredObject, error)
}

type VerifyKind int

const (
	VerifyDocument VerifyKind = iota
	VerifyIPA
)

type Verifier interface {
	Verify(context.Context, VerifyRequest) error
}

type VerifyRequest struct {
	URL         string
	Kind        VerifyKind
	SHA256      string
	SizeBytes   int64
	ContentType string
}

type PublishOptions struct {
	Store           ObjectStore
	Verifier        Verifier
	Bucket          string
	Prefix          string
	Access          Access
	PublicBaseURL   string
	URLTTL          time.Duration
	DownloadGrace   time.Duration
	CredentialLimit time.Time
	Now             func() time.Time
	RandomID        func() (string, error)
}

type PublishReceipt struct {
	SchemaVersion    string         `json:"schemaVersion"`
	Endpoint         string         `json:"endpoint"`
	DownloadEndpoint string         `json:"downloadEndpoint,omitempty"`
	PublicBaseURL    string         `json:"publicBaseUrl,omitempty"`
	Region           string         `json:"region"`
	AddressingStyle  string         `json:"addressingStyle"`
	Access           Access         `json:"access"`
	Bucket           string         `json:"bucket"`
	Prefix           string         `json:"prefix"`
	URLTTL           string         `json:"urlTtl,omitempty"`
	DownloadGrace    string         `json:"downloadGrace,omitempty"`
	Artifact         StoredObject   `json:"artifact"`
	Manifest         StoredObject   `json:"manifest"`
	Page             StoredObject   `json:"page"`
	InstallURL       string         `json:"installUrl"`
	DirectInstallURL string         `json:"directInstallUrl"`
	ExpiresAt        *time.Time     `json:"expiresAt,omitempty"`
	Verified         bool           `json:"verified"`
	App              PreparedApp    `json:"app"`
	Signing          ReceiptSigning `json:"signing"`
	ReceiptPath      string         `json:"receiptPath,omitempty"`
	LinkPath         string         `json:"linkPath,omitempty"`
}

type ReceiptSigning struct {
	ProfileClass                   string                            `json:"profileClass"`
	ProfileUUID                    string                            `json:"profileUuid"`
	EmbeddedProfileSHA256          string                            `json:"embeddedProfileSha256"`
	TeamID                         string                            `json:"teamId"`
	ProfileExpiresAt               string                            `json:"profileExpiresAt"`
	DeviceCount                    int                               `json:"deviceCount"`
	DeviceSetSHA256                string                            `json:"deviceSetSha256"`
	ProfileCertificateFingerprints []string                          `json:"profileCertificateSha256Fingerprints"`
	ProfileIntegrityVerification   PreparedCodeSignatureVerification `json:"profileIntegrityVerification"`
	ProfileTrustVerification       PreparedCodeSignatureVerification `json:"profileTrustVerification"`
	CodeSignatureVerification      PreparedCodeSignatureVerification `json:"codeSignatureVerification"`
}

func receiptSigningFromPrepared(signing PreparedSigning) ReceiptSigning {
	return ReceiptSigning{
		ProfileClass: signing.ProfileClass, ProfileUUID: signing.ProfileUUID, TeamID: signing.TeamID,
		EmbeddedProfileSHA256: signing.EmbeddedProfileSHA256,
		ProfileExpiresAt:      signing.ExpiresAt, DeviceCount: signing.DeviceCount, DeviceSetSHA256: signing.DeviceSetSHA256,
		ProfileCertificateFingerprints: append([]string(nil), signing.ProfileCertificateSHA256Fingerprints...),
		ProfileIntegrityVerification:   signing.ProfileIntegrityVerification, ProfileTrustVerification: signing.ProfileTrustVerification,
		CodeSignatureVerification: signing.CodeSignatureVerification,
	}
}

// MatchesPrepared reports whether receipt signing facts exactly bind to a
// prepared descriptor without consuming raw device identifiers.
func (signing ReceiptSigning) MatchesPrepared(prepared PreparedSigning) bool {
	want := receiptSigningFromPrepared(prepared)
	return signing.ProfileClass == want.ProfileClass && signing.ProfileUUID == want.ProfileUUID && signing.EmbeddedProfileSHA256 == want.EmbeddedProfileSHA256 && signing.TeamID == want.TeamID &&
		signing.ProfileExpiresAt == want.ProfileExpiresAt && signing.DeviceCount == want.DeviceCount && signing.DeviceSetSHA256 == want.DeviceSetSHA256 &&
		slices.Equal(signing.ProfileCertificateFingerprints, want.ProfileCertificateFingerprints) &&
		verificationMatches(signing.ProfileIntegrityVerification, want.ProfileIntegrityVerification) && verificationMatches(signing.ProfileTrustVerification, want.ProfileTrustVerification) &&
		verificationMatches(signing.CodeSignatureVerification, want.CodeSignatureVerification)
}

func verificationMatches(left, right PreparedCodeSignatureVerification) bool {
	return left.Status == right.Status && left.Reason == right.Reason && left.Scope == right.Scope && slices.Equal(left.SignerCertificateSHA256Fingerprints, right.SignerCertificateSHA256Fingerprints)
}

type SensitiveLinks struct {
	SchemaVersion    string     `json:"schemaVersion"`
	InstallURL       string     `json:"installUrl"`
	DirectInstallURL string     `json:"directInstallUrl"`
	ArtifactURL      string     `json:"artifactUrl"`
	ManifestURL      string     `json:"manifestUrl"`
	ExpiresAt        *time.Time `json:"expiresAt,omitempty"`
}

func Publish(ctx context.Context, ipa io.ReadSeeker, descriptor PreparedDescriptor, options PublishOptions) (PublishReceipt, SensitiveLinks, error) {
	if options.Store == nil || options.Verifier == nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("object store and verifier are required")
	}
	if err := validateDescriptor(descriptor); err != nil {
		return PublishReceipt{}, SensitiveLinks{}, err
	}
	prefix, err := NormalizePrefix(options.Prefix)
	if err != nil {
		return PublishReceipt{}, SensitiveLinks{}, err
	}
	if err := ValidateBucket(options.Bucket); err != nil {
		return PublishReceipt{}, SensitiveLinks{}, err
	}
	if options.Access != AccessPrivate && options.Access != AccessPublic {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("access must be private or public")
	}
	if options.Access == AccessPublic {
		if strings.TrimSpace(options.PublicBaseURL) == "" {
			return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("public base URL is required for public access")
		}
		if _, err := ValidatePublicBaseURL(options.PublicBaseURL); err != nil {
			return PublishReceipt{}, SensitiveLinks{}, err
		}
	}
	clock := func() time.Time { return time.Now().UTC() }
	if options.Now != nil {
		clock = func() time.Time { return options.Now().UTC() }
	}
	now := clock()
	profileExpiry, _ := time.Parse(time.RFC3339, descriptor.Signing.ExpiresAt)
	urlTTL, grace, err := boundedLifetimes(now, options.URLTTL, options.DownloadGrace, options.CredentialLimit, options.Access)
	if err != nil {
		return PublishReceipt{}, SensitiveLinks{}, err
	}
	requiredProfileValidity := time.Minute
	if options.Access == AccessPrivate {
		requiredProfileValidity += urlTTL + grace
	}
	if !profileExpiry.After(now.Add(requiredProfileValidity)) {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("signing profile expires too soon for the requested link lifetime and safety margin")
	}
	if ipa == nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("IPA is required")
	}
	if _, err := ipa.Seek(0, io.SeekStart); err != nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("rewind IPA for snapshot: %w", err)
	}
	ipaSnapshot, ipaDigest, cleanupSnapshot, err := snapshotReaderContext(ctx, io.LimitReader(ipa, descriptor.Artifact.SizeBytes+1), descriptor.Artifact.SizeBytes)
	if err != nil {
		return PublishReceipt{}, SensitiveLinks{}, err
	}
	defer cleanupSnapshot()
	if ipaDigest != strings.ToLower(descriptor.Artifact.SHA256) {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("IPA snapshot SHA-256 %s does not match bundle.json %s", ipaDigest, descriptor.Artifact.SHA256)
	}
	ipa = ipaSnapshot
	randomID := randomLinkID
	if options.RandomID != nil {
		randomID = options.RandomID
	}
	linkID, err := randomID()
	if err != nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("generate link ID: %w", err)
	}

	ipaKey := path.Join(prefix, "objects", "sha256", strings.ToLower(descriptor.Artifact.SHA256)+".ipa")
	if _, err := ipa.Seek(0, io.SeekStart); err != nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("rewind IPA: %w", err)
	}
	ipaObject, err := options.Store.Ensure(ctx, PutObject{Key: ipaKey, Body: ipa, SHA256: strings.ToLower(descriptor.Artifact.SHA256), SizeBytes: descriptor.Artifact.SizeBytes, ContentType: ContentTypeIPA})
	if err != nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("publish IPA: %w", err)
	}
	pageDeadline := now.Add(urlTTL)
	downloadDeadline := pageDeadline.Add(grace)
	ipaURL, err := resolveObjectURL(ctx, options, ipaKey, downloadDeadline, clock)
	if err != nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("create IPA URL: %w", err)
	}
	ipaVerification := VerifyRequest{URL: ipaURL, Kind: VerifyIPA, SHA256: ipaObject.SHA256, SizeBytes: ipaObject.SizeBytes, ContentType: ipaObject.ContentType}
	if verifyErr := options.Verifier.Verify(ctx, ipaVerification); verifyErr != nil {
		replacer, canReplace := options.Store.(CorruptObjectReplacer)
		if !canReplace || !errors.Is(verifyErr, ErrObjectVerificationMismatch) {
			return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("verify IPA: %w", verifyErr)
		}
		if _, err := ipa.Seek(0, io.SeekStart); err != nil {
			return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("rewind IPA for conditional replacement: %w", err)
		}
		ipaObject, err = replacer.ReplaceCorrupt(ctx, PutObject{
			Key: ipaKey, Body: ipa, SHA256: strings.ToLower(descriptor.Artifact.SHA256),
			SizeBytes: descriptor.Artifact.SizeBytes, ContentType: ContentTypeIPA,
		})
		if err != nil {
			return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("replace corrupt IPA after verification mismatch: %w", err)
		}
		ipaURL, err = resolveObjectURL(ctx, options, ipaKey, downloadDeadline, clock)
		if err != nil {
			return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("create replacement IPA URL: %w", err)
		}
		ipaVerification.URL = ipaURL
		ipaVerification.SHA256 = ipaObject.SHA256
		ipaVerification.SizeBytes = ipaObject.SizeBytes
		ipaVerification.ContentType = ipaObject.ContentType
		if err := options.Verifier.Verify(ctx, ipaVerification); err != nil {
			return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("verify replaced IPA: %w", err)
		}
	}

	if err := requireHTTPSURL(ipaURL); err != nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("IPA URL: %w", err)
	}
	manifestBytes, err := makeManifest(descriptor.App, ipaURL)
	if err != nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("generate manifest: %w", err)
	}
	manifestKey := path.Join(prefix, "links", linkID, "manifest.plist")
	manifestObject, err := ensureBytes(ctx, options.Store, manifestKey, manifestBytes, ContentTypeManifest)
	if err != nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("publish manifest: %w", err)
	}
	manifestURL, err := resolveObjectURL(ctx, options, manifestKey, downloadDeadline, clock)
	if err != nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("create manifest URL: %w", err)
	}
	if err := requireHTTPSURL(manifestURL); err != nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("manifest URL: %w", err)
	}
	if err := options.Verifier.Verify(ctx, VerifyRequest{URL: manifestURL, Kind: VerifyDocument, SHA256: manifestObject.SHA256, SizeBytes: manifestObject.SizeBytes, ContentType: manifestObject.ContentType}); err != nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("verify manifest: %w", err)
	}
	directInstallURL := "itms-services://?action=download-manifest&url=" + url.QueryEscape(manifestURL)

	pageBytes, err := makeInstallPage(descriptor.App, directInstallURL)
	if err != nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("generate install page: %w", err)
	}
	pageKey := path.Join(prefix, "links", linkID, "index.html")
	pageObject, err := ensureBytes(ctx, options.Store, pageKey, pageBytes, ContentTypeHTML)
	if err != nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("publish install page: %w", err)
	}
	installURL, err := resolveObjectURL(ctx, options, pageKey, pageDeadline, clock)
	if err != nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("create install page URL: %w", err)
	}
	if err := requireHTTPSURL(installURL); err != nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("install page URL: %w", err)
	}
	if err := options.Verifier.Verify(ctx, VerifyRequest{URL: installURL, Kind: VerifyDocument, SHA256: pageObject.SHA256, SizeBytes: pageObject.SizeBytes, ContentType: pageObject.ContentType}); err != nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("verify install page: %w", err)
	}
	if options.Access == AccessPublic && !profileExpiry.After(clock().Add(time.Minute)) {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("signing profile expires too soon to complete publication with the required safety margin")
	}

	var expiresAt *time.Time
	if options.Access == AccessPrivate {
		expiresAt = &pageDeadline
	}
	sensitive := SensitiveLinks{SchemaVersion: "1", InstallURL: installURL, DirectInstallURL: directInstallURL, ArtifactURL: ipaURL, ManifestURL: manifestURL, ExpiresAt: expiresAt}
	receipt := PublishReceipt{
		SchemaVersion: "1", Access: options.Access, Bucket: options.Bucket, Prefix: prefix,
		Artifact: ipaObject, Manifest: manifestObject, Page: pageObject,
		InstallURL: redactBearerURL(installURL), DirectInstallURL: redactDirectInstallURL(directInstallURL),
		ExpiresAt: expiresAt, Verified: true, App: descriptor.App,
		Signing: receiptSigningFromPrepared(descriptor.Signing),
	}
	if options.Access == AccessPublic {
		publicBase, _ := ValidatePublicBaseURL(options.PublicBaseURL)
		receipt.PublicBaseURL = publicBase.String()
	}
	return receipt, sensitive, nil
}

func Reverify(ctx context.Context, verifier Verifier, receipt PublishReceipt, links SensitiveLinks, now time.Time) error {
	return reverifyWithClock(ctx, verifier, receipt, links, now, func() time.Time { return time.Now().UTC() })
}

func reverifyWithClock(ctx context.Context, verifier Verifier, receipt PublishReceipt, links SensitiveLinks, now time.Time, clock func() time.Time) error {
	if verifier == nil || receipt.SchemaVersion != "1" || links.SchemaVersion != "1" || !receipt.Verified {
		return fmt.Errorf("invalid recovery verification input")
	}
	profileExpiry, err := time.Parse(time.RFC3339, receipt.Signing.ProfileExpiresAt)
	if err != nil || !profileExpiry.After(now.Add(time.Minute)) {
		return fmt.Errorf("%w: saved signing profile is expired or expires too soon", ErrPrivatePublishProfileExpired)
	}
	if receipt.Access == AccessPrivate && (links.ExpiresAt == nil || !links.ExpiresAt.After(now)) {
		return fmt.Errorf("%w: saved install link is expired", ErrPrivatePublishLinkExpired)
	}
	if links.ExpiresAt != nil && !profileExpiry.After(links.ExpiresAt.Add(time.Minute)) {
		return fmt.Errorf("saved install link exceeds the signing profile expiry")
	}
	if (receipt.ExpiresAt == nil) != (links.ExpiresAt == nil) || (receipt.ExpiresAt != nil && !receipt.ExpiresAt.Equal(*links.ExpiresAt)) {
		return fmt.Errorf("saved link expiry conflicts with receipt")
	}
	if redactBearerURL(links.InstallURL) != receipt.InstallURL || redactDirectInstallURL(links.DirectInstallURL) != receipt.DirectInstallURL {
		return fmt.Errorf("saved sensitive links conflict with redacted receipt")
	}
	direct, err := url.Parse(links.DirectInstallURL)
	if err != nil || direct.Scheme != "itms-services" || direct.Host != "" || direct.Query().Get("action") != "download-manifest" || direct.Query().Get("url") != links.ManifestURL {
		return fmt.Errorf("saved direct install URL does not reference the saved manifest URL")
	}
	if err := linkMatchesDestination(links.ArtifactURL, receipt.Artifact.Key, receipt); err != nil {
		return fmt.Errorf("saved artifact URL: %w", err)
	}
	if err := linkMatchesDestination(links.ManifestURL, receipt.Manifest.Key, receipt); err != nil {
		return fmt.Errorf("saved manifest URL: %w", err)
	}
	if err := linkMatchesDestination(links.InstallURL, receipt.Page.Key, receipt); err != nil {
		return fmt.Errorf("saved install URL: %w", err)
	}
	if receipt.Access == AccessPrivate {
		grace, err := time.ParseDuration(receipt.DownloadGrace)
		if err != nil || grace < 0 || grace > maxLinkLifetime {
			return fmt.Errorf("saved publication has invalid download grace")
		}
		pageDeadline := *receipt.ExpiresAt
		downloadDeadline := pageDeadline.Add(grace)
		for _, link := range []struct {
			name     string
			rawURL   string
			deadline time.Time
		}{
			{name: "artifact", rawURL: links.ArtifactURL, deadline: downloadDeadline},
			{name: "manifest", rawURL: links.ManifestURL, deadline: downloadDeadline},
			{name: "install", rawURL: links.InstallURL, deadline: pageDeadline},
		} {
			if err := privateSignatureWithinDeadline(link.rawURL, link.deadline); err != nil {
				return fmt.Errorf("saved %s URL: %w", link.name, err)
			}
		}
	}
	for _, object := range []struct {
		name     string
		stored   StoredObject
		expected string
	}{
		{name: "artifact", stored: receipt.Artifact, expected: ContentTypeIPA},
		{name: "manifest", stored: receipt.Manifest, expected: ContentTypeManifest},
		{name: "install page", stored: receipt.Page, expected: ContentTypeHTML},
	} {
		if !equivalentContentType(object.stored.ContentType, object.expected) {
			return fmt.Errorf("saved %s has invalid content type", object.name)
		}
	}
	manifest, err := makeManifest(receipt.App, links.ArtifactURL)
	if err != nil || !objectMatchesBytes(receipt.Manifest, manifest, ContentTypeManifest) {
		return fmt.Errorf("saved manifest identity does not match generated content")
	}
	page, err := makeInstallPage(receipt.App, links.DirectInstallURL)
	if err != nil || !objectMatchesBytes(receipt.Page, page, ContentTypeHTML) {
		return fmt.Errorf("saved install-page identity does not match generated content")
	}
	checks := []VerifyRequest{
		{URL: links.ArtifactURL, Kind: VerifyIPA, SHA256: receipt.Artifact.SHA256, SizeBytes: receipt.Artifact.SizeBytes, ContentType: receipt.Artifact.ContentType},
		{URL: links.ManifestURL, Kind: VerifyDocument, SHA256: receipt.Manifest.SHA256, SizeBytes: receipt.Manifest.SizeBytes, ContentType: receipt.Manifest.ContentType},
		{URL: links.InstallURL, Kind: VerifyDocument, SHA256: receipt.Page.SHA256, SizeBytes: receipt.Page.SizeBytes, ContentType: receipt.Page.ContentType},
	}
	for _, check := range checks {
		if err := requireHTTPSURL(check.URL); err != nil {
			return err
		}
		if err := verifier.Verify(ctx, check); err != nil {
			if errors.Is(err, ErrVerificationContentConflict) {
				return fmt.Errorf("%w: recovered object fetch conflicts with saved receipt: %w", ErrPrivatePublishConflict, err)
			}
			return err
		}
	}
	if receipt.Access == AccessPublic && !profileExpiry.After(clock().Add(time.Minute)) {
		return fmt.Errorf("saved publication signing profile is expired or expires too soon")
	}
	return nil
}

func privateSignatureWithinDeadline(rawURL string, deadline time.Time) error {
	signedExpiry, err := privateSignatureExpiry(rawURL)
	if err != nil {
		return err
	}
	delta := signedExpiry.Sub(deadline)
	if delta <= -time.Second || delta >= time.Second {
		return fmt.Errorf("signed expiry does not match saved publication deadline")
	}
	return nil
}

func privateSignatureExpiry(rawURL string) (time.Time, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return time.Time{}, fmt.Errorf("has invalid AWS Signature V4 expiry")
	}
	query := parsed.Query()
	dates, dateFound := query["X-Amz-Date"]
	lifetimes, lifetimeFound := query["X-Amz-Expires"]
	if !dateFound || len(dates) != 1 || dates[0] == "" || !lifetimeFound || len(lifetimes) != 1 || lifetimes[0] == "" {
		return time.Time{}, fmt.Errorf("has invalid AWS Signature V4 expiry")
	}
	signedAt, err := time.Parse("20060102T150405Z", dates[0])
	if err != nil {
		return time.Time{}, fmt.Errorf("has invalid AWS Signature V4 expiry")
	}
	lifetimeSeconds, err := strconv.ParseInt(lifetimes[0], 10, 64)
	if err != nil || lifetimeSeconds <= 0 || lifetimeSeconds > int64(maxLinkLifetime/time.Second) || strconv.FormatInt(lifetimeSeconds, 10) != lifetimes[0] {
		return time.Time{}, fmt.Errorf("has invalid AWS Signature V4 expiry")
	}
	return signedAt.Add(time.Duration(lifetimeSeconds) * time.Second), nil
}

func objectMatchesBytes(object StoredObject, body []byte, contentType string) bool {
	digest := sha256.Sum256(body)
	return equivalentContentType(object.ContentType, contentType) && object.SizeBytes == int64(len(body)) && strings.EqualFold(object.SHA256, hex.EncodeToString(digest[:]))
}

func linkMatchesDestination(rawURL, key string, receipt PublishReceipt) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return fmt.Errorf("must be HTTPS")
	}
	if receipt.Access == AccessPublic {
		expected, err := PublicObjectURL(receipt.PublicBaseURL, key)
		if err != nil || rawURL != expected {
			return fmt.Errorf("does not match the configured public base URL and object key %q", key)
		}
		return nil
	}
	if receipt.Access != AccessPrivate {
		return fmt.Errorf("has unsupported access mode")
	}
	base, err := ValidateEndpoint(receipt.DownloadEndpoint)
	if err != nil {
		return fmt.Errorf("receipt has invalid download endpoint")
	}
	expectedPath := "/" + key
	switch receipt.AddressingStyle {
	case "path":
		expectedPath = "/" + receipt.Bucket + "/" + key
		if !strings.EqualFold(parsed.Host, base.Host) {
			return fmt.Errorf("does not match configured download endpoint")
		}
	case "virtual":
		expectedHost := receipt.Bucket + "." + base.Hostname()
		if base.Port() != "" {
			expectedHost += ":" + base.Port()
		}
		if !strings.EqualFold(parsed.Host, expectedHost) {
			return fmt.Errorf("does not match configured virtual-host download endpoint")
		}
	default:
		return fmt.Errorf("receipt has invalid addressing style")
	}
	if parsed.Path != expectedPath {
		return fmt.Errorf("does not reference expected object key %q", key)
	}
	if parsed.Query().Get("X-Amz-Signature") == "" {
		return fmt.Errorf("does not contain an AWS Signature V4 bearer signature")
	}
	return nil
}

func boundedLifetimes(now time.Time, ttl, grace time.Duration, credentialLimit time.Time, access Access) (time.Duration, time.Duration, error) {
	if access == AccessPublic {
		return 0, 0, nil
	}
	if ttl <= 0 {
		return 0, 0, fmt.Errorf("URL TTL must be positive")
	}
	if grace < 0 {
		return 0, 0, fmt.Errorf("download grace must not be negative")
	}
	if ttl > maxLinkLifetime {
		return 0, 0, fmt.Errorf("URL TTL must not exceed 7d")
	}
	if grace > maxLinkLifetime {
		return 0, 0, fmt.Errorf("download grace must not exceed 7d")
	}
	if ttl > maxLinkLifetime-grace {
		return 0, 0, fmt.Errorf("URL TTL plus download grace must not exceed 7d")
	}
	totalLifetime := ttl + grace
	if !credentialLimit.IsZero() {
		remaining := credentialLimit.Sub(now) - time.Minute
		if remaining <= 0 {
			return 0, 0, fmt.Errorf("storage credentials expire too soon to publish a link")
		}
		if remaining <= grace {
			return 0, 0, fmt.Errorf("storage credentials do not remain valid for the requested download grace")
		}
		if totalLifetime > remaining {
			ttl = remaining - grace
		}
	}
	return ttl, grace, nil
}

func resolveObjectURL(ctx context.Context, options PublishOptions, key string, deadline time.Time, clock func() time.Time) (string, error) {
	if options.Access == AccessPrivate {
		ttl := deadline.Sub(clock())
		if ttl <= 0 {
			return "", fmt.Errorf("credential-safe URL lifetime elapsed before presigning")
		}
		// SigV4 records X-Amz-Date and X-Amz-Expires at whole-second
		// precision. Round the remaining lifetime up so flooring both fields
		// cannot produce a signature one second earlier than the receipt.
		ttl = (ttl + time.Second - 1).Truncate(time.Second)
		for attempt := 0; attempt < 2; attempt++ {
			rawURL, err := options.Store.PresignGet(ctx, key, ttl)
			if err != nil {
				return "", err
			}
			parsed, parseErr := url.Parse(rawURL)
			if parseErr != nil {
				return "", fmt.Errorf("presigned URL has invalid AWS Signature V4 expiry")
			}
			query := parsed.Query()
			if !query.Has("X-Amz-Date") && !query.Has("X-Amz-Expires") {
				return rawURL, nil
			}
			signedExpiry, expiryErr := privateSignatureExpiry(rawURL)
			if expiryErr != nil {
				return "", fmt.Errorf("presigned URL: %w", expiryErr)
			}
			delta := signedExpiry.Sub(deadline)
			if delta <= 0 && delta > -time.Second {
				return rawURL, nil
			}
			if attempt == 0 && delta > 0 && delta < ttl {
				adjustment := delta.Truncate(time.Second)
				if delta%time.Second != 0 {
					adjustment += time.Second
				}
				if adjustment >= ttl {
					return "", fmt.Errorf("presigned URL: signed expiry does not match saved publication deadline")
				}
				ttl -= adjustment
				continue
			}
			return "", fmt.Errorf("presigned URL: signed expiry does not match saved publication deadline")
		}
		return "", fmt.Errorf("presigned URL: signed expiry does not match saved publication deadline")
	}
	return PublicObjectURL(options.PublicBaseURL, key)
}

func ensureBytes(ctx context.Context, store ObjectStore, key string, body []byte, contentType string) (StoredObject, error) {
	digest := sha256.Sum256(body)
	return store.Ensure(ctx, PutObject{Key: key, Body: bytes.NewReader(body), SHA256: hex.EncodeToString(digest[:]), SizeBytes: int64(len(body)), ContentType: contentType})
}

func makeManifest(app PreparedApp, ipaURL string) ([]byte, error) {
	manifest := map[string]any{"items": []any{map[string]any{
		"assets":   []any{map[string]any{"kind": "software-package", "url": ipaURL}},
		"metadata": map[string]any{"bundle-identifier": app.BundleID, "bundle-version": app.BuildNumber, "kind": "software", "title": app.Title},
	}}}
	return plist.MarshalIndent(manifest, plist.XMLFormat, "  ")
}

func makeInstallPage(app PreparedApp, directInstallURL string) ([]byte, error) {
	const source = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="referrer" content="no-referrer"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; img-src data:; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'">
<title>Install {{.Title}}</title><style>body{font-family:-apple-system,BlinkMacSystemFont,sans-serif;max-width:34rem;margin:12vh auto;padding:0 1.5rem;color:#171717}a{display:inline-block;background:#111;color:#fff;text-decoration:none;padding:.8rem 1.1rem;border-radius:.65rem}small{color:#666}</style></head>
<body><h1>{{.Title}}</h1><p><small>Version {{.Version}} ({{.Build}})</small></p><p><a href="{{.URL}}" rel="noreferrer">Install on this iPhone</a></p></body></html>
`
	tmpl, err := template.New("install").Parse(source)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	data := struct {
		Title, Version, Build string
		URL                   template.URL
	}{app.Title, app.Version, app.BuildNumber, template.URL(directInstallURL)} // #nosec G203 -- URL is generated internally from a validated HTTPS manifest URL.
	if err := tmpl.Execute(&output, data); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func containsUnsafeText(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) || unicode.Is(unicode.Cf, character) || unicode.In(character, unicode.Bidi_Control) {
			return true
		}
	}
	return false
}

func ValidateBucket(raw string) error {
	if raw == "" || len([]byte(raw)) > 255 || strings.IndexFunc(raw, unicode.IsSpace) >= 0 || containsUnsafeText(raw) {
		return fmt.Errorf("bucket must be a bounded name without whitespace, control, or formatting characters")
	}
	return nil
}

func requireHTTPSURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("must be HTTPS without embedded credentials")
	}
	return nil
}

func NormalizePrefix(raw string) (string, error) {
	trimmed := strings.Trim(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return "", fmt.Errorf("prefix is required")
	}
	if strings.Contains(trimmed, "\\") || strings.ContainsAny(trimmed, "\r\n\x00") {
		return "", fmt.Errorf("prefix contains unsafe characters")
	}
	if len([]byte(trimmed)) > 1024 || containsUnsafeText(trimmed) || containsUnsafeIdentifierText(trimmed) {
		return "", fmt.Errorf("prefix is too long or contains control or bidi characters")
	}
	parts := strings.Split(trimmed, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("prefix contains an unsafe path segment")
		}
	}
	return strings.Join(parts, "/"), nil
}

func ValidateEndpoint(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if containsUnsafeIdentifierText(trimmed) {
		return nil, fmt.Errorf("endpoint contains whitespace, control, or format characters")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return nil, fmt.Errorf("endpoint must be an HTTPS origin")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return nil, fmt.Errorf("endpoint must not include credentials, path, query, or fragment")
	}
	parsed.Path = ""
	return parsed, nil
}

func containsUnsafeIdentifierText(value string) bool {
	for _, character := range value {
		if unicode.IsSpace(character) || unicode.IsControl(character) || unicode.In(character, unicode.Cf) {
			return true
		}
	}
	return false
}

func ValidatePublicBaseURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid public base URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("public base URL must be an HTTPS URL without credentials, query, or fragment")
	}
	return parsed, nil
}

func PublicObjectURL(base, key string) (string, error) {
	parsed, err := ValidatePublicBaseURL(base)
	if err != nil {
		return "", err
	}
	basePath := strings.TrimSuffix(parsed.Path, "/")
	parts := strings.Split(key, "/")
	parsed.RawPath = ""
	parsed.Path = basePath + "/" + strings.Join(parts, "/")
	return parsed.String(), nil
}

func randomLinkID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func redactBearerURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.RawQuery == "" {
		return raw
	}
	parsed.RawQuery = "REDACTED"
	parsed.Fragment = ""
	return parsed.String()
}

func redactDirectInstallURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "itms-services://?action=download-manifest&url=REDACTED"
	}
	query := parsed.Query()
	if query.Has("url") {
		query.Set("url", "REDACTED")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

type HTTPVerifier struct {
	client  *http.Client
	timeout time.Duration
}

func NewHTTPVerifier(client *http.Client, timeout time.Duration) *HTTPVerifier {
	if client == nil {
		client = &http.Client{}
	}
	clone := *client
	clone.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return &HTTPVerifier{client: &clone, timeout: timeout}
}

// NewHTTPVerifierWithEnvironmentTrust builds a verifier that honors the same
// AWS_CA_BUNDLE trust override used by the S3 SDK for private endpoints.
func NewHTTPVerifierWithEnvironmentTrust(timeout time.Duration) (*HTTPVerifier, error) {
	caPath := strings.TrimSpace(os.Getenv("AWS_CA_BUNDLE"))
	if caPath == "" {
		return NewHTTPVerifier(nil, timeout), nil
	}
	file, err := os.Open(caPath)
	if err != nil {
		return nil, fmt.Errorf("open custom CA bundle: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 2<<20 {
		return nil, fmt.Errorf("custom CA bundle must be a regular file no larger than 2 MiB")
	}
	pemBytes, err := io.ReadAll(io.LimitReader(file, (2<<20)+1))
	if err != nil {
		return nil, fmt.Errorf("read custom CA bundle")
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		roots = x509.NewCertPool()
	}
	if !roots.AppendCertsFromPEM(pemBytes) {
		return nil, fmt.Errorf("custom CA bundle contains no certificates")
	}
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("default HTTP transport is unavailable")
	}
	clone := transport.Clone()
	clone.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	return NewHTTPVerifier(&http.Client{Transport: clone}, timeout), nil
}

func (v *HTTPVerifier) Verify(ctx context.Context, verification VerifyRequest) error {
	if err := requireHTTPSURL(verification.URL); err != nil {
		return fmt.Errorf("verification URL: %w", err)
	}
	verifyCtx := ctx
	cancel := func() {}
	if v.timeout > 0 {
		verifyCtx, cancel = context.WithTimeout(ctx, v.timeout)
	}
	defer cancel()
	request, err := http.NewRequestWithContext(verifyCtx, http.MethodGet, verification.URL, nil)
	if err != nil {
		return fmt.Errorf("create verification request: %w", err)
	}
	response, err := v.client.Do(request)
	if err != nil {
		return fmt.Errorf("GET %s: request failed", safeURLForError(verification.URL))
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 && response.StatusCode < 400 {
		return fmt.Errorf("GET %s returned redirect status %d", safeURLForError(verification.URL), response.StatusCode)
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s returned status %d", safeURLForError(verification.URL), response.StatusCode)
	}
	if !equivalentContentType(response.Header.Get("Content-Type"), verification.ContentType) {
		return fmt.Errorf("%w: %w: GET %s returned an unexpected content type", ErrObjectVerificationMismatch, ErrVerificationContentConflict, safeURLForError(verification.URL))
	}
	if verification.Kind == VerifyIPA {
		if response.ContentLength >= 0 && response.ContentLength != verification.SizeBytes {
			return fmt.Errorf("%w: %w: GET %s returned content length %d, expected %d", ErrObjectVerificationMismatch, ErrVerificationContentConflict, safeURLForError(verification.URL), response.ContentLength, verification.SizeBytes)
		}
		digest := sha256.New()
		written, err := io.Copy(digest, io.LimitReader(response.Body, verification.SizeBytes+1))
		if err != nil {
			return fmt.Errorf("GET %s failed while reading IPA body", safeURLForError(verification.URL))
		}
		if written != verification.SizeBytes {
			return fmt.Errorf("%w: %w: GET %s returned unexpected IPA length", ErrObjectVerificationMismatch, ErrVerificationContentConflict, safeURLForError(verification.URL))
		}
		if !strings.EqualFold(hex.EncodeToString(digest.Sum(nil)), verification.SHA256) {
			return fmt.Errorf("%w: %w: GET %s returned an unexpected IPA SHA-256", ErrObjectVerificationMismatch, ErrVerificationContentConflict, safeURLForError(verification.URL))
		}
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, verification.SizeBytes+1))
	if err != nil {
		return fmt.Errorf("GET %s response body failed", safeURLForError(verification.URL))
	}
	if int64(len(body)) != verification.SizeBytes {
		return fmt.Errorf("%w: GET %s returned unexpected body length", ErrVerificationContentConflict, safeURLForError(verification.URL))
	}
	digest := sha256.Sum256(body)
	if !strings.EqualFold(hex.EncodeToString(digest[:]), verification.SHA256) {
		return fmt.Errorf("%w: GET %s returned an unexpected SHA-256", ErrVerificationContentConflict, safeURLForError(verification.URL))
	}
	return nil
}

func safeURLForError(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "<redacted URL>"
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}
