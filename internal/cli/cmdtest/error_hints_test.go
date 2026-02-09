package cmdtest

import (
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestRunPrintsHintForMissingAuth(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_CONFIG_PATH", t.TempDir()+"/config.json")

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"testflight", "apps", "list"}, "1.2.3")
		if code != cmd.ExitAuth {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitAuth, code)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "missing authentication") {
		t.Fatalf("expected missing auth error, got %q", stderr)
	}
	if !strings.Contains(stderr, "Hint:") || !strings.Contains(stderr, "asc auth login") {
		t.Fatalf("expected auth hint, got %q", stderr)
	}
}
