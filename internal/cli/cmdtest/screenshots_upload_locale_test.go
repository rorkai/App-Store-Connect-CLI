package cmdtest

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestScreenshotsUploadLocaleUsageErrorsExitBeforeRequests(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name: "locale with version localization",
			args: []string{
				"screenshots", "upload",
				"--version-localization", "LOC_ID",
				"--locale", "en-US",
				"--path", "./screenshots",
				"--device-type", "IPHONE_65",
			},
			wantStderr: "Error: --locale cannot be combined with --version-localization",
		},
		{
			name: "locale with resume",
			args: []string{
				"screenshots", "upload",
				"--resume", "./failures.json",
				"--locale", "en-US",
			},
			wantStderr: "Error: --resume cannot be combined with --locale",
		},
		{
			name: "locale without app-scoped mode",
			args: []string{
				"screenshots", "upload",
				"--locale", "en-US",
				"--path", "./screenshots",
				"--device-type", "IPHONE_65",
			},
			wantStderr: "Error: --locale requires --app with --version or --version-id",
		},
		{
			name: "several locales",
			args: []string{
				"screenshots", "upload",
				"--app", "123456789",
				"--version", "1.2.3",
				"--locale", "en-US,fr-FR",
				"--path", "./screenshots",
				"--device-type", "IPHONE_65",
			},
			wantStderr: "Error: --locale accepts exactly one locale",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("usage error must not send requests, got %s %s", req.Method, req.URL.String())
				return nil, nil
			})
			stdout, stderr, runErr := runRootCommand(t, test.args)
			if runErr == nil {
				t.Fatal("expected usage error")
			}
			if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, runErr)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("expected %q in stderr %q", test.wantStderr, stderr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}

func TestScreenshotsUploadMissingRemoteLocalizationIsResourceNotFound(t *testing.T) {
	tests := []struct {
		name          string
		localizations string
		locale        string
		localeDir     string
		wantErr       string
		wantParameter string
	}{
		{
			name:          "locale not on version",
			localizations: `[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}},{"type":"appStoreVersionLocalizations","id":"loc-fr","attributes":{"locale":"fr-FR"}}]`,
			locale:        "de-DE",
			wantErr:       `screenshots upload: no App Store version localization found for locale "de-DE"; available locales: en-US, fr-FR`,
			wantParameter: "--locale",
		},
		{
			name:          "version without localizations",
			localizations: `[]`,
			locale:        "de-DE",
			wantErr:       "screenshots upload: no App Store version localizations found for version version-1; create one with: asc localizations create --version version-1 --locale en-US",
			wantParameter: "--locale",
		},
		{
			name:          "locale directory not on version",
			localizations: `[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US"}}]`,
			localeDir:     "ja",
			wantErr:       "screenshots upload: no matching App Store version localizations found for locales: ja",
			wantParameter: "--path",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			pathDir := t.TempDir()
			imageDir := pathDir
			if test.localeDir != "" {
				imageDir = filepath.Join(pathDir, test.localeDir)
				if err := os.MkdirAll(imageDir, 0o755); err != nil {
					t.Fatalf("MkdirAll() error: %v", err)
				}
			}
			writePNG(t, filepath.Join(imageDir, "01-home.png"), 1242, 2688)

			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/appStoreVersions":
					return statusJSONResponse(`{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"1.2.3"}}],"links":{}}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
					return statusJSONResponse(`{"data":` + test.localizations + `,"links":{}}`), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}))

			args := []string{
				"screenshots", "upload",
				"--app", "123456789",
				"--version", "1.2.3",
				"--path", pathDir,
				"--device-type", "IPHONE_65",
				"--dry-run",
				"--output", "json",
			}
			if test.locale != "" {
				args = append(args, "--locale", test.locale)
			}
			stdout, stderr, runErr := runRootCommand(t, args)
			if runErr == nil {
				t.Fatal("expected missing localization error")
			}
			if runErr.Error() != test.wantErr {
				t.Fatalf("error = %q, want %q", runErr.Error(), test.wantErr)
			}
			if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitError {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitError, runErr)
			}
			if !shared.IsValidationError(runErr) {
				t.Fatalf("expected validation-classified error, got %v", runErr)
			}
			diagnostic, ok := shared.DiagnosticFromError(runErr)
			if !ok || diagnostic.Code != shared.DiagnosticResourceNotFound || diagnostic.Parameter != test.wantParameter {
				t.Fatalf("diagnostic = %+v (ok=%v), want %q for %s", diagnostic, ok, shared.DiagnosticResourceNotFound, test.wantParameter)
			}
			if stdout != "" || stderr != "" {
				t.Fatalf("expected empty stdout and stderr, got stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
}
