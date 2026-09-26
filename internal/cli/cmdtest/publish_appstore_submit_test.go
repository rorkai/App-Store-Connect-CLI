package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func respondToPublishAppLookup(t *testing.T, req *http.Request) (*http.Response, error, bool) {
	t.Helper()

	const app = "app-1"

	if req.Method != http.MethodGet || req.URL.Path != "/v1/apps" {
		return nil, nil, false
	}
	if req.URL.Query().Get("filter[bundleId]") != app {
		t.Fatalf("expected filter[bundleId]=%s, got %q", app, req.URL.Query().Get("filter[bundleId]"))
	}
	if limit := req.URL.Query().Get("limit"); limit != "2" {
		t.Fatalf("expected lookup limit=2, got %q", limit)
	}
	resp, err := jsonResponse(http.StatusOK, `{"data":[{"type":"apps","id":"`+app+`","attributes":{"bundleId":"`+app+`","name":"`+app+`"}}]}`)
	return resp, err, true
}

func respondToFinalReviewSubmissionValidation(req *http.Request) (*http.Response, error, bool) {
	if req.Method != http.MethodGet {
		return nil, nil, false
	}

	switch req.URL.Path {
	case "/v1/reviewSubmissions/review-sub-1":
		resp, err := jsonResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`)
		return resp, err, true
	case "/v1/reviewSubmissions/review-sub-1/items":
		resp, err := jsonResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissionItems","id":"item-1","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}],"links":{"self":"/v1/reviewSubmissions/review-sub-1/items"}}`)
		return resp, err, true
	default:
		return nil, nil, false
	}
}

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
		if resp, err, ok := respondToPublishAppLookup(t, req); ok {
			return resp, err
		}
		if resp, err, ok := respondToFinalReviewSubmissionValidation(req); ok {
			return resp, err
		}

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
			return jsonResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`)
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
	if got := strings.Count(joined, "GET /v1/appStoreVersions/version-1/appStoreVersionSubmission"); got != 1 {
		t.Fatalf("expected exactly one existing submission lookup, got %d requests: %v", got, recordedRequests)
	}
	reviewPreflightIndex, buildAttachmentIndex := -1, -1
	for idx, request := range recordedRequests {
		switch request {
		case "GET /v1/apps/app-1/reviewSubmissions":
			if reviewPreflightIndex == -1 {
				reviewPreflightIndex = idx
			}
		case "GET /v1/appStoreVersions/version-1/build":
			if buildAttachmentIndex == -1 {
				buildAttachmentIndex = idx
			}
		}
	}
	if reviewPreflightIndex == -1 || buildAttachmentIndex == -1 || reviewPreflightIndex > buildAttachmentIndex {
		t.Fatalf("expected review submission preflight before build attachment, requests: %v", recordedRequests)
	}
}

func TestPublishAppStoreSubmitAlreadySubmittedSkipsPreflightAndBuildAttachment(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	ipaPath := filepath.Join(t.TempDir(), "app.ipa")
	if err := os.WriteFile(ipaPath, []byte("test"), 0o600); err != nil {
		t.Fatalf("write ipa fixture: %v", err)
	}

	requests := newRequestLog(30)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests.Add(req.Method + " " + req.URL.Path)
		if resp, err, ok := respondToPublishAppLookup(t, req); ok {
			return resp, err
		}

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
				t.Fatalf("did not expect publish-state preflight when version is already submitted")
				return nil, nil
			default:
				t.Fatalf("unexpected app store versions query: %s", req.URL.RawQuery)
				return nil, nil
			}
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionSubmission":
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"legacy-sub-1"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			t.Fatalf("did not expect build lookup when version is already submitted")
			return nil, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			t.Fatalf("did not expect build attachment mutation when version is already submitted")
			return nil, nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			t.Fatalf("did not expect localization preflight when version is already submitted")
			return nil, nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			t.Fatalf("did not expect subscription preflight when version is already submitted")
			return nil, nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			t.Fatalf("did not expect review submission reuse lookup when version is already submitted")
			return nil, nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			t.Fatalf("did not expect new review submission creation when version is already submitted")
			return nil, nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			t.Fatalf("did not expect review submission item creation when version is already submitted")
			return nil, nil
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/reviewSubmissions/"):
			t.Fatalf("did not expect review submission submission when version is already submitted")
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
			"--output", "json",
			"--poll-interval", "1ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr == "" {
		t.Fatalf("expected upload progress output on stderr, got empty string")
	}

	var payload struct {
		BuildID      string `json:"buildId"`
		VersionID    string `json:"versionId"`
		SubmissionID string `json:"submissionId"`
		Uploaded     bool   `json:"uploaded"`
		Attached     bool   `json:"attached"`
		Submitted    bool   `json:"submitted"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}

	if payload.BuildID != "build-42" {
		t.Fatalf("expected buildId build-42, got %q", payload.BuildID)
	}
	if payload.VersionID != "version-1" {
		t.Fatalf("expected versionId version-1, got %q", payload.VersionID)
	}
	if payload.SubmissionID != "legacy-sub-1" {
		t.Fatalf("expected submissionId legacy-sub-1, got %q", payload.SubmissionID)
	}
	if !payload.Uploaded {
		t.Fatal("expected uploaded=true")
	}
	if payload.Attached {
		t.Fatalf("expected attached=false when rerun short-circuits before build attachment, got %s", stdout)
	}
	if !payload.Submitted {
		t.Fatalf("expected submitted=true when existing submission is found, got %s", stdout)
	}

	recordedRequests := strings.Join(requests.Snapshot(), "\n")
	if !strings.Contains(recordedRequests, "GET /v1/appStoreVersions/version-1/appStoreVersionSubmission") {
		t.Fatalf("expected existing submission lookup, got requests: %s", recordedRequests)
	}
	if strings.Contains(recordedRequests, "/relationships/build") {
		t.Fatalf("did not expect build mutation requests, got requests: %s", recordedRequests)
	}
}

func TestPublishAppStoreSubmitLocalizationPreflightUsesCanonicalGuidance(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	ipaPath := filepath.Join(t.TempDir(), "app.ipa")
	if err := os.WriteFile(ipaPath, []byte("test"), 0o600); err != nil {
		t.Fatalf("write ipa fixture: %v", err)
	}

	buildAttachmentAttempted := false
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if resp, err, ok := respondToPublishAppLookup(t, req); ok {
			return resp, err
		}

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
			buildAttachmentAttempted = true
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			buildAttachmentAttempted = true
			return jsonResponse(http.StatusNoContent, "")
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionSubmission":
			return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}]}`)
		default:
			t.Fatalf("unexpected request during localization preflight failure: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
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
			"--timeout", "1s",
			"--poll-interval", "1ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil {
		t.Fatal("expected localization preflight error, got nil")
	}
	if !strings.Contains(runErr.Error(), "publish appstore: submit preflight failed") {
		t.Fatalf("expected publish appstore preflight error, got %v", runErr)
	}
	if strings.Contains(runErr.Error(), "submit create:") {
		t.Fatalf("did not expect removed submit create prefix, got %v", runErr)
	}
	if !strings.Contains(stderr, "before retrying `asc publish appstore --submit`") {
		t.Fatalf("expected canonical retry guidance, got %q", stderr)
	}
	if strings.Contains(stderr, "submit create") {
		t.Fatalf("did not expect removed submit create guidance, got %q", stderr)
	}
	if buildAttachmentAttempted {
		t.Fatal("publish appstore --submit must not inspect or mutate build attachment before submission preflight succeeds")
	}
}

func TestPublishAppStoreSubmitDryRunPrintsPlanWithoutMutations(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	ipaPath := filepath.Join(t.TempDir(), "app.ipa")
	if err := os.WriteFile(ipaPath, []byte("test"), 0o600); err != nil {
		t.Fatalf("write ipa fixture: %v", err)
	}

	requests := newRequestLog(10)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests.Add(req.Method + " " + req.URL.Path)
		if resp, err, ok := respondToPublishAppLookup(t, req); ok {
			return resp, err
		}
		t.Fatalf("unexpected HTTP request during dry-run: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
		return nil, nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var stdout string
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "appstore",
			"--app", "app-1",
			"--ipa", ipaPath,
			"--version", "1.2.3",
			"--build-number", "42",
			"--submit",
			"--dry-run",
			"--wait",
			"--poll-interval", "1ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if strings.Contains(stderr, "Uploading ") {
		t.Fatalf("expected dry-run to avoid upload progress output, got %q", stderr)
	}

	var payload struct {
		DryRun       bool   `json:"dryRun"`
		BuildID      string `json:"buildId"`
		BuildVersion string `json:"buildVersion"`
		BuildNumber  string `json:"buildNumber"`
		Uploaded     bool   `json:"uploaded"`
		Attached     bool   `json:"attached"`
		Submitted    bool   `json:"submitted"`
		Plan         []struct {
			Name    string `json:"name"`
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"plan"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if !payload.DryRun {
		t.Fatal("expected dryRun=true")
	}
	if payload.BuildID != "" {
		t.Fatalf("expected empty buildId before upload in dry-run, got %q", payload.BuildID)
	}
	if payload.BuildVersion != "1.2.3" {
		t.Fatalf("expected buildVersion 1.2.3, got %q", payload.BuildVersion)
	}
	if payload.BuildNumber != "42" {
		t.Fatalf("expected buildNumber 42, got %q", payload.BuildNumber)
	}
	if payload.Uploaded || payload.Attached || payload.Submitted {
		t.Fatalf("expected no completed mutations in dry-run, got uploaded=%t attached=%t submitted=%t", payload.Uploaded, payload.Attached, payload.Submitted)
	}

	planNames := make([]string, 0, len(payload.Plan))
	for _, step := range payload.Plan {
		planNames = append(planNames, step.Name)
		if step.Status != "planned" {
			t.Fatalf("expected planned step status, got %q for %q", step.Status, step.Name)
		}
		if strings.TrimSpace(step.Message) == "" {
			t.Fatalf("expected non-empty plan message for %q", step.Name)
		}
	}
	expectedPlanNames := []string{
		"upload_build",
		"wait_for_build_processing",
		"ensure_version",
		"attach_build",
		"submit_review",
	}
	if strings.Join(planNames, ",") != strings.Join(expectedPlanNames, ",") {
		t.Fatalf("expected plan %v, got %v", expectedPlanNames, planNames)
	}

	recordedRequests := requests.Snapshot()
	if len(recordedRequests) != 1 || recordedRequests[0] != "GET /v1/apps" {
		t.Fatalf("expected dry-run lookup-only request log, got %v", recordedRequests)
	}
}

func TestPublishAppStoreDryRunNumericAppIDDoesNotRequireAuth(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	ipaPath := filepath.Join(t.TempDir(), "app.ipa")
	if err := os.WriteFile(ipaPath, []byte("test"), 0o600); err != nil {
		t.Fatalf("write ipa fixture: %v", err)
	}

	requests := newRequestLog(5)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests.Add(req.Method + " " + req.URL.Path)
		t.Fatalf("unexpected HTTP request during authless dry-run: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
		return nil, nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var stdout string
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "appstore",
			"--app", "123456789",
			"--ipa", ipaPath,
			"--version", "1.2.3",
			"--build-number", "42",
			"--submit",
			"--dry-run",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("expected authless dry-run to succeed, got %v", runErr)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected empty stderr for authless dry-run, got %q", stderr)
	}

	var payload struct {
		DryRun       bool   `json:"dryRun"`
		BuildVersion string `json:"buildVersion"`
		BuildNumber  string `json:"buildNumber"`
		Submitted    bool   `json:"submitted"`
		Plan         []struct {
			Name string `json:"name"`
		} `json:"plan"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if !payload.DryRun {
		t.Fatal("expected dryRun=true")
	}
	if payload.BuildVersion != "1.2.3" {
		t.Fatalf("expected buildVersion 1.2.3, got %q", payload.BuildVersion)
	}
	if payload.BuildNumber != "42" {
		t.Fatalf("expected buildNumber 42, got %q", payload.BuildNumber)
	}
	if payload.Submitted {
		t.Fatal("expected submitted=false in dry-run")
	}
	if len(payload.Plan) == 0 {
		t.Fatal("expected non-empty plan in dry-run output")
	}
	if payload.Plan[len(payload.Plan)-1].Name != "submit_review" {
		t.Fatalf("expected last dry-run step to submit review, got %q", payload.Plan[len(payload.Plan)-1].Name)
	}

	if recordedRequests := requests.Snapshot(); len(recordedRequests) != 0 {
		t.Fatalf("expected authless dry-run to avoid all HTTP requests, got %v", recordedRequests)
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
		if resp, err, ok := respondToPublishAppLookup(t, req); ok {
			return resp, err
		}
		if resp, err, ok := respondToFinalReviewSubmissionValidation(req); ok {
			return resp, err
		}

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
			return jsonResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`)
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
		if resp, err, ok := respondToPublishAppLookup(t, req); ok {
			return resp, err
		}
		if resp, err, ok := respondToFinalReviewSubmissionValidation(req); ok {
			return resp, err
		}

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
			return jsonResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`)
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
		if resp, err, ok := respondToPublishAppLookup(t, req); ok {
			return resp, err
		}

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
			return jsonResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`)
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
		if resp, err, ok := respondToPublishAppLookup(t, req); ok {
			return resp, err
		}
		if resp, err, ok := respondToFinalReviewSubmissionValidation(req); ok {
			return resp, err
		}

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
			return jsonResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`)
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
