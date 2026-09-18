package assets

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestExecuteScreenshotListCommandResolvesVersionLocalizationByVersionIDAndLocale(t *testing.T) {
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			if req.URL.Query().Get("cursor") == "page-2" {
				writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}],"links":{}}`)
				return
			}
			if got := req.URL.Query().Get("limit"); got != "200" {
				t.Errorf("localizations limit = %q, want 200", got)
			}
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-de","attributes":{"locale":"de-DE"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations?cursor=page-2"}}`)
		case "/v1/appStoreVersionLocalizations/loc-en/appScreenshotSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case "/v1/appScreenshotSets/set-1/appScreenshots":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshots","id":"shot-1","attributes":{"fileName":"home.png","fileSize":42}}],"links":{}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))

	result, err := executeScreenshotListCommand(context.Background(), screenshotListCommandOptions{
		VersionID: "version-1",
		Locale:    "en-us",
	}, screenshotListDependencies{
		GetClient:      func() (*asc.Client, error) { return client, nil },
		RequestContext: shared.ContextWithTimeout,
	})
	if err != nil {
		t.Fatalf("executeScreenshotListCommand() error: %v", err)
	}
	if result.VersionLocalizationID != "loc-en" {
		t.Fatalf("version localization ID = %q, want loc-en", result.VersionLocalizationID)
	}
	if len(result.Sets) != 1 || len(result.Sets[0].Screenshots) != 1 || result.Sets[0].Screenshots[0].ID != "shot-1" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestExecuteScreenshotListCommandIncludesPaginatedScreenshotSetsAndScreenshots(t *testing.T) {
	const setsNext = "https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets?cursor=sets-2"
	const screenshotsNext = "https://api.appstoreconnect.apple.com/v1/appScreenshotSets/set-1/appScreenshots?cursor=screenshots-2"

	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets":
			if req.URL.Query().Get("cursor") == "sets-2" {
				writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-2","attributes":{"screenshotDisplayType":"APP_IPAD_PRO_129"}}],"links":{}}`)
				return
			}
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{"next":"`+setsNext+`"}}`)
		case "/v1/appScreenshotSets/set-1/appScreenshots":
			if req.URL.Query().Get("cursor") == "screenshots-2" {
				writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshots","id":"shot-2","attributes":{"fileName":"02-settings.png"}}],"links":{}}`)
				return
			}
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshots","id":"shot-1","attributes":{"fileName":"01-home.png"}}],"links":{"next":"`+screenshotsNext+`"}}`)
		case "/v1/appScreenshotSets/set-2/appScreenshots":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshots","id":"shot-3","attributes":{"fileName":"03-ipad.png"}}],"links":{}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))

	result, err := executeScreenshotListCommand(context.Background(), screenshotListCommandOptions{
		VersionLocalizationID: "loc-1",
	}, screenshotListDependencies{
		GetClient:      func() (*asc.Client, error) { return client, nil },
		RequestContext: shared.ContextWithTimeout,
	})
	if err != nil {
		t.Fatalf("executeScreenshotListCommand() error: %v", err)
	}
	if len(result.Sets) != 2 {
		t.Fatalf("screenshot sets = %d, want 2", len(result.Sets))
	}
	if got := result.Sets[0].Screenshots; len(got) != 2 || got[0].ID != "shot-1" || got[1].ID != "shot-2" {
		t.Fatalf("set-1 screenshots = %#v, want shot-1 and shot-2", got)
	}
	if got := result.Sets[1].Screenshots; len(got) != 1 || got[0].ID != "shot-3" {
		t.Fatalf("set-2 screenshots = %#v, want shot-3", got)
	}
}

func TestExecuteScreenshotListCommandUsesASCAppIDForVersionIDPlatformVerification(t *testing.T) {
	t.Setenv("ASC_APP_ID", "123456789")
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v1/appStoreVersions/version-1":
			if got := req.URL.Query().Get("include"); got != "app" {
				t.Errorf("version include = %q, want app", got)
			}
			writeAssetsTestJSON(w, http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`)
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}],"links":{}}`)
		case "/v1/appStoreVersionLocalizations/loc-en/appScreenshotSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[],"links":{}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))

	result, err := executeScreenshotListCommand(context.Background(), screenshotListCommandOptions{
		VersionID: "version-1",
		Platform:  "IOS",
		Locale:    "en-US",
	}, screenshotListDependencies{
		GetClient:      func() (*asc.Client, error) { return client, nil },
		RequestContext: shared.ContextWithTimeout,
	})
	if err != nil {
		t.Fatalf("executeScreenshotListCommand() error: %v", err)
	}
	if result.VersionLocalizationID != "loc-en" {
		t.Fatalf("version localization ID = %q, want loc-en", result.VersionLocalizationID)
	}
}

func TestExecuteScreenshotListCommandResolvesAppVersionAndLocale(t *testing.T) {
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v1/apps/123456789/appStoreVersions":
			query := req.URL.Query()
			if got := query.Get("filter[versionString]"); got != "1.2.3" {
				t.Errorf("version filter = %q, want 1.2.3", got)
			}
			if got := query.Get("filter[platform]"); got != "MAC_OS" {
				t.Errorf("platform filter = %q, want MAC_OS", got)
			}
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"MAC_OS"}}],"links":{}}`)
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}],"links":{}}`)
		case "/v1/appStoreVersionLocalizations/loc-en/appScreenshotSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[],"links":{}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))

	result, err := executeScreenshotListCommand(context.Background(), screenshotListCommandOptions{
		AppID:    "123456789",
		Version:  "1.2.3",
		Platform: "mac_os",
		Locale:   "en-US",
	}, screenshotListDependencies{
		GetClient:      func() (*asc.Client, error) { return client, nil },
		RequestContext: shared.ContextWithTimeout,
	})
	if err != nil {
		t.Fatalf("executeScreenshotListCommand() error: %v", err)
	}
	if result.VersionLocalizationID != "loc-en" || len(result.Sets) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestExecuteScreenshotListCommandValidatesSelectorModesBeforeAuth(t *testing.T) {
	tests := []struct {
		name    string
		opts    screenshotListCommandOptions
		wantErr string
	}{
		{name: "missing selector", wantErr: "choose a localization selector"},
		{
			name:    "direct and version selector",
			opts:    screenshotListCommandOptions{VersionLocalizationID: "loc-1", VersionID: "version-1", Locale: "en-US"},
			wantErr: "--version-localization cannot be combined",
		},
		{
			name:    "version selectors conflict",
			opts:    screenshotListCommandOptions{AppID: "123456789", Version: "1.0", VersionID: "version-1", Locale: "en-US"},
			wantErr: "--version and --version-id are mutually exclusive",
		},
		{
			name:    "version string requires app",
			opts:    screenshotListCommandOptions{Version: "1.0", Locale: "en-US"},
			wantErr: "--app is required with --version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			var err error
			stdout, stderr := captureOutput(t, func() {
				_, err = executeScreenshotListCommand(context.Background(), tt.opts, screenshotListDependencies{
					GetClient: func() (*asc.Client, error) {
						t.Fatal("authentication attempted before selector validation")
						return nil, errors.New("unexpected authentication")
					},
				})
			})
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, tt.wantErr) {
				t.Fatalf("stderr = %q, want substring %q", stderr, tt.wantErr)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want flag.ErrHelp", err)
			}
		})
	}
}

func TestExecuteScreenshotListCommandValidatesPlatformBeforeAuth(t *testing.T) {
	clientRequested := false
	_, err := executeScreenshotListCommand(context.Background(), screenshotListCommandOptions{
		AppID:    "123456789",
		Version:  "1.2.3",
		Platform: "WATCH_OS",
		Locale:   "en-US",
	}, screenshotListDependencies{
		GetClient: func() (*asc.Client, error) {
			clientRequested = true
			return nil, errors.New("unexpected authentication")
		},
	})
	if clientRequested {
		t.Fatal("authentication attempted before platform validation")
	}
	if err == nil || !strings.Contains(err.Error(), "--platform must be one of") {
		t.Fatalf("error = %v, want platform enumeration", err)
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want flag.ErrHelp", err)
	}
}

func TestAssetsScreenshotsListCommandDoesNotDefineLocalizationIDAlias(t *testing.T) {
	cmd := AssetsScreenshotsListCommand()
	if cmd.FlagSet.Lookup("localization-id") != nil {
		t.Fatal("--localization-id alias should not be defined; only --version-localization is supported")
	}
	if cmd.FlagSet.Lookup("version-localization") == nil {
		t.Fatal("--version-localization flag not found")
	}
}

func TestAssetsScreenshotsListCommandRejectsUnexpectedArguments(t *testing.T) {
	cmd := AssetsScreenshotsListCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-localization", "loc-1", "typo"}); err != nil {
		t.Fatalf("parse arguments: %v", err)
	}

	stdout, stderr := captureOutput(t, func() {
		err := cmd.Exec(context.Background(), cmd.FlagSet.Args())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("error = %v, want usage error", err)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "unexpected argument(s): typo") {
		t.Fatalf("stderr = %q, want unexpected argument diagnostic", stderr)
	}
}

func TestAssetsScreenshotsListPlatformHelpExplainsSelectorSpecificAppRequirement(t *testing.T) {
	platformFlag := AssetsScreenshotsListCommand().FlagSet.Lookup("platform")
	if platformFlag == nil {
		t.Fatal("--platform flag not found")
	}
	for _, want := range []string{"defaults to IOS with --version", "--version-id requires --app or ASC_APP_ID"} {
		if !strings.Contains(platformFlag.Usage, want) {
			t.Fatalf("--platform help = %q, want substring %q", platformFlag.Usage, want)
		}
	}
}

func TestExecuteScreenshotListCommandWithoutLocaleListsEveryLocalization(t *testing.T) {
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}},{"type":"appStoreVersionLocalizations","id":"loc-de","attributes":{"locale":"de-DE"}}],"links":{}}`)
		case "/v1/appStoreVersionLocalizations/loc-en/appScreenshotSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-en","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case "/v1/appScreenshotSets/set-en/appScreenshots":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshots","id":"shot-en","attributes":{"fileName":"home.png","fileSize":42}}],"links":{}}`)
		case "/v1/appStoreVersionLocalizations/loc-de/appScreenshotSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[],"links":{}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))

	var (
		result *asc.AppScreenshotListResult
		err    error
	)
	stdout, stderr := captureOutput(t, func() {
		result, err = executeScreenshotListCommand(context.Background(), screenshotListCommandOptions{
			VersionID: "version-1",
		}, screenshotListDependencies{
			GetClient:      func() (*asc.Client, error) { return client, nil },
			RequestContext: shared.ContextWithTimeout,
		})
	})
	if err != nil {
		t.Fatalf("executeScreenshotListCommand() error: %v", err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "--locale not set") || !strings.Contains(stderr, "2 localizations") {
		t.Fatalf("stderr = %q, want all-localization note", stderr)
	}
	if result.VersionLocalizationID != "" {
		t.Fatalf("version localization ID = %q, want empty in all-localization mode", result.VersionLocalizationID)
	}
	if len(result.Sets) != 0 {
		t.Fatalf("sets = %#v, want empty in all-localization mode", result.Sets)
	}
	if len(result.Localizations) != 2 {
		t.Fatalf("localizations = %#v, want 2 entries", result.Localizations)
	}
	first := result.Localizations[0]
	if first.Locale != "de-DE" || first.VersionLocalizationID != "loc-de" || len(first.Sets) != 0 {
		t.Fatalf("first localization = %#v, want de-DE/loc-de without sets", first)
	}
	second := result.Localizations[1]
	if second.Locale != "en-US" || second.VersionLocalizationID != "loc-en" {
		t.Fatalf("second localization = %#v, want en-US/loc-en", second)
	}
	if len(second.Sets) != 1 || len(second.Sets[0].Screenshots) != 1 || second.Sets[0].Screenshots[0].ID != "shot-en" {
		t.Fatalf("en-US sets = %#v, want set-en with shot-en", second.Sets)
	}
}

func TestExecuteScreenshotListCommandWithoutLocaleReportsMissingLocalizations(t *testing.T) {
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[],"links":{}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))

	_, err := executeScreenshotListCommand(context.Background(), screenshotListCommandOptions{
		VersionID: "version-1",
	}, screenshotListDependencies{
		GetClient:      func() (*asc.Client, error) { return client, nil },
		RequestContext: shared.ContextWithTimeout,
	})
	if err == nil {
		t.Fatal("executeScreenshotListCommand() error = nil, want no-localization error")
	}
	if !strings.Contains(err.Error(), "no App Store version localizations found for version") {
		t.Fatalf("error = %v, want no-localization diagnostic", err)
	}
	if !strings.Contains(err.Error(), "asc localizations create") {
		t.Fatalf("error = %v, want localization creation guidance", err)
	}
}

func TestExecuteScreenshotListCommandEnumeratesAvailableLocalesWhenLocaleIsMissing(t *testing.T) {
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}},{"type":"appStoreVersionLocalizations","id":"loc-de","attributes":{"locale":"de-DE"}}],"links":{}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))

	_, err := executeScreenshotListCommand(context.Background(), screenshotListCommandOptions{
		VersionID: "version-1",
		Locale:    "ja",
	}, screenshotListDependencies{
		GetClient:      func() (*asc.Client, error) { return client, nil },
		RequestContext: shared.ContextWithTimeout,
	})
	if err == nil {
		t.Fatal("executeScreenshotListCommand() error = nil, want locale enumeration error")
	}
	if !strings.Contains(err.Error(), `no App Store version localization found for locale "ja"`) {
		t.Fatalf("error = %v, want missing locale diagnostic", err)
	}
	if !strings.Contains(err.Error(), "available locales: de-DE, en-US") {
		t.Fatalf("error = %v, want sorted available locales", err)
	}
}

func TestAssetsScreenshotsListLocaleHelpDocumentsAllLocalizationDefault(t *testing.T) {
	localeFlag := AssetsScreenshotsListCommand().FlagSet.Lookup("locale")
	if localeFlag == nil {
		t.Fatal("--locale flag not found")
	}
	if !strings.Contains(localeFlag.Usage, "omit to list every localization") {
		t.Fatalf("--locale help = %q, want all-localization default documented", localeFlag.Usage)
	}
}
