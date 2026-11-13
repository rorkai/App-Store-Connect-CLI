package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestLocalizationsCreateSuccess(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var seenPayload struct {
		Data struct {
			Attributes struct {
				Locale      string `json:"locale"`
				Description string `json:"description"`
				Keywords    string `json:"keywords"`
				SupportURL  string `json:"supportUrl"`
				WhatsNew    string `json:"whatsNew"`
			} `json:"attributes"`
			Relationships struct {
				AppStoreVersion struct {
					Data struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"appStoreVersion"`
			} `json:"relationships"`
		} `json:"data"`
	}

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			body := `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"2.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			body := `{"data":[],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersionLocalizations":
			if err := json.NewDecoder(req.Body).Decode(&seenPayload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}

			body := `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"ja","description":"Hello","keywords":"foo,bar","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"localizations", "create",
			"--version", "version-1",
			"--locale", "ja",
			"--description", "  Hello  ",
			"--keywords", "foo,bar",
			"--support-url", " https://example.com/support ",
			"--whats-new", " Bug fixes ",
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

	var out struct {
		Data struct {
			ID         string `json:"id"`
			Attributes struct {
				Locale string `json:"locale"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout should be valid json: %v\nstdout=%q", err, stdout)
	}
	if out.Data.ID != "loc-1" {
		t.Fatalf("expected localization id loc-1, got %q", out.Data.ID)
	}
	if out.Data.Attributes.Locale != "ja" {
		t.Fatalf("expected locale ja, got %q", out.Data.Attributes.Locale)
	}
	if seenPayload.Data.Relationships.AppStoreVersion.Data.ID != "version-1" {
		t.Fatalf("expected version relationship version-1, got %q", seenPayload.Data.Relationships.AppStoreVersion.Data.ID)
	}
	if seenPayload.Data.Attributes.Locale != "ja" {
		t.Fatalf("expected locale ja, got %q", seenPayload.Data.Attributes.Locale)
	}
	if seenPayload.Data.Attributes.Description != "Hello" {
		t.Fatalf("expected trimmed description Hello, got %q", seenPayload.Data.Attributes.Description)
	}
	if seenPayload.Data.Attributes.Keywords != "foo,bar" {
		t.Fatalf("expected keywords foo,bar, got %q", seenPayload.Data.Attributes.Keywords)
	}
	if seenPayload.Data.Attributes.SupportURL != "https://example.com/support" {
		t.Fatalf("expected trimmed support url, got %q", seenPayload.Data.Attributes.SupportURL)
	}
	if seenPayload.Data.Attributes.WhatsNew != "Bug fixes" {
		t.Fatalf("expected trimmed whatsNew, got %q", seenPayload.Data.Attributes.WhatsNew)
	}
}

func TestLocalizationsCreateSubmitReadinessWarnings(t *testing.T) {
	const (
		versionWithApp = `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`
		noReleased     = `{"data":[]}`
		released       = `{"data":[{"type":"appStoreVersions","id":"live-1","attributes":{"platform":"IOS","appVersionState":"READY_FOR_SALE"}}]}`
	)
	tests := []struct {
		name            string
		args            []string
		createdID       string
		createdAttrs    string
		versionStatus   int
		releasedBody    string
		wantStderr      []string
		wantEmptyStderr bool
	}{
		{
			name:          "new locale missing submit fields",
			args:          []string{"--locale", "ja", "--keywords", "集中,勉強タイマー,集中タイマー"},
			createdID:     "loc-2",
			createdAttrs:  `{"locale":"ja","keywords":"集中,勉強タイマー,集中タイマー"}`,
			versionStatus: http.StatusOK,
			releasedBody:  noReleased,
			wantStderr:    []string{"created locale ja now participates in submission validation", "description", "supportUrl"},
		},
		{
			name:          "update version missing whats new",
			args:          []string{"--locale", "en-US", "--description", "Updated app", "--keywords", "timer,focus", "--support-url", "https://example.com/support"},
			createdID:     "loc-4",
			createdAttrs:  `{"locale":"en-US","description":"Updated app","keywords":"timer,focus","supportUrl":"https://example.com/support"}`,
			versionStatus: http.StatusOK,
			releasedBody:  released,
			wantStderr:    []string{"submit-required fields", "whatsNew"},
		},
		{
			name:          "update version lists whats new alongside base fields",
			args:          []string{"--locale", "en-US", "--keywords", "timer,focus"},
			createdID:     "loc-4b",
			createdAttrs:  `{"locale":"en-US","keywords":"timer,focus"}`,
			versionStatus: http.StatusOK,
			releasedBody:  released,
			wantStderr:    []string{"description", "supportUrl", "whatsNew"},
		},
		{
			name:          "readiness lookup failure stays a warning",
			args:          []string{"--locale", "en-US", "--description", "Updated app", "--keywords", "timer,focus", "--support-url", "https://example.com/support"},
			createdID:     "loc-5",
			createdAttrs:  `{"locale":"en-US","description":"Updated app","keywords":"timer,focus","supportUrl":"https://example.com/support"}`,
			versionStatus: http.StatusInternalServerError,
			wantStderr:    []string{"could not determine whether this version is an app update"},
		},
		{
			name:            "complete create does not warn",
			args:            []string{"--locale", "ja", "--description", "Hello", "--keywords", "foo,bar", "--support-url", "https://example.com/support", "--whats-new", "Fixes"},
			createdID:       "loc-2",
			createdAttrs:    `{"locale":"ja","description":"Hello","keywords":"foo,bar","supportUrl":"https://example.com/support","whatsNew":"Fixes"}`,
			wantEmptyStderr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersionLocalizations":
					return jsonResponse(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"`+test.createdID+`","attributes":`+test.createdAttrs+`}}`)
				case test.versionStatus != 0 && req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
					if test.versionStatus != http.StatusOK {
						return jsonResponse(test.versionStatus, `{"errors":[{"status":"500","code":"INTERNAL_SERVER_ERROR","title":"Internal Server Error"}]}`)
					}
					if got := req.URL.Query().Get("include"); got != "app" {
						t.Fatalf("expected include=app, got %q", got)
					}
					return jsonResponse(http.StatusOK, versionWithApp)
				case test.releasedBody != "" && req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
					query := req.URL.Query()
					if !isReleasedVersionStateQuery(query) {
						t.Fatalf("expected released-state filter, got %q", req.URL.RawQuery)
					}
					if got := query.Get("filter[platform]"); got != "IOS" {
						t.Fatalf("expected platform filter IOS, got %q", got)
					}
					if got := query.Get("limit"); got != "1" {
						t.Fatalf("expected limit=1, got %q", got)
					}
					return jsonResponse(http.StatusOK, test.releasedBody)
				default:
					t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
					return nil, nil
				}
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(append([]string{"localizations", "create", "--version", "version-1"}, test.args...)); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if test.wantEmptyStderr && stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			for _, want := range test.wantStderr {
				if !strings.Contains(stderr, want) {
					t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
				}
			}

			var out struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(stdout), &out); err != nil {
				t.Fatalf("stdout should be valid json: %v\nstdout=%q", err, stdout)
			}
			if out.Data.ID != test.createdID {
				t.Fatalf("expected localization id %s, got %q", test.createdID, out.Data.ID)
			}
		})
	}
}

func TestLocalizationsCreate_InvalidLocaleReturnsUsageError(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"localizations", "create",
			"--version", "version-1",
			"--locale", "not_a_locale",
		}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, `invalid locale "not_a_locale"`) {
		t.Fatalf("expected invalid locale error, got %q", stderr)
	}
	if requestCount != 0 {
		t.Fatalf("expected no HTTP requests, got %d", requestCount)
	}
}

func TestLocalizationsCreate_NormalizesKnownLocaleCodes(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var seenLocale string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersionLocalizations":
			var payload struct {
				Data struct {
					Attributes struct {
						Locale string `json:"locale"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			seenLocale = payload.Data.Attributes.Locale

			body := `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Updated app","keywords":"timer,focus","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.Path)
		}
		return nil, nil
	})

	stdout, stderr := captureOutput(t, func() {
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		if err := root.Parse([]string{
			"localizations", "create",
			"--version", "version-1",
			"--locale", "en_us",
			"--description", "Updated app",
			"--keywords", "timer,focus",
			"--support-url", "https://example.com/support",
			"--whats-new", "Bug fixes",
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
	if !strings.Contains(stdout, `"locale":"en-US"`) {
		t.Fatalf("expected canonicalized locale in output, got %q", stdout)
	}
	if seenLocale != "en-US" {
		t.Fatalf("expected request locale en-US, got %q", seenLocale)
	}
}

func TestLocalizationsCreate_RejectsUnsupportedLocaleWithSuggestion(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"localizations", "create",
			"--version", "version-1",
			"--locale", "nl",
		}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, want := range []string{`unsupported locale "nl"`, "nl-NL"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}
	if requestCount != 0 {
		t.Fatalf("expected no HTTP requests, got %d", requestCount)
	}
}

func TestLocalizationsCreate_AllowsForwardCompatibleLocaleCodes(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var seenLocale string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/appStoreVersionLocalizations" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}

		var payload struct {
			Data struct {
				Attributes struct {
					Locale string `json:"locale"`
				} `json:"attributes"`
			} `json:"data"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		seenLocale = payload.Data.Attributes.Locale

		body := `{"data":{"type":"appStoreVersionLocalizations","id":"loc-forward","attributes":{"locale":"zh-Hant-HK","description":"Updated app","keywords":"timer,focus","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	stdout, stderr := captureOutput(t, func() {
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		if err := root.Parse([]string{
			"localizations", "create",
			"--version", "version-1",
			"--locale", "zh-Hant-HK",
			"--description", "Updated app",
			"--keywords", "timer,focus",
			"--support-url", "https://example.com/support",
			"--whats-new", "Bug fixes",
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
	if !strings.Contains(stdout, `"locale":"zh-Hant-HK"`) {
		t.Fatalf("expected forward-compatible locale in output, got %q", stdout)
	}
	if seenLocale != "zh-Hant-HK" {
		t.Fatalf("expected request locale zh-Hant-HK, got %q", seenLocale)
	}
}

func TestLocalizationsCreate_RejectsPositionalArgs(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"localizations", "create",
			"--version", "version-1",
			"--locale", "ja",
			"extra",
		}, "1.2.3")
		if code != cmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, `unexpected argument "extra"`) {
		t.Fatalf("expected positional-args error, got %q", stderr)
	}
	if requestCount != 0 {
		t.Fatalf("expected no HTTP requests, got %d", requestCount)
	}
}
