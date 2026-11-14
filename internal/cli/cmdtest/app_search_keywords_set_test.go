package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestAppsSearchKeywordsSetUpdatesVersionLocalization(t *testing.T) {
	setupAppSearchKeywordsSetAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"1.2.3"}}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "filter[locale]=en-US", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","keywords":"old"}}]}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-1").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"keywords":"alpha,beta"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","keywords":"alpha,beta"}}}`))

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

	if requestCount := len(reg.Requests()); requestCount != 3 {
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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=2.0", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions?cursor=version-next"}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "cursor=version-next"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"VISION_OS","versionString":"2.0"}}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "filter[locale]=ja", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations?cursor=locale-next"}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "cursor=locale-next"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja"}}]}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-ja").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"keywords":"one,two"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","keywords":"one,two"}}}`))

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
	if requestCount := len(reg.Requests()); requestCount != 5 {
		t.Fatalf("request count = %d, want 5", requestCount)
	}
}

func TestAppsSearchKeywordsSetResolutionFailures(t *testing.T) {
	tests := []struct {
		name      string
		requests  []asctest.Matcher
		responses []string
		wantErr   string
		wantCalls int
	}{
		{
			name:     "version not found",
			requests: []asctest.Matcher{asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200"), asctest.GET("/v1/apps/app-1/appStoreVersions", "limit=200")},
			responses: []string{
				`{"data":[]}`,
				// The not-found diagnostic lists the app's existing versions.
				`{"data":[{"type":"appStoreVersions","id":"version-2","attributes":{"platform":"IOS","versionString":"2.0","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`,
			},
			wantErr:   "app store version not found for version \"1.2.3\"\nExisting App Store versions for app \"app-1\" (newest first):\n  2.0  IOS  PREPARE_FOR_SUBMISSION  version-2\n",
			wantCalls: 2,
		},
		{
			name:      "platform is ambiguous",
			requests:  []asctest.Matcher{asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200")},
			responses: []string{`{"data":[{"type":"appStoreVersions","id":"ios-version","attributes":{"platform":"IOS","versionString":"1.2.3"}},{"type":"appStoreVersions","id":"mac-version","attributes":{"platform":"MAC_OS","versionString":"1.2.3"}}]}`},
			wantErr:   `2 app store versions match version "1.2.3"; pass --platform with one of:`,
			wantCalls: 1,
		},
		{
			name:     "locale not found",
			requests: []asctest.Matcher{asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200"), asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "filter[locale]=en-US", "limit=200")},
			responses: []string{
				`{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"1.2.3"}}]}`,
				`{"data":[]}`,
			},
			wantErr:   `no existing version localization found for locale "en-US"`,
			wantCalls: 2,
		},
		{
			name:     "locale is ambiguous",
			requests: []asctest.Matcher{asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200"), asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "filter[locale]=en-US", "limit=200")},
			responses: []string{
				`{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"1.2.3"}}]}`,
				`{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}},{"type":"appStoreVersionLocalizations","id":"loc-2","attributes":{"locale":"en-US"}}]}`,
			},
			wantErr:   `2 version localizations match locale "en-US":`,
			wantCalls: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAppSearchKeywordsSetAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			for i, request := range test.requests {
				reg.Register(request, asctest.JSON(http.StatusOK, test.responses[i]))
			}

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
			if requestCount := len(reg.Requests()); requestCount != test.wantCalls {
				t.Fatalf("request count = %d, want %d", requestCount, test.wantCalls)
			}
		})
	}
}

func TestAppsSearchKeywordsSetAPIFailure(t *testing.T) {
	setupAppSearchKeywordsSetAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"1.2.3"}}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "filter[locale]=en-US", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-1").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"keywords":"one,two"}}}`),
		asctest.JSON(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"Invalid keywords"}]}`))

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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[versionString]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"1.2.3"}}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "filter[locale]=en-US", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersionLocalizations/loc-1").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"keywords":"set,list"}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","keywords":"set,list"}}}`))

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
