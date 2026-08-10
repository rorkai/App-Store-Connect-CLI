package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type buildExpireAllRequestDeadline struct {
	method    string
	path      string
	remaining time.Duration
}

type buildExpireAllDeadlineTransport struct {
	base http.RoundTripper

	mu       sync.Mutex
	requests []buildExpireAllRequestDeadline
}

func (t *buildExpireAllDeadlineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	deadline, ok := req.Context().Deadline()
	remaining := time.Duration(0)
	if ok {
		remaining = time.Until(deadline)
	}

	t.mu.Lock()
	t.requests = append(t.requests, buildExpireAllRequestDeadline{
		method:    req.Method,
		path:      req.URL.Path,
		remaining: remaining,
	})
	t.mu.Unlock()

	return t.base.RoundTrip(req)
}

func (t *buildExpireAllDeadlineTransport) snapshot() []buildExpireAllRequestDeadline {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]buildExpireAllRequestDeadline(nil), t.requests...)
}

func TestBuildsExpireAllUsesFreshRequestDeadlines(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_TIMEOUT", "500ms")
	t.Setenv("ASC_MAX_RETRIES", "0")

	var requestCount atomic.Int64
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		index := int(requestCount.Add(1) - 1)

		if req.Header.Get("Authorization") == "" {
			t.Errorf("request %d missing authorization header", index+1)
		}

		w.Header().Set("Content-Type", "application/json")
		switch index {
		case 0:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds" {
				t.Errorf("first request = %s %s, want GET /v1/builds", req.Method, req.URL.Path)
			}
			query := req.URL.Query()
			if query.Get("filter[app]") != "app-1" || query.Get("limit") != "200" || query.Get("sort") != "-uploadedDate" {
				t.Errorf("first request query = %q, want app filter, limit 200, and descending upload date", req.URL.RawQuery)
			}
			time.Sleep(100 * time.Millisecond)
			_, _ = io.WriteString(w, `{"data":[{"type":"builds","id":"build-new","attributes":{"version":"2.0","uploadedDate":"2026-01-02T00:00:00Z","expired":false}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/builds?cursor=next"}}`)
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds" || req.URL.Query().Get("cursor") != "next" {
				t.Errorf("second request = %s %s?%s, want paginated GET", req.Method, req.URL.Path, req.URL.RawQuery)
			}
			time.Sleep(100 * time.Millisecond)
			_, _ = io.WriteString(w, `{"data":[{"type":"builds","id":"build-old","attributes":{"version":"1.0","uploadedDate":"2026-01-01T00:00:00Z","expired":false}}],"links":{}}`)
		case 2, 3:
			wantID := "build-new"
			if index == 3 {
				wantID = "build-old"
			}
			if req.Method != http.MethodPatch || req.URL.Path != "/v1/builds/"+wantID {
				t.Errorf("request %d = %s %s, want PATCH /v1/builds/%s", index+1, req.Method, req.URL.Path, wantID)
			}
			var payload struct {
				Data struct {
					Type       string `json:"type"`
					ID         string `json:"id"`
					Attributes struct {
						Expired bool `json:"expired"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Errorf("decode request %d body: %v", index+1, err)
			}
			if payload.Data.Type != "builds" || payload.Data.ID != wantID || !payload.Data.Attributes.Expired {
				t.Errorf("request %d payload = %+v, want expired build %s", index+1, payload.Data, wantID)
			}
			if index == 2 {
				time.Sleep(100 * time.Millisecond)
			}
			_, _ = io.WriteString(w, `{"data":{"type":"builds","id":"`+wantID+`","attributes":{"version":"1.0","uploadedDate":"2026-01-01T00:00:00Z","expired":true}},"links":{}}`)
		default:
			t.Errorf("unexpected request %d: %s %s", index+1, req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)

	transport, ok := server.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("test server transport type = %T, want *http.Transport", server.Client().Transport)
	}
	transport = transport.Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	recorder := &buildExpireAllDeadlineTransport{base: transport}
	originalTransport := http.DefaultTransport
	http.DefaultTransport = recorder
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "expire-all",
			"--app", "app-1",
			"--older-than", "2026-02-01",
			"--confirm",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse command: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run command: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	var result asc.BuildExpireAllResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output %q: %v", stdout, err)
	}
	if result.SelectedCount != 2 || result.ExpiredCount != 2 || len(result.Failures) != 0 {
		t.Fatalf("unexpected summary: %+v", result)
	}
	if len(result.Builds) != 2 || result.Builds[0].ID != "build-new" || result.Builds[1].ID != "build-old" {
		t.Fatalf("unexpected build order: %+v", result.Builds)
	}

	requests := recorder.snapshot()
	wantRequests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/v1/builds"},
		{method: http.MethodGet, path: "/v1/builds"},
		{method: http.MethodPatch, path: "/v1/builds/build-new"},
		{method: http.MethodPatch, path: "/v1/builds/build-old"},
	}
	if len(requests) != len(wantRequests) {
		t.Fatalf("requests = %+v, want %d requests", requests, len(wantRequests))
	}
	for i, want := range wantRequests {
		got := requests[i]
		if got.method != want.method || got.path != want.path {
			t.Errorf("request %d = %s %s, want %s %s", i+1, got.method, got.path, want.method, want.path)
		}
		if got.remaining < 350*time.Millisecond || got.remaining > 500*time.Millisecond {
			t.Errorf("request %d %s %s deadline remaining = %s, want a fresh timeout near 500ms", i+1, got.method, got.path, got.remaining)
		}
	}
}
