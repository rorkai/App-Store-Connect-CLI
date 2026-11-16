package builds

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDownloadDSYMRefusesRedirectWithoutForwardingReferer(t *testing.T) {
	var redirectedRequests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectedRequests.Add(1)
		t.Errorf("redirect target received request with referer %q", r.Referer())
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/capture", http.StatusFound)
	}))
	defer source.Close()

	_, err := downloadDSYM(
		context.Background(),
		dsymDownloadTestDeps(source.Client().Transport),
		source.URL+"/symbols?Signature=signed-secret",
		filepath.Join(t.TempDir(), "symbols.zip"),
	)
	if err == nil {
		t.Fatal("expected redirect response to fail")
	}
	if got := redirectedRequests.Load(); got != 0 {
		t.Fatalf("redirect target received %d requests, want 0", got)
	}
	if strings.Contains(err.Error(), "signed-secret") {
		t.Fatalf("redirect error leaked signed query: %v", err)
	}
}

func TestDownloadDSYMSanitizesTransportErrorAndPreservesCause(t *testing.T) {
	sentinel := errors.New("dial failed for https://download.example/file?Signature=signed-secret")
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, sentinel
	})

	_, err := downloadDSYM(
		context.Background(),
		dsymDownloadTestDeps(transport),
		"https://download.example/file?Signature=signed-secret",
		filepath.Join(t.TempDir(), "symbols.zip"),
	)
	if err == nil {
		t.Fatal("expected transport error")
	}
	if strings.Contains(err.Error(), "signed-secret") || strings.Contains(err.Error(), "Signature") {
		t.Fatalf("transport error leaked signed query: %v", err)
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("transport cause was not preserved: %v", err)
	}
}

func TestDownloadDSYMDoesNotWriteRedirectOrErrorBody(t *testing.T) {
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Status:     "403 Forbidden",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("signed-secret")),
		}, nil
	})

	dest := filepath.Join(t.TempDir(), "symbols.zip")
	if _, err := downloadDSYM(context.Background(), dsymDownloadTestDeps(transport), "https://download.example/file", dest); err == nil {
		t.Fatal("expected HTTP error")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
