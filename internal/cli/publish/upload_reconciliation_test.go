package publish

import (
	"context"
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

func TestUploadBuildAndWaitForIDRecoversAmbiguousCommit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Demo.ipa")
	if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
		t.Fatalf("write IPA: %v", err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat IPA: %v", err)
	}

	buildUploadLookups := 0
	swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploads":
			return publishCommandJSONResponse(http.StatusCreated, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.2.3","cfBundleVersion":"42","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploadFiles":
			return publishCommandJSONResponse(http.StatusCreated, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"Demo.ipa","fileSize":4,"uti":"com.apple.ipa","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part-1","length":4,"offset":0}]}}}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/buildUploadFiles/file-1":
			return publishCommandJSONResponse(http.StatusServiceUnavailable, `{"errors":[{"status":"503","code":"SERVICE_UNAVAILABLE","title":"Service unavailable"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/buildUploads/upload-1":
			buildUploadLookups++
			if buildUploadLookups == 1 {
				return publishCommandJSONResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"state":{"state":"PROCESSING"}}}}`)
			}
			return publishCommandJSONResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"state":{"state":"COMPLETE"}},"relationships":{"build":{"data":{"type":"builds","id":"build-1"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1":
			return publishCommandJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"PROCESSING"}}}`)
		default:
			err := fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			t.Error(err)
			return nil, err
		}
	}))
	client := newPublishCommandTestClient(t)

	var result *publishUploadResult
	var uploadErr error
	stdout, stderr := capturePublishCommandOutput(t, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		result, uploadErr = uploadBuildAndWaitForID(ctx, client, "app-1", path, fileInfo, "1.2.3", "42", asc.PlatformIOS, time.Millisecond, time.Second, true)
		return uploadErr
	})

	if uploadErr != nil {
		t.Fatalf("uploadBuildAndWaitForID() error: %v", uploadErr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout from upload helper, got %q", stdout)
	}
	if !strings.Contains(stderr, "Upload committed in App Store Connect.") {
		t.Fatalf("expected commit progress on stderr, got %q", stderr)
	}
	if buildUploadLookups != 2 {
		t.Fatalf("expected reconciliation and build-wait lookups, got %d", buildUploadLookups)
	}
	if result == nil || result.Build == nil || result.Build.Data.ID != "build-1" {
		t.Fatalf("expected recovered build result, got %#v", result)
	}
}

func TestUploadPKGBuildAndWaitForIDPinsValidatedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Demo.pkg")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatalf("write PKG: %v", err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat PKG: %v", err)
	}

	uploadedOriginal := false
	swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploads":
			return publishCommandJSONResponse(http.StatusCreated, `{"data":{"type":"buildUploads","id":"upload-1"}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploadFiles":
			if err := os.Rename(path, path+".validated"); err != nil {
				t.Fatalf("rename validated PKG: %v", err)
			}
			if err := os.WriteFile(path, []byte("new"), 0o600); err != nil {
				t.Fatalf("replace PKG path: %v", err)
			}
			return publishCommandJSONResponse(http.StatusCreated, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"Demo.pkg","fileSize":3,"uti":"com.apple.pkg","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part-1","length":3,"offset":0}]}}}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			body, readErr := io.ReadAll(req.Body)
			if readErr != nil {
				t.Fatalf("read upload body: %v", readErr)
			}
			if string(body) != "old" {
				t.Fatalf("uploaded swapped path contents %q", body)
			}
			uploadedOriginal = true
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/buildUploadFiles/file-1":
			return publishCommandJSONResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"INVALID","title":"stop after upload"}]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	}))

	_, err = uploadPKGBuildAndWaitForID(context.Background(), newPublishCommandTestClient(t), "app-1", path, fileInfo, "1.2.3", "42", asc.PlatformMacOS, time.Millisecond, time.Second, true)
	if err == nil || !strings.Contains(err.Error(), "stop after upload") {
		t.Fatalf("expected commit sentinel, got %v", err)
	}
	if !uploadedOriginal {
		t.Fatal("expected upload to read the pinned validated PKG")
	}
}
