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

func TestBuildsUploadReportsBuildID(t *testing.T) {
	tests := []struct {
		name          string
		linkedBuildID string
		args          []string
		wantStdout    []string
		rejectStdout  []string
		wantStderr    []string
		rejectStderr  []string
	}{
		{
			name:          "verify timeout reports linked build",
			linkedBuildID: "build-1",
			args:          []string{"--verify-timeout", "2s", "--output", "json"},
			wantStdout:    []string{`"uploadId":"upload-1"`, `"buildId":"build-1"`},
			rejectStderr:  []string{"is not available yet"},
		},
		{
			name:          "verify timeout table shows build",
			linkedBuildID: "build-1",
			args:          []string{"--verify-timeout", "2s", "--output", "table"},
			wantStdout:    []string{"Build ID", "build-1"},
		},
		{
			name:         "verify timeout reports unresolved build on stderr",
			args:         []string{"--verify-timeout", "20ms", "--output", "json"},
			wantStdout:   []string{`"uploadId":"upload-1"`},
			rejectStdout: []string{`"buildId"`},
			wantStderr: []string{
				`Build ID for upload upload-1 is not available yet: verification ended before App Store Connect exposed the build; look it up later with: asc builds info --app "123456789" --build-number "42" --version "1.0.0" --platform IOS`,
			},
		},
		{
			name:          "wait reports build",
			linkedBuildID: "build-1",
			args:          []string{"--wait", "--output", "json"},
			wantStdout:    []string{`"uploadId":"upload-1"`, `"buildId":"build-1"`},
		},
		{
			name:          "default omits build",
			linkedBuildID: "build-1",
			args:          []string{"--output", "json"},
			rejectStdout:  []string{`"buildId"`},
			rejectStderr:  []string{"is not available yet"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			installBuildUploadBuildIDTransport(t, test.linkedBuildID)

			stdout, stderr, err := runBuildsUploadForBuildID(t, test.args...)
			if err != nil {
				t.Fatalf("expected builds upload to succeed, got %v", err)
			}
			for _, want := range test.wantStdout {
				if !strings.Contains(stdout, want) {
					t.Fatalf("expected stdout to contain %q, got %q", want, stdout)
				}
			}
			for _, reject := range test.rejectStdout {
				if strings.Contains(stdout, reject) {
					t.Fatalf("expected stdout not to contain %q, got %q", reject, stdout)
				}
			}
			for _, want := range test.wantStderr {
				if !strings.Contains(stderr, want) {
					t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
				}
			}
			for _, reject := range test.rejectStderr {
				if strings.Contains(stderr, reject) {
					t.Fatalf("expected stderr not to contain %q, got %q", reject, stderr)
				}
			}
		})
	}
}
