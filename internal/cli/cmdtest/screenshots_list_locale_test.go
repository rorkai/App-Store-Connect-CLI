package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func installScreenshotsListTestClient(t *testing.T, handler http.HandlerFunc) {
	t.Helper()

	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	})
	client, err := asc.NewClientWithHTTPClient(
		os.Getenv("ASC_KEY_ID"),
		os.Getenv("ASC_ISSUER_ID"),
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("create screenshots list test client: %v", err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))
}

func TestScreenshotsListWithoutLocaleListsEveryLocalization(t *testing.T) {
	installScreenshotsListTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v1/apps/123456789/appStoreVersions":
			writeScreenshotsListJSON(t, w, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{}}`)
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			writeScreenshotsListJSON(t, w, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}},{"type":"appStoreVersionLocalizations","id":"loc-de","attributes":{"locale":"de-DE"}}],"links":{}}`)
		case "/v1/appStoreVersionLocalizations/loc-en/appScreenshotSets":
			writeScreenshotsListJSON(t, w, `{"data":[{"type":"appScreenshotSets","id":"set-en","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case "/v1/appScreenshotSets/set-en/appScreenshots":
			writeScreenshotsListJSON(t, w, `{"data":[{"type":"appScreenshots","id":"shot-en","attributes":{"fileName":"home.png","fileSize":42}}],"links":{}}`)
		case "/v1/appStoreVersionLocalizations/loc-de/appScreenshotSets":
			writeScreenshotsListJSON(t, w, `{"data":[],"links":{}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})

	stdout, stderr, runErr := runRootCommand(t, []string{
		"screenshots", "list",
		"--app", "123456789",
		"--version", "1.2.3",
		"--output", "json",
	})
	if runErr != nil {
		t.Fatalf("screenshots list error: %v (stderr %q)", runErr, stderr)
	}
	if !strings.Contains(stderr, "--locale not set") {
		t.Fatalf("stderr = %q, want all-localization note", stderr)
	}

	var payload struct {
		VersionLocalizationID string `json:"versionLocalizationId"`
		Localizations         []struct {
			Locale                string `json:"locale"`
			VersionLocalizationID string `json:"versionLocalizationId"`
			Sets                  []struct {
				Set struct {
					ID string `json:"id"`
				} `json:"set"`
				Screenshots []struct {
					ID string `json:"id"`
				} `json:"screenshots"`
			} `json:"sets"`
		} `json:"localizations"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal stdout %q: %v", stdout, err)
	}
	if payload.VersionLocalizationID != "" {
		t.Fatalf("versionLocalizationId = %q, want empty", payload.VersionLocalizationID)
	}
	if len(payload.Localizations) != 2 {
		t.Fatalf("localizations = %#v, want 2 entries", payload.Localizations)
	}
	if payload.Localizations[0].Locale != "de-DE" || len(payload.Localizations[0].Sets) != 0 {
		t.Fatalf("first localization = %#v, want de-DE without sets", payload.Localizations[0])
	}
	second := payload.Localizations[1]
	if second.Locale != "en-US" || second.VersionLocalizationID != "loc-en" {
		t.Fatalf("second localization = %#v, want en-US/loc-en", second)
	}
	if len(second.Sets) != 1 || second.Sets[0].Set.ID != "set-en" || len(second.Sets[0].Screenshots) != 1 {
		t.Fatalf("en-US sets = %#v, want set-en with one screenshot", second.Sets)
	}
}

func TestScreenshotsListWithoutLocaleTableOutputIncludesLocaleColumn(t *testing.T) {
	installScreenshotsListTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			writeScreenshotsListJSON(t, w, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}],"links":{}}`)
		case "/v1/appStoreVersionLocalizations/loc-en/appScreenshotSets":
			writeScreenshotsListJSON(t, w, `{"data":[{"type":"appScreenshotSets","id":"set-en","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case "/v1/appScreenshotSets/set-en/appScreenshots":
			writeScreenshotsListJSON(t, w, `{"data":[{"type":"appScreenshots","id":"shot-en","attributes":{"fileName":"home.png","fileSize":42}}],"links":{}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})

	stdout, stderr, runErr := runRootCommand(t, []string{
		"screenshots", "list",
		"--version-id", "version-1",
		"--output", "table",
	})
	if runErr != nil {
		t.Fatalf("screenshots list error: %v (stderr %q)", runErr, stderr)
	}
	if !strings.Contains(stdout, "Locale") || !strings.Contains(stdout, "en-US") {
		t.Fatalf("stdout = %q, want locale column", stdout)
	}
}

func TestScreenshotsListWithoutLocaleReportsVersionWithoutLocalizations(t *testing.T) {
	installScreenshotsListTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			writeScreenshotsListJSON(t, w, `{"data":[],"links":{}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})

	stdout, _, runErr := runRootCommand(t, []string{
		"screenshots", "list",
		"--version-id", "version-1",
		"--output", "json",
	})
	if runErr == nil {
		t.Fatal("screenshots list error = nil, want no-localization error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(runErr.Error(), "no App Store version localizations found for version") {
		t.Fatalf("error = %v, want no-localization diagnostic", runErr)
	}
	if !strings.Contains(runErr.Error(), "asc localizations create") {
		t.Fatalf("error = %v, want localization creation guidance", runErr)
	}
}

func writeScreenshotsListJSON(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := io.WriteString(w, body); err != nil {
		t.Errorf("write response: %v", err)
	}
}
