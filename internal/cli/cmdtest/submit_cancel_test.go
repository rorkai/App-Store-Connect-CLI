package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

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

func TestSubmitCancelByIDUsesReviewSubmissionEndpoint(t *testing.T) {
	setupSubmitCancelAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.PATCH("/v1/reviewSubmissions/review-submission-456").JSONBody(`{"data":{"type":"reviewSubmissions","id":"review-submission-456","attributes":{"canceled":true}}}`), asctest.JSON(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-submission-456"}}`))

	stdout, stderr, err := runCommand(t, []string{"submit", "cancel", "--id", "review-submission-456", "--confirm"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

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
	requests := submitCancelRequests(reg)
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelByVersionIDFallsBackToLegacyDelete(t *testing.T) {
	setupSubmitCancelAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-456", "include=app"), asctest.JSON(http.StatusOK, `{
		"data": {
			"type": "appStoreVersions", "id": "version-456",
			"attributes": {"platform": "IOS", "versionString": "1.0"},
			"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-456/appStoreVersionSubmission"), asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"legacy-submission-456"}}`))
	reg.Register(asctest.PATCH("/v1/reviewSubmissions/legacy-submission-456").JSONBody(`{"data":{"type":"reviewSubmissions","id":"legacy-submission-456","attributes":{"canceled":true}}}`), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.DELETE("/v1/appStoreVersionSubmissions/legacy-submission-456"), asctest.JSON(http.StatusNoContent, ""))

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
	requests := submitCancelRequests(reg)
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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-456", "include=app"), asctest.JSON(http.StatusOK, `{
		"data": {
			"type": "appStoreVersions",
			"id": "version-456",
			"attributes": {"platform": "IOS", "versionString": "1.0"},
			"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{
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
	}`))

	stdout, stderr, err := runCommand(t, []string{"submit", "cancel", "--version-id", "version-456", "--confirm"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

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
	requests := submitCancelRequestURIs(reg)
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelByVersionIDModernConflictDoesNotFallBackToLegacy(t *testing.T) {
	setupSubmitCancelAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-456", "include=app"), asctest.JSON(http.StatusOK, `{
		"data": {
			"type": "appStoreVersions",
			"id": "version-456",
			"attributes": {"platform": "IOS", "versionString": "1.0"},
			"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{
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
	}`))
	reg.Register(asctest.PATCH("/v1/reviewSubmissions/review-submission-456").JSONBody(`{"data":{"type":"reviewSubmissions","id":"review-submission-456","attributes":{"canceled":true}}}`), asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"CONFLICT","title":"Resource state is invalid.","detail":"Resource is not in cancellable state"}]}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-submission-456"), func(*http.Request) (*http.Response, error) {
		return nil, errors.New("unexpected request: GET /v1/reviewSubmissions/review-submission-456")
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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-456", "include=app"), asctest.JSON(http.StatusOK, `{
		"data": {
			"type": "appStoreVersions",
			"id": "version-456",
			"attributes": {"platform": "IOS", "versionString": "1.0"},
			"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{
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
	}`))
	reg.Register(asctest.PATCH("/v1/reviewSubmissions/review-submission-456").JSONBody(`{"data":{"type":"reviewSubmissions","id":"review-submission-456","attributes":{"canceled":true}}}`), asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"CONFLICT","title":"Resource state is invalid.","detail":"Resource is not in cancellable state"}]}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-submission-456"), asctest.JSON(http.StatusOK, `{
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
	}`))

	stdout, stderr, err := runCommand(t, []string{"submit", "cancel", "--version-id", "version-456", "--confirm"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

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
	requests := submitCancelRequestURIs(reg)
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelByVersionIDVersionLookupErrorFallsBackToLegacy(t *testing.T) {
	setupSubmitCancelAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-lookup-error", "include=app"), asctest.JSON(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","title":"Server Error"}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-lookup-error/appStoreVersionSubmission"), asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"legacy-submission-123"}}`))
	reg.Register(asctest.PATCH("/v1/reviewSubmissions/legacy-submission-123").JSONBody(`{"data":{"type":"reviewSubmissions","id":"legacy-submission-123","attributes":{"canceled":true}}}`), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.DELETE("/v1/appStoreVersionSubmissions/legacy-submission-123"), asctest.JSON(http.StatusNoContent, ""))

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
	requests := submitCancelRequestURIs(reg)
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelByVersionIDVersionLookupErrorFallsBackToExplicitApp(t *testing.T) {
	setupSubmitCancelAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-lookup-error", "include=app"), asctest.JSON(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","title":"Server Error"}]}`))
	reg.Register(asctest.GET("/v1/apps/app-123/reviewSubmissions", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{
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
	}`))
	reg.Register(asctest.PATCH("/v1/reviewSubmissions/review-submission-123").JSONBody(`{"data":{"type":"reviewSubmissions","id":"review-submission-123","attributes":{"canceled":true}}}`), asctest.JSON(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-submission-123"}}`))

	stdout, stderr, err := runCommand(t, []string{"submit", "cancel", "--version-id", "version-lookup-error", "--app", "app-123", "--confirm"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

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
	requests := submitCancelRequestURIs(reg)
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelByVersionIDModernLookupErrorFallsBackToLegacy(t *testing.T) {
	setupSubmitCancelAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-123", "include=app"), asctest.JSON(http.StatusOK, `{
		"data": {
			"type": "appStoreVersions",
			"id": "version-123",
			"attributes": {"platform": "IOS", "versionString": "1.0"},
			"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","title":"Server Error"}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-123/appStoreVersionSubmission"), asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"legacy-submission-123"}}`))
	reg.Register(asctest.PATCH("/v1/reviewSubmissions/legacy-submission-123").JSONBody(`{"data":{"type":"reviewSubmissions","id":"legacy-submission-123","attributes":{"canceled":true}}}`), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.DELETE("/v1/appStoreVersionSubmissions/legacy-submission-123"), asctest.JSON(http.StatusNoContent, ""))

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
	requests := submitCancelRequestURIs(reg)
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelByVersionIDMissingLegacySubmissionReturnsClearError(t *testing.T) {
	setupSubmitCancelAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-missing", "include=app"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-missing/appStoreVersionSubmission"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))

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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-forbidden", "include=app"), asctest.JSON(http.StatusOK, `{
		"data": {
			"type": "appStoreVersions",
			"id": "version-forbidden",
			"attributes": {"platform": "IOS", "versionString": "1.0"},
			"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-forbidden/appStoreVersionSubmission"), asctest.JSON(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden"}]}`))

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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-456", "include=app"), asctest.JSON(http.StatusOK, `{
		"data": {
			"type": "appStoreVersions",
			"id": "version-456",
			"attributes": {"platform": "IOS", "versionString": "1.0"},
			"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{
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
	}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-456/appStoreVersionSubmission"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))

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
	requests := submitCancelRequestURIs(reg)
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelByIDExplainsNonCancellableState(t *testing.T) {
	setupSubmitCancelAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.PATCH("/v1/reviewSubmissions/review-submission-456").JSONBody(`{"data":{"type":"reviewSubmissions","id":"review-submission-456","attributes":{"canceled":true}}}`), asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"CONFLICT","title":"Resource state is invalid.","detail":"Resource is not in cancellable state"}]}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-submission-456"), asctest.JSON(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-submission-456","attributes":{"state":"UNRESOLVED_ISSUES"}}}`))

	stdout, _, runErr := runCommand(t, []string{"submit", "cancel", "--id", "review-submission-456", "--confirm"})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	for _, want := range []string{
		"submit cancel: review submission review-submission-456 cannot be canceled in state UNRESOLVED_ISSUES",
		"Resolution Center",
		"Resource is not in cancellable state",
	} {
		if !strings.Contains(runErr.Error(), want) {
			t.Fatalf("error %q does not contain %q", runErr, want)
		}
	}
	wantRequests := []string{
		"PATCH /v1/reviewSubmissions/review-submission-456",
		"GET /v1/reviewSubmissions/review-submission-456",
	}
	requests := submitCancelRequests(reg)
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestSubmitCancelByVersionIDModernConflictExplainsRefreshedState(t *testing.T) {
	setupSubmitCancelAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-456", "include=app"), asctest.JSON(http.StatusOK, `{
		"data": {
			"type": "appStoreVersions",
			"id": "version-456",
			"attributes": {"platform": "IOS", "versionString": "1.0"},
			"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{
		"data": [{
			"type": "reviewSubmissions",
			"id": "review-submission-456",
			"attributes": {"state": "READY_FOR_REVIEW"},
			"relationships": {
				"appStoreVersionForReview": {
					"data": {"type": "appStoreVersions", "id": "version-456"}
				}
			}
		}]
	}`))
	reg.Register(asctest.PATCH("/v1/reviewSubmissions/review-submission-456").JSONBody(`{"data":{"type":"reviewSubmissions","id":"review-submission-456","attributes":{"canceled":true}}}`), asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"CONFLICT","title":"Resource state is invalid.","detail":"Resource is not in cancellable state"}]}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-submission-456"), asctest.JSON(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-submission-456","attributes":{"state":"READY_FOR_REVIEW"}}}`))

	stdout, _, runErr := runCommand(t, []string{"submit", "cancel", "--version-id", "version-456", "--confirm"})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	for _, want := range []string{
		"submit cancel: review submission review-submission-456 cannot be canceled in state READY_FOR_REVIEW",
		"unsubmitted draft",
		"Resource is not in cancellable state",
	} {
		if !strings.Contains(runErr.Error(), want) {
			t.Fatalf("error %q does not contain %q", runErr, want)
		}
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestSubmitCancelByVersionIDLegacyLookupConflictExplainsState(t *testing.T) {
	setupSubmitCancelAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-456", "include=app"), asctest.JSON(http.StatusOK, `{
		"data": {
			"type": "appStoreVersions", "id": "version-456",
			"attributes": {"platform": "IOS", "versionString": "1.0"},
			"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
		}
	}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-456/appStoreVersionSubmission"), asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"submission-456"}}`))
	reg.Register(asctest.PATCH("/v1/reviewSubmissions/submission-456").JSONBody(`{"data":{"type":"reviewSubmissions","id":"submission-456","attributes":{"canceled":true}}}`), asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"CONFLICT","title":"Resource state is invalid.","detail":"Resource is not in cancellable state"}]}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/submission-456"), asctest.JSON(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"submission-456","attributes":{"state":"COMPLETE"}}}`))

	stdout, _, runErr := runCommand(t, []string{"submit", "cancel", "--version-id", "version-456", "--confirm"})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	for _, want := range []string{
		"submit cancel: review submission submission-456 cannot be canceled in state COMPLETE",
		"nothing left to cancel",
		"Resource is not in cancellable state",
	} {
		if !strings.Contains(runErr.Error(), want) {
			t.Fatalf("error %q does not contain %q", runErr, want)
		}
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func submitCancelRequests(reg *asctest.Registry) []string {
	requests := make([]string, 0, len(reg.Requests()))
	for _, req := range reg.Requests() {
		requests = append(requests, req.Method+" "+req.URL.Path)
	}
	return requests
}

func submitCancelRequestURIs(reg *asctest.Registry) []string {
	requests := make([]string, 0, len(reg.Requests()))
	for _, req := range reg.Requests() {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())
	}
	return requests
}
