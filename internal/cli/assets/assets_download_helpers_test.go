package assets

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type readerThatFailsAfterFirstRead struct {
	readOnce bool
}

func TestDownloadHTTPStatusErrorExposesHTTPStatus(t *testing.T) {
	err := &downloadHTTPStatusError{StatusCode: 503}

	if got := err.HTTPStatusCode(); got != 503 {
		t.Fatalf("HTTPStatusCode() = %d, want 503", got)
	}
}

func (r *readerThatFailsAfterFirstRead) Read(p []byte) (int, error) {
	if !r.readOnce {
		r.readOnce = true
		return copy(p, "NEW-DATA"), nil
	}
	return 0, errors.New("simulated read failure")
}

func TestWriteDownloadedFile_Overwrite_ErrorPreservesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.bin")

	if err := os.WriteFile(path, []byte("OLD-DATA"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := writeDownloadedFile(path, &readerThatFailsAfterFirstRead{}, true)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile() error: %v", readErr)
	}
	if string(data) != "OLD-DATA" {
		t.Fatalf("expected existing file contents preserved, got %q", string(data))
	}
}

func TestWriteDownloadedFile_Overwrite_ReplacesExistingFileOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.bin")

	if err := os.WriteFile(path, []byte("OLD-DATA"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	written, err := writeDownloadedFile(path, strings.NewReader("NEW-DATA"), true)
	if err != nil {
		t.Fatalf("writeDownloadedFile() error: %v", err)
	}
	if written != int64(len("NEW-DATA")) {
		t.Fatalf("expected written=%d, got %d", len("NEW-DATA"), written)
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile() error: %v", readErr)
	}
	if string(data) != "NEW-DATA" {
		t.Fatalf("expected new file contents, got %q", string(data))
	}
}

func TestIsRetryableDownloadError_ContextErrorsAreNotRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "deadline exceeded",
			err:  &url.Error{Op: "Get", URL: "https://example.com", Err: context.DeadlineExceeded},
		},
		{
			name: "context canceled",
			err:  &url.Error{Op: "Get", URL: "https://example.com", Err: context.Canceled},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if isRetryableDownloadError(tt.err) {
				t.Fatalf("expected non-retryable error for %q", tt.name)
			}
		})
	}
}

func TestIsRetryableDownloadError_TransientNetworkErrorIsRetryable(t *testing.T) {
	err := &url.Error{
		Op:  "Get",
		URL: "https://example.com",
		Err: &net.DNSError{IsTimeout: true},
	}
	if !isRetryableDownloadError(err) {
		t.Fatalf("expected retryable network error")
	}
}
