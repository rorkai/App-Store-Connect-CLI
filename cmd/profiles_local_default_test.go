package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

func TestProfilesLocalDefaultFallsBackWhenXcodeDiscoveryFails(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("active Xcode profile directory is macOS-specific")
	}

	developerDir := filepath.Join(t.TempDir(), "Xcode.app", "Contents", "Developer")
	binDir := filepath.Join(developerDir, "usr", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create fake developer directory: %v", err)
	}
	xcodebuildPath := filepath.Join(binDir, "xcodebuild")
	script := "#!/bin/sh\nprintf 'xcode-select: active developer directory is a command line tools instance\\n' >&2\nexit 1\n"
	if err := os.WriteFile(xcodebuildPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake xcodebuild: %v", err)
	}
	fakeXcrun := filepath.Join(t.TempDir(), "xcrun")
	xcrunScript := "#!/bin/sh\nif [ \"$1\" = \"--find\" ] && [ \"$2\" = \"xcodebuild\" ]; then\n  printf '%s\\n' \"$DEVELOPER_DIR/usr/bin/xcodebuild\"\n  exit 0\nfi\nexit 2\n"
	if err := os.WriteFile(fakeXcrun, []byte(xcrunScript), 0o700); err != nil {
		t.Fatalf("write fake trusted xcrun: %v", err)
	}
	t.Cleanup(localxcode.OverrideTrustedXcrunPathForTesting(fakeXcrun))
	t.Setenv("DEVELOPER_DIR", developerDir)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	legacyDir := filepath.Join(homeDir, "Library", "MobileDevice", "Provisioning Profiles")

	tests := []struct {
		name string
		args []string
	}{
		{name: "list", args: []string{"profiles", "local", "list", "--output", "json"}},
		{name: "clean", args: []string{"profiles", "local", "clean", "--expired", "--dry-run", "--output", "json"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetReportFlags(t)
			exitCode := ExitSuccess
			stdout, stderr := captureCommandOutput(t, func() {
				exitCode = Run(test.args, "test")
			})
			if exitCode != ExitSuccess {
				t.Fatalf("Run() exit code = %d, want %d (stderr=%q)", exitCode, ExitSuccess, stderr)
			}

			if !strings.Contains(stdout, legacyDir) {
				t.Fatalf("stdout %q should report the legacy install directory %q", stdout, legacyDir)
			}
			for _, want := range []string{
				"Note: could not determine the active Xcode version",
				"active developer directory is a command line tools instance",
				legacyDir,
				"--install-dir",
			} {
				if !strings.Contains(stderr, want) {
					t.Fatalf("stderr missing %q: %q", want, stderr)
				}
			}
			if strings.Contains(stderr, "Error:") {
				t.Fatalf("fallback must not report an error: %q", stderr)
			}
		})
	}
}
