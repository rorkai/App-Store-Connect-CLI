package distribute

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	core "github.com/rudrankriyam/App-Store-Connect-CLI/internal/distribution"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

const privatePublishIntentStateSchemaVersion = "2"

var errPrivatePublishIntentConflict = errors.New("private publication intent conflict")

var (
	preparePrivatePublicationIntent                                            = core.PreparePrivatePublishIntent
	executePrivatePublicationIntent                                            = core.ExecutePrivatePublishIntent
	newPrivatePublicationVerifier   func(time.Duration) (core.Verifier, error) = func(timeout time.Duration) (core.Verifier, error) {
		return newPublicationVerifier(timeout)
	}
	afterPrivatePublicationIntentPersisted = func() error { return nil }
	afterPrivatePublicationIntentExecuted  = func() error { return nil }
)

// privatePublishIntentRequest has no access or public-base fields, so the
// agent orchestrator cannot express public access, and it must provide a
// dedicated protected intent path.
type privatePublishIntentRequest struct {
	BundleDir        string
	ExpectedBundle   privatePublishBundleAuthorization
	Endpoint         string
	DownloadEndpoint string
	Region           string
	Bucket           string
	Prefix           string
	AddressingStyle  string
	URLTTL           time.Duration
	DownloadGrace    time.Duration
	VerifyTimeout    time.Duration
	ReceiptPath      string
	IntentPath       string
	DiagnosticWriter io.Writer
}

// privatePublishBundleAuthorization is copied from the authorized run evidence.
// It binds the protected handles opened by LoadPreparedBundle to the exact
// descriptor, payload, profile, device set, team, and signing identity that the
// agent was authorized to publish.
type privatePublishBundleAuthorization struct {
	DescriptorSHA256  string `json:"descriptorSha256"`
	DescriptorSize    int64  `json:"descriptorSize"`
	IPASHA256         string `json:"ipaSha256"`
	IPASize           int64  `json:"ipaSize"`
	ProfileUUID       string `json:"profileUuid"`
	ProfileSHA256     string `json:"profileSha256"`
	TeamID            string `json:"teamId"`
	DeviceSetSHA256   string `json:"deviceSetSha256"`
	DeviceCount       int    `json:"deviceCount"`
	CertificateSHA256 string `json:"certificateSha256"`
}

type privatePublishIntentBinding struct {
	Endpoint         string                            `json:"endpoint"`
	DownloadEndpoint string                            `json:"downloadEndpoint"`
	Region           string                            `json:"region"`
	AddressingStyle  string                            `json:"addressingStyle"`
	Bucket           string                            `json:"bucket"`
	Prefix           string                            `json:"prefix"`
	RequestedURLTTL  string                            `json:"requestedUrlTtl"`
	DownloadGrace    string                            `json:"downloadGrace"`
	ReceiptPath      string                            `json:"receiptPath"`
	IntentPath       string                            `json:"intentPath"`
	ExpectedBundle   privatePublishBundleAuthorization `json:"expectedBundle"`
}

type privatePublishIntentState struct {
	SchemaVersion string                      `json:"schemaVersion"`
	IntentSHA256  string                      `json:"intentSha256"`
	Binding       privatePublishIntentBinding `json:"binding"`
	Intent        core.PrivatePublishIntent   `json:"intent"`
}

// executePrivatePublishIntent durably records every publication choice before
// the first remote write, then converges only those recorded destinations.
func executePrivatePublishIntent(ctx context.Context, request privatePublishIntentRequest) (publishExecutionResult, error) {
	diagnostic := request.DiagnosticWriter
	if diagnostic == nil {
		diagnostic = io.Discard
	}
	if err := validatePrivatePublishIntentRequest(request); err != nil {
		return publishExecutionResult{}, err
	}
	bundleDir := strings.TrimSpace(request.BundleDir)
	receiptPath := strings.TrimSpace(request.ReceiptPath)
	intentPath := strings.TrimSpace(request.IntentPath)
	validatedEndpoint, _ := core.ValidateEndpoint(request.Endpoint)
	downloadEndpoint := effectiveDownloadEndpoint(request.Endpoint, request.DownloadEndpoint)
	normalizedPrefix, _ := core.NormalizePrefix(request.Prefix)

	bundleRoot, err := rootfs.New(bundleDir)
	if err != nil {
		return publishExecutionResult{}, fmt.Errorf("distribute private publish intent: open prepared bundle: %w", err)
	}
	defer bundleRoot.Close()
	if err := rejectBundleContainedArtifacts(bundleRoot, receiptPath, intentPath); err != nil {
		return publishExecutionResult{}, shared.UsageErrorf("publish artifacts: %v", err)
	}
	artifacts, err := inspectArtifactPaths(receiptPath, intentPath)
	if err != nil {
		return publishExecutionResult{}, shared.UsageErrorf("publish artifacts: %v", err)
	}
	defer artifacts.close()

	bundle, err := loadPreparedBundle(ctx, bundleRoot)
	if err != nil {
		return publishExecutionResult{}, fmt.Errorf("distribute private publish intent: %w", err)
	}
	defer bundle.IPA.Close()
	if err := validateAuthorizedPrivatePublishBundle(bundle, request.ExpectedBundle); err != nil {
		return publishExecutionResult{}, fmt.Errorf("%w: authorized prepared bundle changed: %w", errPrivatePublishIntentConflict, err)
	}
	binding := privatePublishIntentBinding{
		Endpoint: validatedEndpoint.String(), DownloadEndpoint: downloadEndpoint, Region: request.Region,
		AddressingStyle: request.AddressingStyle, Bucket: request.Bucket, Prefix: normalizedPrefix,
		RequestedURLTTL: request.URLTTL.String(), DownloadGrace: request.DownloadGrace.String(),
		ReceiptPath: artifacts.receiptPath, IntentPath: artifacts.linkPath, ExpectedBundle: request.ExpectedBundle,
	}

	state, found, err := loadPrivatePublishIntentState(artifacts)
	if err != nil {
		return publishExecutionResult{}, fmt.Errorf("%w: load protected publication intent: %w", errPrivatePublishIntentConflict, err)
	}
	if artifacts.receiptExists && !found {
		return publishExecutionResult{}, fmt.Errorf("%w: receipt exists without protected intent", errPrivatePublishIntentConflict)
	}
	if !found {
		if err := artifacts.preflightNew(); err != nil {
			return publishExecutionResult{}, shared.UsageErrorf("publish artifacts: %v", err)
		}
	}
	if found {
		if err := validatePrivatePublishIntentState(state, binding, bundle); err != nil {
			return publishExecutionResult{}, fmt.Errorf("%w: validate saved intent", errPrivatePublishIntentConflict)
		}
		if artifacts.receiptExists {
			receipt, err := readPrivatePublishIntentReceipt(artifacts)
			if err != nil {
				return publishExecutionResult{}, fmt.Errorf("%w: read saved receipt", errPrivatePublishIntentConflict)
			}
			if err := validatePrivatePublishIntentReceipt(receipt, state, bundle); err != nil {
				return publishExecutionResult{}, fmt.Errorf("%w: validate saved receipt", errPrivatePublishIntentConflict)
			}
			verifier, err := newPrivatePublicationVerifier(request.VerifyTimeout)
			if err != nil {
				return publishExecutionResult{}, fmt.Errorf("configure publication verifier: %w", err)
			}
			if err := reverifyPublication(ctx, verifier, receipt, state.Intent.Links, time.Now().UTC()); err != nil {
				return publishExecutionResult{}, fmt.Errorf("reverify recovered private publication: %w", err)
			}
			return publishExecutionResult{Receipt: receipt, Recovered: true}, nil
		}
	}

	setupCtx, setupCancel := shared.ContextWithUploadTimeout(ctx)
	store, credentialLimit, err := newObjectStore(setupCtx, core.S3StoreConfig{
		Endpoint: request.Endpoint, DownloadEndpoint: request.DownloadEndpoint, Region: request.Region,
		Bucket: request.Bucket, AddressingStyle: request.AddressingStyle,
	})
	setupCancel()
	if err != nil {
		return publishExecutionResult{}, fmt.Errorf("distribute private publish intent: %w", err)
	}
	credentialSource := "standard-sdk-chain"
	if provider, ok := store.(interface{ CredentialSource() string }); ok {
		credentialSource = provider.CredentialSource()
	}
	fmt.Fprintf(diagnostic, "Publishing recoverably to endpoint=%s download-endpoint=%s bucket=%s region=%s addressing=%s prefix=%s access=private credentials=%s\n", endpointOrigin(request.Endpoint), downloadEndpoint, request.Bucket, request.Region, request.AddressingStyle, normalizedPrefix, credentialSource)
	verifier, err := newPrivatePublicationVerifier(request.VerifyTimeout)
	if err != nil {
		return publishExecutionResult{}, fmt.Errorf("configure publication verifier: %w", err)
	}
	publicationNow := time.Now().UTC()
	options := core.PublishOptions{
		Store: publicationStore{delegate: store}, Verifier: verifier, Bucket: request.Bucket, Prefix: normalizedPrefix, Access: core.AccessPrivate,
		URLTTL: request.URLTTL, DownloadGrace: request.DownloadGrace, CredentialLimit: credentialLimit,
		Now: func() time.Time { return publicationNow },
	}
	recovered := found
	if !found {
		if !credentialLimitCoversDistributionPublication(credentialLimit, publicationNow, request.URLTTL, request.DownloadGrace) {
			return publishExecutionResult{}, errDistributionPublicationCredentialsExpireTooSoon
		}
		intent, err := preparePrivatePublicationIntent(ctx, bundle.Descriptor, options)
		if err != nil {
			return publishExecutionResult{}, fmt.Errorf("prepare private publication intent: %w", err)
		}
		state = privatePublishIntentState{SchemaVersion: privatePublishIntentStateSchemaVersion, Binding: binding, Intent: intent}
		state.IntentSHA256, err = privatePublishIntentDigest(intent)
		if err != nil {
			return publishExecutionResult{}, fmt.Errorf("hash private publication intent: %w", err)
		}
		if err := validatePrivatePublishIntentState(state, binding, bundle); err != nil {
			return publishExecutionResult{}, fmt.Errorf("%w: prepared publication intent does not match the authorized request", errPrivatePublishIntentConflict)
		}
		if err := publishPrivatePublishIntentState(artifacts, state); err != nil {
			return publishExecutionResult{}, fmt.Errorf("persist private publication intent: %w", err)
		}
		if err := afterPrivatePublicationIntentPersisted(); err != nil {
			return publishExecutionResult{}, err
		}
	}

	receipt, links, err := executePrivatePublicationIntent(ctx, bundle.IPA, bundle.Descriptor, options, state.Intent)
	if err != nil {
		return publishExecutionResult{}, fmt.Errorf("execute private publication intent: %w", err)
	}
	if !privateSensitiveLinksEqual(links, state.Intent.Links) {
		return publishExecutionResult{}, fmt.Errorf("%w: execute returned different sensitive links", core.ErrPrivatePublishConflict)
	}
	if err := afterPrivatePublicationIntentExecuted(); err != nil {
		return publishExecutionResult{}, err
	}
	receipt.ReceiptPath = artifacts.receiptPath
	receipt.LinkPath = artifacts.linkPath
	receipt.Endpoint = binding.Endpoint
	receipt.DownloadEndpoint = binding.DownloadEndpoint
	receipt.Region = binding.Region
	receipt.AddressingStyle = binding.AddressingStyle
	receipt.Bucket = binding.Bucket
	receipt.Prefix = binding.Prefix
	receipt.Access = core.AccessPrivate
	receipt.PublicBaseURL = ""
	if err := validatePrivatePublishIntentReceipt(receipt, state, bundle); err != nil {
		return publishExecutionResult{}, fmt.Errorf("%w: validate completed publication", core.ErrPrivatePublishConflict)
	}
	if err := artifacts.publishReceipt(receipt); err != nil {
		return publishExecutionResult{}, fmt.Errorf("publish private publication receipt: %w", err)
	}
	return publishExecutionResult{Receipt: receipt, Recovered: recovered}, nil
}

// reverifyPrivatePublishIntent is the read-only verification seam for a
// completed agent publication. It never constructs an object store, repairs a
// receipt, or refreshes an expired URL.
func reverifyPrivatePublishIntent(ctx context.Context, request privatePublishVerificationRequest) (publishExecutionResult, error) {
	if strings.TrimSpace(request.BundleDir) == "" || strings.TrimSpace(request.ReceiptPath) == "" || strings.TrimSpace(request.LinkPath) == "" {
		return publishExecutionResult{}, shared.UsageError("bundle directory, receipt, and private intent paths are required")
	}
	if request.VerifyTimeout <= 0 {
		return publishExecutionResult{}, shared.UsageError("verify timeout must be positive")
	}
	bundleDir := strings.TrimSpace(request.BundleDir)
	receiptPath := strings.TrimSpace(request.ReceiptPath)
	linkPath := strings.TrimSpace(request.LinkPath)
	bundleRoot, err := rootfs.New(bundleDir)
	if err != nil {
		return publishExecutionResult{}, fmt.Errorf("open prepared bundle: %w", err)
	}
	defer bundleRoot.Close()
	if err := rejectBundleContainedArtifacts(bundleRoot, receiptPath, linkPath); err != nil {
		return publishExecutionResult{}, shared.UsageErrorf("publish artifacts: %v", err)
	}
	artifacts, err := openExistingArtifactPaths(receiptPath, linkPath)
	if err != nil {
		return publishExecutionResult{}, fmt.Errorf("open private publication artifacts: %w", err)
	}
	defer artifacts.close()
	state, found, err := loadPrivatePublishIntentState(artifacts)
	if err != nil {
		return publishExecutionResult{}, fmt.Errorf("load private publication intent: %w", err)
	}
	if !found {
		return publishExecutionResult{}, fmt.Errorf("private publication intent does not exist")
	}
	bundle, err := loadPreparedBundle(ctx, bundleRoot)
	if err != nil {
		return publishExecutionResult{}, fmt.Errorf("load prepared bundle: %w", err)
	}
	defer bundle.IPA.Close()
	if state.Binding.ReceiptPath != artifacts.receiptPath || state.Binding.IntentPath != artifacts.linkPath {
		return publishExecutionResult{}, fmt.Errorf("%w: local artifact paths", errPrivatePublishIntentConflict)
	}
	if err := validatePrivatePublishIntentState(state, state.Binding, bundle); err != nil {
		return publishExecutionResult{}, fmt.Errorf("%w: validate private publication intent: %w", errPrivatePublishIntentConflict, err)
	}
	receipt, err := readPrivatePublishIntentReceipt(artifacts)
	if err != nil {
		return publishExecutionResult{}, fmt.Errorf("read private publication receipt: %w", err)
	}
	if err := validatePrivatePublishIntentReceipt(receipt, state, bundle); err != nil {
		return publishExecutionResult{}, fmt.Errorf("%w: validate private publication receipt: %w", errPrivatePublishIntentConflict, err)
	}
	verifier, err := newPrivatePublicationVerifier(request.VerifyTimeout)
	if err != nil {
		return publishExecutionResult{}, fmt.Errorf("configure publication verifier: %w", err)
	}
	if err := reverifyPublication(ctx, verifier, receipt, state.Intent.Links, time.Now().UTC()); err != nil {
		return publishExecutionResult{}, fmt.Errorf("reverify private publication intent: %w", err)
	}
	return publishExecutionResult{Receipt: receipt, Recovered: true}, nil
}

func validatePrivatePublishIntentRequest(request privatePublishIntentRequest) error {
	for _, item := range []struct{ name, value string }{
		{"bundle-dir", request.BundleDir},
		{"endpoint", request.Endpoint},
		{"region", request.Region},
		{"bucket", request.Bucket},
		{"prefix", request.Prefix},
		{"receipt", request.ReceiptPath},
		{"intent-path", request.IntentPath},
	} {
		if strings.TrimSpace(item.value) == "" {
			return shared.UsageErrorf("--%s is required", item.name)
		}
	}
	if _, err := core.ValidateEndpoint(request.Endpoint); err != nil {
		return shared.UsageErrorf("--endpoint: %v", err)
	}
	if strings.TrimSpace(request.DownloadEndpoint) != "" {
		if _, err := core.ValidateEndpoint(request.DownloadEndpoint); err != nil {
			return shared.UsageErrorf("--download-endpoint: %v", err)
		}
	}
	if request.AddressingStyle != "path" && request.AddressingStyle != "virtual" {
		return shared.UsageError("--addressing-style must be path or virtual")
	}
	if !validPrivatePublishLifetime(request.URLTTL, request.DownloadGrace) {
		return shared.UsageError("private link lifetimes must be positive, non-negative, and at most 7d combined")
	}
	if request.VerifyTimeout <= 0 {
		return shared.UsageError("--verify-timeout must be positive")
	}
	if _, err := core.NormalizePrefix(request.Prefix); err != nil {
		return shared.UsageErrorf("--prefix: %v", err)
	}
	if !regionPattern.MatchString(request.Region) {
		return shared.UsageError("invalid object-store region")
	}
	if !validPublishBucket(request.Bucket) {
		return shared.UsageError("invalid object-store bucket")
	}
	if err := validatePrivatePublishBundleAuthorization(request.ExpectedBundle); err != nil {
		return shared.UsageErrorf("authorized prepared bundle: %v", err)
	}
	return nil
}

func validatePrivatePublishBundleAuthorization(expected privatePublishBundleAuthorization) error {
	for name, value := range map[string]string{
		"descriptor SHA-256":  expected.DescriptorSHA256,
		"IPA SHA-256":         expected.IPASHA256,
		"profile SHA-256":     expected.ProfileSHA256,
		"device-set SHA-256":  expected.DeviceSetSHA256,
		"certificate SHA-256": expected.CertificateSHA256,
	} {
		if !distributionDigestPattern.MatchString(value) {
			return fmt.Errorf("%s must be a canonical SHA-256", name)
		}
	}
	if expected.DescriptorSize <= 0 || expected.IPASize <= 0 || expected.DeviceCount <= 0 ||
		strings.TrimSpace(expected.ProfileUUID) == "" || strings.TrimSpace(expected.TeamID) == "" {
		return fmt.Errorf("descriptor size, IPA size, profile UUID, team ID, and device count are required")
	}
	return nil
}

func validateAuthorizedPrivatePublishBundle(bundle *core.PreparedBundle, expected privatePublishBundleAuthorization) error {
	if bundle == nil || bundle.IPA == nil {
		return fmt.Errorf("prepared bundle is unavailable")
	}
	signing := bundle.Descriptor.Signing
	if bundle.DescriptorSHA256 != expected.DescriptorSHA256 || bundle.DescriptorSize != expected.DescriptorSize ||
		bundle.IPASHA256 != expected.IPASHA256 || bundle.IPASize != expected.IPASize ||
		signing.ProfileUUID != expected.ProfileUUID || signing.EmbeddedProfileSHA256 != expected.ProfileSHA256 ||
		signing.TeamID != expected.TeamID || signing.DeviceSetSHA256 != expected.DeviceSetSHA256 || signing.DeviceCount != expected.DeviceCount ||
		!containsFold(signing.ProfileCertificateSHA256Fingerprints, expected.CertificateSHA256) ||
		!containsFold(signing.CodeSignatureVerification.SignerCertificateSHA256Fingerprints, expected.CertificateSHA256) {
		return fmt.Errorf("loaded descriptor, payload, or signing evidence differs from authorization")
	}
	return nil
}

func privatePublishIntentDigest(intent core.PrivatePublishIntent) (string, error) {
	data, err := json.Marshal(intent)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func validatePrivatePublishIntentState(state privatePublishIntentState, binding privatePublishIntentBinding, bundle *core.PreparedBundle) error {
	if state.SchemaVersion != privatePublishIntentStateSchemaVersion || state.Binding != binding {
		return fmt.Errorf("saved intent conflicts with requested destination or local artifact paths")
	}
	digest, err := privatePublishIntentDigest(state.Intent)
	if err != nil || digest != state.IntentSHA256 {
		return fmt.Errorf("saved intent digest conflicts with its contents")
	}
	if state.Intent.Bucket != binding.Bucket || state.Intent.Prefix != binding.Prefix || state.Intent.App != bundle.Descriptor.App || !state.Intent.Signing.MatchesPrepared(bundle.Descriptor.Signing) ||
		state.Intent.Artifact.SHA256 != bundle.IPASHA256 || state.Intent.Artifact.SizeBytes != bundle.IPASize ||
		state.Intent.URLTTL != binding.RequestedURLTTL || state.Intent.DownloadGrace != binding.DownloadGrace {
		return fmt.Errorf("saved intent conflicts with prepared bundle evidence")
	}
	for _, item := range []struct{ raw, key, label string }{
		{state.Intent.Links.ArtifactURL, state.Intent.Artifact.Key, "artifact"},
		{state.Intent.Links.ManifestURL, state.Intent.Manifest.Key, "manifest"},
		{state.Intent.Links.InstallURL, state.Intent.Page.Key, "page"},
	} {
		if err := validatePrivateIntentDestinationURL(item.raw, item.key, binding); err != nil {
			return fmt.Errorf("saved %s URL: %w", item.label, err)
		}
	}
	return nil
}

func validatePrivateIntentDestinationURL(raw, key string, binding privatePublishIntentBinding) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || parsed.Query().Get("X-Amz-Signature") == "" {
		return fmt.Errorf("must be a signed HTTPS URL")
	}
	base, err := core.ValidateEndpoint(binding.DownloadEndpoint)
	if err != nil {
		return fmt.Errorf("invalid bound download endpoint")
	}
	wantHost := base.Host
	wantPath := "/" + binding.Bucket + "/" + key
	if binding.AddressingStyle == "virtual" {
		wantHost = binding.Bucket + "." + base.Hostname()
		if base.Port() != "" {
			wantHost += ":" + base.Port()
		}
		wantPath = "/" + key
	}
	if !strings.EqualFold(parsed.Host, wantHost) || parsed.Path != wantPath {
		return fmt.Errorf("does not match the bound endpoint, bucket, addressing style, and object key")
	}
	return nil
}

func loadPrivatePublishIntentState(paths artifactPaths) (privatePublishIntentState, bool, error) {
	parent, name, err := paths.openExistingParent(paths.link)
	if err != nil {
		if os.IsNotExist(err) {
			return privatePublishIntentState{}, false, nil
		}
		return privatePublishIntentState{}, false, err
	}
	defer parent.Close()
	file, err := secureopen.OpenExistingNoFollowInRoot(parent, name)
	if os.IsNotExist(err) {
		return privatePublishIntentState{}, false, nil
	}
	if err != nil {
		return privatePublishIntentState{}, false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return privatePublishIntentState{}, true, err
	}
	if err := validateProtectedPublishArtifact(file, info, "private publication intent"); err != nil {
		return privatePublishIntentState{}, true, err
	}
	if info.Size() > 2<<20 {
		return privatePublishIntentState{}, true, fmt.Errorf("private publication intent exceeds 2 MiB")
	}
	data, err := readStableProtectedPublishArtifact(parent, name, file, info, "private publication intent")
	if err != nil {
		return privatePublishIntentState{}, true, err
	}
	var state privatePublishIntentState
	if err := decodeStrictDistributionJSON(data, &state); err != nil {
		return privatePublishIntentState{}, true, fmt.Errorf("decode private publication intent: %w", err)
	}
	return state, true, nil
}

func publishPrivatePublishIntentState(paths artifactPaths, state privatePublishIntentState) error {
	parent, name, err := paths.openParent(paths.link)
	if err != nil {
		return err
	}
	defer parent.Close()
	staged, err := stageFile(parent, name)
	if err != nil {
		return err
	}
	defer staged.cleanup()
	data, err := encodeJSON(state)
	if err != nil {
		return err
	}
	return staged.publish(data)
}

func readPrivatePublishIntentReceipt(paths artifactPaths) (core.PublishReceipt, error) {
	parent, name, err := paths.openExistingParent(paths.receipt)
	if err != nil {
		return core.PublishReceipt{}, err
	}
	defer parent.Close()
	file, err := secureopen.OpenExistingNoFollowInRoot(parent, name)
	if err != nil {
		return core.PublishReceipt{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return core.PublishReceipt{}, err
	}
	if err := validateProtectedPublishArtifact(file, info, "receipt"); err != nil {
		return core.PublishReceipt{}, err
	}
	if info.Size() > 2<<20 {
		return core.PublishReceipt{}, fmt.Errorf("receipt exceeds 2 MiB")
	}
	data, err := readStableProtectedPublishArtifact(parent, name, file, info, "receipt")
	if err != nil {
		return core.PublishReceipt{}, err
	}
	var receipt core.PublishReceipt
	if err := decodeStrictDistributionJSON(data, &receipt); err != nil {
		return core.PublishReceipt{}, err
	}
	return receipt, nil
}

func validatePrivatePublishIntentReceipt(receipt core.PublishReceipt, state privatePublishIntentState, bundle *core.PreparedBundle) error {
	binding, intent := state.Binding, state.Intent
	if receipt.SchemaVersion != "1" || receipt.Endpoint != binding.Endpoint || receipt.DownloadEndpoint != binding.DownloadEndpoint || receipt.Region != binding.Region || receipt.AddressingStyle != binding.AddressingStyle ||
		receipt.Access != core.AccessPrivate || receipt.PublicBaseURL != "" || receipt.Bucket != binding.Bucket || receipt.Prefix != binding.Prefix || receipt.ReceiptPath != binding.ReceiptPath || receipt.LinkPath != binding.IntentPath || !receipt.Verified {
		return fmt.Errorf("redacted receipt conflicts with saved destination intent")
	}
	if receipt.URLTTL != intent.URLTTL || receipt.DownloadGrace != intent.DownloadGrace || receipt.App != bundle.Descriptor.App || !receipt.Signing.MatchesPrepared(bundle.Descriptor.Signing) ||
		!storedObjectMatchesIntent(receipt.Artifact, intent.Artifact) || !storedObjectMatchesIntent(receipt.Manifest, intent.Manifest.StoredObject) || !storedObjectMatchesIntent(receipt.Page, intent.Page.StoredObject) {
		return fmt.Errorf("redacted receipt conflicts with saved bundle or object intent")
	}
	if receipt.ExpiresAt == nil || !receipt.ExpiresAt.Equal(intent.PageExpiresAt) || receipt.InstallURL != core.RedactedInstallURL(intent.Links) || receipt.DirectInstallURL != core.RedactedDirectInstallURL(intent.Links) {
		return fmt.Errorf("redacted receipt conflicts with saved sensitive links")
	}
	return nil
}

func storedObjectMatchesIntent(got, want core.StoredObject) bool {
	return got.Key == want.Key && got.SHA256 == want.SHA256 && got.SizeBytes == want.SizeBytes && got.ContentType == want.ContentType && (got.Status == "uploaded" || got.Status == "reused")
}

func privateSensitiveLinksEqual(left, right core.SensitiveLinks) bool {
	if left.SchemaVersion != right.SchemaVersion || left.InstallURL != right.InstallURL || left.DirectInstallURL != right.DirectInstallURL || left.ArtifactURL != right.ArtifactURL || left.ManifestURL != right.ManifestURL {
		return false
	}
	if (left.ExpiresAt == nil) != (right.ExpiresAt == nil) {
		return false
	}
	return left.ExpiresAt == nil || left.ExpiresAt.Equal(*right.ExpiresAt)
}
