package shared

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
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type buildUploadsRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn buildUploadsRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func newBuildUploadsTestClient(t *testing.T, transport buildUploadsRoundTripFunc) *asc.Client {
	t.Helper()

	keyPath := filepath.Join(t.TempDir(), "key.p8")
	writeECDSAPEM(t, keyPath)

	httpClient := &http.Client{Transport: transport}
	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, httpClient)
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient() error: %v", err)
	}
	return client
}

func newBuildUploadsServerTestClient(t *testing.T, server *httptest.Server) *asc.Client {
	t.Helper()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse() error: %v", err)
	}

	httpClient := server.Client()
	serverTransport := httpClient.Transport
	httpClient.Transport = buildUploadsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		routedReq := req.Clone(req.Context())
		routedURL := *req.URL
		routedURL.Scheme = serverURL.Scheme
		routedURL.Host = serverURL.Host
		routedReq.URL = &routedURL
		routedReq.Host = serverURL.Host
		return serverTransport.RoundTrip(routedReq)
	})

	keyPath := filepath.Join(t.TempDir(), "key.p8")
	writeECDSAPEM(t, keyPath)

	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, httpClient)
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient() error: %v", err)
	}
	return client
}

func buildUploadsJSONStatusResponse(statusCode int, body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func TestPrepareBuildUploadCreatesUploadAndFileReservation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.ipa")
	if err := os.WriteFile(path, []byte("payload"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	requestCount := 0
	client := newBuildUploadsTestClient(t, func(req *http.Request) (*http.Response, error) {
		requestCount++
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("ReadAll() error: %v", err)
		}

		switch requestCount {
		case 1:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/buildUploads" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := string(bodyBytes)
			if !strings.Contains(body, `"cfBundleShortVersionString":"1.2.3"`) || !strings.Contains(body, `"cfBundleVersion":"42"`) || !strings.Contains(body, `"platform":"IOS"`) {
				t.Fatalf("unexpected upload request body: %s", body)
			}
			if !strings.Contains(body, `"id":"app-123"`) {
				t.Fatalf("expected app relationship in upload request body: %s", body)
			}
			return buildUploadsJSONStatusResponse(http.StatusCreated, `{"data":{"type":"buildUploads","id":"upload-123"}}`)
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/buildUploadFiles" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := string(bodyBytes)
			if !strings.Contains(body, `"fileName":"app.ipa"`) || !strings.Contains(body, `"uti":"com.apple.ipa"`) {
				t.Fatalf("unexpected file request body: %s", body)
			}
			if !strings.Contains(body, `"id":"upload-123"`) {
				t.Fatalf("expected build upload relationship in file request body: %s", body)
			}
			return buildUploadsJSONStatusResponse(http.StatusCreated, `{"data":{"type":"buildUploadFiles","id":"file-456","attributes":{"fileName":"app.ipa","fileSize":7}}}`)
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	uploadResp, fileResp, err := PrepareBuildUpload(context.Background(), client, "app-123", fileInfo, "1.2.3", "42", asc.PlatformIOS, asc.UTIIPA)
	if err != nil {
		t.Fatalf("PrepareBuildUpload() error: %v", err)
	}
	if uploadResp.Data.ID != "upload-123" {
		t.Fatalf("expected upload ID upload-123, got %q", uploadResp.Data.ID)
	}
	if fileResp.Data.ID != "file-456" {
		t.Fatalf("expected file ID file-456, got %q", fileResp.Data.ID)
	}
}

func TestPrepareBuildUploadRecoversLostFileReservationResponse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.ipa")
	if err := os.WriteFile(path, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name      string
		fileName  string
		recovered bool
	}{
		{name: "matching file", fileName: "app.ipa", recovered: true},
		{name: "different file", fileName: "other.ipa"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := []string{}
			client := newBuildUploadsTestClient(t, func(req *http.Request) (*http.Response, error) {
				call := req.Method + " " + req.URL.Path
				requests = append(requests, call)
				switch call {
				case "POST /v1/buildUploads":
					return buildUploadsJSONStatusResponse(http.StatusCreated, `{"data":{"type":"buildUploads","id":"upload-123"}}`)
				case "POST /v1/buildUploadFiles":
					resp, _ := buildUploadsJSONStatusResponse(http.StatusCreated, "")
					resp.Body = io.NopCloser(iotest.ErrReader(io.ErrUnexpectedEOF))
					return resp, nil
				case "GET /v1/buildUploads/upload-123/buildUploadFiles":
					return buildUploadsJSONStatusResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"buildUploadFiles","id":"file-456","attributes":{"fileName":%q,"fileSize":7,"uti":"com.apple.ipa","assetType":"ASSET","assetDeliveryState":{"state":"AWAITING_UPLOAD"},"uploadOperations":[{"method":"PUT","url":"https://upload.example/reserved","length":7,"offset":0}]}}],"links":{}}`, tc.fileName))
				default:
					return nil, fmt.Errorf("unexpected request: %s", call)
				}
			})

			upload, file, err := PrepareBuildUpload(context.Background(), client, "app-123", fileInfo, "1.2.3", "42", asc.PlatformIOS, asc.UTIIPA)
			if !tc.recovered {
				if !errors.Is(err, io.ErrUnexpectedEOF) || !strings.Contains(err.Error(), `build upload "upload-123"`) || upload != nil || file != nil {
					t.Fatalf("upload=%v file=%v err=%v, want the original error and upload ID", upload, file, err)
				}
			} else if err != nil || upload == nil || upload.Data.ID != "upload-123" || file == nil || file.Data.ID != "file-456" || len(file.Data.Attributes.UploadOperations) != 1 {
				t.Fatalf("upload=%+v file=%+v err=%v, want original upload and recovered file reservation", upload, file, err)
			}
			want := []string{"POST /v1/buildUploads", "POST /v1/buildUploadFiles", "GET /v1/buildUploads/upload-123/buildUploadFiles"}
			if !slices.Equal(requests, want) {
				t.Fatalf("requests = %v, want %v", requests, want)
			}
		})
	}
}

func TestCommitBuildUploadFileMarksUploadComplete(t *testing.T) {
	client := newBuildUploadsTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/buildUploadFiles/file-456" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("ReadAll() error: %v", err)
		}
		body := string(bodyBytes)
		if !strings.Contains(body, `"uploaded":true`) {
			t.Fatalf("expected uploaded=true in request body: %s", body)
		}
		if !strings.Contains(body, `"hash":"abc123"`) || !strings.Contains(body, `"algorithm":"SHA_256"`) {
			t.Fatalf("expected checksums in request body: %s", body)
		}
		return buildUploadsJSONStatusResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-456","attributes":{"uploaded":true}}}`)
	})

	resp, err := CommitBuildUploadFile(context.Background(), client, "upload-123", "file-456", &asc.Checksums{
		File: &asc.Checksum{
			Hash:      "abc123",
			Algorithm: asc.ChecksumAlgorithmSHA256,
		},
	})
	if err != nil {
		t.Fatalf("CommitBuildUploadFile() error: %v", err)
	}
	if resp == nil || resp.Data.Attributes.Uploaded == nil || !*resp.Data.Attributes.Uploaded {
		t.Fatalf("expected uploaded response, got %#v", resp)
	}
}

func TestCommitBuildUploadFileReconcilesAmbiguousServerFailure(t *testing.T) {
	requestCount := 0
	client := newBuildUploadsTestClient(t, func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodPatch || req.URL.Path != "/v1/buildUploadFiles/file-456" {
				t.Fatalf("unexpected commit request: %s %s", req.Method, req.URL.String())
			}
			return buildUploadsJSONStatusResponse(http.StatusServiceUnavailable, `{"errors":[{"status":"503","code":"SERVICE_UNAVAILABLE","title":"Service unavailable"}]}`)
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/buildUploads/upload-123" {
				t.Fatalf("unexpected reconciliation request: %s %s", req.Method, req.URL.String())
			}
			return buildUploadsJSONStatusResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-123","attributes":{"state":{"state":"PROCESSING"}}}}`)
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	resp, err := CommitBuildUploadFile(context.Background(), client, "upload-123", "file-456", nil)
	if err != nil {
		t.Fatalf("CommitBuildUploadFile() error: %v", err)
	}
	if resp != nil {
		t.Fatalf("expected no synthetic file response after reconciliation, got %#v", resp)
	}
	if requestCount != 2 {
		t.Fatalf("expected commit and reconciliation requests, got %d", requestCount)
	}
}

func TestCommitBuildUploadFileReconcilesSuccessfulResponseDecodeFailure(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty", body: ""},
		{name: "malformed", body: "{"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var patchCount atomic.Int32
			var getCount atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case req.Method == http.MethodPatch && req.URL.Path == "/v1/buildUploadFiles/file-456":
					patchCount.Add(1)
					w.WriteHeader(http.StatusOK)
					if _, err := io.WriteString(w, tt.body); err != nil {
						t.Errorf("WriteString() error: %v", err)
					}
				case req.Method == http.MethodGet && req.URL.Path == "/v1/buildUploads/upload-123":
					getCount.Add(1)
					if _, err := io.WriteString(w, `{"data":{"type":"buildUploads","id":"upload-123","attributes":{"state":{"state":"COMPLETE"}}}}`); err != nil {
						t.Errorf("WriteString() error: %v", err)
					}
				default:
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
					http.Error(w, "unexpected request", http.StatusInternalServerError)
				}
			}))
			defer server.Close()
			client := newBuildUploadsServerTestClient(t, server)

			resp, err := CommitBuildUploadFile(context.Background(), client, "upload-123", "file-456", nil)
			if err != nil {
				t.Fatalf("CommitBuildUploadFile() error: %v", err)
			}
			if resp != nil {
				t.Fatalf("expected no synthetic file response after reconciliation, got %#v", resp)
			}
			if patchCount.Load() != 1 || getCount.Load() != 1 {
				t.Fatalf("expected one commit and one reconciliation request, got patch=%d get=%d", patchCount.Load(), getCount.Load())
			}
		})
	}
}

func TestCommitBuildUploadFileReconcilesSuccessfulResponseReadFailure(t *testing.T) {
	patchCount := 0
	getCount := 0
	client := newBuildUploadsTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/buildUploadFiles/file-456":
			patchCount++
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(iotest.ErrReader(errors.New("response body read failed"))),
			}, nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/buildUploads/upload-123":
			getCount++
			return buildUploadsJSONStatusResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-123","attributes":{"state":{"state":"COMPLETE"}}}}`)
		default:
			err := fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			t.Error(err)
			return nil, err
		}
	})

	resp, err := CommitBuildUploadFile(context.Background(), client, "upload-123", "file-456", nil)
	if err != nil {
		t.Fatalf("CommitBuildUploadFile() error: %v", err)
	}
	if resp != nil {
		t.Fatalf("expected no synthetic file response after reconciliation, got %#v", resp)
	}
	if patchCount != 1 || getCount != 1 {
		t.Fatalf("expected one commit and one reconciliation request, got patch=%d get=%d", patchCount, getCount)
	}
}

func TestCommitBuildUploadFileReconcilesAfterMutationDeadline(t *testing.T) {
	patchCount := 0
	getCount := 0
	client := newBuildUploadsTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/buildUploadFiles/file-456":
			patchCount++
			<-req.Context().Done()
			return nil, req.Context().Err()
		case req.Method == http.MethodGet && req.URL.Path == "/v1/buildUploads/upload-123":
			getCount++
			if err := req.Context().Err(); err != nil {
				t.Fatalf("expected fresh reconciliation context, got %v", err)
			}
			if _, ok := req.Context().Deadline(); !ok {
				t.Fatal("expected reconciliation lookup to have a bounded deadline")
			}
			return buildUploadsJSONStatusResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-123","attributes":{"state":{"state":"COMPLETE"}}}}`)
		default:
			err := fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			t.Error(err)
			return nil, err
		}
	})

	commitCtx, cancel := ContextWithTimeoutDuration(context.Background(), 5*time.Millisecond)
	defer cancel()
	resp, err := CommitBuildUploadFile(commitCtx, client, "upload-123", "file-456", nil)
	if err != nil {
		t.Fatalf("CommitBuildUploadFile() error: %v", err)
	}
	if resp != nil {
		t.Fatalf("expected no synthetic file response after reconciliation, got %#v", resp)
	}
	if patchCount > 1 {
		t.Fatalf("expected at most one commit request, got %d", patchCount)
	}
	if getCount != 1 {
		t.Fatalf("expected one reconciliation lookup, got %d", getCount)
	}
}

func TestCommitBuildUploadFileDoesNotReconcilePastParentDeadline(t *testing.T) {
	patchCount := 0
	getCount := 0
	var parentCtx context.Context
	client := newBuildUploadsTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/buildUploadFiles/file-456":
			patchCount++
			<-req.Context().Done()
			<-parentCtx.Done()
			return nil, req.Context().Err()
		case req.Method == http.MethodGet && req.URL.Path == "/v1/buildUploads/upload-123":
			getCount++
			return buildUploadsJSONStatusResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-123","attributes":{"state":{"state":"COMPLETE"}}}}`)
		default:
			err := fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			t.Error(err)
			return nil, err
		}
	})

	var parentCancel context.CancelFunc
	parentCtx, parentCancel = ContextWithTimeoutDuration(context.Background(), 5*time.Millisecond)
	defer parentCancel()
	commitCtx, commitCancel := ContextWithTimeoutDuration(parentCtx, time.Second)
	defer commitCancel()

	_, err := CommitBuildUploadFile(commitCtx, client, "upload-123", "file-456", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected parent deadline error, got %v", err)
	}
	if patchCount > 1 {
		t.Fatalf("expected at most one commit request, got %d", patchCount)
	}
	if getCount != 0 {
		t.Fatalf("expected no reconciliation after parent deadline, got %d lookups", getCount)
	}
}

func TestCommitBuildUploadFileDoesNotReconcileDefinitiveClientErrors(t *testing.T) {
	tests := []struct {
		status      int
		wantCommits int
	}{
		{status: http.StatusUnprocessableEntity, wantCommits: 1},
		// App Store Connect rejects a rate-limited commit before applying it, so
		// the client replays it. The outcome stays unambiguous either way, so no
		// reconciliation lookup is warranted.
		{status: http.StatusTooManyRequests, wantCommits: 2},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			t.Setenv("ASC_MAX_RETRIES", "1")
			t.Setenv("ASC_BASE_DELAY", "1ms")
			t.Setenv("ASC_MAX_DELAY", "1ms")

			commitCount := 0
			client := newBuildUploadsTestClient(t, func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPatch {
					t.Fatalf("unexpected reconciliation request: %s %s", req.Method, req.URL.String())
				}
				commitCount++
				return buildUploadsJSONStatusResponse(tt.status, fmt.Sprintf(`{"errors":[{"status":"%d","code":"CLIENT_ERROR","title":"Client error"}]}`, tt.status))
			})

			_, err := CommitBuildUploadFile(context.Background(), client, "upload-123", "file-456", nil)
			if err == nil {
				t.Fatal("expected commit error, got nil")
			}
			if commitCount != tt.wantCommits {
				t.Fatalf("expected %d commit attempts and no reconciliation lookup, got %d requests", tt.wantCommits, commitCount)
			}
			if !strings.Contains(err.Error(), `build upload "upload-123"`) || !strings.Contains(err.Error(), `asc builds uploads view --id "upload-123"`) {
				t.Fatalf("expected upload ID and remediation, got %v", err)
			}
			var apiErr *asc.APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != tt.status {
				t.Fatalf("expected original status %d to remain inspectable, got %v", tt.status, err)
			}
		})
	}
}

func TestCommitBuildUploadFileRejectsUnprovenReconciliationStates(t *testing.T) {
	tests := []struct {
		name      string
		stateJSON string
		want      string
	}{
		{name: "awaiting upload", stateJSON: `"AWAITING_UPLOAD"`, want: "AWAITING_UPLOAD"},
		{name: "failed", stateJSON: `"FAILED"`, want: "FAILED"},
		{name: "unknown", stateJSON: `"FUTURE_STATE"`, want: "FUTURE_STATE"},
		{name: "missing", stateJSON: `null`, want: "no authoritative upload state"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestCount := 0
			client := newBuildUploadsTestClient(t, func(req *http.Request) (*http.Response, error) {
				requestCount++
				switch requestCount {
				case 1:
					return buildUploadsJSONStatusResponse(http.StatusServiceUnavailable, `{"errors":[{"status":"503","code":"SERVICE_UNAVAILABLE","title":"Service unavailable"}]}`)
				case 2:
					body := fmt.Sprintf(`{"data":{"type":"buildUploads","id":"upload-123","attributes":{"state":{"state":%s}}}}`, tt.stateJSON)
					return buildUploadsJSONStatusResponse(http.StatusOK, body)
				default:
					t.Fatalf("unexpected request count %d", requestCount)
					return nil, nil
				}
			})

			_, err := CommitBuildUploadFile(context.Background(), client, "upload-123", "file-456", nil)
			if err == nil {
				t.Fatal("expected original commit error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q in error, got %v", tt.want, err)
			}
			var retryableErr *asc.RetryableError
			if !errors.As(err, &retryableErr) {
				t.Fatalf("expected original retryable commit error to remain inspectable, got %v", err)
			}
		})
	}
}

func TestCommitBuildUploadFilePreservesMutationErrorWhenReadbackFails(t *testing.T) {
	requestCount := 0
	client := newBuildUploadsTestClient(t, func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return buildUploadsJSONStatusResponse(http.StatusServiceUnavailable, `{"errors":[{"status":"503","code":"SERVICE_UNAVAILABLE","title":"Service unavailable"}]}`)
		case 2:
			return buildUploadsJSONStatusResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden"}]}`)
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	_, err := CommitBuildUploadFile(context.Background(), client, "upload-123", "file-456", nil)
	if err == nil {
		t.Fatal("expected commit error, got nil")
	}
	if !strings.Contains(err.Error(), "reconciliation lookup failed") || !strings.Contains(err.Error(), "Forbidden") {
		t.Fatalf("expected reconciliation diagnostics, got %v", err)
	}
	var retryableErr *asc.RetryableError
	if !errors.As(err, &retryableErr) {
		t.Fatalf("expected original mutation error to remain inspectable, got %v", err)
	}
}
