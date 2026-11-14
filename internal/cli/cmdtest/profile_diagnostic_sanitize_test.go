package cmdtest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func TestMissingProfileDiagnosticSanitizesConfiguredValues(t *testing.T) {
	for _, mode := range []string{"profile-name", "config-path"} {
		t.Run(mode, func(t *testing.T) {
			setupAuth(t)
			path := filepath.Join(t.TempDir(), "config.json")
			if mode == "config-path" {
				path = filepath.Join(t.TempDir(), "config\x1b[2J\r\nforged.json")
			}
			t.Setenv("ASC_CONFIG_PATH", path)
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			if mode == "profile-name" {
				name := "known\x1b[2J\r\nforged"
				if err := config.SaveAt(path, &config.Config{DefaultKeyName: name, Keys: []config.Credential{{Name: name, KeyID: "KEY", IssuerID: "ISSUER", PrivateKeyPath: "unused.p8"}}}); err != nil {
					t.Fatal(err)
				}
			}
			reg := asctest.NewRegistry(t)
			reg.Install()
			code := -1
			stdout, stderr := captureOutput(t, func() { code = cmd.Run([]string{"apps", "list", "--profile", "missing"}, "test") })
			if calls := len(reg.Requests()); code != cmd.ExitAuth || stdout != "" || calls != 0 {
				t.Fatalf("code=%d stdout=%q calls=%d stderr=%q", code, stdout, calls, stderr)
			}
			if !strings.Contains(stderr, "credentials not found for profile") {
				t.Fatalf("missing profile error: %q", stderr)
			}
			if strings.ContainsAny(stderr, "\x1b\r") || strings.Contains(stderr, "\nforged") {
				t.Fatalf("configured value injected terminal controls: %q", stderr)
			}
			if mode == "profile-name" && !strings.Contains(stderr, "available profiles:") {
				t.Fatalf("missing available names: %q", stderr)
			}
			if mode == "config-path" && !strings.Contains(stderr, "config file:") {
				t.Fatalf("missing config hint: %q", stderr)
			}
		})
	}
}
