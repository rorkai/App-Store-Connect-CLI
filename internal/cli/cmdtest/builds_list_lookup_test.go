package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppScopedCommandsResolveAppByBundleID(t *testing.T) {
	type lookupRequest struct {
		path  string
		query map[string]string
		body  string
	}
	tests := []struct {
		name       string
		args       []string
		bundleID   string
		appID      string
		requests   []lookupRequest
		wantStdout string
	}{
		{
			name:     "builds list",
			args:     []string{"builds", "list", "--app", "com.example.lookup"},
			bundleID: "com.example.lookup",
			appID:    "app-lookup",
			requests: []lookupRequest{{
				path:  "/v1/builds",
				query: map[string]string{"filter[app]": "app-lookup", "include": "preReleaseVersion"},
				body:  `{"data":[{"type":"builds","id":"build-1"}]}`,
			}},
			wantStdout: `"id":"build-1"`,
		},
		{
			name:     "builds info latest",
			args:     []string{"builds", "info", "--app", "com.example.latest", "--latest"},
			bundleID: "com.example.latest",
			appID:    "100000001",
			requests: []lookupRequest{
				{
					path:  "/v1/builds",
					query: map[string]string{"filter[app]": "100000001", "sort": "-uploadedDate", "limit": "200"},
					body:  `{"data":[{"type":"builds","id":"build-1","attributes":{"version":"42"}}]}`,
				},
				{
					path: "/v1/builds/build-1/preReleaseVersion",
					body: `{"data":{"type":"preReleaseVersions","id":"prv-1"}}`,
				},
			},
			wantStdout: `"id":"build-1"`,
		},
		{
			name:     "testflight crashes list",
			args:     []string{"testflight", "crashes", "list", "--app", "com.example.crashes"},
			bundleID: "com.example.crashes",
			appID:    "app-lookup",
			requests: []lookupRequest{{
				path: "/v1/apps/app-lookup/betaFeedbackCrashSubmissions",
				body: `{"data":[{"type":"betaFeedbackCrashSubmissions","id":"crash-1"}]}`,
			}},
			wantStdout: `"id":"crash-1"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			requests := append([]lookupRequest{{
				path:  "/v1/apps",
				query: map[string]string{"filter[bundleId]": test.bundleID, "limit": "2"},
				body:  `{"data":[{"type":"apps","id":"` + test.appID + `"}]}`,
			}}, test.requests...)
			callCount := 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				callCount++
				if callCount > len(requests) {
					t.Fatalf("unexpected request %d: %s %s", callCount, req.Method, req.URL.String())
				}
				want := requests[callCount-1]
				if req.Method != http.MethodGet || req.URL.Path != want.path {
					t.Fatalf("request %d: got %s %s, want GET %s", callCount, req.Method, req.URL.String(), want.path)
				}
				query := req.URL.Query()
				for key, value := range want.query {
					if got := query.Get(key); got != value {
						t.Fatalf("request %d: expected %s=%q, got %q", callCount, key, value, got)
					}
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(want.body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if callCount != len(requests) {
				t.Fatalf("request count = %d, want %d", callCount, len(requests))
			}
			if !strings.Contains(stdout, test.wantStdout) {
				t.Fatalf("expected %s in output, got %q", test.wantStdout, stdout)
			}
		})
	}
}

func TestBuildsListNextURLSkipsAppLookupForInheritedNonNumericApp(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const nextURL = "https://api.appstoreconnect.apple.com/v1/builds?cursor=AQ"
	t.Setenv("ASC_APP_ID", "com.example.lookup")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if requestCount != 1 {
			t.Fatalf("unexpected request count %d: %s %s", requestCount, req.Method, req.URL.String())
		}
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.String() != nextURL {
			t.Fatalf("expected next URL %q, got %q", nextURL, req.URL.String())
		}
		body := `{"data":[{"type":"builds","id":"build-next"}],"links":{"next":""}}`
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
			"builds", "list",
			"--next", nextURL,
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
	if !strings.Contains(stdout, `"id":"build-next"`) {
		t.Fatalf("expected next-page build output, got %q", stdout)
	}
}
