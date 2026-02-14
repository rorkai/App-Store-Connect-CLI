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

	requests := make([]string, 0, 3)
	http.DefaultTransport = submitCancelRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)

		switch {
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
	if result.ID != "legacy-submission-456" || !result.Cancelled {
		t.Fatalf("unexpected result: %+v", result)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-456/appStoreVersionSubmission",
		"PATCH /v1/reviewSubmissions/legacy-submission-456",
		"DELETE /v1/appStoreVersionSubmissions/legacy-submission-456",
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
		if req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-missing/appStoreVersionSubmission" {
			return submitCancelJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"submit", "cancel", "--version-id", "version-missing", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), `no legacy submission found for version "version-missing"`) {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}
