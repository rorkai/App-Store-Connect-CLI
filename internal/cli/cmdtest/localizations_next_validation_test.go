package cmdtest

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestLocalizationsListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"localizations", "preview-sets", "list"}, wantErr: "localizations preview-sets list: --next", deps: true},
		{args: []string{"localizations", "preview-sets", "links"}, wantErr: "localizations preview-sets links: --next", deps: true},
		{args: []string{"localizations", "screenshot-sets", "list"}, wantErr: "localizations screenshot-sets list: --next", deps: true},
		{args: []string{"localizations", "screenshot-sets", "links"}, wantErr: "localizations screenshot-sets links: --next", deps: true},
		{args: []string{"testflight", "app-localizations", "list"}, wantErr: "testflight app-localizations list: --next", deps: true},
	})
}

func TestLocalizationsListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"localizations", "list", "--version", "version-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations?cursor=AQ&limit=200",
			dataType: "appStoreVersionLocalizations",
			deps:     true,
		},
		{
			args:     []string{"build-localizations", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations?cursor=AQ&limit=200",
			dataType: "appStoreVersionLocalizations",
			deps:     true,
		},
		{
			args:     []string{"localizations", "preview-sets", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/localization-1/appPreviewSets?cursor=AQ&limit=200",
			dataType: "appPreviewSets",
			deps:     true,
		},
		{
			args:     []string{"localizations", "preview-sets", "links"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/localization-1/relationships/appPreviewSets?cursor=AQ&limit=200",
			dataType: "appPreviewSets",
			deps:     true,
		},
		{
			args:     []string{"localizations", "screenshot-sets", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/localization-1/appScreenshotSets?cursor=AQ&limit=200",
			dataType: "appScreenshotSets",
			deps:     true,
		},
		{
			args:     []string{"localizations", "screenshot-sets", "links"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/localization-1/relationships/appScreenshotSets?cursor=AQ&limit=200",
			dataType: "appScreenshotSets",
			deps:     true,
		},
		{
			args:     []string{"testflight", "app-localizations", "list"},
			deps:     true,
			firstURL: "https://api.appstoreconnect.apple.com/v1/betaAppLocalizations?cursor=AQ&limit=200",
			dataType: "betaAppLocalizations",
		},
	})
}

func TestAppInfoLocalizationsNextBindsToSelectedAppInfo(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appInfos/info-foreign", "include=app"), asctest.JSON(http.StatusOK, `{"data":{"type":"appInfos","id":"info-foreign","relationships":{"app":{"data":{"type":"apps","id":"100000002"}}}}}`))

	nextURL := "https://api.appstoreconnect.apple.com/v1/appInfos/info-foreign/appInfoLocalizations?cursor=AQ"
	stdout, _, runErr := runRootWithDeps(t, stubClient(t, reg), "localizations", "list", "--app", "100000001", "--type", "app-info", "--next", nextURL)
	if runErr == nil || stdout != "" || !strings.Contains(runErr.Error(), `belongs to app "100000002", not "100000001"`) {
		t.Fatalf("run error=%v stdout=%q, want app-info ownership refusal", runErr, stdout)
	}
	if requests := reg.Requests(); len(requests) != 1 || requests[0].URL.Path != "/v1/appInfos/info-foreign" {
		t.Fatalf("requests=%v, want only app-info ownership read", requests)
	}
}

func TestAppInfoLocalizationsNextAcceptsSelectedAppInfo(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appInfos/info-1", "include=app"), asctest.JSON(http.StatusOK, `{"data":{"type":"appInfos","id":"info-1","relationships":{"app":{"data":{"type":"apps","id":"100000001"}}}}}`))
	reg.Register(asctest.GET("/v1/appInfos/info-1/appInfoLocalizations", "cursor=AQ"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}],"links":{}}`))

	nextURL := "https://api.appstoreconnect.apple.com/v1/appInfos/info-1/appInfoLocalizations?cursor=AQ"
	stdout, stderr, runErr := runRootWithDeps(t, stubClient(t, reg), "localizations", "list", "--app", "100000001", "--type", "app-info", "--next", nextURL, "--output", "json")
	if runErr != nil || stderr != "" || !strings.Contains(stdout, `"id":"loc-1"`) {
		t.Fatalf("run error=%v stdout=%q stderr=%q, want selected app-info page", runErr, stdout, stderr)
	}
	requests := reg.Requests()
	if len(requests) != 2 || requests[0].URL.Path != "/v1/appInfos/info-1" || requests[1].URL.String() != nextURL {
		t.Fatalf("requests=%v, want ownership then selected continuation", requests)
	}
}

func TestAppInfoLocalizationsDownloadNextBindsToSelectedAppInfo(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appInfos/info-foreign", "include=app"), asctest.JSON(http.StatusOK, `{"data":{"type":"appInfos","id":"info-foreign","relationships":{"app":{"data":{"type":"apps","id":"100000002"}}}}}`))

	nextURL := "https://api.appstoreconnect.apple.com/v1/appInfos/info-foreign/appInfoLocalizations?cursor=AQ"
	stdout, _, runErr := runRootWithDeps(t, stubClient(t, reg), "localizations", "download", "--app", "100000001", "--type", "app-info", "--next", nextURL, "--path", filepath.Join(t.TempDir(), "localizations"))
	if runErr == nil || stdout != "" || !strings.Contains(runErr.Error(), `belongs to app "100000002", not "100000001"`) {
		t.Fatalf("run error=%v stdout=%q, want app-info ownership refusal", runErr, stdout)
	}
	if requests := reg.Requests(); len(requests) != 1 || requests[0].URL.Path != "/v1/appInfos/info-foreign" {
		t.Fatalf("requests=%v, want only app-info ownership read", requests)
	}
}

func TestAppInfoLocalizationsNextRequiresSelectedApp(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	reg := asctest.NewRegistry(t)
	reg.Install()
	nextURL := "https://api.appstoreconnect.apple.com/v1/appInfos/info-1/appInfoLocalizations?cursor=AQ"

	stdout, stderr, runErr := runRootWithDeps(t, stubClient(t, reg), "localizations", "list", "--type", "app-info", "--next", nextURL, "--output", "json")
	if rootcmd.ExitCodeFromError(runErr) != rootcmd.ExitUsage || stdout != "" || !strings.Contains(stderr, "--app is required") {
		t.Fatalf("run error=%v stdout=%q stderr=%q, want usage refusal before requests", runErr, stdout, stderr)
	}
	if requests := reg.Requests(); len(requests) != 0 {
		t.Fatalf("requests=%v, want zero requests", requests)
	}
}

func TestBuildLocalizationsListNextWithBuildID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations?cursor=BQ&limit=200"

	tests := []struct {
		name      string
		next      string
		wantPages []string
		wantErr   string
	}{
		{name: "matching version", next: firstURL, wantPages: []string{firstURL, secondURL}},
		{
			name:    "other version",
			next:    "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-2/appStoreVersionLocalizations?cursor=AQ",
			wantErr: `build-localizations list: --next is not a page of the App Store version attached to --build-id "build-1"`,
		},
		{
			name:    "other collection",
			next:    "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionPhasedRelease?cursor=AQ",
			wantErr: `build-localizations list: --next is not a page of the App Store version attached to --build-id "build-1"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wantRequests := append([]string{"https://api.appstoreconnect.apple.com/v1/builds/build-1/appStoreVersion"}, tc.wantPages...)
			reg := asctest.NewRegistry(t)
			for _, want := range wantRequests {
				body := `{"data":{"type":"appStoreVersions","id":"version-1"}}`
				switch want {
				case firstURL:
					body = `{"data":[{"type":"appStoreVersionLocalizations","id":"page-1"}],"links":{"next":"` + secondURL + `"}}`
				case secondURL:
					body = `{"data":[{"type":"appStoreVersionLocalizations","id":"page-2"}],"links":{"next":""}}`
				}
				wantURL, err := url.Parse(want)
				if err != nil {
					t.Fatalf("parse %s: %v", want, err)
				}
				reg.Register(asctest.GET(wantURL.EscapedPath(), strings.Split(wantURL.RawQuery, "&")...), asctest.JSON(http.StatusOK, body))
			}

			stdout, stderr, runErr := runRootWithDeps(t, stubClient(t, reg), "build-localizations", "list", "--build-id", "build-1", "--paginate", "--next", tc.next)

			if tc.wantErr != "" {
				if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d (err %v)", got, rootcmd.ExitUsage, runErr)
				}
				if stdout != "" {
					t.Fatalf("expected empty stdout, got %q", stdout)
				}
				assertUsageDiagnosticFirstLine(t, stderr, tc.wantErr)
				return
			}
			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			requests := reg.Requests()
			if requestCount := len(requests); requestCount != len(wantRequests) {
				t.Fatalf("requests = %d, want %d", requestCount, len(wantRequests))
			}
			for index, req := range requests {
				if req.Method != http.MethodGet || req.URL.String() != wantRequests[index] {
					t.Fatalf("request %d = %s %s, want GET %s", index+1, req.Method, req.URL.String(), wantRequests[index])
				}
			}
			for _, needle := range []string{`"id":"page-1"`, `"id":"page-2"`} {
				if !strings.Contains(stdout, needle) {
					t.Fatalf("expected output to contain %q, got %q", needle, stdout)
				}
			}
		})
	}
}
