package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
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
