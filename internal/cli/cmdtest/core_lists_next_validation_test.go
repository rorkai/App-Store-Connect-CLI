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

type invalidNextURLCase struct {
	args        []string // command args without --next
	wantErr     string   // diagnostic prefix, e.g. "builds icons list: --next"
	wantWarning string   // stderr line printed before the diagnostic
	deps        bool     // the command is built on shared.Deps: stub its client and read its deps streams
}

// runInvalidNextURLCases runs each command with an http:// and a malformed
// --next URL and requires the command's own diagnostic.
func runInvalidNextURLCases(t *testing.T, cases []invalidNextURLCase) {
	t.Helper()

	nextCases := []struct {
		name    string
		next    string
		wantErr string
	}{
		{
			name:    "invalid scheme",
			next:    "http://api.appstoreconnect.apple.com/v1/apps?cursor=AQ",
			wantErr: " must be an App Store Connect URL",
		},
		{
			name:    "malformed URL",
			next:    "https://api.appstoreconnect.apple.com/%zz",
			wantErr: " must be a valid URL:",
		},
	}

	for _, tc := range cases {
		for _, nc := range nextCases {
			t.Run(strings.Join(tc.args, " ")+"/"+nc.name, func(t *testing.T) {
				args := append(append([]string{}, tc.args...), "--next", nc.next)
				assertInvalidNextURLRun(t, args, tc.wantErr+nc.wantErr, tc.wantWarning, tc.deps)
			})
		}
	}
}

func assertInvalidNextURLRun(t *testing.T, args []string, wantErr, wantWarning string, deps bool) {
	t.Helper()

	var stdout, stderr string
	var runErr error
	if deps {
		stdout, stderr, runErr = runRootWithDeps(t, stubClient(t, asctest.NewRegistry(t)), args...)
	} else {
		asctest.NewRegistry(t).Install()
		stdout, stderr, runErr = runCommand(t, args)
	}

	if runErr == nil || !strings.Contains(runErr.Error(), wantErr) {
		t.Fatalf("expected error %q, got %v", wantErr, runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", got, rootcmd.ExitUsage)
	}
	if wantWarning != "" {
		warning, rest, _ := strings.Cut(stderr, "\n")
		if warning != wantWarning {
			t.Fatalf("stderr first line = %q, want %q", warning, wantWarning)
		}
		stderr = rest
	}
	assertUsageDiagnosticFirstLine(t, stderr, wantErr)
}

type paginateFromNextCase struct {
	args       []string // command args without --paginate/--next
	firstURL   string   // must contain cursor=AQ; page two is served at cursor=BQ
	dataType   string
	wantStderr string
	env        map[string]string
	singlePage bool // run without --paginate and expect only the first page
	deps       bool // the command is built on shared.Deps: stub its client and read its deps streams
}

// runPaginateFromNextCases runs each command with --paginate --next firstURL
// against a two-page stub and requires both pages in stdout, so a command
// that still demands its parent selector alongside --next fails.
func runPaginateFromNextCases(t *testing.T, cases []paginateFromNextCase) {
	t.Helper()

	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			for key, value := range tc.env {
				t.Setenv(key, value)
			}

			secondURL := strings.Replace(tc.firstURL, "cursor=AQ", "cursor=BQ", 1)
			if secondURL == tc.firstURL {
				t.Fatalf("firstURL %q has no cursor=AQ", tc.firstURL)
			}
			pages := []struct{ url, body string }{
				{tc.firstURL, `{"data":[{"type":"` + tc.dataType + `","id":"page-1"}],"links":{"next":"` + secondURL + `"}}`},
				{secondURL, `{"data":[{"type":"` + tc.dataType + `","id":"page-2"}],"links":{"next":""}}`},
			}
			args := append(append([]string{}, tc.args...), "--paginate", "--next", tc.firstURL)
			if tc.singlePage {
				pages = pages[:1]
				pages[0].body = `{"data":[{"type":"` + tc.dataType + `","id":"page-1"}],"links":{"next":""}}`
				args = append(append([]string{}, tc.args...), "--next", tc.firstURL)
			}

			reg := asctest.NewRegistry(t)
			for _, page := range pages {
				pageURL, err := url.Parse(page.url)
				if err != nil {
					t.Fatalf("parse page URL %q: %v", page.url, err)
				}
				reg.Register(asctest.GET(pageURL.EscapedPath(), pageURL.RawQuery), asctest.JSON(http.StatusOK, page.body))
			}

			var stdout, stderr string
			var runErr error
			if tc.deps {
				stdout, stderr, runErr = runRootWithDeps(t, stubClient(t, reg), args...)
			} else {
				reg.Install()
				stdout, stderr, runErr = runCommand(t, args)
			}
			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}

			if stderr != tc.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, tc.wantStderr)
			}
			for _, id := range []string{"page-1", "page-2"}[:len(pages)] {
				if needle := `"id":"` + id + `"`; !strings.Contains(stdout, needle) {
					t.Fatalf("expected output to contain %q, got %q", needle, stdout)
				}
			}
		})
	}
}

func TestCoreListsRejectsInvalidNextURL(t *testing.T) {
	runInvalidNextURLCases(t, []invalidNextURLCase{
		{args: []string{"certificates", "list"}, wantErr: "certificates list: --next"},
		{args: []string{"nominations", "list", "--status", "DRAFT"}, wantErr: "nominations list: --next", deps: true},
		{args: []string{"accessibility", "list"}, wantErr: "accessibility list: --next", deps: true},
		{args: []string{"actors", "list"}, wantErr: "actors list: --next", deps: true},
		{args: []string{"android-ios-mapping", "list", "--app", "app-1"}, wantErr: "android-ios-mapping list: --next"},
		{args: []string{"testflight", "feedback", "list"}, wantErr: "testflight feedback list: --next"},
		{args: []string{"testflight", "crashes", "list"}, wantErr: "testflight crashes list: --next"},
		{args: []string{"agreements", "territories", "list"}, wantErr: "agreements territories list: --next", deps: true},
	})
}

func TestCoreListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"certificates", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/certificates?cursor=AQ&limit=200",
			dataType: "certificates",
		},
		{
			args:     []string{"agreements", "territories", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/endUserLicenseAgreements/eula-1/territories?cursor=AQ&limit=200",
			dataType: "territories",
			deps:     true,
		},
		{
			args:     []string{"nominations", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/nominations?cursor=AQ&limit=200",
			dataType: "nominations",
			deps:     true,
		},
		{
			args:     []string{"accessibility", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/accessibilityDeclarations?cursor=AQ&limit=200",
			dataType: "accessibilityDeclarations",
			deps:     true,
		},
		{
			args:     []string{"actors", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/actors?cursor=AQ&limit=200",
			dataType: "actors",
			deps:     true,
		},
		{
			args:     []string{"android-ios-mapping", "list", "--app", "app-1"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/androidToIosAppMappingDetails?cursor=AQ&limit=200",
			dataType: "androidToIosAppMappingDetails",
		},
		{
			args:     []string{"testflight", "feedback", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/betaFeedbackScreenshotSubmissions?cursor=AQ&limit=200",
			dataType: "betaFeedbackScreenshotSubmissions",
		},
		{
			args:     []string{"testflight", "crashes", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/betaFeedbackCrashSubmissions?cursor=AQ&limit=200",
			dataType: "betaFeedbackCrashSubmissions",
		},
	})
}

func TestListsRejectNextURLWithEmptyParentID(t *testing.T) {
	tests := []struct {
		args    []string
		next    string
		wantErr string
	}{
		{
			args:    []string{"agreements", "territories", "list"},
			next:    "https://api.appstoreconnect.apple.com/v1/endUserLicenseAgreements//territories?cursor=AQ",
			wantErr: "agreements territories list: invalid --next URL",
		},
		{
			args:    []string{"bundle-ids", "profiles", "list"},
			next:    "https://api.appstoreconnect.apple.com/v1/bundleIds//profiles?cursor=AQ",
			wantErr: "bundle-ids profiles list: invalid --next URL",
		},
		{
			args:    []string{"users", "visible-apps", "list"},
			next:    "https://api.appstoreconnect.apple.com/v1/users//visibleApps?cursor=AQ",
			wantErr: "users visible-apps list: invalid --next URL",
		},
		{
			args:    []string{"webhooks", "deliveries", "links"},
			next:    "https://api.appstoreconnect.apple.com/v1/webhooks//relationships/deliveries?cursor=AQ",
			wantErr: "webhooks deliveries links: invalid --next URL",
		},
	}

	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			args := append(append([]string{}, test.args...), "--next", test.next)
			assertInvalidNextURLRun(t, args, test.wantErr, "", false)
		})
	}
}
