package distribution

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

func TestS3StorePathStyleConditionalUploadsAndPrivateVerification(t *testing.T) {
	t.Setenv("ASC_S3_ACCESS_KEY_ID", "test-access")
	t.Setenv("ASC_S3_SECRET_ACCESS_KEY", "test-secret")
	t.Setenv("ASC_S3_SESSION_TOKEN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")

	type object struct {
		body, sha, contentType string
	}
	var mu sync.Mutex
	objects := map[string]object{}
	var operations []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(request.URL.Path, "/bucket/") {
			t.Errorf("path = %q", request.URL.Path)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.Header.Get("Authorization") == "" && request.Method != http.MethodGet {
			t.Error("signed API request omitted Authorization")
		}
		key := strings.TrimPrefix(request.URL.Path, "/bucket/")
		mu.Lock()
		defer mu.Unlock()
		switch request.Method {
		case http.MethodHead:
			operations = append(operations, "head:"+key)
			stored, ok := objects[key]
			if !ok {
				writer.WriteHeader(http.StatusNotFound)
				return
			}
			writer.Header().Set("Content-Length", strconv.Itoa(len(stored.body)))
			writer.Header().Set("Content-Type", stored.contentType)
			writer.Header().Set("x-amz-meta-asc-sha256", stored.sha)
			writer.WriteHeader(http.StatusOK)
		case http.MethodPut:
			operations = append(operations, "put:"+key)
			if request.Header.Get("If-None-Match") != "*" {
				t.Errorf("If-None-Match = %q", request.Header.Get("If-None-Match"))
			}
			metadataDigest, decodeErr := hex.DecodeString(request.Header.Get("x-amz-meta-asc-sha256"))
			if decodeErr != nil || request.Header.Get("x-amz-checksum-sha256") != base64.StdEncoding.EncodeToString(metadataDigest) {
				t.Errorf("checksum header = %q, metadata digest = %q", request.Header.Get("x-amz-checksum-sha256"), request.Header.Get("x-amz-meta-asc-sha256"))
			}
			body, _ := io.ReadAll(request.Body)
			objects[key] = object{body: string(body), sha: request.Header.Get("x-amz-meta-asc-sha256"), contentType: request.Header.Get("Content-Type")}
			writer.WriteHeader(http.StatusOK)
		case http.MethodGet:
			operations = append(operations, "get:"+key)
			stored, ok := objects[key]
			if !ok {
				writer.WriteHeader(http.StatusNotFound)
				return
			}
			writer.Header().Set("Content-Type", stored.contentType)
			writer.Header().Set("Content-Length", strconv.Itoa(len(stored.body)))
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(stored.body))
		default:
			writer.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	store, credentialExpiry, err := NewS3Store(context.Background(), S3StoreConfig{
		Endpoint: server.URL, Region: "auto", Bucket: "bucket", AddressingStyle: "path", HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewS3Store() error = %v", err)
	}
	if !credentialExpiry.IsZero() {
		t.Fatalf("static credential expiry = %v", credentialExpiry)
	}
	ipa := []byte("ipa")
	receipt, sensitive, err := Publish(context.Background(), bytes.NewReader(ipa), minimalDescriptor(ipa), PublishOptions{
		Store: store, Verifier: NewHTTPVerifier(server.Client(), 5*time.Second), Bucket: "bucket", Prefix: "channel", Access: AccessPrivate,
		URLTTL: time.Hour, DownloadGrace: time.Minute, RandomID: func() (string, error) { return "link", nil },
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if !receipt.Verified || strings.Contains(receipt.InstallURL, "test-secret") || !strings.Contains(sensitive.InstallURL, "X-Amz-") {
		t.Fatalf("unexpected receipt/links: %#v %#v", receipt, sensitive)
	}
	wantTail := []string{
		"head:channel/objects/sha256/" + sha256Hex(ipa) + ".ipa",
		"put:channel/objects/sha256/" + sha256Hex(ipa) + ".ipa",
		"get:channel/objects/sha256/" + sha256Hex(ipa) + ".ipa",
		"head:channel/links/link/manifest.plist", "put:channel/links/link/manifest.plist", "get:channel/links/link/manifest.plist",
		"head:channel/links/link/index.html", "put:channel/links/link/index.html", "get:channel/links/link/index.html",
	}
	if strings.Join(operations, "\n") != strings.Join(wantTail, "\n") {
		t.Fatalf("operations = %#v, want %#v", operations, wantTail)
	}
}

func TestPrivatePublishStopsAfterS3HeadMatchesButFetchedBodyIsCorrupt(t *testing.T) {
	t.Setenv("ASC_S3_ACCESS_KEY_ID", "test-access")
	t.Setenv("ASC_S3_SECRET_ACCESS_KEY", "test-secret")
	t.Setenv("ASC_S3_SESSION_TOKEN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_SESSION_TOKEN", "")
	synctest.Test(t, func(t *testing.T) {
		var operations []string
		server := httptest.NewTestServer(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			operations = append(operations, request.Method)
			switch request.Method {
			case http.MethodHead:
				writer.Header().Set("Content-Length", "3")
				writer.Header().Set("Content-Type", ContentTypeIPA)
				writer.Header().Set("x-amz-meta-asc-sha256", sha256Hex([]byte("ipa")))
				writer.WriteHeader(http.StatusOK)
			case http.MethodGet:
				writer.Header().Set("Content-Length", "3")
				writer.Header().Set("Content-Type", ContentTypeIPA)
				writer.WriteHeader(http.StatusOK)
				_, _ = writer.Write([]byte("bad"))
			case http.MethodPut:
				t.Error("exact HEAD evidence unexpectedly triggered a PUT")
				writer.WriteHeader(http.StatusInternalServerError)
			default:
				writer.WriteHeader(http.StatusMethodNotAllowed)
			}
		}))
		store, _, err := NewS3Store(context.Background(), S3StoreConfig{
			Endpoint: "https://s3.example.com", Region: "auto", Bucket: "bucket", AddressingStyle: "path", HTTPClient: server.Client(),
		})
		if err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC().Truncate(time.Second)
		descriptor := minimalDescriptor([]byte("ipa"))
		descriptor.Signing.ExpiresAt = now.Add(48 * time.Hour).Format(time.RFC3339)
		options := PublishOptions{
			Store: store, Verifier: NewHTTPVerifier(server.Client(), time.Second), Bucket: "bucket", Prefix: "app", Access: AccessPrivate,
			URLTTL: time.Hour, DownloadGrace: time.Minute, Now: func() time.Time { return now }, RandomID: func() (string, error) { return "stable", nil },
		}
		intent, err := PreparePrivatePublishIntent(context.Background(), descriptor, options)
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = ExecutePrivatePublishIntent(context.Background(), bytes.NewReader([]byte("ipa")), descriptor, options, intent)
		if !errors.Is(err, ErrPrivatePublishConflict) || !errors.Is(err, ErrVerificationContentConflict) {
			t.Fatalf("corrupt provider body error = %v, want permanent private conflict", err)
		}
		if want := []string{http.MethodHead, http.MethodGet}; !slices.Equal(operations, want) {
			t.Fatalf("provider operations = %v, want %v (no PUT or later objects)", operations, want)
		}
	})
}

func TestS3StoreDefaultClientHonorsAWSCABundle(t *testing.T) {
	t.Setenv("ASC_S3_ACCESS_KEY_ID", "test-access")
	t.Setenv("ASC_S3_SECRET_ACCESS_KEY", "test-secret")
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		switch request.Method {
		case http.MethodHead:
			writer.WriteHeader(http.StatusNotFound)
		case http.MethodPut:
			writer.WriteHeader(http.StatusOK)
		default:
			writer.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	if err := os.WriteFile(caPath, certificate, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_CA_BUNDLE", caPath)
	store, _, err := NewS3Store(context.Background(), S3StoreConfig{Endpoint: server.URL, Region: "auto", Bucket: "bucket", AddressingStyle: "path"})
	if err != nil {
		t.Fatalf("NewS3Store() error = %v", err)
	}
	input := PutObject{Key: "app.ipa", Body: strings.NewReader("ipa"), SHA256: sha256Hex([]byte("ipa")), SizeBytes: 3, ContentType: ContentTypeIPA}
	if _, err := store.Ensure(context.Background(), input); err != nil {
		t.Fatalf("Ensure() with AWS_CA_BUNDLE error = %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want HEAD and PUT only", requests)
	}
}

func TestS3StoreDefaultClientRefuses307And308WithoutForwardingCredentials(t *testing.T) {
	for _, redirectStatus := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(strconv.Itoa(redirectStatus), func(t *testing.T) {
			t.Setenv("ASC_S3_ACCESS_KEY_ID", "test-access")
			t.Setenv("ASC_S3_SECRET_ACCESS_KEY", "test-secret")
			targetRequests := 0
			target := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				targetRequests++
				if request.Header.Get("Authorization") != "" || request.Header.Get("X-Amz-Security-Token") != "" {
					t.Errorf("redirect target received credentials")
				}
				writer.WriteHeader(http.StatusOK)
			}))
			defer target.Close()
			origin := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Location", target.URL+"/stolen")
				writer.WriteHeader(redirectStatus)
			}))
			defer origin.Close()
			caPath := filepath.Join(t.TempDir(), "ca.pem")
			var certificate []byte
			for _, server := range []*httptest.Server{origin, target} {
				certificate = append(certificate, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})...)
			}
			if err := os.WriteFile(caPath, certificate, 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("AWS_CA_BUNDLE", caPath)
			store, _, err := NewS3Store(context.Background(), S3StoreConfig{Endpoint: origin.URL, Region: "auto", Bucket: "bucket", AddressingStyle: "path"})
			if err != nil {
				t.Fatal(err)
			}
			_, err = store.Ensure(context.Background(), PutObject{Key: "app.ipa", Body: strings.NewReader("ipa"), SHA256: sha256Hex([]byte("ipa")), SizeBytes: 3, ContentType: ContentTypeIPA})
			if err == nil {
				t.Fatal("expected redirect response to fail")
			}
			if targetRequests != 0 {
				t.Fatalf("redirect target requests = %d, want 0", targetRequests)
			}
		})
	}
}

func TestStorageErrorsNeverEchoSignedQueryOrSessionMaterial(t *testing.T) {
	err := sanitizedStorageError("put object", "safe/key", errors.New("https://host/key?X-Amz-Signature=secret&X-Amz-Security-Token=session"))
	if text := err.Error(); strings.Contains(text, "secret") || strings.Contains(text, "session") || strings.Contains(text, "X-Amz") {
		t.Fatalf("sanitized error leaked bearer material: %q", text)
	}
}

func TestProviderControlledDiagnosticsNeverEchoMetadataOrAPICode(t *testing.T) {
	secret := "X-Amz-Security-Token=secret\x1b[31m"
	_, err := reconcileStoredObject(
		PutObject{Key: "safe/key", SHA256: "expected", SizeBytes: 1, ContentType: ContentTypeIPA},
		StoredObject{Key: "safe/key", SHA256: secret, SizeBytes: 2, ContentType: secret},
	)
	if err == nil || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "\x1b") {
		t.Fatalf("collision diagnostic leaked provider data: %q", err)
	}
	apiErr := sanitizedStorageError("head object", "safe/key", &maliciousAPIError{code: secret})
	if strings.Contains(apiErr.Error(), "secret") || strings.Contains(apiErr.Error(), "\x1b") {
		t.Fatalf("API diagnostic leaked provider code: %q", apiErr)
	}
}

func TestReconcileStoredObjectNormalizesEquivalentContentTypes(t *testing.T) {
	for _, test := range []struct {
		name     string
		expected string
		provider string
	}{
		{name: "semicolon whitespace and charset case", expected: "text/html; charset=utf-8", provider: " Text/HTML;charset=UTF-8 "},
		{
			name:     "parameter case order and quoted value",
			expected: "text/html; charset=utf-8; level=1; title=release",
			provider: `TEXT/HTML; TITLE="release"; LEVEL=1; CHARSET=UTF-8`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := PutObject{Key: "index.html", SHA256: "digest", SizeBytes: 10, ContentType: test.expected}
			existing := StoredObject{Key: input.Key, SHA256: input.SHA256, SizeBytes: input.SizeBytes, ContentType: test.provider}
			got, err := reconcileStoredObject(input, existing)
			if err != nil {
				t.Fatalf("reconcileStoredObject() error = %v", err)
			}
			if got.Status != "reused" {
				t.Fatalf("status = %q, want reused", got.Status)
			}
		})
	}
}

func TestReconcileStoredObjectRejectsGenuineOrMalformedContentTypeDifferences(t *testing.T) {
	const expected = "text/html; charset=utf-8; profile=mobile"
	for _, test := range []struct {
		name     string
		expected string
		provider string
	}{
		{name: "media type", expected: expected, provider: "application/xml; charset=utf-8; profile=mobile"},
		{name: "charset", expected: expected, provider: "text/html; charset=iso-8859-1; profile=mobile"},
		{name: "extra parameter", expected: expected, provider: "text/html; charset=utf-8; profile=mobile; level=1"},
		{name: "non-charset parameter value case", expected: expected, provider: "text/html; charset=utf-8; profile=Mobile"},
		{name: "malformed provider", expected: expected, provider: "text/html; charset"},
		{name: "malformed expected", expected: "text/html; charset", provider: expected},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := PutObject{Key: "index.html", SHA256: "digest", SizeBytes: 10, ContentType: test.expected}
			existing := StoredObject{Key: input.Key, SHA256: input.SHA256, SizeBytes: input.SizeBytes, ContentType: test.provider}
			if _, err := reconcileStoredObject(input, existing); err == nil {
				t.Fatal("reconcileStoredObject() accepted differing content types")
			}
		})
	}
}

func TestReconcileStoredObjectRetainsDigestAndSizeChecks(t *testing.T) {
	input := PutObject{Key: "index.html", SHA256: "digest", SizeBytes: 10, ContentType: "text/html; charset=utf-8"}
	for _, existing := range []StoredObject{
		{SHA256: "different", SizeBytes: input.SizeBytes, ContentType: input.ContentType},
		{SHA256: input.SHA256, SizeBytes: input.SizeBytes + 1, ContentType: input.ContentType},
	} {
		if _, err := reconcileStoredObject(input, existing); err == nil {
			t.Fatal("reconcileStoredObject() accepted mismatched digest or size")
		}
	}
}

func TestS3EnsureReconcilesAmbiguousPutFailure(t *testing.T) {
	client := &ambiguousPutClient{}
	store := &S3Store{client: client, bucket: "bucket"}
	input := PutObject{Key: "app.ipa", Body: strings.NewReader("ipa"), SHA256: sha256Hex([]byte("ipa")), SizeBytes: 3, ContentType: ContentTypeIPA}
	got, err := store.Ensure(context.Background(), input)
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if got.Status != "reused" || client.headCalls != 2 {
		t.Fatalf("object=%#v headCalls=%d", got, client.headCalls)
	}
}

func TestS3EnsureReconcilesAfterPutContextExpires(t *testing.T) {
	client := &expiredPutClient{}
	contextCalls := 0
	store := &S3Store{
		client: client,
		bucket: "bucket",
		requestContext: func(parent context.Context) (context.Context, context.CancelFunc) {
			contextCalls++
			requestCtx, cancel := context.WithCancel(parent)
			if contextCalls == 2 {
				cancel()
				return requestCtx, func() {}
			}
			return requestCtx, cancel
		},
	}
	input := PutObject{Key: "app.ipa", Body: strings.NewReader("ipa"), SHA256: sha256Hex([]byte("ipa")), SizeBytes: 3, ContentType: ContentTypeIPA}

	got, err := store.Ensure(context.Background(), input)
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if got.Status != "reused" || client.headCalls != 2 || contextCalls != 3 {
		t.Fatalf("object=%#v headCalls=%d contextCalls=%d", got, client.headCalls, contextCalls)
	}
}

func TestS3ReplaceCorruptUsesFreshHeadAndConditionalPut(t *testing.T) {
	body := []byte("expected")
	client := &conditionalReplaceClient{
		object: StoredObject{
			Key: "objects/app.ipa", SHA256: sha256Hex(body), SizeBytes: int64(len(body)),
			ContentType: " Application/Octet-Stream ", entityTag: `"poisoned-generation"`,
		},
	}
	store := &S3Store{client: client, bucket: "bucket"}

	replaced, err := store.ReplaceCorrupt(context.Background(), PutObject{
		Key: "objects/app.ipa", Body: bytes.NewReader(body), SHA256: sha256Hex(body),
		SizeBytes: int64(len(body)), ContentType: ContentTypeIPA,
	})
	if err != nil {
		t.Fatalf("ReplaceCorrupt() error = %v", err)
	}
	if client.ifMatch != `"poisoned-generation"` {
		t.Fatalf("conditional If-Match = %q", client.ifMatch)
	}
	if !bytes.Equal(client.body, body) || replaced.Status != "replaced" {
		t.Fatalf("replacement body=%q object=%#v", client.body, replaced)
	}
}

func TestS3ReplaceCorruptRefusesChangedObjectGeneration(t *testing.T) {
	body := []byte("expected")
	client := &conditionalReplaceClient{
		object: StoredObject{
			Key: "objects/app.ipa", SHA256: sha256Hex([]byte("legitimate-new-object")),
			SizeBytes: int64(len(body)), ContentType: ContentTypeIPA, entityTag: `"new-generation"`,
		},
	}
	store := &S3Store{client: client, bucket: "bucket"}

	_, err := store.ReplaceCorrupt(context.Background(), PutObject{
		Key: "objects/app.ipa", Body: bytes.NewReader(body), SHA256: sha256Hex(body),
		SizeBytes: int64(len(body)), ContentType: ContentTypeIPA,
	})
	if err == nil || !strings.Contains(err.Error(), "refuse") {
		t.Fatalf("ReplaceCorrupt() error = %v, want changed-generation refusal", err)
	}
	if client.ifMatch != "" || client.body != nil {
		t.Fatalf("changed object was overwritten: If-Match=%q body=%q", client.ifMatch, client.body)
	}
}

func TestS3ReplaceCorruptReconcilesAmbiguousConditionalPut(t *testing.T) {
	body := []byte("expected")
	old := StoredObject{SHA256: sha256Hex(body), SizeBytes: int64(len(body)), ContentType: ContentTypeIPA, entityTag: `"old"`}
	replacement := StoredObject{SHA256: sha256Hex(body), SizeBytes: int64(len(body)), ContentType: " Application/Octet-Stream ", entityTag: `"replacement"`}
	for _, test := range []struct {
		name           string
		putErr         error
		after          StoredObject
		headErr        error
		deadlineParent bool
		wantSuccess    bool
		wantConflict   bool
	}{
		{name: "accepted then response lost", putErr: errors.New("connection reset"), after: replacement, wantSuccess: true},
		{name: "accepted then parent deadline", putErr: context.DeadlineExceeded, after: replacement, deadlineParent: true, wantSuccess: true},
		{name: "old generation unchanged", putErr: errors.New("connection reset"), after: old},
		{name: "competing replacement", putErr: errors.New("connection reset"), after: StoredObject{SHA256: "different", SizeBytes: old.SizeBytes, ContentType: old.ContentType, entityTag: `"competitor"`}, wantConflict: true},
		{name: "reconcile HEAD failure", putErr: errors.New("connection reset"), headErr: errors.New("HEAD failed")},
		{name: "reconcile HEAD timeout", putErr: errors.New("connection reset"), headErr: context.DeadlineExceeded},
		{name: "explicit precondition is not success", putErr: &maliciousAPIError{code: "PreconditionFailed"}, after: replacement, wantConflict: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			if test.deadlineParent {
				cancel()
				parent, cancel = context.WithTimeout(context.Background(), 10*time.Millisecond)
			}
			defer cancel()
			client := &ambiguousReplaceClient{before: old, after: test.after, headErr: test.headErr, putErr: test.putErr}
			if test.deadlineParent {
				client.afterPut = func() { <-parent.Done() }
			}
			store := &S3Store{client: client, bucket: "bucket"}
			replaced, err := store.ReplaceCorrupt(parent, PutObject{
				Key: "objects/app.ipa", Body: bytes.NewReader(body), SHA256: sha256Hex(body),
				SizeBytes: int64(len(body)), ContentType: ContentTypeIPA,
			})
			if test.wantSuccess {
				if err != nil {
					t.Fatalf("ReplaceCorrupt() error = %v", err)
				}
				if replaced.Status != "replaced" || replaced.entityTag != replacement.entityTag {
					t.Fatalf("replacement = %#v", replaced)
				}
			} else if err == nil {
				t.Fatal("ReplaceCorrupt() unexpectedly succeeded")
			} else if test.wantConflict && !strings.Contains(err.Error(), "conflict") {
				t.Fatalf("ReplaceCorrupt() error = %v, want conflict", err)
			}
			if client.putCalls != 1 || client.headCalls != 2 {
				t.Fatalf("PutObject calls=%d HeadObject calls=%d, want 1 and 2", client.putCalls, client.headCalls)
			}
			if test.deadlineParent && !errors.Is(parent.Err(), context.DeadlineExceeded) {
				t.Fatalf("parent error = %v, want deadline exceeded", parent.Err())
			}
		})
	}
}

func TestS3ReplaceCorruptPreservesPreexistingParentCancellation(t *testing.T) {
	body := []byte("expected")
	object := StoredObject{SHA256: sha256Hex(body), SizeBytes: int64(len(body)), ContentType: ContentTypeIPA, entityTag: `"old"`}
	client := &ambiguousReplaceClient{before: object}
	store := &S3Store{client: client, bucket: "bucket"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.ReplaceCorrupt(ctx, PutObject{
		Key: "objects/app.ipa", Body: bytes.NewReader(body), SHA256: sha256Hex(body),
		SizeBytes: int64(len(body)), ContentType: ContentTypeIPA,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ReplaceCorrupt() error = %v, want context cancellation", err)
	}
	if client.putCalls != 0 || client.headCalls != 1 {
		t.Fatalf("PutObject calls=%d HeadObject calls=%d, want 0 and 1", client.putCalls, client.headCalls)
	}
}

type conditionalReplaceClient struct {
	object  StoredObject
	ifMatch string
	body    []byte
}

func (client *conditionalReplaceClient) HeadObject(context.Context, *awss3.HeadObjectInput, ...func(*awss3.Options)) (*awss3.HeadObjectOutput, error) {
	return &awss3.HeadObjectOutput{
		ContentLength: aws.Int64(client.object.SizeBytes),
		ContentType:   aws.String(client.object.ContentType),
		ETag:          aws.String(client.object.entityTag),
		Metadata:      map[string]string{objectSHA256MetadataKey: client.object.SHA256},
	}, nil
}

func (client *conditionalReplaceClient) PutObject(_ context.Context, input *awss3.PutObjectInput, _ ...func(*awss3.Options)) (*awss3.PutObjectOutput, error) {
	client.ifMatch = aws.ToString(input.IfMatch)
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	client.body = body
	return &awss3.PutObjectOutput{}, nil
}

type ambiguousReplaceClient struct {
	before, after StoredObject
	headErr       error
	putErr        error
	afterPut      func()
	headCalls     int
	putCalls      int
}

func (client *ambiguousReplaceClient) HeadObject(context.Context, *awss3.HeadObjectInput, ...func(*awss3.Options)) (*awss3.HeadObjectOutput, error) {
	client.headCalls++
	object := client.before
	if client.headCalls > 1 {
		if client.headErr != nil {
			return nil, client.headErr
		}
		object = client.after
	}
	return &awss3.HeadObjectOutput{
		ContentLength: aws.Int64(object.SizeBytes), ContentType: aws.String(object.ContentType),
		ETag: aws.String(object.entityTag), Metadata: map[string]string{objectSHA256MetadataKey: object.SHA256},
	}, nil
}

func (client *ambiguousReplaceClient) PutObject(context.Context, *awss3.PutObjectInput, ...func(*awss3.Options)) (*awss3.PutObjectOutput, error) {
	client.putCalls++
	if client.afterPut != nil {
		client.afterPut()
	}
	return nil, client.putErr
}

type ambiguousPutClient struct{ headCalls int }

func (client *ambiguousPutClient) HeadObject(context.Context, *awss3.HeadObjectInput, ...func(*awss3.Options)) (*awss3.HeadObjectOutput, error) {
	client.headCalls++
	if client.headCalls == 1 {
		return nil, &notFoundAPIError{}
	}
	return &awss3.HeadObjectOutput{ContentLength: aws.Int64(3), ContentType: aws.String(ContentTypeIPA), Metadata: map[string]string{objectSHA256MetadataKey: sha256Hex([]byte("ipa"))}}, nil
}

func (*ambiguousPutClient) PutObject(_ context.Context, input *awss3.PutObjectInput, _ ...func(*awss3.Options)) (*awss3.PutObjectOutput, error) {
	digest, _ := hex.DecodeString(sha256Hex([]byte("ipa")))
	if aws.ToString(input.ChecksumSHA256) != base64.StdEncoding.EncodeToString(digest) {
		return nil, errors.New("missing exact payload checksum")
	}
	return nil, errors.New("connection reset after server accepted object")
}

type expiredPutClient struct{ headCalls int }

func (client *expiredPutClient) HeadObject(ctx context.Context, _ *awss3.HeadObjectInput, _ ...func(*awss3.Options)) (*awss3.HeadObjectOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	client.headCalls++
	if client.headCalls == 1 {
		return nil, &notFoundAPIError{}
	}
	return &awss3.HeadObjectOutput{ContentLength: aws.Int64(3), ContentType: aws.String(ContentTypeIPA), Metadata: map[string]string{objectSHA256MetadataKey: sha256Hex([]byte("ipa"))}}, nil
}

func (*expiredPutClient) PutObject(ctx context.Context, _ *awss3.PutObjectInput, _ ...func(*awss3.Options)) (*awss3.PutObjectOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("expected canceled PUT context")
}

type notFoundAPIError struct{}

func (*notFoundAPIError) Error() string                 { return "not found" }
func (*notFoundAPIError) ErrorCode() string             { return "NotFound" }
func (*notFoundAPIError) ErrorMessage() string          { return "not found" }
func (*notFoundAPIError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

type maliciousAPIError struct{ code string }

func (err *maliciousAPIError) Error() string             { return err.code }
func (err *maliciousAPIError) ErrorCode() string         { return err.code }
func (err *maliciousAPIError) ErrorMessage() string      { return err.code }
func (*maliciousAPIError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }
