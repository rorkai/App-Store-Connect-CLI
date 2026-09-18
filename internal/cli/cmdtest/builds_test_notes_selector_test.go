package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildsTestNotesViewLatestByAppLocale(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
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
			return jsonResponse(http.StatusOK, `{
				"data":[{"type":"builds","id":"build-latest","attributes":{"processingState":"VALID","uploadedDate":"2026-03-05T12:00:00Z"}}]
			}`)
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/betaBuildLocalizations" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("filter[build]") != "build-latest" {
				t.Fatalf("expected filter[build]=build-latest, got %q", query.Get("filter[build]"))
			}
			if query.Get("filter[locale]") != "en-US" {
				t.Fatalf("expected filter[locale]=en-US, got %q", query.Get("filter[locale]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", query.Get("limit"))
			}
			return jsonResponse(http.StatusOK, `{
				"data":[{"type":"betaBuildLocalizations","id":"loc-1","attributes":{"locale":"en-US","whatsNew":"Latest notes"}}]
			}`)
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "test-notes", "view",
			"--app", "123456789",
			"--latest",
			"--locale", "en-US",
			"--output", "json",
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
		t.Fatalf("expected localization output, got %q", stdout)
	}
}

func TestBuildsTestNotesUpdateByBuildNumberAndLocale(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
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
				t.Fatalf("expected limit=200, got %q", query.Get("limit"))
			}
			return jsonResponse(http.StatusOK, `{
				"data":[{"type":"builds","id":"build-42","attributes":{"version":"42","processingState":"VALID"}}]
			}`)
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/betaBuildLocalizations" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("filter[build]") != "build-42" {
				t.Fatalf("expected filter[build]=build-42, got %q", query.Get("filter[build]"))
			}
			if query.Get("filter[locale]") != "en-US" {
				t.Fatalf("expected filter[locale]=en-US, got %q", query.Get("filter[locale]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", query.Get("limit"))
			}
			return jsonResponse(http.StatusOK, `{
				"data":[{"type":"betaBuildLocalizations","id":"loc-42","attributes":{"locale":"en-US","whatsNew":"Old notes"}}]
			}`)
		case 3:
			if req.Method != http.MethodPatch || req.URL.Path != "/v1/betaBuildLocalizations/loc-42" {
				t.Fatalf("unexpected third request: %s %s", req.Method, req.URL.String())
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body error: %v", err)
			}
			if !strings.Contains(string(payload), `"whatsNew":"Updated notes"`) {
				t.Fatalf("expected whatsNew payload, got %s", string(payload))
			}
			return jsonResponse(http.StatusOK, `{
				"data":{"type":"betaBuildLocalizations","id":"loc-42","attributes":{"locale":"en-US","whatsNew":"Updated notes"}}
			}`)
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "test-notes", "update",
			"--app", "123456789",
			"--build-number", "42",
			"--platform", "IOS",
			"--locale", "en-US",
			"--whats-new", "Updated notes",
			"--output", "json",
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
	if !strings.Contains(stdout, `"id":"loc-42"`) {
		t.Fatalf("expected updated localization output, got %q", stdout)
	}
}

func TestBuildsTestNotesCreateUpdatesExistingLocale(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			return jsonResponse(http.StatusOK, `{
				"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}
			}`)
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1/app" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"123456789"}}`)
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/betaAppLocalizations" {
				t.Fatalf("unexpected third request: %s %s", req.Method, req.URL.String())
			}
			if got := req.URL.Query().Get("filter[app]"); got != "123456789" {
				t.Fatalf("expected filter[app]=123456789, got %q", got)
			}
			return jsonResponse(http.StatusOK, `{
				"data":[{"type":"betaAppLocalizations","id":"bal-en","attributes":{"locale":"en-US"}}]
			}`)
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1/betaBuildLocalizations" {
				t.Fatalf("unexpected fourth request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", query.Get("limit"))
			}
			return jsonResponse(http.StatusOK, `{
				"data":[{"type":"betaBuildLocalizations","id":"loc-en","attributes":{"locale":"en-US","whatsNew":"Old notes"}}]
			}`)
		case 5:
			if req.Method != http.MethodPatch || req.URL.Path != "/v1/betaBuildLocalizations/loc-en" {
				t.Fatalf("unexpected fifth request: %s %s", req.Method, req.URL.String())
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body error: %v", err)
			}
			if !strings.Contains(string(payload), `"whatsNew":"Updated notes"`) {
				t.Fatalf("expected whatsNew payload, got %s", string(payload))
			}
			return jsonResponse(http.StatusOK, `{
				"data":{"type":"betaBuildLocalizations","id":"loc-en","attributes":{"locale":"en-US","whatsNew":"Updated notes"}}
			}`)
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "test-notes", "create",
			"--build-id", "build-1",
			"--locale", "en-US",
			"--whats-new", "Updated notes",
			"--output", "json",
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
	if !strings.Contains(stdout, `"id":"loc-en"`) {
		t.Fatalf("expected existing localization output, got %q", stdout)
	}
	if requestCount != 5 {
		t.Fatalf("expected five requests, got %d", requestCount)
	}
}

func TestBuildsTestNotesRemovedSelectorAliasesHaveGuidance(t *testing.T) {
	tests := []struct {
		name string
		args []string
		flag string
	}{
		{
			name: "list build alias",
			args: []string{"builds", "test-notes", "list", "--build", "BUILD_123"},
			flag: "--build",
		},
		{
			name: "view build alias",
			args: []string{"builds", "test-notes", "view", "--build", "BUILD_123", "--locale", "en-US"},
			flag: "--build",
		},
		{
			name: "view localization id alias",
			args: []string{"builds", "test-notes", "view", "--id", "loc-1"},
			flag: "--id",
		},
		{
			name: "create build alias",
			args: []string{"builds", "test-notes", "create", "--build", "BUILD_123", "--locale", "en-US", "--whats-new", "Notes"},
			flag: "--build",
		},
		{
			name: "update localization id alias",
			args: []string{"builds", "test-notes", "update", "--id", "loc-1", "--whats-new", "Notes"},
			flag: "--id",
		},
		{
			name: "delete localization id alias",
			args: []string{"builds", "test-notes", "delete", "--id", "loc-1", "--confirm"},
			flag: "--id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertRemovedFlagGuidance(t, test.args, test.flag)
		})
	}
}
