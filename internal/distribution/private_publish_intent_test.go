package distribution

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPreparePrivatePublishIntentHasNoRemoteWritesAndExecutionUsesSavedIdentity(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	store := &intentTestStore{now: func() time.Time { return now }}
	descriptor := minimalDescriptor([]byte("ipa"))
	descriptor.Signing.ExpiresAt = now.Add(48 * time.Hour).Format(time.RFC3339)
	options := PublishOptions{
		Store: store, Verifier: &recordingVerifier{}, Bucket: "bucket", Prefix: "team/app", Access: AccessPrivate,
		URLTTL: time.Hour, DownloadGrace: 10 * time.Minute, CredentialLimit: now.Add(24 * time.Hour),
		Now: func() time.Time { return now }, RandomID: func() (string, error) { return "stable-link", nil },
	}
	intent, err := PreparePrivatePublishIntent(context.Background(), descriptor, options)
	if err != nil {
		t.Fatalf("PreparePrivatePublishIntent() error = %v", err)
	}
	if len(store.ensureKeys) != 0 {
		t.Fatalf("preparation wrote objects: %v", store.ensureKeys)
	}
	wantPresigns := []string{intent.Artifact.Key, intent.Manifest.Key, intent.Page.Key}
	if !reflect.DeepEqual(store.presignKeys, wantPresigns) {
		t.Fatalf("presigns = %v, want %v", store.presignKeys, wantPresigns)
	}
	wantManifest, err := makeManifest(descriptor.App, intent.Links.ArtifactURL)
	if err != nil {
		t.Fatal(err)
	}
	wantPage, err := makeInstallPage(descriptor.App, intent.Links.DirectInstallURL)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(intent.Manifest.Body, wantManifest) || !bytes.Equal(intent.Page.Body, wantPage) {
		t.Fatal("intent did not persist the exact generated documents")
	}

	store.presignKeys = nil
	receipt, links, err := ExecutePrivatePublishIntent(context.Background(), bytes.NewReader([]byte("ipa")), descriptor, options, intent)
	if err != nil {
		t.Fatalf("ExecutePrivatePublishIntent() error = %v", err)
	}
	if len(store.presignKeys) != 0 {
		t.Fatalf("execution minted replacement URLs: %v", store.presignKeys)
	}
	wantEnsures := []string{intent.Artifact.Key, intent.Manifest.Key, intent.Page.Key}
	if !reflect.DeepEqual(store.ensureKeys, wantEnsures) {
		t.Fatalf("ensures = %v, want %v", store.ensureKeys, wantEnsures)
	}
	if links != intent.Links || receipt.InstallURL != redactBearerURL(intent.Links.InstallURL) || !receipt.Verified {
		t.Fatalf("receipt=%+v links=%+v intent=%+v", receipt, links, intent)
	}
}

func TestPreparePrivatePublishIntentPresignsToFixedDeadlines(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	store := &delayedPresignStore{now: now}
	descriptor := minimalDescriptor([]byte("ipa"))
	descriptor.Signing.ExpiresAt = now.Add(48 * time.Hour).Format(time.RFC3339)
	_, err := PreparePrivatePublishIntent(context.Background(), descriptor, PublishOptions{
		Store: store, Verifier: &recordingVerifier{}, Bucket: "bucket", Prefix: "app", Access: AccessPrivate,
		URLTTL: time.Hour, DownloadGrace: 10 * time.Minute, Now: func() time.Time { return store.now },
		RandomID: func() (string, error) { return "stable-link", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []time.Duration{70 * time.Minute, 69 * time.Minute, 48 * time.Minute}
	if !reflect.DeepEqual(store.ttls, want) {
		t.Fatalf("presigned TTLs = %v, want fixed-deadline TTLs %v", store.ttls, want)
	}
}

func TestPreparePrivatePublishIntentTypesPermanentProfileExpiryBeforeStorage(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	descriptor := minimalDescriptor([]byte("ipa"))
	descriptor.Signing.ExpiresAt = now.Add(30 * time.Minute).Format(time.RFC3339)
	store := &intentTestStore{now: func() time.Time { return now }}
	_, err := PreparePrivatePublishIntent(context.Background(), descriptor, PublishOptions{
		Store: store, Verifier: &recordingVerifier{}, Bucket: "bucket", Prefix: "app", Access: AccessPrivate,
		URLTTL: time.Hour, DownloadGrace: time.Minute, Now: func() time.Time { return now },
	})
	if !errors.Is(err, ErrPrivatePublishProfileExpired) {
		t.Fatalf("PreparePrivatePublishIntent() error = %v, want ErrPrivatePublishProfileExpired", err)
	}
	if len(store.presignKeys) != 0 || len(store.ensureKeys) != 0 {
		t.Fatalf("profile expiry reached storage: presigns=%v ensures=%v", store.presignKeys, store.ensureKeys)
	}
}

func TestExecutePrivatePublishIntentRecoversEveryAmbiguousBoundaryWithoutNewIdentity(t *testing.T) {
	for failAfter := 1; failAfter <= 3; failAfter++ {
		t.Run(intentStageName(failAfter), func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Second)
			store := &intentTestStore{now: func() time.Time { return now }}
			descriptor := minimalDescriptor([]byte("ipa"))
			descriptor.Signing.ExpiresAt = now.Add(48 * time.Hour).Format(time.RFC3339)
			options := PublishOptions{
				Store: store, Verifier: &recordingVerifier{}, Bucket: "bucket", Prefix: "team/app", Access: AccessPrivate,
				URLTTL: time.Hour, DownloadGrace: time.Minute, CredentialLimit: now.Add(24 * time.Hour),
				Now: func() time.Time { return now }, RandomID: func() (string, error) { return "stable-link", nil },
			}
			intent, err := PreparePrivatePublishIntent(context.Background(), descriptor, options)
			if err != nil {
				t.Fatal(err)
			}
			savedPresigns := append([]string(nil), store.presignKeys...)
			store.failAfterEnsure = failAfter
			if _, _, err := ExecutePrivatePublishIntent(context.Background(), bytes.NewReader([]byte("ipa")), descriptor, options, intent); err == nil || !strings.Contains(err.Error(), "ambiguous") {
				t.Fatalf("first execution error = %v", err)
			}
			store.failAfterEnsure = 0
			if _, _, err := ExecutePrivatePublishIntent(context.Background(), bytes.NewReader([]byte("ipa")), descriptor, options, intent); err != nil {
				t.Fatalf("recovery error = %v", err)
			}
			if !reflect.DeepEqual(store.presignKeys, savedPresigns) {
				t.Fatalf("recovery changed presigns: before=%v after=%v", savedPresigns, store.presignKeys)
			}
			if len(store.objectBodies) != 3 {
				t.Fatalf("unique destinations = %d, want 3", len(store.objectBodies))
			}
		})
	}
}

func TestExecutePrivatePublishIntentTypesImmutableObjectConflict(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	store := &intentTestStore{now: func() time.Time { return now }}
	descriptor := minimalDescriptor([]byte("ipa"))
	descriptor.Signing.ExpiresAt = now.Add(48 * time.Hour).Format(time.RFC3339)
	options := PublishOptions{
		Store: store, Verifier: &recordingVerifier{}, Bucket: "bucket", Prefix: "team/app", Access: AccessPrivate,
		URLTTL: time.Hour, DownloadGrace: time.Minute, Now: func() time.Time { return now },
		RandomID: func() (string, error) { return "stable-link", nil },
	}
	intent, err := PreparePrivatePublishIntent(context.Background(), descriptor, options)
	if err != nil {
		t.Fatal(err)
	}
	store.objectBodies = map[string][]byte{intent.Artifact.Key: []byte("different immutable object")}
	if _, _, err := ExecutePrivatePublishIntent(context.Background(), bytes.NewReader([]byte("ipa")), descriptor, options, intent); err == nil || !errors.Is(err, ErrPrivatePublishConflict) {
		t.Fatalf("immutable object error = %v, want ErrPrivatePublishConflict", err)
	}
}

func TestExecutePrivatePublishIntentTreatsCorruptReusedBodyAsPermanentConflict(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	store := &intentTestStore{now: func() time.Time { return now }}
	descriptor := minimalDescriptor([]byte("ipa"))
	descriptor.Signing.ExpiresAt = now.Add(48 * time.Hour).Format(time.RFC3339)
	options := PublishOptions{
		Store: store, Bucket: "bucket", Prefix: "app", Access: AccessPrivate,
		URLTTL: time.Hour, DownloadGrace: time.Minute, Now: func() time.Time { return now },
		RandomID: func() (string, error) { return "stable-link", nil }, Verifier: &recordingVerifier{},
	}
	intent, err := PreparePrivatePublishIntent(context.Background(), descriptor, options)
	if err != nil {
		t.Fatal(err)
	}
	store.objectBodies = map[string][]byte{intent.Artifact.Key: []byte("ipa")}
	getCalls := 0
	client := &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		getCalls++
		return &http.Response{
			StatusCode: http.StatusOK, ContentLength: intent.Artifact.SizeBytes,
			Header: http.Header{"Content-Type": []string{intent.Artifact.ContentType}},
			Body:   io.NopCloser(bytes.NewReader([]byte("bad"))), Request: request,
		}, nil
	})}
	options.Verifier = NewHTTPVerifier(client, time.Second)
	_, _, err = ExecutePrivatePublishIntent(context.Background(), bytes.NewReader([]byte("ipa")), descriptor, options, intent)
	if !errors.Is(err, ErrPrivatePublishConflict) || !errors.Is(err, ErrVerificationContentConflict) {
		t.Fatalf("corrupt reused body error = %v, want permanent private conflict", err)
	}
	if getCalls != 1 || !reflect.DeepEqual(store.ensureKeys, []string{intent.Artifact.Key}) {
		t.Fatalf("corrupt reuse calls: GET=%d ensures=%v", getCalls, store.ensureKeys)
	}
	if _, _, secondErr := ExecutePrivatePublishIntent(context.Background(), bytes.NewReader([]byte("ipa")), descriptor, options, intent); !errors.Is(secondErr, ErrPrivatePublishConflict) {
		t.Fatalf("second direct execution error = %v", secondErr)
	}
	if getCalls != 2 || len(store.ensureKeys) != 2 {
		t.Fatalf("direct core retry did not remain exact: GET=%d ensures=%v", getCalls, store.ensureKeys)
	}
}

func TestExecutePrivatePublishIntentRejectsTamperAndExpiryBeforeWrite(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	descriptor := minimalDescriptor([]byte("ipa"))
	descriptor.Signing.ExpiresAt = now.Add(48 * time.Hour).Format(time.RFC3339)
	baseStore := &intentTestStore{now: func() time.Time { return now }}
	options := PublishOptions{
		Store: baseStore, Verifier: &recordingVerifier{}, Bucket: "bucket", Prefix: "team/app", Access: AccessPrivate,
		URLTTL: time.Hour, DownloadGrace: time.Minute, CredentialLimit: now.Add(24 * time.Hour),
		Now: func() time.Time { return now }, RandomID: func() (string, error) { return "stable-link", nil },
	}
	intent, err := PreparePrivatePublishIntent(context.Background(), descriptor, options)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]func(*PrivatePublishIntent, *PreparedDescriptor, *PublishOptions){
		"manifest body": func(got *PrivatePublishIntent, _ *PreparedDescriptor, _ *PublishOptions) {
			got.Manifest.Body = append(got.Manifest.Body, 'x')
		},
		"artifact key": func(got *PrivatePublishIntent, _ *PreparedDescriptor, _ *PublishOptions) {
			got.Artifact.Key = "different/app.ipa"
		},
		"bundle": func(_ *PrivatePublishIntent, got *PreparedDescriptor, _ *PublishOptions) {
			got.App.BundleID = "com.other"
		},
		"destination": func(_ *PrivatePublishIntent, _ *PreparedDescriptor, got *PublishOptions) { got.Prefix = "other" },
		"link expiry": func(_ *PrivatePublishIntent, _ *PreparedDescriptor, got *PublishOptions) {
			got.Now = func() time.Time { return now.Add(2 * time.Hour) }
		},
		"credential expiry": func(got *PrivatePublishIntent, _ *PreparedDescriptor, _ *PublishOptions) {
			limit := now.Add(30 * time.Minute)
			got.CredentialLimit = &limit
		},
		"profile expiry": func(_ *PrivatePublishIntent, got *PreparedDescriptor, _ *PublishOptions) {
			got.Signing.ExpiresAt = now.Add(30 * time.Minute).Format(time.RFC3339)
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			candidate := intent.Clone()
			candidateDescriptor := descriptor
			candidateOptions := options
			store := &intentTestStore{now: func() time.Time { return now }}
			candidateOptions.Store = store
			mutate(&candidate, &candidateDescriptor, &candidateOptions)
			if _, _, err := ExecutePrivatePublishIntent(context.Background(), bytes.NewReader([]byte("ipa")), candidateDescriptor, candidateOptions, candidate); err == nil {
				t.Fatal("expected intent validation error")
			}
			if len(store.ensureKeys) != 0 {
				t.Fatalf("invalid intent wrote objects: %v", store.ensureKeys)
			}
		})
	}
}

func TestExecutePrivatePublishIntentRejectsSignatureBeyondSavedDeadlineBeforeWrite(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	descriptor := minimalDescriptor([]byte("ipa"))
	descriptor.Signing.ExpiresAt = now.Add(48 * time.Hour).Format(time.RFC3339)
	initialStore := &intentTestStore{now: func() time.Time { return now }}
	options := PublishOptions{
		Store: initialStore, Verifier: &recordingVerifier{}, Bucket: "bucket", Prefix: "team/app", Access: AccessPrivate,
		URLTTL: time.Hour, DownloadGrace: time.Minute, Now: func() time.Time { return now },
		RandomID: func() (string, error) { return "stable-link", nil },
	}
	intent, err := PreparePrivatePublishIntent(context.Background(), descriptor, options)
	if err != nil {
		t.Fatal(err)
	}

	intent.Links.ArtifactURL = privateSignatureFixture(
		"/bucket/"+intent.Artifact.Key,
		intent.CreatedAt,
		intent.DownloadExpiresAt.Sub(intent.CreatedAt)+time.Second,
	)
	manifestBody, err := makeManifest(descriptor.App, intent.Links.ArtifactURL)
	if err != nil {
		t.Fatal(err)
	}
	intent.Manifest = privatePublishDocument(intent.Manifest.Key, manifestBody, ContentTypeManifest)

	executionStore := &intentTestStore{now: func() time.Time { return now }}
	options.Store = executionStore
	_, _, err = ExecutePrivatePublishIntent(context.Background(), bytes.NewReader([]byte("ipa")), descriptor, options, intent)
	if err == nil || !strings.Contains(err.Error(), "signed expiry does not match") {
		t.Fatalf("ExecutePrivatePublishIntent() error = %v, want signed deadline rejection", err)
	}
	if len(executionStore.ensureKeys) != 0 {
		t.Fatalf("invalid signature reached object writes: %v", executionStore.ensureKeys)
	}
}

func intentStageName(index int) string {
	return []string{"", "ipa", "manifest", "page"}[index]
}

type intentTestStore struct {
	presignKeys     []string
	ensureKeys      []string
	objectBodies    map[string][]byte
	failAfterEnsure int
	failed          bool
	now             func() time.Time
}

func (store *intentTestStore) PresignGet(_ context.Context, key string, ttl time.Duration) (string, error) {
	store.presignKeys = append(store.presignKeys, key)
	now := time.Now
	if store.now != nil {
		now = store.now
	}
	return privateSignatureFixture("/bucket/"+key, now().UTC().Truncate(time.Second), ttl), nil
}

func (store *intentTestStore) Ensure(_ context.Context, input PutObject) (StoredObject, error) {
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return StoredObject{}, err
	}
	if store.objectBodies == nil {
		store.objectBodies = map[string][]byte{}
	}
	if existing, ok := store.objectBodies[input.Key]; ok && !bytes.Equal(existing, body) {
		return StoredObject{}, fmt.Errorf("%w: conflicting body", ErrImmutableObjectConflict)
	}
	store.objectBodies[input.Key] = append([]byte(nil), body...)
	store.ensureKeys = append(store.ensureKeys, input.Key)
	if !store.failed && store.failAfterEnsure == len(store.ensureKeys) {
		store.failed = true
		return StoredObject{}, errors.New("ambiguous put response")
	}
	return StoredObject{Key: input.Key, SHA256: input.SHA256, SizeBytes: input.SizeBytes, ContentType: input.ContentType, Status: "ensured"}, nil
}

func TestPreparePrivatePublishIntentChecksProfileExpiryOnItsClock(t *testing.T) {
	injected := time.Date(2019, 6, 1, 0, 0, 0, 0, time.UTC)
	descriptor := minimalDescriptor([]byte("ipa"))
	descriptor.Signing.ExpiresAt = "2020-01-01T00:00:00Z"
	_, err := PreparePrivatePublishIntent(context.Background(), descriptor, PublishOptions{
		Store: &intentTestStore{now: func() time.Time { return injected }}, Verifier: &recordingVerifier{}, Bucket: "bucket", Prefix: "team/app", Access: AccessPrivate,
		URLTTL: time.Hour, DownloadGrace: 10 * time.Minute, CredentialLimit: injected.Add(24 * time.Hour),
		Now: func() time.Time { return injected }, RandomID: func() (string, error) { return "stable-link", nil },
	})
	if err != nil {
		t.Fatalf("PreparePrivatePublishIntent() at %s with a profile that expires on %s: %v", injected.Format(time.RFC3339), descriptor.Signing.ExpiresAt, err)
	}
}
