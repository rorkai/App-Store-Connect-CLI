package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildsTestNotesUpdateByBuildLocaleNotFound(t *testing.T) {
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
			t.Fatalf("unexpected request count %d", requestCount)
		}
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds/build-1/betaBuildLocalizations" {
			t.Fatalf("expected path /v1/builds/build-1/betaBuildLocalizations, got %s", req.URL.Path)
		}
		query := req.URL.Query()
		if query.Get("filter[locale]") != "en-US" {
			t.Fatalf("expected filter[locale]=en-US, got %q", query.Get("filter[locale]"))
		}
		if query.Get("limit") != "200" {
			t.Fatalf("expected limit=200, got %q", query.Get("limit"))
		}
		body := `{"data":[]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "test-notes", "update",
			"--build", "build-1",
			"--locale", "en-US",
			"--whats-new", "Updated notes",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected localization lookup error, got nil")
	}
	if !strings.Contains(runErr.Error(), `no localization found for build "build-1" and locale "en-US"`) {
		t.Fatalf("expected not-found localization error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on lookup failure, got %q", stdout)
	}
}

func TestBuildsTestNotesUpdateByIDSkipsLookup(t *testing.T) {
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
			t.Fatalf("unexpected request count %d", requestCount)
		}
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaBuildLocalizations/loc-1" {
			t.Fatalf("expected path /v1/betaBuildLocalizations/loc-1, got %s", req.URL.Path)
		}
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		if !strings.Contains(string(payload), `"whatsNew":"Updated notes"`) {
			t.Fatalf("expected whatsNew payload, got %s", string(payload))
		}
		body := `{"data":{"type":"betaBuildLocalizations","id":"loc-1","attributes":{"locale":"en-US","whatsNew":"Updated notes"}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "test-notes", "update",
			"--id", "loc-1",
			"--whats-new", "Updated notes",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"loc-1"`) {
		t.Fatalf("expected updated localization in output, got %q", stdout)
	}
}
