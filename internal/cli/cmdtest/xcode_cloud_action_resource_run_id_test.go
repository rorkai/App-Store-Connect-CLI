package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestXcodeCloudIssuesListResolvesActionIDFromRunID(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciBuildRuns/run-1/actions" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("limit") != "2" {
				t.Fatalf("expected limit=2, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[{"type":"ciBuildActions","id":"act-1","attributes":{"name":"Archive"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciBuildActions/act-1/issues" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"ciIssues","id":"issue-1"}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "issues", "list", "--run-id", "run-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"issue-1"`) {
		t.Fatalf("expected issue output, got %q", stdout)
	}
}

func TestXcodeCloudArtifactsListResolvesActionIDFromRunID(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciBuildRuns/run-1/actions" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("limit") != "2" {
				t.Fatalf("expected limit=2, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[{"type":"ciBuildActions","id":"act-1","attributes":{"name":"Archive"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciBuildActions/act-1/artifacts" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"ciArtifacts","id":"artifact-1"}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "artifacts", "list", "--run-id", "run-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"artifact-1"`) {
		t.Fatalf("expected artifact output, got %q", stdout)
	}
}

func TestXcodeCloudIssuesListRejectsAmbiguousRunID(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/ciBuildRuns/run-1/actions" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		body := `{"data":[{"type":"ciBuildActions","id":"act-1"},{"type":"ciBuildActions","id":"act-2"}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "issues", "list", "--run-id", "run-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "matched multiple build actions") {
		t.Fatalf("expected ambiguous run-id error, got %q", stderr)
	}
	if !strings.Contains(stderr, `asc xcode-cloud actions --run-id "run-1"`) {
		t.Fatalf("expected follow-up guidance, got %q", stderr)
	}
}

func TestXcodeCloudIssuesListRejectsConflictingSelectors(t *testing.T) {
	setupAuth(t)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "issues", "list", "--action-id", "act-1", "--run-id", "run-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--action-id and --run-id are mutually exclusive") {
		t.Fatalf("expected mutually exclusive error, got %q", stderr)
	}
}
