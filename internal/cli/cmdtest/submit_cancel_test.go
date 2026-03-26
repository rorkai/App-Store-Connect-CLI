package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type submitCancelRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn submitCancelRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func setupSubmitCancelAuth(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeECDSAPEM(t, keyPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "TEST_KEY")
	t.Setenv("ASC_ISSUER_ID", "TEST_ISSUER")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
	t.Setenv("ASC_APP_ID", "")
}

func submitCancelJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func TestSubmitCancelByIDUsesReviewSubmissionEndpoint(t *testing.T) {
	setupSubmitCancelAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 1)
	http.DefaultTransport = submitCancelRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)
		if req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/review-submission-456" {
			return submitCancelJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-submission-456"}}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"submit", "cancel", "--id", "review-submission-456", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result asc.AppStoreVersionSubmissionCancelResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v (stdout=%q)", err, stdout)
	}
	if result.ID != "review-submission-456" || !result.Cancelled {
		t.Fatalf("unexpected result: %+v", result)
	}

	wantRequests := []string{"PATCH /v1/reviewSubmissions/review-submission-456"}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelByVersionIDFallsBackToLegacyDelete(t *testing.T) {
	setupSubmitCancelAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 6)
	http.DefaultTransport = submitCancelRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-456" && req.URL.Query().Get("include") == "app":
			return submitCancelJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions", "id": "version-456",
					"attributes": {"platform": "IOS", "versionString": "1.0"},
					"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
				}
			}`)
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/reviewSubmissions"):
			return submitCancelJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-456/appStoreVersionSubmission":
			return submitCancelJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"legacy-submission-456"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/legacy-submission-456":
			return submitCancelJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/appStoreVersionSubmissions/legacy-submission-456":
			return submitCancelJSONResponse(http.StatusNoContent, "")
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"submit", "cancel", "--version-id", "version-456", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var result asc.AppStoreVersionSubmissionCancelResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v (stdout=%q)", err, stdout)
	}
	if result.ID != "legacy-submission-456" || !result.Cancelled {
		t.Fatalf("unexpected result: %+v", result)
	}

	foundLegacyDelete := false
	for _, r := range requests {
		if r == "DELETE /v1/appStoreVersionSubmissions/legacy-submission-456" {
			foundLegacyDelete = true
		}
	}
	if !foundLegacyDelete {
		t.Fatalf("expected legacy delete fallback; requests: %v", requests)
	}
}

func TestSubmitCancelByVersionIDTreatsCancelingModernSubmissionAsSuccess(t *testing.T) {
	setupSubmitCancelAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 2)
	http.DefaultTransport = submitCancelRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-456" && req.URL.Query().Get("include") == "app":
			return submitCancelJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-456",
					"attributes": {"platform": "IOS", "versionString": "1.0"},
					"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitCancelJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "review-submission-456",
					"attributes": {
						"state": "CANCELING",
						"submittedDate": "2026-03-15T11:00:00Z"
					},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-456"}
						}
					}
				}]
			}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/review-submission-456":
			t.Fatalf("did not expect a second cancel request while submission is already CANCELING")
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-456/appStoreVersionSubmission":
			t.Fatalf("did not expect legacy fallback for a matched CANCELING modern submission")
		}

		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"submit", "cancel", "--version-id", "version-456", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result asc.AppStoreVersionSubmissionCancelResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v (stdout=%q)", err, stdout)
	}
	if result.ID != "review-submission-456" || !result.Cancelled {
		t.Fatalf("unexpected result: %+v", result)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-456?include=app",
		"GET /v1/apps/app-1/reviewSubmissions?include=appStoreVersionForReview&limit=200",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelByVersionIDModernConflictDoesNotFallBackToLegacy(t *testing.T) {
	setupSubmitCancelAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitCancelRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-456" && req.URL.Query().Get("include") == "app":
			return submitCancelJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-456",
					"attributes": {"platform": "IOS", "versionString": "1.0"},
					"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitCancelJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "review-submission-456",
					"attributes": {"state": "WAITING_FOR_REVIEW"},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-456"}
						}
					}
				}]
			}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/review-submission-456":
			return submitCancelJSONResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"CONFLICT","title":"Resource state is invalid.","detail":"Resource is not in cancellable state"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-456/appStoreVersionSubmission":
			t.Fatalf("did not expect legacy fallback after a matched modern submission conflict")
		}

		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"submit", "cancel", "--version-id", "version-456", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), `submission review-submission-456 is no longer cancellable`) {
			t.Fatalf("expected modern non-cancellable error, got %v", err)
		}
		if !strings.Contains(err.Error(), "Resource is not in cancellable state") {
			t.Fatalf("expected original ASC conflict detail to be preserved, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestSubmitCancelByVersionIDModernConflictRefreshesCancelingStateToSuccess(t *testing.T) {
	setupSubmitCancelAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 4)
	http.DefaultTransport = submitCancelRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-456" && req.URL.Query().Get("include") == "app":
			return submitCancelJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-456",
					"attributes": {"platform": "IOS", "versionString": "1.0"},
					"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitCancelJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "review-submission-456",
					"attributes": {"state": "WAITING_FOR_REVIEW"},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-456"}
						}
					}
				}]
			}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/review-submission-456":
			return submitCancelJSONResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"CONFLICT","title":"Resource state is invalid.","detail":"Resource is not in cancellable state"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/review-submission-456":
			return submitCancelJSONResponse(http.StatusOK, `{
				"data": {
					"type": "reviewSubmissions",
					"id": "review-submission-456",
					"attributes": {"state": "CANCELING"},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-456"}
						}
					}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-456/appStoreVersionSubmission":
			t.Fatalf("did not expect legacy fallback after refreshed CANCELING state")
		}

		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"submit", "cancel", "--version-id", "version-456", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result asc.AppStoreVersionSubmissionCancelResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v (stdout=%q)", err, stdout)
	}
	if result.ID != "review-submission-456" || !result.Cancelled {
		t.Fatalf("unexpected result: %+v", result)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-456?include=app",
		"GET /v1/apps/app-1/reviewSubmissions?include=appStoreVersionForReview&limit=200",
		"PATCH /v1/reviewSubmissions/review-submission-456",
		"GET /v1/reviewSubmissions/review-submission-456",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelByVersionIDVersionLookupErrorFallsBackToLegacy(t *testing.T) {
	setupSubmitCancelAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 4)
	http.DefaultTransport = submitCancelRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-lookup-error" && req.URL.Query().Get("include") == "app":
			return submitCancelJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","title":"Server Error"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-lookup-error/appStoreVersionSubmission":
			return submitCancelJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"legacy-submission-123"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/legacy-submission-123":
			return submitCancelJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/appStoreVersionSubmissions/legacy-submission-123":
			return submitCancelJSONResponse(http.StatusNoContent, "")
		}

		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"submit", "cancel", "--version-id", "version-lookup-error", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected legacy fallback success, got %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result asc.AppStoreVersionSubmissionCancelResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v (stdout=%q)", err, stdout)
	}
	if result.ID != "legacy-submission-123" || !result.Cancelled {
		t.Fatalf("unexpected result: %+v", result)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-lookup-error?include=app",
		"GET /v1/appStoreVersions/version-lookup-error/appStoreVersionSubmission",
		"PATCH /v1/reviewSubmissions/legacy-submission-123",
		"DELETE /v1/appStoreVersionSubmissions/legacy-submission-123",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelByVersionIDVersionLookupErrorFallsBackToExplicitApp(t *testing.T) {
	setupSubmitCancelAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 3)
	http.DefaultTransport = submitCancelRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-lookup-error" && req.URL.Query().Get("include") == "app":
			return submitCancelJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","title":"Server Error"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/reviewSubmissions":
			return submitCancelJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "review-submission-123",
					"attributes": {"state": "READY_FOR_REVIEW"},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-lookup-error"}
						}
					}
				}]
			}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/review-submission-123":
			return submitCancelJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-submission-123"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-lookup-error/appStoreVersionSubmission":
			t.Fatalf("did not expect legacy fallback when explicit --app modern lookup succeeds")
		}

		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"submit", "cancel", "--version-id", "version-lookup-error", "--app", "app-123", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result asc.AppStoreVersionSubmissionCancelResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v (stdout=%q)", err, stdout)
	}
	if result.ID != "review-submission-123" || !result.Cancelled {
		t.Fatalf("unexpected result: %+v", result)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-lookup-error?include=app",
		"GET /v1/apps/app-123/reviewSubmissions?include=appStoreVersionForReview&limit=200",
		"PATCH /v1/reviewSubmissions/review-submission-123",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelByVersionIDModernLookupErrorFallsBackToLegacy(t *testing.T) {
	setupSubmitCancelAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 5)
	http.DefaultTransport = submitCancelRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123" && req.URL.Query().Get("include") == "app":
			return submitCancelJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-123",
					"attributes": {"platform": "IOS", "versionString": "1.0"},
					"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitCancelJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","title":"Server Error"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123/appStoreVersionSubmission":
			return submitCancelJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"legacy-submission-123"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/legacy-submission-123":
			return submitCancelJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/appStoreVersionSubmissions/legacy-submission-123":
			return submitCancelJSONResponse(http.StatusNoContent, "")
		}

		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"submit", "cancel", "--version-id", "version-123", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected legacy fallback success, got %v", err)
		}
	})

	var result asc.AppStoreVersionSubmissionCancelResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v (stdout=%q)", err, stdout)
	}
	if result.ID != "legacy-submission-123" || !result.Cancelled {
		t.Fatalf("unexpected result: %+v", result)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-123?include=app",
		"GET /v1/apps/app-1/reviewSubmissions?include=appStoreVersionForReview&limit=200",
		"GET /v1/appStoreVersions/version-123/appStoreVersionSubmission",
		"PATCH /v1/reviewSubmissions/legacy-submission-123",
		"DELETE /v1/appStoreVersionSubmissions/legacy-submission-123",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelByVersionIDMissingLegacySubmissionReturnsClearError(t *testing.T) {
	setupSubmitCancelAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitCancelRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-missing":
			return submitCancelJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-missing/appStoreVersionSubmission":
			return submitCancelJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"submit", "cancel", "--version-id", "version-missing", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), `no active submission found for version "version-missing"`) {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestSubmitCancelByVersionIDLegacyForbiddenSurfacesPermissionError(t *testing.T) {
	setupSubmitCancelAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitCancelRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-forbidden" && req.URL.Query().Get("include") == "app":
			return submitCancelJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-forbidden",
					"attributes": {"platform": "IOS", "versionString": "1.0"},
					"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitCancelJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-forbidden/appStoreVersionSubmission":
			return submitCancelJSONResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden"}]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"submit", "cancel", "--version-id", "version-forbidden", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if strings.Contains(err.Error(), `no active submission found for version "version-forbidden"`) {
			t.Fatalf("expected legacy forbidden error to surface, got %v", err)
		}
		if !strings.Contains(strings.ToUpper(err.Error()), "FORBIDDEN") {
			t.Fatalf("expected forbidden error to be preserved, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestSubmitCancelByVersionIDIgnoresHistoricalCompleteReviewSubmission(t *testing.T) {
	setupSubmitCancelAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 3)
	http.DefaultTransport = submitCancelRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-456" && req.URL.Query().Get("include") == "app":
			return submitCancelJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-456",
					"attributes": {"platform": "IOS", "versionString": "1.0"},
					"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitCancelJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "historical-submission",
					"attributes": {
						"state": "COMPLETE",
						"submittedDate": "2026-03-15T11:00:00Z"
					},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-456"}
						}
					}
				}]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-456/appStoreVersionSubmission":
			return submitCancelJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"submit", "cancel", "--version-id", "version-456", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), `no active submission found for version "version-456"`) {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-456?include=app",
		"GET /v1/apps/app-1/reviewSubmissions?include=appStoreVersionForReview&limit=200",
		"GET /v1/appStoreVersions/version-456/appStoreVersionSubmission",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}
