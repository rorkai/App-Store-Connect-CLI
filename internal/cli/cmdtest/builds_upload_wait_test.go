package cmdtest

import (
	"context"
	"io"
	"net/http"
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

	ipaPath := writeBuildUploadIPA(t, "com.example.demo")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	buildUploadChecks := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789":
			return jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"123456789","attributes":{"name":"Demo","bundleId":"com.example.demo"}}}`)
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

	ipaPath := writeBuildUploadIPA(t, "com.example.demo")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	buildUploadChecks := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789":
			return jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"123456789","attributes":{"name":"Demo","bundleId":"com.example.demo"}}}`)
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

	ipaPath := writeBuildUploadIPA(t, "com.example.demo")

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

func TestBuildsUploadTestNotesWritesBeforeProcessingCompletes(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	ipaPath := writeBuildUploadIPA(t, "com.example.demo")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	buildReads := 0
	localizationCreated := false
	appLocalizationCreated := false
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789":
			return jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"123456789","attributes":{"name":"Demo","bundleId":"com.example.demo"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploads":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"42","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploadFiles":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"app.ipa","fileSize":4,"uti":"com.apple.itunes.ipa","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part-1","length":4,"offset":0,"requestHeaders":[{"name":"Content-Type","value":"application/octet-stream"}]}]}}}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/buildUploadFiles/file-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/buildUploads/upload-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","relationships":{"build":{"data":{"type":"builds","id":"build-1"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1":
			buildReads++
			if buildReads > 1 {
				return nil, http.ErrUseLastResponse
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"PROCESSING"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/betaAppLocalizations":
			if got := req.URL.Query().Get("filter[app]"); got != "123456789" {
				t.Fatalf("expected filter[app]=123456789, got %q", got)
			}
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/betaAppLocalizations":
			appLocalizationCreated = true
			return jsonResponse(http.StatusCreated, `{"data":{"type":"betaAppLocalizations","id":"bal-1","attributes":{"locale":"en-US"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1/betaBuildLocalizations":
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/betaBuildLocalizations":
			localizationCreated = true
			return jsonResponse(http.StatusCreated, `{"data":{"type":"betaBuildLocalizations","id":"loc-1","attributes":{"locale":"en-US","whatsNew":"Test the new flow"}}}`)
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
			"--test-notes", "Test the new flow",
			"--locale", "en-US",
			"--poll-interval", "1ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr != nil {
		t.Fatalf("expected test notes to be written after discovery, got %v", runErr)
	}
	if buildReads != 1 {
		t.Fatalf("expected one build read for discovery, got %d", buildReads)
	}
	if !appLocalizationCreated {
		t.Fatal("expected the missing TestFlight app localization to be created")
	}
	if !localizationCreated {
		t.Fatal("expected beta build localization to be created")
	}
	if !strings.Contains(stderr, "Build build-1 discovered; setting What to Test notes...") {
		t.Fatalf("expected note progress on stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"uploadId":"upload-1"`) {
		t.Fatalf("expected JSON output with upload ID, got %q", stdout)
	}
}

func TestBuildsUploadTestNotesFailurePreservesDiscoveredBuildRecoveryContext(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	testNotes := "Line one\nLine two\x1b[31m"

	ipaPath := writeBuildUploadIPA(t, "com.example.demo")

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
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"app.ipa","fileSize":4,"uti":"com.apple.itunes.ipa","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part-1","length":4,"offset":0,"requestHeaders":[{"name":"Content-Type","value":"application/octet-stream"}]}]}}}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/buildUploadFiles/file-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/buildUploads/upload-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","relationships":{"build":{"data":{"type":"builds","id":"build-1"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"PROCESSING"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/betaAppLocalizations":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"betaAppLocalizations","id":"bal-1","attributes":{"locale":"en-US"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1/betaBuildLocalizations":
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/betaBuildLocalizations":
			return jsonResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"The provided entity has an invalid attribute","detail":"What to Test was rejected by the server"}]}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "upload",
			"--app", "123456789",
			"--ipa", ipaPath,
			"--version", "1.0.0",
			"--build-number", "42",
			"--test-notes", testNotes,
			"--locale", "en-US",
			"--poll-interval", "1ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected test notes rejection after upload")
	}
	if stdout != "" {
		t.Fatalf("expected no new structured output contract on failure, got %q", stdout)
	}
	wantParts := []string{
		`build "build-1" is available`,
		`locale "en-US"`,
		"The provided entity has an invalid attribute: What to Test was rejected by the server",
		"retry without uploading the build again",
		"reuse the original notes",
		"asc builds test-notes create --build-id BUILD_ID --locale LOCALE --whats-new NOTES",
	}
	for _, want := range wantParts {
		if !strings.Contains(runErr.Error(), want) {
			t.Fatalf("expected recovery error to contain %q, got %v", want, runErr)
		}
	}
	if asc.HasInterpretedTerminalSequence(runErr.Error()) || strings.Contains(runErr.Error(), "Line one") {
		t.Fatalf("expected sanitized human recovery without embedded notes, got %q", runErr)
	}
}

func TestBuildsUploadPostCommitVerificationUsesFreshTimeoutWindow(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_TIMEOUT", "250ms")

	ipaPath := writeBuildUploadIPA(t, "com.example.demo")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	buildUploadChecks := 0
	initialRequestDeadline := make(chan time.Time, 1)
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789":
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected initial request context deadline")
			}
			initialRequestDeadline <- deadline
			return jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"123456789","attributes":{"name":"Demo","bundleId":"com.example.demo"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploads":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"42","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploadFiles":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"app.ipa","fileSize":4,"uti":"com.apple.itunes.ipa","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part-1","length":4,"offset":0,"requestHeaders":[{"name":"Content-Type","value":"application/octet-stream"}]}]}}}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			if delay := time.Until((<-initialRequestDeadline).Add(50 * time.Millisecond)); delay > 0 {
				time.Sleep(delay)
			}
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
			"--verify-timeout", "250ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr != nil {
		t.Fatalf("expected builds upload to succeed, got %v", runErr)
	}
	if !strings.Contains(stderr, "Verifying initial App Store Connect processing for up to 250ms...") {
		t.Fatalf("expected verification progress output, got %q", stderr)
	}
	if !strings.Contains(stdout, `"uploadId":"upload-1"`) {
		t.Fatalf("expected JSON output with upload ID, got %q", stdout)
	}
	if buildUploadChecks == 0 {
		t.Fatal("expected build upload verification check")
	}
}
