package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type publishTestFlightSubmitOutput struct {
	BuildID                string   `json:"buildId"`
	GroupIDs               []string `json:"groupIds"`
	BetaReviewSubmitted    *bool    `json:"betaReviewSubmitted,omitempty"`
	BetaReviewSubmissionID string   `json:"betaReviewSubmissionId,omitempty"`
}

func TestPublishTestflightSubmitCreatesBetaReviewSubmissionForExternalGroups(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/123/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-external","attributes":{"name":"External QA","isInternalGroup":false}}]}`), nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`), nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-1/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read group assignment payload: %v", err)
			}
			if !strings.Contains(string(payload), `"id":"group-external"`) {
				t.Fatalf("expected group assignment payload to include group-external, got %s", string(payload))
			}
			return jsonHTTPResponse(http.StatusNoContent, ``), nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1/betaAppReviewSubmission" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`), nil
		case 5:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/betaAppReviewSubmissions" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read beta review submission payload: %v", err)
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
			"publish", "testflight",
			"--app", "123",
			"--build", "build-1",
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
		t.Fatalf("expected group lookup, build lookup, group add, submission lookup, and submission create; got %d requests", requestCount)
	}
	result := decodePublishTestFlightSubmitOutput(t, stdout)
	if result.BuildID != "build-1" {
		t.Fatalf("expected build-1 output, got %q", result.BuildID)
	}
	if !slices.Contains(result.GroupIDs, "group-external") {
		t.Fatalf("expected group-external in output, got %#v", result.GroupIDs)
	}
	if result.BetaReviewSubmitted == nil || !*result.BetaReviewSubmitted {
		t.Fatalf("expected betaReviewSubmitted=true, got %#v", result.BetaReviewSubmitted)
	}
	if result.BetaReviewSubmissionID != "submission-1" {
		t.Fatalf("expected submission-1 output, got %q", result.BetaReviewSubmissionID)
	}
	if !strings.Contains(stderr, "Submitted build build-1 for beta app review (submission-1)") {
		t.Fatalf("expected beta review submission message, got %q", stderr)
	}
}

func TestPublishTestflightSubmitSkipsBetaReviewSubmissionForInternalGroups(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/123/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-internal","attributes":{"name":"Friends and Family","isInternalGroup":true}}]}`), nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`), nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-1/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusNoContent, ``), nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "testflight",
			"--app", "123",
			"--build", "build-1",
			"--group", "group-internal",
			"--submit",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if requestCount != 3 {
		t.Fatalf("expected group lookup, build lookup, and group add; got %d requests", requestCount)
	}
	result := decodePublishTestFlightSubmitOutput(t, stdout)
	if result.BetaReviewSubmitted == nil || *result.BetaReviewSubmitted {
		t.Fatalf("expected betaReviewSubmitted=false, got %#v", result.BetaReviewSubmitted)
	}
	if result.BetaReviewSubmissionID != "" {
		t.Fatalf("expected empty beta review submission ID, got %q", result.BetaReviewSubmissionID)
	}
	if !strings.Contains(stderr, "Skipped beta app review submission for build build-1 because no external groups were added") {
		t.Fatalf("expected beta review skip message, got %q", stderr)
	}
}

func TestPublishTestflightSubmitTreatsExistingSubmissionAsAlreadyDone(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/123/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-external","attributes":{"name":"External QA","isInternalGroup":false}}]}`), nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`), nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-1/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusNoContent, ``), nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1/betaAppReviewSubmission" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"betaAppReviewSubmissions","id":"submission-existing"}}`), nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "testflight",
			"--app", "123",
			"--build", "build-1",
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

	if requestCount != 4 {
		t.Fatalf("expected group lookup, build lookup, group add, and submission lookup; got %d requests", requestCount)
	}
	result := decodePublishTestFlightSubmitOutput(t, stdout)
	if result.BetaReviewSubmitted == nil || !*result.BetaReviewSubmitted {
		t.Fatalf("expected betaReviewSubmitted=true, got %#v", result.BetaReviewSubmitted)
	}
	if result.BetaReviewSubmissionID != "submission-existing" {
		t.Fatalf("expected existing submission ID, got %q", result.BetaReviewSubmissionID)
	}
	if !strings.Contains(stderr, "Build build-1 already has beta app review submission submission-existing") {
		t.Fatalf("expected existing submission message, got %q", stderr)
	}
}

func TestPublishTestflightSubmitPreservesPartialSuccessWhenSubmissionFails(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/123/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-external","attributes":{"name":"External QA","isInternalGroup":false}}]}`), nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`), nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-1/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusNoContent, ``), nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1/betaAppReviewSubmission" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`), nil
		case 5:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/betaAppReviewSubmissions" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusServiceUnavailable, `{"errors":[{"status":"503","code":"SERVICE_UNAVAILABLE","title":"Service unavailable","detail":"beta review temporarily unavailable"}]}`), nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"publish", "testflight",
			"--app", "123",
			"--build", "build-1",
			"--group", "group-external",
			"--submit",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(runErr.Error(), `publish testflight: beta groups were added to build "build-1", but beta app review submission failed`) {
		t.Fatalf("expected partial-success submission error, got %v", runErr)
	}
	if strings.Contains(runErr.Error(), "failed to add groups") {
		t.Fatalf("did not expect misleading add-groups wrapper, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "Service unavailable: beta review temporarily unavailable") {
		t.Fatalf("expected underlying submission error, got %v", runErr)
	}
}

func decodePublishTestFlightSubmitOutput(t *testing.T, stdout string) publishTestFlightSubmitOutput {
	t.Helper()

	var result publishTestFlightSubmitOutput
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to decode publish testflight output %q: %v", stdout, err)
	}
	return result
}
