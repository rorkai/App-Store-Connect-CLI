package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestIAPLocalizationsCreateReusesMatchingLocale(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method == http.MethodPost {
			t.Fatalf("create should reuse matching localization, got POST %s", req.URL.Path)
		}
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/123456789/inAppPurchaseLocalizations" {
				t.Fatalf("unexpected lookup request: %s %s", req.Method, req.URL.String())
			}
			if got := req.URL.Query().Get("limit"); got != "200" {
				t.Fatalf("expected limit=200 lookup, got %q", got)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"inAppPurchaseLocalizations","id":"loc-1","attributes":{"name":"Coins","locale":"en-US","description":"Pack of coins."}}],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var result struct {
		Data struct {
			ID         string `json:"id"`
			Attributes struct {
				Name        string `json:"name"`
				Locale      string `json:"locale"`
				Description string `json:"description"`
			} `json:"attributes"`
		} `json:"data"`
	}
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "localizations", "create",
			"--iap-id", "123456789",
			"--locale", "en-us",
			"--name", "Coins",
			"--description", "Pack of coins.",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	assertOnlyDeprecatedCommandWarnings(t, stderr)
	if requestCount != 1 {
		t.Fatalf("expected only localization lookup request, got %d", requestCount)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse output: %v\nstdout=%q", err, stdout)
	}
	if result.Data.ID != "loc-1" || result.Data.Attributes.Locale != "en-US" || result.Data.Attributes.Name != "Coins" {
		t.Fatalf("unexpected reused localization output: %+v", result)
	}
}

func TestIAPLocalizationsCreateRejectsDifferentExistingLocale(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPost {
			t.Fatalf("create should reject mismatched existing localization before POST")
		}
		if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/123456789/inAppPurchaseLocalizations" {
			t.Fatalf("unexpected lookup request: %s %s", req.Method, req.URL.String())
		}
		return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"inAppPurchaseLocalizations","id":"loc-1","attributes":{"name":"Old Coins","locale":"en-US","description":"Old description."}}],"links":{"next":""}}`), nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "localizations", "create",
			"--iap-id", "123456789",
			"--locale", "en-US",
			"--name", "Coins",
			"--description", "Pack of coins.",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected existing localization mismatch error")
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, `localization for locale "en-US" already exists as loc-1`) {
		t.Fatalf("expected existing localization guidance, got %q", stderr)
	}
	if !strings.Contains(stderr, "iap localizations update --localization-id loc-1") {
		t.Fatalf("expected update guidance, got %q", stderr)
	}
}
