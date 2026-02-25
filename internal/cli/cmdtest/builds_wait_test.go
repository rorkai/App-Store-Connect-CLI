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
)

func TestBuildsWaitByBuildIDPollsUntilValid(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds/build-1" {
			t.Fatalf("expected path /v1/builds/build-1, got %s", req.URL.Path)
		}

		state := "PROCESSING"
		if requestCount >= 2 {
			state = "VALID"
		}
		body := `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"` + state + `","version":"42"}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "wait", "--build", "build-1", "--poll-interval", "1ms", "--timeout", "200ms"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stdout, `"id":"build-1"`) {
		t.Fatalf("expected build output, got %q", stdout)
	}
	if !strings.Contains(stderr, "Waiting for build build-1... (PROCESSING") {
		t.Fatalf("expected processing progress output, got %q", stderr)
	}
	if !strings.Contains(stderr, "Waiting for build build-1... (VALID") {
		t.Fatalf("expected terminal-state progress output, got %q", stderr)
	}
}

func TestBuildsWaitByAppAndBuildNumberResolvesThenWaits(t *testing.T) {
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
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/builds" {
				t.Fatalf("expected path /v1/builds, got %s", req.URL.Path)
			}
			query := req.URL.Query()
			if query.Get("filter[app]") != "123456789" {
				t.Fatalf("expected filter[app]=123456789, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[version]") != "42" {
				t.Fatalf("expected filter[version]=42, got %q", query.Get("filter[version]"))
			}
			if query.Get("filter[preReleaseVersion.platform]") != "IOS" {
				t.Fatalf("expected filter[preReleaseVersion.platform]=IOS, got %q", query.Get("filter[preReleaseVersion.platform]"))
			}
			if query.Get("filter[processingState]") != "PROCESSING,FAILED,INVALID,VALID" {
				t.Fatalf(
					"expected filter[processingState]=PROCESSING,FAILED,INVALID,VALID, got %q",
					query.Get("filter[processingState]"),
				)
			}
			body := `{"data":[{"type":"builds","id":"build-42","attributes":{"processingState":"PROCESSING","version":"42"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/builds/build-42" {
				t.Fatalf("expected path /v1/builds/build-42, got %s", req.URL.Path)
			}
			body := `{"data":{"type":"builds","id":"build-42","attributes":{"processingState":"VALID","version":"42"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "wait",
			"--app", "123456789",
			"--build-number", "42",
			"--platform", "IOS",
			"--poll-interval", "1ms",
			"--timeout", "200ms",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stdout, `"id":"build-42"`) {
		t.Fatalf("expected build output, got %q", stdout)
	}
	if !strings.Contains(stderr, "Waiting for build build-42... (VALID") {
		t.Fatalf("expected wait progress output, got %q", stderr)
	}
}

func TestBuildsWaitFailOnInvalidReturnsError(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds/build-1" {
			t.Fatalf("expected path /v1/builds/build-1, got %s", req.URL.Path)
		}
		body := `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"INVALID","version":"42"}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "wait", "--build", "build-1", "--fail-on-invalid", "--poll-interval", "1ms", "--timeout", "100ms"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected INVALID-state failure error")
	}
	if errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected runtime error, got usage error: %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "build processing failed with state INVALID") {
		t.Fatalf("expected INVALID failure, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on failure, got %q", stdout)
	}
	if !strings.Contains(stderr, "Waiting for build build-1... (INVALID") {
		t.Fatalf("expected progress output on stderr, got %q", stderr)
	}
}
