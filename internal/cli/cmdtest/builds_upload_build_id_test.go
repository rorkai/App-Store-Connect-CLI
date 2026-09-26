package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// installBuildUploadBuildIDTransport serves a committed IPA upload whose
// buildUploads lookup exposes the linked build only when include=build is
// requested, as App Store Connect does. linkedBuildID empty means the build
// never links during the run.
func installBuildUploadBuildIDTransport(t *testing.T, linkedBuildID string) {
	t.Helper()

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789":
			return jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"123456789","attributes":{"name":"Demo","bundleId":"com.example.demo"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploads":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"42","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploadFiles":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"app.ipa","fileSize":4,"uti":"com.apple.itunes.ipa","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part-1","length":4,"offset":0}]}}}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/buildUploadFiles/file-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/buildUploads/upload-1":
			if linkedBuildID == "" || req.URL.Query().Get("include") != "build" {
				return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"42","platform":"IOS","state":{"state":"PROCESSING"}}}}`)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"42","platform":"IOS","state":{"state":"COMPLETE"}},"relationships":{"build":{"data":{"type":"builds","id":"`+linkedBuildID+`"}}}}}`)
		case linkedBuildID == "" && req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case linkedBuildID != "" && req.Method == http.MethodGet && req.URL.Path == "/v1/builds/"+linkedBuildID:
			return jsonResponse(http.StatusOK, `{"data":{"type":"builds","id":"`+linkedBuildID+`","attributes":{"version":"42","processingState":"VALID"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
}

func runBuildsUploadForBuildID(t *testing.T, extraArgs ...string) (string, string, error) {
	t.Helper()

	ipaPath := writeBuildUploadIPA(t, "com.example.demo")
	args := append([]string{
		"builds", "upload",
		"--app", "123456789",
		"--ipa", ipaPath,
		"--version", "1.0.0",
		"--build-number", "42",
		"--poll-interval", "1ms",
	}, extraArgs...)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func TestBuildsUploadVerifyTimeoutReportsLinkedBuildID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	installBuildUploadBuildIDTransport(t, "build-1")

	stdout, stderr, err := runBuildsUploadForBuildID(t, "--verify-timeout", "2s", "--output", "json")
	if err != nil {
		t.Fatalf("expected builds upload to succeed, got %v", err)
	}
	if !strings.Contains(stdout, `"uploadId":"upload-1"`) || !strings.Contains(stdout, `"buildId":"build-1"`) {
		t.Fatalf("expected receipt with uploadId and buildId, got %q", stdout)
	}
	if strings.Contains(stderr, "is not available yet") {
		t.Fatalf("did not expect an unresolved build notice, got %q", stderr)
	}
}

func TestBuildsUploadVerifyTimeoutTableShowsBuildID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	installBuildUploadBuildIDTransport(t, "build-1")

	stdout, _, err := runBuildsUploadForBuildID(t, "--verify-timeout", "2s", "--output", "table")
	if err != nil {
		t.Fatalf("expected builds upload to succeed, got %v", err)
	}
	if !strings.Contains(stdout, "Build ID") || !strings.Contains(stdout, "build-1") {
		t.Fatalf("expected table with Build ID column, got %q", stdout)
	}
}

func TestBuildsUploadVerifyTimeoutReportsUnresolvedBuildOnStderr(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	installBuildUploadBuildIDTransport(t, "")

	stdout, stderr, err := runBuildsUploadForBuildID(t, "--verify-timeout", "20ms", "--output", "json")
	if err != nil {
		t.Fatalf("expected builds upload to succeed, got %v", err)
	}
	if strings.Contains(stdout, `"buildId"`) {
		t.Fatalf("expected no buildId when the build is not linked yet, got %q", stdout)
	}
	if !strings.Contains(stdout, `"uploadId":"upload-1"`) {
		t.Fatalf("expected receipt with uploadId, got %q", stdout)
	}
	want := `Build ID for upload upload-1 is not available yet: verification ended before App Store Connect exposed the build; look it up later with: asc builds info --app "123456789" --build-number "42" --version "1.0.0" --platform IOS`
	if !strings.Contains(stderr, want) {
		t.Fatalf("expected unresolved build notice %q on stderr, got %q", want, stderr)
	}
}

func TestBuildsUploadWaitReportsBuildID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	installBuildUploadBuildIDTransport(t, "build-1")

	stdout, _, err := runBuildsUploadForBuildID(t, "--wait", "--output", "json")
	if err != nil {
		t.Fatalf("expected builds upload --wait to succeed, got %v", err)
	}
	if !strings.Contains(stdout, `"uploadId":"upload-1"`) || !strings.Contains(stdout, `"buildId":"build-1"`) {
		t.Fatalf("expected receipt with uploadId and buildId, got %q", stdout)
	}
}

func TestBuildsUploadDefaultOmitsBuildID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	installBuildUploadBuildIDTransport(t, "build-1")

	stdout, stderr, err := runBuildsUploadForBuildID(t, "--output", "json")
	if err != nil {
		t.Fatalf("expected builds upload to succeed, got %v", err)
	}
	if strings.Contains(stdout, `"buildId"`) || strings.Contains(stderr, "is not available yet") {
		t.Fatalf("expected default mode to stay unchanged, got stdout %q stderr %q", stdout, stderr)
	}
}
