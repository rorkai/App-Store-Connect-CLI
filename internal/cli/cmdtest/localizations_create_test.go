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

func TestLocalizationsCreate_WarnsWhenCreatedLocaleIsSubmitIncomplete(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersionLocalizations":
			body := `{"data":{"type":"appStoreVersionLocalizations","id":"loc-2","attributes":{"locale":"ja","keywords":"集中,勉強タイマー,集中タイマー"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			if got := req.URL.Query().Get("include"); got != "app" {
				t.Fatalf("expected include=app, got %q", got)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			if got := query.Get("filter[appStoreState]"); got != "READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE" {
				t.Fatalf("expected released-state filter, got %q", got)
			}
			if got := query.Get("filter[platform]"); got != "IOS" {
				t.Fatalf("expected platform filter IOS, got %q", got)
			}
			if got := query.Get("limit"); got != "1" {
				t.Fatalf("expected limit=1, got %q", got)
			}
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
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
			"--keywords", "集中,勉強タイマー,集中タイマー",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "created locale ja now participates in submission validation") {
		t.Fatalf("expected submit-required warning in stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "description") || !strings.Contains(stderr, "supportUrl") {
		t.Fatalf("expected missing submit-required fields in warning, got %q", stderr)
	}

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout should be valid json: %v\nstdout=%q", err, stdout)
	}
	if out.Data.ID != "loc-2" {
		t.Fatalf("expected localization id loc-2, got %q", out.Data.ID)
	}
}

func TestLocalizationsCreate_DoesNotWarnWhenCreatedLocaleIsSubmitComplete(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/appStoreVersionLocalizations" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}

		body := `{"data":{"type":"appStoreVersionLocalizations","id":"loc-3","attributes":{"locale":"de-DE","description":"Meine App","keywords":"schluessel,woerter","supportUrl":"https://example.com/support","whatsNew":"Fehlerbehebungen"}}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"localizations", "create",
			"--version", "version-1",
			"--locale", "de-DE",
			"--description", "Meine App",
			"--keywords", "schluessel,woerter",
			"--support-url", "https://example.com/support",
			"--whats-new", "Fehlerbehebungen",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if strings.Contains(stderr, "submit-required fields") {
		t.Fatalf("did not expect submit-required warning, got %q", stderr)
	}
}

func TestLocalizationsCreate_WarnsWhenUpdateVersionIsMissingWhatsNew(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersionLocalizations":
			body := `{"data":{"type":"appStoreVersionLocalizations","id":"loc-4","attributes":{"locale":"en-US","description":"Updated app","keywords":"timer,focus","supportUrl":"https://example.com/support"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			if got := req.URL.Query().Get("include"); got != "app" {
				t.Fatalf("expected include=app, got %q", got)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			if got := query.Get("filter[appStoreState]"); got != "READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE" {
				t.Fatalf("expected released-state filter, got %q", got)
			}
			if got := query.Get("filter[platform]"); got != "IOS" {
				t.Fatalf("expected platform filter IOS, got %q", got)
			}
			if got := query.Get("limit"); got != "1" {
				t.Fatalf("expected limit=1, got %q", got)
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"live-1","attributes":{"platform":"IOS","appVersionState":"READY_FOR_SALE"}}]}`)
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"localizations", "create",
			"--version", "version-1",
			"--locale", "en-US",
			"--description", "Updated app",
			"--keywords", "timer,focus",
			"--support-url", "https://example.com/support",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "submit-required fields") || !strings.Contains(stderr, "whatsNew") {
		t.Fatalf("expected whatsNew submit warning in stderr, got %q", stderr)
	}

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout should be valid json: %v\nstdout=%q", err, stdout)
	}
	if out.Data.ID != "loc-4" {
		t.Fatalf("expected localization id loc-4, got %q", out.Data.ID)
	}
}

func TestLocalizationsCreate_UpdateWarningIncludesWhatsNewAlongsideBaseMissingFields(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersionLocalizations":
			body := `{"data":{"type":"appStoreVersionLocalizations","id":"loc-4b","attributes":{"locale":"en-US","keywords":"timer,focus"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			if got := req.URL.Query().Get("include"); got != "app" {
				t.Fatalf("expected include=app, got %q", got)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			if got := query.Get("filter[appStoreState]"); got != "READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE" {
				t.Fatalf("expected released-state filter, got %q", got)
			}
			if got := query.Get("filter[platform]"); got != "IOS" {
				t.Fatalf("expected platform filter IOS, got %q", got)
			}
			if got := query.Get("limit"); got != "1" {
				t.Fatalf("expected limit=1, got %q", got)
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"live-1","attributes":{"platform":"IOS","appVersionState":"READY_FOR_SALE"}}]}`)
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"localizations", "create",
			"--version", "version-1",
			"--locale", "en-US",
			"--keywords", "timer,focus",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	for _, field := range []string{"description", "supportUrl", "whatsNew"} {
		if !strings.Contains(stderr, field) {
			t.Fatalf("expected %s in submit warning, got %q", field, stderr)
		}
	}
}

func TestLocalizationsCreate_DoesNotFailWhenUpdateReadinessLookupFails(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersionLocalizations":
			body := `{"data":{"type":"appStoreVersionLocalizations","id":"loc-5","attributes":{"locale":"en-US","description":"Updated app","keywords":"timer,focus","supportUrl":"https://example.com/support"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1":
			return jsonResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_SERVER_ERROR","title":"Internal Server Error"}]}`)
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"localizations", "create",
			"--version", "version-1",
			"--locale", "en-US",
			"--description", "Updated app",
			"--keywords", "timer,focus",
			"--support-url", "https://example.com/support",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "could not determine whether this version is an app update") {
		t.Fatalf("expected soft readiness lookup warning, got %q", stderr)
	}

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout should be valid json: %v\nstdout=%q", err, stdout)
	}
	if out.Data.ID != "loc-5" {
		t.Fatalf("expected localization id loc-5, got %q", out.Data.ID)
	}
}

func TestLocalizationsCreateWarnsForIncompleteCreate(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

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
			body := `{"data":{"type":"appStoreVersionLocalizations","id":"loc-3","attributes":{"locale":"ja","description":"Hello"}}}`
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
			"--description", "Hello",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "created locale ja now participates in submission validation") {
		t.Fatalf("expected create warning on stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "keywords, supportUrl") {
		t.Fatalf("expected missing fields in warning, got %q", stderr)
	}

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout should be valid json: %v\nstdout=%q", err, stdout)
	}
	if out.Data.ID != "loc-3" {
		t.Fatalf("expected localization id loc-3, got %q", out.Data.ID)
	}
}

func TestLocalizationsCreateCompleteCreateDoesNotWarn(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/appStoreVersionLocalizations" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}

		body := `{"data":{"type":"appStoreVersionLocalizations","id":"loc-2","attributes":{"locale":"ja","description":"Hello","keywords":"foo,bar","supportUrl":"https://example.com/support","whatsNew":"Fixes"}}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"localizations", "create",
			"--version", "version-1",
			"--locale", "ja",
			"--description", "Hello",
			"--keywords", "foo,bar",
			"--support-url", "https://example.com/support",
			"--whats-new", "Fixes",
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
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout should be valid json: %v\nstdout=%q", err, stdout)
	}
	if out.Data.ID != "loc-2" {
		t.Fatalf("expected localization id loc-2, got %q", out.Data.ID)
	}
}

func TestLocalizationsCreateWarnsWhenUpdateRequiresWhatsNew(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

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
			body := `{"data":[{"type":"appStoreVersions","id":"released-version","attributes":{"versionString":"1.0","platform":"IOS","appStoreState":"READY_FOR_SALE"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersionLocalizations":
			body := `{"data":{"type":"appStoreVersionLocalizations","id":"loc-4","attributes":{"locale":"ja","description":"Hello","keywords":"foo,bar","supportUrl":"https://example.com/support"}}}`
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
			"--description", "Hello",
			"--keywords", "foo,bar",
			"--support-url", "https://example.com/support",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "created locale ja now participates in submission validation") {
		t.Fatalf("expected create warning on stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "whatsNew") {
		t.Fatalf("expected whatsNew requirement in warning, got %q", stderr)
	}

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout should be valid json: %v\nstdout=%q", err, stdout)
	}
	if out.Data.ID != "loc-4" {
		t.Fatalf("expected localization id loc-4, got %q", out.Data.ID)
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
	if !strings.Contains(stderr, "localizations create does not accept positional arguments") {
		t.Fatalf("expected positional-args error, got %q", stderr)
	}
	if requestCount != 0 {
		t.Fatalf("expected no HTTP requests, got %d", requestCount)
	}
}
