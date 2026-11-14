package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

var deleteOutputCommands = []struct {
	name       string
	args       []string
	path       string
	wantOutput string
	errPrefix  string
}{
	{
		name:       "localizations screenshot-sets delete",
		args:       []string{"localizations", "screenshot-sets", "delete", "--id", "set-1", "--confirm"},
		path:       "/v1/appScreenshotSets/set-1",
		wantOutput: `"id":"set-1"`,
		errPrefix:  "localizations screenshot-sets delete:",
	},
	{
		name:       "versions delete",
		args:       []string{"versions", "delete", "--version-id", "ver-1", "--confirm"},
		path:       "/v1/appStoreVersions/ver-1",
		wantOutput: `"versionId":"ver-1"`,
		errPrefix:  "versions delete:",
	},
}

func TestDeleteCommandsOutput(t *testing.T) {
	for _, test := range deleteOutputCommands {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.DELETE(test.path), asctest.JSON(http.StatusNoContent, ""))

			stdout, stderr, err := runCommand(t, test.args)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			for _, want := range []string{test.wantOutput, `"deleted":true`} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("expected %s in output, got %q", want, stdout)
				}
			}
		})
	}
}

func TestDeleteCommandsAPIError(t *testing.T) {
	for _, test := range deleteOutputCommands {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.DELETE(test.path),
				asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found","detail":"Resource does not exist"}]}`))

			stdout, stderr, runErr := runCommand(t, test.args)

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(runErr.Error(), test.errPrefix) {
				t.Fatalf("expected error prefixed %q, got %v", test.errPrefix, runErr)
			}
			if !strings.Contains(strings.ToLower(runErr.Error()), "not found") {
				t.Fatalf("expected not-found details, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}
