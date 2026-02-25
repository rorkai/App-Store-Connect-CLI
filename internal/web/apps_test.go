package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeCreateAttrsDefaults(t *testing.T) {
	attrs, err := normalizeCreateAttrs(AppCreateAttributes{
		Name:     "My App",
		BundleID: "com.example.app",
		SKU:      "SKU123",
	})
	if err != nil {
		t.Fatalf("normalizeCreateAttrs error: %v", err)
	}
	if attrs.PrimaryLocale != defaultPrimaryLocale {
		t.Fatalf("expected default locale %q, got %q", defaultPrimaryLocale, attrs.PrimaryLocale)
	}
	if attrs.Platform != defaultPlatform {
		t.Fatalf("expected default platform %q, got %q", defaultPlatform, attrs.Platform)
	}
	if attrs.VersionString != defaultVersion {
		t.Fatalf("expected default version %q, got %q", defaultVersion, attrs.VersionString)
	}
}

func TestNormalizeCreateAttrsRejectsInvalidPlatform(t *testing.T) {
	_, err := normalizeCreateAttrs(AppCreateAttributes{
		Name:     "My App",
		BundleID: "com.example.app",
		SKU:      "SKU123",
		Platform: "WATCH_OS",
	})
	if err == nil {
		t.Fatal("expected invalid platform error")
	}
}

func TestBuildAppCreateRequestUsesLocalizationForName(t *testing.T) {
	req := buildAppCreateRequest(AppCreateAttributes{
		Name:          "My App",
		BundleID:      "com.example.app",
		SKU:           "SKU123",
		PrimaryLocale: "en-US",
		Platform:      "IOS",
		VersionString: "1.0",
	})

	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	payload := string(raw)

	if strings.Contains(payload, `"attributes":{"name":"My App","sku"`) {
		t.Fatalf("expected name not to be part of top-level app attributes, payload=%s", payload)
	}
	if !strings.Contains(payload, `"appInfoLocalizations"`) {
		t.Fatalf("expected appInfoLocalization relationship, payload=%s", payload)
	}
	if !strings.Contains(payload, `"name":"My App"`) {
		t.Fatalf("expected localized app name in payload, payload=%s", payload)
	}
}

func TestFindAppEscapesBundleIDQuery(t *testing.T) {
	var gotRawQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
	}
	_, err := client.FindApp(context.Background(), "com.example/app?x=1")
	if err != nil {
		t.Fatalf("FindApp error: %v", err)
	}
	if strings.Contains(gotRawQuery, "com.example/app?x=1") {
		t.Fatalf("expected escaped query value, got raw query %q", gotRawQuery)
	}
	if !strings.Contains(gotRawQuery, "filter%5BbundleId%5D=") && !strings.Contains(gotRawQuery, "filter[bundleId]=") {
		t.Fatalf("expected bundleId filter query, got %q", gotRawQuery)
	}
}
