package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestBuildsBetaAppReviewSubmissionViewReturnsNotFoundWhenAPIDataIsNull(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1/betaAppReviewSubmission" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return jsonHTTPResponse(http.StatusOK, `{"data":null,"links":{"self":"https://api.appstoreconnect.apple.com/v1/builds/build-1/betaAppReviewSubmission"}}`), nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "beta-app-review-submission", "view", "--build-id", "build-1", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected not-found error")
	}
	if errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected runtime not-found error, got usage error: %v", runErr)
	}
	if !errors.Is(runErr, asc.ErrNotFound) {
		t.Fatalf("expected asc.ErrNotFound, got %v", runErr)
	}
	if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitNotFound {
		t.Fatalf("expected exit code %d, got %d", cmd.ExitNotFound, got)
	}
	if !strings.Contains(runErr.Error(), `builds beta-app-review-submission view: no beta app review submission found for build "build-1"`) {
		t.Fatalf("expected not-found message, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestBuildsAddGroupsSubmitCreatesBetaReviewSubmissionWhenLookupDataIsNull(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1/app" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`), nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-external","attributes":{"name":"External QA","isInternalGroup":false}}]}`), nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-1/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusNoContent, ``), nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1/betaAppReviewSubmission" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":null,"links":{"self":"https://api.appstoreconnect.apple.com/v1/builds/build-1/betaAppReviewSubmission"}}`), nil
		case 5:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/betaAppReviewSubmissions" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}
			bodyText := string(payload)
			if !strings.Contains(bodyText, `"type":"betaAppReviewSubmissions"`) || !strings.Contains(bodyText, `"id":"build-1"`) {
				t.Fatalf("expected beta review submission payload for build-1, got %s", bodyText)
			}
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"betaAppReviewSubmissions","id":"submission-1"}}`), nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "add-groups",
			"--build-id", "build-1",
			"--group", "group-external",
			"--submit",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if requestCount != 5 {
		t.Fatalf("expected app lookup, group lookup, add request, submission lookup, and submission create; got %d requests", requestCount)
	}
	if !strings.Contains(stdout, `"groupIds":["group-external"]`) {
		t.Fatalf("expected external group in output, got %q", stdout)
	}
	if !strings.Contains(stderr, "Successfully added 1 group(s) to build build-1") {
		t.Fatalf("expected add-groups success message, got %q", stderr)
	}
	if !strings.Contains(stderr, "Submitted build build-1 for beta app review (submission-1)") {
		t.Fatalf("expected beta review submission message, got %q", stderr)
	}
}

func TestBuildsBetaAppReviewSubmissionViewPreservesAPI404Context(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-404/betaAppReviewSubmission" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return jsonHTTPResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found","detail":"build not found"}]}`), nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "beta-app-review-submission", "view", "--build-id", "build-404", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected not-found error")
	}
	if !errors.Is(runErr, asc.ErrNotFound) {
		t.Fatalf("expected asc.ErrNotFound, got %v", runErr)
	}
	if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitNotFound {
		t.Fatalf("expected exit code %d, got %d", cmd.ExitNotFound, got)
	}
	if strings.Contains(runErr.Error(), "no beta app review submission found") {
		t.Fatalf("expected upstream 404 context, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "builds beta-app-review-submission view: failed to fetch: Not Found: build not found") {
		t.Fatalf("expected API 404 context, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}
