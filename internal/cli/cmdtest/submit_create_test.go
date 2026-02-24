package cmdtest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

type submitCreateRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn submitCreateRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func setupSubmitCreateAuth(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeECDSAPEM(t, keyPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "TEST_KEY")
	t.Setenv("ASC_ISSUER_ID", "TEST_ISSUER")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
}

func submitCreateJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func TestSubmitCreateCancelsStaleSubmissions(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 8)
	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Method + " " + req.URL.Path
		requests = append(requests, key)

		switch {
		// Version resolution
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		// Stale submissions query — returns one stale submission
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			if req.URL.Query().Get("filter[state]") != "READY_FOR_REVIEW" || req.URL.Query().Get("filter[platform]") != "IOS" {
				return nil, fmt.Errorf("unexpected review submissions filters: %s", req.URL.RawQuery)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"stale-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}],"links":{}}`)

		// Cancel stale submission
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/stale-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"stale-1","attributes":{"state":"CANCELING"}}}`)

		// Attach build to version
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		// Create new review submission
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		// Add version as submission item
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		// Submit for review
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-02-22T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	// Verify stale submission was canceled (logged to stderr)
	if !strings.Contains(stderr, "Canceled stale review submission stale-1") {
		t.Errorf("expected stale submission cancel message in stderr, got: %q", stderr)
	}

	// Verify the cancel happened before creating the new submission
	cancelIdx := -1
	createIdx := -1
	for i, req := range requests {
		if req == "PATCH /v1/reviewSubmissions/stale-1" {
			cancelIdx = i
		}
		if req == "POST /v1/reviewSubmissions" {
			createIdx = i
		}
	}
	if cancelIdx == -1 {
		t.Fatal("expected stale submission cancel request")
	}
	if createIdx == -1 {
		t.Fatal("expected new submission create request")
	}
	if cancelIdx >= createIdx {
		t.Fatalf("stale cancel (idx=%d) should happen before new create (idx=%d)", cancelIdx, createIdx)
	}

	// Verify stdout has valid JSON result
	if stdout == "" {
		t.Fatal("expected JSON output on stdout")
	}
}

func TestSubmitCreateNoStaleSubmissions(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 8)
	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Method + " " + req.URL.Path
		requests = append(requests, key)

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		// No stale submissions
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			if req.URL.Query().Get("filter[state]") != "READY_FOR_REVIEW" || req.URL.Query().Get("filter[platform]") != "IOS" {
				return nil, fmt.Errorf("unexpected review submissions filters: %s", req.URL.RawQuery)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-02-22T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	// No stale cancel messages
	if strings.Contains(stderr, "stale") {
		t.Errorf("expected no stale messages, got: %q", stderr)
	}

	if stdout == "" {
		t.Fatal("expected JSON output on stdout")
	}
}

func TestSubmitCreateSkipsNonStaleSubmissionsFromCleanupResults(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 10)
	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Method + " " + req.URL.Path
		requests = append(requests, key)

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		// Return mixed records defensively; cleanup should only cancel READY_FOR_REVIEW + IOS.
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			if req.URL.Query().Get("filter[state]") != "READY_FOR_REVIEW" || req.URL.Query().Get("filter[platform]") != "IOS" {
				return nil, fmt.Errorf("unexpected review submissions filters: %s", req.URL.RawQuery)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"stale-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}},{"type":"reviewSubmissions","id":"active-1","attributes":{"state":"WAITING_FOR_REVIEW","platform":"IOS"}},{"type":"reviewSubmissions","id":"other-platform-1","attributes":{"state":"READY_FOR_REVIEW","platform":"MAC_OS"}}],"links":{}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/stale-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"stale-1","attributes":{"state":"CANCELING"}}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-02-22T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "Canceled stale review submission stale-1") {
		t.Fatalf("expected stale cancel message, got: %q", stderr)
	}
	if strings.Contains(strings.Join(requests, "\n"), "PATCH /v1/reviewSubmissions/active-1") {
		t.Fatalf("did not expect cancel request for non-stale submission, requests: %v", requests)
	}
	if strings.Contains(strings.Join(requests, "\n"), "PATCH /v1/reviewSubmissions/other-platform-1") {
		t.Fatalf("did not expect cancel request for other platform submission, requests: %v", requests)
	}
	if stdout == "" {
		t.Fatal("expected JSON output on stdout")
	}
}

func TestSubmitCreateWarnsWhenStaleSubmissionQueryFails(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			if req.URL.Query().Get("filter[state]") != "READY_FOR_REVIEW" || req.URL.Query().Get("filter[platform]") != "IOS" {
				return nil, fmt.Errorf("unexpected review submissions filters: %s", req.URL.RawQuery)
			}
			return submitCreateJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","title":"Internal Server Error"}]}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-02-22T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "Warning: failed to query stale review submissions") {
		t.Fatalf("expected stale query warning in stderr, got: %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output on stdout")
	}
}

func TestSubmitCreateWarnsWhenStaleSubmissionCancelFails(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 9)
	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Method + " " + req.URL.Path
		requests = append(requests, key)

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			if req.URL.Query().Get("filter[state]") != "READY_FOR_REVIEW" || req.URL.Query().Get("filter[platform]") != "IOS" {
				return nil, fmt.Errorf("unexpected review submissions filters: %s", req.URL.RawQuery)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"stale-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}],"links":{}}`)

		// Cancel fails, but submit create should continue.
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/stale-1":
			return submitCreateJSONResponse(http.StatusBadGateway, `{"errors":[{"status":"502","code":"BAD_GATEWAY","title":"Bad Gateway"}]}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-02-22T00:00:00Z"}}}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-1",
			"--version", "1.0",
			"--build", "build-1",
			"--platform", "IOS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "Warning: failed to cancel stale submission stale-1") {
		t.Fatalf("expected cancel warning in stderr, got: %q", stderr)
	}

	cancelIdx := -1
	createIdx := -1
	for i, req := range requests {
		if req == "PATCH /v1/reviewSubmissions/stale-1" {
			cancelIdx = i
		}
		if req == "POST /v1/reviewSubmissions" {
			createIdx = i
		}
	}
	if cancelIdx == -1 {
		t.Fatal("expected stale submission cancel attempt")
	}
	if createIdx == -1 {
		t.Fatal("expected new submission create request")
	}
	if cancelIdx >= createIdx {
		t.Fatalf("stale cancel attempt (idx=%d) should happen before new create (idx=%d)", cancelIdx, createIdx)
	}

	if stdout == "" {
		t.Fatal("expected JSON output on stdout")
	}
}
