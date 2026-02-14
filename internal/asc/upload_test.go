package asc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestExecuteUploadOperations_UploadsSlices(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "app.ipa")
	content := []byte("abcdefghijklmnopqrstuvwxyz")
	if err := os.WriteFile(filePath, content, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var mu sync.Mutex
	received := map[string]string{}
	headers := map[string]string{}
	methods := map[string]string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		mu.Lock()
		received[r.URL.Path] = string(body)
		headers[r.URL.Path] = r.Header.Get("X-Test")
		methods[r.URL.Path] = r.Method
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ops := []UploadOperation{
		{
			Method: "PUT",
			URL:    server.URL + "/op0",
			Length: 5,
			Offset: 0,
			RequestHeaders: []HTTPHeader{
				{Name: "X-Test", Value: "alpha"},
			},
		},
		{
			Method: "PUT",
			URL:    server.URL + "/op1",
			Length: 4,
			Offset: 5,
			RequestHeaders: []HTTPHeader{
				{Name: "X-Test", Value: "bravo"},
			},
		},
	}

	err := ExecuteUploadOperations(context.Background(), filePath, ops,
		WithUploadConcurrency(2),
		WithUploadHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("ExecuteUploadOperations() error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if received["/op0"] != "abcde" {
		t.Fatalf("expected /op0 body=abcde, got %q", received["/op0"])
	}
	if received["/op1"] != "fghi" {
		t.Fatalf("expected /op1 body=fghi, got %q", received["/op1"])
	}
	if headers["/op0"] != "alpha" || headers["/op1"] != "bravo" {
		t.Fatalf("expected headers alpha/bravo, got %q and %q", headers["/op0"], headers["/op1"])
	}
	if methods["/op0"] != http.MethodPut || methods["/op1"] != http.MethodPut {
		t.Fatalf("expected PUT methods, got %q and %q", methods["/op0"], methods["/op1"])
	}
}

func TestExecuteUploadOperations_FailsOnHTTPError(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "app.ipa")
	if err := os.WriteFile(filePath, []byte("abcdefghij"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if strings.Contains(r.URL.Path, "op1") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ops := []UploadOperation{
		{
			Method: "PUT",
			URL:    server.URL + "/op0",
			Length: 5,
			Offset: 0,
		},
		{
			Method: "PUT",
			URL:    server.URL + "/op1",
			Length: 5,
			Offset: 5,
		},
	}

	err := ExecuteUploadOperations(context.Background(), filePath, ops, WithUploadConcurrency(1))
	if err == nil {
		t.Fatalf("expected error from ExecuteUploadOperations")
	}
}

func TestExecuteUploadOperations_FailsOnInvalidRange(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "app.ipa")
	if err := os.WriteFile(filePath, []byte("abc"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	ops := []UploadOperation{
		{
			Method: "PUT",
			URL:    "https://example.com/upload",
			Length: 10,
			Offset: 0,
		},
	}

	err := ExecuteUploadOperations(context.Background(), filePath, ops)
	if err == nil {
		t.Fatalf("expected range validation error")
	}
}

func TestExecuteUploadOperations_RejectsSymlinkPath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.ipa")
	if err := os.WriteFile(target, []byte("abc"), 0o600); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	link := filepath.Join(dir, "app.ipa")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	ops := []UploadOperation{
		{
			Method: "PUT",
			URL:    "https://example.com/upload",
			Length: 1,
			Offset: 0,
		},
	}

	err := ExecuteUploadOperations(context.Background(), link, ops)
	if err == nil {
		t.Fatal("expected symlink rejection error")
	}
	if !strings.Contains(err.Error(), "refusing to read symlink") {
		t.Fatalf("expected symlink rejection error, got %v", err)
	}
}

func TestExecuteUploadOperations_CancelsDuringDispatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "app.ipa")
	if err := os.WriteFile(filePath, []byte("abcdefghij"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	started := make(chan struct{})
	var startedOnce sync.Once
	var op1Seen int32

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/op0":
				startedOnce.Do(func() { close(started) })
				<-req.Context().Done()
				return nil, req.Context().Err()
			case "/op1":
				atomic.StoreInt32(&op1Seen, 1)
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
				}, nil
			default:
				return nil, errors.New("unexpected request path: " + req.URL.Path)
			}
		}),
	}

	ops := []UploadOperation{
		{
			Method: "PUT",
			URL:    "https://example.test/op0",
			Length: 5,
			Offset: 0,
		},
		{
			Method: "PUT",
			URL:    "https://example.test/op1",
			Length: 5,
			Offset: 5,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ExecuteUploadOperations(ctx, filePath, ops,
			WithUploadConcurrency(1),
			WithUploadHTTPClient(client),
		)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for upload dispatch")
	}

	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected cancellation error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for cancellation")
	}

	if atomic.LoadInt32(&op1Seen) != 0 {
		t.Fatalf("unexpected upload dispatch after cancellation")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestComputeFileChecksum_MD5(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "checksum.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	sum, err := ComputeFileChecksum(filePath, ChecksumAlgorithmMD5)
	if err != nil {
		t.Fatalf("ComputeFileChecksum() error: %v", err)
	}
	if sum.Hash != "5d41402abc4b2a76b9719d911017c592" {
		t.Fatalf("unexpected MD5 hash: %s", sum.Hash)
	}
}

func TestVerifySourceFileChecksums(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "checksum.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	expected := &Checksums{
		File: &Checksum{
			Hash:      "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
			Algorithm: ChecksumAlgorithmSHA256,
		},
	}

	computed, err := VerifySourceFileChecksums(filePath, expected)
	if err != nil {
		t.Fatalf("VerifySourceFileChecksums() error: %v", err)
	}
	if computed.File == nil || computed.File.Hash != expected.File.Hash {
		t.Fatalf("expected SHA256 hash %s, got %#v", expected.File.Hash, computed.File)
	}
}
