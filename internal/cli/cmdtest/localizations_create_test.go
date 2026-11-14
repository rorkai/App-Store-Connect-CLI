package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestLocalizationsCreateSuccess(t *testing.T) {
	setupAuth(t)

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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"ja","description":"Hello","keywords":"foo,bar","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"ja","description":"Hello","keywords":"foo,bar","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"localizations", "create",
		"--version", "version-1",
		"--locale", "ja",
		"--description", "  Hello  ",
		"--keywords", "foo,bar",
		"--support-url", " https://example.com/support ",
		"--whats-new", " Bug fixes ",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

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
	for _, req := range reg.Requests() {
		if req.Method == http.MethodPost {
			if err := json.Unmarshal(req.Body, &seenPayload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
		}
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
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.POST("/v1/appStoreVersionLocalizations"),
				asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"`+test.createdID+`","attributes":`+test.createdAttrs+`}}`))
			if test.versionStatus == http.StatusOK {
				reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"), asctest.JSON(http.StatusOK, versionWithApp))
			} else if test.versionStatus != 0 {
				reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"),
					asctest.JSON(test.versionStatus, `{"errors":[{"status":"500","code":"INTERNAL_SERVER_ERROR","title":"Internal Server Error"}]}`))
			}
			if test.releasedBody != "" {
				reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE", "filter[platform]=IOS", "limit=1"),
					asctest.JSON(http.StatusOK, test.releasedBody))
			}
			if test.releasedBody == noReleased {
				reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appVersionState]=READY_FOR_DISTRIBUTION", "filter[platform]=IOS", "limit=1"),
					asctest.JSON(http.StatusOK, test.releasedBody))
			}

			stdout, stderr, err := runCommand(t, append([]string{"localizations", "create", "--version", "version-1"}, test.args...))
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

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

	reg := asctest.NewRegistry(t)
	reg.Install()

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
	if requestCount := len(reg.Requests()); requestCount != 0 {
		t.Fatalf("expected no HTTP requests, got %d", requestCount)
	}
}

func TestLocalizationsCreate_NormalizesKnownLocaleCodes(t *testing.T) {
	setupAuth(t)

	var seenLocale string
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"en-US","description":"Updated app","keywords":"timer,focus","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Updated app","keywords":"timer,focus","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}}`))

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
	for _, req := range reg.Requests() {
		var payload struct {
			Data struct {
				Attributes struct {
					Locale string `json:"locale"`
				} `json:"attributes"`
			} `json:"data"`
		}
		if err := json.Unmarshal(req.Body, &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		seenLocale = payload.Data.Attributes.Locale
	}
	if seenLocale != "en-US" {
		t.Fatalf("expected request locale en-US, got %q", seenLocale)
	}
}

func TestLocalizationsCreate_RejectsUnsupportedLocaleWithSuggestion(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()

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
	if requestCount := len(reg.Requests()); requestCount != 0 {
		t.Fatalf("expected no HTTP requests, got %d", requestCount)
	}
}

func TestLocalizationsCreate_AllowsForwardCompatibleLocaleCodes(t *testing.T) {
	setupAuth(t)

	var seenLocale string
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"zh-Hant-HK","description":"Updated app","keywords":"timer,focus","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-forward","attributes":{"locale":"zh-Hant-HK","description":"Updated app","keywords":"timer,focus","supportUrl":"https://example.com/support","whatsNew":"Bug fixes"}}}`))

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
	for _, req := range reg.Requests() {
		var payload struct {
			Data struct {
				Attributes struct {
					Locale string `json:"locale"`
				} `json:"attributes"`
			} `json:"data"`
		}
		if err := json.Unmarshal(req.Body, &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		seenLocale = payload.Data.Attributes.Locale
	}
	if seenLocale != "zh-Hant-HK" {
		t.Fatalf("expected request locale zh-Hant-HK, got %q", seenLocale)
	}
}

func TestLocalizationsCreate_RejectsPositionalArgs(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()

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
	if requestCount := len(reg.Requests()); requestCount != 0 {
		t.Fatalf("expected no HTTP requests, got %d", requestCount)
	}
}
