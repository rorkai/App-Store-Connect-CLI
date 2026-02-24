package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildsListAllowsIndependentVersionAndBuildNumberFilters(t *testing.T) {
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
			if req.URL.Path != "/v1/preReleaseVersions" {
				t.Fatalf("expected path /v1/preReleaseVersions, got %s", req.URL.Path)
			}
			query := req.URL.Query()
			if query.Get("filter[app]") != "123456789" {
				t.Fatalf("expected filter[app]=123456789, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[version]") != "1.2.3" {
				t.Fatalf("expected filter[version]=1.2.3, got %q", query.Get("filter[version]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", query.Get("limit"))
			}
			body := `{"data":[{"type":"preReleaseVersions","id":"prv-1"},{"type":"preReleaseVersions","id":"prv-2"}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
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
			if query.Get("filter[preReleaseVersion]") != "prv-1,prv-2" {
				t.Fatalf("expected filter[preReleaseVersion]=prv-1,prv-2, got %q", query.Get("filter[preReleaseVersion]"))
			}
			if query.Get("filter[version]") != "77" {
				t.Fatalf("expected filter[version]=77, got %q", query.Get("filter[version]"))
			}
			body := `{"data":[{"type":"builds","id":"build-77","attributes":{"version":"77"}}]}`
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
		if err := root.Parse([]string{"builds", "list", "--app", "123456789", "--version", "1.2.3", "--build-number", "77"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-77"`) {
		t.Fatalf("expected filtered build in output, got %q", stdout)
	}
}

func TestBuildsListVersionLookupPaginatesAndUsesAllPreReleaseVersions(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	const nextPreReleaseURL = "https://api.appstoreconnect.apple.com/v1/preReleaseVersions?cursor=AQ"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/preReleaseVersions" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("filter[app]") != "123456789" {
				t.Fatalf("expected filter[app]=123456789, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[version]") != "2.0.0" {
				t.Fatalf("expected filter[version]=2.0.0, got %q", query.Get("filter[version]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", query.Get("limit"))
			}
			body := `{"data":[{"type":"preReleaseVersions","id":"prv-1"}],"links":{"next":"` + nextPreReleaseURL + `"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != nextPreReleaseURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"preReleaseVersions","id":"prv-2"}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds" {
				t.Fatalf("unexpected third request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("filter[preReleaseVersion]") != "prv-1,prv-2" {
				t.Fatalf("expected filter[preReleaseVersion]=prv-1,prv-2, got %q", req.URL.Query().Get("filter[preReleaseVersion]"))
			}
			body := `{"data":[{"type":"builds","id":"build-2"}]}`
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
		if err := root.Parse([]string{"builds", "list", "--app", "123456789", "--version", "2.0.0"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-2"`) {
		t.Fatalf("expected filtered build in output, got %q", stdout)
	}
}

func TestBuildsListNextURLIgnoresVersionFilters(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	const nextURL = "https://api.appstoreconnect.apple.com/v1/builds?cursor=AQ"

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
			"--version", "1.2.3",
			"--build-number", "77",
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
		t.Fatalf("expected next-page build in output, got %q", stdout)
	}
}

func TestBuildsListVersionFilterNoResultsReturnsEmptyData(t *testing.T) {
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
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/preReleaseVersions" {
			t.Fatalf("expected path /v1/preReleaseVersions, got %s", req.URL.Path)
		}
		query := req.URL.Query()
		if query.Get("filter[app]") != "123456789" {
			t.Fatalf("expected filter[app]=123456789, got %q", query.Get("filter[app]"))
		}
		if query.Get("filter[version]") != "9.9.9" {
			t.Fatalf("expected filter[version]=9.9.9, got %q", query.Get("filter[version]"))
		}
		if query.Get("limit") != "200" {
			t.Fatalf("expected limit=200, got %q", query.Get("limit"))
		}
		body := `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/builds"}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "list", "--app", "123456789", "--version", "9.9.9"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"data":[]`) {
		t.Fatalf("expected empty data output, got %q", stdout)
	}
	if requestCount != 1 {
		t.Fatalf("expected only pre-release lookup request, got %d", requestCount)
	}
}

func TestBuildsListBuildNumberMapsToVersionFilter(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
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
		if query.Get("filter[version]") != "77" {
			t.Fatalf("expected filter[version]=77, got %q", query.Get("filter[version]"))
		}
		body := `{"data":[{"type":"builds","id":"build-77","attributes":{"version":"77"}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "list", "--app", "123456789", "--build-number", "77"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-77"`) {
		t.Fatalf("expected build id in output, got %q", stdout)
	}
}
