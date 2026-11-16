//go:build !darwin

package cmdtest

import (
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestSigningRunUnsupportedPlatformRendersStderrAndErrorExit(t *testing.T) {
	args := []string{
		"signing", "run",
		"--identity", "identity.p12",
		"--profile", "profile.mobileprovision",
		"--", "child-tool", "--child-flag",
	}

	stdout, stderr, exitCode := runWithDeps(t, nil, args...)
	if exitCode != rootcmd.ExitError {
		t.Fatalf("exit code = %d, want %d", exitCode, rootcmd.ExitError)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "Error: signing run is supported only on macOS") {
		t.Fatalf("stderr = %q, want platform diagnostic", stderr)
	}
}
