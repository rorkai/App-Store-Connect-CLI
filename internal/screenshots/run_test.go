package screenshots

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAxeMatchesTarget_ParsesJSONWhenStderrHasWarnings(t *testing.T) {
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

func TestRunPlan_ScreenshotStepsDoNotRelaunchApp(t *testing.T) {
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
			BundleID:  "com.example.app",
			UDID:      "SIM-UDID-123",
			OutputDir: t.TempDir(),
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
	if !strings.Contains(lines[0], "simctl launch SIM-UDID-123 com.example.app") {
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
