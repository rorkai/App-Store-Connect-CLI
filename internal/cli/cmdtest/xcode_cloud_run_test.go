package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestXcodeCloudRunWithPullRequestIDPostsExpectedPayload(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if requestCount != 1 {
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
		}

		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciBuildRuns" {
			t.Fatalf("expected path /v1/ciBuildRuns, got %s", req.URL.Path)
		}

		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		data, ok := payload["data"].(map[string]any)
		if !ok {
			t.Fatalf("expected data object in payload, got %#v", payload["data"])
		}
		relationships, ok := data["relationships"].(map[string]any)
		if !ok {
			t.Fatalf("expected relationships object in payload, got %#v", data["relationships"])
		}

		if _, ok := relationships["workflow"]; !ok {
			t.Fatalf("expected workflow relationship in payload, got %#v", relationships)
		}
		if _, ok := relationships["pullRequest"]; !ok {
			t.Fatalf("expected pullRequest relationship in payload, got %#v", relationships)
		}
		if _, ok := relationships["sourceBranchOrTag"]; ok {
			t.Fatalf("did not expect sourceBranchOrTag relationship in payload, got %#v", relationships)
		}
		if _, ok := relationships["buildRun"]; ok {
			t.Fatalf("did not expect buildRun relationship in payload, got %#v", relationships)
		}
		if _, ok := data["attributes"]; ok {
			t.Fatalf("did not expect attributes in payload, got %#v", data["attributes"])
		}

		body := `{"data":{"type":"ciBuildRuns","id":"run-pr-1","attributes":{"number":1,"executionProgress":"PENDING","startReason":"MANUAL","createdDate":"2026-03-05T10:00:00Z"}}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "run", "--workflow-id", "wf-1", "--pull-request-id", "pr-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"buildRunId":"run-pr-1"`) {
		t.Fatalf("expected build run ID in output, got %q", stdout)
	}
	if requestCount != 1 {
		t.Fatalf("expected exactly one request, got %d", requestCount)
	}
}

func TestXcodeCloudRunWithSourceRunAndCleanPostsExpectedPayload(t *testing.T) {
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
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/ciBuildRuns/run-1" {
				t.Fatalf("expected path /v1/ciBuildRuns/run-1, got %s", req.URL.Path)
			}
			values := req.URL.Query()
			if values.Get("include") != "workflow" {
				t.Fatalf("expected include=workflow, got %q", values.Get("include"))
			}
			if values.Get("fields[ciBuildRuns]") != "workflow" {
				t.Fatalf("expected fields[ciBuildRuns]=workflow, got %q", values.Get("fields[ciBuildRuns]"))
			}
			body := `{"data":{"type":"ciBuildRuns","id":"run-1","relationships":{"workflow":{"data":{"type":"ciWorkflows","id":"wf-1"}}}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.URL.Path != "/v1/ciBuildRuns" {
				t.Fatalf("expected path /v1/ciBuildRuns, got %s", req.URL.Path)
			}

			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("failed to decode request body: %v", err)
			}

			data, ok := payload["data"].(map[string]any)
			if !ok {
				t.Fatalf("expected data object in payload, got %#v", payload["data"])
			}
			attributes, ok := data["attributes"].(map[string]any)
			if !ok {
				t.Fatalf("expected attributes object in payload, got %#v", data["attributes"])
			}
			clean, ok := attributes["clean"].(bool)
			if !ok || !clean {
				t.Fatalf("expected attributes.clean=true in payload, got %#v", attributes["clean"])
			}

			relationships, ok := data["relationships"].(map[string]any)
			if !ok {
				t.Fatalf("expected relationships object in payload, got %#v", data["relationships"])
			}
			if _, ok := relationships["buildRun"]; !ok {
				t.Fatalf("expected buildRun relationship in payload, got %#v", relationships)
			}
			if _, ok := relationships["workflow"]; !ok {
				t.Fatalf("expected workflow relationship in payload, got %#v", relationships)
			}
			if _, ok := relationships["sourceBranchOrTag"]; ok {
				t.Fatalf("did not expect sourceBranchOrTag relationship in payload, got %#v", relationships)
			}
			if _, ok := relationships["pullRequest"]; ok {
				t.Fatalf("did not expect pullRequest relationship in payload, got %#v", relationships)
			}

			body := `{"data":{"type":"ciBuildRuns","id":"run-rerun-1","attributes":{"number":2,"executionProgress":"PENDING","startReason":"MANUAL","createdDate":"2026-03-05T10:00:00Z"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
		}
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "run", "--source-run-id", "run-1", "--clean"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"buildRunId":"run-rerun-1"`) {
		t.Fatalf("expected build run ID in output, got %q", stdout)
	}
	if requestCount != 2 {
		t.Fatalf("expected exactly two requests, got %d", requestCount)
	}
}

func TestXcodeCloudBuildRunsGetFetchesBuildRun(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if requestCount != 1 {
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
		}

		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciBuildRuns/run-1" {
			t.Fatalf("expected path /v1/ciBuildRuns/run-1, got %s", req.URL.Path)
		}

		body := `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"number":42,"executionProgress":"RUNNING"}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "build-runs", "view", "--id", "run-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"run-1"`) {
		t.Fatalf("expected run ID in output, got %q", stdout)
	}
	if requestCount != 1 {
		t.Fatalf("expected exactly one request, got %d", requestCount)
	}
}
