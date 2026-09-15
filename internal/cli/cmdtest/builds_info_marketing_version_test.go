package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildsInfoTableShowsMarketingVersionAndPlatform(t *testing.T) {
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
			if req.URL.Path != "/v1/builds/build-1" {
				t.Fatalf("expected build request path /v1/builds/build-1, got %s", req.URL.Path)
			}
			body := `{
				"data":{
					"type":"builds",
					"id":"build-1",
					"attributes":{"version":"9","uploadedDate":"2026-03-13T00:00:00Z","processingState":"VALID","expired":false}
				}
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
			if req.URL.Path != "/v1/builds/build-1/preReleaseVersion" {
				t.Fatalf("expected pre-release-version path, got %s", req.URL.Path)
			}
			body := `{
				"data":{
					"type":"preReleaseVersions",
					"id":"prv-1",
					"attributes":{"version":"1.2.3","platform":"TV_OS"}
				}
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
		if err := root.Parse([]string{"builds", "info", "--build-id", "build-1", "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "1.2.3") || !strings.Contains(stdout, "TV_OS") {
		t.Fatalf("expected builds info table output to include marketing version and platform, got %q", stdout)
	}
}

func TestBuildsInfoJSONPreservesExistingRelationships(t *testing.T) {
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
			body := `{
				"data":{
					"type":"builds",
					"id":"build-1",
					"attributes":{"version":"9","uploadedDate":"2026-03-13T00:00:00Z","processingState":"VALID","expired":false},
					"relationships":{
						"app":{"links":{"related":"https://example.com/apps/app-1"}}
					}
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			body := `{
				"data":{
					"type":"preReleaseVersions",
					"id":"prv-1",
					"attributes":{"version":"1.2.3","platform":"TV_OS"}
				}
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
		if err := root.Parse([]string{"builds", "info", "--build-id", "build-1", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got %v", err)
	}

	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected object data payload, got %#v", payload["data"])
	}
	relationships, ok := data["relationships"].(map[string]any)
	if !ok {
		t.Fatalf("expected relationships object, got %#v", data["relationships"])
	}
	if _, ok := relationships["app"]; !ok {
		t.Fatalf("expected existing app relationship to be preserved, got %#v", relationships)
	}
	if _, ok := relationships["preReleaseVersion"]; !ok {
		t.Fatalf("expected preReleaseVersion relationship to be attached, got %#v", relationships)
	}
}

func TestBuildsInfoBuildNumberRequiresUniqueMatch(t *testing.T) {
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
			t.Fatalf("expected builds list path, got %s", req.URL.Path)
		}

		query := req.URL.Query()
		if query.Get("filter[app]") != "123456789" {
			t.Fatalf("expected filter[app]=123456789, got %q", query.Get("filter[app]"))
		}
		if query.Get("filter[version]") != "42" {
			t.Fatalf("expected filter[version]=42, got %q", query.Get("filter[version]"))
		}
		if query.Get("filter[preReleaseVersion.platform]") != "IOS" {
			t.Fatalf("expected IOS platform filter, got %q", query.Get("filter[preReleaseVersion.platform]"))
		}
		if query.Get("sort") != "-uploadedDate" {
			t.Fatalf("expected sort=-uploadedDate, got %q", query.Get("sort"))
		}
		if query.Get("limit") != "200" {
			t.Fatalf("expected limit=200 for uniqueness check, got %q", query.Get("limit"))
		}

		body := `{
			"data":[
				{"type":"builds","id":"build-ios","attributes":{"version":"42","uploadedDate":"2026-03-13T00:00:00Z","processingState":"VALID","expired":false}},
				{"type":"builds","id":"build-macos","attributes":{"version":"42","uploadedDate":"2026-03-12T00:00:00Z","processingState":"VALID","expired":false}}
			]
		}`
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
		if err := root.Parse([]string{"builds", "info", "--app", "123456789", "--build-number", "42", "--platform", "IOS", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected unique build-number lookup error")
	}
	if !isUsageClassError(runErr) {
		t.Fatalf("expected ambiguity to be a usage error, got %v", runErr)
	}
	assertUsageDiagnosticFirstLine(t, stderr, `2 builds match build number "42" for platform IOS for app 123456789; pass --build-id with one of:`)
	if !strings.Contains(runErr.Error(), `2 builds match build number "42" for platform IOS for app 123456789; pass --build-id with one of:`) {
		t.Fatalf("expected ambiguity error, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "build-ios") || !strings.Contains(runErr.Error(), "build-macos") {
		t.Fatalf("expected candidate build IDs in the ambiguity error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on ambiguity error, got %q", stdout)
	}
}
