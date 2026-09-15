package builds

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type buildsWaitRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn buildsWaitRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func newBuildsWaitTestClient(t *testing.T, transport buildsWaitRoundTripFunc) *asc.Client {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error: %v", err)
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if data == nil {
		t.Fatal("failed to encode PEM")
	}
	keyPath := filepath.Join(t.TempDir(), "key.p8")
	if err := os.WriteFile(keyPath, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, &http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient() error: %v", err)
	}
	return client
}

func buildsWaitJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func buildsWaitServiceUnavailable() (*http.Response, error) {
	return buildsWaitJSONResponse(http.StatusServiceUnavailable, `{
		"errors": [
			{"status": "503", "code": "SERVICE_UNAVAILABLE", "title": "unavailable"}
		]
	}`)
}

func captureBuildsWaitStderr(t *testing.T, fn func()) string {
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

func TestWaitForBuildProcessingStateToleratesTransientLookupFailures(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	calls := 0
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds/build-1" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		calls++
		if calls <= 2 {
			return buildsWaitServiceUnavailable()
		}
		return buildsWaitJSONResponse(http.StatusOK, `{
			"data": {
				"type": "builds",
				"id": "build-1",
				"attributes": {"version": "42", "processingState": "VALID"}
			}
		}`)
	})

	var buildResp *asc.BuildResponse
	var err error
	stderr := captureBuildsWaitStderr(t, func() {
		buildResp, err = waitForBuildProcessingState(context.Background(), client, "build-1", time.Millisecond, false)
	})
	if err != nil {
		t.Fatalf("waitForBuildProcessingState() error: %v", err)
	}
	if buildResp == nil || buildResp.Data.Attributes.ProcessingState != asc.BuildProcessingStateValid {
		t.Fatalf("expected VALID build after transient failures, got %#v", buildResp)
	}
	for _, want := range []string{
		"transient App Store Connect error while waiting (1/5)",
		"transient App Store Connect error while waiting (2/5)",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}
}

func TestWaitForBuildProcessingStateFailsAfterConsecutiveTransientLimit(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	calls := 0
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds/build-1" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		calls++
		return buildsWaitServiceUnavailable()
	})

	var err error
	captureBuildsWaitStderr(t, func() {
		_, err = waitForBuildProcessingState(context.Background(), client, "build-1", time.Millisecond, false)
	})
	if err == nil {
		t.Fatal("expected error once transient failures exceed the ceiling, got nil")
	}
	if !strings.Contains(err.Error(), "giving up after 6 consecutive transient App Store Connect errors") {
		t.Fatalf("expected consecutive transient failure error, got %v", err)
	}
	if calls != asc.DefaultMaxConsecutivePollFailures+1 {
		t.Fatalf("expected %d lookups, got %d", asc.DefaultMaxConsecutivePollFailures+1, calls)
	}
}

func TestWaitForBuildProcessingStateReturnsTerminalFailure(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds/build-1" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		return buildsWaitJSONResponse(http.StatusOK, `{
			"data": {
				"type": "builds",
				"id": "build-1",
				"attributes": {"version": "42", "processingState": "FAILED"}
			}
		}`)
	})

	var err error
	captureBuildsWaitStderr(t, func() {
		_, err = waitForBuildProcessingState(context.Background(), client, "build-1", time.Millisecond, false)
	})
	if err == nil {
		t.Fatal("expected terminal FAILED error, got nil")
	}
	if !strings.Contains(err.Error(), "build processing failed with state FAILED") {
		t.Fatalf("expected terminal FAILED error, got %v", err)
	}
}

func TestWaitForBuildDiscoveryToleratesTransientLookupFailures(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	calls := 0
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		calls++
		if calls <= 2 {
			return buildsWaitServiceUnavailable()
		}
		return buildsWaitJSONResponse(http.StatusOK, `{
			"data": [
				{
					"type": "builds",
					"id": "build-1",
					"attributes": {"version": "42", "processingState": "PROCESSING", "uploadedDate": "2026-03-16T12:00:05Z"}
				}
			],
			"links": {}
		}`)
	})

	selector := appBuildWaitSelector{
		AppID:       "123456789",
		BuildNumber: "42",
		Platform:    "IOS",
	}

	var buildResp *asc.BuildResponse
	var err error
	stderr := captureBuildsWaitStderr(t, func() {
		buildResp, err = waitForBuildDiscovery(context.Background(), client, selector, time.Millisecond)
	})
	if err != nil {
		t.Fatalf("waitForBuildDiscovery() error: %v", err)
	}
	if buildResp == nil || buildResp.Data.ID != "build-1" {
		t.Fatalf("expected discovered build-1 after transient failures, got %#v", buildResp)
	}
	for _, want := range []string{
		"transient App Store Connect error while waiting (1/5)",
		"transient App Store Connect error while waiting (2/5)",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}
}

func TestWaitForBuildDiscoveryFailsAfterConsecutiveTransientLimit(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	calls := 0
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		calls++
		return buildsWaitServiceUnavailable()
	})

	selector := appBuildWaitSelector{
		AppID:       "123456789",
		BuildNumber: "42",
		Platform:    "IOS",
	}

	var err error
	captureBuildsWaitStderr(t, func() {
		_, err = waitForBuildDiscovery(context.Background(), client, selector, time.Millisecond)
	})
	if err == nil {
		t.Fatal("expected error once transient failures exceed the ceiling, got nil")
	}
	if !strings.Contains(err.Error(), "giving up after 6 consecutive transient App Store Connect errors") {
		t.Fatalf("expected consecutive transient failure error, got %v", err)
	}
	if calls != asc.DefaultMaxConsecutivePollFailures+1 {
		t.Fatalf("expected %d lookups, got %d", asc.DefaultMaxConsecutivePollFailures+1, calls)
	}
}

// The --since build-number path reports later-page matches from the response's
// pagination links, not from the fact that a second match was seen.
func TestResolveBuildByNumberSelectionSinceReportsCandidatesWithoutClaimingLaterPages(t *testing.T) {
	client := newBuildsWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		return buildsWaitJSONResponse(http.StatusOK, `{"data":[
			{"type":"builds","id":"build-ios","attributes":{"version":"42","uploadedDate":"2026-03-02T18:01:00Z","processingState":"VALID"}},
			{"type":"builds","id":"build-macos","attributes":{"version":"42","uploadedDate":"2026-03-02T18:00:30Z","processingState":"VALID"}}
		],"links":{}}`)
	})

	since := time.Date(2026, 3, 2, 17, 0, 0, 0, time.UTC)
	_, err := resolveBuildByNumberSelectionSince(
		context.Background(),
		client,
		"123456789",
		"42",
		"",
		"IOS",
		[]asc.BuildsOption{asc.WithBuildsVersion("42")},
		&since,
		false,
	)
	if err == nil {
		t.Fatal("expected an ambiguity error for two matching builds")
	}
	message := err.Error()
	for _, want := range []string{"pass --build-id with one of:", "build-ios", "build-macos"} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected %q in %q", want, message)
		}
	}
	if strings.Contains(message, "later pages") {
		t.Fatalf("no later pages exist for this response: %q", message)
	}
}
