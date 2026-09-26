package asc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

func TestReadOnlyModeRefusesMutatingRequestsBeforeSending(t *testing.T) {
	t.Setenv(readonly.EnvVar, "1")

	var sent atomic.Int32
	client := newTestServerClient(t, func(w http.ResponseWriter, r *http.Request) {
		sent.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	})

	for _, method := range []string{http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete} {
		_, err := client.do(context.Background(), method, "/v1/apps/123", strings.NewReader(`{}`))
		if err == nil {
			t.Fatalf("%s error = nil, want read-only refusal", method)
		}
		if !errors.Is(err, readonly.ErrRefused) {
			t.Fatalf("%s error = %v, want readonly.ErrRefused", method, err)
		}
		want := "ASC_READ_ONLY is set; refusing " + method + " /v1/apps/123"
		if err.Error() != want {
			t.Fatalf("%s error = %q, want %q", method, err.Error(), want)
		}
	}
	if got := sent.Load(); got != 0 {
		t.Fatalf("server received %d mutating requests, want 0", got)
	}

	if _, err := client.do(context.Background(), http.MethodGet, "/v1/apps", nil); err != nil {
		t.Fatalf("GET error = %v, want nil", err)
	}
	if got := sent.Load(); got != 1 {
		t.Fatalf("server received %d requests after GET, want 1", got)
	}
}

func TestReadOnlyModeRefusesUploadOperations(t *testing.T) {
	t.Setenv(readonly.EnvVar, "1")

	var sent atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	path := filepath.Join(t.TempDir(), "asset.bin")
	if err := os.WriteFile(path, []byte("payload"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	operations := []UploadOperation{{
		Method: http.MethodPut,
		URL:    server.URL + "/upload/part?signature=secret",
		Length: 7,
		Offset: 0,
	}}
	err := ExecuteUploadOperations(context.Background(), path, operations, WithUploadHTTPClient(server.Client()))
	if !errors.Is(err, readonly.ErrRefused) {
		t.Fatalf("ExecuteUploadOperations() error = %v, want readonly.ErrRefused", err)
	}
	if strings.Contains(err.Error(), "signature=secret") {
		t.Fatalf("refusal leaked the signed query string: %v", err)
	}
	if got := sent.Load(); got != 0 {
		t.Fatalf("upload server received %d requests, want 0", got)
	}
}
