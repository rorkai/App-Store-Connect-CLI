package web

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebReviewShowPreservesRawDownloadDirectoryError(t *testing.T) {
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })

	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					var body string
					switch req.URL.Path {
					case "/iris/v1/apps/app-1/reviewSubmissions":
						body = `{
							"data": [{
								"id": "sub-1",
								"type": "reviewSubmissions",
								"attributes": {
									"state": "UNRESOLVED_ISSUES",
									"submittedDate": "2026-02-25T00:00:00Z",
									"platform": "IOS"
								}
							}]
						}`
					case "/iris/v1/reviewSubmissions/sub-1/items":
						body = `{"data":[]}`
					case "/iris/v1/apps/app-1/resolutionCenterThreads":
						body = `{"data":[]}`
					case "/iris/v1/resolutionCenterThreads":
						body = `{
							"data": [{
								"id": "thread-1",
								"type": "resolutionCenterThreads",
								"attributes": {
									"threadType": "OPEN",
									"state": "OPEN",
									"createdDate": "2026-02-25T00:00:00Z"
								},
								"relationships": {
									"reviewSubmission": {"data": {"type":"reviewSubmissions","id":"sub-1"}}
								}
							}]
						}`
					case "/iris/v1/resolutionCenterThreads/thread-1/resolutionCenterMessages":
						body = `{
							"data": [{
								"id": "m1",
								"type": "resolutionCenterMessages",
								"attributes": {"createdDate":"2026-02-25T10:00:00Z","messageBody":"<p>Hello</p>"},
								"relationships": {
									"resolutionCenterMessageAttachments": {"data": [{"type":"resolutionCenterMessageAttachments","id":"att-1"}]}
								}
							}],
							"included": [{
								"id":"att-1",
								"type":"resolutionCenterMessageAttachments",
								"attributes":{
									"fileName":"Screenshot-1.png",
									"fileSize":1024,
									"assetDeliveryState":"AVAILABLE",
									"downloadUrl":"https://example.invalid/download/att-1"
								}
							}]
						}`
					case "/iris/v1/reviewRejections":
						body = `{"data":[]}`
					default:
						t.Fatalf("unexpected path: %s", req.URL.Path)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(body)),
						Request:    req,
					}, nil
				}),
			},
		}, "cache", nil
	}

	blocker := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocker, []byte("file"), 0o600); err != nil {
		t.Fatalf("failed to create blocker file: %v", err)
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebReviewShowCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--out", filepath.Join(blocker, "downloads"),
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, _ = runWithWebTestDeps(depsStdout, depsStderr, func() {
		err := cmd.Exec(context.Background(), nil)
		if err == nil {
			t.Fatal("expected review show error")
		}
		if !strings.Contains(err.Error(), "failed to create output directory") {
			t.Fatalf("expected raw output directory error, got %v", err)
		}
		if strings.Contains(err.Error(), "web review show failed:") {
			t.Fatalf("expected download error to remain unwrapped, got %v", err)
		}
		if strings.Contains(err.Error(), "web session is unauthorized or expired") {
			t.Fatalf("expected no auth hint for local download error, got %v", err)
		}
	})
}
