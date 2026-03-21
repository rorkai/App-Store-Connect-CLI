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
		if req.Method != http.MethodPost || req.URL.Path != "/v1/appStoreVersionLocalizations" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if err := json.NewDecoder(req.Body).Decode(&seenPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}

		body := `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"ja","description":"Hello","keywords":"foo,bar","supportUrl":"https://example.com/support"}}}`
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
			"--description", "  Hello  ",
			"--keywords", "foo,bar",
			"--support-url", " https://example.com/support ",
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
