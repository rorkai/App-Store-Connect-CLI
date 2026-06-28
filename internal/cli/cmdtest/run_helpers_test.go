package cmdtest

import (
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func assertUsageExit(t *testing.T, args []string, wantErr string) {
	t.Helper()

	stdout, stderr := captureOutput(t, func() {
		if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitUsage {
			t.Fatalf("expected exit code %d, got %d", rootcmd.ExitUsage, code)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, wantErr) {
		t.Fatalf("expected stderr to contain %q, got %q", wantErr, stderr)
	}
}
