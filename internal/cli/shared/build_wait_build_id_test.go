package shared

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// buildUploadLinkedOnlyWithIncludeResponse mirrors App Store Connect, which
// returns the buildUpload -> build linkage only when include=build is sent.
func buildUploadLinkedOnlyWithIncludeResponse(req *http.Request, buildID string) (*http.Response, error) {
	if req.URL.Query().Get("include") != "build" {
		return buildWaitJSONResponse(`{
			"data": {
				"type": "buildUploads",
				"id": "upload-current",
				"attributes": {
					"cfBundleShortVersionString": "1.2.3",
					"cfBundleVersion": "42",
					"platform": "IOS",
					"state": {"state": "COMPLETE"}
				}
			}
		}`)
	}
	return buildWaitJSONResponse(fmt.Sprintf(`{
		"data": {
			"type": "buildUploads",
			"id": "upload-current",
			"attributes": {
				"cfBundleShortVersionString": "1.2.3",
				"cfBundleVersion": "42",
				"platform": "IOS",
				"state": {"state": "COMPLETE"}
			},
			"relationships": {
				"build": {"data": {"type": "builds", "id": %q}}
			}
		}
	}`, buildID))
}

func TestWaitForBuildByNumberOrUploadFailureRequestsLinkedBuildInclude(t *testing.T) {
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/buildUploads/upload-current":
			return buildUploadLinkedOnlyWithIncludeResponse(req, "build-123")
		case "/v1/builds/build-123":
			return buildWaitJSONResponse(`{"data":{"type":"builds","id":"build-123","attributes":{"version":"42","processingState":"PROCESSING"}}}`)
		default:
			t.Fatalf("did not expect build discovery fallback once the upload links a build: %s", req.URL.String())
			return nil, nil
		}
	})

	buildResp, err := WaitForBuildByNumberOrUploadFailure(context.Background(), client, "app-1", "upload-current", "1.2.3", "42", "IOS", time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForBuildByNumberOrUploadFailure() error: %v", err)
	}
	if buildResp == nil || buildResp.Data.ID != "build-123" {
		t.Fatalf("expected linked build build-123, got %+v", buildResp)
	}
}

func TestVerifyBuildUploadAfterCommitReturnsLinkedBuildID(t *testing.T) {
	lookups := 0
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/buildUploads/upload-current" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		lookups++
		return buildUploadLinkedOnlyWithIncludeResponse(req, "build-123")
	})

	buildID, err := VerifyBuildUploadAfterCommit(context.Background(), client, "app-1", "upload-current", time.Millisecond, time.Second)
	if err != nil {
		t.Fatalf("VerifyBuildUploadAfterCommit() error: %v", err)
	}
	if buildID != "build-123" {
		t.Fatalf("expected linked build ID build-123, got %q", buildID)
	}
	if lookups != 1 {
		t.Fatalf("expected verification to stop once the build links, got %d lookups", lookups)
	}
}

func TestVerifyBuildUploadAfterCommitReturnsEmptyBuildIDWhenWindowEndsUnlinked(t *testing.T) {
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/buildUploads/upload-current" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		return buildWaitJSONResponse(`{"data":{"type":"buildUploads","id":"upload-current","attributes":{"state":{"state":"PROCESSING"}}}}`)
	})

	buildID, err := VerifyBuildUploadAfterCommit(context.Background(), client, "app-1", "upload-current", time.Millisecond, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("VerifyBuildUploadAfterCommit() error: %v", err)
	}
	if buildID != "" {
		t.Fatalf("expected no build ID when the window ends before linkage, got %q", buildID)
	}
}

func TestVerifyBuildUploadAfterCommitFindsBuildBeforeUploadLinksIt(t *testing.T) {
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/buildUploads/upload-current":
			return buildWaitJSONResponse(`{"data":{"type":"buildUploads","id":"upload-current","attributes":{"cfBundleShortVersionString":"1.2.3","cfBundleVersion":"42","platform":"IOS","state":{"state":"PROCESSING"}}}}`)
		case "/v1/builds":
			query := req.URL.Query()
			if query.Get("filter[app]") != "app-1" || query.Get("filter[version]") != "42" || query.Get("include") != "buildUpload" {
				t.Fatalf("unexpected builds lookup query: %s", req.URL.RawQuery)
			}
			return buildWaitJSONResponse(`{"data":[
				{"type":"builds","id":"build-other","attributes":{"version":"42"},"relationships":{"buildUpload":{"data":{"type":"buildUploads","id":"upload-older"}}}},
				{"type":"builds","id":"build-123","attributes":{"version":"42"},"relationships":{"buildUpload":{"data":{"type":"buildUploads","id":"upload-current"}}}}
			]}`)
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	buildID, err := VerifyBuildUploadAfterCommit(context.Background(), client, "app-1", "upload-current", time.Millisecond, time.Second)
	if err != nil {
		t.Fatalf("VerifyBuildUploadAfterCommit() error: %v", err)
	}
	if buildID != "build-123" {
		t.Fatalf("expected build-123 matched by its buildUpload linkage, got %q", buildID)
	}
}

func TestVerifyBuildUploadAfterCommitIgnoresBuildLookupFailures(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/buildUploads/upload-current":
			return buildWaitJSONResponse(`{"data":{"type":"buildUploads","id":"upload-current","attributes":{"cfBundleVersion":"42","state":{"state":"PROCESSING"}}}}`)
		case "/v1/builds":
			return buildWaitJSONStatusResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN_ERROR","title":"forbidden"}]}`)
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	buildID, err := VerifyBuildUploadAfterCommit(context.Background(), client, "app-1", "upload-current", time.Millisecond, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("expected best-effort build lookup failures to be ignored, got %v", err)
	}
	if buildID != "" {
		t.Fatalf("expected no build ID, got %q", buildID)
	}
}
