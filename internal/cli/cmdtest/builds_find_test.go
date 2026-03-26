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

func TestBuildsInfoByBuildNumberSuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/builds":
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
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
			if query.Get("sort") != "-uploadedDate" {
				t.Fatalf("expected sort=-uploadedDate, got %q", query.Get("sort"))
			}
			if query.Get("limit") != "1" {
				t.Fatalf("expected limit=1, got %q", query.Get("limit"))
			}
			body := `{"data":[{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"PROCESSING"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/builds/build-42/preReleaseVersion":
			body := `{"data":{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request path %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "info", "--app", "123456789", "--build-number", "42", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-42"`) {
		t.Fatalf("expected build output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"preReleaseVersions"`) {
		t.Fatalf("expected attached pre-release version output, got %q", stdout)
	}
}

func TestBuildsInfoByBuildNumberExplicitPlatformOverridesIOSDefault(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/builds":
			if got := req.URL.Query().Get("filter[preReleaseVersion.platform]"); got != "TV_OS" {
				t.Fatalf("expected explicit platform override TV_OS, got %q", got)
			}
			body := `{"data":[{"type":"builds","id":"build-tv","attributes":{"version":"42","processingState":"VALID"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/builds/build-tv/preReleaseVersion":
			body := `{"data":{"type":"preReleaseVersions","id":"prv-tv","attributes":{"version":"1.2.3","platform":"TV_OS"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request path %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if _, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "info", "--app", "123456789", "--build-number", "42", "--platform", "TV_OS", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	}); stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestBuildsInfoByBuildNumberNotFound(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/builds" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
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
		if err := root.Parse([]string{"builds", "info", "--app", "123456789", "--build-number", "42"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected not-found error")
	}
	if errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected runtime not-found error, got usage error: %v", runErr)
	}
	if !strings.Contains(runErr.Error(), `builds info: no build found for app 123456789 with build number "42"`) {
		t.Fatalf("expected not-found message, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on failure, got %q", stdout)
	}
}

func TestBuildsInfoByLatestSuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/builds":
			query := req.URL.Query()
			if query.Get("filter[app]") != "123456789" {
				t.Fatalf("expected filter[app]=123456789, got %q", query.Get("filter[app]"))
			}
			if query.Get("sort") != "-uploadedDate" {
				t.Fatalf("expected sort=-uploadedDate, got %q", query.Get("sort"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", query.Get("limit"))
			}
			if query.Get("filter[version]") != "" {
				t.Fatalf("expected no build-number filter, got %q", query.Get("filter[version]"))
			}
			body := `{"data":[{"type":"builds","id":"build-latest","attributes":{"version":"99","processingState":"VALID"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/builds/build-latest/preReleaseVersion":
			body := `{"data":{"type":"preReleaseVersions","id":"prv-latest","attributes":{"version":"3.4.5","platform":"IOS"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request path %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "info", "--app", "123456789", "--latest", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-latest"`) {
		t.Fatalf("expected latest build output, got %q", stdout)
	}
}

func TestBuildsInfoByLatestNormalizesLowercasePlatform(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/preReleaseVersions":
			query := req.URL.Query()
			if query.Get("filter[app]") != "123456789" {
				t.Fatalf("expected filter[app]=123456789, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[version]") != "1.2.3" {
				t.Fatalf("expected filter[version]=1.2.3, got %q", query.Get("filter[version]"))
			}
			if query.Get("filter[platform]") != "IOS" {
				t.Fatalf("expected normalized IOS platform filter, got %q", query.Get("filter[platform]"))
			}
			body := `{"data":[{"type":"preReleaseVersions","id":"prv-ios","attributes":{"version":"1.2.3","platform":"IOS"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/builds":
			query := req.URL.Query()
			if query.Get("filter[preReleaseVersion]") != "prv-ios" {
				t.Fatalf("expected preReleaseVersion=prv-ios, got %q", query.Get("filter[preReleaseVersion]"))
			}
			body := `{"data":[{"type":"builds","id":"build-ios","attributes":{"version":"88","processingState":"VALID"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/builds/build-ios/preReleaseVersion":
			body := `{"data":{"type":"preReleaseVersions","id":"prv-ios","attributes":{"version":"1.2.3","platform":"IOS"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request path %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "info", "--app", "123456789", "--latest", "--version", "1.2.3", "--platform", "ios", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-ios"`) {
		t.Fatalf("expected normalized-platform latest build output, got %q", stdout)
	}
}

func TestBuildsInfoByLatestVersionWithoutPlatformSelectsNewestAcrossPlatforms(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/preReleaseVersions":
			query := req.URL.Query()
			if query.Get("filter[app]") != "123456789" {
				t.Fatalf("expected filter[app]=123456789, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[version]") != "1.2.3" {
				t.Fatalf("expected filter[version]=1.2.3, got %q", query.Get("filter[version]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200 for version-only latest lookup, got %q", query.Get("limit"))
			}
			body := `{"data":[{"type":"preReleaseVersions","id":"prv-ios","attributes":{"version":"1.2.3","platform":"IOS"}},{"type":"preReleaseVersions","id":"prv-macos","attributes":{"version":"1.2.3","platform":"MAC_OS"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/builds":
			query := req.URL.Query()
			switch query.Get("filter[preReleaseVersion]") {
			case "prv-ios":
				body := `{"data":[{"type":"builds","id":"build-ios-old","attributes":{"version":"100","uploadedDate":"2026-03-01T10:00:00Z"}}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			case "prv-macos":
				body := `{"data":[{"type":"builds","id":"build-macos-new","attributes":{"version":"101","uploadedDate":"2026-03-02T10:00:00Z"}}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			default:
				t.Fatalf("unexpected preReleaseVersion filter %q", query.Get("filter[preReleaseVersion]"))
				return nil, nil
			}
		case "/v1/builds/build-macos-new/preReleaseVersion":
			body := `{"data":{"type":"preReleaseVersions","id":"prv-macos","attributes":{"version":"1.2.3","platform":"MAC_OS"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request path %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "info", "--app", "123456789", "--latest", "--version", "1.2.3", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-macos-new"`) {
		t.Fatalf("expected newest cross-platform latest build, got %q", stdout)
	}
}

func TestBuildsInfoByLatestVersionIgnoresNearMatchPreReleaseVersions(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/preReleaseVersions":
			query := req.URL.Query()
			if query.Get("filter[app]") != "123456789" {
				t.Fatalf("expected filter[app]=123456789, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[version]") != "1.1" {
				t.Fatalf("expected filter[version]=1.1, got %q", query.Get("filter[version]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200 for version-only latest lookup, got %q", query.Get("limit"))
			}
			body := `{"data":[{"type":"preReleaseVersions","id":"prv-exact","attributes":{"version":"1.1","platform":"MAC_OS"}},{"type":"preReleaseVersions","id":"prv-near","attributes":{"version":"1.1.0","platform":"IOS"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/builds":
			query := req.URL.Query()
			if query.Get("filter[preReleaseVersion]") != "prv-exact" {
				t.Fatalf("expected exact pre-release version match only, got %q", query.Get("filter[preReleaseVersion]"))
			}
			body := `{"data":[{"type":"builds","id":"build-exact","attributes":{"version":"101","uploadedDate":"2026-03-03T10:00:00Z"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/builds/build-exact/preReleaseVersion":
			body := `{"data":{"type":"preReleaseVersions","id":"prv-exact","attributes":{"version":"1.1","platform":"MAC_OS"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request path %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "info", "--app", "123456789", "--latest", "--version", "1.1", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-exact"`) {
		t.Fatalf("expected exact-version latest build output, got %q", stdout)
	}
}

func TestBuildsInfoByLatestVersionKeepsServerMatchedPreReleaseVersionsWithoutAttributes(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/preReleaseVersions":
			query := req.URL.Query()
			if query.Get("filter[app]") != "123456789" {
				t.Fatalf("expected filter[app]=123456789, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[version]") != "1.1" {
				t.Fatalf("expected filter[version]=1.1, got %q", query.Get("filter[version]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200 for version-only latest lookup, got %q", query.Get("limit"))
			}
			body := `{"data":[{"type":"preReleaseVersions","id":"prv-server","attributes":{}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/builds":
			query := req.URL.Query()
			if query.Get("filter[preReleaseVersion]") != "prv-server" {
				t.Fatalf("expected server-matched pre-release version to be preserved, got %q", query.Get("filter[preReleaseVersion]"))
			}
			body := `{"data":[{"type":"builds","id":"build-server","attributes":{"version":"101","uploadedDate":"2026-03-03T10:00:00Z"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/builds/build-server/preReleaseVersion":
			body := `{"data":{"type":"preReleaseVersions","id":"prv-server","attributes":{"version":"1.1","platform":"IOS"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request path %s", req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "info", "--app", "123456789", "--latest", "--version", "1.1", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-server"`) {
		t.Fatalf("expected server-matched latest build output, got %q", stdout)
	}
}

func TestBuildsInfoByLatestVersionAndPlatformPaginatesPastNearMatches(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	const nextURL = "https://api.appstoreconnect.apple.com/v1/preReleaseVersions?cursor=page-2"

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.String() == nextURL:
			body := `{"data":[{"type":"preReleaseVersions","id":"prv-exact","attributes":{"version":"1.1","platform":"IOS"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.URL.Path == "/v1/preReleaseVersions":
			query := req.URL.Query()
			if query.Get("filter[app]") != "123456789" {
				t.Fatalf("expected filter[app]=123456789, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[version]") != "1.1" {
				t.Fatalf("expected filter[version]=1.1, got %q", query.Get("filter[version]"))
			}
			if query.Get("filter[platform]") != "IOS" {
				t.Fatalf("expected filter[platform]=IOS, got %q", query.Get("filter[platform]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200 for version+platform latest lookup, got %q", query.Get("limit"))
			}
			body := `{"data":[{"type":"preReleaseVersions","id":"prv-near","attributes":{"version":"1.1.0","platform":"IOS"}}],"links":{"next":"` + nextURL + `"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			switch req.URL.Path {
			case "/v1/builds":
				query := req.URL.Query()
				if query.Get("filter[preReleaseVersion]") != "prv-exact" {
					t.Fatalf("expected exact pre-release version match after pagination, got %q", query.Get("filter[preReleaseVersion]"))
				}
				body := `{"data":[{"type":"builds","id":"build-exact-ios","attributes":{"version":"101","uploadedDate":"2026-03-03T10:00:00Z"}}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			case "/v1/builds/build-exact-ios/preReleaseVersion":
				body := `{"data":{"type":"preReleaseVersions","id":"prv-exact","attributes":{"version":"1.1","platform":"IOS"}}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			default:
				t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
				return nil, nil
			}
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "info", "--app", "123456789", "--latest", "--version", "1.1", "--platform", "IOS", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-exact-ios"`) {
		t.Fatalf("expected paginated exact-version latest build output, got %q", stdout)
	}
}

func TestBuildsFindAliasWarnsAndMatchesCanonicalInfoOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/builds":
			query := req.URL.Query()
			if query.Get("filter[app]") != "123456789" {
				t.Fatalf("expected filter[app]=123456789, got %q", query.Get("filter[app]"))
			}
			if query.Get("filter[version]") != "42" {
				t.Fatalf("expected filter[version]=42, got %q", query.Get("filter[version]"))
			}
			if query.Get("filter[preReleaseVersion.platform]") != "IOS" {
				t.Fatalf("expected default IOS platform filter, got %q", query.Get("filter[preReleaseVersion.platform]"))
			}
			body := `{"data":[{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"VALID"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/builds/build-42/preReleaseVersion":
			body := `{"data":{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request path %s", req.URL.Path)
			return nil, nil
		}
	})

	run := func(args []string) (string, string) {
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		return captureOutput(t, func() {
			if err := root.Parse(args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := root.Run(context.Background()); err != nil {
				t.Fatalf("run error: %v", err)
			}
		})
	}

	canonicalStdout, canonicalStderr := run([]string{"builds", "info", "--app", "123456789", "--build-number", "42", "--output", "json"})
	aliasStdout, aliasStderr := run([]string{"builds", "find", "--app", "123456789", "--build-number", "42", "--output", "json"})

	if canonicalStderr != "" {
		t.Fatalf("expected canonical command to avoid warnings, got %q", canonicalStderr)
	}
	requireStderrContainsWarning(t, aliasStderr, "Warning: `asc builds find` is deprecated. Use `asc builds info`.")
	assertOnlyDeprecatedCommandWarnings(t, aliasStderr)
	if canonicalStdout != aliasStdout {
		t.Fatalf("expected canonical and alias output to match, canonical=%q alias=%q", canonicalStdout, aliasStdout)
	}
}

func TestBuildsFindAliasHiddenFromCanonicalHelp(t *testing.T) {
	usage := usageForCommand(t, "builds")
	if strings.Contains(usage, "\n  find\t") || strings.Contains(usage, "\n  find ") {
		t.Fatalf("expected deprecated builds find alias to stay hidden from canonical help, got %q", usage)
	}
}
