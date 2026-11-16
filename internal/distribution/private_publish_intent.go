package distribution

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

const privatePublishIntentSchemaVersion = "1"

var privatePublishLinkIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// PrivatePublishDocument is an exact, persistable document publication. Body
// is encoded as base64 by encoding/json and belongs only in protected intent
// state because it may contain bearer URLs.
type PrivatePublishDocument struct {
	StoredObject
	Body []byte `json:"body"`
}

// PrivatePublishIntent contains every nondeterministic choice and generated
// byte required to converge a private publication after a crash. It must be
// persisted before ExecutePrivatePublishIntent is called.
type PrivatePublishIntent struct {
	SchemaVersion     string                 `json:"schemaVersion"`
	CreatedAt         time.Time              `json:"createdAt"`
	PageExpiresAt     time.Time              `json:"pageExpiresAt"`
	DownloadExpiresAt time.Time              `json:"downloadExpiresAt"`
	CredentialLimit   *time.Time             `json:"credentialLimit,omitempty"`
	Bucket            string                 `json:"bucket"`
	Prefix            string                 `json:"prefix"`
	URLTTL            string                 `json:"urlTtl"`
	DownloadGrace     string                 `json:"downloadGrace"`
	LinkID            string                 `json:"linkId"`
	Artifact          StoredObject           `json:"artifact"`
	Manifest          PrivatePublishDocument `json:"manifest"`
	Page              PrivatePublishDocument `json:"page"`
	Links             SensitiveLinks         `json:"links"`
	App               PreparedApp            `json:"app"`
	Signing           ReceiptSigning         `json:"signing"`
}

// Clone returns an independent intent suitable for validation tests and
// callers that must retain an immutable in-memory copy.
func (intent PrivatePublishIntent) Clone() PrivatePublishIntent {
	clone := intent
	clone.Manifest.Body = append([]byte(nil), intent.Manifest.Body...)
	clone.Page.Body = append([]byte(nil), intent.Page.Body...)
	if intent.CredentialLimit != nil {
		value := *intent.CredentialLimit
		clone.CredentialLimit = &value
	}
	if intent.Links.ExpiresAt != nil {
		value := *intent.Links.ExpiresAt
		clone.Links.ExpiresAt = &value
	}
	return clone
}

// PreparePrivatePublishIntent performs every private-publication choice and
// presign before any remote object write. Callers must durably persist the
// returned intent before execution.
func PreparePrivatePublishIntent(ctx context.Context, descriptor PreparedDescriptor, options PublishOptions) (PrivatePublishIntent, error) {
	if options.Store == nil || options.Verifier == nil {
		return PrivatePublishIntent{}, fmt.Errorf("object store and verifier are required")
	}
	if options.Access != AccessPrivate {
		return PrivatePublishIntent{}, fmt.Errorf("private publication intent requires private access")
	}
	if err := validateDescriptor(descriptor, publishClock(options)()); err != nil {
		return PrivatePublishIntent{}, err
	}
	prefix, err := NormalizePrefix(options.Prefix)
	if err != nil {
		return PrivatePublishIntent{}, err
	}
	if err := ValidateBucket(options.Bucket); err != nil {
		return PrivatePublishIntent{}, err
	}
	clock := publishClock(options)
	now := clock()
	urlTTL, grace, err := boundedLifetimes(now, options.URLTTL, options.DownloadGrace, options.CredentialLimit, AccessPrivate)
	if err != nil {
		return PrivatePublishIntent{}, err
	}
	pageDeadline := now.Add(urlTTL)
	downloadDeadline := pageDeadline.Add(grace)
	profileExpiry, _ := time.Parse(time.RFC3339, descriptor.Signing.ExpiresAt)
	if !profileExpiry.After(downloadDeadline.Add(time.Minute)) {
		return PrivatePublishIntent{}, fmt.Errorf("%w: signing profile expires too soon for the requested link lifetime and safety margin", ErrPrivatePublishProfileExpired)
	}

	randomID := randomLinkID
	if options.RandomID != nil {
		randomID = options.RandomID
	}
	linkID, err := randomID()
	if err != nil {
		return PrivatePublishIntent{}, fmt.Errorf("generate link ID: %w", err)
	}
	if !privatePublishLinkIDPattern.MatchString(linkID) {
		return PrivatePublishIntent{}, fmt.Errorf("generated link ID is not a bounded path-safe identifier")
	}

	artifact := StoredObject{
		Key:         path.Join(prefix, "objects", "sha256", strings.ToLower(descriptor.Artifact.SHA256)+".ipa"),
		SHA256:      strings.ToLower(descriptor.Artifact.SHA256),
		SizeBytes:   descriptor.Artifact.SizeBytes,
		ContentType: ContentTypeIPA,
	}
	artifactURL, err := presignIntentObject(ctx, options.Store, artifact.Key, downloadDeadline.Sub(clock()))
	if err != nil {
		return PrivatePublishIntent{}, fmt.Errorf("create IPA URL: %w", err)
	}
	manifestBody, err := makeManifest(descriptor.App, artifactURL)
	if err != nil {
		return PrivatePublishIntent{}, fmt.Errorf("generate manifest: %w", err)
	}
	manifest := privatePublishDocument(path.Join(prefix, "links", linkID, "manifest.plist"), manifestBody, ContentTypeManifest)
	manifestURL, err := presignIntentObject(ctx, options.Store, manifest.Key, downloadDeadline.Sub(clock()))
	if err != nil {
		return PrivatePublishIntent{}, fmt.Errorf("create manifest URL: %w", err)
	}
	directInstallURL := "itms-services://?action=download-manifest&url=" + urlQueryEscape(manifestURL)
	pageBody, err := makeInstallPage(descriptor.App, directInstallURL)
	if err != nil {
		return PrivatePublishIntent{}, fmt.Errorf("generate install page: %w", err)
	}
	page := privatePublishDocument(path.Join(prefix, "links", linkID, "index.html"), pageBody, ContentTypeHTML)
	pageURL, err := presignIntentObject(ctx, options.Store, page.Key, pageDeadline.Sub(clock()))
	if err != nil {
		return PrivatePublishIntent{}, fmt.Errorf("create install page URL: %w", err)
	}

	expires := pageDeadline
	intent := PrivatePublishIntent{
		SchemaVersion: privatePublishIntentSchemaVersion, CreatedAt: now, PageExpiresAt: pageDeadline, DownloadExpiresAt: downloadDeadline,
		Bucket: options.Bucket, Prefix: prefix, URLTTL: urlTTL.String(), DownloadGrace: grace.String(), LinkID: linkID,
		Artifact: artifact, Manifest: manifest, Page: page,
		Links: SensitiveLinks{SchemaVersion: "1", InstallURL: pageURL, DirectInstallURL: directInstallURL, ArtifactURL: artifactURL, ManifestURL: manifestURL, ExpiresAt: &expires},
		App:   descriptor.App, Signing: receiptSigningFromPrepared(descriptor.Signing),
	}
	if !options.CredentialLimit.IsZero() {
		limit := options.CredentialLimit.UTC()
		intent.CredentialLimit = &limit
	}
	if err := validatePrivatePublishIntent(descriptor, options, intent, now); err != nil {
		return PrivatePublishIntent{}, fmt.Errorf("validate prepared private publication intent: %w", err)
	}
	return intent, nil
}

// ExecutePrivatePublishIntent converges exactly the saved destinations in IPA,
// manifest, page order. It never generates an identifier or presigned URL.
func ExecutePrivatePublishIntent(ctx context.Context, ipa io.ReadSeeker, descriptor PreparedDescriptor, options PublishOptions, intent PrivatePublishIntent) (PublishReceipt, SensitiveLinks, error) {
	if options.Store == nil || options.Verifier == nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("object store and verifier are required")
	}
	now := publishClock(options)()
	if err := validatePrivatePublishIntent(descriptor, options, intent, now); err != nil {
		return PublishReceipt{}, SensitiveLinks{}, err
	}
	if err := verifyIntentIPA(ipa, intent.Artifact); err != nil {
		return PublishReceipt{}, SensitiveLinks{}, err
	}

	artifact, err := ensureIntentObject(ctx, options.Store, PutObject{Key: intent.Artifact.Key, Body: ipa, SHA256: intent.Artifact.SHA256, SizeBytes: intent.Artifact.SizeBytes, ContentType: intent.Artifact.ContentType}, intent.Artifact)
	if err != nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("publish IPA: %w", err)
	}
	if err := options.Verifier.Verify(ctx, VerifyRequest{URL: intent.Links.ArtifactURL, Kind: VerifyIPA, SHA256: artifact.SHA256, SizeBytes: artifact.SizeBytes, ContentType: artifact.ContentType}); err != nil {
		if errors.Is(err, ErrVerificationContentConflict) {
			return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("%w: IPA fetch conflicts with persisted publication intent: %w", ErrPrivatePublishConflict, err)
		}
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("verify IPA: %w", err)
	}

	manifest, err := ensureIntentObject(ctx, options.Store, PutObject{Key: intent.Manifest.Key, Body: bytes.NewReader(intent.Manifest.Body), SHA256: intent.Manifest.SHA256, SizeBytes: intent.Manifest.SizeBytes, ContentType: intent.Manifest.ContentType}, intent.Manifest.StoredObject)
	if err != nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("publish manifest: %w", err)
	}
	if err := options.Verifier.Verify(ctx, VerifyRequest{URL: intent.Links.ManifestURL, Kind: VerifyDocument, SHA256: manifest.SHA256, SizeBytes: manifest.SizeBytes, ContentType: manifest.ContentType}); err != nil {
		if errors.Is(err, ErrVerificationContentConflict) {
			return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("%w: manifest fetch conflicts with persisted publication intent: %w", ErrPrivatePublishConflict, err)
		}
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("verify manifest: %w", err)
	}

	pageObject, err := ensureIntentObject(ctx, options.Store, PutObject{Key: intent.Page.Key, Body: bytes.NewReader(intent.Page.Body), SHA256: intent.Page.SHA256, SizeBytes: intent.Page.SizeBytes, ContentType: intent.Page.ContentType}, intent.Page.StoredObject)
	if err != nil {
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("publish install page: %w", err)
	}
	if err := options.Verifier.Verify(ctx, VerifyRequest{URL: intent.Links.InstallURL, Kind: VerifyDocument, SHA256: pageObject.SHA256, SizeBytes: pageObject.SizeBytes, ContentType: pageObject.ContentType}); err != nil {
		if errors.Is(err, ErrVerificationContentConflict) {
			return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("%w: install-page fetch conflicts with persisted publication intent: %w", ErrPrivatePublishConflict, err)
		}
		return PublishReceipt{}, SensitiveLinks{}, fmt.Errorf("verify install page: %w", err)
	}

	expires := intent.PageExpiresAt
	receipt := PublishReceipt{
		SchemaVersion: "1", Access: AccessPrivate, Bucket: intent.Bucket, Prefix: intent.Prefix,
		URLTTL: intent.URLTTL, DownloadGrace: intent.DownloadGrace,
		Artifact: artifact, Manifest: manifest, Page: pageObject,
		InstallURL: RedactedInstallURL(intent.Links), DirectInstallURL: RedactedDirectInstallURL(intent.Links),
		ExpiresAt: &expires, Verified: true, App: intent.App, Signing: intent.Signing,
	}
	return receipt, intent.Links, nil
}

// RedactedInstallURL returns the exact non-secret install-page URL form used in
// a public receipt. It is safe for logs and structured output.
func RedactedInstallURL(links SensitiveLinks) string {
	return redactBearerURL(links.InstallURL)
}

// RedactedDirectInstallURL returns the exact non-secret itms-services URL form
// used in a public receipt.
func RedactedDirectInstallURL(links SensitiveLinks) string {
	return redactDirectInstallURL(links.DirectInstallURL)
}

func validatePrivatePublishIntent(descriptor PreparedDescriptor, options PublishOptions, intent PrivatePublishIntent, now time.Time) error {
	if options.Access != AccessPrivate {
		return fmt.Errorf("private publication intent requires private access")
	}
	if err := validateDescriptor(descriptor, now); err != nil {
		return err
	}
	prefix, err := NormalizePrefix(options.Prefix)
	if err != nil {
		return err
	}
	if intent.SchemaVersion != privatePublishIntentSchemaVersion || intent.CreatedAt.IsZero() || intent.PageExpiresAt.IsZero() || intent.DownloadExpiresAt.IsZero() {
		return fmt.Errorf("invalid private publication intent schema or timestamps")
	}
	if intent.Bucket != options.Bucket || intent.Prefix != prefix || intent.App != descriptor.App || !intent.Signing.MatchesPrepared(descriptor.Signing) {
		return fmt.Errorf("private publication intent conflicts with destination or prepared bundle")
	}
	urlTTL, err := time.ParseDuration(intent.URLTTL)
	if err != nil || urlTTL <= 0 {
		return fmt.Errorf("private publication intent has invalid URL lifetime")
	}
	grace, err := time.ParseDuration(intent.DownloadGrace)
	if err != nil || grace < 0 || urlTTL+grace > maxLinkLifetime {
		return fmt.Errorf("private publication intent has invalid download grace")
	}
	if !intent.PageExpiresAt.Equal(intent.CreatedAt.Add(urlTTL)) || !intent.DownloadExpiresAt.Equal(intent.PageExpiresAt.Add(grace)) {
		return fmt.Errorf("private publication intent expiry bounds conflict")
	}
	if !intent.PageExpiresAt.After(now) {
		return fmt.Errorf("%w: private publication intent install link is expired", ErrPrivatePublishLinkExpired)
	}
	if intent.CredentialLimit != nil && intent.DownloadExpiresAt.After(intent.CredentialLimit.Add(-time.Minute)) {
		return fmt.Errorf("private publication intent exceeds saved credential expiry")
	}
	profileExpiry, err := time.Parse(time.RFC3339, descriptor.Signing.ExpiresAt)
	if err != nil || !profileExpiry.After(intent.DownloadExpiresAt.Add(time.Minute)) {
		return fmt.Errorf("%w: private publication intent exceeds signing profile expiry", ErrPrivatePublishProfileExpired)
	}
	if !privatePublishLinkIDPattern.MatchString(intent.LinkID) {
		return fmt.Errorf("private publication intent has invalid link ID")
	}
	wantArtifactKey := path.Join(prefix, "objects", "sha256", strings.ToLower(descriptor.Artifact.SHA256)+".ipa")
	wantManifestKey := path.Join(prefix, "links", intent.LinkID, "manifest.plist")
	wantPageKey := path.Join(prefix, "links", intent.LinkID, "index.html")
	if intent.Artifact.Key != wantArtifactKey || intent.Artifact.SHA256 != strings.ToLower(descriptor.Artifact.SHA256) || intent.Artifact.SizeBytes != descriptor.Artifact.SizeBytes || intent.Artifact.ContentType != ContentTypeIPA || intent.Artifact.Status != "" {
		return fmt.Errorf("private publication intent artifact evidence conflicts")
	}
	if intent.Manifest.Key != wantManifestKey || intent.Page.Key != wantPageKey || intent.Manifest.Status != "" || intent.Page.Status != "" {
		return fmt.Errorf("private publication intent document keys conflict")
	}
	if intent.Links.SchemaVersion != "1" || intent.Links.ExpiresAt == nil || !intent.Links.ExpiresAt.Equal(intent.PageExpiresAt) {
		return fmt.Errorf("private publication intent sensitive-link expiry conflicts")
	}
	for _, link := range []struct {
		label    string
		rawURL   string
		deadline time.Time
	}{
		{label: "artifact", rawURL: intent.Links.ArtifactURL, deadline: intent.DownloadExpiresAt},
		{label: "manifest", rawURL: intent.Links.ManifestURL, deadline: intent.DownloadExpiresAt},
		{label: "page", rawURL: intent.Links.InstallURL, deadline: intent.PageExpiresAt},
	} {
		if err := requireHTTPSURL(link.rawURL); err != nil {
			return fmt.Errorf("private publication intent %s URL: %w", link.label, err)
		}
		if err := privateSignatureWithinDeadline(link.rawURL, link.deadline); err != nil {
			return fmt.Errorf("private publication intent %s URL: %w", link.label, err)
		}
	}
	wantManifest, err := makeManifest(descriptor.App, intent.Links.ArtifactURL)
	if err != nil || !bytes.Equal(wantManifest, intent.Manifest.Body) || !objectMatchesBytes(intent.Manifest.StoredObject, intent.Manifest.Body, ContentTypeManifest) {
		return fmt.Errorf("private publication intent manifest evidence conflicts")
	}
	wantDirect := "itms-services://?action=download-manifest&url=" + urlQueryEscape(intent.Links.ManifestURL)
	if intent.Links.DirectInstallURL != wantDirect {
		return fmt.Errorf("private publication intent direct install URL conflicts")
	}
	wantPage, err := makeInstallPage(descriptor.App, wantDirect)
	if err != nil || !bytes.Equal(wantPage, intent.Page.Body) || !objectMatchesBytes(intent.Page.StoredObject, intent.Page.Body, ContentTypeHTML) {
		return fmt.Errorf("private publication intent install-page evidence conflicts")
	}
	return nil
}

func publishClock(options PublishOptions) func() time.Time {
	if options.Now == nil {
		return func() time.Time { return time.Now().UTC() }
	}
	return func() time.Time { return options.Now().UTC() }
}

func privatePublishDocument(key string, body []byte, contentType string) PrivatePublishDocument {
	digest := sha256.Sum256(body)
	return PrivatePublishDocument{StoredObject: StoredObject{Key: key, SHA256: hex.EncodeToString(digest[:]), SizeBytes: int64(len(body)), ContentType: contentType}, Body: append([]byte(nil), body...)}
}

func presignIntentObject(ctx context.Context, store ObjectStore, key string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		return "", fmt.Errorf("credential-safe URL lifetime elapsed before presigning")
	}
	raw, err := store.PresignGet(ctx, key, ttl)
	if err != nil {
		return "", err
	}
	if err := requireHTTPSURL(raw); err != nil {
		return "", err
	}
	return raw, nil
}

func ensureIntentObject(ctx context.Context, store ObjectStore, input PutObject, planned StoredObject) (StoredObject, error) {
	got, err := store.Ensure(ctx, input)
	if err != nil {
		if errors.Is(err, ErrImmutableObjectConflict) {
			return StoredObject{}, fmt.Errorf("%w: %w", ErrPrivatePublishConflict, err)
		}
		return StoredObject{}, err
	}
	if got.Key != planned.Key || got.SHA256 != planned.SHA256 || got.SizeBytes != planned.SizeBytes || got.ContentType != planned.ContentType {
		return StoredObject{}, fmt.Errorf("%w: object-store evidence conflicts with persisted publication intent", ErrPrivatePublishConflict)
	}
	return got, nil
}

func verifyIntentIPA(ipa io.ReadSeeker, planned StoredObject) error {
	if ipa == nil {
		return fmt.Errorf("IPA reader is required")
	}
	if _, err := ipa.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind IPA: %w", err)
	}
	digest := sha256.New()
	written, err := io.Copy(digest, io.LimitReader(ipa, planned.SizeBytes+1))
	if err != nil || written != planned.SizeBytes || hex.EncodeToString(digest.Sum(nil)) != planned.SHA256 {
		return fmt.Errorf("%w: IPA content conflicts with persisted publication intent", ErrPrivatePublishConflict)
	}
	if _, err := ipa.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind IPA: %w", err)
	}
	return nil
}

func urlQueryEscape(raw string) string {
	// Kept local so intent construction and validation share one canonical
	// representation without exposing an additional public helper.
	return url.QueryEscape(raw)
}
