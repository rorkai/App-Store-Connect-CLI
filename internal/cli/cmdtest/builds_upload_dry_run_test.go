package cmdtest

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// TestBuildsUploadDryRunSendsNothing pins the 7.4.0 contract: --dry-run checks
// the artifact and credentials locally, prints what it would upload, and sends
// no request to App Store Connect, not even the app read.
func TestBuildsUploadDryRunSendsNothing(t *testing.T) {
	dir := t.TempDir()
	pkgPath := filepath.Join(dir, "app.pkg")
	if err := os.WriteFile(pkgPath, []byte("pkg-bytes"), 0o600); err != nil {
		t.Fatalf("write pkg: %v", err)
	}

	tests := []struct {
		name       string
		args       func(ipaPath string) []string
		noAuth     bool
		wantExit   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "ipa",
			args:       func(ipaPath string) []string { return []string{"--app", "123456789", "--ipa", ipaPath} },
			wantStdout: `{"dryRun":true,"app":"123456789","fileName":"app.ipa","fileSize":FILESIZE,"platform":"IOS","version":"1.0.0","buildNumber":"42","bundleId":"com.example.demo"}` + "\n",
			wantStderr: "Dry run: checked app.ipa locally; nothing was sent to App Store Connect. The app's bundle ID was not checked; use --reserve-only to check it and reserve the upload.\n",
		},
		{
			name:       "ipa with app given as a bundle ID is not looked up",
			args:       func(ipaPath string) []string { return []string{"--app", "com.example.demo", "--ipa", ipaPath} },
			wantStdout: `{"dryRun":true,"app":"com.example.demo","fileName":"app.ipa","fileSize":FILESIZE,"platform":"IOS","version":"1.0.0","buildNumber":"42","bundleId":"com.example.demo"}` + "\n",
			wantStderr: "Dry run: checked app.ipa locally; nothing was sent to App Store Connect. The app's bundle ID was not checked; use --reserve-only to check it and reserve the upload.\n",
		},
		{
			name: "pkg",
			args: func(string) []string {
				return []string{"--app", "123456789", "--pkg", pkgPath, "--version", "2.0.0", "--build-number", "7"}
			},
			wantStdout: `{"dryRun":true,"app":"123456789","fileName":"app.pkg","fileSize":9,"platform":"MAC_OS","version":"2.0.0","buildNumber":"7"}` + "\n",
			wantStderr: "Dry run: checked app.pkg locally; nothing was sent to App Store Connect. Use --reserve-only to reserve the upload.\n",
		},
		{
			name:     "missing credentials fail like an upload",
			args:     func(ipaPath string) []string { return []string{"--app", "123456789", "--ipa", ipaPath} },
			noAuth:   true,
			wantExit: rootcmd.ExitAuth,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.noAuth {
				isolateBearerTokenAuth(t, "")
			} else {
				setupAuth(t)
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			}
			ipaPath := writeBuildUploadIPA(t, "com.example.demo")
			info, err := os.Stat(ipaPath)
			if err != nil {
				t.Fatalf("stat ipa: %v", err)
			}
			reg := asctest.NewRegistry(t)
			reg.Install()

			args := append([]string{"builds", "upload"}, test.args(ipaPath)...)
			args = append(args, "--dry-run", "--output", "json")
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(args, "1.2.3")
			})

			if code != test.wantExit {
				t.Fatalf("exit code = %d, want %d; stderr %q", code, test.wantExit, stderr)
			}
			if got := len(reg.Requests()); got != 0 {
				t.Fatalf("--dry-run sent %d request(s) to App Store Connect: %+v", got, reg.Requests())
			}
			if test.wantExit != 0 {
				if stdout != "" {
					t.Fatalf("expected empty stdout, got %q", stdout)
				}
				return
			}
			wantStdout := strings.ReplaceAll(test.wantStdout, "FILESIZE", strconv.FormatInt(info.Size(), 10))
			if stdout != wantStdout {
				t.Fatalf("stdout = %q, want %q", stdout, wantStdout)
			}
			if stderr != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
		})
	}
}
