package asc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func TestWaitForBuildProcessing_ReturnsValid(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	calls := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		state := BuildProcessingStateProcessing
		if calls > 1 {
			state = BuildProcessingStateValid
		}
		body := fmt.Sprintf(`{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"%s"}}}`, state)
		return jsonResponse(http.StatusOK, body), nil
	})

	client := &Client{
		httpClient: &http.Client{Transport: transport},
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: key,
	}

	var build *BuildResponse
	synctest.Test(t, func(*testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		build, err = client.WaitForBuildProcessing(ctx, "build-1", 1*time.Millisecond)
	})
	if err != nil {
		t.Fatalf("WaitForBuildProcessing() error: %v", err)
	}
	if build.Data.Attributes.ProcessingState != BuildProcessingStateValid {
		t.Fatalf("expected processing state %q, got %q", BuildProcessingStateValid, build.Data.Attributes.ProcessingState)
	}
}

func TestWaitForBuildProcessing_InvalidReturnsError(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := fmt.Sprintf(`{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"%s"}}}`, BuildProcessingStateInvalid)
		return jsonResponse(http.StatusOK, body), nil
	})

	client := &Client{
		httpClient: &http.Client{Transport: transport},
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: key,
	}

	var build *BuildResponse
	synctest.Test(t, func(*testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		build, err = client.WaitForBuildProcessing(ctx, "build-1", 1*time.Millisecond)
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if build == nil || build.Data.Attributes.ProcessingState != BuildProcessingStateInvalid {
		t.Fatalf("expected terminal INVALID build response, got %#v", build)
	}
}

func TestWaitForBuildProcessing_FailedReturnsError(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := fmt.Sprintf(`{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"%s"}}}`, BuildProcessingStateFailed)
		return jsonResponse(http.StatusOK, body), nil
	})

	client := &Client{
		httpClient: &http.Client{Transport: transport},
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: key,
	}

	var build *BuildResponse
	synctest.Test(t, func(*testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		build, err = client.WaitForBuildProcessing(ctx, "build-1", 1*time.Millisecond)
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), BuildProcessingStateFailed) {
		t.Fatalf("expected FAILED error, got %v", err)
	}
	if build == nil || build.Data.Attributes.ProcessingState != BuildProcessingStateFailed {
		t.Fatalf("expected terminal FAILED build response, got %#v", build)
	}
}

func TestWaitForBuildProcessing_ToleratesTransientLookupFailures(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	calls := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls <= 2 {
			return jsonResponse(http.StatusServiceUnavailable, `{"errors":[{"status":"503","code":"SERVICE_UNAVAILABLE","title":"unavailable"}]}`), nil
		}
		body := fmt.Sprintf(`{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"%s"}}}`, BuildProcessingStateValid)
		return jsonResponse(http.StatusOK, body), nil
	})

	client := &Client{
		httpClient: &http.Client{Transport: transport},
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: key,
	}

	var build *BuildResponse
	stderr := captureWaitStderr(t, func() {
		build, err = client.WaitForBuildProcessing(context.Background(), "build-1", time.Millisecond)
	})
	if err != nil {
		t.Fatalf("WaitForBuildProcessing() error: %v", err)
	}
	if build == nil || build.Data.Attributes.ProcessingState != BuildProcessingStateValid {
		t.Fatalf("expected VALID build after transient failures, got %#v", build)
	}
	for _, want := range []string{
		"transient App Store Connect error while waiting (1/5)",
		"transient App Store Connect error while waiting (2/5)",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}

	calls = 0
	var notices bytes.Buffer
	processStderr := captureWaitStderr(t, func() {
		build, err = client.WaitForBuildProcessingTo(context.Background(), &notices, "build-1", time.Millisecond)
	})
	if err != nil || build == nil || build.Data.Attributes.ProcessingState != BuildProcessingStateValid {
		t.Fatalf("WaitForBuildProcessingTo() = %#v, %v; want the VALID build", build, err)
	}
	if processStderr != "" {
		t.Fatalf("process stderr = %q, want empty", processStderr)
	}
	if notices.String() != stderr {
		t.Fatalf("notices = %q, want the process form's %q", notices.String(), stderr)
	}
}

func TestWaitForBuildProcessing_FailsAfterConsecutiveTransientLimit(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	calls := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return jsonResponse(http.StatusServiceUnavailable, `{"errors":[{"status":"503","code":"SERVICE_UNAVAILABLE","title":"unavailable"}]}`), nil
	})

	client := &Client{
		httpClient: &http.Client{Transport: transport},
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: key,
	}

	captureWaitStderr(t, func() {
		_, err = client.WaitForBuildProcessing(context.Background(), "build-1", time.Millisecond)
	})
	if err == nil {
		t.Fatal("expected error once transient failures exceed the ceiling, got nil")
	}
	if !strings.Contains(err.Error(), "giving up after 6 consecutive transient App Store Connect errors") {
		t.Fatalf("expected consecutive transient failure error, got %v", err)
	}
	if calls != DefaultMaxConsecutivePollFailures+1 {
		t.Fatalf("expected %d lookups, got %d", DefaultMaxConsecutivePollFailures+1, calls)
	}
}

func captureWaitStderr(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}
	os.Stderr = writer

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, reader)
		_ = reader.Close()
		done <- buf.String()
	}()

	fn()

	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("close error: %v", closeErr)
	}
	os.Stderr = orig

	return <-done
}
