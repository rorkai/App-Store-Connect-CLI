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

const (
	deprecatedBetaBuildLocalizationsGetWarning    = "Warning: `asc beta-build-localizations get` is deprecated. Use `asc builds test-notes view`"
	deprecatedBetaBuildLocalizationsCreateWarning = "Warning: `asc beta-build-localizations create` is deprecated. Use `asc builds test-notes create`"
)

func TestBetaBuildLocalizationsGetLatestByAppWithStateAlias(t *testing.T) {
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
			if query.Get("limit") != "1" {
				t.Fatalf("expected limit=1, got %q", query.Get("limit"))
			}
			if query.Get("filter[processingState]") != "PROCESSING,VALID" {
				t.Fatalf("expected filter[processingState]=PROCESSING,VALID, got %q", query.Get("filter[processingState]"))
			}
			body := `{"data":[{"type":"builds","id":"build-latest","attributes":{"processingState":"PROCESSING","uploadedDate":"2026-03-05T12:00:00Z"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-latest/betaBuildLocalizations" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[{"type":"betaBuildLocalizations","id":"loc-1","attributes":{"locale":"en-US","whatsNew":"Notes"}}]}`
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
		if err := root.Parse([]string{
			"beta-build-localizations", "get",
			"--app", "123456789",
			"--latest",
			"--state", "PROCESSING,COMPLETE",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, deprecatedBetaBuildLocalizationsGetWarning) {
		t.Fatalf("expected deprecation warning, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"loc-1"`) {
		t.Fatalf("expected localization output, got %q", stdout)
	}
}

func TestBetaBuildLocalizationsGetLatestRequiresLocaleWhenMultipleLocalizations(t *testing.T) {
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
			body := `{"data":[{"type":"builds","id":"build-latest","attributes":{"processingState":"VALID","uploadedDate":"2026-03-05T12:00:00Z"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			body := `{"data":[{"type":"betaBuildLocalizations","id":"loc-1","attributes":{"locale":"en-US"}},{"type":"betaBuildLocalizations","id":"loc-2","attributes":{"locale":"ja"}}]}`
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

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"beta-build-localizations", "get",
			"--app", "123456789",
			"--latest",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "pass --locale to disambiguate") {
		t.Fatalf("expected disambiguation error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on error, got %q", stdout)
	}
}

func TestBetaBuildLocalizationsCreateLatestByAppWithStateAlias(t *testing.T) {
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
			if query.Get("filter[processingState]") != "PROCESSING,VALID" {
				t.Fatalf("expected filter[processingState]=PROCESSING,VALID, got %q", query.Get("filter[processingState]"))
			}
			body := `{"data":[{"type":"builds","id":"build-create","attributes":{"processingState":"PROCESSING"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/betaBuildLocalizations" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read create payload: %v", err)
			}
			bodyText := string(payload)
			if !strings.Contains(bodyText, `"id":"build-create"`) || !strings.Contains(bodyText, `"locale":"en-US"`) || !strings.Contains(bodyText, `"whatsNew":"Latest notes"`) {
				t.Fatalf("unexpected create payload: %s", bodyText)
			}
			body := `{"data":{"type":"betaBuildLocalizations","id":"loc-create","attributes":{"locale":"en-US","whatsNew":"Latest notes"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
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
		if err := root.Parse([]string{
			"beta-build-localizations", "create",
			"--app", "123456789",
			"--latest",
			"--state", "PROCESSING,COMPLETE",
			"--locale", "en-US",
			"--whats-new", "Latest notes",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, deprecatedBetaBuildLocalizationsCreateWarning) {
		t.Fatalf("expected deprecation warning, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"loc-create"`) {
		t.Fatalf("expected create output, got %q", stdout)
	}
}

func TestBetaBuildLocalizationsLatestSelectorValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "get app without latest",
			args:    []string{"beta-build-localizations", "get", "--app", "123456789"},
			wantErr: "--latest is required with --app",
		},
		{
			name:    "get latest without app",
			args:    []string{"beta-build-localizations", "get", "--latest"},
			wantErr: "--app is required with --latest",
		},
		{
			name:    "create state without latest",
			args:    []string{"beta-build-localizations", "create", "--app", "123456789", "--state", "PROCESSING", "--locale", "en-US", "--whats-new", "notes"},
			wantErr: "--latest is required with --app",
		},
		{
			name:    "create latest without app",
			args:    []string{"beta-build-localizations", "create", "--latest", "--locale", "en-US", "--whats-new", "notes"},
			wantErr: "--app is required with --latest",
		},
		{
			name:    "create build with latest mode flags",
			args:    []string{"beta-build-localizations", "create", "--build", "build-1", "--app", "123456789", "--latest", "--locale", "en-US", "--whats-new", "notes"},
			wantErr: "--build is mutually exclusive",
		},
		{
			name:    "get invalid state value",
			args:    []string{"beta-build-localizations", "get", "--app", "123456789", "--latest", "--state", "WRONG"},
			wantErr: "--state must be one of",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			originalTransport := http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
			})
			requestCount := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requestCount++
				t.Fatalf("unexpected request for validation case %q: %s %s", tc.name, req.Method, req.URL.String())
				return nil, nil
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(tc.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, tc.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", tc.wantErr, stderr)
			}
			if requestCount != 0 {
				t.Fatalf("expected zero HTTP calls, got %d", requestCount)
			}
		})
	}
}
