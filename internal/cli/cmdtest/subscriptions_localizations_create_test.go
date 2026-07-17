package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestSubscriptionsLocalizationsCreateReusesMatchingLocale(t *testing.T) {
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
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/123456789/subscriptionLocalizations" {
				t.Fatalf("unexpected lookup request: %s %s", req.Method, req.URL.String())
			}
			if got := req.URL.Query().Get("limit"); got != "200" {
				t.Fatalf("expected limit=200 lookup, got %q", got)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionLocalizations","id":"loc-1","attributes":{"name":"Pro","locale":"en-US","description":"Premium features."}}],"links":{"next":""}}`), nil
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
			"subscriptions", "localizations", "create",
			"--subscription-id", "123456789",
			"--locale", "en-us",
			"--name", "Pro",
			"--description", "Premium features.",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	assertOnlyCommandDeprecationWarning(t, stderr, subscriptionsLocalizationsCreateDeprecationWarning)
	if requestCount != 1 {
		t.Fatalf("expected only localization lookup request, got %d", requestCount)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse output: %v\nstdout=%q", err, stdout)
	}
	if result.Data.ID != "loc-1" || result.Data.Attributes.Locale != "en-US" || result.Data.Attributes.Name != "Pro" {
		t.Fatalf("unexpected reused localization output: %+v", result)
	}
}

func TestSubscriptionsLocalizationsCreateRejectsDifferentExistingLocale(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPost {
			t.Fatalf("create should reject mismatched existing localization before POST")
		}
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/123456789/subscriptionLocalizations" {
			t.Fatalf("unexpected lookup request: %s %s", req.Method, req.URL.String())
		}
		return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionLocalizations","id":"loc-1","attributes":{"name":"Old Pro","locale":"en-US","description":"Old description."}}],"links":{"next":""}}`), nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "localizations", "create",
			"--subscription-id", "123456789",
			"--locale", "en-US",
			"--name", "Pro",
			"--description", "Premium features.",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil {
		t.Fatal("expected existing localization mismatch error")
	}
	if !errors.Is(runErr, asc.ErrConflict) {
		t.Fatalf("expected conflict error, got %v", runErr)
	}

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	assertOnlyCommandDeprecationWarning(t, stderr, subscriptionsLocalizationsCreateDeprecationWarning)
	if !strings.Contains(runErr.Error(), `localization for locale "en-US" already exists as loc-1`) {
		t.Fatalf("expected existing localization guidance, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "subscriptions localizations update --id loc-1") {
		t.Fatalf("expected update guidance, got %v", runErr)
	}
}
