package cmdtest

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestSigningKeychainManageUsageErrors(t *testing.T) {
	keychain := filepath.Join(t.TempDir(), "release.keychain-db")
	for _, test := range []struct {
		name       string
		args       []string
		want       string
		darwinOnly bool
	}{
		{name: "delete requires confirm", args: []string{"signing", "keychain", "delete", "--keychain", keychain}, want: "--confirm is required"},
		{name: "delete refuses login keychain", args: []string{"signing", "keychain", "delete", "--keychain", "~/Library/Keychains/login.keychain-db", "--confirm"}, want: "refusing to operate on the login or System keychain", darwinOnly: true},
		{name: "unlock requires password file", args: []string{"signing", "keychain", "unlock", "--keychain", keychain}, want: "--keychain-password-file is required"},
		{name: "set-timeout requires a timeout", args: []string{"signing", "keychain", "set-timeout", "--keychain", keychain, "--keychain-password-file", keychain}, want: "--timeout or --no-timeout is required"},
		{name: "timeout flags are exclusive", args: []string{"signing", "keychain", "set-timeout", "--keychain", keychain, "--keychain-password-file", keychain, "--timeout", "60", "--no-timeout"}, want: "mutually exclusive"},
		{name: "identity digest is validated", args: []string{"signing", "keychain", "set-partition-list", "--keychain", keychain, "--keychain-password-file", keychain, "--identity-sha256", strings.Repeat("z", 64)}, want: "64 hexadecimal characters"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.darwinOnly && runtime.GOOS != "darwin" {
				t.Skip("keychain paths are checked only on macOS")
			}
			var exitCode int
			stdout, stderr := captureOutput(t, func() {
				exitCode = rootcmd.Run(test.args, "test")
			})
			if exitCode != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr = %q", exitCode, rootcmd.ExitUsage, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.want) {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
		})
	}
}
