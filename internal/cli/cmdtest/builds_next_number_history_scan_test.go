package cmdtest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	// buildHistoryScanTimeout stands in for asc.DefaultTimeout so a slow
	// multi-page scan can be simulated without a 30 second test.
	buildHistoryScanTimeout = 800 * time.Millisecond
	// buildHistoryScanDelay makes every page slow enough that two pages
	// together exceed a single request deadline, while leaving each single
	// page a wide margin inside one refreshed deadline.
	buildHistoryScanDelay = 450 * time.Millisecond
)

type buildHistoryScanRequest struct {
	path        string
	cursor      string
	hasDeadline bool
	remaining   time.Duration
}

type buildHistoryScanRecorder struct {
	base http.RoundTripper

	mu       sync.Mutex
	requests []buildHistoryScanRequest
}

func (r *buildHistoryScanRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	deadline, hasDeadline := req.Context().Deadline()
	remaining := time.Duration(0)
	if hasDeadline {
		remaining = time.Until(deadline)
	}

	r.mu.Lock()
	r.requests = append(r.requests, buildHistoryScanRequest{
		path:        req.URL.EscapedPath(),
		cursor:      req.URL.Query().Get("cursor"),
		hasDeadline: hasDeadline,
		remaining:   remaining,
	})
	r.mu.Unlock()

	return r.base.RoundTrip(req)
}

func (r *buildHistoryScanRecorder) snapshot() []buildHistoryScanRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]buildHistoryScanRequest(nil), r.requests...)
}

// setBuildHistoryScanTestClient serves a paginated build history where every
// response is slow, so the command only succeeds when each outbound request
// receives its own deadline instead of sharing one command-wide deadline.
func setBuildHistoryScanTestClient(t *testing.T, pages map[string]string) *buildHistoryScanRecorder {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_TIMEOUT", buildHistoryScanTimeout.String())
	t.Setenv("ASC_TIMEOUT_SECONDS", "")
	t.Setenv("ASC_MAX_RETRIES", "0")

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Errorf("request method = %s, want GET", req.Method)
		}

		body := ""
		switch {
		case req.URL.EscapedPath() == "/v1/apps/100000001/buildUploads":
			body = `{"data":[],"links":{"next":""}}`
		case req.URL.EscapedPath() == "/v1/builds":
			cursor := req.URL.Query().Get("cursor")
			page, ok := pages[cursor]
			if !ok {
				t.Errorf("unexpected builds cursor %q", cursor)
				http.Error(w, "unexpected cursor", http.StatusInternalServerError)
				return
			}
			body = page
		default:
			t.Errorf("unexpected request path %q", req.URL.EscapedPath())
			http.Error(w, "unexpected path", http.StatusInternalServerError)
			return
		}

		time.Sleep(buildHistoryScanDelay)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	})

	rewrite := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.Scheme + "://" + req.URL.Host; got != asc.BaseURL {
			t.Errorf("request origin = %s, want %s", got, asc.BaseURL)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response.Result(), nil
	})
	recorder := &buildHistoryScanRecorder{base: rewrite}

	client, err := asc.NewClientWithHTTPClient(
		"TEST_KEY",
		"TEST_ISSUER",
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: recorder},
	)
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))

	return recorder
}

func requireFreshBuildHistoryScanDeadlines(t *testing.T, recorder *buildHistoryScanRecorder, wantCursors []string) {
	t.Helper()

	requests := recorder.snapshot()
	cursors := make([]string, 0, len(requests))
	for i, request := range requests {
		if !request.hasDeadline {
			t.Fatalf("request %d (%s cursor=%q) had no deadline, want a per-request deadline", i+1, request.path, request.cursor)
		}
		if request.remaining != buildHistoryScanTimeout {
			t.Fatalf(
				"request %d (%s cursor=%q) remaining deadline = %s, want the full %s of a refreshed deadline",
				i+1, request.path, request.cursor, request.remaining, buildHistoryScanTimeout,
			)
		}
		if request.path == "/v1/builds" {
			cursors = append(cursors, request.cursor)
		}
	}
	if strings.Join(cursors, ",") != strings.Join(wantCursors, ",") {
		t.Fatalf("build page cursors = %v, want %v", cursors, wantCursors)
	}
}

func requireBuildHistoryScanWarnings(t *testing.T, stderr string, want []string) {
	t.Helper()

	got := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	if stderr == "" {
		got = nil
	}
	if len(got) != len(want) {
		t.Fatalf("stderr lines = %d (%q), want %d warnings (%q)", len(got), stderr, len(want), want)
	}
	for _, wantLine := range want {
		matches := 0
		for _, gotLine := range got {
			if gotLine == wantLine {
				matches++
			}
		}
		if matches != 1 {
			t.Fatalf("stderr contained %d copies of %q, want exactly 1; stderr=%q", matches, wantLine, stderr)
		}
	}
}

func TestBuildsNextBuildNumberRefreshesDeadlinesAcrossHistoryPages(t *testing.T) {
	const page2 = "https://api.appstoreconnect.apple.com/v1/builds?cursor=page-2"
	const page3 = "https://api.appstoreconnect.apple.com/v1/builds?cursor=page-3"
	recorder := setBuildHistoryScanTestClient(t, map[string]string{
		"":       `{"data":[{"type":"builds","id":"build-new-50","attributes":{"version":"50","uploadedDate":"2026-02-03T00:00:00Z"}}],"links":{"next":"` + page2 + `"}}`,
		"page-2": `{"data":[{"type":"builds","id":"build-old-40","attributes":{"version":"40","uploadedDate":"2026-02-02T00:00:00Z"}}],"links":{"next":"` + page3 + `"}}`,
		"page-3": `{"data":[{"type":"builds","id":"build-old-100","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}],"links":{"next":""}}`,
	})

	root := RootCommand("test")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "next-build-number", "--app", "100000001", "--output", "json"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		synctest.Test(t, func(*testing.T) { runErr = root.Run(context.Background()) })
	})
	if runErr != nil {
		t.Fatalf("run: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	output := decodeProcessedMaximumOutput(t, stdout)
	requireProcessedMaximumValue(t, "latestProcessedBuildNumber", output.LatestProcessedBuildNumber, "50")
	requireProcessedMaximumValue(t, "latestObservedBuildNumber", output.LatestObservedBuildNumber, "100")
	if output.NextBuildNumber != "101" {
		t.Fatalf("nextBuildNumber = %q, want 101", output.NextBuildNumber)
	}

	requireFreshBuildHistoryScanDeadlines(t, recorder, []string{"", "page-2", "", "page-2", "page-3"})
}

func TestBuildsNextBuildNumberSkipsUnparseableHistoricalBuildNumbers(t *testing.T) {
	const page2 = "https://api.appstoreconnect.apple.com/v1/builds?cursor=page-2"
	const page3 = "https://api.appstoreconnect.apple.com/v1/builds?cursor=page-3"
	recorder := setBuildHistoryScanTestClient(t, map[string]string{
		"": `{"data":[{"type":"builds","id":"build-new-50","attributes":{"version":"50","uploadedDate":"2026-02-03T00:00:00Z"}}],"links":{"next":"` + page2 + `"}}`,
		"page-2": `{"data":[
			{"type":"builds","id":"build-legacy-beta","attributes":{"version":"1.0b2","uploadedDate":"2026-02-02T00:00:00Z"}},
			{"type":"builds","id":"build-blank","attributes":{"uploadedDate":"2026-02-01T12:00:00Z"}},
			{"type":"builds","id":"build-old-100","attributes":{"version":"100","uploadedDate":"2026-02-01T00:00:00Z"}}
		],"links":{"next":"` + page3 + `"}}`,
		"page-3": `{"data":[
			{"type":"builds","id":"build-legacy-rc","attributes":{"version":"2021.08.10-rc1","uploadedDate":"2026-01-31T00:00:00Z"}},
			{"type":"builds","id":"build-old-zero","attributes":{"version":"0","uploadedDate":"2026-01-30T00:00:00Z"}},
			{"type":"builds","id":"build-old-40","attributes":{"version":"40","uploadedDate":"2026-01-29T00:00:00Z"}}
		],"links":{"next":""}}`,
	})

	root := RootCommand("test")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "next-build-number", "--app", "100000001", "--output", "json"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		synctest.Test(t, func(*testing.T) { runErr = root.Run(context.Background()) })
	})
	if runErr != nil {
		t.Fatalf("run: %v", runErr)
	}

	output := decodeProcessedMaximumOutput(t, stdout)
	requireProcessedMaximumValue(t, "latestProcessedBuildNumber", output.LatestProcessedBuildNumber, "50")
	requireProcessedMaximumValue(t, "latestObservedBuildNumber", output.LatestObservedBuildNumber, "100")
	if output.NextBuildNumber != "101" {
		t.Fatalf("nextBuildNumber = %q, want 101", output.NextBuildNumber)
	}

	requireBuildHistoryScanWarnings(t, stderr, []string{
		`Warning: skipping processed build build-legacy-beta: build number "1.0b2" is not a positive integer`,
		`Warning: skipping processed build build-blank: build number "" is not a positive integer`,
		`Warning: skipping processed build build-legacy-rc: build number "2021.08.10-rc1" is not a positive integer`,
	})
	requireFreshBuildHistoryScanDeadlines(t, recorder, []string{"", "page-2", "", "page-2", "page-3"})
}
