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

func screenshotsListTestClient(t *testing.T, handler http.HandlerFunc) func() (*asc.Client, error) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return stubServerClient(t, server)
}

func TestScreenshotsListWithoutLocaleListsEveryLocalization(t *testing.T) {
	client := screenshotsListTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v1/apps/123456789/appStoreVersions":
			writeScreenshotsListJSON(t, w, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{}}`)
		case "/v1/appStoreVersions/version-1":
			writeScreenshotsListJSON(t, w, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`)
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			writeScreenshotsListJSON(t, w, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}},{"type":"appStoreVersionLocalizations","id":"loc-de","attributes":{"locale":"de-DE"}}],"links":{}}`)
		case "/v1/appStoreVersionLocalizations/loc-en/appScreenshotSets":
			writeScreenshotsListJSON(t, w, `{"data":[{"type":"appScreenshotSets","id":"set-en","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case "/v1/appScreenshotSets/set-en/appScreenshots":
			writeScreenshotsListJSON(t, w, `{"data":[{"type":"appScreenshots","id":"shot-en","attributes":{"fileName":"home.png","fileSize":42}}],"links":{}}`)
		case "/v1/appScreenshotSets/set-en/relationships/appScreenshots":
			writeScreenshotsListJSON(t, w, `{"data":[{"type":"appScreenshots","id":"shot-en"}],"links":{}}`)
		case "/v1/appStoreVersionLocalizations/loc-de/appScreenshotSets":
			writeScreenshotsListJSON(t, w, `{"data":[],"links":{}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})

	stdout, stderr, runErr := runRootWithDeps(t, client, []string{
		"screenshots", "list",
		"--app", "123456789",
		"--version", "1.2.3",
		"--output", "json",
	}...)
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
	for _, test := range []struct{ flag, value, id string }{
		{"--version-id", "version-1", "version-1"},
		{"--version", "version-1", "version-1"},
		{"--version", "https://api.appstoreconnect.apple.com/v1/appStoreVersions/123", "123"},
		{"--version-id", "123", "123"},
	} {
		t.Run(test.flag+"/"+test.value, func(t *testing.T) {
			testScreenshotListVersionSelector(t, test.flag, test.value, test.id)
		})
	}
}

func testScreenshotListVersionSelector(t *testing.T, selector, value, id string) {
	t.Helper()
	client := screenshotsListTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v1/appStoreVersions/" + id + "/appStoreVersionLocalizations":
			writeScreenshotsListJSON(t, w, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}],"links":{}}`)
		case "/v1/appStoreVersionLocalizations/loc-en/appScreenshotSets":
			writeScreenshotsListJSON(t, w, `{"data":[{"type":"appScreenshotSets","id":"set-en","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case "/v1/appScreenshotSets/set-en/appScreenshots":
			writeScreenshotsListJSON(t, w, `{"data":[{"type":"appScreenshots","id":"shot-en","attributes":{"fileName":"home.png","fileSize":42}}],"links":{}}`)
		case "/v1/appScreenshotSets/set-en/relationships/appScreenshots":
			writeScreenshotsListJSON(t, w, `{"data":[{"type":"appScreenshots","id":"shot-en"}],"links":{}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})

	t.Setenv("ASC_APP_ID", "ambient-app-must-not-be-used")
	stdout, stderr, runErr := runRootWithDeps(t, client,
		"screenshots", "list",
		selector, value,
		"--output", "table",
	)
	if runErr != nil {
		t.Fatalf("screenshots list error: %v (stderr %q)", runErr, stderr)
	}
	if !strings.Contains(stdout, "Locale") || !strings.Contains(stdout, "en-US") {
		t.Fatalf("stdout = %q, want locale column", stdout)
	}
}

func TestScreenshotsListWithoutLocaleReportsVersionWithoutLocalizations(t *testing.T) {
	client := screenshotsListTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			writeScreenshotsListJSON(t, w, `{"data":[],"links":{}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})

	stdout, _, runErr := runRootWithDeps(t, client, []string{
		"screenshots", "list",
		"--version-id", "version-1",
		"--output", "json",
	}...)
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

func TestVideoPreviewsListReadsEveryPreviewSetPage(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/loc-1/appPreviewSets?cursor=page-2&include=appPreviews"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/v1/appStoreVersionLocalizations/loc-1/appPreviewSets" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		if req.URL.Query().Get("cursor") == "page-2" {
			writeScreenshotsListJSON(t, w, `{"data":[{"type":"appPreviewSets","id":"set-2","attributes":{"previewType":"IPAD_PRO_3GEN_129"},"relationships":{"appPreviews":{"data":[{"type":"appPreviews","id":"preview-2"}]}}}],"included":[{"type":"appPreviews","id":"preview-2","attributes":{"fileName":"ipad.mov"}}],"links":{}}`)
			return
		}
		writeScreenshotsListJSON(t, w, `{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"},"relationships":{"appPreviews":{"data":[{"type":"appPreviews","id":"preview-1"}]}}}],"included":[{"type":"appPreviews","id":"preview-1","attributes":{"fileName":"iphone.mov"}}],"links":{"next":"`+nextURL+`"}}`)
	}))
	t.Cleanup(server.Close)

	stdout, stderr, runErr := runRootWithDeps(t, stubServerClient(t, server), "video-previews", "list", "--version-localization", "loc-1", "--output", "json")
	if runErr != nil {
		t.Fatalf("video-previews list error: %v (stderr %q)", runErr, stderr)
	}

	var payload struct {
		Sets []struct {
			Set struct {
				ID string `json:"id"`
			} `json:"set"`
			Previews []struct {
				ID string `json:"id"`
			} `json:"previews"`
		} `json:"sets"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal stdout %q: %v", stdout, err)
	}
	if len(payload.Sets) != 2 || payload.Sets[0].Set.ID != "set-1" || payload.Sets[1].Set.ID != "set-2" {
		t.Fatalf("sets = %#v, want set-1 and set-2", payload.Sets)
	}
	if len(payload.Sets[1].Previews) != 1 || payload.Sets[1].Previews[0].ID != "preview-2" {
		t.Fatalf("set-2 previews = %#v, want preview-2", payload.Sets[1].Previews)
	}
}
