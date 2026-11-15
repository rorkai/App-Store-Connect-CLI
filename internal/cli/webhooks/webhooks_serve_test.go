package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestWebhooksServeReceivesAndWritesEvent(t *testing.T) {
	eventsDir := filepath.Join(t.TempDir(), "events")
	port := freeLocalPort(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	cmd, stdout, stderr := newServeTestCommand(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	if err := cmd.Parse([]string{
		"--host", "127.0.0.1",
		"--port", fmt.Sprintf("%d", port),
		"--dir", eventsDir,
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	go func() {
		errCh <- cmd.Run(ctx)
	}()

	statusCode := postJSONWithRetry(t, fmt.Sprintf("http://127.0.0.1:%d", port), `{
		"id":"evt-123",
		"eventType":"BUILD_UPLOAD_STATE_UPDATED",
		"data":{"type":"webhookEvents"}
	}`)
	if statusCode != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, statusCode)
	}

	payload := waitForJSONPayloadFile(t, eventsDir)
	if payload["id"] != "evt-123" {
		t.Fatalf("expected id evt-123, got %v", payload["id"])
	}
	if payload["eventType"] != "BUILD_UPLOAD_STATE_UPDATED" {
		t.Fatalf("expected eventType BUILD_UPLOAD_STATE_UPDATED, got %v", payload["eventType"])
	}
	shutdownServeCommand(t, cancel, errCh)

	startup, err := json.Marshal(webhookServeStartup{
		URL:          fmt.Sprintf("http://127.0.0.1:%d", port),
		Host:         "127.0.0.1",
		Port:         port,
		Dir:          eventsDir,
		MaxBodyBytes: webhooksServeDefaultMaxBodyBytes,
	})
	if err != nil {
		t.Fatalf("marshal startup: %v", err)
	}
	if got, want := stdout.String(), string(startup)+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	compact := `{"id":"evt-123","eventType":"BUILD_UPLOAD_STATE_UPDATED","data":{"type":"webhookEvents"}}`
	wantStderr := fmt.Sprintf("webhooks serve: received event type=BUILD_UPLOAD_STATE_UPDATED id=evt-123 bytes=%d\n", len(compact)) +
		"webhooks serve: wrote event payload to " + filepath.Join(eventsDir, "20260102T030405.000000000Z-000001-build_upload_state_updated.json") + "\n"
	if got := stderr.String(); got != wantStderr {
		t.Fatalf("stderr = %q, want %q", got, wantStderr)
	}
}

func TestWebhooksServeExecReceivesPayload(t *testing.T) {
	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "payload.json")
	port := freeLocalPort(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	cmd, stdout, stderr := newServeTestCommand(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	if err := cmd.Parse([]string{
		"--host", "127.0.0.1",
		"--port", fmt.Sprintf("%d", port),
		"--exec", fmt.Sprintf("cat > %q", outPath),
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	go func() {
		errCh <- cmd.Run(ctx)
	}()

	payload := `{"id":"evt-exec-1","eventType":"TEST_EVENT"}`
	statusCode := postJSONWithRetry(t, fmt.Sprintf("http://127.0.0.1:%d", port), payload)
	if statusCode != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, statusCode)
	}

	waitForFileContains(t, outPath, `"id":"evt-exec-1"`)
	shutdownServeCommand(t, cancel, errCh)

	if got, want := stdout.String(), fmt.Sprintf("Listening for webhook events on http://127.0.0.1:%d\n", port); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if got, want := stderr.String(), fmt.Sprintf("webhooks serve: received event type=TEST_EVENT id=evt-exec-1 bytes=%d\n", len(payload)); got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestWebhooksServeRejectsNonLoopbackWithoutAllowRemote(t *testing.T) {
	cmd, stdout, stderr := newServeTestCommand(time.Time{})
	if err := cmd.Parse([]string{
		"--host", "0.0.0.0",
		"--port", "0",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := cmd.Run(ctx)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp for non-loopback bind without --allow-remote, got %v", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
	if got, want := stderr.String(), "Error: binding to non-loopback host \"0.0.0.0\" requires --allow-remote\n"+cmd.UsageFunc(cmd)+"\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestWebhooksServeAllowsNonLoopbackWithAllowRemote(t *testing.T) {
	port := freeLocalPort(t)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	cmd, _, _ := newServeTestCommand(time.Time{})
	if err := cmd.Parse([]string{
		"--host", "0.0.0.0",
		"--port", fmt.Sprintf("%d", port),
		"--allow-remote",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	go func() {
		errCh <- cmd.Run(ctx)
	}()
	defer shutdownServeCommand(t, cancel, errCh)

	statusCode := postJSONWithRetry(t, fmt.Sprintf("http://127.0.0.1:%d", port), `{"id":"evt-remote-1","eventType":"REMOTE_EVENT"}`)
	if statusCode != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, statusCode)
	}
}

func TestWebhookServeRuntimeStopsWorkersImmediatelyAfterStart(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousProcs) })

	serveRuntime := &webhookServeRuntime{
		eventQueue:  make(chan webhookServeEvent),
		workerCount: webhooksServeDefaultWorkerCount,
	}
	serveRuntime.startWorkers(context.Background())
	serveRuntime.stopWorkers()
}

func TestReadWebhookServeJSONPayloadAllowsMaxInt64Limit(t *testing.T) {
	payload, err := readWebhookServeJSONPayload(
		io.NopCloser(strings.NewReader(`{"id":"evt-max-int","eventType":"TEST_EVENT"}`)),
		math.MaxInt64,
	)
	if err != nil {
		t.Fatalf("expected payload to be accepted with max int64 limit, got error: %v", err)
	}
	if got, want := string(payload), `{"id":"evt-max-int","eventType":"TEST_EVENT"}`; got != want {
		t.Fatalf("expected compact payload %q, got %q", want, got)
	}
}

func TestReadWebhookServeJSONPayloadAllowsExactLimit(t *testing.T) {
	const rawPayload = `{"id":"evt-exact"}`
	payload, err := readWebhookServeJSONPayload(io.NopCloser(strings.NewReader(rawPayload)), int64(len(rawPayload)))
	if err != nil {
		t.Fatalf("expected exact-limit payload to be accepted, got %v", err)
	}
	if string(payload) != rawPayload {
		t.Fatalf("expected compact payload %q, got %q", rawPayload, string(payload))
	}
}

func TestReadWebhookServeJSONPayloadRejectsOneByteOverLimit(t *testing.T) {
	const rawPayload = `{"id":"evt-over"}`
	_, err := readWebhookServeJSONPayload(io.NopCloser(strings.NewReader(rawPayload)), int64(len(rawPayload)-1))
	if !errors.Is(err, errWebhookPayloadTooLarge) {
		t.Fatalf("expected payload-too-large error, got %v", err)
	}
}

func TestWebhooksServeHandlerRejectsNonPOST(t *testing.T) {
	runtime := &webhookServeRuntime{maxBodyBytes: webhooksServeDefaultMaxBodyBytes}
	handler := runtime.newHandler()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON response body: %v", err)
	}
	if payload["error"] != "method not allowed" {
		t.Fatalf("expected method-not-allowed response, got %v", payload)
	}
}

func TestWebhooksServeHandlerRejectsInvalidJSON(t *testing.T) {
	runtime := &webhookServeRuntime{maxBodyBytes: webhooksServeDefaultMaxBodyBytes}
	handler := runtime.newHandler()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{invalid"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON response body: %v", err)
	}
	if payload["error"] != "invalid JSON payload" {
		t.Fatalf("expected invalid-json response, got %v", payload)
	}
}

func TestWebhooksServeHandlerRejectsLargePayload(t *testing.T) {
	runtime := &webhookServeRuntime{maxBodyBytes: 8}
	handler := runtime.newHandler()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"id":"evt-oversized"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON response body: %v", err)
	}
	if payload["error"] != "payload too large" {
		t.Fatalf("expected payload-too-large response, got %v", payload)
	}
}

func TestPrepareWebhookServeDirectoryRejectsFilePath(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := prepareWebhookServeDirectory(filePath)
	if err == nil {
		t.Fatal("expected error for non-directory path")
	}
	if !strings.Contains(err.Error(), "--dir must point to a directory") {
		t.Fatalf("expected directory validation error, got %v", err)
	}
}

func TestPrepareWebhookServeDirectoryRejectsSymlinkPath(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "target")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}

	symlinkPath := filepath.Join(tempDir, "events-link")
	if err := os.Symlink(targetDir, symlinkPath); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not supported") {
			t.Skipf("symlink not supported: %v", err)
		}
		t.Fatalf("create symlink: %v", err)
	}

	_, err := prepareWebhookServeDirectory(symlinkPath)
	if err == nil {
		t.Fatal("expected symlink path error")
	}
	if !strings.Contains(err.Error(), "refusing to use symlink directory") {
		t.Fatalf("expected symlink rejection error, got %v", err)
	}
}

func TestExtractWebhookServeEventMetadataHeaderPrecedence(t *testing.T) {
	header := make(http.Header)
	header.Set("X-Apple-Event-Type", "HEADER_EVENT")
	header.Set("X-Request-ID", "header-1")

	eventType, eventID := extractWebhookServeEventMetadata(header, []byte(`{
		"id":"payload-1",
		"eventType":"PAYLOAD_EVENT"
	}`))
	if eventType != "HEADER_EVENT" {
		t.Fatalf("expected header event type, got %q", eventType)
	}
	if eventID != "header-1" {
		t.Fatalf("expected header event id, got %q", eventID)
	}
}

func TestExtractWebhookServeEventMetadataPayloadFallback(t *testing.T) {
	eventType, eventID := extractWebhookServeEventMetadata(http.Header{}, []byte(`{
		"data": {"id":"nested-1","type":"NESTED_EVENT"}
	}`))
	if eventType != "NESTED_EVENT" {
		t.Fatalf("expected payload fallback event type, got %q", eventType)
	}
	if eventID != "nested-1" {
		t.Fatalf("expected payload fallback event id, got %q", eventID)
	}
}

func TestSanitizeWebhookServeFilenameSegment(t *testing.T) {
	got := sanitizeWebhookServeFilenameSegment("  BUILD.UPLOAD/STATE*UPDATED  ")
	if got != "build_upload_state_updated" {
		t.Fatalf("unexpected sanitized filename segment: %q", got)
	}

	longValue := sanitizeWebhookServeFilenameSegment(strings.Repeat("x", 120))
	if len(longValue) > 48 {
		t.Fatalf("expected max filename segment length 48, got %d", len(longValue))
	}
}

func TestRunWebhookExecCommandReturnsStderrOnFailure(t *testing.T) {
	err := runWebhookExecCommand(context.Background(), `echo "boom" 1>&2; exit 2`, []byte(`{"id":"evt-1"}`))
	if err == nil {
		t.Fatal("expected exec failure")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected stderr content in error, got %v", err)
	}
}

// lockedBuffer is a bytes.Buffer that the serve goroutines can write to while
// the test reads it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func newServeTestCommand(now time.Time) (*ffcli.Command, *lockedBuffer, *lockedBuffer) {
	stdout, stderr := &lockedBuffer{}, &lockedBuffer{}
	cmd := newWebhooksServeCommand(&shared.Deps{
		IO:  &shared.IO{Out: stdout, ErrOut: stderr},
		Now: func() time.Time { return now },
	})
	return cmd, stdout, stderr
}

func freeLocalPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for free port: %v", err)
	}
	defer listener.Close()

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("expected TCP addr, got %T", listener.Addr())
	}
	return tcpAddr.Port
}

func postJSONWithRetry(t *testing.T, baseURL string, payload string) int {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	for time.Now().Before(deadline) {
		resp, err := client.Post(baseURL, "application/json", strings.NewReader(payload))
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.StatusCode
	}

	t.Fatalf("timed out posting webhook payload to %s", baseURL)
	return 0
}

func waitForJSONPayloadFile(t *testing.T, dir string) map[string]any {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				if strings.HasSuffix(entry.Name(), ".json") {
					data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
					if readErr != nil {
						continue
					}

					var payload map[string]any
					if err := json.Unmarshal(data, &payload); err == nil {
						return payload
					}
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for a valid JSON payload file in %q", dir)
	return nil
}

func waitForFileContains(t *testing.T, path, substring string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), substring) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("timed out waiting for file %q to contain %q: %v", path, substring, err)
	}
	t.Fatalf("timed out waiting for file %q to contain %q, got %q", path, substring, string(data))
}

func shutdownServeCommand(t *testing.T, cancel context.CancelFunc, errCh <-chan error) {
	t.Helper()

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("serve command returned error on shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for serve command shutdown")
	}
}
