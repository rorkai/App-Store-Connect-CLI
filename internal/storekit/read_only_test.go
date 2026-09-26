package storekit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

func TestReadOnlyModeRefusesRetentionWritesBeforeSending(t *testing.T) {
	t.Setenv(readonly.EnvVar, "1")

	var sent atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"images":[]}`))
	}))
	defer server.Close()

	client, err := NewClient(testCredentials(t), Sandbox, WithHTTPClient(server.Client()), WithBaseURL(server.URL+"/inApps/v1/messaging"))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if err := client.UploadImage(context.Background(), "11111111-2222-4333-8444-555555555555", ImageSizeFull, []byte("png")); !errors.Is(err, readonly.ErrRefused) {
		t.Fatalf("UploadImage() error = %v, want readonly.ErrRefused", err)
	}
	if err := client.DeleteImage(context.Background(), "11111111-2222-4333-8444-555555555555"); !errors.Is(err, readonly.ErrRefused) {
		t.Fatalf("DeleteImage() error = %v, want readonly.ErrRefused", err)
	}
	if got := sent.Load(); got != 0 {
		t.Fatalf("server received %d requests, want 0", got)
	}

	if _, err := client.ListImages(context.Background()); err != nil {
		t.Fatalf("ListImages() error = %v, want nil", err)
	}
	if got := sent.Load(); got != 1 {
		t.Fatalf("server received %d requests after GET, want 1", got)
	}
}
