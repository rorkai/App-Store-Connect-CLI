package cmdtest

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPublishAppStoreSubmitUsesModernReviewSubmissionFlow(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	ipaPath := filepath.Join(t.TempDir(), "app.ipa")
	if err := os.WriteFile(ipaPath, []byte("test"), 0o600); err != nil {
		t.Fatalf("write ipa fixture: %v", err)
	}

	requests := newRequestLog(20)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests.Add(req.Method + " " + req.URL.Path)

		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploads":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.2.3","cfBundleVersion":"42","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploadFiles":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"app.ipa","fileSize":4,"uti":"com.apple.itunes.ipa","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part-1","length":4,"offset":0,"requestHeaders":[{"name":"Content-Type","value":"application/octet-stream"}]}]}}}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/buildUploadFiles/file-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/buildUploads/upload-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.2.3","cfBundleVersion":"42","platform":"IOS"},"relationships":{"build":{"data":{"type":"builds","id":"build-42"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-42":
			return jsonResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"VALID"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions":
			if req.URL.Query().Get("filter[version]") != "1.2.3" {
				t.Fatalf("expected filter[version]=1.2.3, got %q", req.URL.Query().Get("filter[version]"))
			}
			if req.URL.Query().Get("filter[platform]") != "IOS" {
				t.Fatalf("expected filter[platform]=IOS, got %q", req.URL.Query().Get("filter[platform]"))
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			query := req.URL.Query()
			if query.Get("filter[app]") != "app-1" {
				t.Fatalf("expected filter[app]=app-1, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[preReleaseVersion]") != "prv-1" {
				t.Fatalf("expected filter[preReleaseVersion]=prv-1, got %q", query.Get("filter[preReleaseVersion]"))
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"VALID"}}],"links":{"next":""}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			switch {
			case query.Get("filter[versionString]") == "1.2.3":
				return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`)
			case strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE"):
				return jsonResponse(http.StatusOK, `{"data":[]}`)
			default:
				t.Fatalf("unexpected app store versions query: %s", req.URL.RawQuery)
				return nil, nil
			}
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionSubmission":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return jsonResponse(http.StatusNoContent, "")
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			return jsonResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			if req.URL.Query().Get("filter[state]") != "READY_FOR_REVIEW" {
				t.Fatalf("expected READY_FOR_REVIEW filter, got %q", req.URL.Query().Get("filter[state]"))
			}
			if req.URL.Query().Get("filter[platform]") != "IOS" {
				t.Fatalf("expected platform filter IOS, got %q", req.URL.Query().Get("filter[platform]"))
			}
			return jsonResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/review-sub-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-03-15T00:00:00Z"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersionSubmissions":
			t.Fatalf("publish appstore should not use legacy appStoreVersionSubmissions endpoint")
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "appstore",
			"--app", "app-1",
			"--ipa", ipaPath,
			"--version", "1.2.3",
			"--build-number", "42",
			"--submit",
			"--confirm",
			"--timeout", "1s",
			"--poll-interval", "1ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr == "" {
		t.Fatalf("expected progress output on stderr, got empty string")
	}
	if !strings.Contains(stdout, `"submissionId":"review-sub-1"`) {
		t.Fatalf("expected review submission ID in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"submitted":true`) {
		t.Fatalf("expected submitted=true in output, got %q", stdout)
	}

	recordedRequests := requests.Snapshot()
	joined := strings.Join(recordedRequests, "\n")
	if strings.Contains(joined, "POST /v1/appStoreVersionSubmissions") {
		t.Fatalf("did not expect legacy submission endpoint, requests: %v", recordedRequests)
	}
	if !strings.Contains(joined, "POST /v1/reviewSubmissions") {
		t.Fatalf("expected modern review submission create request, requests: %v", recordedRequests)
	}
}

func TestPublishAppStoreSubmitUsesFreshTimeoutBudgetsForPreflightAndSubmission(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_TIMEOUT", "100ms")

	ipaPath := filepath.Join(t.TempDir(), "app.ipa")
	if err := os.WriteFile(ipaPath, []byte("test"), 0o600); err != nil {
		t.Fatalf("write ipa fixture: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var reviewSubmissionBudget time.Duration

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploads":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.2.3","cfBundleVersion":"42","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploadFiles":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"app.ipa","fileSize":4,"uti":"com.apple.itunes.ipa","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part-1","length":4,"offset":0,"requestHeaders":[{"name":"Content-Type","value":"application/octet-stream"}]}]}}}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/buildUploadFiles/file-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/buildUploads/upload-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.2.3","cfBundleVersion":"42","platform":"IOS"},"relationships":{"build":{"data":{"type":"builds","id":"build-42"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-42":
			return jsonResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"VALID"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"VALID"}}],"links":{"next":""}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			switch {
			case query.Get("filter[versionString]") == "1.2.3":
				return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`)
			case strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE"):
				return jsonResponse(http.StatusOK, `{"data":[]}`)
			default:
				t.Fatalf("unexpected app store versions query: %s", req.URL.RawQuery)
				return nil, nil
			}
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return jsonResponse(http.StatusNoContent, "")
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			if err := sleepWithContext(req.Context()); err != nil {
				return nil, err
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			return jsonResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionSubmission":
			if err := sleepWithContext(req.Context()); err != nil {
				return nil, err
			}
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			if req.URL.Query().Get("filter[state]") != "READY_FOR_REVIEW" {
				t.Fatalf("expected READY_FOR_REVIEW filter, got %q", req.URL.Query().Get("filter[state]"))
			}
			if req.URL.Query().Get("filter[platform]") != "IOS" {
				t.Fatalf("expected platform filter IOS, got %q", req.URL.Query().Get("filter[platform]"))
			}
			return jsonResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected review submission request to have a deadline")
			}
			reviewSubmissionBudget = time.Until(deadline)
			return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/review-sub-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-03-15T00:00:00Z"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "appstore",
			"--app", "app-1",
			"--ipa", ipaPath,
			"--version", "1.2.3",
			"--build-number", "42",
			"--submit",
			"--confirm",
			"--timeout", "1s",
			"--poll-interval", "1ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected publish appstore submit to succeed with fresh timeout budgets, got %v", err)
		}
	})

	if stderr == "" {
		t.Fatalf("expected progress output on stderr, got empty string")
	}
	if !strings.Contains(stdout, `"submissionId":"review-sub-1"`) {
		t.Fatalf("expected review submission ID in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"submitted":true`) {
		t.Fatalf("expected submitted=true in output, got %q", stdout)
	}
	if reviewSubmissionBudget < 60*time.Millisecond {
		t.Fatalf("expected fresh review submission timeout budget, got %v", reviewSubmissionBudget)
	}
}

func TestPublishAppStoreSubmitPreflightUsesPublishTimeoutOverride(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_TIMEOUT", "100ms")

	ipaPath := filepath.Join(t.TempDir(), "app.ipa")
	if err := os.WriteFile(ipaPath, []byte("test"), 0o600); err != nil {
		t.Fatalf("write ipa fixture: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var localizationBudget time.Duration
	var subscriptionBudget time.Duration

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploads":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.2.3","cfBundleVersion":"42","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploadFiles":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"app.ipa","fileSize":4,"uti":"com.apple.itunes.ipa","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part-1","length":4,"offset":0,"requestHeaders":[{"name":"Content-Type","value":"application/octet-stream"}]}]}}}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/buildUploadFiles/file-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/buildUploads/upload-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.2.3","cfBundleVersion":"42","platform":"IOS"},"relationships":{"build":{"data":{"type":"builds","id":"build-42"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-42":
			return jsonResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"VALID"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"VALID"}}],"links":{"next":""}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			switch {
			case query.Get("filter[versionString]") == "1.2.3":
				return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`)
			case strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE"):
				return jsonResponse(http.StatusOK, `{"data":[]}`)
			default:
				t.Fatalf("unexpected app store versions query: %s", req.URL.RawQuery)
				return nil, nil
			}
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return jsonResponse(http.StatusNoContent, "")
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected localization preflight request to have a deadline")
			}
			localizationBudget = time.Until(deadline)
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected subscription preflight request to have a deadline")
			}
			subscriptionBudget = time.Until(deadline)
			return jsonResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionSubmission":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return jsonResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/review-sub-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-03-15T00:00:00Z"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "appstore",
			"--app", "app-1",
			"--ipa", ipaPath,
			"--version", "1.2.3",
			"--build-number", "42",
			"--submit",
			"--confirm",
			"--timeout", "1s",
			"--poll-interval", "1ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected publish appstore submit to succeed with explicit timeout override, got %v", err)
		}
	})

	if stderr == "" {
		t.Fatalf("expected progress output on stderr, got empty string")
	}
	if !strings.Contains(stdout, `"submissionId":"review-sub-1"`) {
		t.Fatalf("expected review submission ID in output, got %q", stdout)
	}
	if localizationBudget < 700*time.Millisecond {
		t.Fatalf("expected localization preflight to use publish timeout budget, got %v", localizationBudget)
	}
	if subscriptionBudget < 700*time.Millisecond {
		t.Fatalf("expected subscription preflight to use publish timeout budget, got %v", subscriptionBudget)
	}
}

func TestPublishAppStoreSubmitDefaultPathHonorsASCTimeout(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_TIMEOUT", "250ms")

	stopErr := errors.New("stop after publish timeout budget capture")

	ipaPath := filepath.Join(t.TempDir(), "app.ipa")
	if err := os.WriteFile(ipaPath, []byte("test"), 0o600); err != nil {
		t.Fatalf("write ipa fixture: %v", err)
	}

	var localizationBudget time.Duration
	var subscriptionBudget time.Duration

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploads":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.2.3","cfBundleVersion":"42","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploadFiles":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"app.ipa","fileSize":4,"uti":"com.apple.itunes.ipa","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part-1","length":4,"offset":0,"requestHeaders":[{"name":"Content-Type","value":"application/octet-stream"}]}]}}}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/buildUploadFiles/file-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/buildUploads/upload-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.2.3","cfBundleVersion":"42","platform":"IOS"},"relationships":{"build":{"data":{"type":"builds","id":"build-42"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-42":
			return jsonResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"VALID"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"VALID"}}],"links":{"next":""}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			switch {
			case query.Get("filter[versionString]") == "1.2.3":
				return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`)
			case strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE"):
				return jsonResponse(http.StatusOK, `{"data":[]}`)
			default:
				t.Fatalf("unexpected app store versions query: %s", req.URL.RawQuery)
				return nil, nil
			}
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return jsonResponse(http.StatusNoContent, "")
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected localization preflight request to have a deadline")
			}
			localizationBudget = time.Until(deadline)
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected subscription preflight request to have a deadline")
			}
			subscriptionBudget = time.Until(deadline)
			return jsonResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionSubmission":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return jsonResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return nil, stopErr
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/review-sub-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-03-15T00:00:00Z"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "appstore",
			"--app", "app-1",
			"--ipa", ipaPath,
			"--version", "1.2.3",
			"--build-number", "42",
			"--submit",
			"--confirm",
			"--poll-interval", "1ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil || !strings.Contains(runErr.Error(), stopErr.Error()) {
		t.Fatalf("expected publish appstore submit to stop after capturing ASC_TIMEOUT budgets, got %v", runErr)
	}
	if stderr == "" {
		t.Fatal("expected progress output on stderr, got empty string")
	}
	if localizationBudget <= 0 || localizationBudget > time.Second {
		t.Fatalf("expected localization preflight to honor ASC_TIMEOUT-derived budget, got %v", localizationBudget)
	}
	if subscriptionBudget <= 0 || subscriptionBudget > time.Second {
		t.Fatalf("expected subscription preflight to honor ASC_TIMEOUT-derived budget, got %v", subscriptionBudget)
	}
}

func TestPublishAppStoreSubmitDefaultTimeoutUsesSharedPipelineBudget(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	ipaPath := filepath.Join(t.TempDir(), "app.ipa")
	if err := os.WriteFile(ipaPath, []byte("test"), 0o600); err != nil {
		t.Fatalf("write ipa fixture: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var localizationBudget time.Duration
	var subscriptionBudget time.Duration
	var reviewSubmissionBudget time.Duration

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploads":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.2.3","cfBundleVersion":"42","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploadFiles":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"app.ipa","fileSize":4,"uti":"com.apple.itunes.ipa","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part-1","length":4,"offset":0,"requestHeaders":[{"name":"Content-Type","value":"application/octet-stream"}]}]}}}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/buildUploadFiles/file-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/buildUploads/upload-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.2.3","cfBundleVersion":"42","platform":"IOS"},"relationships":{"build":{"data":{"type":"builds","id":"build-42"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-42":
			return jsonResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"VALID"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"VALID"}}],"links":{"next":""}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			switch {
			case query.Get("filter[versionString]") == "1.2.3":
				return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`)
			case strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE"):
				return jsonResponse(http.StatusOK, `{"data":[]}`)
			default:
				t.Fatalf("unexpected app store versions query: %s", req.URL.RawQuery)
				return nil, nil
			}
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return jsonResponse(http.StatusNoContent, "")
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected localization preflight request to have a deadline")
			}
			localizationBudget = time.Until(deadline)
			timer := time.NewTimer(20 * time.Millisecond)
			defer timer.Stop()
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-timer.C:
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected subscription preflight request to have a deadline")
			}
			subscriptionBudget = time.Until(deadline)
			return jsonResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionSubmission":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return jsonResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected review submission request to have a deadline")
			}
			reviewSubmissionBudget = time.Until(deadline)
			return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/review-sub-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-03-15T00:00:00Z"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "appstore",
			"--app", "app-1",
			"--ipa", ipaPath,
			"--version", "1.2.3",
			"--build-number", "42",
			"--submit",
			"--confirm",
			"--poll-interval", "1ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected publish appstore submit to succeed with default timeout, got %v", err)
		}
	})

	if stderr == "" {
		t.Fatalf("expected progress output on stderr, got empty string")
	}
	if !strings.Contains(stdout, `"submissionId":"review-sub-1"`) {
		t.Fatalf("expected review submission ID in output, got %q", stdout)
	}
	if localizationBudget == 0 {
		t.Fatal("expected localization preflight budget to be captured")
	}
	if subscriptionBudget == 0 {
		t.Fatal("expected subscription preflight budget to be captured")
	}
	if reviewSubmissionBudget == 0 {
		t.Fatal("expected review submission budget to be captured")
	}
	if localizationBudget < time.Minute {
		t.Fatalf("expected localization preflight to inherit publish pipeline timeout, got %v", localizationBudget)
	}
	if subscriptionBudget < time.Minute {
		t.Fatalf("expected subscription preflight to inherit publish pipeline timeout, got %v", subscriptionBudget)
	}
	// The default publish path should keep consuming one shared timeout budget
	// across preflight and submission instead of refreshing a fresh deadline.
	if reviewSubmissionBudget >= localizationBudget-5*time.Millisecond {
		t.Fatalf("expected review submission budget %v to be lower than localization preflight budget %v", reviewSubmissionBudget, localizationBudget)
	}
}
