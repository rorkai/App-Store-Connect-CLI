package distribution

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

func TestReadOnlyModeRefusesS3PutAfterHeadMiss(t *testing.T) {
	t.Setenv(readonly.EnvVar, "1")
	t.Setenv("ASC_S3_ACCESS_KEY_ID", "test-access")
	t.Setenv("ASC_S3_SECRET_ACCESS_KEY", "test-secret")
	t.Setenv("ASC_S3_SESSION_TOKEN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")

	var puts atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodHead:
			writer.WriteHeader(http.StatusNotFound)
		default:
			puts.Add(1)
			t.Errorf("mutating request left the S3 store: %s %s", request.Method, request.URL.Path)
			writer.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	store, _, err := NewS3Store(context.Background(), S3StoreConfig{
		Endpoint: server.URL, Region: "auto", Bucket: "bucket", AddressingStyle: "path", HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewS3Store() error = %v", err)
	}
	input := PutObject{Key: "app.ipa", Body: strings.NewReader("ipa"), SHA256: sha256Hex([]byte("ipa")), SizeBytes: 3, ContentType: ContentTypeIPA}
	_, err = store.Ensure(context.Background(), input)
	if !errors.Is(err, readonly.ErrRefused) {
		t.Fatalf("Ensure() error = %v, want readonly.ErrRefused", err)
	}
	if !strings.Contains(err.Error(), "refusing PUT s3://bucket/app.ipa") {
		t.Fatalf("Ensure() error = %q, want the refused object target", err.Error())
	}
	if got := puts.Load(); got != 0 {
		t.Fatalf("S3 received %d mutating requests, want 0", got)
	}
}

func TestReadOnlyModeRefusesS3ConditionalReplacement(t *testing.T) {
	t.Setenv(readonly.EnvVar, "1")

	body := []byte("expected")
	client := &conditionalReplaceClient{
		object: StoredObject{
			Key: "objects/app.ipa", SHA256: sha256Hex(body), SizeBytes: int64(len(body)),
			ContentType: ContentTypeIPA, entityTag: `"poisoned-generation"`,
		},
	}
	store := &S3Store{client: client, bucket: "bucket"}

	_, err := store.ReplaceCorrupt(context.Background(), PutObject{
		Key: "objects/app.ipa", Body: bytes.NewReader(body), SHA256: sha256Hex(body),
		SizeBytes: int64(len(body)), ContentType: ContentTypeIPA,
	})
	if !errors.Is(err, readonly.ErrRefused) {
		t.Fatalf("ReplaceCorrupt() error = %v, want readonly.ErrRefused", err)
	}
	if !strings.Contains(err.Error(), "refusing PUT s3://bucket/objects/app.ipa") {
		t.Fatalf("ReplaceCorrupt() error = %q, want the refused object target", err.Error())
	}
	if client.body != nil || client.ifMatch != "" {
		t.Fatalf("conditional PUT reached S3: body=%q ifMatch=%q", client.body, client.ifMatch)
	}
}
