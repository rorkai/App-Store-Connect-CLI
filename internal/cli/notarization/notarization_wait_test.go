package notarization

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const testNotarizationSubmissionID = "550e8400-e29b-41d4-a716-446655440000"

func TestWaitForNotarizationRetriesTransientStatusFailures(t *testing.T) {
	var mu sync.Mutex
	attempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/notary/v2/submissions/"+testNotarizationSubmissionID {
			t.Errorf("status request path = %q, want /notary/v2/submissions/%s", req.URL.Path, testNotarizationSubmissionID)
		}

		mu.Lock()
		attempts++
		attempt := attempts
		mu.Unlock()

		switch attempt {
		case 1:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"errors":[{"code":"SERVICE_UNAVAILABLE","title":"Service Unavailable","detail":"try again"}]}`)
		case 2:
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, "slow down")
		case 3:
			writeNotaryStatus(t, w, asc.NotaryStatusInProgress)
		default:
			writeNotaryStatus(t, w, asc.NotaryStatusAccepted)
		}
	}))
	t.Cleanup(server.Close)

	client := newNotarizationTestClient(t, server)

	var resp *asc.NotarySubmissionStatusResponse
	var waitErr error
	stderr := captureNotarizationStderr(t, func() {
		resp, waitErr = waitForNotarization(context.Background(), client, testNotarizationSubmissionID, 5*time.Millisecond)
	})

	if waitErr != nil {
		t.Fatalf("waitForNotarization() error = %v, want transient failures to be retried", waitErr)
	}
	if resp == nil || resp.Data.Attributes.Status != asc.NotaryStatusAccepted {
		t.Fatalf("waitForNotarization() response = %+v, want Accepted status", resp)
	}

	mu.Lock()
	got := attempts
	mu.Unlock()
	if got != 4 {
		t.Fatalf("status requests = %d, want 4", got)
	}
	if !strings.Contains(stderr, "notarization status check failed") {
		t.Fatalf("stderr = %q, want a transient poll failure warning", stderr)
	}
}

func TestWaitForNotarizationFailsFastOnTerminalStatusError(t *testing.T) {
	var mu sync.Mutex
	attempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		attempts++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"errors":[{"code":"UNAUTHORIZED","title":"Unauthorized","detail":"invalid credentials"}]}`)
	}))
	t.Cleanup(server.Close)

	client := newNotarizationTestClient(t, server)

	var waitErr error
	captureNotarizationStderr(t, func() {
		_, waitErr = waitForNotarization(context.Background(), client, testNotarizationSubmissionID, 5*time.Millisecond)
	})

	if waitErr == nil {
		t.Fatal("waitForNotarization() error = nil, want terminal authentication failure")
	}
	if !strings.Contains(waitErr.Error(), "failed to check status") {
		t.Fatalf("waitForNotarization() error = %v, want failed-to-check-status wrapping", waitErr)
	}

	mu.Lock()
	got := attempts
	mu.Unlock()
	if got != 1 {
		t.Fatalf("status requests = %d, want 1 (terminal errors must not be retried)", got)
	}
}

func TestWaitForNotarizationReportsWaitDeadlineDuringInFlightStatusRequest(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		select {
		case <-release:
		case <-req.Context().Done():
		}
		writeNotaryStatus(t, w, asc.NotaryStatusAccepted)
	}))
	t.Cleanup(func() {
		close(release)
		server.Close()
	})

	client := newNotarizationTestClient(t, server)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	var waitErr error
	captureNotarizationStderr(t, func() {
		_, waitErr = waitForNotarization(ctx, client, testNotarizationSubmissionID, 5*time.Millisecond)
	})

	if waitErr == nil {
		t.Fatal("waitForNotarization() error = nil, want wait deadline failure")
	}
	if !strings.Contains(waitErr.Error(), "timed out waiting for notarization") {
		t.Fatalf("waitForNotarization() error = %v, want timed-out-waiting message", waitErr)
	}
	if strings.Contains(waitErr.Error(), "failed to check status") {
		t.Fatalf("waitForNotarization() error = %v, must not report the wait deadline as a status-check failure", waitErr)
	}
}

func TestWaitForNotarizationReportsLastTransientFailureAtDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, `{"errors":[{"code":"BAD_GATEWAY","title":"Bad Gateway","detail":"notary is unavailable"}]}`)
	}))
	t.Cleanup(server.Close)

	client := newNotarizationTestClient(t, server)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()

	var waitErr error
	captureNotarizationStderr(t, func() {
		_, waitErr = waitForNotarization(ctx, client, testNotarizationSubmissionID, 5*time.Millisecond)
	})

	if waitErr == nil {
		t.Fatal("waitForNotarization() error = nil, want wait deadline failure")
	}
	if !strings.Contains(waitErr.Error(), "timed out waiting for notarization") {
		t.Fatalf("waitForNotarization() error = %v, want timed-out-waiting message", waitErr)
	}
	if !strings.Contains(waitErr.Error(), "notary is unavailable") {
		t.Fatalf("waitForNotarization() error = %v, want the last transient failure preserved", waitErr)
	}
}

func TestWaitForNotarizationReportsCancellationSeparatelyFromTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeNotaryStatus(t, w, asc.NotaryStatusInProgress)
	}))
	t.Cleanup(server.Close)

	client := newNotarizationTestClient(t, server)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	defer cancel()

	var waitErr error
	captureNotarizationStderr(t, func() {
		_, waitErr = waitForNotarization(ctx, client, testNotarizationSubmissionID, 5*time.Millisecond)
	})

	if waitErr == nil {
		t.Fatal("waitForNotarization() error = nil, want cancellation failure")
	}
	if !strings.Contains(waitErr.Error(), "canceled while waiting for notarization") {
		t.Fatalf("waitForNotarization() error = %v, want cancellation message", waitErr)
	}
	if strings.Contains(waitErr.Error(), "timed out") {
		t.Fatalf("waitForNotarization() error = %v, cancellation must not be reported as a timeout", waitErr)
	}
}

func writeNotaryStatus(t *testing.T, w http.ResponseWriter, status asc.NotarySubmissionStatus) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(asc.NotarySubmissionStatusResponse{
		Data: asc.NotarySubmissionStatusData{
			ID:   testNotarizationSubmissionID,
			Type: "submissions",
			Attributes: asc.NotarySubmissionStatusAttributes{
				Status: status,
				Name:   "Demo.zip",
			},
		},
	}); err != nil {
		t.Errorf("encode notary status response: %v", err)
	}
}

func captureNotarizationStderr(t *testing.T, fn func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	original := os.Stderr
	os.Stderr = writer

	captured := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, reader)
		_ = reader.Close()
		captured <- buf.String()
	}()

	defer func() {
		os.Stderr = original
		_ = writer.Close()
	}()

	fn()

	os.Stderr = original
	_ = writer.Close()
	return <-captured
}
