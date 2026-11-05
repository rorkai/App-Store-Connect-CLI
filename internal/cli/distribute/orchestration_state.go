package distribute

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode"

	core "github.com/rudrankriyam/App-Store-Connect-CLI/internal/distribution"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

const (
	distributionStateSchemaVersion = 1
	distributionConfigMaxBytes     = 64 << 10
	distributionStateMaxBytes      = 1 << 20
	distributionMaxMutations       = 1000
)

var (
	distributionDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	distributionCodePattern   = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	distributionPlanIDPattern = regexp.MustCompile(`^dplan_[0-9a-f]{32}$`)
	// Tests use this to replace a pathname after its directory handle is pinned.
	distributionAfterParentOpenForTest    func()
	distributionAfterProtectedReadForTest func()
	distributionSyncDirectoryForTest      = syncDistributionDirectory
	distributionRenameNoReplaceForTest    = secureopen.RenameNoReplaceInRoot
	distributionRemoveTemporaryForTest    = func(rooted *os.Root, name string) error { return rooted.Remove(name) }
)

// distributionConfig is deliberately narrower than the standalone commands:
// v1 accepts one local PKCS#12 identity and one private S3-compatible target.
// Credentials and passwords are references, never inline values.
type distributionConfig struct {
	SchemaVersion int                           `json:"schemaVersion"`
	DevicesFile   string                        `json:"devicesFile"`
	Signing       distributionSigningConfig     `json:"signing"`
	Publication   distributionPublicationConfig `json:"publication"`
	Metadata      distributionMetadataConfig    `json:"metadata,omitempty"`
}

type distributionSigningConfig struct {
	Identity            distributionIdentityConfig `json:"identity"`
	MinimumValidityDays int                        `json:"minimumValidityDays"`
	MaxMutations        int                        `json:"maxMutations"`
}

type distributionIdentityConfig struct {
	Format            string `json:"format"`
	Path              string `json:"path"`
	PasswordFile      string `json:"passwordFile,omitempty"`
	CertificateSHA256 string `json:"certificateSha256,omitempty"`
}

type distributionPublicationConfig struct {
	Endpoint              string        `json:"endpoint"`
	DownloadEndpoint      string        `json:"downloadEndpoint,omitempty"`
	Region                string        `json:"region"`
	Bucket                string        `json:"bucket"`
	Prefix                string        `json:"prefix"`
	AddressingStyle       string        `json:"addressingStyle"`
	URLTTL                string        `json:"urlTtl"`
	DownloadGrace         string        `json:"downloadGrace"`
	VerifyTimeout         string        `json:"verifyTimeout"`
	URLTTLDuration        time.Duration `json:"-"`
	DownloadGraceDuration time.Duration `json:"-"`
	VerifyTimeoutDuration time.Duration `json:"-"`
}

type distributionMetadataConfig struct {
	Title          string `json:"title,omitempty"`
	Channel        string `json:"channel,omitempty"`
	SourceRevision string `json:"sourceRevision,omitempty"`
	SourceURL      string `json:"sourceUrl,omitempty"`
}

type persistedDistributionPlan struct {
	SchemaVersion int                           `json:"schemaVersion"`
	PlanID        string                        `json:"planId"`
	PlanHash      string                        `json:"planHash"`
	CreatedAt     string                        `json:"createdAt"`
	Ready         bool                          `json:"ready"`
	ConfigPath    string                        `json:"configPath"`
	ConfigSHA256  string                        `json:"configSha256"`
	Archive       distributionArchiveBinding    `json:"archive"`
	DeviceSet     distributionDeviceSetBinding  `json:"deviceSet"`
	Identity      distributionIdentityBinding   `json:"identity"`
	Publication   distributionPublicationConfig `json:"publication"`
	Reconcile     distributionReconcileBinding  `json:"reconcile"`
	Effects       []distributionEffect          `json:"effects"`
	Blockers      []distributionBlocker         `json:"blockers,omitempty"`
	Paths         distributionPlanPaths         `json:"paths"`
}

type distributionArchiveBinding struct {
	Path             string `json:"path"`
	TreeSHA256       string `json:"treeSha256"`
	SizeBytes        int64  `json:"sizeBytes"`
	FileCount        int    `json:"fileCount"`
	BundleID         string `json:"bundleId"`
	Title            string `json:"title"`
	PublishedTitle   string `json:"publishedTitle"`
	Version          string `json:"version"`
	BuildNumber      string `json:"buildNumber"`
	MinimumOSVersion string `json:"minimumOSVersion,omitempty"`
	TeamID           string `json:"teamId"`
	TargetCount      int    `json:"targetCount"`
}

type distributionDeviceSetBinding struct {
	SHA256     string `json:"sha256"`
	FileSHA256 string `json:"fileSha256"`
	Count      int    `json:"count"`
}

type distributionIdentityBinding struct {
	CertificateResourceID string `json:"certificateResourceId"`
	CertificateSHA256     string `json:"certificateSha256"`
	TeamID                string `json:"teamId"`
	ExpirationDate        string `json:"expirationDate"`
	MinimumValidUntil     string `json:"minimumValidUntil"`
}

type distributionReconcileBinding struct {
	PlanPath            string `json:"planPath"`
	PlanHash            string `json:"planHash"`
	ReceiptPath         string `json:"receiptPath"`
	MinimumValidityDays int    `json:"minimumValidityDays"`
	MutationCount       int    `json:"mutationCount"`
	MaxMutations        int    `json:"maxMutations"`
}

type distributionEffect struct {
	Stage    string `json:"stage"`
	Kind     string `json:"kind"`
	BundleID string `json:"bundleId,omitempty"`
	Count    int    `json:"count,omitempty"`
}

type distributionBlocker struct {
	Code    string `json:"code"`
	Stage   string `json:"stage"`
	Message string `json:"message"`
}

type distributionPlanPaths struct {
	StateDir string `json:"stateDir"`
}

type persistedDistributionRunState struct {
	SchemaVersion   int                      `json:"schemaVersion"`
	RunID           string                   `json:"runId"`
	PlanID          string                   `json:"planId"`
	PlanPath        string                   `json:"planPath"`
	PlanHash        string                   `json:"planHash"`
	Status          string                   `json:"status"`
	Stage           string                   `json:"stage"`
	UpdatedAt       string                   `json:"updatedAt"`
	Attempt         int                      `json:"attempt"`
	Recoverable     bool                     `json:"recoverable"`
	LastFailureCode string                   `json:"lastFailureCode,omitempty"`
	Artifacts       distributionRunArtifacts `json:"artifacts,omitempty"`
}

type distributionRunArtifacts struct {
	ReconcileReceipt *distributionFileArtifact        `json:"reconcileReceipt,omitempty"`
	SigningReceipt   *distributionFileArtifact        `json:"signingReceipt,omitempty"`
	ExportOptions    *distributionFileArtifact        `json:"exportOptions,omitempty"`
	ArchiveSnapshot  *distributionArchiveSnapshot     `json:"archiveSnapshot,omitempty"`
	Profile          *distributionProfileArtifact     `json:"profile,omitempty"`
	IPA              *distributionSizedFileArtifact   `json:"ipa,omitempty"`
	Bundle           *distributionBundleArtifact      `json:"bundle,omitempty"`
	Publication      *distributionPublicationArtifact `json:"publication,omitempty"`
}

type distributionArchiveSnapshot struct {
	RelativePath string             `json:"relativePath"`
	TreeSHA256   string             `json:"treeSha256"`
	SizeBytes    int64              `json:"sizeBytes"`
	EntryCount   int                `json:"entryCount"`
	App          archiveAppIdentity `json:"app"`
}

type distributionProfileArtifact struct {
	ResourceID string `json:"resourceId"`
	UUID       string `json:"uuid"`
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	BundleID   string `json:"bundleId"`
}

type distributionFileArtifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type distributionSizedFileArtifact struct {
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"sizeBytes"`
}

type distributionBundleArtifact struct {
	Path             string `json:"path"`
	DescriptorSHA256 string `json:"descriptorSha256"`
}

type distributionPublicationArtifact struct {
	ReceiptPath        string `json:"receiptPath"`
	ReceiptSHA256      string `json:"receiptSha256"`
	LinkPath           string `json:"linkPath"`
	LinkSHA256         string `json:"linkSha256"`
	ArtifactKey        string `json:"artifactKey"`
	ManifestKey        string `json:"manifestKey"`
	PageKey            string `json:"pageKey"`
	InstallURLRedacted string `json:"installUrlRedacted"`
}

type persistedDistributionReceipt struct {
	SchemaVersion                        int      `json:"schemaVersion"`
	RunID                                string   `json:"runId"`
	PlanID                               string   `json:"planId"`
	PlanHash                             string   `json:"planHash"`
	Status                               string   `json:"status"`
	CompletedAt                          string   `json:"completedAt"`
	PublicationReceiptPath               string   `json:"publicationReceiptPath"`
	PublicationReceiptSHA256             string   `json:"publicationReceiptSha256"`
	LinkPath                             string   `json:"linkPath"`
	LinkSHA256                           string   `json:"linkSha256"`
	ArtifactSHA256                       string   `json:"artifactSha256"`
	BundleDescriptorSHA256               string   `json:"bundleDescriptorSha256"`
	ArtifactKey                          string   `json:"artifactKey,omitempty"`
	ManifestKey                          string   `json:"manifestKey,omitempty"`
	PageKey                              string   `json:"pageKey,omitempty"`
	InstallURLRedacted                   string   `json:"installUrlRedacted"`
	AppBundleID                          string   `json:"appBundleId"`
	AppVersion                           string   `json:"appVersion"`
	AppBuildNumber                       string   `json:"appBuildNumber"`
	TeamID                               string   `json:"teamId"`
	ProfileResourceID                    string   `json:"profileResourceId"`
	ProfileClass                         string   `json:"profileClass"`
	ProfileUUID                          string   `json:"profileUuid"`
	ProfileExpiresAt                     string   `json:"profileExpiresAt"`
	ProfileSHA256                        string   `json:"profileSha256"`
	DeviceSetSHA256                      string   `json:"deviceSetSha256"`
	DeviceCount                          int      `json:"deviceCount"`
	CertificateSHA256                    string   `json:"certificateSha256"`
	ProfileCertificateSHA256Fingerprints []string `json:"profileCertificateSha256Fingerprints"`
	SignerCertificateSHA256Fingerprints  []string `json:"signerCertificateSha256Fingerprints"`
	ProfileIntegrityStatus               string   `json:"profileIntegrityStatus"`
	ProfileTrustStatus                   string   `json:"profileTrustStatus"`
	CodeSignatureStatus                  string   `json:"codeSignatureStatus"`
	CodeSignatureScope                   string   `json:"codeSignatureScope"`
	ArtifactSizeBytes                    int64    `json:"artifactSizeBytes"`
	BundleDescriptorSizeBytes            int64    `json:"bundleDescriptorSizeBytes"`
	ManifestSHA256                       string   `json:"manifestSha256"`
	ManifestSizeBytes                    int64    `json:"manifestSizeBytes"`
	PageSHA256                           string   `json:"pageSha256"`
	PageSizeBytes                        int64    `json:"pageSizeBytes"`
	FetchVerified                        bool     `json:"fetchVerified"`
	FetchVerifiedAt                      string   `json:"fetchVerifiedAt"`
}

type distributionJSONFrame struct {
	object    bool
	expectKey bool
	keys      map[string]struct{}
}

func readDistributionConfig(path string) (distributionConfig, string, error) {
	data, err := readProtectedDistributionFile(path, distributionConfigMaxBytes)
	if err != nil {
		return distributionConfig{}, "", fmt.Errorf("read distribution config: %w", err)
	}
	var config distributionConfig
	if err := decodeStrictDistributionJSON(data, &config); err != nil {
		return distributionConfig{}, "", fmt.Errorf("decode distribution config: %w", err)
	}
	if err := validateDistributionConfig(&config); err != nil {
		return distributionConfig{}, "", fmt.Errorf("invalid distribution config: %w", err)
	}
	digest := sha256.Sum256(data)
	return config, hex.EncodeToString(digest[:]), nil
}

func validateDistributionConfig(config *distributionConfig) error {
	if config.SchemaVersion != distributionStateSchemaVersion {
		return fmt.Errorf("schemaVersion must be %d", distributionStateSchemaVersion)
	}
	if err := validateDistributionPath("devicesFile", config.DevicesFile); err != nil {
		return err
	}
	identity := &config.Signing.Identity
	if identity.Format != "pkcs12" {
		return fmt.Errorf("signing.identity.format must be pkcs12")
	}
	if err := validateDistributionPath("signing.identity.path", identity.Path); err != nil {
		return err
	}
	if identity.PasswordFile != "" {
		if err := validateDistributionPath("signing.identity.passwordFile", identity.PasswordFile); err != nil {
			return err
		}
	}
	if identity.CertificateSHA256 != "" && !distributionDigestPattern.MatchString(identity.CertificateSHA256) {
		return fmt.Errorf("signing.identity.certificateSha256 must be a lowercase SHA-256 digest")
	}
	if config.Signing.MinimumValidityDays < 0 || config.Signing.MinimumValidityDays > 3650 {
		return fmt.Errorf("signing.minimumValidityDays must be between 0 and 3650")
	}
	if config.Signing.MaxMutations < 1 || config.Signing.MaxMutations > distributionMaxMutations {
		return fmt.Errorf("signing.maxMutations must be between 1 and %d", distributionMaxMutations)
	}
	publication := &config.Publication
	if _, err := core.ValidateEndpoint(publication.Endpoint); err != nil {
		return fmt.Errorf("publication.endpoint: %w", err)
	}
	if publication.DownloadEndpoint != "" {
		if _, err := core.ValidateEndpoint(publication.DownloadEndpoint); err != nil {
			return fmt.Errorf("publication.downloadEndpoint: %w", err)
		}
	}
	if !regionPattern.MatchString(publication.Region) {
		return fmt.Errorf("publication.region must be 1-100 letters, digits, dots, underscores, or hyphens")
	}
	if !validPublishBucket(publication.Bucket) {
		return fmt.Errorf("publication.bucket must be a bounded name without whitespace or control characters")
	}
	normalizedPrefix, err := core.NormalizePrefix(publication.Prefix)
	if err != nil || normalizedPrefix != publication.Prefix {
		return fmt.Errorf("publication.prefix must be a normalized safe object prefix")
	}
	if publication.AddressingStyle != "path" && publication.AddressingStyle != "virtual" {
		return fmt.Errorf("publication.addressingStyle must be path or virtual")
	}
	if publication.URLTTLDuration, err = parseBoundedDistributionDuration("publication.urlTtl", publication.URLTTL, time.Second, 7*24*time.Hour); err != nil {
		return err
	}
	if publication.DownloadGraceDuration, err = parseBoundedDistributionDuration("publication.downloadGrace", publication.DownloadGrace, 0, 7*24*time.Hour); err != nil {
		return err
	}
	if publication.URLTTLDuration+publication.DownloadGraceDuration > 7*24*time.Hour {
		return fmt.Errorf("publication.urlTtl plus publication.downloadGrace must not exceed 7d")
	}
	if publication.VerifyTimeoutDuration, err = parseBoundedDistributionDuration("publication.verifyTimeout", publication.VerifyTimeout, time.Second, 10*time.Minute); err != nil {
		return err
	}
	metadata := config.Metadata
	for _, item := range []struct {
		name  string
		value string
		limit int
	}{{"metadata.title", metadata.Title, 512}, {"metadata.channel", metadata.Channel, 512}, {"metadata.sourceRevision", metadata.SourceRevision, 1024}} {
		if err := validateDistributionText(item.name, item.value, item.limit); err != nil {
			return err
		}
	}
	if err := validateDistributionSourceURL(metadata.SourceURL); err != nil {
		return err
	}
	return nil
}

func parseBoundedDistributionDuration(name, raw string, minimum, maximum time.Duration) (time.Duration, error) {
	if strings.TrimSpace(raw) != raw || raw == "" || len(raw) > 32 {
		return 0, fmt.Errorf("%s must be a bounded Go duration", name)
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("%s must be between %s and %s", name, minimum, maximum)
	}
	return value, nil
}

func validateDistributionSourceURL(raw string) error {
	if raw == "" {
		return nil
	}
	if err := validateDistributionText("metadata.sourceUrl", raw, 2048); err != nil {
		return err
	}
	if len(raw) > 2048 || !strings.HasPrefix(raw, "https://") || strings.ContainsAny(raw, "?#@\r\n\x00") {
		return fmt.Errorf("metadata.sourceUrl must be an HTTPS URL without credentials, query, or fragment")
	}
	parsed, err := core.ValidatePublicBaseURL(raw)
	if err != nil {
		return fmt.Errorf("metadata.sourceUrl: %w", err)
	}
	// ValidatePublicBaseURL intentionally permits an empty path; source
	// provenance must identify a concrete resource rather than only an origin.
	if parsed.Path == "" {
		return fmt.Errorf("metadata.sourceUrl must include a non-empty path")
	}
	return nil
}

func validateDistributionPath(name, value string) error {
	if value == "" || strings.TrimSpace(value) != value || len(value) > 4096 {
		return fmt.Errorf("%s must be a non-empty bounded path", name)
	}
	return validateDistributionText(name, value, 4096)
}

func validateDistributionText(name, value string, limit int) error {
	if len(value) > limit {
		return fmt.Errorf("%s must be at most %d bytes", name, limit)
	}
	for _, character := range value {
		if unicode.IsControl(character) || unicode.In(character, unicode.Bidi_Control) || unicode.Is(unicode.Cf, character) {
			return fmt.Errorf("%s contains control or formatting characters", name)
		}
	}
	return nil
}

func newDistributionPlanID() (string, error) { return newDistributionID("dplan_") }

func newDistributionID(prefix string) (string, error) {
	var random [16]byte
	if _, err := io.ReadFull(rand.Reader, random[:]); err != nil {
		return "", fmt.Errorf("generate distribution identifier: %w", err)
	}
	return prefix + hex.EncodeToString(random[:]), nil
}

func sealDistributionPlan(plan *persistedDistributionPlan) error {
	if plan == nil {
		return fmt.Errorf("distribution plan is required")
	}
	hash, err := canonicalDistributionPlanHash(*plan)
	if err != nil {
		return err
	}
	plan.PlanHash = hash
	return nil
}

func verifyDistributionPlanHash(plan persistedDistributionPlan) error {
	if !distributionDigestPattern.MatchString(plan.PlanHash) {
		return fmt.Errorf("planHash must be a lowercase SHA-256 digest")
	}
	want, err := canonicalDistributionPlanHash(plan)
	if err != nil {
		return err
	}
	if plan.PlanHash != want {
		return fmt.Errorf("planHash does not match the canonical plan")
	}
	return nil
}

func canonicalDistributionPlanHash(plan persistedDistributionPlan) (string, error) {
	plan.PlanHash = ""
	plan.CreatedAt = ""
	encoded, err := json.Marshal(plan)
	if err != nil {
		return "", fmt.Errorf("encode canonical distribution plan: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func writePersistedDistributionPlan(path string, plan persistedDistributionPlan) error {
	if err := validatePersistedDistributionPlan(plan); err != nil {
		return err
	}
	if err := verifyDistributionPlanHash(plan); err != nil {
		return err
	}
	return writeProtectedDistributionJSONCreateOnly(path, plan)
}

func readPersistedDistributionPlan(path string) (persistedDistributionPlan, error) {
	var plan persistedDistributionPlan
	if err := readProtectedDistributionJSON(path, &plan); err != nil {
		return plan, err
	}
	if err := validatePersistedDistributionPlan(plan); err != nil {
		return plan, err
	}
	return plan, verifyDistributionPlanHash(plan)
}

func validatePersistedDistributionPlan(plan persistedDistributionPlan) error {
	if plan.SchemaVersion != distributionStateSchemaVersion || !distributionPlanIDPattern.MatchString(plan.PlanID) {
		return fmt.Errorf("invalid persisted distribution plan identity")
	}
	createdAt, err := time.Parse(time.RFC3339, plan.CreatedAt)
	if err != nil {
		return fmt.Errorf("invalid plan createdAt")
	}
	for name, digest := range map[string]string{
		"configSha256": plan.ConfigSHA256, "archive.treeSha256": plan.Archive.TreeSHA256,
		"deviceSet.sha256": plan.DeviceSet.SHA256, "deviceSet.fileSha256": plan.DeviceSet.FileSHA256,
		"identity.certificateSha256": plan.Identity.CertificateSHA256,
		"reconcile.planHash":         plan.Reconcile.PlanHash,
	} {
		if !distributionDigestPattern.MatchString(digest) {
			return fmt.Errorf("%s must be a lowercase SHA-256 digest", name)
		}
	}
	if err := validateDistributionText("identity.certificateResourceId", plan.Identity.CertificateResourceID, 256); err != nil {
		return fmt.Errorf("identity.certificateResourceId is required and bounded")
	}
	if plan.Ready && plan.Identity.CertificateResourceID == "" {
		return fmt.Errorf("ready plan requires identity.certificateResourceId")
	}
	if _, err := time.Parse(time.RFC3339, plan.Identity.ExpirationDate); err != nil {
		return fmt.Errorf("identity.expirationDate must be RFC3339")
	}
	expiresAt, _ := time.Parse(time.RFC3339, plan.Identity.ExpirationDate)
	minimumValidUntil, err := time.Parse(time.RFC3339, plan.Identity.MinimumValidUntil)
	if err != nil || minimumValidUntil.Before(createdAt) {
		return fmt.Errorf("identity.minimumValidUntil must be at or after plan createdAt")
	}
	if plan.Ready && !expiresAt.After(minimumValidUntil) {
		return fmt.Errorf("ready plan identity must remain valid after minimumValidUntil")
	}
	for name, value := range map[string]string{
		"archive.bundleId": plan.Archive.BundleID,
		"archive.version":  plan.Archive.Version, "archive.buildNumber": plan.Archive.BuildNumber,
		"archive.teamId": plan.Archive.TeamID, "identity.teamId": plan.Identity.TeamID,
	} {
		if value == "" {
			return fmt.Errorf("%s is required", name)
		}
		if err := validateDistributionText(name, value, 256); err != nil {
			return err
		}
	}
	if plan.Archive.Title == "" || plan.Archive.PublishedTitle == "" {
		return fmt.Errorf("archive.title and archive.publishedTitle are required")
	}
	if err := validateDistributionText("archive.title", plan.Archive.Title, 512); err != nil {
		return err
	}
	if err := validateDistributionText("archive.publishedTitle", plan.Archive.PublishedTitle, 512); err != nil {
		return err
	}
	if err := validateDistributionText("archive.minimumOSVersion", plan.Archive.MinimumOSVersion, 256); err != nil {
		return err
	}
	for name, path := range map[string]string{
		"configPath": plan.ConfigPath, "archive.path": plan.Archive.Path, "reconcile.planPath": plan.Reconcile.PlanPath,
		"reconcile.receiptPath": plan.Reconcile.ReceiptPath, "paths.stateDir": plan.Paths.StateDir,
	} {
		if err := validateCanonicalAbsoluteDistributionPath(name, path); err != nil {
			return err
		}
	}
	if plan.Archive.SizeBytes < 1 || plan.Archive.FileCount < 1 || plan.Archive.TargetCount < 1 || plan.DeviceSet.Count < 1 {
		return fmt.Errorf("plan requires positive archive target, archive, and device counts")
	}
	if plan.Ready && (plan.Archive.TargetCount != 1 || plan.Archive.TeamID != plan.Identity.TeamID) {
		return fmt.Errorf("ready plan requires one archive target and matching archive/identity teamId")
	}
	if plan.Reconcile.MutationCount < 0 || plan.Reconcile.MaxMutations < 1 || plan.Reconcile.MutationCount > plan.Reconcile.MaxMutations {
		return fmt.Errorf("invalid reconcile mutation bounds")
	}
	if plan.Reconcile.MinimumValidityDays < 0 || plan.Reconcile.MinimumValidityDays > distributionMaximumValidityDays {
		return fmt.Errorf("invalid reconcile minimum validity policy")
	}
	publication := plan.Publication
	boundConfig := &distributionConfig{SchemaVersion: 1, DevicesFile: "bound", Signing: distributionSigningConfig{Identity: distributionIdentityConfig{Format: "pkcs12", Path: "bound", PasswordFile: "bound"}, MinimumValidityDays: 0, MaxMutations: plan.Reconcile.MaxMutations}, Publication: publication}
	if err := validateDistributionConfig(boundConfig); err != nil {
		return fmt.Errorf("invalid bound publication: %w", err)
	}
	publication = boundConfig.Publication
	effectiveValidityDays, err := effectiveDistributionMinimumValidityDays(0, publication.URLTTLDuration, publication.DownloadGraceDuration)
	if err != nil || plan.Reconcile.MinimumValidityDays < effectiveValidityDays {
		return fmt.Errorf("reconcile minimum validity policy does not cover publication lifetime")
	}
	for _, effect := range plan.Effects {
		if !validDistributionEffect(effect) || effect.Count < 0 {
			return fmt.Errorf("invalid distribution effect")
		}
		if effect.BundleID != "" {
			if err := validateDistributionText("effect.bundleId", effect.BundleID, 256); err != nil {
				return err
			}
		}
	}
	for _, blocker := range plan.Blockers {
		if !distributionCodePattern.MatchString(blocker.Code) || !validDistributionStage(blocker.Stage) {
			return fmt.Errorf("invalid distribution blocker")
		}
		if err := validateDistributionText("blocker.message", blocker.Message, 512); err != nil || blocker.Message == "" {
			return fmt.Errorf("invalid distribution blocker message")
		}
		if containsPotentialDistributionSecret(blocker.Message) {
			return fmt.Errorf("distribution blocker message must be redacted")
		}
	}
	if plan.Ready && len(plan.Blockers) != 0 {
		return fmt.Errorf("ready plan must not contain blockers")
	}
	if !plan.Ready && len(plan.Blockers) == 0 {
		return fmt.Errorf("blocked plan must contain at least one blocker")
	}
	return nil
}

func validateDistributionEffectInventory(plan persistedDistributionPlan) error {
	counts := make(map[string]int, len(plan.Effects))
	mutations := 0
	for _, effect := range plan.Effects {
		key := effect.Stage + "/" + effect.Kind
		counts[key]++
		if counts[key] > 1 {
			return fmt.Errorf("distribution plan contains duplicate effect %s", key)
		}
		switch key {
		case "account_reconcile/register_device":
			if effect.Count < 1 || effect.BundleID != "" {
				return fmt.Errorf("register_device requires a positive count and no bundleId")
			}
			mutations += effect.Count
		case "account_reconcile/create_bundle_id", "account_reconcile/create_profile":
			if effect.Count != 0 || effect.BundleID != plan.Archive.BundleID {
				return fmt.Errorf("%s must bind the planned bundleId", key)
			}
			mutations++
		case "account_reconcile/write_profile", "export/write_export_options", "export/write_ipa", "prepare/write_bundle":
			if effect.Count != 0 || effect.BundleID != plan.Archive.BundleID {
				return fmt.Errorf("%s must bind the planned bundleId", key)
			}
		case "publish/ensure_ipa", "publish/ensure_manifest", "publish/ensure_install_page":
			if effect.Count != 0 || effect.BundleID != "" {
				return fmt.Errorf("%s must not carry count or bundleId", key)
			}
		}
	}
	mandatory := []string{
		"account_reconcile/write_profile",
		"export/write_export_options", "export/write_ipa", "prepare/write_bundle",
		"publish/ensure_ipa", "publish/ensure_manifest", "publish/ensure_install_page",
	}
	for _, key := range mandatory {
		if counts[key] != 1 {
			return fmt.Errorf("distribution plan requires exact effect %s", key)
		}
	}
	allowed := map[string]bool{
		"account_reconcile/register_device": true, "account_reconcile/create_bundle_id": true,
		"account_reconcile/create_profile": true, "account_reconcile/write_profile": true,
		"export/write_export_options": true, "export/write_ipa": true, "prepare/write_bundle": true,
		"publish/ensure_ipa": true, "publish/ensure_manifest": true, "publish/ensure_install_page": true,
	}
	for key := range counts {
		if !allowed[key] {
			return fmt.Errorf("distribution plan contains unauthorized effect %s", key)
		}
	}
	if mutations != plan.Reconcile.MutationCount {
		return fmt.Errorf("distribution plan mutation effects do not match reconcile mutationCount")
	}
	return nil
}

func createDistributionRunDirectory(stateDir, runID string) error {
	if !distributionRunIDPattern.MatchString(runID) {
		return fmt.Errorf("invalid distribution run identifier")
	}
	rooted, err := openOrCreateDistributionDirectory(stateDir)
	if err != nil {
		return err
	}
	defer rooted.Close()
	if _, err := rooted.Lstat(runID); err == nil {
		return fmt.Errorf("distribution run %s already exists: %w", runID, os.ErrExist)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := rooted.Mkdir(runID, 0o700); err != nil {
		return err
	}
	if err := distributionSyncDirectoryForTest(rooted); err != nil {
		_ = rooted.Remove(runID)
		_ = distributionSyncDirectoryForTest(rooted)
		return fmt.Errorf("sync distribution state directory after run creation: %w", err)
	}
	runRoot, err := openPinnedDistributionChild(rooted, runID)
	if err != nil {
		return err
	}
	defer runRoot.Close()
	return validatePrivateDistributionDirectoryRoot(runRoot)
}

func writeDistributionRunState(stateDir string, state persistedDistributionRunState) error {
	if err := validatePersistedDistributionRunStateForRoot(stateDir, state); err != nil {
		return err
	}
	runRoot, err := protectedDistributionRunRoot(stateDir, state.RunID)
	if err != nil {
		return err
	}
	defer runRoot.Close()
	if state.Status == "complete" {
		var receipt persistedDistributionReceipt
		if err := readProtectedDistributionJSONInRoot(runRoot, "receipt.json", &receipt); err != nil {
			return fmt.Errorf("complete state requires immutable receipt: %w", err)
		}
		if err := validatePersistedDistributionReceipt(receipt); err != nil {
			return fmt.Errorf("complete state receipt is invalid: %w", err)
		}
		plan, err := readPersistedDistributionPlan(state.PlanPath)
		if err != nil {
			return fmt.Errorf("complete state requires exact persisted plan: %w", err)
		}
		if err := distributionReceiptMatchesRunAndPlan(runRoot, receipt, state, plan); err != nil {
			return fmt.Errorf("complete state receipt mismatch: %w", err)
		}
	}
	if _, err := runRoot.Lstat("state.json"); err == nil {
		if _, err := readProtectedDistributionFileInRoot(runRoot, "state.json", distributionStateMaxBytes); err != nil {
			return fmt.Errorf("refusing unsafe existing run state: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	encoded, err := marshalProtectedDistributionJSON(state)
	if err != nil {
		return err
	}
	return writeProtectedDistributionFileInRoot(runRoot, "state.json", encoded, false)
}

func readDistributionRunState(stateDir, runID string) (persistedDistributionRunState, error) {
	var state persistedDistributionRunState
	runRoot, err := protectedDistributionRunRoot(stateDir, runID)
	if err != nil {
		return state, err
	}
	defer runRoot.Close()
	if err := readProtectedDistributionJSONInRoot(runRoot, "state.json", &state); err != nil {
		return state, err
	}
	if state.RunID != runID {
		return state, fmt.Errorf("persisted distribution run ID does not match its directory")
	}
	return state, validatePersistedDistributionRunStateForRoot(stateDir, state)
}

func validatePersistedDistributionRunState(state persistedDistributionRunState) error {
	if state.SchemaVersion != 1 || !distributionRunIDPattern.MatchString(state.RunID) || !distributionPlanIDPattern.MatchString(state.PlanID) || !distributionDigestPattern.MatchString(state.PlanHash) {
		return fmt.Errorf("invalid persisted distribution run identity")
	}
	if err := validateCanonicalAbsoluteDistributionPath("planPath", state.PlanPath); err != nil {
		return err
	}
	if !validDistributionStatus(state.Status) || !validDistributionStage(state.Stage) || state.Attempt < 1 {
		return fmt.Errorf("invalid persisted distribution run status")
	}
	if _, err := time.Parse(time.RFC3339, state.UpdatedAt); err != nil {
		return fmt.Errorf("invalid run updatedAt")
	}
	if state.LastFailureCode != "" && !distributionCodePattern.MatchString(state.LastFailureCode) {
		return fmt.Errorf("invalid lastFailureCode")
	}
	if err := validateDistributionRunArtifacts(state.Artifacts); err != nil {
		return err
	}
	if err := validateDistributionArtifactDependencies(state.Artifacts); err != nil {
		return err
	}
	return validateDistributionRunTransition(state)
}

func validatePersistedDistributionRunStateForRoot(stateDir string, state persistedDistributionRunState) error {
	if strings.TrimSpace(stateDir) == "" {
		return fmt.Errorf("distribution state root is required")
	}
	if _, err := filepath.Abs(filepath.Clean(stateDir)); err != nil {
		return fmt.Errorf("resolve distribution state root: %w", err)
	}
	return validatePersistedDistributionRunState(state)
}

func validateDistributionRunArtifacts(artifacts distributionRunArtifacts) error {
	files := []*distributionFileArtifact{artifacts.ReconcileReceipt, artifacts.SigningReceipt, artifacts.ExportOptions}
	for _, artifact := range files {
		if artifact != nil && (!distributionDigestPattern.MatchString(artifact.SHA256) || validateDistributionRunRelativePath("artifact.path", artifact.Path) != nil) {
			return fmt.Errorf("invalid distribution file artifact")
		}
	}
	if snapshot := artifacts.ArchiveSnapshot; snapshot != nil {
		if validateDistributionRunRelativePath("archiveSnapshot.relativePath", snapshot.RelativePath) != nil || snapshot.SizeBytes < 1 || snapshot.EntryCount < 1 || !distributionDigestPattern.MatchString(snapshot.TreeSHA256) {
			return fmt.Errorf("invalid distribution archive snapshot")
		}
		if err := validateArchiveAppIdentity(snapshot.App); err != nil {
			return fmt.Errorf("invalid distribution archive snapshot app identity: %w", err)
		}
	}
	if profile := artifacts.Profile; profile != nil {
		if profile.ResourceID == "" || profile.UUID == "" || profile.BundleID == "" || !distributionDigestPattern.MatchString(profile.SHA256) || validateDistributionRunRelativePath("profile.path", profile.Path) != nil {
			return fmt.Errorf("invalid distribution profile artifact")
		}
		for name, value := range map[string]string{"profile.resourceId": profile.ResourceID, "profile.uuid": profile.UUID, "profile.bundleId": profile.BundleID} {
			if err := validateDistributionText(name, value, 256); err != nil {
				return err
			}
		}
	}
	if artifacts.IPA != nil && (artifacts.IPA.SizeBytes < 1 || !distributionDigestPattern.MatchString(artifacts.IPA.SHA256) || validateDistributionRunRelativePath("ipa.path", artifacts.IPA.Path) != nil) {
		return fmt.Errorf("invalid distribution IPA artifact")
	}
	if artifacts.Bundle != nil && (!distributionDigestPattern.MatchString(artifacts.Bundle.DescriptorSHA256) || validateDistributionRunRelativePath("bundle.path", artifacts.Bundle.Path) != nil) {
		return fmt.Errorf("invalid distribution bundle artifact")
	}
	if publication := artifacts.Publication; publication != nil {
		if !distributionDigestPattern.MatchString(publication.ReceiptSHA256) || !distributionDigestPattern.MatchString(publication.LinkSHA256) || validateDistributionRunRelativePath("publication.receiptPath", publication.ReceiptPath) != nil || validateDistributionRunRelativePath("publication.linkPath", publication.LinkPath) != nil {
			return fmt.Errorf("invalid distribution publication artifact")
		}
		if err := validateRedactedDistributionURL(publication.InstallURLRedacted); err != nil {
			return err
		}
		for name, key := range map[string]string{"artifactKey": publication.ArtifactKey, "manifestKey": publication.ManifestKey, "pageKey": publication.PageKey} {
			if err := validateDistributionObjectKey(name, key); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateDistributionRunRelativePath(name, value string) error {
	if value == "" || len(value) > 4096 || strings.TrimSpace(value) != value || filepath.IsAbs(value) || strings.Contains(value, `\`) || strings.ContainsAny(value, "?#") {
		return fmt.Errorf("%s must be a canonical path relative to the run root", name)
	}
	cleaned := path.Clean(value)
	if cleaned != value || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("%s must be a canonical path relative to the run root", name)
	}
	return validateDistributionText(name, value, 4096)
}

func validateCanonicalAbsoluteDistributionPath(name, value string) error {
	if err := validateDistributionPath(name, value); err != nil {
		return err
	}
	if !filepath.IsAbs(value) || filepath.Clean(value) != value {
		return fmt.Errorf("%s must be a canonical absolute path", name)
	}
	return nil
}

func validateDistributionRunTransition(state persistedDistributionRunState) error {
	artifacts := state.Artifacts
	anyArtifacts := artifacts.ArchiveSnapshot != nil || artifacts.ReconcileReceipt != nil || artifacts.Profile != nil || artifacts.ExportOptions != nil || artifacts.IPA != nil || artifacts.SigningReceipt != nil || artifacts.Bundle != nil || artifacts.Publication != nil
	if state.Status == "planned" {
		if state.Stage != "preflight" || anyArtifacts || state.Recoverable || state.LastFailureCode != "" {
			return fmt.Errorf("planned run must be an empty non-recoverable preflight")
		}
		return nil
	}
	if state.Status == "complete" {
		if state.Stage != "complete" || state.Recoverable || state.LastFailureCode != "" || !distributionArtifactsComplete(artifacts) {
			return fmt.Errorf("complete run requires every stage artifact and no failure state")
		}
		return nil
	}
	if state.Stage == "complete" {
		return fmt.Errorf("only complete status may use complete stage")
	}
	switch state.Status {
	case "running":
		if state.Recoverable || state.LastFailureCode != "" {
			return fmt.Errorf("running state must not carry recovery failure state")
		}
	case "recoverable":
		if !state.Recoverable || state.LastFailureCode == "" {
			return fmt.Errorf("recoverable state requires recoverable=true and a failure code")
		}
	case "blocked":
		if state.Recoverable || state.LastFailureCode == "" {
			return fmt.Errorf("blocked state requires a non-recoverable failure code")
		}
	}
	if err := validateDistributionStagePrerequisites(state.Stage, artifacts); err != nil {
		return err
	}
	return validateDistributionStageCeiling(state.Stage, artifacts)
}

func distributionArtifactsComplete(a distributionRunArtifacts) bool {
	return a.ArchiveSnapshot != nil && a.ReconcileReceipt != nil && a.Profile != nil && a.ExportOptions != nil && a.IPA != nil && a.SigningReceipt != nil && a.Bundle != nil && a.Publication != nil
}

func validateDistributionArtifactDependencies(a distributionRunArtifacts) error {
	checks := []struct {
		present      bool
		prerequisite bool
		name         string
		requires     string
	}{
		{a.ReconcileReceipt != nil, a.ArchiveSnapshot != nil, "reconcile receipt", "archive snapshot"},
		{a.Profile != nil, a.ReconcileReceipt != nil, "profile", "reconcile receipt"},
		{a.ExportOptions != nil, a.Profile != nil, "export options", "profile"},
		{a.IPA != nil, a.ExportOptions != nil, "IPA", "export options"},
		{a.SigningReceipt != nil, a.IPA != nil, "signing receipt", "IPA"},
		{a.Bundle != nil, a.SigningReceipt != nil, "prepared bundle", "signing receipt"},
		{a.Publication != nil, a.Bundle != nil, "publication", "prepared bundle"},
	}
	for _, check := range checks {
		if check.present && !check.prerequisite {
			return fmt.Errorf("%s artifact requires %s artifact", check.name, check.requires)
		}
	}
	return nil
}

func validateDistributionStagePrerequisites(stage string, a distributionRunArtifacts) error {
	require := func(ok bool, name string) error {
		if !ok {
			return fmt.Errorf("stage %s requires %s artifact", stage, name)
		}
		return nil
	}
	switch stage {
	case "preflight":
		return nil
	case "identity_validate":
		// Identity and archive validation failures must be checkpointable before
		// a durable archive snapshot exists. The next stage is the first stage
		// that may mutate account state, and still requires that snapshot.
		return nil
	case "account_reconcile":
		return require(a.ArchiveSnapshot != nil, "archive snapshot")
	case "export":
		if err := require(a.ArchiveSnapshot != nil, "archive snapshot"); err != nil {
			return err
		}
		if err := require(a.ReconcileReceipt != nil, "reconcile receipt"); err != nil {
			return err
		}
		return require(a.Profile != nil, "profile")
	case "prepare":
		if err := validateDistributionStagePrerequisites("export", a); err != nil {
			return err
		}
		if err := require(a.ExportOptions != nil, "export options"); err != nil {
			return err
		}
		if err := require(a.IPA != nil, "IPA"); err != nil {
			return err
		}
		return require(a.SigningReceipt != nil, "signing receipt")
	case "publish":
		if err := validateDistributionStagePrerequisites("prepare", a); err != nil {
			return err
		}
		return require(a.Bundle != nil, "prepared bundle")
	case "fetch_verify":
		if err := validateDistributionStagePrerequisites("publish", a); err != nil {
			return err
		}
		return require(a.Publication != nil, "publication")
	default:
		return fmt.Errorf("invalid distribution stage %q", stage)
	}
}

func validateDistributionStageCeiling(stage string, a distributionRunArtifacts) error {
	switch stage {
	case "preflight":
		if a.ReconcileReceipt != nil || a.Profile != nil || a.ExportOptions != nil || a.IPA != nil || a.SigningReceipt != nil || a.Bundle != nil || a.Publication != nil {
			return fmt.Errorf("preflight contains future-stage artifacts")
		}
	case "identity_validate":
		if a.ReconcileReceipt != nil || a.Profile != nil || a.ExportOptions != nil || a.IPA != nil || a.SigningReceipt != nil || a.Bundle != nil || a.Publication != nil {
			return fmt.Errorf("identity_validate contains future-stage artifacts")
		}
	case "account_reconcile":
		if a.ExportOptions != nil || a.IPA != nil || a.SigningReceipt != nil || a.Bundle != nil || a.Publication != nil {
			return fmt.Errorf("account_reconcile contains future-stage artifacts")
		}
	case "export":
		if a.Bundle != nil || a.Publication != nil {
			return fmt.Errorf("export contains future-stage artifacts")
		}
	case "prepare":
		if a.Publication != nil {
			return fmt.Errorf("prepare contains future-stage artifacts")
		}
	case "publish":
		// Publication may exist after the provider committed but before the
		// fetch-verification checkpoint advances.
	case "fetch_verify":
	}
	return nil
}

func writeDistributionReceipt(stateDir string, receipt persistedDistributionReceipt) error {
	if err := validatePersistedDistributionReceipt(receipt); err != nil {
		return err
	}
	runRoot, err := protectedDistributionRunRoot(stateDir, receipt.RunID)
	if err != nil {
		return err
	}
	defer runRoot.Close()
	var state persistedDistributionRunState
	if err := readProtectedDistributionJSONInRoot(runRoot, "state.json", &state); err != nil {
		return fmt.Errorf("final receipt requires persisted fetch verification state: %w", err)
	}
	if err := validatePersistedDistributionRunState(state); err != nil {
		return fmt.Errorf("final receipt run state is invalid: %w", err)
	}
	if state.Status != "running" || state.Stage != "fetch_verify" || state.Recoverable || state.LastFailureCode != "" || state.Artifacts.Publication == nil {
		return fmt.Errorf("final receipt requires clean running fetch_verify state with publication evidence")
	}
	plan, err := readPersistedDistributionPlan(state.PlanPath)
	if err != nil {
		return fmt.Errorf("final receipt requires exact persisted plan: %w", err)
	}
	if err := distributionReceiptMatchesRunAndPlan(runRoot, receipt, state, plan); err != nil {
		return fmt.Errorf("final receipt does not match persisted run: %w", err)
	}
	return writeProtectedDistributionJSONCreateOnlyInRoot(runRoot, "receipt.json", receipt)
}

func distributionReceiptMatchesRunAndPlan(runRoot *os.Root, receipt persistedDistributionReceipt, state persistedDistributionRunState, plan persistedDistributionPlan) error {
	if receipt.RunID != state.RunID || receipt.PlanID != state.PlanID || receipt.PlanHash != state.PlanHash {
		return fmt.Errorf("run or plan identity differs")
	}
	if plan.PlanID != state.PlanID || plan.PlanHash != state.PlanHash {
		return fmt.Errorf("persisted plan identity differs")
	}
	if !distributionArchiveSnapshotMatchesPlan(state.Artifacts.ArchiveSnapshot, plan.Archive) {
		return fmt.Errorf("archive snapshot evidence differs from plan")
	}
	if receipt.DeviceSetSHA256 != plan.DeviceSet.SHA256 || receipt.DeviceCount != plan.DeviceSet.Count {
		return fmt.Errorf("device-set evidence differs from plan")
	}
	if receipt.CertificateSHA256 != plan.Identity.CertificateSHA256 || receipt.TeamID != plan.Identity.TeamID {
		return fmt.Errorf("identity evidence differs from plan")
	}
	if receipt.AppBundleID != plan.Archive.BundleID || state.Artifacts.Profile == nil || state.Artifacts.Profile.BundleID != plan.Archive.BundleID {
		return fmt.Errorf("app bundle evidence differs from plan")
	}
	if state.Artifacts.IPA == nil || receipt.ArtifactSHA256 != state.Artifacts.IPA.SHA256 || receipt.ArtifactSizeBytes != state.Artifacts.IPA.SizeBytes {
		return fmt.Errorf("IPA evidence differs")
	}
	if state.Artifacts.Bundle == nil || receipt.BundleDescriptorSHA256 != state.Artifacts.Bundle.DescriptorSHA256 {
		return fmt.Errorf("bundle descriptor evidence differs")
	}
	publication := state.Artifacts.Publication
	if publication == nil || receipt.PublicationReceiptPath != publication.ReceiptPath || receipt.PublicationReceiptSHA256 != publication.ReceiptSHA256 || receipt.LinkPath != publication.LinkPath || receipt.LinkSHA256 != publication.LinkSHA256 || receipt.ArtifactKey != publication.ArtifactKey || receipt.ManifestKey != publication.ManifestKey || receipt.PageKey != publication.PageKey || receipt.InstallURLRedacted != publication.InstallURLRedacted {
		return fmt.Errorf("publication evidence differs")
	}
	if receipt.ProfileResourceID != state.Artifacts.Profile.ResourceID || receipt.ProfileUUID != state.Artifacts.Profile.UUID || receipt.ProfileSHA256 != state.Artifacts.Profile.SHA256 || receipt.AppBundleID != state.Artifacts.Profile.BundleID {
		return fmt.Errorf("profile evidence differs")
	}
	descriptor, descriptorSize, publicationReceipt, err := readDistributionFinalizationEvidence(runRoot, state)
	if err != nil {
		return err
	}
	if _, err := readDistributionRunEvidenceFileExact(runRoot, state.Artifacts.Publication.LinkPath, distributionStateMaxBytes, state.Artifacts.Publication.LinkSHA256); err != nil {
		return fmt.Errorf("read sensitive install-link evidence: %w", err)
	}
	return matchDistributionFinalizationEvidence(receipt, state, plan, descriptor, descriptorSize, publicationReceipt)
}

func distributionArchiveSnapshotMatchesPlan(snapshot *distributionArchiveSnapshot, plan distributionArchiveBinding) bool {
	if snapshot == nil {
		return false
	}
	return snapshot.TreeSHA256 == plan.TreeSHA256 && snapshot.SizeBytes == plan.SizeBytes && snapshot.EntryCount == plan.FileCount &&
		snapshot.App.BundleID == plan.BundleID && snapshot.App.Title == plan.Title && snapshot.App.Version == plan.Version &&
		snapshot.App.BuildNumber == plan.BuildNumber && snapshot.App.MinimumOSVersion == plan.MinimumOSVersion
}

func readDistributionFinalizationEvidence(runRoot *os.Root, state persistedDistributionRunState) (core.Descriptor, int64, core.PublishReceipt, error) {
	if runRoot == nil || state.Artifacts.Bundle == nil || state.Artifacts.Publication == nil {
		return core.Descriptor{}, 0, core.PublishReceipt{}, fmt.Errorf("finalization evidence is incomplete")
	}
	descriptorPath := path.Join(state.Artifacts.Bundle.Path, "bundle.json")
	descriptorData, err := readDistributionRunEvidenceFile(runRoot, descriptorPath, distributionStateMaxBytes, false)
	if err != nil {
		return core.Descriptor{}, 0, core.PublishReceipt{}, fmt.Errorf("read prepared bundle descriptor: %w", err)
	}
	if digestDistributionBytes(descriptorData) != state.Artifacts.Bundle.DescriptorSHA256 {
		return core.Descriptor{}, 0, core.PublishReceipt{}, fmt.Errorf("prepared bundle descriptor digest differs from run state")
	}
	var descriptor core.Descriptor
	if err := decodeStrictDistributionJSON(descriptorData, &descriptor); err != nil {
		return core.Descriptor{}, 0, core.PublishReceipt{}, fmt.Errorf("decode prepared bundle descriptor: %w", err)
	}
	if err := core.ValidateDescriptorForPublish(descriptor); err != nil {
		return core.Descriptor{}, 0, core.PublishReceipt{}, fmt.Errorf("validate prepared bundle descriptor: %w", err)
	}

	publicationData, err := readDistributionRunEvidenceFile(runRoot, state.Artifacts.Publication.ReceiptPath, distributionStateMaxBytes, true)
	if err != nil {
		return core.Descriptor{}, 0, core.PublishReceipt{}, fmt.Errorf("read publication receipt: %w", err)
	}
	if digestDistributionBytes(publicationData) != state.Artifacts.Publication.ReceiptSHA256 {
		return core.Descriptor{}, 0, core.PublishReceipt{}, fmt.Errorf("publication receipt digest differs from run state")
	}
	var publicationReceipt core.PublishReceipt
	if err := decodeStrictDistributionJSON(publicationData, &publicationReceipt); err != nil {
		return core.Descriptor{}, 0, core.PublishReceipt{}, fmt.Errorf("decode publication receipt: %w", err)
	}
	return descriptor, int64(len(descriptorData)), publicationReceipt, nil
}

func matchDistributionFinalizationEvidence(receipt persistedDistributionReceipt, state persistedDistributionRunState, plan persistedDistributionPlan, descriptor core.Descriptor, descriptorSize int64, publication core.PublishReceipt) error {
	if err := validateDistributionText("descriptor.app.title", descriptor.App.Title, 512); err != nil || descriptor.App.Title == "" {
		return fmt.Errorf("prepared app title is invalid")
	}
	if receipt.BundleDescriptorSizeBytes != descriptorSize || receipt.AppBundleID != plan.Archive.BundleID || receipt.AppVersion != plan.Archive.Version || receipt.AppBuildNumber != plan.Archive.BuildNumber ||
		descriptor.App.BundleID != plan.Archive.BundleID || descriptor.App.Title != plan.Archive.PublishedTitle || descriptor.App.Version != plan.Archive.Version ||
		descriptor.App.BuildNumber != plan.Archive.BuildNumber || descriptor.App.MinimumOSVersion != plan.Archive.MinimumOSVersion ||
		receipt.ArtifactSHA256 != descriptor.Artifact.SHA256 || receipt.ArtifactSizeBytes != descriptor.Artifact.SizeBytes {
		return fmt.Errorf("prepared app or descriptor evidence differs")
	}
	signing := descriptor.Signing
	if receipt.ProfileClass != string(signing.ProfileClass) || receipt.ProfileUUID != signing.ProfileUUID || receipt.ProfileExpiresAt != signing.ExpiresAt || receipt.ProfileSHA256 != signing.EmbeddedProfileSHA256 || receipt.TeamID != signing.TeamID || receipt.DeviceCount != signing.DeviceCount || receipt.DeviceSetSHA256 != signing.DeviceSetSHA256 || !reflect.DeepEqual(receipt.ProfileCertificateSHA256Fingerprints, signing.ProfileCertificateSHA256Fingerprints) || !reflect.DeepEqual(receipt.SignerCertificateSHA256Fingerprints, signing.CodeSignatureVerification.SignerCertificateSHA256Fingerprints) || receipt.ProfileIntegrityStatus != string(signing.ProfileIntegrityVerification.Status) || receipt.ProfileTrustStatus != string(signing.ProfileTrustVerification.Status) || receipt.CodeSignatureStatus != string(signing.CodeSignatureVerification.Status) || receipt.CodeSignatureScope != signing.CodeSignatureVerification.Scope {
		return fmt.Errorf("prepared signing evidence differs")
	}
	if publication.SchemaVersion != "1" || publication.Access != core.AccessPrivate || !publication.Verified || receipt.AppBundleID != publication.App.BundleID || receipt.AppVersion != publication.App.Version || receipt.AppBuildNumber != publication.App.BuildNumber || publication.App.BundleID != descriptor.App.BundleID || publication.App.Title != descriptor.App.Title || publication.App.Version != descriptor.App.Version || publication.App.BuildNumber != descriptor.App.BuildNumber || publication.App.MinimumOSVersion != descriptor.App.MinimumOSVersion {
		return fmt.Errorf("publication app verification evidence differs")
	}
	wantEndpoint, endpointErr := normalizedDistributionEndpoint(plan.Publication.Endpoint)
	wantDownloadEndpoint, downloadEndpointErr := normalizedDistributionEndpoint(plan.Publication.DownloadEndpoint)
	if plan.Publication.DownloadEndpoint == "" {
		wantDownloadEndpoint, downloadEndpointErr = wantEndpoint, endpointErr
	}
	publicationURLTTL, urlTTLErr := time.ParseDuration(publication.URLTTL)
	planURLTTL, planURLTTLErr := time.ParseDuration(plan.Publication.URLTTL)
	publicationGrace, graceErr := time.ParseDuration(publication.DownloadGrace)
	planGrace, planGraceErr := time.ParseDuration(plan.Publication.DownloadGrace)
	if endpointErr != nil || downloadEndpointErr != nil || urlTTLErr != nil || planURLTTLErr != nil || graceErr != nil || planGraceErr != nil || publication.Endpoint != wantEndpoint || publication.DownloadEndpoint != wantDownloadEndpoint || publication.PublicBaseURL != "" || publication.Region != plan.Publication.Region || publication.AddressingStyle != plan.Publication.AddressingStyle || publication.Bucket != plan.Publication.Bucket || publication.Prefix != plan.Publication.Prefix || publicationURLTTL != planURLTTL || publicationGrace != planGrace {
		return fmt.Errorf("publication destination or lifetime evidence differs from plan")
	}
	wantRunRoot := filepath.Join(plan.Paths.StateDir, state.RunID)
	wantPublicationPath := filepath.Clean(filepath.Join(wantRunRoot, filepath.FromSlash(state.Artifacts.Publication.ReceiptPath)))
	wantLinkPath := filepath.Clean(filepath.Join(wantRunRoot, filepath.FromSlash(state.Artifacts.Publication.LinkPath)))
	if !filepath.IsAbs(publication.ReceiptPath) || !filepath.IsAbs(publication.LinkPath) || filepath.Clean(publication.ReceiptPath) != wantPublicationPath || filepath.Clean(publication.LinkPath) != wantLinkPath {
		return fmt.Errorf("publication internal artifact paths differ from run state")
	}
	completedAt, _ := time.Parse(time.RFC3339, receipt.CompletedAt)
	fetchVerifiedAt, _ := time.Parse(time.RFC3339, receipt.FetchVerifiedAt)
	profileExpiresAt, profileExpiryErr := time.Parse(time.RFC3339, receipt.ProfileExpiresAt)
	if profileExpiryErr != nil || !profileExpiresAt.After(completedAt) || !profileExpiresAt.After(fetchVerifiedAt) || publication.ExpiresAt == nil || !publication.ExpiresAt.After(completedAt) || !publication.ExpiresAt.After(fetchVerifiedAt) || publication.InstallURL != receipt.InstallURLRedacted || publication.DirectInstallURL != "itms-services://?action=download-manifest&url=REDACTED" {
		return fmt.Errorf("publication redacted link or expiry evidence differs")
	}
	if receipt.ArtifactKey != publication.Artifact.Key || receipt.ArtifactSHA256 != publication.Artifact.SHA256 || receipt.ArtifactSizeBytes != publication.Artifact.SizeBytes || receipt.ManifestKey != publication.Manifest.Key || receipt.ManifestSHA256 != publication.Manifest.SHA256 || receipt.ManifestSizeBytes != publication.Manifest.SizeBytes || receipt.PageKey != publication.Page.Key || receipt.PageSHA256 != publication.Page.SHA256 || receipt.PageSizeBytes != publication.Page.SizeBytes || receipt.InstallURLRedacted != publication.InstallURL {
		return fmt.Errorf("publication object evidence differs")
	}
	if publication.Artifact.ContentType != core.ContentTypeIPA || publication.Manifest.ContentType != core.ContentTypeManifest || publication.Page.ContentType != core.ContentTypeHTML || !validDistributionStoredObjectStatus(publication.Artifact.Status) || !validDistributionStoredObjectStatus(publication.Manifest.Status) || !validDistributionStoredObjectStatus(publication.Page.Status) {
		return fmt.Errorf("publication object content type or status evidence differs")
	}
	publicationSigning := publication.Signing
	if receipt.ProfileClass != publicationSigning.ProfileClass || receipt.ProfileUUID != publicationSigning.ProfileUUID || receipt.ProfileExpiresAt != publicationSigning.ProfileExpiresAt || receipt.ProfileSHA256 != publicationSigning.EmbeddedProfileSHA256 || receipt.TeamID != publicationSigning.TeamID || receipt.DeviceCount != publicationSigning.DeviceCount || receipt.DeviceSetSHA256 != publicationSigning.DeviceSetSHA256 || !reflect.DeepEqual(receipt.ProfileCertificateSHA256Fingerprints, publicationSigning.ProfileCertificateFingerprints) || !reflect.DeepEqual(receipt.SignerCertificateSHA256Fingerprints, publicationSigning.CodeSignatureVerification.SignerCertificateSHA256Fingerprints) || receipt.ProfileIntegrityStatus != publicationSigning.ProfileIntegrityVerification.Status || receipt.ProfileTrustStatus != publicationSigning.ProfileTrustVerification.Status || receipt.CodeSignatureStatus != publicationSigning.CodeSignatureVerification.Status || receipt.CodeSignatureScope != publicationSigning.CodeSignatureVerification.Scope {
		return fmt.Errorf("publication signing evidence differs")
	}
	return nil
}

func normalizedDistributionEndpoint(raw string) (string, error) {
	parsed, err := core.ValidateEndpoint(raw)
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}

func validDistributionStoredObjectStatus(value string) bool {
	return value == "uploaded" || value == "reused"
}

func readDistributionRunEvidenceFile(runRoot *os.Root, relative string, limit int64, requirePrivate bool) ([]byte, error) {
	if err := validateDistributionRunRelativePath("evidence.path", relative); err != nil {
		return nil, err
	}
	components := strings.Split(relative, "/")
	current := runRoot
	owned := false
	for _, component := range components[:len(components)-1] {
		next, err := openPinnedDistributionChild(current, component)
		if err != nil {
			if owned {
				_ = current.Close()
			}
			return nil, err
		}
		if owned {
			_ = current.Close()
		}
		current, owned = next, true
	}
	if owned {
		defer current.Close()
	}
	name := components[len(components)-1]
	before, err := current.Lstat(name)
	if err != nil {
		return nil, err
	}
	file, err := secureopen.OpenExistingNoFollowInRoot(current, name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(before, info) || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("finalization evidence changed during validation or is not a regular file")
	}
	uid, nlink, ok := distributionStatIdentity(info)
	if runtime.GOOS != "windows" && (!ok || uid != uint64(os.Geteuid())) {
		return nil, fmt.Errorf("finalization evidence must be owned by the current user")
	}
	if ok && nlink != 1 {
		return nil, fmt.Errorf("finalization evidence must not be a hard link")
	}
	if runtime.GOOS != "windows" {
		mode := info.Mode().Perm()
		if (requirePrivate && mode != 0o600) || (!requirePrivate && mode != 0o600 && mode != 0o644) {
			return nil, fmt.Errorf("finalization evidence has unsafe permissions")
		}
	}
	if info.Size() < 1 || info.Size() > limit {
		return nil, fmt.Errorf("finalization evidence must be between 1 and %d bytes", limit)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != info.Size() || int64(len(data)) > limit {
		return nil, fmt.Errorf("finalization evidence size changed or exceeds %d bytes", limit)
	}
	return data, nil
}

func readDistributionRunEvidenceFileExact(runRoot *os.Root, relative string, limit int64, wantSHA256 string) ([]byte, error) {
	data, err := readDistributionRunEvidenceFile(runRoot, relative, limit, true)
	if err != nil {
		return nil, err
	}
	if !distributionDigestPattern.MatchString(wantSHA256) || digestDistributionBytes(data) != wantSHA256 {
		return nil, fmt.Errorf("finalization evidence digest differs from run state")
	}
	return data, nil
}

func digestDistributionBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func readDistributionReceipt(stateDir, runID string) (persistedDistributionReceipt, error) {
	var receipt persistedDistributionReceipt
	runRoot, err := protectedDistributionRunRoot(stateDir, runID)
	if err != nil {
		return receipt, err
	}
	defer runRoot.Close()
	if err := readProtectedDistributionJSONInRoot(runRoot, "receipt.json", &receipt); err != nil {
		return receipt, err
	}
	return receipt, validatePersistedDistributionReceipt(receipt)
}

func validatePersistedDistributionReceipt(receipt persistedDistributionReceipt) error {
	if receipt.SchemaVersion != 1 || !distributionRunIDPattern.MatchString(receipt.RunID) || !distributionPlanIDPattern.MatchString(receipt.PlanID) || receipt.Status != "published_and_fetch_verified" {
		return fmt.Errorf("invalid persisted distribution receipt")
	}
	for name, digest := range map[string]string{
		"planHash": receipt.PlanHash, "publicationReceiptSha256": receipt.PublicationReceiptSHA256, "linkSha256": receipt.LinkSHA256,
		"artifactSha256": receipt.ArtifactSHA256, "bundleDescriptorSha256": receipt.BundleDescriptorSHA256,
		"profileSha256": receipt.ProfileSHA256, "deviceSetSha256": receipt.DeviceSetSHA256,
		"certificateSha256": receipt.CertificateSHA256, "manifestSha256": receipt.ManifestSHA256, "pageSha256": receipt.PageSHA256,
	} {
		if !distributionDigestPattern.MatchString(digest) {
			return fmt.Errorf("%s must be a lowercase SHA-256 digest", name)
		}
	}
	completedAt, err := time.Parse(time.RFC3339, receipt.CompletedAt)
	if err != nil {
		return fmt.Errorf("invalid receipt completedAt")
	}
	fetchVerifiedAt, err := time.Parse(time.RFC3339, receipt.FetchVerifiedAt)
	if err != nil || !receipt.FetchVerified || fetchVerifiedAt.After(completedAt) {
		return fmt.Errorf("receipt requires fetch verification at or before completion")
	}
	if err := validateDistributionRunRelativePath("publicationReceiptPath", receipt.PublicationReceiptPath); err != nil {
		return err
	}
	if err := validateDistributionRunRelativePath("linkPath", receipt.LinkPath); err != nil {
		return err
	}
	for name, value := range map[string]string{
		"appBundleId": receipt.AppBundleID, "appVersion": receipt.AppVersion, "appBuildNumber": receipt.AppBuildNumber,
		"teamId": receipt.TeamID, "profileResourceId": receipt.ProfileResourceID, "profileUuid": receipt.ProfileUUID,
	} {
		if value == "" {
			return fmt.Errorf("%s is required", name)
		}
		if err := validateDistributionText(name, value, 256); err != nil {
			return err
		}
	}
	if receipt.ProfileClass != string(core.ProfileClassAdHoc) {
		return fmt.Errorf("profileClass must be ad-hoc")
	}
	if _, err := time.Parse(time.RFC3339, receipt.ProfileExpiresAt); err != nil {
		return fmt.Errorf("profileExpiresAt must be RFC3339")
	}
	if receipt.DeviceCount < 1 || receipt.ArtifactSizeBytes < 1 || receipt.BundleDescriptorSizeBytes < 1 || receipt.ManifestSizeBytes < 1 || receipt.PageSizeBytes < 1 {
		return fmt.Errorf("receipt artifact sizes and device count must be positive")
	}
	if err := validateDistributionFingerprintSet("profileCertificateSha256Fingerprints", receipt.ProfileCertificateSHA256Fingerprints, receipt.CertificateSHA256); err != nil {
		return err
	}
	if err := validateDistributionFingerprintSet("signerCertificateSha256Fingerprints", receipt.SignerCertificateSHA256Fingerprints, receipt.CertificateSHA256); err != nil {
		return err
	}
	if receipt.ProfileIntegrityStatus != string(core.CodeSignatureVerified) || receipt.ProfileTrustStatus != string(core.CodeSignatureVerified) || receipt.CodeSignatureStatus != string(core.CodeSignatureVerified) || receipt.CodeSignatureScope != core.CodeSignatureScopeCompleteMainApp {
		return fmt.Errorf("receipt requires verified profile integrity, trust, and complete code-signature scope")
	}
	for name, key := range map[string]string{"artifactKey": receipt.ArtifactKey, "manifestKey": receipt.ManifestKey, "pageKey": receipt.PageKey} {
		if err := validateDistributionObjectKey(name, key); err != nil {
			return err
		}
	}
	return validateRedactedDistributionURL(receipt.InstallURLRedacted)
}

func validateDistributionFingerprintSet(name string, values []string, required string) error {
	if len(values) == 0 || len(values) > 32 {
		return fmt.Errorf("%s must contain 1-32 fingerprints", name)
	}
	found := false
	previous := ""
	for _, value := range values {
		if !distributionDigestPattern.MatchString(value) || (previous != "" && value <= previous) {
			return fmt.Errorf("%s must be sorted unique lowercase SHA-256 fingerprints", name)
		}
		if value == required {
			found = true
		}
		previous = value
	}
	if !found {
		return fmt.Errorf("%s does not contain selected certificate", name)
	}
	return nil
}

func validateRedactedDistributionURL(value string) error {
	if value == "" || len(value) > 4096 {
		return fmt.Errorf("install URL must be present and redacted")
	}
	parsed, err := url.Parse(value)
	queryRedacted := parsed != nil && (parsed.RawQuery == "REDACTED" || (parsed.RawQuery == "" && strings.HasSuffix(parsed.EscapedPath(), "/REDACTED")))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || !queryRedacted {
		return fmt.Errorf("install URL must be HTTPS with its complete query redacted")
	}
	return validateDistributionText("installUrlRedacted", value, 4096)
}

func validateDistributionObjectKey(name, value string) error {
	if strings.ContainsAny(value, "?#") || strings.Contains(value, "://") || strings.Contains(strings.ToLower(value), "x-amz-") {
		return fmt.Errorf("%s must not contain URL credentials or query syntax", name)
	}
	normalized, err := core.NormalizePrefix(value)
	if err != nil || normalized != value {
		return fmt.Errorf("%s must be a normalized bounded object key", name)
	}
	return nil
}

func containsPotentialDistributionSecret(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "://") || strings.Contains(lower, "x-amz-") || strings.Contains(lower, "password") || strings.ContainsAny(value, "?&=")
}

func protectedDistributionRunRoot(stateDir, runID string) (*os.Root, error) {
	if !distributionRunIDPattern.MatchString(runID) {
		return nil, fmt.Errorf("invalid distribution run identifier")
	}
	stateRoot, err := openExistingDistributionDirectory(stateDir, true)
	if err != nil {
		return nil, err
	}
	defer stateRoot.Close()
	runRoot, err := openPinnedDistributionChild(stateRoot, runID)
	if err != nil {
		return nil, err
	}
	if err := validatePrivateDistributionDirectoryRoot(runRoot); err != nil {
		_ = runRoot.Close()
		return nil, err
	}
	return runRoot, nil
}

func validDistributionStatus(value string) bool {
	switch value {
	case "planned", "running", "recoverable", "blocked", "complete":
		return true
	default:
		return false
	}
}

func validDistributionStage(value string) bool {
	switch value {
	case "preflight", "identity_validate", "account_reconcile", "export", "prepare", "publish", "fetch_verify", "complete":
		return true
	default:
		return false
	}
}

func validDistributionEffectKind(value string) bool {
	switch value {
	case "register_device", "create_bundle_id", "create_profile", "write_profile", "write_export_options", "write_ipa", "write_bundle", "ensure_ipa", "ensure_manifest", "ensure_install_page":
		return true
	default:
		return false
	}
}

func validDistributionEffect(effect distributionEffect) bool {
	if !validDistributionStage(effect.Stage) || !validDistributionEffectKind(effect.Kind) {
		return false
	}
	switch effect.Kind {
	case "register_device", "create_bundle_id", "create_profile", "write_profile":
		return effect.Stage == "account_reconcile"
	case "write_export_options", "write_ipa":
		return effect.Stage == "export"
	case "write_bundle":
		return effect.Stage == "prepare"
	case "ensure_ipa", "ensure_manifest", "ensure_install_page":
		return effect.Stage == "publish"
	default:
		return false
	}
}

func writeProtectedDistributionJSONCreateOnly(path string, value any) error {
	parent, err := openOrCreateDistributionDirectory(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer parent.Close()
	return writeProtectedDistributionJSONCreateOnlyInRoot(parent, filepath.Base(path), value)
}

func writeProtectedDistributionJSONCreateOnlyInRoot(rooted *os.Root, name string, value any) error {
	encoded, err := marshalProtectedDistributionJSON(value)
	if err != nil {
		return err
	}
	return writeProtectedDistributionFileInRoot(rooted, name, encoded, true)
}

func writeProtectedDistributionFileInRoot(rooted *os.Root, name string, data []byte, createOnly bool) error {
	if rooted == nil || filepath.Base(name) != name || name == "." {
		return fmt.Errorf("protected output name must be one file component")
	}
	info, err := rooted.Lstat(name)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return err
	case createOnly:
		return fmt.Errorf("protected output %q already exists: %w", name, os.ErrExist)
	default:
		file, openErr := secureopen.OpenExistingNoFollowInRoot(rooted, name)
		if openErr != nil {
			return openErr
		}
		openedInfo, statErr := file.Stat()
		closeErr := file.Close()
		if statErr != nil || closeErr != nil {
			return errors.Join(statErr, closeErr)
		}
		if !os.SameFile(info, openedInfo) {
			return fmt.Errorf("protected output changed during validation")
		}
		if err := validatePrivateDistributionFileInfo(openedInfo); err != nil {
			return err
		}
	}
	if distributionAfterParentOpenForTest != nil {
		distributionAfterParentOpenForTest()
	}
	temporary, temporaryName, err := secureopen.CreateTempNoFollowInRoot(rooted, ".", ".asc-distribution-state-*", 0o600)
	if err != nil {
		return err
	}
	published := false
	defer func() {
		_ = temporary.Close()
		if !published {
			_ = rooted.Remove(temporaryName)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	createdViaLink := false
	if createOnly {
		if err := distributionRenameNoReplaceForTest(rooted, temporaryName, name); err != nil {
			if !errors.Is(err, secureopen.ErrRenameNoReplaceUnsupported) {
				return err
			}
			if err := rooted.Link(temporaryName, name); err != nil {
				return err
			}
			createdViaLink = true
			if err := distributionRemoveTemporaryForTest(rooted, temporaryName); err != nil {
				rollbackErr := rooted.Remove(name)
				syncErr := distributionSyncDirectoryForTest(rooted)
				failures := []error{fmt.Errorf("remove linked distribution state temporary file: %w", err)}
				if rollbackErr != nil && !errors.Is(rollbackErr, os.ErrNotExist) {
					failures = append(failures, fmt.Errorf("roll back linked distribution state file: %w", rollbackErr))
				}
				if syncErr != nil {
					failures = append(failures, fmt.Errorf("sync protected output parent after rollback: %w", syncErr))
				}
				return errors.Join(failures...)
			}
		}
	} else if err := rooted.Rename(temporaryName, name); err != nil {
		return err
	}
	if err := distributionSyncDirectoryForTest(rooted); err != nil {
		if createOnly {
			_ = rooted.Remove(name)
			_ = distributionSyncDirectoryForTest(rooted)
		} else if createdViaLink {
			_ = rooted.Remove(name)
		}
		return fmt.Errorf("sync protected output parent: %w", err)
	}
	published = true
	return nil
}

func syncDistributionDirectory(rooted *os.Root) error {
	if rooted == nil {
		return fmt.Errorf("directory root is nil")
	}
	directory, err := rooted.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		// Windows does not support flushing directory handles. File Flush plus
		// atomic rename remains the strongest available contract there.
		if runtime.GOOS == "windows" {
			return nil
		}
		return err
	}
	return nil
}

func marshalProtectedDistributionJSON(value any) ([]byte, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	encoded = append(encoded, '\n')
	if len(encoded) > distributionStateMaxBytes {
		return nil, fmt.Errorf("distribution state exceeds %d bytes", distributionStateMaxBytes)
	}
	return encoded, nil
}

func readProtectedDistributionJSON(path string, target any) error {
	parent, err := openExistingDistributionDirectory(filepath.Dir(path), false)
	if err != nil {
		return err
	}
	defer parent.Close()
	return readProtectedDistributionJSONInRoot(parent, filepath.Base(path), target)
}

func readProtectedDistributionJSONInRoot(rooted *os.Root, name string, target any) error {
	data, err := readProtectedDistributionFileInRoot(rooted, name, distributionStateMaxBytes)
	if err != nil {
		return err
	}
	return decodeStrictDistributionJSON(data, target)
}

func readProtectedDistributionFile(path string, limit int64) ([]byte, error) {
	parent, err := openExistingDistributionDirectory(filepath.Dir(path), false)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	return readProtectedDistributionFileInRoot(parent, filepath.Base(path), limit)
}

func readProtectedDistributionFileInRoot(rooted *os.Root, name string, limit int64) ([]byte, error) {
	file, err := secureopen.OpenExistingNoFollowInRoot(rooted, name)
	if err != nil {
		if info, statErr := rooted.Lstat(name); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("refusing to follow symlink %q", name)
		}
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if err := validatePrivateDistributionFileInfo(info); err != nil {
		return nil, err
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("protected file exceeds %d bytes", limit)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("protected file exceeds %d bytes", limit)
	}
	if distributionAfterProtectedReadForTest != nil {
		distributionAfterProtectedReadForTest()
	}
	after, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if err := validatePrivateDistributionFileInfo(after); err != nil {
		return nil, err
	}
	if !os.SameFile(info, after) || info.Mode() != after.Mode() || info.Size() != after.Size() || !info.ModTime().Equal(after.ModTime()) || int64(len(data)) != info.Size() {
		return nil, fmt.Errorf("protected file changed while reading")
	}
	return data, nil
}

func validatePrivateDistributionFileInfo(info os.FileInfo) error {
	if !info.Mode().IsRegular() {
		return fmt.Errorf("protected input is not a regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		return fmt.Errorf("protected input permissions must be exactly 0600")
	}
	uid, nlink, ok := distributionStatIdentity(info)
	if runtime.GOOS != "windows" && (!ok || uid != uint64(os.Geteuid())) {
		return fmt.Errorf("protected input must be owned by the current user")
	}
	if ok && nlink != 1 {
		return fmt.Errorf("protected input must not be a hard link")
	}
	return nil
}

func distributionStatIdentity(info os.FileInfo) (uid, nlink uint64, ok bool) {
	value := reflect.ValueOf(info.Sys())
	if !value.IsValid() {
		return 0, 0, false
	}
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return 0, 0, false
	}
	uidField, nlinkField := value.FieldByName("Uid"), value.FieldByName("Nlink")
	if !uidField.IsValid() || !nlinkField.IsValid() || !uidField.CanUint() || !nlinkField.CanUint() {
		return 0, 0, false
	}
	return uidField.Uint(), nlinkField.Uint(), true
}

func openOrCreateDistributionDirectory(path string) (*os.Root, error) {
	return openDistributionDirectory(path, true, true)
}

func openExistingDistributionDirectory(path string, requirePrivate bool) (*os.Root, error) {
	return openDistributionDirectory(path, false, requirePrivate)
}

func openDistributionDirectory(path string, createMissing, requirePrivate bool) (*os.Root, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	volume := filepath.VolumeName(absolute)
	anchor := volume + string(filepath.Separator)
	current, err := os.OpenRoot(anchor)
	if err != nil {
		return nil, err
	}
	for _, component := range strings.Split(strings.TrimPrefix(absolute, anchor), string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		before, statErr := current.Lstat(component)
		if errors.Is(statErr, os.ErrNotExist) {
			if !createMissing {
				_ = current.Close()
				return nil, statErr
			}
			if err := current.Mkdir(component, 0o700); err != nil {
				_ = current.Close()
				return nil, err
			}
			if err := distributionSyncDirectoryForTest(current); err != nil {
				_ = current.Remove(component)
				_ = distributionSyncDirectoryForTest(current)
				_ = current.Close()
				return nil, fmt.Errorf("sync parent after creating distribution directory: %w", err)
			}
			before, statErr = current.Lstat(component)
		}
		if statErr != nil {
			_ = current.Close()
			return nil, statErr
		}
		if before.Mode()&os.ModeSymlink != 0 {
			if !trustedSystemDirectorySymlink(current, before) {
				_ = current.Close()
				return nil, fmt.Errorf("refusing distribution state symlink or non-directory component %q", component)
			}
			next, openErr := current.OpenRoot(component)
			if openErr != nil {
				_ = current.Close()
				return nil, openErr
			}
			after, afterErr := next.Stat(".")
			if afterErr != nil || !after.IsDir() {
				_ = next.Close()
				_ = current.Close()
				if afterErr != nil {
					return nil, afterErr
				}
				return nil, fmt.Errorf("system symlink target is not a directory")
			}
			_ = current.Close()
			current = next
			continue
		}
		if !before.IsDir() {
			_ = current.Close()
			return nil, fmt.Errorf("refusing distribution state symlink or non-directory component %q", component)
		}
		next, openErr := current.OpenRoot(component)
		if openErr != nil {
			_ = current.Close()
			return nil, openErr
		}
		after, afterErr := next.Stat(".")
		if afterErr != nil || !os.SameFile(before, after) {
			_ = next.Close()
			_ = current.Close()
			if afterErr != nil {
				return nil, afterErr
			}
			return nil, fmt.Errorf("distribution directory changed during traversal")
		}
		_ = current.Close()
		current = next
	}
	if requirePrivate {
		if err := validatePrivateDistributionDirectoryRoot(current); err != nil {
			_ = current.Close()
			return nil, err
		}
	}
	return current, nil
}

func trustedSystemDirectorySymlink(parent *os.Root, link os.FileInfo) bool {
	if runtime.GOOS == "windows" {
		return false
	}
	linkUID, _, linkOK := distributionStatIdentity(link)
	parentInfo, err := parent.Stat(".")
	if err != nil {
		return false
	}
	parentUID, _, parentOK := distributionStatIdentity(parentInfo)
	return linkOK && parentOK && linkUID == 0 && parentUID == 0 && parentInfo.Mode().Perm()&0o022 == 0
}

func openPinnedDistributionChild(parent *os.Root, name string) (*os.Root, error) {
	before, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return nil, fmt.Errorf("refusing distribution directory symlink or non-directory %q", name)
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	after, err := child.Stat(".")
	if err != nil || !os.SameFile(before, after) {
		_ = child.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("distribution directory changed while opening")
	}
	return child, nil
}

func validatePrivateDistributionDirectoryRoot(rooted *os.Root) error {
	info, err := rooted.Stat(".")
	if err != nil {
		return err
	}
	return validatePrivateDistributionDirectoryInfo(rooted.Name(), info)
}

func validatePrivateDistributionDirectoryInfo(path string, info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing protected distribution directory symlink %q", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("protected distribution path %q is not a directory", path)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o700 {
		return fmt.Errorf("protected distribution directory permissions must be exactly 0700")
	}
	uid, _, ok := distributionStatIdentity(info)
	if runtime.GOOS != "windows" && (!ok || uid != uint64(os.Geteuid())) {
		return fmt.Errorf("protected distribution directory must be owned by the current user")
	}
	return nil
}

func decodeStrictDistributionJSON(data []byte, target any) error {
	if err := rejectDuplicateDistributionJSONKeys(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func rejectDuplicateDistributionJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var stack []distributionJSONFrame
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case json.Delim:
			switch value {
			case '{':
				stack = append(stack, distributionJSONFrame{object: true, expectKey: true, keys: make(map[string]struct{})})
			case '[':
				stack = append(stack, distributionJSONFrame{})
			case '}', ']':
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
				markDistributionJSONValueConsumed(stack)
			}
		case string:
			if len(stack) > 0 && stack[len(stack)-1].object && stack[len(stack)-1].expectKey {
				current := &stack[len(stack)-1]
				if _, exists := current.keys[value]; exists {
					return fmt.Errorf("duplicate JSON field %q", value)
				}
				current.keys[value] = struct{}{}
				current.expectKey = false
			} else {
				markDistributionJSONValueConsumed(stack)
			}
		default:
			markDistributionJSONValueConsumed(stack)
		}
	}
}

func markDistributionJSONValueConsumed(stack []distributionJSONFrame) {
	if len(stack) > 0 && stack[len(stack)-1].object && !stack[len(stack)-1].expectKey {
		stack[len(stack)-1].expectKey = true
	}
}
