package cmdtest

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestBuildsUploadRejectsUnsafePKGBeforeNetwork(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	dir := t.TempDir()
	emptyPath := filepath.Join(dir, "empty.pkg")
	if err := os.WriteFile(emptyPath, nil, 0o600); err != nil {
		t.Fatalf("write empty PKG: %v", err)
	}
	targetPath := filepath.Join(dir, "target.pkg")
	if err := os.WriteFile(targetPath, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write PKG target: %v", err)
	}
	linkPath := filepath.Join(dir, "link.pkg")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("create PKG symlink: %v", err)
	}

	asctest.NewRegistry(t).Install()

	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{name: "empty", path: emptyPath, wantErr: "--pkg must not be empty"},
		{name: "symlink", path: linkPath, wantErr: "from --pkg"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			if err := root.Parse([]string{
				"builds", "upload",
				"--app", "123456789",
				"--pkg", test.path,
				"--version", "1.0.0",
				"--build-number", "42",
				"--dry-run",
			}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			err := root.Run(context.Background())
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("expected error containing %q, got %v", test.wantErr, err)
			}
		})
	}
}

func TestBuildsUploadPinsValidatedPKGAcrossPathReplacement(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	dir := t.TempDir()
	pkgPath := filepath.Join(dir, "app.pkg")
	replacementPath := filepath.Join(dir, "replacement.pkg")
	originalPayload := []byte("original")
	replacementPayload := []byte("replaced")
	if err := os.WriteFile(pkgPath, originalPayload, 0o600); err != nil {
		t.Fatalf("write original PKG: %v", err)
	}
	if err := os.WriteFile(replacementPath, replacementPayload, 0o600); err != nil {
		t.Fatalf("write replacement PKG: %v", err)
	}
	originalHash := fmt.Sprintf("%x", md5.Sum(originalPayload))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	var uploadedPayload string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploads":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"42","platform":"MAC_OS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploadFiles":
			if err := os.Remove(pkgPath); err != nil {
				t.Fatalf("remove original PKG path: %v", err)
			}
			if err := os.Rename(replacementPath, pkgPath); err != nil {
				t.Fatalf("replace original PKG path: %v", err)
			}
			body := fmt.Sprintf(`{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"app.pkg","fileSize":%d,"uti":"com.apple.pkg","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part-1","length":%d,"offset":0}],"sourceFileChecksums":{"file":{"hash":%q,"algorithm":"MD5"}}}}}`, len(originalPayload), len(originalPayload), originalHash)
			return jsonResponse(http.StatusOK, body)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			uploadedPayload = string(body)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/buildUploadFiles/file-1":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			if !strings.Contains(string(body), originalHash) {
				t.Fatalf("commit body does not contain original checksum %q: %s", originalHash, body)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"uploaded":true}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{
		"builds", "upload",
		"--app", "123456789",
		"--pkg", pkgPath,
		"--version", "1.0.0",
		"--build-number", "42",
		"--checksum",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var runErr error
	_, _ = captureOutput(t, func() {
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if uploadedPayload != string(originalPayload) {
		t.Fatalf("uploaded payload = %q, want original %q", uploadedPayload, originalPayload)
	}
}
