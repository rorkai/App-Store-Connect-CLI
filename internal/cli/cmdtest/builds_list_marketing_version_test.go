package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildsListIncludesPreReleaseVersion(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var capturedPath string
	var capturedAppFilter string
	var includeValue string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		// The builds list request (either endpoint)
		if strings.Contains(req.URL.Path, "builds") || strings.HasPrefix(req.URL.Path, "/v1/apps/123456789/builds") {
			capturedPath = req.URL.Path
			capturedAppFilter = req.URL.Query().Get("filter[app]")
			includeValue = req.URL.Query().Get("include")

			body := `{
				"data":[{
					"type":"builds",
					"id":"build-1",
					"attributes":{"version":"9","uploadedDate":"2026-03-13T00:00:00Z","processingState":"VALID","expired":false},
					"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"prv-1"}}}
				}],
				"included":[{
					"type":"preReleaseVersions",
					"id":"prv-1",
					"attributes":{"version":"1.2.3","platform":"TV_OS"}
				}]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}

		// Unexpected request
		t.Logf("unexpected request: %s %s", req.Method, req.URL.String())
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"404"}]}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "list", "--app", "123456789"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(includeValue, "preReleaseVersion") {
		t.Fatalf("expected include=preReleaseVersion in API request, got %q", includeValue)
	}
	if capturedPath != "/v1/builds" {
		t.Fatalf("expected /v1/builds when include=preReleaseVersion is requested, got %q", capturedPath)
	}
	if capturedAppFilter != "123456789" {
		t.Fatalf("expected filter[app]=123456789 on /v1/builds request, got %q", capturedAppFilter)
	}
	if !strings.Contains(stdout, "1.2.3") {
		t.Fatalf("expected marketing version 1.2.3 in output, got %q", stdout)
	}
}

func TestBuildsListIncludeParamSentWithFilters(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var capturedPath string
	var capturedInclude string
	var capturedProcessingState string

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "builds") {
			capturedPath = req.URL.Path
			query := req.URL.Query()
			capturedInclude = query.Get("include")
			capturedProcessingState = query.Get("filter[processingState]")

			body := `{"data":[{"type":"builds","id":"build-1","attributes":{"version":"5","processingState":"VALID"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}

		t.Logf("unexpected request: %s %s", req.Method, req.URL.String())
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"404"}]}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "list", "--app", "123456789", "--processing-state", "VALID"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if capturedPath != "/v1/builds" {
		t.Fatalf("expected /v1/builds with filters, got %s", capturedPath)
	}
	if capturedProcessingState != "VALID" {
		t.Fatalf("expected filter[processingState]=VALID, got %q", capturedProcessingState)
	}
	if !strings.Contains(capturedInclude, "preReleaseVersion") {
		t.Fatalf("expected include=preReleaseVersion, got %q", capturedInclude)
	}
}

func TestBuildsListPaginateKeepsPreReleaseVersionOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_SPINNER_DISABLED", "1")

	const nextURL = "https://api.appstoreconnect.apple.com/v1/builds?filter%5Bapp%5D=123456789&include=preReleaseVersion&limit=200&cursor=AQ"

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
				t.Fatalf("expected first request to /v1/builds, got %s", req.URL.Path)
			}
			query := req.URL.Query()
			if query.Get("filter[app]") != "123456789" {
				t.Fatalf("expected filter[app]=123456789, got %q", query.Get("filter[app]"))
			}
			if query.Get("include") != "preReleaseVersion" {
				t.Fatalf("expected include=preReleaseVersion, got %q", query.Get("include"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200 for paginated fetch, got %q", query.Get("limit"))
			}
			body := `{
				"data":[{
					"type":"builds",
					"id":"build-1",
					"attributes":{"version":"9","uploadedDate":"2026-03-13T00:00:00Z","processingState":"VALID","expired":false},
					"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"prv-1"}}}
				}],
				"included":[{
					"type":"preReleaseVersions",
					"id":"prv-1",
					"attributes":{"version":"1.2.3","platform":"TV_OS"}
				}],
				"links":{"next":"` + nextURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.String() != nextURL {
				t.Fatalf("expected next URL %q, got %q", nextURL, req.URL.String())
			}
			body := `{
				"data":[{
					"type":"builds",
					"id":"build-2",
					"attributes":{"version":"10","uploadedDate":"2026-03-14T00:00:00Z","processingState":"VALID","expired":false},
					"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"prv-2"}}}
				}],
				"included":[{
					"type":"preReleaseVersions",
					"id":"prv-2",
					"attributes":{"version":"2.0.0","platform":"IOS"}
				}],
				"links":{"next":""}
			}`
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
		if err := root.Parse([]string{"builds", "list", "--app", "123456789", "--paginate", "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "1.2.3") || !strings.Contains(stdout, "2.0.0") {
		t.Fatalf("expected paginated table output to contain both marketing versions, got %q", stdout)
	}
	if !strings.Contains(stdout, "TV_OS") || !strings.Contains(stdout, "IOS") {
		t.Fatalf("expected paginated table output to contain both platforms, got %q", stdout)
	}
}
