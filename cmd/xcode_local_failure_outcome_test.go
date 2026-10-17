package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

// installFailingXcodebuild routes trusted xcrun lookups to an xcodebuild that
// reports its version but fails every action like a broken project would.
func installFailingXcodebuild(t *testing.T) {
	t.Helper()
	developerDir := t.TempDir()
	binDir := filepath.Join(developerDir, "usr", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	xcodebuild := `#!/bin/sh
for arg in "$@"; do
  if [ "$arg" = "-version" ]; then
    printf 'Xcode 26.0\nBuild version 17A324\n'
    exit 0
  fi
done
printf '/tmp/App/ContentView.swift:3:5: error: cannot find value in scope\n' >&2
printf '** ARCHIVE FAILED **\n' >&2
exit 65
`
	if err := os.WriteFile(filepath.Join(binDir, "xcodebuild"), []byte(xcodebuild), 0o755); err != nil {
		t.Fatalf("WriteFile() xcodebuild error: %v", err)
	}
	fakeXcrun := filepath.Join(t.TempDir(), "xcrun")
	xcrun := `#!/bin/sh
if [ "$1" = "--find" ]; then
  printf '%s\n' "$DEVELOPER_DIR/usr/bin/$2"
  exit 0
fi
exit 1
`
	if err := os.WriteFile(fakeXcrun, []byte(xcrun), 0o700); err != nil {
		t.Fatalf("WriteFile() xcrun error: %v", err)
	}
	t.Cleanup(localxcode.OverrideTrustedXcrunPathForTesting(fakeXcrun))
	t.Setenv("DEVELOPER_DIR", developerDir)
}

func TestRunXcodeLocalFailuresAreClassified(t *testing.T) {
	workDir := t.TempDir()
	projectPath := filepath.Join(workDir, "App.xcodeproj")
	if err := os.Mkdir(projectPath, 0o755); err != nil {
		t.Fatalf("Mkdir() project error: %v", err)
	}
	archivePath := filepath.Join(workDir, "Existing.xcarchive")
	if err := os.Mkdir(archivePath, 0o755); err != nil {
		t.Fatalf("Mkdir() archive error: %v", err)
	}
	exportOptions := filepath.Join(workDir, "ExportOptions.plist")
	if err := os.WriteFile(exportOptions, []byte(`<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>method</key><string>app-store-connect</string></dict></plist>`), 0o600); err != nil {
		t.Fatalf("WriteFile() export options error: %v", err)
	}
	emptyDir := t.TempDir()
	multiDir := t.TempDir()
	for _, name := range []string{"One.xcodeproj", "Two.xcodeproj"} {
		if err := os.Mkdir(filepath.Join(multiDir, name), 0o755); err != nil {
			t.Fatalf("Mkdir() error: %v", err)
		}
	}

	tests := []struct {
		name          string
		args          []string
		fakeXcode     bool
		wantStderr    string
		wantDiagnosis shared.DiagnosticCode
		wantParameter string
	}{
		{
			name:          "archive xcodebuild failure",
			args:          []string{"xcode", "archive", "--project", projectPath, "--scheme", "App", "--archive-path", filepath.Join(workDir, "out.xcarchive")},
			fakeXcode:     true,
			wantStderr:    "cannot find value in scope",
			wantDiagnosis: shared.DiagnosticDependencyFailed,
		},
		{
			name:          "export xcodebuild failure",
			args:          []string{"xcode", "export", "--archive-path", archivePath, "--export-options", exportOptions, "--ipa-path", filepath.Join(workDir, "out.ipa")},
			fakeXcode:     true,
			wantStderr:    "cannot find value in scope",
			wantDiagnosis: shared.DiagnosticDependencyFailed,
		},
		{
			name:          "version view without a project",
			args:          []string{"xcode", "version", "view", "--project-dir", emptyDir},
			wantStderr:    "Error: xcode version view: no .xcodeproj found in " + emptyDir,
			wantDiagnosis: shared.DiagnosticFileNotFound,
			wantParameter: "--project-dir",
		},
		{
			name:          "version view with a missing project",
			args:          []string{"xcode", "version", "view", "--project", filepath.Join(workDir, "Missing.xcodeproj")},
			wantStderr:    "Error: xcode version view: failed to read Xcode project " + filepath.Join(workDir, "Missing.xcodeproj"),
			wantDiagnosis: shared.DiagnosticFileNotFound,
			wantParameter: "--project",
		},
		{
			name:          "version edit with several projects",
			args:          []string{"xcode", "version", "edit", "--project-dir", multiDir, "--version", "1.2.3"},
			wantStderr:    "Error: xcode version edit: multiple .xcodeproj found in " + multiDir,
			wantDiagnosis: shared.DiagnosticInvalidInput,
			wantParameter: "--project",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			if test.fakeXcode {
				// asc refuses Xcode tool commands off macOS before running them.
				if runtime.GOOS != "darwin" {
					t.Skip("xcodebuild failures require macOS")
				}
				installFailingXcodebuild(t)
			}

			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
			var gotExitCode int
			var gotContext telemetry.EventContext
			emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
				gotExitCode = exitCode
				gotContext = eventContext
			}

			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "4.0.0"); code != ExitError {
					t.Errorf("Run() exit code = %d, want %d", code, ExitError)
				}
			})

			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if gotExitCode != ExitError ||
				gotContext.OutcomeKind != telemetry.OutcomeExpectedNegative ||
				gotContext.DiagnosticCode != string(test.wantDiagnosis) ||
				gotContext.FailureParameter != test.wantParameter {
				t.Fatalf("telemetry exit=%d context=%+v, want expected_negative %s on %q", gotExitCode, gotContext, test.wantDiagnosis, test.wantParameter)
			}
		})
	}
}
