package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubmitCreatePreflightCatchesMissingWhatsNew(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		// Resolve version by marketing version string
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/appStoreVersions":
			query := req.URL.Query()
			if strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE") {
				// isAppUpdate check — return a READY_FOR_SALE version
				body := `{"data":[{"type":"appStoreVersions","id":"version-old","attributes":{"versionString":"1.0.0","appStoreState":"READY_FOR_SALE"}}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
			// Version lookup for --version flag
			body := `{"data":[{"type":"appStoreVersions","id":"version-new","attributes":{"versionString":"1.1.0","appStoreState":"PREPARE_FOR_SUBMISSION","platform":"TV_OS"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		// Fetch localizations for preflight — whatsNew is empty
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-new/appStoreVersionLocalizations":
			body := `{"data":[
				{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"desc","keywords":"kw","supportUrl":"https://example.com","whatsNew":""}},
				{"type":"appStoreVersionLocalizations","id":"loc-2","attributes":{"locale":"ar-SA","description":"وصف","keywords":"كلمات","supportUrl":"https://example.com","whatsNew":""}}
			]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		default:
			t.Logf("unexpected request: %s %s", req.Method, req.URL.String())
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"404"}]}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-123",
			"--version", "1.1.0",
			"--build", "build-999",
			"--platform", "TV_OS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected preflight error for missing whatsNew")
	}
	if !strings.Contains(runErr.Error(), "submit preflight failed") {
		t.Fatalf("expected preflight failed error, got %v", runErr)
	}
	if !strings.Contains(stderr, "whatsNew") {
		t.Fatalf("expected stderr to mention whatsNew, got %q", stderr)
	}
	if !strings.Contains(stderr, "en-US") {
		t.Fatalf("expected stderr to mention en-US locale, got %q", stderr)
	}
	if !strings.Contains(stderr, "ar-SA") {
		t.Fatalf("expected stderr to mention ar-SA locale, got %q", stderr)
	}
}

func TestSubmitCreatePreflightSkipsWhatsNewForFirstVersionOnDifferentPlatform(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	preflightPassed := false
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/appStoreVersions":
			query := req.URL.Query()
			if strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE") {
				if query.Get("filter[platform]") == "TV_OS" {
					body := `{"data":[]}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(body)),
						Header:     http.Header{"Content-Type": []string{"application/json"}},
					}, nil
				}

				// The app has a shipped iOS version, but this TV_OS submission is a first release.
				body := `{"data":[{"type":"appStoreVersions","id":"version-ios-live","attributes":{"versionString":"1.0.0","appStoreState":"READY_FOR_SALE","platform":"IOS"}}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}

			body := `{"data":[{"type":"appStoreVersions","id":"version-tv","attributes":{"versionString":"1.0.0","appStoreState":"PREPARE_FOR_SUBMISSION","platform":"TV_OS"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-tv/appStoreVersionLocalizations":
			preflightPassed = true
			body := `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"desc","keywords":"kw","supportUrl":"https://example.com","whatsNew":""}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/subscriptionGroups":
			body := `{"data":[]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-tv/relationships/build":
			body := `{"errors":[{"status":"409","detail":"attach build reached"}]}`
			return &http.Response{
				StatusCode: http.StatusConflict,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		default:
			t.Logf("unexpected request: %s %s", req.Method, req.URL.String())
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"404"}]}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-123",
			"--version", "1.0.0",
			"--build", "build-tv",
			"--platform", "TV_OS",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !preflightPassed {
		t.Fatal("localization preflight was never reached")
	}
	if runErr == nil {
		t.Fatal("expected build attach error after passing preflight")
	}
	if !strings.Contains(runErr.Error(), "attach build reached") {
		t.Fatalf("expected attach build error, got %v", runErr)
	}
	if strings.Contains(runErr.Error(), "submit preflight failed") {
		t.Fatalf("expected non-preflight error, got %v", runErr)
	}
	if strings.Contains(stderr, "whatsNew") {
		t.Fatalf("expected stderr not to mention whatsNew, got %q", stderr)
	}
}

func TestSubmitCreatePreflightReturnsUpdateLookupError(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/appStoreVersions":
			query := req.URL.Query()
			if strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE") {
				body := `{"errors":[{"status":"500","detail":"update lookup failed"}]}`
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}

			body := `{"data":[{"type":"appStoreVersions","id":"version-first","attributes":{"versionString":"1.0.0","appStoreState":"PREPARE_FOR_SUBMISSION","platform":"IOS"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-first/appStoreVersionLocalizations":
			body := `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"desc","keywords":"kw","supportUrl":"https://example.com","whatsNew":""}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		default:
			t.Logf("unexpected request: %s %s", req.Method, req.URL.String())
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"404"}]}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-123",
			"--version", "1.0.0",
			"--build", "build-1",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected update lookup error")
	}
	if !strings.Contains(runErr.Error(), "failed to determine whether version is an app update") {
		t.Fatalf("expected update lookup context, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "update lookup failed") {
		t.Fatalf("expected original lookup error, got %v", runErr)
	}
	if strings.Contains(runErr.Error(), "submit preflight failed") {
		t.Fatalf("expected update lookup error, got %v", runErr)
	}
	if strings.Contains(stderr, "whatsNew") {
		t.Fatalf("expected stderr not to mention whatsNew, got %q", stderr)
	}
}

func TestSubmitCreateVersionIDUsesActualVersionPlatform(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	preflightPassed := false
	var reviewSubmissionBody string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-tv":
			body := `{"data":{"type":"appStoreVersions","id":"version-tv","attributes":{"versionString":"1.0.0","appStoreState":"PREPARE_FOR_SUBMISSION","platform":"TV_OS"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-tv/appStoreVersionLocalizations":
			preflightPassed = true
			body := `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"desc","keywords":"kw","supportUrl":"https://example.com","whatsNew":""}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/appStoreVersions":
			query := req.URL.Query()
			if !strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE") {
				t.Fatalf("unexpected app store versions query: %s", req.URL.RawQuery)
			}
			switch query.Get("filter[platform]") {
			case "TV_OS":
				body := `{"data":[]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			case "IOS":
				// This would trigger a false positive if submit create used the default IOS platform.
				body := `{"data":[{"type":"appStoreVersions","id":"version-ios-live","attributes":{"versionString":"1.0.0","appStoreState":"READY_FOR_SALE","platform":"IOS"}}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			default:
				t.Fatalf("unexpected released-version platform filter: %q", query.Get("filter[platform]"))
				return nil, nil
			}

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/subscriptionGroups":
			body := `{"data":[]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-tv/relationships/build":
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/reviewSubmissions":
			if req.URL.Query().Get("filter[state]") != "READY_FOR_REVIEW" || req.URL.Query().Get("filter[platform]") != "TV_OS" {
				t.Fatalf("unexpected review submissions filters: %s", req.URL.RawQuery)
			}
			body := `{"data":[]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read review submission body: %v", err)
			}
			reviewSubmissionBody = string(payload)
			if !strings.Contains(reviewSubmissionBody, `"platform":"TV_OS"`) {
				t.Fatalf("expected review submission payload to use TV_OS, got %s", reviewSubmissionBody)
			}
			body := `{"data":{"type":"reviewSubmissions","id":"rs-1","attributes":{"submittedDate":"2026-03-13T00:00:00Z","platform":"TV_OS"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			body := `{"data":{"type":"reviewSubmissionItems","id":"rsi-1"}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/rs-1":
			body := `{"data":{"type":"reviewSubmissions","id":"rs-1","attributes":{"submittedDate":"2026-03-13T00:00:00Z","platform":"TV_OS"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		default:
			t.Logf("unexpected request: %s %s", req.Method, req.URL.String())
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"404"}]}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-123",
			"--version-id", "version-tv",
			"--build", "build-tv",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr != nil {
		t.Fatalf("expected success using actual version platform, got %v", runErr)
	}
	if !preflightPassed {
		t.Fatal("localization preflight was never reached")
	}
	if reviewSubmissionBody == "" {
		t.Fatal("expected review submission request body to be captured")
	}
	if strings.Contains(stderr, "whatsNew") {
		t.Fatalf("expected stderr not to mention whatsNew, got %q", stderr)
	}
	if strings.TrimSpace(stdout) == "" {
		t.Fatal("expected JSON output on stdout")
	}
}

func TestSubmitCreatePreflightSkipsWhatsNewForFirstVersion(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	preflightPassed := false
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/appStoreVersions":
			query := req.URL.Query()
			if strings.Contains(query.Get("filter[appStoreState]"), "READY_FOR_SALE") {
				// No READY_FOR_SALE version — this is a first release
				body := `{"data":[]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
			body := `{"data":[{"type":"appStoreVersions","id":"version-first","attributes":{"versionString":"1.0.0","appStoreState":"PREPARE_FOR_SUBMISSION","platform":"IOS"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-first/appStoreVersionLocalizations":
			// whatsNew is empty but this is a first version, so it should pass
			body := `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"desc","keywords":"kw","supportUrl":"https://example.com","whatsNew":""}}]}`
			preflightPassed = true
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/subscriptionGroups":
			body := `{"data":[]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-first/relationships/build":
			// Attach build to version
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/reviewSubmissions":
			if req.URL.Query().Get("filter[state]") != "READY_FOR_REVIEW" || req.URL.Query().Get("filter[platform]") != "IOS" {
				t.Fatalf("unexpected review submissions filters: %s", req.URL.RawQuery)
			}
			body := `{"data":[]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			body := `{"data":{"type":"reviewSubmissions","id":"rs-1","attributes":{"submittedDate":"2026-03-13T00:00:00Z"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			body := `{"data":{"type":"reviewSubmissionItems","id":"rsi-1"}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/rs-1":
			body := `{"data":{"type":"reviewSubmissions","id":"rs-1","attributes":{"submittedDate":"2026-03-13T00:00:00Z"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		default:
			t.Logf("unexpected request: %s %s", req.Method, req.URL.String())
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"404"}]}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{
			"submit", "create",
			"--app", "app-123",
			"--version", "1.0.0",
			"--build", "build-1",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("expected success for first version without whatsNew, got %v", err)
		}
	})

	if !preflightPassed {
		t.Fatal("localization preflight was never reached")
	}
}
