package distribution

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func TestLoadPreparedBundleValidatesArtifact(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "payload"), 0o755); err != nil {
		t.Fatal(err)
	}
	ipa := []byte("ipa bytes")
	digest := sha256.Sum256(ipa)
	certificateFingerprint := strings.Repeat("a", 64)
	descriptor := `{"schemaVersion":"1","platform":"IOS","distributionMethod":"release-testing","app":{"bundleId":"com.example.app","title":"Example","version":"1.2","buildNumber":"3"},"artifact":{"relativePath":"payload/app.ipa","sha256":"` + hex.EncodeToString(digest[:]) + `","sizeBytes":9},"signing":{"profileClass":"ad-hoc","profileUuid":"uuid","embeddedProfileSha256":"` + strings.Repeat("c", 64) + `","teamId":"TEAM","expiresAt":"2035-01-01T00:00:00Z","deviceCount":1,"deviceSetSha256":"` + strings.Repeat("b", 64) + `","profileCertificateSha256Fingerprints":["` + certificateFingerprint + `"],"profileIntegrityVerification":{"status":"verified"},"profileTrustVerification":{"status":"verified"},"codeSignatureVerification":{"status":"verified","scope":"complete-main-app-code-resources-entitlements-and-profile-certificate-binding","signerCertificateSha256Fingerprints":["` + certificateFingerprint + `"]}}}`
	if err := os.WriteFile(filepath.Join(dir, "bundle.json"), []byte(descriptor), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "payload", "app.ipa"), ipa, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadPreparedBundle(dir)
	if err != nil {
		t.Fatalf("LoadPreparedBundle() error = %v", err)
	}
	defer got.IPA.Close()
	if got.Descriptor.App.BundleID != "com.example.app" || got.IPASHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("unexpected bundle: %#v", got)
	}

	if err := os.WriteFile(filepath.Join(dir, "payload", "app.ipa"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPreparedBundle(dir); err == nil || !strings.Contains(err.Error(), "size") {
		t.Fatalf("error = %v, want early size mismatch", err)
	}
}

func TestLoadPreparedBundleContextPinsSelectedRoot(t *testing.T) {
	t.Run("symlink retarget keeps original descriptor", func(t *testing.T) {
		base := t.TempDir()
		original := filepath.Join(base, "original")
		replacement := filepath.Join(base, "replacement")
		alias := filepath.Join(base, "selected")
		writePreparedBundleFixture(t, original, []byte("original"), "Original")
		writePreparedBundleFixture(t, replacement, []byte("replacement"), "Replacement")
		if err := os.Symlink(original, alias); err != nil {
			t.Fatal(err)
		}
		root, err := rootfs.New(alias)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(alias); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(replacement, alias); err != nil {
			t.Fatal(err)
		}

		bundle, err := LoadPreparedBundleContext(context.Background(), root)
		if err != nil {
			t.Fatalf("LoadPreparedBundleContext() error = %v", err)
		}
		if bundle.Descriptor.App.Title != "Original" {
			t.Fatalf("loaded title = %q, want pinned original", bundle.Descriptor.App.Title)
		}
		if _, err := os.Stat(filepath.Join(replacement, "receipt.json")); !os.IsNotExist(err) {
			t.Fatalf("retargeted replacement received an unexpected write: %v", err)
		}
		if err := bundle.IPA.Close(); err != nil {
			t.Fatal(err)
		}
		if err := root.Close(); err != nil {
			t.Fatal(err)
		}
		if opened, err := root.OpenRoot(); opened != nil || err == nil {
			if opened != nil {
				_ = opened.Close()
			}
			t.Fatal("pinned descriptor remained usable after Close")
		}
	})

	t.Run("ordinary directory replacement fails closed", func(t *testing.T) {
		base := t.TempDir()
		selected := filepath.Join(base, "selected")
		moved := filepath.Join(base, "moved")
		writePreparedBundleFixture(t, selected, []byte("original"), "Original")
		root, err := rootfs.New(selected)
		if err != nil {
			t.Fatal(err)
		}
		defer root.Close()
		if err := os.Rename(selected, moved); err != nil {
			t.Fatal(err)
		}
		writePreparedBundleFixture(t, selected, []byte("replacement"), "Replacement")

		if bundle, err := LoadPreparedBundleContext(context.Background(), root); err == nil {
			_ = bundle.IPA.Close()
			t.Fatal("LoadPreparedBundleContext() accepted replacement directory")
		}
		if _, err := os.Stat(filepath.Join(selected, "receipt.json")); !os.IsNotExist(err) {
			t.Fatalf("replacement directory received an unexpected write: %v", err)
		}
	})
}

func writePreparedBundleFixture(t *testing.T, dir string, ipa []byte, title string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "payload"), 0o755); err != nil {
		t.Fatal(err)
	}
	descriptor := minimalDescriptor(ipa)
	descriptor.App.Title = title
	encoded, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bundle.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "payload", "app.ipa"), ipa, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestPreparedSigningRejectsConflictingFingerprintAlias(t *testing.T) {
	var descriptor PreparedDescriptor
	err := json.Unmarshal([]byte(`{"signing":{"profileCertificateSha256Fingerprints":["new"],"certificateSha256Fingerprints":["old"]}}`), &descriptor)
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("error = %v, want conflict", err)
	}
}

func TestValidateDescriptorRejectsUnsafePresentationText(t *testing.T) {
	descriptor := minimalDescriptor([]byte("ipa"))
	descriptor.App.Title = "Trusted\u202eexe"
	if err := validateDescriptor(descriptor); err == nil || !strings.Contains(err.Error(), "bidi") {
		t.Fatalf("error = %v, want bidi rejection", err)
	}
}

func TestValidateDescriptorRequiresAllVerificationsAndExactFullScope(t *testing.T) {
	for _, field := range []string{"integrity", "trust", "code", "scope"} {
		t.Run(field, func(t *testing.T) {
			descriptor := minimalDescriptor([]byte("ipa"))
			switch field {
			case "integrity":
				descriptor.Signing.ProfileIntegrityVerification.Status = ""
			case "trust":
				descriptor.Signing.ProfileTrustVerification.Status = "not-verified"
			case "code":
				descriptor.Signing.CodeSignatureVerification.Status = "unknown"
			case "scope":
				descriptor.Signing.CodeSignatureVerification.Scope = "main-executable-and-profile-certificate-binding"
			}
			if err := validateDescriptor(descriptor); err == nil || (!strings.Contains(err.Error(), "must be verified") && !strings.Contains(err.Error(), "scope must be")) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestValidateDescriptorRequiresSignerFingerprintBoundToProfile(t *testing.T) {
	for name, mutate := range map[string]func(*PreparedDescriptor){
		"missing": func(descriptor *PreparedDescriptor) {
			descriptor.Signing.CodeSignatureVerification.SignerCertificateSHA256Fingerprints = nil
		},
		"malformed": func(descriptor *PreparedDescriptor) {
			descriptor.Signing.CodeSignatureVerification.SignerCertificateSHA256Fingerprints = []string{"not-a-digest"}
		},
		"not in profile": func(descriptor *PreparedDescriptor) {
			descriptor.Signing.CodeSignatureVerification.SignerCertificateSHA256Fingerprints = []string{strings.Repeat("c", 64)}
		},
	} {
		t.Run(name, func(t *testing.T) {
			descriptor := minimalDescriptor([]byte("ipa"))
			mutate(&descriptor)
			if err := validateDescriptor(descriptor); err == nil {
				t.Fatal("expected signer fingerprint validation error")
			}
		})
	}
}

func TestReceiptSigningBindsSignerFingerprintEvidence(t *testing.T) {
	descriptor := minimalDescriptor([]byte("ipa"))
	receipt := receiptSigningFromPrepared(descriptor.Signing)
	if !receipt.MatchesPrepared(descriptor.Signing) {
		t.Fatal("fresh receipt signing facts did not match prepared descriptor")
	}
	receipt.CodeSignatureVerification.SignerCertificateSHA256Fingerprints = []string{strings.Repeat("c", 64)}
	if receipt.MatchesPrepared(descriptor.Signing) {
		t.Fatal("tampered signer fingerprint evidence matched prepared descriptor")
	}
}

func TestPublishOrdersObjectsAndRedactsInstallURL(t *testing.T) {
	store := &fakeObjectStore{}
	verifier := &recordingVerifier{}
	descriptor := minimalDescriptor([]byte("ipa"))
	descriptor.App.Version = "1.2"
	descriptor.App.BuildNumber = "3"
	result, sensitive, err := Publish(context.Background(), bytes.NewReader([]byte("ipa")), descriptor, PublishOptions{
		Store: store, Verifier: verifier, Bucket: "bucket", Prefix: "team/app", Access: AccessPrivate,
		URLTTL: time.Hour, DownloadGrace: 10 * time.Minute, Now: func() time.Time { return time.Unix(1000, 0).UTC() },
		RandomID: func() (string, error) { return "link-id", nil },
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	wantOrder := []string{
		"put:team/app/objects/sha256/78324857e8d9bfa749dc301271df54a6572de9f4c3df8a9507cfa7b7d2b25f8e.ipa",
		"put:team/app/links/link-id/manifest.plist",
		"put:team/app/links/link-id/index.html",
	}
	if !reflect.DeepEqual(store.calls, wantOrder) {
		t.Fatalf("calls = %#v, want %#v", store.calls, wantOrder)
	}
	if strings.Contains(result.InstallURL, "X-Amz-Signature") || !strings.Contains(result.InstallURL, "REDACTED") {
		t.Fatalf("result URL was not redacted: %q", result.InstallURL)
	}
	if !strings.Contains(sensitive.InstallURL, "X-Amz-Signature") {
		t.Fatalf("sensitive URL missing signature: %q", sensitive.InstallURL)
	}
	manifest := string(store.bodies["team/app/links/link-id/manifest.plist"])
	for name, marker := range map[string]string{
		"software-package asset": "software-package",
		"bundle identifier":      descriptor.App.BundleID,
		"intended IPA URL":       "team/app/objects/sha256/" + descriptor.Artifact.SHA256 + ".ipa",
	} {
		if !strings.Contains(manifest, marker) {
			t.Fatalf("manifest missing %s marker %q: %s", name, marker, manifest)
		}
	}
	page := string(store.bodies["team/app/links/link-id/index.html"])
	if !strings.Contains(page, "itms-services://?action=download-manifest") || strings.Contains(page, "<script") {
		t.Fatalf("unexpected install page: %s", page)
	}
	if len(verifier.urls) != 3 {
		t.Fatalf("verified %d URLs, want 3", len(verifier.urls))
	}
}

func TestPublishUploadsImmutableIPASnapshot(t *testing.T) {
	source := bytes.NewReader([]byte("ipa"))
	store := &fakeObjectStore{
		beforeEnsure: func() {
			source.Reset([]byte("bad"))
		},
	}
	descriptor := minimalDescriptor([]byte("ipa"))

	if _, _, err := Publish(context.Background(), source, descriptor, PublishOptions{
		Store: store, Verifier: &recordingVerifier{}, Bucket: "bucket", Prefix: "team/app", Access: AccessPrivate,
		URLTTL: time.Hour, Now: func() time.Time { return time.Unix(1000, 0).UTC() },
		RandomID: func() (string, error) { return "link-id", nil },
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	key := "team/app/objects/sha256/" + strings.ToLower(descriptor.Artifact.SHA256) + ".ipa"
	if got := string(store.bodies[key]); got != "ipa" {
		t.Fatalf("uploaded IPA = %q, want immutable snapshot", got)
	}
}

func TestPublishRejectsMutationDuringSnapshotWithoutUploading(t *testing.T) {
	original := bytes.Repeat([]byte("a"), 128<<10)
	source := &mutatingReadSeeker{data: append([]byte(nil), original...)}
	store := &fakeObjectStore{}
	snapshotDirectory := ""
	previousSnapshotHook := snapshotCreatedForTest
	snapshotCreatedForTest = func(path string) { snapshotDirectory = path }
	t.Cleanup(func() { snapshotCreatedForTest = previousSnapshotHook })

	_, _, err := Publish(context.Background(), source, minimalDescriptor(original), PublishOptions{
		Store: store, Verifier: &recordingVerifier{}, Bucket: "bucket", Prefix: "team/app", Access: AccessPrivate,
		URLTTL: time.Hour, Now: func() time.Time { return time.Unix(1000, 0).UTC() },
	})
	if err == nil || !strings.Contains(err.Error(), "snapshot SHA-256") {
		t.Fatalf("Publish() error = %v, want mutation mismatch", err)
	}
	if len(store.calls) != 0 {
		t.Fatalf("store calls = %v, want no upload", store.calls)
	}
	if snapshotDirectory == "" {
		t.Fatal("snapshot directory was not created")
	}
	if _, statErr := os.Stat(snapshotDirectory); !os.IsNotExist(statErr) {
		t.Fatalf("snapshot directory survived failure: %v", statErr)
	}
}

func TestPublishSnapshotCancellationCleansBeforeUploading(t *testing.T) {
	source := bytes.NewReader(bytes.Repeat([]byte("a"), 128<<10))
	store := &fakeObjectStore{}
	ctx, cancel := context.WithCancel(context.Background())
	snapshotDirectory := ""
	previousCreatedHook := snapshotCreatedForTest
	previousDuringHook := duringIPASnapshotForTest
	snapshotCreatedForTest = func(path string) { snapshotDirectory = path }
	duringIPASnapshotForTest = cancel
	t.Cleanup(func() {
		snapshotCreatedForTest = previousCreatedHook
		duringIPASnapshotForTest = previousDuringHook
	})

	_, _, err := Publish(ctx, source, minimalDescriptor(bytes.Repeat([]byte("a"), 128<<10)), PublishOptions{
		Store: store, Verifier: &recordingVerifier{}, Bucket: "bucket", Prefix: "team/app", Access: AccessPrivate,
		URLTTL: time.Hour, Now: func() time.Time { return time.Unix(1000, 0).UTC() },
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Publish() error = %v, want context.Canceled", err)
	}
	if len(store.calls) != 0 {
		t.Fatalf("store calls = %v, want no upload", store.calls)
	}
	if snapshotDirectory == "" {
		t.Fatal("snapshot directory was not created")
	}
	if _, statErr := os.Stat(snapshotDirectory); !os.IsNotExist(statErr) {
		t.Fatalf("snapshot directory survived cancellation: %v", statErr)
	}
}

func TestPublishConditionallyRepairsPoisonedReusedIPA(t *testing.T) {
	ipa := []byte("ipa")
	key := "team/app/objects/sha256/" + sha256Hex(ipa) + ".ipa"
	store := &repairingObjectStore{fakeObjectStore: fakeObjectStore{
		objects: map[string]StoredObject{
			key: {Key: key, SHA256: sha256Hex(ipa), SizeBytes: int64(len(ipa)), ContentType: ContentTypeIPA},
		},
		bodies: map[string][]byte{key: []byte("bad")},
	}}
	verifier := &mismatchOnceVerifier{}

	if _, _, err := Publish(context.Background(), bytes.NewReader(ipa), minimalDescriptor(ipa), PublishOptions{
		Store: store, Verifier: verifier, Bucket: "bucket", Prefix: "team/app", Access: AccessPrivate,
		URLTTL: time.Hour, Now: func() time.Time { return time.Unix(1000, 0).UTC() },
		RandomID: func() (string, error) { return "link-id", nil },
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if store.repairs != 1 {
		t.Fatalf("conditional repairs = %d, want 1", store.repairs)
	}
	if got := store.bodies[key]; !bytes.Equal(got, ipa) || sha256Hex(got) != sha256Hex(ipa) {
		t.Fatalf("replacement body = %q digest=%s", got, sha256Hex(got))
	}
	if verifier.ipaChecks != 2 {
		t.Fatalf("IPA verification calls = %d, want initial mismatch plus repaired verification", verifier.ipaChecks)
	}
}

func TestPublishRejectsProfileExpiringBeforePrivateLinkWithoutWriting(t *testing.T) {
	store := &fakeObjectStore{}
	now := time.Now().UTC().Truncate(time.Second)
	descriptor := minimalDescriptor([]byte("ipa"))
	descriptor.Signing.ExpiresAt = now.Add(70 * time.Minute).Format(time.RFC3339)
	_, _, err := Publish(context.Background(), bytes.NewReader([]byte("ipa")), descriptor, PublishOptions{
		Store: store, Verifier: &recordingVerifier{}, Bucket: "bucket", Prefix: "app", Access: AccessPrivate,
		URLTTL: time.Hour, DownloadGrace: 10 * time.Minute, Now: func() time.Time { return now },
	})
	if err == nil || !strings.Contains(err.Error(), "profile expires too soon") {
		t.Fatalf("error = %v", err)
	}
	if len(store.calls) != 0 {
		t.Fatalf("store calls = %v, want none", store.calls)
	}
}

func TestPublishRejectsPublicProfileThatExpiresDuringVerification(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	descriptor := minimalDescriptor([]byte("ipa"))
	descriptor.Signing.ExpiresAt = now.Add(2 * time.Minute).Format(time.RFC3339)
	store := &fakeObjectStore{}
	verifier := &advancingVerifier{now: &now, advance: 30 * time.Second}

	_, _, err := Publish(context.Background(), bytes.NewReader([]byte("ipa")), descriptor, PublishOptions{
		Store: store, Verifier: verifier, Bucket: "bucket", Prefix: "app", Access: AccessPublic,
		PublicBaseURL: "https://downloads.example.com", Now: func() time.Time { return now },
		RandomID: func() (string, error) { return "id", nil },
	})
	if err == nil || !strings.Contains(err.Error(), "profile expires too soon") {
		t.Fatalf("Publish() error = %v, want post-verification profile expiry rejection", err)
	}
	if len(store.calls) == 0 {
		t.Fatal("expected publication work before final profile validity check")
	}
}

func TestReverifyRejectsPublicProfileThatExpiresDuringVerification(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	profileExpiry := now.Add(2 * time.Minute)
	receipt := PublishReceipt{
		SchemaVersion: "1", Access: AccessPublic, PublicBaseURL: "https://downloads.example.com",
		Artifact: StoredObject{Key: "prefix/app.ipa", SHA256: "a", SizeBytes: 1, ContentType: ContentTypeIPA},
		Manifest: StoredObject{Key: "prefix/manifest.plist"},
		Page:     StoredObject{Key: "prefix/index.html"},
		App:      PreparedApp{BundleID: "com.example.app", Title: "Example", Version: "1", BuildNumber: "1"},
		Signing:  ReceiptSigning{ProfileExpiresAt: profileExpiry.Format(time.RFC3339)},
		Verified: true,
	}
	links := SensitiveLinks{
		SchemaVersion: "1",
		ArtifactURL:   "https://downloads.example.com/prefix/app.ipa",
		ManifestURL:   "https://downloads.example.com/prefix/manifest.plist",
		InstallURL:    "https://downloads.example.com/prefix/index.html",
	}
	links.DirectInstallURL = "itms-services://?action=download-manifest&url=" + url.QueryEscape(links.ManifestURL)
	receipt.InstallURL = links.InstallURL
	receipt.DirectInstallURL = redactDirectInstallURL(links.DirectInstallURL)
	bindGeneratedRecoveryObjects(t, &receipt, links)
	verifier := &advancingVerifier{now: &now, advance: 30 * time.Second}

	err := reverifyWithClock(context.Background(), verifier, receipt, links, now, func() time.Time { return now })
	if err == nil || !strings.Contains(err.Error(), "profile is expired or expires too soon") {
		t.Fatalf("Reverify() error = %v, want post-verification profile expiry rejection", err)
	}
}

func TestPublishRejectsUnsafeBucketBeforeWriting(t *testing.T) {
	store := &fakeObjectStore{}
	_, _, err := Publish(context.Background(), bytes.NewReader([]byte("ipa")), minimalDescriptor([]byte("ipa")), PublishOptions{
		Store: store, Verifier: &recordingVerifier{}, Bucket: "bucket\u200bhidden", Prefix: "app", Access: AccessPrivate,
		URLTTL: time.Hour, DownloadGrace: time.Minute,
	})
	if err == nil || !strings.Contains(err.Error(), "bucket") {
		t.Fatalf("Publish() error = %v, want unsafe bucket rejection", err)
	}
	if len(store.calls) != 0 {
		t.Fatalf("store calls = %v, want none", store.calls)
	}
}

func TestPresignedLifetimesShrinkAsPublicationWorkElapses(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	store := &delayedPresignStore{fakeObjectStore: fakeObjectStore{}, now: now}
	descriptor := minimalDescriptor([]byte("ipa"))
	descriptor.Signing.ExpiresAt = now.Add(48 * time.Hour).Format(time.RFC3339)
	_, _, err := Publish(context.Background(), bytes.NewReader([]byte("ipa")), descriptor, PublishOptions{
		Store: store, Verifier: &recordingVerifier{}, Bucket: "bucket", Prefix: "app", Access: AccessPrivate,
		URLTTL: time.Hour, DownloadGrace: 10 * time.Minute, Now: func() time.Time { return store.now }, RandomID: func() (string, error) { return "id", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []time.Duration{70 * time.Minute, 69 * time.Minute, 48 * time.Minute}
	if !reflect.DeepEqual(store.ttls, want) {
		t.Fatalf("presigned TTLs = %v, want %v", store.ttls, want)
	}
}

func TestResolveObjectURLRetriesSignerTimestampPastDeadlineBudget(t *testing.T) {
	now := time.Date(2026, time.August, 14, 10, 0, 0, 0, time.UTC)
	deadline := now.Add(time.Hour)
	for _, test := range []struct {
		name           string
		deadlineOffset time.Duration
		signedAt       []time.Time
		wantCalls      int
		wantError      bool
	}{
		{name: "exact signer timestamp", signedAt: []time.Time{now}, wantCalls: 1},
		{name: "fractional deadline overshoot", deadlineOffset: 500 * time.Millisecond, signedAt: []time.Time{now, now}, wantCalls: 2},
		{name: "one-second delayed signer timestamp", signedAt: []time.Time{now.Add(time.Second), now.Add(time.Second)}, wantCalls: 2},
		{name: "persistent signer delay", signedAt: []time.Time{now.Add(time.Second), now.Add(2 * time.Second)}, wantCalls: 2, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &fixedSignatureStore{signedAt: test.signedAt}
			_, err := resolveObjectURL(context.Background(), PublishOptions{Store: store, Access: AccessPrivate}, "prefix/app.ipa", deadline.Add(test.deadlineOffset), func() time.Time { return now })
			if test.wantError {
				if err == nil || !strings.Contains(err.Error(), "signed expiry does not match") {
					t.Fatalf("resolveObjectURL() error = %v, want deadline mismatch", err)
				}
			} else if err != nil {
				t.Fatalf("resolveObjectURL() error = %v", err)
			}
			if store.calls != test.wantCalls {
				t.Fatalf("PresignGet() calls = %d, want %d", store.calls, test.wantCalls)
			}
		})
	}
}

func TestPublishReusesExactObjectAndRejectsConflict(t *testing.T) {
	ipa := []byte("ipa")
	key := "prefix/objects/sha256/" + sha256Hex(ipa) + ".ipa"
	store := &fakeObjectStore{objects: map[string]StoredObject{
		key: {Key: key, SHA256: sha256Hex(ipa), SizeBytes: 3, ContentType: ContentTypeIPA},
	}}
	_, _, err := Publish(context.Background(), bytes.NewReader(ipa), minimalDescriptor(ipa), PublishOptions{
		Store: store, Verifier: &recordingVerifier{}, Bucket: "bucket", Prefix: "prefix", Access: AccessPrivate,
		URLTTL: time.Hour, DownloadGrace: time.Minute, RandomID: func() (string, error) { return "id", nil },
	})
	if err != nil {
		t.Fatalf("exact reuse error = %v", err)
	}
	if len(store.calls) == 0 || store.calls[0] != "reuse:"+key {
		t.Fatalf("calls = %#v", store.calls)
	}

	store = &fakeObjectStore{objects: map[string]StoredObject{key: {Key: key, SHA256: "wrong", SizeBytes: 3, ContentType: ContentTypeIPA}}}
	_, _, err = Publish(context.Background(), bytes.NewReader(ipa), minimalDescriptor(ipa), PublishOptions{
		Store: store, Verifier: &recordingVerifier{}, Bucket: "bucket", Prefix: "prefix", Access: AccessPrivate,
		URLTTL: time.Hour, DownloadGrace: time.Minute, RandomID: func() (string, error) { return "id", nil },
	})
	if err == nil || !strings.Contains(err.Error(), "immutable object conflict") {
		t.Fatalf("error = %v, want immutable conflict", err)
	}
}

func TestValidateEndpointRequiresUnadornedHTTPS(t *testing.T) {
	for _, raw := range []string{"http://objects.example.com", "https://user@example.com", "https://objects.example.com/path", "https://objects.example.com?x=1", "https://objects.example.com/#frag"} {
		t.Run(raw, func(t *testing.T) {
			if _, err := ValidateEndpoint(raw); err == nil {
				t.Fatalf("ValidateEndpoint(%q) unexpectedly succeeded", raw)
			}
		})
	}
	got, err := ValidateEndpoint("https://objects.example.com")
	if err != nil || got.String() != "https://objects.example.com" {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestPublicObjectURLPreservesBasePathWithoutDoubleEncoding(t *testing.T) {
	got, err := PublicObjectURL("https://downloads.example.com/team%20name/%E2%9C%93", "app path/100%.ipa")
	if err != nil {
		t.Fatal(err)
	}
	const want = "https://downloads.example.com/team%20name/%E2%9C%93/app%20path/100%25.ipa"
	if got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
}

func TestBoundedLifetimesPreservesGraceWhenCredentialsExpireSooner(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	ttl, grace, err := boundedLifetimes(now, 24*time.Hour, time.Hour, now.Add(10*time.Hour), AccessPrivate)
	if err != nil {
		t.Fatal(err)
	}
	if grace != time.Hour || ttl != 8*time.Hour+59*time.Minute {
		t.Fatalf("ttl=%s grace=%s", ttl, grace)
	}
}

func TestBoundedLifetimesValidatesWithoutDurationOverflow(t *testing.T) {
	for _, test := range []struct {
		name      string
		ttl       time.Duration
		grace     time.Duration
		access    Access
		wantTTL   time.Duration
		wantGrace time.Duration
		wantErr   string
	}{
		{name: "exactly seven days", ttl: 6 * 24 * time.Hour, grace: 24 * time.Hour, access: AccessPrivate, wantTTL: 6 * 24 * time.Hour, wantGrace: 24 * time.Hour},
		{name: "one nanosecond over", ttl: maxLinkLifetime, grace: time.Nanosecond, access: AccessPrivate, wantErr: "plus download grace"},
		{name: "maximum URL TTL", ttl: time.Duration(1<<63 - 1), grace: 100 * time.Hour, access: AccessPrivate, wantErr: "URL TTL must not exceed"},
		{name: "maximum download grace", ttl: time.Hour, grace: time.Duration(1<<63 - 1), access: AccessPrivate, wantErr: "download grace must not exceed"},
		{name: "zero URL TTL", grace: time.Hour, access: AccessPrivate, wantErr: "URL TTL must be positive"},
		{name: "negative URL TTL", ttl: -time.Nanosecond, grace: time.Hour, access: AccessPrivate, wantErr: "URL TTL must be positive"},
		{name: "negative download grace", ttl: time.Hour, grace: -time.Nanosecond, access: AccessPrivate, wantErr: "download grace must not be negative"},
		{name: "public ignores private lifetime inputs", ttl: time.Duration(1<<63 - 1), grace: time.Duration(1<<63 - 1), access: AccessPublic},
	} {
		t.Run(test.name, func(t *testing.T) {
			ttl, grace, err := boundedLifetimes(time.Unix(1000, 0), test.ttl, test.grace, time.Time{}, test.access)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("boundedLifetimes() error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("boundedLifetimes() error = %v", err)
			}
			if ttl != test.wantTTL || grace != test.wantGrace {
				t.Fatalf("boundedLifetimes() = (%s, %s), want (%s, %s)", ttl, grace, test.wantTTL, test.wantGrace)
			}
		})
	}
}

func TestReverifyRejectsTamperedSensitiveLinkBinding(t *testing.T) {
	receipt := PublishReceipt{
		SchemaVersion: "1", Access: AccessPrivate, DownloadEndpoint: "https://host", AddressingStyle: "virtual", Bucket: "",
		Artifact:   StoredObject{Key: "prefix/app.ipa", SHA256: "a", SizeBytes: 1, ContentType: ContentTypeIPA},
		Manifest:   StoredObject{Key: "prefix/manifest.plist", SHA256: "b", SizeBytes: 1, ContentType: ContentTypeManifest},
		Page:       StoredObject{Key: "prefix/index.html", SHA256: "c", SizeBytes: 1, ContentType: ContentTypeHTML},
		InstallURL: "https://host/prefix/index.html?REDACTED", DirectInstallURL: "itms-services://?action=download-manifest&url=REDACTED",
	}
	expires := time.Now().Add(time.Hour)
	receipt.ExpiresAt = &expires
	links := SensitiveLinks{SchemaVersion: "1", InstallURL: "https://host/prefix/index.html?secret=one", ManifestURL: "https://host/prefix/manifest.plist?secret=two", ArtifactURL: "https://host/prefix/app.ipa?secret=three", ExpiresAt: &expires}
	links.DirectInstallURL = "itms-services://?action=download-manifest&url=" + url.QueryEscape(links.ManifestURL)
	receipt.InstallURL = redactBearerURL(links.InstallURL)
	receipt.DirectInstallURL = redactDirectInstallURL(links.DirectInstallURL)
	markRecoveryFixtureVerified(&receipt)
	links.ManifestURL = "https://evil.example/other.plist?secret=tampered"
	if err := Reverify(context.Background(), &recordingVerifier{}, receipt, links, time.Now()); err == nil || !strings.Contains(err.Error(), "does not reference") {
		t.Fatalf("error = %v", err)
	}
}

func TestReverifyRejectsDifferentPrivateOriginWithExpectedObjectPath(t *testing.T) {
	expires := time.Now().Add(time.Hour)
	receipt := PublishReceipt{
		SchemaVersion: "1", Access: AccessPrivate, DownloadEndpoint: "https://downloads.example.com", AddressingStyle: "path", Bucket: "bucket",
		Artifact:  StoredObject{Key: "prefix/app.ipa", SHA256: "a", SizeBytes: 1, ContentType: ContentTypeIPA},
		Manifest:  StoredObject{Key: "prefix/manifest.plist", SHA256: "b", SizeBytes: 1, ContentType: ContentTypeManifest},
		Page:      StoredObject{Key: "prefix/index.html", SHA256: "c", SizeBytes: 1, ContentType: ContentTypeHTML},
		ExpiresAt: &expires,
	}
	links := SensitiveLinks{
		SchemaVersion: "1", ExpiresAt: &expires,
		InstallURL:  "https://evil.example/bucket/prefix/index.html?X-Amz-Signature=one",
		ManifestURL: "https://evil.example/bucket/prefix/manifest.plist?X-Amz-Signature=two",
		ArtifactURL: "https://evil.example/bucket/prefix/app.ipa?X-Amz-Signature=three",
	}
	links.DirectInstallURL = "itms-services://?action=download-manifest&url=" + url.QueryEscape(links.ManifestURL)
	receipt.InstallURL = redactBearerURL(links.InstallURL)
	receipt.DirectInstallURL = redactDirectInstallURL(links.DirectInstallURL)
	markRecoveryFixtureVerified(&receipt)
	if err := Reverify(context.Background(), &recordingVerifier{}, receipt, links, time.Now()); err == nil || !strings.Contains(err.Error(), "download endpoint") {
		t.Fatalf("error = %v", err)
	}
}

func TestReverifyAcceptsBoundPrivatePathStyleLinks(t *testing.T) {
	signedAt := time.Date(2026, time.August, 14, 10, 0, 0, 0, time.UTC)
	expires := signedAt.Add(time.Hour)
	receipt := PublishReceipt{
		SchemaVersion: "1", Access: AccessPrivate, DownloadEndpoint: "https://downloads.example.com", AddressingStyle: "path", Bucket: "bucket",
		Artifact:      StoredObject{Key: "prefix/app.ipa", SHA256: "a", SizeBytes: 1, ContentType: ContentTypeIPA},
		Manifest:      StoredObject{Key: "prefix/manifest.plist", SHA256: "b", SizeBytes: 1, ContentType: ContentTypeManifest},
		Page:          StoredObject{Key: "prefix/index.html", SHA256: "c", SizeBytes: 1, ContentType: ContentTypeHTML},
		DownloadGrace: "0s", ExpiresAt: &expires,
	}
	links := SensitiveLinks{
		SchemaVersion: "1", ExpiresAt: &expires,
		InstallURL:  privateSignatureFixture("/bucket/prefix/index.html", signedAt, time.Hour),
		ManifestURL: privateSignatureFixture("/bucket/prefix/manifest.plist", signedAt, time.Hour),
		ArtifactURL: privateSignatureFixture("/bucket/prefix/app.ipa", signedAt, time.Hour),
	}
	links.DirectInstallURL = "itms-services://?action=download-manifest&url=" + url.QueryEscape(links.ManifestURL)
	receipt.InstallURL = redactBearerURL(links.InstallURL)
	receipt.DirectInstallURL = redactDirectInstallURL(links.DirectInstallURL)
	receipt.Verified = true
	receipt.Signing.ProfileExpiresAt = expires.Add(time.Hour).Format(time.RFC3339)
	bindGeneratedRecoveryObjects(t, &receipt, links)
	verifier := &recordingVerifier{}
	if err := Reverify(context.Background(), verifier, receipt, links, signedAt.Add(time.Minute)); err != nil {
		t.Fatalf("Reverify() error = %v", err)
	}
	if len(verifier.urls) != 3 {
		t.Fatalf("verified URLs = %d, want 3", len(verifier.urls))
	}
}

func TestReverifyRequiresPrivateSignaturesToMatchReceiptDeadlines(t *testing.T) {
	signedAt := time.Date(2026, time.August, 14, 10, 0, 0, 0, time.UTC)
	pageDeadline := signedAt.Add(time.Hour)
	grace := 10 * time.Minute

	for _, target := range []string{"install", "manifest", "artifact"} {
		for _, test := range []struct {
			name           string
			deadlineOffset time.Duration
			lifetimeOffset time.Duration
			wantError      bool
		}{
			{name: "exact"},
			{name: "subsecond early", deadlineOffset: 500 * time.Millisecond},
			{name: "subsecond late", deadlineOffset: -500 * time.Millisecond},
			{name: "one second early", lifetimeOffset: -time.Second, wantError: true},
			{name: "one second late", lifetimeOffset: time.Second, wantError: true},
		} {
			t.Run(target+"/"+test.name, func(t *testing.T) {
				recordedPageDeadline := pageDeadline.Add(test.deadlineOffset)
				receipt := PublishReceipt{
					SchemaVersion: "1", Access: AccessPrivate, DownloadEndpoint: "https://downloads.example.com", AddressingStyle: "path", Bucket: "bucket",
					DownloadGrace: grace.String(), ExpiresAt: &recordedPageDeadline, Verified: true,
					Artifact: StoredObject{Key: "prefix/app.ipa", SHA256: "a", SizeBytes: 1, ContentType: ContentTypeIPA},
					Manifest: StoredObject{Key: "prefix/manifest.plist", SHA256: "b", SizeBytes: 1, ContentType: ContentTypeManifest},
					Page:     StoredObject{Key: "prefix/index.html", SHA256: "c", SizeBytes: 1, ContentType: ContentTypeHTML},
					Signing:  ReceiptSigning{ProfileExpiresAt: pageDeadline.Add(grace + time.Hour).Format(time.RFC3339)},
				}
				links := SensitiveLinks{
					SchemaVersion: "1", ExpiresAt: &recordedPageDeadline,
					InstallURL:  privateSignatureFixture("/bucket/prefix/index.html", signedAt, time.Hour),
					ManifestURL: privateSignatureFixture("/bucket/prefix/manifest.plist", signedAt, time.Hour+grace),
					ArtifactURL: privateSignatureFixture("/bucket/prefix/app.ipa", signedAt, time.Hour+grace),
				}
				switch target {
				case "install":
					links.InstallURL = privateSignatureFixture("/bucket/prefix/index.html", signedAt, time.Hour+test.lifetimeOffset)
				case "manifest":
					links.ManifestURL = privateSignatureFixture("/bucket/prefix/manifest.plist", signedAt, time.Hour+grace+test.lifetimeOffset)
				case "artifact":
					links.ArtifactURL = privateSignatureFixture("/bucket/prefix/app.ipa", signedAt, time.Hour+grace+test.lifetimeOffset)
				}
				links.DirectInstallURL = "itms-services://?action=download-manifest&url=" + url.QueryEscape(links.ManifestURL)
				receipt.InstallURL = redactBearerURL(links.InstallURL)
				receipt.DirectInstallURL = redactDirectInstallURL(links.DirectInstallURL)
				bindGeneratedRecoveryObjects(t, &receipt, links)

				verifier := &recordingVerifier{}
				err := Reverify(context.Background(), verifier, receipt, links, signedAt.Add(time.Minute))
				if test.wantError {
					if err == nil || !strings.Contains(err.Error(), "signed expiry does not match") {
						t.Fatalf("Reverify() error = %v, want signed deadline mismatch", err)
					}
					if len(verifier.urls) != 0 {
						t.Fatalf("live verifier calls = %d, want none", len(verifier.urls))
					}
					return
				}
				if err != nil {
					t.Fatalf("Reverify() error = %v", err)
				}
				if len(verifier.urls) != 3 {
					t.Fatalf("live verifier calls = %d, want 3", len(verifier.urls))
				}
			})
		}
	}
}

func TestPrivateSignatureMustMatchReceiptDeadlineAtSigningPrecision(t *testing.T) {
	signedAt := time.Date(2026, time.August, 14, 10, 0, 0, 0, time.UTC)
	deadline := signedAt.Add(time.Hour)

	if err := privateSignatureWithinDeadline(privateSignatureFixture("/bucket/app.ipa", signedAt, time.Hour-time.Second), deadline); err == nil {
		t.Fatal("privateSignatureWithinDeadline() accepted a signature expiring before the receipt")
	}
	if err := privateSignatureWithinDeadline(privateSignatureFixture("/bucket/app.ipa", signedAt, time.Hour), deadline.Add(500*time.Millisecond)); err != nil {
		t.Fatalf("privateSignatureWithinDeadline() rejected subsecond receipt precision: %v", err)
	}
}

func TestReverifyNormalizesAndValidatesStoredContentTypes(t *testing.T) {
	receipt := PublishReceipt{
		SchemaVersion: "1", Access: AccessPublic, PublicBaseURL: "https://downloads.example.com",
		Artifact: StoredObject{Key: "prefix/app.ipa", SHA256: "a", SizeBytes: 1, ContentType: ContentTypeIPA},
		Manifest: StoredObject{Key: "prefix/manifest.plist"},
		Page:     StoredObject{Key: "prefix/index.html"},
		App:      PreparedApp{BundleID: "com.example.app", Title: "Example", Version: "1", BuildNumber: "1"},
		Verified: true,
	}
	links := SensitiveLinks{
		SchemaVersion: "1",
		ArtifactURL:   "https://downloads.example.com/prefix/app.ipa",
		ManifestURL:   "https://downloads.example.com/prefix/manifest.plist",
		InstallURL:    "https://downloads.example.com/prefix/index.html",
	}
	links.DirectInstallURL = "itms-services://?action=download-manifest&url=" + url.QueryEscape(links.ManifestURL)
	receipt.InstallURL = links.InstallURL
	receipt.DirectInstallURL = redactDirectInstallURL(links.DirectInstallURL)
	markRecoveryFixtureVerified(&receipt)
	bindGeneratedRecoveryObjects(t, &receipt, links)

	equivalent := receipt
	equivalent.Artifact.ContentType = " Application/Octet-Stream "
	equivalent.Manifest.ContentType = " APPLICATION/XML "
	equivalent.Page.ContentType = " Text/HTML;charset=UTF-8 "
	verifier := &recordingVerifier{}
	if err := Reverify(context.Background(), verifier, equivalent, links, time.Now()); err != nil {
		t.Fatalf("Reverify() rejected equivalent MIME spellings: %v", err)
	}
	if len(verifier.urls) != 3 {
		t.Fatalf("live verifier calls = %d, want 3", len(verifier.urls))
	}

	for _, test := range []struct {
		name   string
		mutate func(*PublishReceipt)
	}{
		{name: "artifact mismatch", mutate: func(value *PublishReceipt) { value.Artifact.ContentType = "text/plain" }},
		{name: "manifest malformed", mutate: func(value *PublishReceipt) { value.Manifest.ContentType = "application/xml; charset" }},
		{name: "page charset mismatch", mutate: func(value *PublishReceipt) { value.Page.ContentType = "text/html; charset=iso-8859-1" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			invalid := receipt
			test.mutate(&invalid)
			verifier := &recordingVerifier{}
			if err := Reverify(context.Background(), verifier, invalid, links, time.Now()); err == nil || !strings.Contains(err.Error(), "content type") {
				t.Fatalf("Reverify() error = %v, want content-type rejection", err)
			}
			if len(verifier.urls) != 0 {
				t.Fatalf("live verifier calls = %d, want none", len(verifier.urls))
			}
		})
	}
}

func TestReverifyRequiresExactPublicBaseURL(t *testing.T) {
	receipt := PublishReceipt{
		SchemaVersion: "1", Access: AccessPublic, PublicBaseURL: "https://downloads.example.com/releases",
		Artifact: StoredObject{Key: "prefix/app.ipa", SHA256: "a", SizeBytes: 1, ContentType: ContentTypeIPA},
		Manifest: StoredObject{Key: "prefix/manifest.plist", SHA256: "b", SizeBytes: 1, ContentType: ContentTypeManifest},
		Page:     StoredObject{Key: "prefix/index.html", SHA256: "c", SizeBytes: 1, ContentType: ContentTypeHTML},
	}
	links := SensitiveLinks{
		SchemaVersion: "1",
		InstallURL:    "https://evil.example/releases/prefix/index.html",
		ManifestURL:   "https://evil.example/releases/prefix/manifest.plist",
		ArtifactURL:   "https://evil.example/releases/prefix/app.ipa",
	}
	links.DirectInstallURL = "itms-services://?action=download-manifest&url=" + url.QueryEscape(links.ManifestURL)
	receipt.InstallURL = links.InstallURL
	receipt.DirectInstallURL = redactDirectInstallURL(links.DirectInstallURL)
	markRecoveryFixtureVerified(&receipt)
	if err := Reverify(context.Background(), &recordingVerifier{}, receipt, links, time.Now()); err == nil || !strings.Contains(err.Error(), "public base") {
		t.Fatalf("error = %v", err)
	}
}

func markRecoveryFixtureVerified(receipt *PublishReceipt) {
	receipt.Verified = true
	receipt.Signing.ProfileExpiresAt = time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
}

func bindGeneratedRecoveryObjects(t *testing.T, receipt *PublishReceipt, links SensitiveLinks) {
	t.Helper()
	manifest, err := makeManifest(receipt.App, links.ArtifactURL)
	if err != nil {
		t.Fatal(err)
	}
	page, err := makeInstallPage(receipt.App, links.DirectInstallURL)
	if err != nil {
		t.Fatal(err)
	}
	manifestDigest := sha256.Sum256(manifest)
	pageDigest := sha256.Sum256(page)
	receipt.Manifest.SHA256 = hex.EncodeToString(manifestDigest[:])
	receipt.Manifest.SizeBytes = int64(len(manifest))
	receipt.Manifest.ContentType = ContentTypeManifest
	receipt.Page.SHA256 = hex.EncodeToString(pageDigest[:])
	receipt.Page.SizeBytes = int64(len(page))
	receipt.Page.ContentType = ContentTypeHTML
}

func privateSignatureFixture(path string, signedAt time.Time, lifetime time.Duration) string {
	query := url.Values{}
	query.Set("X-Amz-Date", signedAt.UTC().Format("20060102T150405Z"))
	query.Set("X-Amz-Expires", strconv.FormatInt(int64(lifetime/time.Second), 10))
	query.Set("X-Amz-Signature", "fixture-signature")
	return (&url.URL{Scheme: "https", Host: "downloads.example.com", Path: path, RawQuery: query.Encode()}).String()
}

func TestVerifyURLRefusesRedirectAndUsesIPARange(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Range") != "" {
			t.Fatalf("Range = %q, want full cryptographic verification", request.Header.Get("Range"))
		}
		return &http.Response{StatusCode: http.StatusTemporaryRedirect, Header: http.Header{"Location": []string{"https://evil.example"}}, Body: io.NopCloser(strings.NewReader("")), Request: request}, nil
	})}
	err := NewHTTPVerifier(client, time.Second).Verify(context.Background(), VerifyRequest{URL: "https://example.com/app.ipa?secret=yes", Kind: VerifyIPA, ContentType: ContentTypeIPA, SizeBytes: 1})
	if err == nil || !strings.Contains(err.Error(), "redirect") || strings.Contains(err.Error(), "secret=yes") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifierProviderHeadersNeverReachDiagnostic(t *testing.T) {
	secret := "X-Amz-Security-Token=secret\x1b[31m"
	client := &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{secret}}, Body: io.NopCloser(strings.NewReader("x")), Request: request}, nil
	})}
	err := NewHTTPVerifier(client, time.Second).Verify(context.Background(), VerifyRequest{URL: "https://example.com/app.ipa", Kind: VerifyIPA, ContentType: ContentTypeIPA, SizeBytes: 1})
	if err == nil || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "\x1b") {
		t.Fatalf("diagnostic leaked provider header: %q", err)
	}
}

func TestHTTPVerifierComparesContentTypeSemantically(t *testing.T) {
	body := []byte("page")
	digest := sha256.Sum256(body)
	client := &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{`TEXT/HTML; LEVEL=1; CHARSET=UTF-8; TITLE="release"`}},
			Body:       io.NopCloser(bytes.NewReader(body)),
			Request:    request,
		}, nil
	})}
	err := NewHTTPVerifier(client, time.Second).Verify(context.Background(), VerifyRequest{
		URL: "https://example.com/index.html", Kind: VerifyDocument,
		ContentType: "text/html; charset=utf-8; title=release; level=1",
		SizeBytes:   int64(len(body)), SHA256: hex.EncodeToString(digest[:]),
	})
	if err != nil {
		t.Fatalf("Verify() rejected reordered equivalent MIME parameters: %v", err)
	}
}

func TestHTTPVerifierClassifiesDeterministicIPAMismatch(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{ContentTypeIPA}, "Content-Length": []string{"3"}},
			Body:       io.NopCloser(strings.NewReader("bad")),
			Request:    request,
		}, nil
	})}
	err := NewHTTPVerifier(client, time.Second).Verify(context.Background(), VerifyRequest{
		URL: "https://example.com/app.ipa", Kind: VerifyIPA, ContentType: ContentTypeIPA,
		SizeBytes: 3, SHA256: sha256Hex([]byte("ipa")),
	})
	if !errors.Is(err, ErrObjectVerificationMismatch) {
		t.Fatalf("Verify() error = %v, want ErrObjectVerificationMismatch", err)
	}
}

func TestHTTPVerifierDoesNotClassifyIPAReadFailureAsMismatch(t *testing.T) {
	readErr := errors.New("connection reset")
	client := &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK, ContentLength: 3,
			Header:  http.Header{"Content-Type": []string{ContentTypeIPA}},
			Body:    io.NopCloser(io.MultiReader(strings.NewReader("i"), &fixedReadError{err: readErr})),
			Request: request,
		}, nil
	})}
	err := NewHTTPVerifier(client, time.Second).Verify(context.Background(), VerifyRequest{
		URL: "https://example.com/app.ipa", Kind: VerifyIPA, ContentType: ContentTypeIPA,
		SizeBytes: 3, SHA256: sha256Hex([]byte("ipa")),
	})
	if err == nil {
		t.Fatal("Verify() unexpectedly accepted an interrupted IPA response")
	}
	if errors.Is(err, ErrObjectVerificationMismatch) {
		t.Fatalf("Verify() error = %v, must not classify a read failure as deterministic corruption", err)
	}
}

type fixedReadError struct{ err error }

func (reader *fixedReadError) Read([]byte) (int, error) {
	return 0, reader.err
}

type fakeObjectStore struct {
	objects      map[string]StoredObject
	bodies       map[string][]byte
	calls        []string
	beforeEnsure func()
}

type delayedPresignStore struct {
	fakeObjectStore
	now  time.Time
	ttls []time.Duration
}

type repairingObjectStore struct {
	fakeObjectStore
	repairs int
}

func (store *repairingObjectStore) ReplaceCorrupt(_ context.Context, input PutObject) (StoredObject, error) {
	store.repairs++
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return StoredObject{}, err
	}
	replacement := StoredObject{
		Key: input.Key, SHA256: input.SHA256, SizeBytes: input.SizeBytes,
		ContentType: input.ContentType, Status: "replaced",
	}
	store.objects[input.Key] = replacement
	store.bodies[input.Key] = append([]byte(nil), body...)
	return replacement, nil
}

type mismatchOnceVerifier struct {
	ipaChecks int
}

type mutatingReadSeeker struct {
	data    []byte
	offset  int64
	mutated bool
}

func (reader *mutatingReadSeeker) Read(buffer []byte) (int, error) {
	if reader.offset >= int64(len(reader.data)) {
		return 0, io.EOF
	}
	limit := len(buffer)
	if limit > 64<<10 {
		limit = 64 << 10
	}
	remaining := len(reader.data) - int(reader.offset)
	if limit > remaining {
		limit = remaining
	}
	copy(buffer[:limit], reader.data[reader.offset:reader.offset+int64(limit)])
	reader.offset += int64(limit)
	if !reader.mutated {
		reader.mutated = true
		for index := int(reader.offset); index < len(reader.data); index++ {
			reader.data[index] = 'b'
		}
	}
	return limit, nil
}

func (reader *mutatingReadSeeker) Seek(offset int64, whence int) (int64, error) {
	var next int64
	switch whence {
	case io.SeekStart:
		next = offset
	case io.SeekCurrent:
		next = reader.offset + offset
	case io.SeekEnd:
		next = int64(len(reader.data)) + offset
	default:
		return 0, errors.New("invalid seek")
	}
	if next < 0 {
		return 0, errors.New("negative seek")
	}
	reader.offset = next
	return next, nil
}

func (verifier *mismatchOnceVerifier) Verify(_ context.Context, request VerifyRequest) error {
	if request.Kind == VerifyIPA {
		verifier.ipaChecks++
		if verifier.ipaChecks == 1 {
			return ErrObjectVerificationMismatch
		}
	}
	return nil
}

func (store *delayedPresignStore) PresignGet(_ context.Context, key string, ttl time.Duration) (string, error) {
	store.ttls = append(store.ttls, ttl)
	signedAt := store.now
	store.now = store.now.Add(time.Minute)
	if strings.HasSuffix(key, "manifest.plist") {
		store.now = store.now.Add(10 * time.Minute)
	}
	return privateSignatureFixture("/"+key, signedAt, ttl), nil
}

func (f *fakeObjectStore) Ensure(_ context.Context, input PutObject) (StoredObject, error) {
	if f.beforeEnsure != nil {
		beforeEnsure := f.beforeEnsure
		f.beforeEnsure = nil
		beforeEnsure()
	}
	if f.objects == nil {
		f.objects = map[string]StoredObject{}
	}
	if f.bodies == nil {
		f.bodies = map[string][]byte{}
	}
	if existing, ok := f.objects[input.Key]; ok {
		if existing.SHA256 != input.SHA256 || existing.SizeBytes != input.SizeBytes || existing.ContentType != input.ContentType {
			return StoredObject{}, errors.New("immutable object conflict")
		}
		existing.Status = "reused"
		f.calls = append(f.calls, "reuse:"+input.Key)
		return existing, nil
	}
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return StoredObject{}, err
	}
	object := StoredObject{Key: input.Key, SHA256: input.SHA256, SizeBytes: input.SizeBytes, ContentType: input.ContentType, Status: "uploaded"}
	f.objects[input.Key] = object
	f.calls = append(f.calls, "put:"+input.Key)
	// Tests inspect generated bodies without expanding the public receipt type.
	f.objects[input.Key] = object
	f.bodies[input.Key] = body
	return object, nil
}

func (f *fakeObjectStore) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return (&url.URL{Scheme: "https", Host: "download.example.com", Path: "/" + key, RawQuery: "X-Amz-Signature=secret"}).String(), nil
}

type recordingVerifier struct{ urls []string }

func (v *recordingVerifier) Verify(_ context.Context, request VerifyRequest) error {
	v.urls = append(v.urls, request.URL)
	return nil
}

type advancingVerifier struct {
	now     *time.Time
	advance time.Duration
}

func (verifier *advancingVerifier) Verify(context.Context, VerifyRequest) error {
	*verifier.now = verifier.now.Add(verifier.advance)
	return nil
}

type fixedSignatureStore struct {
	signedAt []time.Time
	calls    int
}

func (store *fixedSignatureStore) Ensure(context.Context, PutObject) (StoredObject, error) {
	return StoredObject{}, nil
}

func (store *fixedSignatureStore) PresignGet(_ context.Context, key string, ttl time.Duration) (string, error) {
	index := store.calls
	store.calls++
	if index >= len(store.signedAt) {
		index = len(store.signedAt) - 1
	}
	return privateSignatureFixture("/"+key, store.signedAt[index], ttl), nil
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func minimalDescriptor(ipa []byte) PreparedDescriptor {
	return PreparedDescriptor{
		SchemaVersion: "1", Platform: "IOS", DistributionMethod: "release-testing",
		App:      PreparedApp{BundleID: "com.example.app", Title: "Example", Version: "1", BuildNumber: "1"},
		Artifact: PreparedArtifact{RelativePath: "payload/app.ipa", SHA256: sha256Hex(ipa), SizeBytes: int64(len(ipa))},
		Signing: PreparedSigning{
			ProfileClass: "ad-hoc", ProfileUUID: "uuid", TeamID: "TEAM", ExpiresAt: "2035-01-01T00:00:00Z", DeviceCount: 1,
			DeviceSetSHA256: strings.Repeat("b", 64), EmbeddedProfileSHA256: strings.Repeat("c", 64),
			ProfileCertificateSHA256Fingerprints: []string{strings.Repeat("a", 64)},
			ProfileIntegrityVerification:         PreparedCodeSignatureVerification{Status: "verified"}, ProfileTrustVerification: PreparedCodeSignatureVerification{Status: "verified"},
			CodeSignatureVerification: PreparedCodeSignatureVerification{Status: "verified", Scope: "complete-main-app-code-resources-entitlements-and-profile-certificate-binding", SignerCertificateSHA256Fingerprints: []string{strings.Repeat("a", 64)}},
		},
	}
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

var _ = errors.Is
