package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

type invalidNextURLCase struct {
	args        []string // command args without --next
	wantErr     string   // diagnostic prefix, e.g. "builds icons list: --next"
	wantWarning string   // stderr line printed before the diagnostic
	runtimeErr  bool     // reported as a runtime error with empty stderr
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
				assertInvalidNextURLRun(t, args, tc.wantErr+nc.wantErr, tc.wantWarning, tc.runtimeErr)
			})
		}
	}
}

func assertInvalidNextURLRun(t *testing.T, args []string, wantErr, wantWarning string, runtimeErr bool) {
	t.Helper()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil || !strings.Contains(runErr.Error(), wantErr) {
		t.Fatalf("expected error %q, got %v", wantErr, runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if runtimeErr {
		if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitError {
			t.Fatalf("exit code = %d, want %d", got, rootcmd.ExitError)
		}
		assertEmptyStderr(t, stderr)
		return
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

			originalTransport := http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
			})
			requestCount := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if requestCount >= len(pages) {
					t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
				}
				page := pages[requestCount]
				requestCount++
				if req.Method != http.MethodGet || req.URL.String() != page.url {
					t.Fatalf("request %d = %s %s, want GET %s", requestCount, req.Method, req.URL.String(), page.url)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(page.body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

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
		{args: []string{"nominations", "list", "--status", "DRAFT"}, wantErr: "nominations list: --next"},
		{args: []string{"accessibility", "list"}, wantErr: "accessibility list: --next"},
		{args: []string{"actors", "list"}, wantErr: "actors list: --next"},
		{args: []string{"android-ios-mapping", "list", "--app", "app-1"}, wantErr: "android-ios-mapping list: --next"},
		{args: []string{"testflight", "feedback", "list"}, wantErr: "testflight feedback list: --next"},
		{args: []string{"testflight", "crashes", "list"}, wantErr: "testflight crashes list: --next"},
		{args: []string{"agreements", "territories", "list"}, wantErr: "agreements territories list: --next"},
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
		},
		{
			args:     []string{"nominations", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/nominations?cursor=AQ&limit=200",
			dataType: "nominations",
		},
		{
			args:     []string{"accessibility", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/apps/app-1/accessibilityDeclarations?cursor=AQ&limit=200",
			dataType: "accessibilityDeclarations",
		},
		{
			args:     []string{"actors", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/actors?cursor=AQ&limit=200",
			dataType: "actors",
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
		args       []string
		next       string
		wantErr    string
		runtimeErr bool
	}{
		{
			args:    []string{"agreements", "territories", "list"},
			next:    "https://api.appstoreconnect.apple.com/v1/endUserLicenseAgreements//territories?cursor=AQ",
			wantErr: "agreements territories list: invalid --next URL",
		},
		{
			args:    []string{"users", "visible-apps", "list"},
			next:    "https://api.appstoreconnect.apple.com/v1/users//visibleApps?cursor=AQ",
			wantErr: "users visible-apps list: invalid --next URL",
		},
		{
			args:       []string{"webhooks", "deliveries", "links"},
			next:       "https://api.appstoreconnect.apple.com/v1/webhooks//relationships/deliveries?cursor=AQ",
			wantErr:    "webhooks deliveries links: invalid --next URL",
			runtimeErr: true,
		},
	}

	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			args := append(append([]string{}, test.args...), "--next", test.next)
			assertInvalidNextURLRun(t, args, test.wantErr, "", test.runtimeErr)
		})
	}
}
