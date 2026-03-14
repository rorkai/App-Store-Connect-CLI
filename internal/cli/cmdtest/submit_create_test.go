package cmdtest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

		// Localization preflight
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

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

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

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

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

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

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

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

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

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

func TestSubmitCreatePreflightBlocksWhenRequiredLocalizationFieldsAreMissing(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 4)
	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Method + " " + req.URL.Path
		requests = append(requests, key)

		switch {
		// Version resolution.
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		// Submit preflight localizations check.
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR","whatsNew":"Nouveautes"}}]}`)

		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
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
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected preflight error for submit-incomplete localizations")
	}
	if !strings.Contains(runErr.Error(), "submit preflight failed") {
		t.Fatalf("expected preflight error, got: %v", runErr)
	}
	if !strings.Contains(stderr, "fr-FR") || !strings.Contains(stderr, "description") || !strings.Contains(stderr, "keywords") || !strings.Contains(stderr, "supportUrl") {
		t.Fatalf("expected per-locale missing fields summary in stderr, got: %q", stderr)
	}
	if strings.Contains(strings.Join(requests, "\n"), "PATCH /v1/appStoreVersions/version-1/relationships/build") {
		t.Fatalf("did not expect build attach request after preflight failure, requests: %v", requests)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on preflight failure, got: %q", stdout)
	}
}

func TestSubmitCreateWarnsForSubscriptionPreflightStates(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}}],"links":{}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-ready","attributes":{"name":"Monthly Ready","productId":"com.example.ready","state":"READY_TO_SUBMIT"}},{"type":"subscriptions","id":"sub-missing","attributes":{"name":"Monthly Missing","productId":"com.example.missing","state":"MISSING_METADATA"}}],"links":{}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
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

	if !strings.Contains(stderr, "Warning: the following subscriptions are MISSING_METADATA") {
		t.Fatalf("expected missing metadata warning, got %q", stderr)
	}
	if !strings.Contains(stderr, "Monthly Missing") {
		t.Fatalf("expected missing metadata subscription name, got %q", stderr)
	}
	if !strings.Contains(stderr, "Run `asc validate subscriptions` for details on what's missing.") {
		t.Fatalf("expected validate subscriptions guidance, got %q", stderr)
	}
	if !strings.Contains(stderr, "Warning: the following subscriptions are READY_TO_SUBMIT") {
		t.Fatalf("expected ready-to-submit warning, got %q", stderr)
	}
	if !strings.Contains(stderr, "Monthly Ready") {
		t.Fatalf("expected ready-to-submit subscription name, got %q", stderr)
	}
	if !strings.Contains(stderr, "asc subscriptions review submit --subscription-id \"SUB_ID\" --confirm") {
		t.Fatalf("expected corrected submit command guidance, got %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output on stdout")
	}
}

func TestSubmitCreateSubscriptionPreflightPaginatesAndReportsSkippedGroups(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups" && req.URL.RawQuery == "limit=200":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/subscriptionGroups?cursor=groups-2&limit=200"}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups" && req.URL.RawQuery == "cursor=groups-2&limit=200":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-2","attributes":{"referenceName":"Family"}}],"links":{}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions" && req.URL.RawQuery == "limit=200":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-ready","attributes":{"name":"Monthly Ready","productId":"com.example.ready","state":"READY_TO_SUBMIT"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-1/subscriptions?cursor=subs-2&limit=200"}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions" && req.URL.RawQuery == "cursor=subs-2&limit=200":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-missing","attributes":{"name":"Monthly Missing","productId":"com.example.missing","state":"MISSING_METADATA"}}],"links":{}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-2/subscriptions":
			return submitCreateJSONResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden","detail":"not allowed"}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
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
			return nil, fmt.Errorf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
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

	if !strings.Contains(stderr, "Monthly Ready") || !strings.Contains(stderr, "Monthly Missing") {
		t.Fatalf("expected paginated subscription states in stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "Family") || !strings.Contains(stderr, "could not be fully checked") {
		t.Fatalf("expected skipped group warning in stderr, got %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output on stdout")
	}
}

func TestSubmitCreateSubscriptionPreflightDoesNotConsumeSubmitTimeoutBudget(t *testing.T) {
	setupSubmitCreateAuth(t)
	t.Setenv("ASC_TIMEOUT", "100ms")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			if err := sleepWithContext(req.Context()); err != nil {
				return nil, err
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}}],"links":{}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions":
			if err := sleepWithContext(req.Context()); err != nil {
				return nil, err
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
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
		t.Fatalf("expected submit create to succeed with fresh timeout budget, got %v", err)
	}
}

func TestSubmitCreateLocalizationPreflightDoesNotConsumeSubmitTimeoutBudget(t *testing.T) {
	setupSubmitCreateAuth(t)
	t.Setenv("ASC_TIMEOUT", "100ms")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			if strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE") {
				if err := sleepWithContext(req.Context()); err != nil {
					return nil, err
				}
				return submitCreateJSONResponse(http.StatusOK, `{"data":[]}`)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			if err := sleepWithContext(req.Context()); err != nil {
				return nil, err
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support","whatsNew":""}}]}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
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
		t.Fatalf("expected submit create to succeed with fresh localization timeout budget, got %v", err)
	}
}

func TestSubmitCreateRecoversFromAlreadyAddedError(t *testing.T) {
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
		// Version resolution + isAppUpdate check
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			if strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE") {
				return submitCreateJSONResponse(http.StatusOK, `{"data":[]}`)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		// Localization preflight
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Desc","keywords":"kw","supportUrl":"https://example.com","whatsNew":"Bug fixes"}}]}`)

		// Subscription preflight
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		// No stale submissions
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		// Attach build to version
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		// Create new review submission
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		// Add version fails with "already added" error
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return submitCreateJSONResponse(http.StatusConflict, `{
				"errors": [{
					"status": "409",
					"code": "ENTITY_ERROR",
					"title": "The request entity is not valid.",
					"detail": "An attribute value is not valid.",
					"meta": {
						"associatedErrors": {
							"/v1/reviewSubmissionItems": [{
								"code": "ENTITY_ERROR.RELATIONSHIP.INVALID",
								"detail": "appStoreVersions with id 883340862 was already added to another reviewSubmission with id fb5dad8e-bd5f-4d96-bc2f-561cf74a7e7a"
							}]
						}
					}
				}]
			}`)

		// Cancel the empty new submission we created
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"CANCELING"}}}`)

		// Submit the existing submission for review
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/fb5dad8e-bd5f-4d96-bc2f-561cf74a7e7a":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"fb5dad8e-bd5f-4d96-bc2f-561cf74a7e7a","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-03-13T00:00:00Z"}}}`)

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

	// Verify recovery message was logged
	if !strings.Contains(stderr, "Version already in review submission") {
		t.Errorf("expected recovery message in stderr, got: %q", stderr)
	}

	// Verify the existing submission was submitted (not the new one)
	foundExistingSubmit := false
	for _, req := range requests {
		if req == "PATCH /v1/reviewSubmissions/fb5dad8e-bd5f-4d96-bc2f-561cf74a7e7a" {
			foundExistingSubmit = true
		}
	}
	if !foundExistingSubmit {
		t.Fatal("expected existing submission to be submitted for review")
	}

	// Verify stdout has valid JSON result
	if stdout == "" {
		t.Fatal("expected JSON output on stdout")
	}
	if !strings.Contains(stdout, "fb5dad8e-bd5f-4d96-bc2f-561cf74a7e7a") {
		t.Errorf("expected output to reference existing submission ID, got: %q", stdout)
	}
}

func TestSubmitCreateRetriesWhenConflictPointsToRecentlyCanceledStaleSubmission(t *testing.T) {
	setupSubmitCreateAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 12)
	addItemAttempts := 0
	http.DefaultTransport = submitCreateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Method + " " + req.URL.Path
		requests = append(requests, key)

		switch {
		// Version resolution + isAppUpdate check
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			if strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE") {
				return submitCreateJSONResponse(http.StatusOK, `{"data":[]}`)
			}
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}}]}`)

		// Localization preflight
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Desc","keywords":"kw","supportUrl":"https://example.com","whatsNew":"Bug fixes"}}]}`)

		// Subscription preflight
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/subscriptionGroups":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)

		// One stale submission gets canceled before the new submission is created.
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"stale-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}],"links":{}}`)

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/stale-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"stale-1","attributes":{"state":"CANCELING"}}}`)

		// Attach build to version
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			return submitCreateJSONResponse(http.StatusNoContent, "")

		// Create new review submission
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)

		// The first add still points at the stale submission we just canceled.
		// The retry succeeds once App Store Connect catches up.
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			addItemAttempts++
			if addItemAttempts == 1 {
				return submitCreateJSONResponse(http.StatusConflict, `{
					"errors": [{
						"status": "409",
						"code": "ENTITY_ERROR",
						"title": "The request entity is not valid.",
						"detail": "An attribute value is not valid.",
						"meta": {
							"associatedErrors": {
								"/v1/reviewSubmissionItems": [{
									"code": "ENTITY_ERROR.RELATIONSHIP.INVALID",
									"detail": "appStoreVersions with id version-1 was already added to another reviewSubmission with id stale-1"
								}]
							}
						}
					}]
				}`)
			}
			return submitCreateJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`)

		// The newly created submission is the one that gets submitted.
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/new-sub-1":
			return submitCreateJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"new-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-03-14T00:00:00Z"}}}`)

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

	if addItemAttempts != 2 {
		t.Fatalf("expected 2 add-item attempts, got %d", addItemAttempts)
	}
	if !strings.Contains(stderr, "Version is still detaching from recently canceled review submission stale-1") {
		t.Fatalf("expected stale-detaching retry message in stderr, got: %q", stderr)
	}

	stalePatchCount := 0
	newSubmissionPatchCount := 0
	addItemCount := 0
	for _, req := range requests {
		switch req {
		case "PATCH /v1/reviewSubmissions/stale-1":
			stalePatchCount++
		case "PATCH /v1/reviewSubmissions/new-sub-1":
			newSubmissionPatchCount++
		case "POST /v1/reviewSubmissionItems":
			addItemCount++
		}
	}
	if stalePatchCount != 1 {
		t.Fatalf("expected exactly one PATCH to stale submission for cancel, got %d in %v", stalePatchCount, requests)
	}
	if newSubmissionPatchCount != 1 {
		t.Fatalf("expected exactly one PATCH to new submission for submit, got %d in %v", newSubmissionPatchCount, requests)
	}
	if addItemCount != 2 {
		t.Fatalf("expected exactly two add-item requests, got %d in %v", addItemCount, requests)
	}

	if !strings.Contains(stdout, "new-sub-1") {
		t.Fatalf("expected output to reference new submission ID, got: %q", stdout)
	}
}

func sleepWithContext(ctx context.Context) error {
	timer := time.NewTimer(70 * time.Millisecond)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
