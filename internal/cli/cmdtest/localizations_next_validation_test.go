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
		{args: []string{"localizations", "preview-sets", "list"}, wantErr: "localizations preview-sets list: --next"},
		{args: []string{"localizations", "preview-sets", "links"}, wantErr: "localizations preview-sets links: --next"},
		{args: []string{"localizations", "screenshot-sets", "list"}, wantErr: "localizations screenshot-sets list: --next"},
		{args: []string{"localizations", "screenshot-sets", "links"}, wantErr: "localizations screenshot-sets links: --next"},
		{args: []string{"testflight", "app-localizations", "list"}, wantErr: "testflight app-localizations list: --next"},
	})
}

func TestLocalizationsListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"localizations", "list", "--version", "version-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations?cursor=AQ&limit=200",
			dataType: "appStoreVersionLocalizations",
		},
		{
			args:     []string{"build-localizations", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations?cursor=AQ&limit=200",
			dataType: "appStoreVersionLocalizations",
		},
		{
			args:     []string{"localizations", "preview-sets", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/localization-1/appPreviewSets?cursor=AQ&limit=200",
			dataType: "appPreviewSets",
		},
		{
			args:     []string{"localizations", "preview-sets", "links"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/localization-1/relationships/appPreviewSets?cursor=AQ&limit=200",
			dataType: "appPreviewSets",
		},
		{
			args:     []string{"localizations", "screenshot-sets", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/localization-1/appScreenshotSets?cursor=AQ&limit=200",
			dataType: "appScreenshotSets",
		},
		{
			args:     []string{"localizations", "screenshot-sets", "links"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/localization-1/relationships/appScreenshotSets?cursor=AQ&limit=200",
			dataType: "appScreenshotSets",
		},
		{
			args:     []string{"testflight", "app-localizations", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/betaAppLocalizations?cursor=AQ&limit=200",
			dataType: "betaAppLocalizations",
		},
	})
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
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			wantRequests := append([]string{"https://api.appstoreconnect.apple.com/v1/builds/build-1/appStoreVersion"}, tc.wantPages...)
			reg := asctest.NewRegistry(t)
			reg.Install()
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

			stdout, stderr, runErr := runCommand(t, []string{"build-localizations", "list", "--build-id", "build-1", "--paginate", "--next", tc.next})

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
