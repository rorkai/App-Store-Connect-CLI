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

const deprecatedBetaBuildLocalizationsListWarning = "Warning: `asc beta-build-localizations list` is deprecated. Use `asc builds test-notes list`"

func TestBetaBuildLocalizationsListGlobalSuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaBuildLocalizations" {
			t.Fatalf("expected path /v1/betaBuildLocalizations, got %s", req.URL.Path)
		}
		body := `{"data":[{"type":"betaBuildLocalizations","id":"bbl-1","attributes":{"locale":"en-US","whatsNew":"Test"}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"beta-build-localizations", "list", "--global"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, deprecatedBetaBuildLocalizationsListWarning) {
		t.Fatalf("expected deprecation warning, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"bbl-1"`) {
		t.Fatalf("expected localization id in output, got %q", stdout)
	}
}

func TestBetaBuildLocalizationsListGlobalAndBuildMutuallyExclusive(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"beta-build-localizations", "list", "--global", "--build", "build-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "Error: --global and --build are mutually exclusive") {
		t.Fatalf("expected mutually exclusive error, got %q", stderr)
	}
}

func TestBetaBuildLocalizationsListMissingSelectorError(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"beta-build-localizations", "list"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "Error: --build or --global is required") {
		t.Fatalf("expected missing selector error, got %q", stderr)
	}
}

func TestBetaBuildLocalizationsListScopedStillWorks(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds/build-1/betaBuildLocalizations" {
			t.Fatalf("expected path /v1/builds/build-1/betaBuildLocalizations, got %s", req.URL.Path)
		}
		body := `{"data":[{"type":"betaBuildLocalizations","id":"bbl-scoped","attributes":{"locale":"en-US"}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"beta-build-localizations", "list", "--build", "build-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, deprecatedBetaBuildLocalizationsListWarning) {
		t.Fatalf("expected deprecation warning, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"bbl-scoped"`) {
		t.Fatalf("expected scoped localization in output, got %q", stdout)
	}
}

func TestBetaBuildLocalizationsListNextSkipsSelector(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	nextURL := "https://api.appstoreconnect.apple.com/v1/betaBuildLocalizations?cursor=page2"
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != nextURL {
			t.Fatalf("expected next URL %q, got %q", nextURL, req.URL.String())
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

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"beta-build-localizations", "list", "--next", nextURL}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, deprecatedBetaBuildLocalizationsListWarning) {
		t.Fatalf("expected deprecation warning, got %q", stderr)
	}
}

func TestBetaBuildLocalizationsListGlobalPaginate(t *testing.T) {
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
			if req.URL.Path != "/v1/betaBuildLocalizations" {
				t.Fatalf("expected path /v1/betaBuildLocalizations, got %s", req.URL.Path)
			}
			body := `{"data":[{"type":"betaBuildLocalizations","id":"bbl-1","attributes":{"locale":"en-US"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/betaBuildLocalizations?cursor=page2"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			body := `{"data":[{"type":"betaBuildLocalizations","id":"bbl-2","attributes":{"locale":"ja"}}]}`
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
		if err := root.Parse([]string{"beta-build-localizations", "list", "--global", "--paginate"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, deprecatedBetaBuildLocalizationsListWarning) {
		t.Fatalf("expected deprecation warning, got %q", stderr)
	}
	if !strings.Contains(stdout, `"bbl-1"`) {
		t.Fatalf("expected first page data in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"bbl-2"`) {
		t.Fatalf("expected second page data in output, got %q", stdout)
	}
}
