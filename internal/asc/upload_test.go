package asc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
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

	err := ExecuteUploadOperations(
		context.Background(), filePath, ops,
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

func TestExecuteUploadOperations_DefaultConcurrencyUploadsChunksInParallel(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "app.ipa")
	content := []byte("abcdefgh")
	if err := os.WriteFile(filePath, content, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	arrived := make(chan struct{}, DefaultUploadConcurrency)
	allArrived := make(chan struct{})
	go func() {
		for i := 0; i < DefaultUploadConcurrency; i++ {
			<-arrived
		}
		close(allArrived)
	}()

	var mu sync.Mutex
	received := map[string]string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		arrived <- struct{}{}
		// Block until every chunk is in flight simultaneously. This only
		// completes when the default concurrency dispatches all chunks in
		// parallel; a serial default would deadlock and hit the timeout.
		select {
		case <-allArrived:
		case <-time.After(5 * time.Second):
			t.Error("timed out waiting for concurrent chunk uploads; default concurrency appears serial")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		mu.Lock()
		received[r.URL.Path] = string(body)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ops := make([]UploadOperation, 0, DefaultUploadConcurrency)
	for i := 0; i < DefaultUploadConcurrency; i++ {
		ops = append(ops, UploadOperation{
			Method: "PUT",
			URL:    fmt.Sprintf("%s/op%d", server.URL, i),
			Length: 2,
			Offset: int64(i * 2),
		})
	}

	// No WithUploadConcurrency option: rely on the default.
	err := ExecuteUploadOperations(
		context.Background(), filePath, ops,
		WithUploadHTTPClient(server.Client()),
		withUploadRetryOptions(0),
	)
	if err != nil {
		t.Fatalf("ExecuteUploadOperations() error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for i := 0; i < DefaultUploadConcurrency; i++ {
		path := fmt.Sprintf("/op%d", i)
		want := string(content[i*2 : i*2+2])
		if received[path] != want {
			t.Fatalf("expected %s body=%q, got %q", path, want, received[path])
		}
	}
}

func TestExecuteUploadOperations_ConcurrentUploadFailsOnErrorMidBatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "app.ipa")
	if err := os.WriteFile(filePath, []byte("abcdefghijkl"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		_, _ = io.Copy(io.Discard, r.Body)
		if r.URL.Path == "/op3" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ops := make([]UploadOperation, 0, 6)
	for i := 0; i < 6; i++ {
		ops = append(ops, UploadOperation{
			Method: "PUT",
			URL:    fmt.Sprintf("%s/op%d", server.URL, i),
			Length: 2,
			Offset: int64(i * 2),
		})
	}

	err := ExecuteUploadOperations(
		context.Background(), filePath, ops,
		WithUploadHTTPClient(server.Client()),
		withUploadRetryOptions(0),
	)
	if err == nil {
		t.Fatal("expected mid-batch upload failure to surface an error")
	}
	if !strings.Contains(err.Error(), "upload operation 3") {
		t.Fatalf("expected error to identify failing operation, got %v", err)
	}
	if !strings.Contains(err.Error(), "403") {
		t.Fatalf("expected error to include HTTP status, got %v", err)
	}
}

func TestNewUploadClient_RaisesMaxIdleConnsPerHost(t *testing.T) {
	client := newUploadClient()
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", client.Transport)
	}
	if transport.MaxIdleConnsPerHost < uploadMaxIdleConnsPerHost {
		t.Fatalf("expected MaxIdleConnsPerHost >= %d, got %d", uploadMaxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
	}
	if base, ok := http.DefaultTransport.(*http.Transport); ok && transport == base {
		t.Fatal("expected upload client transport to be a clone, not http.DefaultTransport")
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

	err := ExecuteUploadOperations(context.Background(), filePath, ops, WithUploadConcurrency(1), withUploadRetryOptions(0))
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

func TestExecuteUploadOperations_PreCanceledContextDoesNotSucceed(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "app.ipa")
	if err := os.WriteFile(filePath, []byte("abc"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var requests int32
	client := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		atomic.AddInt32(&requests, 1)
		return nil, errors.New("unexpected upload request")
	})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := ExecuteUploadOperations(ctx, filePath, []UploadOperation{{
		Method: http.MethodPut,
		URL:    "https://example.test/upload",
		Length: 3,
	}}, WithUploadHTTPClient(client))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if got := atomic.LoadInt32(&requests); got != 0 {
		t.Fatalf("expected no upload requests, got %d", got)
	}
}

func TestExecuteUploadOperations_SucceedsWhenParentCanceledAfterFinalPUT(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "app.ipa")
	if err := os.WriteFile(filePath, []byte("abc"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	client := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       &cancelOnCloseReadCloser{cancel: cancel},
			Header:     make(http.Header),
		}, nil
	})}

	err := ExecuteUploadOperations(ctx, filePath, []UploadOperation{{
		Method: http.MethodPut,
		URL:    "https://example.test/upload",
		Length: 3,
	}}, WithUploadHTTPClient(client), withUploadRetryOptions(0))
	if err != nil {
		t.Fatalf("expected completed upload to succeed, got %v", err)
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("expected transport cleanup to cancel parent, got %v", ctx.Err())
	}
}

func TestExecuteUploadOperations_UsesFreshTimeoutForEachOperation(t *testing.T) {
	t.Setenv("ASC_UPLOAD_TIMEOUT", "150ms")
	t.Setenv("ASC_MAX_RETRIES", "0")
	dir := t.TempDir()
	filePath := filepath.Join(dir, "app.ipa")
	if err := os.WriteFile(filePath, []byte("abcdef"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var requests int32
	client := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		request := atomic.AddInt32(&requests, 1)
		deadline, ok := req.Context().Deadline()
		if !ok {
			return nil, errors.New("upload request has no deadline")
		}
		if remaining := time.Until(deadline); remaining < 100*time.Millisecond {
			return nil, fmt.Errorf("upload request %d inherited stale deadline: %s", request, remaining)
		}
		if request == 1 {
			select {
			case <-time.After(75 * time.Millisecond):
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})}

	var err error
	synctest.Test(t, func(*testing.T) {
		err = ExecuteUploadOperations(context.Background(), filePath, []UploadOperation{
			{Method: http.MethodPut, URL: "https://example.test/part-1", Length: 3, Offset: 0},
			{Method: http.MethodPut, URL: "https://example.test/part-2", Length: 3, Offset: 3},
		}, WithUploadConcurrency(1), WithUploadHTTPClient(client), withUploadRetryOptions(0))
	})
	if err != nil {
		t.Fatalf("ExecuteUploadOperations() error: %v", err)
	}
	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Fatalf("expected two upload requests, got %d", got)
	}
}

func TestExecuteUploadOperations_RetriesPUTAfterAttemptTimeout(t *testing.T) {
	t.Setenv("ASC_UPLOAD_TIMEOUT", "20ms")
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
	dir := t.TempDir()
	filePath := filepath.Join(dir, "app.ipa")
	if err := os.WriteFile(filePath, []byte("abc"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var requests int32
	client := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if atomic.AddInt32(&requests, 1) == 1 {
			<-req.Context().Done()
			return nil, req.Context().Err()
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})}

	err := ExecuteUploadOperations(context.Background(), filePath, []UploadOperation{{
		Method: http.MethodPut,
		URL:    "https://example.test/upload",
		Length: 3,
	}}, WithUploadHTTPClient(client), withUploadRetryOptions(1))
	if err != nil {
		t.Fatalf("ExecuteUploadOperations() error: %v", err)
	}
	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Fatalf("expected timed-out PUT to retry once, got %d requests", got)
	}
}

func TestExecuteUploadOperations_RetriesPUTWhenServerClosedIdleConnection(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "app.ipa")
	if err := os.WriteFile(filePath, []byte("abc"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	requests := 0
	client := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			// net/http returns this unexported error when a reused keep-alive
			// connection closes while a request with a body is being sent.
			return nil, errors.New("http: server closed idle connection")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})}

	err := ExecuteUploadOperations(context.Background(), filePath, []UploadOperation{{
		Method: http.MethodPut,
		URL:    "https://example.test/upload",
		Length: 3,
	}}, WithUploadHTTPClient(client), withUploadRetryOptions(1))
	if err != nil {
		t.Fatalf("ExecuteUploadOperations() error: %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected one retry after the closed idle connection, got %d requests", requests)
	}
}

func TestExecuteUploadOperations_RetriesPUTTransientStatuses(t *testing.T) {
	statuses := []int{
		http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	}
	for _, status := range statuses {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Setenv("ASC_MAX_RETRIES", "1")
			t.Setenv("ASC_BASE_DELAY", "1ms")
			t.Setenv("ASC_MAX_DELAY", "1ms")
			dir := t.TempDir()
			filePath := filepath.Join(dir, "app.ipa")
			if err := os.WriteFile(filePath, []byte("abc"), 0o600); err != nil {
				t.Fatalf("write file: %v", err)
			}

			requests := 0
			client := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				requests++
				responseStatus := status
				if requests == 2 {
					responseStatus = http.StatusOK
				}
				return &http.Response{
					StatusCode: responseStatus,
					Status:     fmt.Sprintf("%d %s", responseStatus, http.StatusText(responseStatus)),
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
				}, nil
			})}

			err := ExecuteUploadOperations(context.Background(), filePath, []UploadOperation{{
				Method: http.MethodPut,
				URL:    "https://example.test/upload",
				Length: 3,
			}}, WithUploadHTTPClient(client), withUploadRetryOptions(1))
			if err != nil {
				t.Fatalf("ExecuteUploadOperations() error: %v", err)
			}
			if requests != 2 {
				t.Fatalf("expected one retry for status %d, got %d requests", status, requests)
			}
		})
	}
}

func TestExecuteUploadOperations_DoesNotReplayNonPUT(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "3")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
	dir := t.TempDir()
	filePath := filepath.Join(dir, "app.ipa")
	if err := os.WriteFile(filePath, []byte("abc"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	requests := 0
	client := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Status:     "503 Service Unavailable",
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})}

	err := ExecuteUploadOperations(context.Background(), filePath, []UploadOperation{{
		Method: http.MethodPost,
		URL:    "https://example.test/upload",
		Length: 3,
	}}, WithUploadHTTPClient(client))
	if err == nil {
		t.Fatal("expected non-PUT upload error")
	}
	if requests != 1 {
		t.Fatalf("expected non-PUT operation not to replay, got %d requests", requests)
	}
}

func TestExecuteUploadOperations_RedactsSignedURLInRequestConstructionError(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "app.ipa")
	if err := os.WriteFile(filePath, []byte("abc"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var requests int32
	client := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		atomic.AddInt32(&requests, 1)
		return nil, errors.New("unexpected upload request")
	})}
	err := ExecuteUploadOperations(context.Background(), filePath, []UploadOperation{{
		Method: http.MethodPut,
		URL:    "https://upload.example.test/object%zz?X-Amz-Signature=secret-signature&uploadId=secret-upload-id",
		Length: 3,
	}}, WithUploadHTTPClient(client), withUploadRetryOptions(0))
	if err == nil {
		t.Fatal("expected request construction error")
	}
	if got := atomic.LoadInt32(&requests); got != 0 {
		t.Fatalf("expected no transport call, got %d", got)
	}
	assertUploadErrorRedactsSignedURL(t, err)
}

func TestExecuteUploadOperations_RedactsSignedURLInTransportErrors(t *testing.T) {
	sentinel := errors.New("permanent transport failure for X-Amz-Signature=secret-signature&uploadId=secret-upload-id")
	tests := []struct {
		name         string
		transportErr error
		maxRetries   int
		wantAttempts int32
		wantCause    error
	}{
		{
			name:         "transient retry exhaustion",
			transportErr: context.DeadlineExceeded,
			maxRetries:   1,
			wantAttempts: 2,
			wantCause:    context.DeadlineExceeded,
		},
		{
			name:         "permanent transport error",
			transportErr: sentinel,
			maxRetries:   3,
			wantAttempts: 1,
			wantCause:    sentinel,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filePath := filepath.Join(t.TempDir(), "app.ipa")
			if err := os.WriteFile(filePath, []byte("abc"), 0o600); err != nil {
				t.Fatalf("write file: %v", err)
			}

			var attempts int32
			client := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				atomic.AddInt32(&attempts, 1)
				return nil, test.transportErr
			})}
			err := ExecuteUploadOperations(context.Background(), filePath, []UploadOperation{{
				Method: http.MethodPut,
				URL:    "https://upload.example.test/object/path?X-Amz-Credential=secret-credential&X-Amz-Signature=secret-signature&uploadId=secret-upload-id&correlationKey=secret-correlation-key",
				Length: 3,
			}}, WithUploadHTTPClient(client), withUploadRetryOptions(test.maxRetries))
			if err == nil {
				t.Fatal("expected transport error")
			}
			if !errors.Is(err, test.wantCause) {
				t.Fatalf("expected error to preserve %v, got %v", test.wantCause, err)
			}
			var urlErr *url.Error
			if !errors.As(err, &urlErr) {
				t.Fatalf("expected error to preserve url.Error, got %T: %v", err, err)
			}
			if got := atomic.LoadInt32(&attempts); got != test.wantAttempts {
				t.Fatalf("expected %d attempts, got %d", test.wantAttempts, got)
			}
			assertUploadErrorRedactsSignedURL(t, err)
			if !strings.Contains(err.Error(), "https://upload.example.test/object/path") {
				t.Fatalf("expected safe upload origin/path in error, got %v", err)
			}
		})
	}
}

func assertUploadErrorRedactsSignedURL(t *testing.T, err error) {
	t.Helper()
	for _, secret := range []string{
		"X-Amz-Credential",
		"X-Amz-Signature",
		"secret-credential",
		"secret-signature",
		"uploadId",
		"secret-upload-id",
		"correlationKey",
		"secret-correlation-key",
	} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("upload error leaked %q: %v", secret, err)
		}
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
		done <- ExecuteUploadOperations(
			ctx, filePath, ops,
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

type cancelOnCloseReadCloser struct {
	cancel context.CancelFunc
}

func (*cancelOnCloseReadCloser) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (r *cancelOnCloseReadCloser) Close() error {
	r.cancel()
	return nil
}

func withUploadRetryOptions(maxRetries int) UploadOption {
	return func(opts *UploadOptions) {
		opts.RetryOpts = RetryOptions{
			MaxRetries: maxRetries,
			BaseDelay:  time.Millisecond,
			MaxDelay:   time.Millisecond,
		}
	}
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

func TestVerifySourceFileChecksumsSameAlgorithmComposite(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "checksum.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	const helloMD5 = "5d41402abc4b2a76b9719d911017c592"

	tests := []struct {
		name          string
		compositeHash string
		wantErr       string
	}{
		{name: "match", compositeHash: helloMD5},
		{name: "composite mismatch", compositeHash: "00000000000000000000000000000000", wantErr: "composite checksum mismatch"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			computed, err := VerifySourceFileChecksums(filePath, &Checksums{
				File:      &Checksum{Hash: helloMD5, Algorithm: ChecksumAlgorithmMD5},
				Composite: &Checksum{Hash: test.compositeHash, Algorithm: ChecksumAlgorithmMD5},
			})
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("expected error containing %q, got %v", test.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("VerifySourceFileChecksums() error: %v", err)
			}
			if computed.Composite == nil || computed.Composite.Hash != helloMD5 {
				t.Fatalf("expected composite hash %s, got %#v", helloMD5, computed.Composite)
			}
		})
	}
}
