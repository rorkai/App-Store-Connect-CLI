package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestAppsSearchKeywordsSetUpdatesVersionLocalization(t *testing.T) {
	setupAppSearchKeywordsSetAuth(t)

	requestCount := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/appStoreVersions" {
				t.Fatalf("unexpected version request: %s %s", req.Method, req.URL.String())
			}
			if got := req.URL.Query().Get("filter[versionString]"); got != "1.2.3" {
				t.Fatalf("filter[versionString] = %q, want 1.2.3", got)
			}
			if got := req.URL.Query().Get("filter[platform]"); got != "IOS" {
				t.Fatalf("filter[platform] = %q, want IOS", got)
			}
			if got := req.URL.Query().Get("limit"); got != "200" {
				t.Fatalf("version limit = %q, want 200", got)
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"1.2.3"}}]}`)
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-1/appStoreVersionLocalizations" {
				t.Fatalf("unexpected localization request: %s %s", req.Method, req.URL.String())
			}
			if got := req.URL.Query().Get("filter[locale]"); got != "en-US" {
				t.Fatalf("filter[locale] = %q, want en-US", got)
			}
			if got := req.URL.Query().Get("limit"); got != "200" {
				t.Fatalf("localization limit = %q, want 200", got)
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","keywords":"old"}}]}`)
		case 3:
			if req.Method != http.MethodPatch || req.URL.Path != "/v1/appStoreVersionLocalizations/loc-1" {
				t.Fatalf("unexpected update request: %s %s", req.Method, req.URL.String())
			}
			var payload struct {
				Data struct {
					Type       string            `json:"type"`
					ID         string            `json:"id"`
					Attributes map[string]string `json:"attributes"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode PATCH body: %v", err)
			}
			if payload.Data.Type != "appStoreVersionLocalizations" || payload.Data.ID != "loc-1" {
				t.Fatalf("unexpected PATCH identity: %+v", payload.Data)
			}
			if len(payload.Data.Attributes) != 1 || payload.Data.Attributes["keywords"] != "alpha,beta" {
				t.Fatalf("unexpected PATCH attributes: %#v", payload.Data.Attributes)
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","keywords":"alpha,beta"}}}`)
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	args := []string{
		"apps", "search-keywords", "set",
		"--app", "app-1",
		"--version", "1.2.3",
		"--locale", "en-US",
		"--platform", "ios",
		"--keywords", "alpha,beta",
		"--confirm",
		"--output", "json",
	}
	stdout, stderr := captureOutput(t, func() {
		if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitSuccess {
			t.Fatalf("expected exit code %d, got %d", rootcmd.ExitSuccess, code)
		}
	})

	if requestCount != 3 {
		t.Fatalf("request count = %d, want 3", requestCount)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var response asc.AppStoreVersionLocalizationResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("parse stdout: %v; stdout=%q", err, stdout)
	}
	if response.Data.ID != "loc-1" || response.Data.Attributes.Keywords != "alpha,beta" {
		t.Fatalf("unexpected output: %+v", response.Data)
	}
}

func TestAppsSearchKeywordsSetFollowsResolutionPagination(t *testing.T) {
	setupAppSearchKeywordsSetAuth(t)

	requestCount := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/appStoreVersions" || req.URL.Query().Get("filter[platform]") != "" {
				t.Fatalf("unexpected first version page: %s", req.URL.String())
			}
			if req.URL.Query().Get("filter[versionString]") != "2.0" || req.URL.Query().Get("limit") != "200" {
				t.Fatalf("unexpected first version filters: %s", req.URL.RawQuery)
			}
			return jsonResponse(http.StatusOK, `{"data":[],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions?cursor=version-next"}}`)
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != "https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions?cursor=version-next" {
				t.Fatalf("unexpected next version URL: %s", req.URL.String())
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"VISION_OS","versionString":"2.0"}}]}`)
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-1/appStoreVersionLocalizations" {
				t.Fatalf("unexpected first localization page: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("filter[locale]") != "ja" || req.URL.Query().Get("limit") != "200" {
				t.Fatalf("unexpected first localization filters: %s", req.URL.RawQuery)
			}
			return jsonResponse(http.StatusOK, `{"data":[],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations?cursor=locale-next"}}`)
		case 4:
			if req.Method != http.MethodGet || req.URL.String() != "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations?cursor=locale-next" {
				t.Fatalf("unexpected next localization URL: %s", req.URL.String())
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja"}}]}`)
		case 5:
			if req.Method != http.MethodPatch || req.URL.Path != "/v1/appStoreVersionLocalizations/loc-ja" {
				t.Fatalf("unexpected PATCH: %s %s", req.Method, req.URL.String())
			}
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","keywords":"one,two"}}}`)
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	stdout, stderr := captureOutput(t, func() {
		if code := rootcmd.Run([]string{
			"apps", "search-keywords", "set",
			"--app", "app-1", "--version", "2.0", "--locale", "ja",
			"--keywords", "one,two", "--confirm",
		}, "1.2.3"); code != rootcmd.ExitSuccess {
			t.Fatalf("expected success exit, got %d", code)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"loc-ja"`) {
		t.Fatalf("expected updated localization output, got %q", stdout)
	}
	if requestCount != 5 {
		t.Fatalf("request count = %d, want 5", requestCount)
	}
}

func TestAppsSearchKeywordsSetResolutionFailures(t *testing.T) {
	tests := []struct {
		name      string
		responses []string
		wantErr   string
		wantCalls int
	}{
		{
			name:      "version not found",
			responses: []string{`{"data":[]}`},
			wantErr:   `app store version not found for version "1.2.3"`,
			wantCalls: 1,
		},
		{
			name:      "platform is ambiguous",
			responses: []string{`{"data":[{"type":"appStoreVersions","id":"ios-version","attributes":{"platform":"IOS","versionString":"1.2.3"}},{"type":"appStoreVersions","id":"mac-version","attributes":{"platform":"MAC_OS","versionString":"1.2.3"}}]}`},
			wantErr:   `multiple app store versions found for version "1.2.3" on platforms IOS, MAC_OS; pass --platform`,
			wantCalls: 1,
		},
		{
			name: "locale not found",
			responses: []string{
				`{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"1.2.3"}}]}`,
				`{"data":[]}`,
			},
			wantErr:   `no existing version localization found for locale "en-US"`,
			wantCalls: 2,
		},
		{
			name: "locale is ambiguous",
			responses: []string{
				`{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"1.2.3"}}]}`,
				`{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}},{"type":"appStoreVersionLocalizations","id":"loc-2","attributes":{"locale":"en-US"}}]}`,
			},
			wantErr:   `multiple version localizations found for locale "en-US"`,
			wantCalls: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAppSearchKeywordsSetAuth(t)
			requestCount := 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requestCount++
				if requestCount > len(test.responses) {
					t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
				}
				return jsonResponse(http.StatusOK, test.responses[requestCount-1])
			}))

			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run([]string{
					"apps", "search-keywords", "set",
					"--app", "app-1", "--version", "1.2.3", "--locale", "en-US",
					"--keywords", "one,two", "--confirm",
				}, "1.2.3"); code != rootcmd.ExitError {
					t.Fatalf("expected failure exit, got %d", code)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
			if requestCount != test.wantCalls {
				t.Fatalf("request count = %d, want %d", requestCount, test.wantCalls)
			}
		})
	}
}

func TestAppsSearchKeywordsSetAPIFailure(t *testing.T) {
	setupAppSearchKeywordsSetAuth(t)

	requestCount := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"1.2.3"}}]}`)
		case 2:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`)
		case 3:
			return jsonResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"Invalid keywords"}]}`)
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	stdout, stderr := captureOutput(t, func() {
		if code := rootcmd.Run([]string{
			"apps", "search-keywords", "set",
			"--app", "app-1", "--version", "1.2.3", "--locale", "en-US",
			"--keywords", "one,two", "--confirm",
		}, "1.2.3"); code != rootcmd.HTTPStatusToExitCode(http.StatusUnprocessableEntity) {
			t.Fatalf("expected HTTP 422 exit, got %d", code)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "failed to update") || !strings.Contains(stderr, "Invalid keywords") {
		t.Fatalf("expected API update error, got %q", stderr)
	}
}

func TestAppsSearchKeywordsSetTableOutput(t *testing.T) {
	setupAppSearchKeywordsSetAuth(t)

	requestCount := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"1.2.3"}}]}`)
		case 2:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`)
		case 3:
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","keywords":"set,list"}}}`)
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	stdout, stderr := captureOutput(t, func() {
		if code := rootcmd.Run([]string{
			"apps", "search-keywords", "set",
			"--confirm", "--keywords", "set,list", "--locale", "en-US",
			"--version", "1.2.3", "--app", "app-1", "--output", "table",
		}, "1.2.3"); code != rootcmd.ExitSuccess {
			t.Fatalf("expected success exit, got %d", code)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, value := range []string{"Locale", "Keywords", "en-US", "set,list"} {
		if !strings.Contains(stdout, value) {
			t.Fatalf("expected table output to contain %q, got %q", value, stdout)
		}
	}
}

func TestAppsSearchKeywordsSetUsageErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "app is required",
			args:    []string{"apps", "search-keywords", "set", "--version", "1.2.3", "--locale", "en-US", "--keywords", "one,two", "--confirm"},
			wantErr: "--app is required",
		},
		{
			name:    "legacy invocation needs version",
			args:    []string{"apps", "search-keywords", "set", "--app", "app-1", "--keywords", "one,two", "--confirm"},
			wantErr: "--version is required to select an App Store version",
		},
		{
			name:    "locale is required",
			args:    []string{"apps", "search-keywords", "set", "--app", "app-1", "--version", "1.2.3", "--keywords", "one,two", "--confirm"},
			wantErr: "--locale is required to select a version localization",
		},
		{
			name:    "confirm is required",
			args:    []string{"apps", "search-keywords", "set", "--app", "app-1", "--version", "1.2.3", "--locale", "en-US", "--keywords", "one,two"},
			wantErr: "--confirm is required",
		},
		{
			name:    "keywords are required",
			args:    []string{"apps", "search-keywords", "set", "--app", "app-1", "--version", "1.2.3", "--locale", "en-US", "--confirm"},
			wantErr: "--keywords is required",
		},
		{
			name:    "invalid platform",
			args:    []string{"apps", "search-keywords", "set", "--app", "app-1", "--version", "1.2.3", "--locale", "en-US", "--platform", "watchos", "--keywords", "one,two", "--confirm"},
			wantErr: "--platform must be one of",
		},
		{
			name:    "ambiguous locale root",
			args:    []string{"apps", "search-keywords", "set", "--app", "app-1", "--version", "1.2.3", "--locale", "zh", "--keywords", "one,two", "--confirm"},
			wantErr: `unsupported locale "zh"; use one of`,
		},
		{
			name:    "positional argument",
			args:    []string{"apps", "search-keywords", "set", "unexpected", "--app", "app-1", "--version", "1.2.3", "--locale", "en-US", "--keywords", "one,two", "--confirm"},
			wantErr: `unexpected argument "unexpected"`,
		},
		{
			name:    "invalid output before mutation",
			args:    []string{"apps", "search-keywords", "set", "--app", "app-1", "--version", "1.2.3", "--locale", "en-US", "--keywords", "one,two", "--confirm", "--output", "yaml"},
			wantErr: `(got "yaml")`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}

func TestAppsSearchKeywordsSetRejectsKeywordsOverCharacterLimit(t *testing.T) {
	assertUsageExit(t, []string{
		"apps", "search-keywords", "set",
		"--app", "app-1", "--version", "1.2.3", "--locale", "en-US",
		"--keywords", strings.Repeat("語", 101), "--confirm",
	}, "keywords exceed 100 characters")
}

func setupAppSearchKeywordsSetAuth(t *testing.T) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
}
