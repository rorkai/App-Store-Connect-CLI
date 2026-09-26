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
	"sync/atomic"
	"testing"
	"time"
)

func TestValidateImageFileRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.png")
	if err := os.WriteFile(target, []byte("data"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	link := filepath.Join(dir, "link.png")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	if err := ValidateImageFile(link); err == nil {
		t.Fatalf("expected symlink error, got nil")
	}
}

func TestValidateAssetFileRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.bin")
	if err := os.WriteFile(target, []byte("data"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	link := filepath.Join(dir, "link.bin")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	if err := ValidateAssetFile(link); err == nil {
		t.Fatalf("expected symlink error, got nil")
	}
}

func TestUploadAssetRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.bin")
	if err := os.WriteFile(target, []byte("abc"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	link := filepath.Join(dir, "link.bin")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := UploadAsset(context.Background(), link, []UploadOperation{
		{Method: http.MethodPut, URL: server.URL + "/part1", Length: 3, Offset: 0},
	})
	if err == nil {
		t.Fatal("expected symlink upload to fail")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink error, got %v", err)
	}
	if atomic.LoadInt32(&calls) != 0 {
		t.Fatalf("expected no upload requests, got %d", calls)
	}
}

func TestComputeFileChecksumRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.bin")
	if err := os.WriteFile(target, []byte("abc"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	link := filepath.Join(dir, "link.bin")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}

	_, err := ComputeFileChecksum(link, ChecksumAlgorithmMD5)
	if err == nil {
		t.Fatal("expected symlink checksum to fail")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink error, got %v", err)
	}
}

func TestValidateImageFileRejectsOversize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.bin")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	defer file.Close()

	if err := file.Truncate(maxAssetFileSize + 1); err != nil {
		t.Fatalf("truncate file: %v", err)
	}

	if err := ValidateImageFile(path); err == nil {
		t.Fatalf("expected size error, got nil")
	}
}

func TestValidateAssetFileInfoRejectsGrowthAfterPathValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "screenshot.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	defer file.Close()
	if _, err := file.Write([]byte("image")); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := ValidateImageFile(path); err != nil {
		t.Fatalf("initial path validation: %v", err)
	}
	if err := file.Truncate(maxAssetFileSize + 1); err != nil {
		t.Fatalf("grow file: %v", err)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatalf("stat opened file: %v", err)
	}
	if err := ValidateAssetFileInfo(path, info); err == nil || !strings.Contains(err.Error(), "file size exceeds") {
		t.Fatalf("opened file validation error = %v, want size limit", err)
	}
}

func TestValidateAssetFileRejectsOversize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.bin")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	defer file.Close()

	if err := file.Truncate(maxAssetFileSize + 1); err != nil {
		t.Fatalf("truncate file: %v", err)
	}

	if err := ValidateAssetFile(path); err == nil {
		t.Fatalf("expected size error, got nil")
	}
}

func TestUploadAssetFromFileUploadsChunks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "asset.bin")
	if err := os.WriteFile(path, []byte("abcdef"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer file.Close()

	var call int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&call, 1)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		switch r.URL.Path {
		case "/part1":
			if string(body) != "abc" {
				t.Fatalf("expected part1 body 'abc', got %q", string(body))
			}
		case "/part2":
			if string(body) != "def" {
				t.Fatalf("expected part2 body 'def', got %q", string(body))
			}
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ops := []UploadOperation{
		{Method: "PUT", URL: server.URL + "/part1", Length: 3, Offset: 0},
		{Method: "PUT", URL: server.URL + "/part2", Length: 3, Offset: 3},
	}

	if err := UploadAssetFromFile(context.Background(), file, 6, ops); err != nil {
		t.Fatalf("UploadAssetFromFile() error: %v", err)
	}
	if atomic.LoadInt32(&call) != 2 {
		t.Fatalf("expected 2 upload calls, got %d", call)
	}
}

func TestUploadAssetFromFileRejectsNonPositiveLengthBeforeUpload(t *testing.T) {
	file := createTempAssetFile(t, []byte("abc"))
	defer file.Close()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tests := []struct {
		name   string
		length int64
		want   string
	}{
		{name: "zero", length: 0, want: "non-positive length"},
		{name: "negative", length: -1, want: "negative offset/length"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := calls.Load()
			err := UploadAssetFromFile(context.Background(), file, 3, []UploadOperation{{
				Method: http.MethodPut,
				URL:    server.URL + "/part",
				Length: tt.length,
			}})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("UploadAssetFromFile() error = %v, want %q", err, tt.want)
			}
			if got := calls.Load(); got != before {
				t.Fatalf("expected no upload requests, got %d new request(s)", got-before)
			}
		})
	}
}

func TestUploadAssetFromFilePreflightsAllOperationsBeforeUpload(t *testing.T) {
	file := createTempAssetFile(t, []byte("abcdef"))
	defer file.Close()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := UploadAssetFromFile(context.Background(), file, 6, []UploadOperation{
		{Method: http.MethodPut, URL: server.URL + "/part1", Length: 3, Offset: 0},
		{Method: http.MethodPut, URL: server.URL + "/part2", Length: 0, Offset: 3},
	})
	if err == nil || !strings.Contains(err.Error(), "upload operation 1 has non-positive length") {
		t.Fatalf("UploadAssetFromFile() error = %v, want non-positive length for operation 1", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("expected no upload requests after preflight failure, got %d", got)
	}
}

func TestUploadAssetFromFileUsesUploadTimeoutEnv(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "10ms")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")
	t.Setenv("ASC_UPLOAD_TIMEOUT", "250ms")
	t.Setenv("ASC_UPLOAD_TIMEOUT_SECONDS", "")

	file := createTempAssetFile(t, []byte("abc"))
	defer file.Close()

	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		time.Sleep(60 * time.Millisecond)
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ops := []UploadOperation{
		{Method: http.MethodPut, URL: server.URL + "/part1", Length: 3, Offset: 0},
	}

	if err := UploadAssetFromFile(context.Background(), file, 3, ops); err != nil {
		t.Fatalf("UploadAssetFromFile() error: %v", err)
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Fatalf("expected 1 upload call, got %d", callCount)
	}
}

func TestUploadAssetFromFileUsesUploadTimeoutWhenShorter(t *testing.T) {
	// Retries are covered separately; a single attempt keeps this focused on
	// the per-attempt upload timeout.
	setFastAssetUploadRetries(t, "0")
	t.Setenv("ASC_TIMEOUT", "250ms")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")
	t.Setenv("ASC_UPLOAD_TIMEOUT", "10ms")
	t.Setenv("ASC_UPLOAD_TIMEOUT_SECONDS", "")

	file := createTempAssetFile(t, []byte("abc"))
	defer file.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(60 * time.Millisecond)
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ops := []UploadOperation{
		{Method: http.MethodPut, URL: server.URL + "/part1", Length: 3, Offset: 0},
	}

	err := UploadAssetFromFile(context.Background(), file, 3, ops)
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "Client.Timeout exceeded") {
		t.Fatalf("expected client timeout error, got %v", err)
	}
}

func createTempAssetFile(t *testing.T, content []byte) *os.File {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "asset.bin")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}

	return file
}
