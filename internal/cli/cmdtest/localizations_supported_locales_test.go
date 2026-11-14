package cmdtest

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestLocalizationsSupportedLocalesSuccess(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[
			{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","keywords":"baseline,copy"}},
			{"type":"appStoreVersionLocalizations","id":"loc-bn","attributes":{"locale":"bn-BD","keywords":"bangla,copy"}}
		],"links":{}}`))

	stdout, stderr, err := runCommand(t, []string{
		"localizations", "supported-locales",
		"--version", "ver-1",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		VersionID          string `json:"versionId"`
		CatalogLocaleCount int    `json:"catalogLocaleCount"`
		ConfiguredCount    int    `json:"configuredCount"`
		Locales            []struct {
			Locale         string `json:"locale"`
			Name           string `json:"name"`
			Configured     bool   `json:"configured"`
			LocalizationID string `json:"localizationId,omitempty"`
		} `json:"locales"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout should be valid json: %v\nstdout=%q", err, stdout)
	}

	if out.VersionID != "ver-1" {
		t.Fatalf("expected versionId ver-1, got %q", out.VersionID)
	}
	if out.CatalogLocaleCount != 50 {
		t.Fatalf("expected catalogLocaleCount 50, got %d", out.CatalogLocaleCount)
	}
	if out.ConfiguredCount != 2 {
		t.Fatalf("expected configuredCount 2, got %d", out.ConfiguredCount)
	}

	var seenBangla, seenSlovenian bool
	for _, item := range out.Locales {
		switch item.Locale {
		case "bn-BD":
			seenBangla = true
			if !item.Configured {
				t.Fatalf("expected bn-BD to be configured, got %+v", item)
			}
			if item.LocalizationID != "loc-bn" {
				t.Fatalf("expected bn-BD localization id loc-bn, got %+v", item)
			}
		case "sl-SI":
			seenSlovenian = true
			if item.Name == "" {
				t.Fatalf("expected sl-SI to include a display name, got %+v", item)
			}
			if item.Configured {
				t.Fatalf("expected sl-SI to be unconfigured, got %+v", item)
			}
		}
	}

	if !seenBangla {
		t.Fatalf("expected locales to include bn-BD, got %+v", out.Locales)
	}
	if !seenSlovenian {
		t.Fatalf("expected locales to include sl-SI, got %+v", out.Locales)
	}
}

func TestLocalizationsSupportedLocalesPaginatesConfiguredLocales(t *testing.T) {
	setupAuth(t)

	nextURL := "https://api.appstoreconnect.apple.com/v1/appStoreVersions/ver-1/appStoreVersionLocalizations?cursor=next"
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}],"links":{"next":"`+nextURL+`"}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/ver-1/appStoreVersionLocalizations", "cursor=next"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja"}}],"links":{}}`))

	stdout, stderr, err := runCommand(t, []string{
		"localizations", "supported-locales",
		"--version", "ver-1",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	requests := make([]string, 0, 2)
	for _, req := range reg.Requests() {
		requests = append(requests, req.URL.String())
	}
	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d (%v)", len(requests), requests)
	}

	var out struct {
		ConfiguredCount int `json:"configuredCount"`
		Locales         []struct {
			Locale     string `json:"locale"`
			Configured bool   `json:"configured"`
		} `json:"locales"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout should be valid json: %v\nstdout=%q", err, stdout)
	}
	if out.ConfiguredCount != 2 {
		t.Fatalf("expected configuredCount 2, got %d", out.ConfiguredCount)
	}

	configuredLocales := make(map[string]bool, len(out.Locales))
	for _, item := range out.Locales {
		configuredLocales[item.Locale] = item.Configured
	}
	if !configuredLocales["en-US"] || !configuredLocales["ja"] {
		t.Fatalf("expected en-US and ja to be configured, got %+v", configuredLocales)
	}
}
