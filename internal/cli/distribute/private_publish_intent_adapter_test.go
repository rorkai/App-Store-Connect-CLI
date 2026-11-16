package distribute

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	core "github.com/rudrankriyam/App-Store-Connect-CLI/internal/distribution"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func TestPrivatePublishIntentReceiptRejectsBearerAndEvidenceTamper(t *testing.T) {
	_, bundleFactory := privatePublishIntentTestBundle(t)
	bundle := bundleFactory()
	defer bundle.IPA.Close()
	intent := adapterTestIntent()
	stateDir := t.TempDir()
	binding := privatePublishIntentBinding{
		Endpoint: "https://objects.example.com", DownloadEndpoint: "https://downloads.example.com", Region: "auto", AddressingStyle: "path",
		Bucket: "bucket", Prefix: "app", RequestedURLTTL: time.Hour.String(), DownloadGrace: time.Minute.String(),
		ReceiptPath: filepath.Join(stateDir, "receipt.json"), IntentPath: filepath.Join(stateDir, "intent.json"),
	}
	state := privatePublishIntentState{SchemaVersion: privatePublishIntentStateSchemaVersion, Binding: binding, Intent: intent}
	state.IntentSHA256, _ = privatePublishIntentDigest(intent)
	base := adapterTestIntentReceipt(intent)
	base.Endpoint, base.DownloadEndpoint, base.Region, base.AddressingStyle = binding.Endpoint, binding.DownloadEndpoint, binding.Region, binding.AddressingStyle
	base.ReceiptPath, base.LinkPath = binding.ReceiptPath, binding.IntentPath

	mutations := map[string]func(*core.PublishReceipt){
		"install bearer": func(got *core.PublishReceipt) {
			got.InstallURL = "https://downloads.example.com/index?token=another-secret"
		},
		"direct bearer": func(got *core.PublishReceipt) { got.DirectInstallURL = intent.Links.DirectInstallURL },
		"url ttl":       func(got *core.PublishReceipt) { got.URLTTL = "2h0m0s" },
		"object status": func(got *core.PublishReceipt) { got.Artifact.Status = "planned" },
		"signing":       func(got *core.PublishReceipt) { got.Signing.TeamID = "DIFFERENT" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			candidate := base
			mutate(&candidate)
			if err := validatePrivatePublishIntentReceipt(candidate, state, bundle); err == nil {
				t.Fatal("expected exact receipt binding failure")
			}
		})
	}
}

func TestLoadPrivatePublishIntentStateRejectsDuplicateJSONKeys(t *testing.T) {
	stateDir := t.TempDir()
	receiptPath := filepath.Join(stateDir, "receipt.json")
	intentPath := filepath.Join(stateDir, "intent.json")
	paths, err := preflightArtifactPaths(receiptPath, intentPath)
	if err != nil {
		t.Fatal(err)
	}
	defer paths.close()
	data := []byte(`{"schemaVersion":"1","schemaVersion":"1","intentSha256":"x","binding":{},"intent":{}}`)
	if err := os.WriteFile(intentPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadPrivatePublishIntentState(paths); err == nil || !strings.Contains(err.Error(), "duplicate JSON field") {
		t.Fatalf("duplicate-key error = %v", err)
	}
}

func TestLoadPrivatePublishIntentStateMissingDoesNotCreateArtifactParent(t *testing.T) {
	stateDir := t.TempDir()
	receiptPath := filepath.Join(stateDir, "missing", "receipt.json")
	intentPath := filepath.Join(stateDir, "missing", "intent.json")
	artifacts, err := inspectArtifactPaths(receiptPath, intentPath)
	if err != nil {
		t.Fatal(err)
	}
	defer artifacts.close()
	if _, found, err := loadPrivatePublishIntentState(artifacts); err != nil || found {
		t.Fatalf("loadPrivatePublishIntentState() = found:%t err:%v, want missing state without error", found, err)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "missing")); !os.IsNotExist(err) {
		t.Fatalf("loadPrivatePublishIntentState() created missing artifact parent: %v", err)
	}
}

func TestPrivatePublishIntentArtifactsSupportDistinctTopLevelParents(t *testing.T) {
	receiptDir, err := os.MkdirTemp(t.TempDir(), "asc-private-intent-receipt-*")
	if err != nil {
		t.Fatal(err)
	}
	intentDir, err := os.MkdirTemp(t.TempDir(), "asc-private-intent-state-*")
	if err != nil {
		t.Fatal(err)
	}

	paths, err := preflightArtifactPaths(filepath.Join(receiptDir, "receipt.json"), filepath.Join(intentDir, "intent.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer paths.close()
	state := privatePublishIntentState{SchemaVersion: privatePublishIntentStateSchemaVersion}
	if err := publishPrivatePublishIntentState(paths, state); err != nil {
		t.Fatal(err)
	}
	receipt := core.PublishReceipt{SchemaVersion: "1"}
	if err := paths.publishReceipt(receipt); err != nil {
		t.Fatal(err)
	}
	loaded, found, err := loadPrivatePublishIntentState(paths)
	if err != nil || !found || loaded.SchemaVersion != state.SchemaVersion {
		t.Fatalf("loadPrivatePublishIntentState() = %#v, %t, %v", loaded, found, err)
	}
	loadedReceipt, err := readPrivatePublishIntentReceipt(paths)
	if err != nil || loadedReceipt.SchemaVersion != receipt.SchemaVersion {
		t.Fatalf("readPrivatePublishIntentReceipt() = %#v, %v", loadedReceipt, err)
	}
}

func TestPrivatePublishIntentRequestRejectsDiagnosticInjectionInBucket(t *testing.T) {
	for _, bucket := range []string{"bucket\x1b[31m", "bucket\u202eexe", "bucket\u200bname"} {
		t.Run(bucket, func(t *testing.T) {
			err := validatePrivatePublishIntentRequest(newDistributeTestDeps(), privatePublishIntentRequest{
				BundleDir: "/bundle", Endpoint: "https://objects.example.com", Region: "auto", Bucket: bucket, Prefix: "app", AddressingStyle: "path",
				URLTTL: time.Hour, DownloadGrace: time.Minute, VerifyTimeout: time.Second, ReceiptPath: "/state/receipt", IntentPath: "/state/intent",
				ExpectedBundle: validPrivatePublishIntentAuthorization(),
			})
			if err == nil {
				t.Fatalf("bucket %q passed validation", bucket)
			}
		})
	}
}

func TestPrivatePublishIntentRequestRejectsLifetimeOverflow(t *testing.T) {
	digest := strings.Repeat("a", 64)
	err := validatePrivatePublishIntentRequest(newDistributeTestDeps(), privatePublishIntentRequest{
		BundleDir: "/bundle", Endpoint: "https://objects.example.com", Region: "auto", Bucket: "bucket", Prefix: "app", AddressingStyle: "path",
		URLTTL: time.Duration(1 << 62), DownloadGrace: time.Duration(1 << 62), VerifyTimeout: time.Second,
		ReceiptPath: "/state/receipt", IntentPath: "/state/intent",
		ExpectedBundle: privatePublishBundleAuthorization{
			DescriptorSHA256: digest, DescriptorSize: 1, IPASHA256: digest, IPASize: 1,
			ProfileUUID: "profile", ProfileSHA256: digest, TeamID: "TEAM", DeviceSetSHA256: digest,
			DeviceCount: 1, CertificateSHA256: digest,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "lifetimes") {
		t.Fatalf("overflowing private publication lifetime error = %v", err)
	}
}

func TestPrivatePublishIntentRequestRejectsDiagnosticInjectionInPrefixAndEndpoint(t *testing.T) {
	for name, mutate := range map[string]func(*privatePublishIntentRequest){
		"prefix zero width": func(got *privatePublishIntentRequest) { got.Prefix = "app\u200bhidden" },
		"prefix bidi":       func(got *privatePublishIntentRequest) { got.Prefix = "app\u202ehidden" },
		"endpoint zero width": func(got *privatePublishIntentRequest) {
			got.Endpoint = "https://objects.example.com\u200b"
		},
		"endpoint bidi": func(got *privatePublishIntentRequest) { got.Endpoint = "https://objects.example.com\u202e" },
		"download endpoint control": func(got *privatePublishIntentRequest) {
			got.DownloadEndpoint = "https://downloads.example.com\x1b[31m"
		},
	} {
		t.Run(name, func(t *testing.T) {
			request := privatePublishIntentRequest{
				BundleDir: "/bundle", Endpoint: "https://objects.example.com", Region: "auto", Bucket: "bucket", Prefix: "app", AddressingStyle: "path",
				URLTTL: time.Hour, DownloadGrace: time.Minute, VerifyTimeout: time.Second, ReceiptPath: "/state/receipt", IntentPath: "/state/intent",
				ExpectedBundle: validPrivatePublishIntentAuthorization(),
			}
			mutate(&request)
			if err := validatePrivatePublishIntentRequest(newDistributeTestDeps(), request); err == nil {
				t.Fatal("diagnostic-injection input passed validation")
			}
		})
	}
}

func validPrivatePublishIntentAuthorization() privatePublishBundleAuthorization {
	digest := strings.Repeat("a", 64)
	return privatePublishBundleAuthorization{
		DescriptorSHA256: digest, DescriptorSize: 1, IPASHA256: digest, IPASize: 1,
		ProfileUUID: "profile", ProfileSHA256: digest, TeamID: "TEAM", DeviceSetSHA256: digest,
		DeviceCount: 1, CertificateSHA256: digest,
	}
}

func TestExecutePrivatePublishIntentPersistsBeforeExecutionAndRecoversWithoutPreparingAgain(t *testing.T) {
	originalLoad, originalStore := loadPreparedBundle, newObjectStore
	originalPrepare, originalExecute := preparePrivatePublicationIntent, executePrivatePublicationIntent
	originalVerifier, originalAfterPersist, originalAfterExecute := newPrivatePublicationVerifier, afterPrivatePublicationIntentPersisted, afterPrivatePublicationIntentExecuted
	originalAliasProbe := probeConfiguredArtifactAliasForPreflight
	t.Cleanup(func() {
		loadPreparedBundle, newObjectStore = originalLoad, originalStore
		preparePrivatePublicationIntent, executePrivatePublicationIntent = originalPrepare, originalExecute
		newPrivatePublicationVerifier, afterPrivatePublicationIntentPersisted = originalVerifier, originalAfterPersist
		afterPrivatePublicationIntentExecuted = originalAfterExecute
		probeConfiguredArtifactAliasForPreflight = originalAliasProbe
	})
	probeCalls := 0
	probeConfiguredArtifactAliasForPreflight = func(paths artifactPaths) error {
		probeCalls++
		return originalAliasProbe(paths)
	}

	bundleDir, bundle := privatePublishIntentTestBundle(t)
	loadPreparedBundle = func(context.Context, rootfs.Root, time.Time) (*core.PreparedBundle, error) { return bundle(), nil }
	storeCalls := 0
	newObjectStore = func(ctx context.Context, config core.S3StoreConfig) (core.ObjectStore, time.Time, error) {
		storeCalls++
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("object-store setup context has no deadline")
		}
		if config.HTTPClient != nil {
			t.Fatal("object-store setup with the default transport replaced the AWS SDK's client")
		}
		return noOpStore{}, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), nil
	}
	newPrivatePublicationVerifier = func(*shared.Deps, time.Duration) (core.Verifier, error) { return noOpVerifier{}, nil }
	intent := adapterTestIntent()
	prepareCalls, executeCalls := 0, 0
	preparePrivatePublicationIntent = func(ctx context.Context, _ core.PreparedDescriptor, options core.PublishOptions) (core.PrivatePublishIntent, error) {
		prepareCalls++
		if _, ok := ctx.Deadline(); ok {
			t.Fatal("intent preparation inherited an expiring phase context")
		}
		if _, err := options.Store.PresignGet(ctx, "key", time.Minute); err != nil {
			t.Fatal(err)
		}
		return intent, nil
	}
	executePrivatePublicationIntent = func(ctx context.Context, _ io.ReadSeeker, _ core.PreparedDescriptor, options core.PublishOptions, got core.PrivatePublishIntent) (core.PublishReceipt, core.SensitiveLinks, error) {
		executeCalls++
		if _, ok := ctx.Deadline(); ok {
			t.Fatal("intent execution inherited an expiring phase context")
		}
		if _, err := options.Store.Ensure(ctx, core.PutObject{}); err != nil {
			t.Fatal(err)
		}
		if got.LinkID != intent.LinkID || got.Links.InstallURL != intent.Links.InstallURL {
			t.Fatalf("execution got different intent: %+v", got)
		}
		return adapterTestIntentReceipt(got), got.Links, nil
	}
	afterPrivatePublicationIntentPersisted = func() error { return errors.New("injected crash after intent") }

	stateDir := t.TempDir()
	request := privatePublishIntentRequest{
		BundleDir: bundleDir, ExpectedBundle: privatePublishTestAuthorization(t, bundle), Endpoint: "https://objects.example.com", DownloadEndpoint: "https://downloads.example.com",
		Region: "auto", Bucket: "bucket", Prefix: "app", AddressingStyle: "path",
		URLTTL: time.Hour, DownloadGrace: time.Minute, VerifyTimeout: time.Second,
		ReceiptPath: filepath.Join(stateDir, "receipt.json"), IntentPath: filepath.Join(stateDir, "private-intent.json"), DiagnosticWriter: io.Discard,
	}
	if _, err := executePrivatePublishIntent(context.Background(), newDistributeTestDeps(), request); err == nil || !strings.Contains(err.Error(), "injected crash") {
		t.Fatalf("first execution error = %v", err)
	}
	if prepareCalls != 1 || executeCalls != 0 {
		t.Fatalf("prepare=%d execute=%d, intent was not persisted before execution", prepareCalls, executeCalls)
	}
	info, err := os.Stat(request.IntentPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("intent mode = %o", info.Mode().Perm())
	}
	if _, err := os.Stat(request.ReceiptPath); !os.IsNotExist(err) {
		t.Fatalf("receipt exists before completed publication: %v", err)
	}
	persisted, err := os.ReadFile(request.IntentPath)
	if err != nil || !strings.Contains(string(persisted), "saved-secret-canary") {
		t.Fatalf("persisted intent missing sensitive URL: err=%v body=%s", err, persisted)
	}

	afterPrivatePublicationIntentPersisted = func() error { return nil }
	afterPrivatePublicationIntentExecuted = func() error { return errors.New("injected crash after remote execution") }
	if _, err := executePrivatePublishIntent(context.Background(), newDistributeTestDeps(), request); err == nil || !strings.Contains(err.Error(), "after remote execution") {
		t.Fatalf("post-execution crash error = %v", err)
	}
	if prepareCalls != 1 || executeCalls != 1 {
		t.Fatalf("post-execution crash prepare=%d execute=%d", prepareCalls, executeCalls)
	}
	if _, err := os.Stat(request.ReceiptPath); !os.IsNotExist(err) {
		t.Fatalf("receipt exists after injected post-execution crash: %v", err)
	}
	afterPrivatePublicationIntentExecuted = func() error { return nil }
	result, err := executePrivatePublishIntent(context.Background(), newDistributeTestDeps(), request)
	if err != nil {
		t.Fatalf("recovery error = %v", err)
	}
	if !result.Recovered || prepareCalls != 1 || executeCalls != 2 || storeCalls != 3 || probeCalls != 1 {
		t.Fatalf("result=%+v prepare=%d execute=%d stores=%d probes=%d", result, prepareCalls, executeCalls, storeCalls, probeCalls)
	}
	encoded, _ := encodeJSON(result)
	if strings.Contains(string(encoded), "saved-secret-canary") {
		t.Fatalf("result leaked bearer URL: %s", encoded)
	}
	receipt, err := os.ReadFile(request.ReceiptPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(receipt), "saved-secret-canary") {
		t.Fatalf("receipt leaked bearer URL: %s", receipt)
	}
}

func TestExecutePrivatePublishIntentRejectsChangedDestinationBeforeRemoteExecution(t *testing.T) {
	originalLoad, originalStore := loadPreparedBundle, newObjectStore
	originalPrepare, originalExecute := preparePrivatePublicationIntent, executePrivatePublicationIntent
	originalVerifier, originalAfterPersist := newPrivatePublicationVerifier, afterPrivatePublicationIntentPersisted
	t.Cleanup(func() {
		loadPreparedBundle, newObjectStore = originalLoad, originalStore
		preparePrivatePublicationIntent, executePrivatePublicationIntent = originalPrepare, originalExecute
		newPrivatePublicationVerifier, afterPrivatePublicationIntentPersisted = originalVerifier, originalAfterPersist
	})
	bundleDir, bundle := privatePublishIntentTestBundle(t)
	loadPreparedBundle = func(context.Context, rootfs.Root, time.Time) (*core.PreparedBundle, error) { return bundle(), nil }
	newObjectStore = func(context.Context, core.S3StoreConfig) (core.ObjectStore, time.Time, error) {
		return noOpStore{}, time.Time{}, nil
	}
	newPrivatePublicationVerifier = func(*shared.Deps, time.Duration) (core.Verifier, error) { return noOpVerifier{}, nil }
	preparePrivatePublicationIntent = func(context.Context, core.PreparedDescriptor, core.PublishOptions) (core.PrivatePublishIntent, error) {
		return adapterTestIntent(), nil
	}
	executeCalls := 0
	executePrivatePublicationIntent = func(context.Context, io.ReadSeeker, core.PreparedDescriptor, core.PublishOptions, core.PrivatePublishIntent) (core.PublishReceipt, core.SensitiveLinks, error) {
		executeCalls++
		return core.PublishReceipt{}, core.SensitiveLinks{}, nil
	}
	afterPrivatePublicationIntentPersisted = func() error { return errors.New("stop") }

	stateDir := t.TempDir()
	request := privatePublishIntentRequest{
		BundleDir: bundleDir, ExpectedBundle: privatePublishTestAuthorization(t, bundle), Endpoint: "https://objects.example.com", DownloadEndpoint: "https://downloads.example.com", Region: "auto", Bucket: "bucket", Prefix: "app", AddressingStyle: "path",
		URLTTL: time.Hour, DownloadGrace: time.Minute, VerifyTimeout: time.Second,
		ReceiptPath: filepath.Join(stateDir, "receipt.json"), IntentPath: filepath.Join(stateDir, "intent.json"), DiagnosticWriter: io.Discard,
	}
	_, _ = executePrivatePublishIntent(context.Background(), newDistributeTestDeps(), request)
	afterPrivatePublicationIntentPersisted = func() error { return nil }
	request.Bucket = "different-bucket"
	if _, err := executePrivatePublishIntent(context.Background(), newDistributeTestDeps(), request); err == nil || !errors.Is(err, errPrivatePublishIntentConflict) {
		t.Fatalf("changed-destination error = %v", err)
	}
	if executeCalls != 0 {
		t.Fatalf("changed destination reached remote execution %d times", executeCalls)
	}
}

func TestExecutePrivatePublishIntentRejectsCredentialClampedLifetimeBeforePersistence(t *testing.T) {
	originalLoad, originalStore := loadPreparedBundle, newObjectStore
	originalPrepare, originalExecute := preparePrivatePublicationIntent, executePrivatePublicationIntent
	originalVerifier := newPrivatePublicationVerifier
	t.Cleanup(func() {
		loadPreparedBundle, newObjectStore = originalLoad, originalStore
		preparePrivatePublicationIntent, executePrivatePublicationIntent = originalPrepare, originalExecute
		newPrivatePublicationVerifier = originalVerifier
	})
	bundleDir, bundle := privatePublishIntentTestBundle(t)
	loadPreparedBundle = func(context.Context, rootfs.Root, time.Time) (*core.PreparedBundle, error) { return bundle(), nil }
	newObjectStore = func(context.Context, core.S3StoreConfig) (core.ObjectStore, time.Time, error) {
		return noOpStore{}, time.Now().Add(2 * time.Hour), nil
	}
	newPrivatePublicationVerifier = func(*shared.Deps, time.Duration) (core.Verifier, error) { return noOpVerifier{}, nil }
	preparePrivatePublicationIntent = func(context.Context, core.PreparedDescriptor, core.PublishOptions) (core.PrivatePublishIntent, error) {
		intent := adapterTestIntent()
		intent.URLTTL = "59m0s"
		return intent, nil
	}
	executeCalls := 0
	executePrivatePublicationIntent = func(context.Context, io.ReadSeeker, core.PreparedDescriptor, core.PublishOptions, core.PrivatePublishIntent) (core.PublishReceipt, core.SensitiveLinks, error) {
		executeCalls++
		return core.PublishReceipt{}, core.SensitiveLinks{}, nil
	}
	stateDir := t.TempDir()
	request := privatePublishIntentRequest{
		BundleDir: bundleDir, ExpectedBundle: privatePublishTestAuthorization(t, bundle), Endpoint: "https://objects.example.com", DownloadEndpoint: "https://downloads.example.com", Region: "auto", Bucket: "bucket", Prefix: "app", AddressingStyle: "path",
		URLTTL: time.Hour, DownloadGrace: time.Minute, VerifyTimeout: time.Second,
		ReceiptPath: filepath.Join(stateDir, "receipt.json"), IntentPath: filepath.Join(stateDir, "intent.json"), DiagnosticWriter: io.Discard,
	}
	if _, err := executePrivatePublishIntent(context.Background(), newDistributeTestDeps(), request); err == nil || !errors.Is(err, errPrivatePublishIntentConflict) {
		t.Fatalf("clamped-lifetime error = %v", err)
	}
	if executeCalls != 0 {
		t.Fatalf("clamped lifetime reached remote execution %d times", executeCalls)
	}
	if _, err := os.Stat(request.IntentPath); !os.IsNotExist(err) {
		t.Fatalf("clamped lifetime persisted intent: %v", err)
	}
}

func TestExecutePrivatePublishIntentRejectsSwappedPreparedBundleBeforePresignOrPut(t *testing.T) {
	bundleDir := t.TempDir()
	writePrivatePublishPreparedBundle(t, bundleDir, []byte("authorized ipa"), "authorized-profile")
	loaded, err := core.LoadPreparedBundle(bundleDir, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	expected := privatePublishAuthorizationFromPrepared(loaded)
	if err := loaded.IPA.Close(); err != nil {
		t.Fatal(err)
	}

	// Simulate a same-UID child replacing both exact children after outer
	// orchestration validation but before the publication adapter opens them.
	writePrivatePublishPreparedBundle(t, bundleDir, []byte("unauthorized ipa"), "unauthorized-profile")
	originalStore := newObjectStore
	storeCalls := 0
	newObjectStore = func(context.Context, core.S3StoreConfig) (core.ObjectStore, time.Time, error) {
		storeCalls++
		return noOpStore{}, time.Time{}, nil
	}
	t.Cleanup(func() { newObjectStore = originalStore })

	stateDir := t.TempDir()
	request := privatePublishIntentRequest{
		BundleDir: bundleDir, ExpectedBundle: expected,
		Endpoint: "https://objects.example.com", DownloadEndpoint: "https://downloads.example.com",
		Region: "auto", Bucket: "bucket", Prefix: "app", AddressingStyle: "path",
		URLTTL: time.Hour, DownloadGrace: time.Minute, VerifyTimeout: time.Second,
		ReceiptPath: filepath.Join(stateDir, "receipt.json"), IntentPath: filepath.Join(stateDir, "intent.json"), DiagnosticWriter: io.Discard,
	}
	if _, err := executePrivatePublishIntent(context.Background(), newDistributeTestDeps(), request); err == nil || !errors.Is(err, errPrivatePublishIntentConflict) {
		t.Fatalf("swapped bundle error = %v", err)
	}
	if storeCalls != 0 {
		t.Fatalf("swapped bundle reached presign/store construction %d times", storeCalls)
	}
	if _, err := os.Stat(request.IntentPath); !os.IsNotExist(err) {
		t.Fatalf("swapped bundle persisted intent: %v", err)
	}
}

func adapterTestIntent() core.PrivatePublishIntent {
	now := time.Date(2029, 1, 1, 0, 0, 0, 0, time.UTC)
	expires := now.Add(time.Hour)
	return core.PrivatePublishIntent{
		SchemaVersion: "1", CreatedAt: now, PageExpiresAt: expires, DownloadExpiresAt: expires.Add(time.Minute),
		Bucket: "bucket", Prefix: "app", URLTTL: "1h0m0s", DownloadGrace: "1m0s", LinkID: "stable-link",
		Artifact: core.StoredObject{Key: "app/objects/sha256/sha.ipa", SHA256: strings.Repeat("a", 64), SizeBytes: 3, ContentType: core.ContentTypeIPA},
		Manifest: core.PrivatePublishDocument{StoredObject: core.StoredObject{Key: "app/links/stable-link/manifest.plist", SHA256: "manifest", SizeBytes: 8, ContentType: core.ContentTypeManifest}, Body: []byte("manifest")},
		Page:     core.PrivatePublishDocument{StoredObject: core.StoredObject{Key: "app/links/stable-link/index.html", SHA256: "page", SizeBytes: 4, ContentType: core.ContentTypeHTML}, Body: []byte("page")},
		Links:    core.SensitiveLinks{SchemaVersion: "1", InstallURL: "https://downloads.example.com/bucket/app/links/stable-link/index.html?X-Amz-Signature=saved-secret-canary", DirectInstallURL: "itms-services://?action=download-manifest&url=saved", ArtifactURL: "https://downloads.example.com/bucket/app/objects/sha256/sha.ipa?X-Amz-Signature=saved", ManifestURL: "https://downloads.example.com/bucket/app/links/stable-link/manifest.plist?X-Amz-Signature=saved", ExpiresAt: &expires},
		App:      core.PreparedApp{BundleID: "com.example", Version: "1", BuildNumber: "2"},
		Signing: core.ReceiptSigning{
			ProfileUUID: "profile-uuid", EmbeddedProfileSHA256: strings.Repeat("c", 64), TeamID: "TEAM",
			DeviceCount: 1, DeviceSetSHA256: strings.Repeat("d", 64),
			ProfileCertificateFingerprints: []string{strings.Repeat("e", 64)},
			CodeSignatureVerification:      core.PreparedCodeSignatureVerification{SignerCertificateSHA256Fingerprints: []string{strings.Repeat("e", 64)}},
		},
	}
}

func privatePublishTestAuthorization(t *testing.T, bundle func() *core.PreparedBundle) privatePublishBundleAuthorization {
	t.Helper()
	prepared := bundle()
	defer prepared.IPA.Close()
	return privatePublishAuthorizationFromPrepared(prepared)
}

func privatePublishIntentTestBundle(t *testing.T) (string, func() *core.PreparedBundle) {
	t.Helper()
	dir := t.TempDir()
	ipaPath := filepath.Join(dir, "app.ipa")
	if err := os.WriteFile(ipaPath, []byte("ipa"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir, func() *core.PreparedBundle {
		file, err := os.Open(ipaPath)
		if err != nil {
			t.Fatal(err)
		}
		return &core.PreparedBundle{
			IPA: file, IPASHA256: strings.Repeat("a", 64), IPASize: 3,
			DescriptorSHA256: strings.Repeat("b", 64), DescriptorSize: 123,
			Descriptor: core.PreparedDescriptor{
				App: core.PreparedApp{BundleID: "com.example", Version: "1", BuildNumber: "2"},
				Signing: core.PreparedSigning{
					ProfileUUID: "profile-uuid", EmbeddedProfileSHA256: strings.Repeat("c", 64), TeamID: "TEAM",
					DeviceCount: 1, DeviceSetSHA256: strings.Repeat("d", 64),
					ProfileCertificateSHA256Fingerprints: []string{strings.Repeat("e", 64)},
					CodeSignatureVerification:            core.PreparedCodeSignatureVerification{SignerCertificateSHA256Fingerprints: []string{strings.Repeat("e", 64)}},
				},
			},
		}
	}
}

func privatePublishAuthorizationFromPrepared(prepared *core.PreparedBundle) privatePublishBundleAuthorization {
	return privatePublishBundleAuthorization{
		DescriptorSHA256: prepared.DescriptorSHA256, DescriptorSize: prepared.DescriptorSize,
		IPASHA256: prepared.IPASHA256, IPASize: prepared.IPASize,
		ProfileUUID: prepared.Descriptor.Signing.ProfileUUID, ProfileSHA256: prepared.Descriptor.Signing.EmbeddedProfileSHA256,
		TeamID: prepared.Descriptor.Signing.TeamID, DeviceSetSHA256: prepared.Descriptor.Signing.DeviceSetSHA256,
		DeviceCount: prepared.Descriptor.Signing.DeviceCount, CertificateSHA256: prepared.Descriptor.Signing.ProfileCertificateSHA256Fingerprints[0],
	}
}

func writePrivatePublishPreparedBundle(t *testing.T, bundleDir string, ipa []byte, profileUUID string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(bundleDir, "payload"), 0o700); err != nil {
		t.Fatal(err)
	}
	ipaDigest := sha256.Sum256(ipa)
	certificate := strings.Repeat("e", 64)
	descriptor := core.PreparedDescriptor{
		SchemaVersion: "1", Platform: "IOS", DistributionMethod: "release-testing",
		App:      core.PreparedApp{BundleID: "com.example", Title: "Example", Version: "1", BuildNumber: "2"},
		Artifact: core.PreparedArtifact{RelativePath: "payload/app.ipa", SHA256: hex.EncodeToString(ipaDigest[:]), SizeBytes: int64(len(ipa))},
		Signing: core.PreparedSigning{
			ProfileClass: "ad-hoc", ProfileUUID: profileUUID, EmbeddedProfileSHA256: strings.Repeat("c", 64), TeamID: "TEAM",
			ExpiresAt: "2035-01-01T00:00:00Z", DeviceCount: 1, DeviceSetSHA256: strings.Repeat("d", 64),
			ProfileCertificateSHA256Fingerprints: []string{certificate},
			ProfileIntegrityVerification:         core.PreparedCodeSignatureVerification{Status: "verified"},
			ProfileTrustVerification:             core.PreparedCodeSignatureVerification{Status: "verified"},
			CodeSignatureVerification: core.PreparedCodeSignatureVerification{
				Status: "verified", Scope: "complete-main-app-code-resources-entitlements-and-profile-certificate-binding",
				SignerCertificateSHA256Fingerprints: []string{certificate},
			},
		},
	}
	descriptorData, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "bundle.json"), descriptorData, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "payload", "app.ipa"), ipa, 0o600); err != nil {
		t.Fatal(err)
	}
}

func adapterTestIntentReceipt(intent core.PrivatePublishIntent) core.PublishReceipt {
	expires := intent.PageExpiresAt
	artifact, manifest, page := intent.Artifact, intent.Manifest.StoredObject, intent.Page.StoredObject
	artifact.Status, manifest.Status, page.Status = "reused", "reused", "reused"
	return core.PublishReceipt{
		SchemaVersion: "1", Access: core.AccessPrivate, Bucket: intent.Bucket, Prefix: intent.Prefix,
		URLTTL: intent.URLTTL, DownloadGrace: intent.DownloadGrace,
		Artifact: artifact, Manifest: manifest, Page: page,
		InstallURL:       core.RedactedInstallURL(intent.Links),
		DirectInstallURL: core.RedactedDirectInstallURL(intent.Links), ExpiresAt: &expires, Verified: true, App: intent.App, Signing: intent.Signing,
	}
}

type noOpVerifier struct{}

func (noOpVerifier) Verify(context.Context, core.VerifyRequest) error { return nil }
