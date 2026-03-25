package cmdtest

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestBuildsUploadWaitFailsFastWhenBuildUploadFails(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	restoreDiagnostics := shared.SetBuildUploadFailureDiagnosticsForTesting(func(context.Context, *asc.Client, string, *asc.BuildUploadResponse) (string, error) {
		return `Invalid Siri Support. App Intent description "Searches Apple Music" cannot contain "apple"`, nil
	})
	t.Cleanup(restoreDiagnostics)

	ipaPath := filepath.Join(t.TempDir(), "app.ipa")
	if err := os.WriteFile(ipaPath, []byte("test"), 0o600); err != nil {
		t.Fatalf("write ipa fixture: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	buildUploadChecks := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploads":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"42","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploadFiles":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"app.ipa","fileSize":4,"uti":"com.apple.itunes.ipa","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part-1","length":4,"offset":0,"requestHeaders":[{"name":"Content-Type","value":"application/octet-stream"}]}]}}}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/buildUploadFiles/file-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions":
			query := req.URL.Query()
			if query.Get("filter[version]") != "1.0.0" {
				t.Fatalf("expected filter[version]=1.0.0, got %q", query.Get("filter[version]"))
			}
			if query.Get("filter[platform]") != "IOS" {
				t.Fatalf("expected filter[platform]=IOS, got %q", query.Get("filter[platform]"))
			}
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/buildUploads/upload-1":
			buildUploadChecks++
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"42","platform":"IOS","state":{"state":"FAILED","errors":[{"code":"90062"},{"code":"90186"}]}}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "upload",
			"--app", "123456789",
			"--ipa", ipaPath,
			"--version", "1.0.0",
			"--build-number", "42",
			"--wait",
			"--poll-interval", "1ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected builds upload to fail, got nil")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on failure, got %q", stdout)
	}
	if !strings.Contains(stderr, "Waiting for build 42 (1.0.0) to appear in App Store Connect...") {
		t.Fatalf("expected wait progress output, got %q", stderr)
	}
	if !strings.Contains(runErr.Error(), `build upload "upload-1" failed with state FAILED`) {
		t.Fatalf("expected upload failure in error, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "90062") || !strings.Contains(runErr.Error(), "90186") {
		t.Fatalf("expected Apple error codes in error, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), `Invalid Siri Support. App Intent description "Searches Apple Music" cannot contain "apple"`) {
		t.Fatalf("expected enriched processing detail in error, got %v", runErr)
	}
	if buildUploadChecks == 0 {
		t.Fatal("expected build upload status to be checked")
	}
}

func TestBuildsUploadFailsFastWhenPostCommitVerificationSeesFailure(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	restoreDiagnostics := shared.SetBuildUploadFailureDiagnosticsForTesting(func(context.Context, *asc.Client, string, *asc.BuildUploadResponse) (string, error) {
		return `Invalid Siri Support. App Intent description "Searches Apple Music" cannot contain "apple"`, nil
	})
	t.Cleanup(restoreDiagnostics)

	ipaPath := filepath.Join(t.TempDir(), "app.ipa")
	if err := os.WriteFile(ipaPath, []byte("test"), 0o600); err != nil {
		t.Fatalf("write ipa fixture: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	buildUploadChecks := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploads":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"42","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploadFiles":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"app.ipa","fileSize":4,"uti":"com.apple.itunes.ipa","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part-1","length":4,"offset":0,"requestHeaders":[{"name":"Content-Type","value":"application/octet-stream"}]}]}}}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/buildUploadFiles/file-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/buildUploads/upload-1":
			buildUploadChecks++
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"42","platform":"IOS","state":{"state":"FAILED","errors":[{"code":"90062"},{"code":"90186"}]}}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "upload",
			"--app", "123456789",
			"--ipa", ipaPath,
			"--version", "1.0.0",
			"--build-number", "42",
			"--poll-interval", "1ms",
			"--verify-timeout", "5ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected builds upload to fail, got nil")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on failure, got %q", stdout)
	}
	if !strings.Contains(stderr, "Verifying initial App Store Connect processing for up to 5ms...") {
		t.Fatalf("expected verification progress output, got %q", stderr)
	}
	if !strings.Contains(runErr.Error(), `build upload "upload-1" failed with state FAILED`) {
		t.Fatalf("expected upload failure in error, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "90062") || !strings.Contains(runErr.Error(), "90186") {
		t.Fatalf("expected Apple error codes in error, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), `Invalid Siri Support. App Intent description "Searches Apple Music" cannot contain "apple"`) {
		t.Fatalf("expected enriched processing detail in error, got %v", runErr)
	}
	if buildUploadChecks == 0 {
		t.Fatal("expected build upload status to be checked")
	}
}

func TestBuildsUploadWithoutWaitSkipsPostCommitVerificationByDefault(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	ipaPath := filepath.Join(t.TempDir(), "app.ipa")
	if err := os.WriteFile(ipaPath, []byte("test"), 0o600); err != nil {
		t.Fatalf("write ipa fixture: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploads":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"42","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploadFiles":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"app.ipa","fileSize":4,"uti":"com.apple.itunes.ipa","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part-1","length":4,"offset":0,"requestHeaders":[{"name":"Content-Type","value":"application/octet-stream"}]}]}}}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/buildUploadFiles/file-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"uploaded":true}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "upload",
			"--app", "123456789",
			"--ipa", ipaPath,
			"--version", "1.0.0",
			"--build-number", "42",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr != nil {
		t.Fatalf("expected builds upload to succeed, got %v", runErr)
	}
	if strings.Contains(stderr, "Verifying initial App Store Connect processing") {
		t.Fatalf("expected no post-commit verification progress output by default, got %q", stderr)
	}
	if !strings.Contains(stdout, `"uploadId":"upload-1"`) {
		t.Fatalf("expected JSON output with upload ID, got %q", stdout)
	}
}

func TestBuildsUploadPostCommitVerificationUsesFreshTimeoutWindow(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_TIMEOUT", "1ms")

	ipaPath := filepath.Join(t.TempDir(), "app.ipa")
	if err := os.WriteFile(ipaPath, []byte("test"), 0o600); err != nil {
		t.Fatalf("write ipa fixture: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	buildUploadChecks := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploads":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"42","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploadFiles":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"app.ipa","fileSize":4,"uti":"com.apple.itunes.ipa","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part-1","length":4,"offset":0,"requestHeaders":[{"name":"Content-Type","value":"application/octet-stream"}]}]}}}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			time.Sleep(5 * time.Millisecond)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/buildUploadFiles/file-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/buildUploads/upload-1":
			buildUploadChecks++
			return jsonResponse(http.StatusOK, `{
				"data": {
					"type": "buildUploads",
					"id": "upload-1",
					"attributes": {
						"cfBundleShortVersionString": "1.0.0",
						"cfBundleVersion": "42",
						"platform": "IOS",
						"state": {"state": "UPLOADED"}
					},
					"relationships": {
						"build": {
							"data": {"type": "builds", "id": "build-1"}
						}
					}
				}
			}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "upload",
			"--app", "123456789",
			"--ipa", ipaPath,
			"--version", "1.0.0",
			"--build-number", "42",
			"--poll-interval", "1ms",
			"--verify-timeout", "10ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr != nil {
		t.Fatalf("expected builds upload to succeed, got %v", runErr)
	}
	if !strings.Contains(stderr, "Verifying initial App Store Connect processing for up to 10ms...") {
		t.Fatalf("expected verification progress output, got %q", stderr)
	}
	if !strings.Contains(stdout, `"uploadId":"upload-1"`) {
		t.Fatalf("expected JSON output with upload ID, got %q", stdout)
	}
	if buildUploadChecks == 0 {
		t.Fatal("expected build upload verification check")
	}
}
