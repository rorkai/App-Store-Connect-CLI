package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildsLatestResolvesAppByBundleID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("filter[bundleId]") != "com.example.latest" {
				t.Fatalf("expected bundle filter com.example.latest, got %q", query.Get("filter[bundleId]"))
			}
			if query.Get("limit") != "2" {
				t.Fatalf("expected limit=2, got %q", query.Get("limit"))
			}
			body := `{"data":[{"type":"apps","id":"100000001"}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("filter[app]") != "100000001" {
				t.Fatalf("expected filter[app]=100000001, got %q", query.Get("filter[app]"))
			}
			if query.Get("sort") != "-uploadedDate" {
				t.Fatalf("expected sort=-uploadedDate, got %q", query.Get("sort"))
			}
			if query.Get("limit") != "1" {
				t.Fatalf("expected limit=1, got %q", query.Get("limit"))
			}
			body := `{"data":[{"type":"builds","id":"build-1","attributes":{"version":"42"}}]}`
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
		if err := root.Parse([]string{"builds", "latest", "--app", "com.example.latest"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-1"`) {
		t.Fatalf("expected build output, got %q", stdout)
	}
}
