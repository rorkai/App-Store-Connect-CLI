package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildsDsymsRejectsMissingSelectorBeforeAuth(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "dsyms"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --build or --app is required (or set ASC_APP_ID)") {
		t.Fatalf("expected missing selector usage error, got %q", stderr)
	}
	if strings.Contains(stderr, "missing authentication") {
		t.Fatalf("expected selector validation before auth resolution, got %q", stderr)
	}
}

func TestBuildsDsymsBuildSelectorIgnoresDefaultAppID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "default-app")

	outputDir := filepath.Join(t.TempDir(), "dsyms")
	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1" && req.URL.RawQuery == "":
			body := `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1" && req.URL.RawQuery == "include=buildBundles":
			body := `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42"}},"included":[{"type":"buildBundles","id":"bundle-1","attributes":{"bundleId":"com.example.app","dSYMUrl":"https://downloads.example.com/app.dSYM.zip"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && req.URL.Host == "downloads.example.com" && req.URL.Path == "/app.dSYM.zip":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("zipdata")),
				Header:     http.Header{"Content-Type": []string{"application/zip"}},
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "dsyms", "--build", "build-1", "--output-dir", outputDir}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if strings.Contains(stderr, "--build cannot be combined") {
		t.Fatalf("expected default ASC_APP_ID to be ignored for --build, got %q", stderr)
	}
	if !strings.Contains(stderr, "Resolved build build-1") {
		t.Fatalf("expected resolved build message, got %q", stderr)
	}
	if !strings.Contains(stdout, `"buildId":"build-1"`) {
		t.Fatalf("expected JSON output for build-1, got %q", stdout)
	}

	downloadedPath := filepath.Join(outputDir, "com.example.app-42.dSYM.zip")
	data, err := os.ReadFile(downloadedPath)
	if err != nil {
		t.Fatalf("expected dSYM file to be written: %v", err)
	}
	if string(data) != "zipdata" {
		t.Fatalf("expected downloaded dSYM contents, got %q", string(data))
	}
	if requestCount != 3 {
		t.Fatalf("expected 3 requests (build, build bundles, download), got %d", requestCount)
	}
}
