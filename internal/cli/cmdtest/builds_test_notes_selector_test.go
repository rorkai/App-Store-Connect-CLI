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
			if query.Get("sort") != "-uploadedDate" {
				t.Fatalf("expected sort=-uploadedDate, got %q", query.Get("sort"))
			}
			if query.Get("limit") != "1" {
				t.Fatalf("expected limit=1, got %q", query.Get("limit"))
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

func TestBuildsTestNotesListLegacyBuildAliasWarnsAndMatchesCanonicalOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount % 2 {
		case 1:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/builds/build-1" {
				t.Fatalf("expected path /v1/builds/build-1, got %s", req.URL.Path)
			}
			return jsonResponse(http.StatusOK, `{
				"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}
			}`)
		case 0:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaBuildLocalizations" {
				t.Fatalf("expected path /v1/betaBuildLocalizations, got %s", req.URL.Path)
			}
			if req.URL.Query().Get("filter[build]") != "build-1" {
				t.Fatalf("expected filter[build]=build-1, got %q", req.URL.Query().Get("filter[build]"))
			}
			return jsonResponse(http.StatusOK, `{
				"data":[{"type":"betaBuildLocalizations","id":"loc-1","attributes":{"locale":"en-US","whatsNew":"Notes"}}]
			}`)
		default:
			t.Fatalf("unexpected request count %d", requestCount)
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

	canonicalStdout, canonicalStderr := run([]string{"builds", "test-notes", "list", "--build-id", "build-1", "--output", "json"})
	aliasStdout, aliasStderr := run([]string{"builds", "test-notes", "list", "--build", "build-1", "--output", "json"})

	if canonicalStderr != "" {
		t.Fatalf("expected canonical command to avoid warnings, got %q", canonicalStderr)
	}
	requireStderrContainsWarning(t, aliasStderr, "Warning: `--build` is deprecated. Use `--build-id`.")
	assertOnlyDeprecatedCommandWarnings(t, aliasStderr)
	if canonicalStdout != aliasStdout {
		t.Fatalf("expected canonical and alias output to match, canonical=%q alias=%q", canonicalStdout, aliasStdout)
	}
}
