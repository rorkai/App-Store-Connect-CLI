package cmdtest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// TestIAPReviewScreenshotsKeepChecksumAndUploadBytesTogether rewrites the source
// file after the CLI first reads it and expects the upload and the committed
// checksum to both describe the original bytes.
func TestIAPReviewScreenshotsKeepChecksumAndUploadBytesTogether(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "create", args: []string{"iap", "review-screenshots", "create", "--iap-id", "9000000001"}},
		{name: "update", args: []string{"iap", "review-screenshots", "update", "--screenshot-id", "shot-1"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			filePath := filepath.Join(t.TempDir(), "review.png")
			writeReviewScreenshotPNG(t, filePath)
			original, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			expectedChecksum, err := asc.ComputeChecksumFromReader(bytes.NewReader(original), asc.ChecksumAlgorithmMD5)
			if err != nil {
				t.Fatalf("compute fixture checksum: %v", err)
			}

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })

			mutated := false
			var uploaded []byte
			var committedChecksum string
			// The create POST or the update pre-read hands out the upload
			// operations; the source file changes right after.
			reserveUpload := func(status int) (*http.Response, error) {
				if err := rewriteFileInPlace(filePath, bytes.Repeat([]byte{'B'}, len(original))); err != nil {
					t.Fatalf("rewrite source fixture: %v", err)
				}
				mutated = true
				return jsonResponse(status, iapReviewScreenshotResponse("shot-1", true, "UPLOADING", int64(len(original))))
			}
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodPost && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots":
					return reserveUpload(http.StatusCreated)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1" && !mutated:
					return reserveUpload(http.StatusOK)
				case req.Method == http.MethodPut && req.URL.Path == "/part":
					var err error
					uploaded, err = io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("read uploaded body: %v", err)
					}
					return jsonResponse(http.StatusOK, "")
				case req.Method == http.MethodPatch && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
					var payload asc.InAppPurchaseAppStoreReviewScreenshotUpdateRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode commit payload: %v", err)
					}
					if payload.Data.Attributes == nil || payload.Data.Attributes.SourceFileChecksum == nil {
						t.Fatal("expected sourceFileChecksum in commit payload")
					}
					committedChecksum = *payload.Data.Attributes.SourceFileChecksum
					return jsonResponse(http.StatusOK, iapReviewScreenshotResponse("shot-1", false, "UPLOADING", int64(len(original))))
				case req.Method == http.MethodGet && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
					return jsonResponse(http.StatusOK, iapReviewScreenshotResponse("shot-1", false, "COMPLETE", int64(len(original))))
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			})

			stdout, stderr, runErr := runRootCommand(t, append(test.args, "--file", filePath, "--output", "json"))
			if runErr != nil {
				t.Fatalf("expected %s success, got %v", test.name, runErr)
			}
			if !mutated {
				t.Fatal("transport hook did not mutate source file")
			}
			if !bytes.Equal(uploaded, original) {
				t.Fatalf("uploaded bytes differ from pre-mutation source: got %d bytes", len(uploaded))
			}
			if committedChecksum != expectedChecksum.Hash {
				t.Fatalf("committed checksum = %q, want %q", committedChecksum, expectedChecksum.Hash)
			}
			if stdout == "" || stderr != "" {
				t.Fatalf("unexpected command output: stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
}

func rewriteFileInPlace(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.WriteAt(contents, 0); err != nil {
		return err
	}
	return file.Sync()
}
