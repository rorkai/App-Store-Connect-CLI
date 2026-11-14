package screenshots

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func skipWindowsUnixExecutableFixtures(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell executable fixtures require a Unix PATH")
	}
}

func skipWindowsUnixFileModes(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not preserve Unix permission bits")
	}
}

func TestAxeMatchesTarget_ParsesJSONWhenStderrHasWarnings(t *testing.T) {
	skipWindowsUnixExecutableFixtures(t)
	binDir := t.TempDir()
	axePath := filepath.Join(binDir, "axe")
	script := `#!/bin/sh
if [ "$1" = "describe-ui" ]; then
  echo "warning: test noise" 1>&2
  echo '{"AXLabel":"Ready"}'
  exit 0
fi
exit 1
`
	if err := os.WriteFile(axePath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", axePath, err)
	}
	t.Setenv("PATH", binDir)

	label := "Ready"
	matched, err := axeMatchesTarget(context.Background(), "booted", PlanStep{Label: &label})
	if err != nil {
		t.Fatalf("axeMatchesTarget() error = %v", err)
	}
	if !matched {
		t.Fatal("expected matcher to find AXLabel in describe-ui output")
	}
}

func TestRunExternalOutput_IncludesStderrOnFailure(t *testing.T) {
	skipWindowsUnixExecutableFixtures(t)
	binDir := t.TempDir()
	cmdPath := filepath.Join(binDir, "tool")
	script := `#!/bin/sh
echo "stdout text"
echo "stderr text" 1>&2
exit 1
`
	if err := os.WriteFile(cmdPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", cmdPath, err)
	}
	t.Setenv("PATH", binDir)

	_, err := runExternalOutput(context.Background(), "tool")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "stderr text") {
		t.Fatalf("expected stderr output in error, got %v", err)
	}
}

func TestRunPlan_RejectsNilPlan(t *testing.T) {
	_, err := RunPlan(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil plan")
	}
	if !strings.Contains(err.Error(), "plan is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPlanPreservesLiteralOutputDirectorySpelling(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "run ")
	waitMS := 1
	plan := &Plan{
		Version: 1,
		App:     PlanApp{BundleID: "com.example.app", OutputDir: outputDir},
		Steps:   []PlanStep{{Action: ActionWait, DurationMS: &waitMS}},
	}
	result, err := RunPlan(context.Background(), plan)
	if err != nil {
		t.Fatalf("RunPlan() error = %v", err)
	}
	want, err := filepath.Abs(outputDir)
	if err != nil {
		t.Fatalf("resolve output directory: %v", err)
	}
	if result.OutputDir != want {
		t.Fatalf("RunPlan() OutputDir = %q, want %q", result.OutputDir, want)
	}
}

func TestRunPlanUsesDefaultOutputDirForWhitespaceOnly(t *testing.T) {
	waitMS := 1
	plan := &Plan{
		Version: 1,
		App:     PlanApp{BundleID: "com.example.app", OutputDir: "   "},
		Steps:   []PlanStep{{Action: ActionWait, DurationMS: &waitMS}},
	}
	result, err := RunPlan(context.Background(), plan)
	if err != nil {
		t.Fatalf("RunPlan() error = %v", err)
	}
	want, err := filepath.Abs("./screenshots/raw")
	if err != nil {
		t.Fatalf("resolve default output directory: %v", err)
	}
	if result.OutputDir != want {
		t.Fatalf("RunPlan() OutputDir = %q, want default %q", result.OutputDir, want)
	}
}

func TestRunPlan_ScreenshotStepsDoNotRelaunchApp(t *testing.T) {
	skipWindowsUnixExecutableFixtures(t)
	binDir := t.TempDir()
	logDir := t.TempDir()
	xcrunLog := filepath.Join(logDir, "xcrun.log")
	axeLog := filepath.Join(logDir, "axe.log")
	templatePNG := filepath.Join(logDir, "template.png")
	writeMinimalPNG(t, templatePNG, 10, 10)

	writeExecutable(t, filepath.Join(binDir, "xcrun"), `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$XCRUN_LOG"
`)
	writeExecutable(t, filepath.Join(binDir, "axe"), `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$AXE_LOG"
out=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--output" ]; then
    out="$2"
    break
  fi
  shift
done
if [ -z "$out" ]; then
  exit 1
fi
cp "$AXE_TEMPLATE_PNG" "$out"
`)

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XCRUN_LOG", xcrunLog)
	t.Setenv("AXE_LOG", axeLog)
	t.Setenv("AXE_TEMPLATE_PNG", templatePNG)

	name1 := "home"
	name2 := "settings"
	plan := &Plan{
		Version: 1,
		App: PlanApp{
			BundleID:        "com.example.app",
			UDID:            "SIM-UDID-123",
			OutputDir:       t.TempDir(),
			LaunchArguments: []string{"--fixture", "empty"},
		},
		Steps: []PlanStep{
			{Action: ActionLaunch},
			{Action: ActionScreenshot, Name: &name1},
			{Action: ActionScreenshot, Name: &name2},
		},
	}

	result, err := RunPlan(context.Background(), plan)
	if err != nil {
		t.Fatalf("RunPlan() error = %v", err)
	}
	if len(result.Steps) != 3 {
		t.Fatalf("expected 3 step results, got %d", len(result.Steps))
	}

	xcrunArgs, err := os.ReadFile(xcrunLog)
	if err != nil {
		t.Fatalf("read xcrun log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(xcrunArgs)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected exactly one app launch, got %d (%q)", len(lines), string(xcrunArgs))
	}
	if !strings.Contains(lines[0], "simctl launch SIM-UDID-123 com.example.app --fixture empty") {
		t.Fatalf("unexpected launch args %q", lines[0])
	}

	axeArgs, err := os.ReadFile(axeLog)
	if err != nil {
		t.Fatalf("read axe log: %v", err)
	}
	if strings.Count(string(axeArgs), "screenshot") != 2 {
		t.Fatalf("expected two screenshot captures, got %q", string(axeArgs))
	}
}

func TestRunPlan_TerminatesOnlyTheInitialMatrixLaunch(t *testing.T) {
	skipWindowsUnixExecutableFixtures(t)
	binDir := t.TempDir()
	xcrunLog := filepath.Join(t.TempDir(), "xcrun.log")
	writeExecutable(t, filepath.Join(binDir, "xcrun"), `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$XCRUN_LOG"
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XCRUN_LOG", xcrunLog)

	plan := &Plan{
		Version: 1,
		App: PlanApp{
			BundleID:                "com.example.app",
			UDID:                    "SIM-UDID-123",
			OutputDir:               t.TempDir(),
			LaunchArguments:         []string{"--fixture", "empty"},
			terminateRunningProcess: true,
		},
		Steps: []PlanStep{{Action: ActionLaunch}, {Action: ActionLaunch}},
	}
	if _, err := RunPlan(context.Background(), plan); err != nil {
		t.Fatalf("RunPlan() error = %v", err)
	}
	data, err := os.ReadFile(xcrunLog)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", xcrunLog, err)
	}
	launches := strings.Split(strings.TrimSpace(string(data)), "\n")
	want := []string{
		"simctl launch --terminate-running-process SIM-UDID-123 com.example.app --fixture empty",
		"simctl launch SIM-UDID-123 com.example.app --fixture empty",
	}
	if len(launches) != len(want) || launches[0] != want[0] || launches[1] != want[1] {
		t.Fatalf("launches = %q, want %q", launches, want)
	}
}

func TestRunPlanResolvesBootedSimulatorWhenUDIDOmitted(t *testing.T) {
	tests := []struct {
		name    string
		devices string
		wantErr []string
	}{
		{
			name:    "one booted",
			devices: `[{"udid":"BOOTED-1","name":"iPhone 17","state":"Booted","isAvailable":true},{"udid":"SHUTDOWN-1","name":"iPad Air","state":"Shutdown","isAvailable":true}]`,
		},
		{
			name:    "none booted",
			devices: `[{"udid":"SHUTDOWN-1","name":"iPad Air","state":"Shutdown","isAvailable":true}]`,
			wantErr: []string{"no iPhone or iPad simulator is booted", "--udid"},
		},
		{
			name:    "another platform booted alongside",
			devices: `[{"udid":"BOOTED-1","name":"iPhone 17","state":"Booted","isAvailable":true},{"udid":"BOOTED-TV","name":"Apple TV 4K","state":"Booted","isAvailable":true}]`,
		},
		{
			name:    "several booted",
			devices: `[{"udid":"BOOTED-1","name":"iPhone 17","state":"Booted","isAvailable":true},{"udid":"BOOTED-2","name":"iPad Air","state":"Booted","isAvailable":true}]`,
			wantErr: []string{"2 simulators are booted", "iPhone 17 (BOOTED-1)", "iPad Air (BOOTED-2)", "--udid"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			binDir := t.TempDir()
			logDir := t.TempDir()
			xcrunLog := filepath.Join(logDir, "xcrun.log")
			axeLog := filepath.Join(logDir, "axe.log")
			templatePNG := filepath.Join(logDir, "template.png")
			writeMinimalPNG(t, templatePNG, 10, 10)
			writeExecutable(t, filepath.Join(binDir, "xcrun"), `#!/bin/sh
set -eu
if [ "$1" = "simctl" ] && [ "$2" = "list" ]; then
  printf '{"devices":{"runtime":%s}}\n' "$SIMCTL_DEVICES"
  exit 0
fi
printf '%s\n' "$*" >> "$XCRUN_LOG"
`)
			writeExecutable(t, filepath.Join(binDir, "axe"), `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$AXE_LOG"
out=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--output" ]; then out="$2"; break; fi
  shift
done
if [ -n "$out" ]; then cp "$AXE_TEMPLATE_PNG" "$out"; fi
`)
			t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("SIMCTL_DEVICES", test.devices)
			t.Setenv("XCRUN_LOG", xcrunLog)
			t.Setenv("AXE_LOG", axeLog)
			t.Setenv("AXE_TEMPLATE_PNG", templatePNG)

			text := "hello"
			name := "home"
			plan := &Plan{
				Version: 1,
				App:     PlanApp{BundleID: "com.example.app", OutputDir: t.TempDir()},
				Steps: []PlanStep{
					{Action: ActionLaunch},
					{Action: ActionType, Text: &text},
					{Action: ActionScreenshot, Name: &name},
				},
			}
			result, err := RunPlan(context.Background(), plan)
			if test.wantErr != nil {
				if err == nil {
					t.Fatalf("RunPlan() error = nil, want %q", test.wantErr)
				}
				for _, want := range test.wantErr {
					if !strings.Contains(err.Error(), want) {
						t.Fatalf("RunPlan() error = %q, want it to contain %q", err, want)
					}
				}
				if _, statErr := os.Stat(axeLog); !os.IsNotExist(statErr) {
					t.Fatalf("axe ran before the simulator was resolved")
				}
				return
			}
			if err != nil {
				t.Fatalf("RunPlan() error = %v", err)
			}
			axeArgs, err := os.ReadFile(axeLog)
			if err != nil {
				t.Fatalf("read axe log: %v", err)
			}
			calls := strings.Split(strings.TrimSpace(string(axeArgs)), "\n")
			if len(calls) != 2 {
				t.Fatalf("axe calls = %q, want type and screenshot", calls)
			}
			for _, call := range calls {
				if !strings.HasSuffix(call, "--udid BOOTED-1") {
					t.Fatalf("axe call %q does not target BOOTED-1", call)
				}
			}
			launch, err := os.ReadFile(xcrunLog)
			if err != nil {
				t.Fatalf("read xcrun log: %v", err)
			}
			if got := strings.TrimSpace(string(launch)); got != "simctl launch BOOTED-1 com.example.app" {
				t.Fatalf("launch args = %q", got)
			}
			if result.UDID != "BOOTED-1" {
				t.Fatalf("RunResult.UDID = %q, want BOOTED-1", result.UDID)
			}
		})
	}
}
