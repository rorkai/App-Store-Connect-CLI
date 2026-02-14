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
