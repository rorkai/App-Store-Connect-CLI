package cmdtest

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestShotsCaptureToolPreflight(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell executable fixtures require a Unix PATH")
	}
	for _, test := range []struct {
		name, command, version, defaults, step, wantError string
		missingTool                                       bool
	}{
		{name: "run rejects unsupported hold before launch", version: "1.6.0", step: `{"action":"long_press","x":1,"y":2}`, wantError: "step 2 (long_press) requires AXe 2.0.0 or later; found 1.6.0"},
		{name: "matrix rejects unsupported hold before device changes", command: "matrix", version: "1.6.0", step: `{"action":"long_press","x":1,"y":2}`, wantError: "step 2 (long_press) requires AXe 2.0.0 or later; found 1.6.0"},
		{name: "targeted scroll needs newer tool", version: "1.6.0", step: `{"action":"scroll","direction":"up","label":"Next"}`, wantError: "step 2 (scroll) requires AXe 2.0.0 or later; found 1.6.0"},
		{name: "status bar needs newer tool", version: "1.6.0", defaults: `"status_bar":"screenshot"`, step: `{"action":"launch"}`, wantError: "defaults.status_bar requires AXe 2.0.0 or later; found 1.6.0"},
		{name: "pinch needs newer tool", version: "1.6.0", step: `{"action":"pinch","scale":2}`, wantError: "step 2 (pinch) requires AXe 2.0.0 or later; found 1.6.0"},
		{name: "rotation needs newer tool", version: "1.6.0", step: `{"action":"rotate","x":1,"y":2,"radius":3,"degrees":90}`, wantError: "step 2 (rotate) requires AXe 2.0.0 or later; found 1.6.0"},
		{name: "double tap needs newer tool", version: "1.6.0", step: `{"action":"double_tap","x":1,"y":2}`, wantError: "step 2 (double_tap) requires AXe 2.0.0 or later; found 1.6.0"},
		{name: "supported older swipe and basic scroll", version: "1.6.0", step: `{"action":"swipe","x":1,"y":2,"end_x":3,"end_y":4},{"action":"scroll","direction":"up"}`},
		{name: "supported advanced action", version: "2.0.0", step: `{"action":"long_press","x":1,"y":2}`},
		{name: "unknown version stops advanced action", version: "development", step: `{"action":"long_press","x":1,"y":2}`, wantError: "cannot determine AXe version required by step 2 (long_press)"},
		{name: "missing executable stops before launch", missingTool: true, step: `{"action":"tap","x":1,"y":2}`, wantError: "AXe is required by step 2 (tap); install axe and add it to PATH"},
		{name: "launch needs no AXe", missingTool: true, step: `{"action":"launch"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir, binDir := t.TempDir(), t.TempDir()
			callsPath := filepath.Join(dir, "calls")
			writeFile(t, callsPath, "")
			writeShotsMatrixExecutable(t, filepath.Join(binDir, "xcrun"), `#!/bin/sh
if [ "$2" = "list" ]; then
  printf '%s\n' '{"devices":{"runtime":[{"udid":"SIM-UDID","state":"Booted","isAvailable":true}]}}'
  exit 0
fi
printf '%s\n' "$*" >> "$PREFLIGHT_CALLS"
if [ "$2" = "ui" ] && [ "$#" = "4" ]; then
  printf '%s\n' light
fi
`)
			if !test.missingTool {
				writeShotsMatrixExecutable(t, filepath.Join(binDir, "axe"), `#!/bin/sh
if [ "$1" = "--version" ]; then
  printf '%s\n' "$PREFLIGHT_VERSION"
  exit 0
fi
printf 'axe %s\n' "$*" >> "$PREFLIGHT_CALLS"
`)
			}
			t.Setenv("PATH", binDir)
			t.Setenv("PREFLIGHT_CALLS", callsPath)
			t.Setenv("PREFLIGHT_VERSION", test.version)
			planPath := filepath.Join(dir, "base.json")
			steps := test.step
			if test.command == "matrix" {
				steps += `,{"action":"screenshot","name":"home"}`
			}
			writeFile(t, planPath, `{"version":1,"app":{"bundle_id":"com.example.app","udid":"SIM-UDID","output_dir":"`+filepath.Join(dir, "output")+`"},"defaults":{`+test.defaults+`},"steps":[{"action":"launch"},`+steps+`]}`)
			command := "run"
			if test.command == "matrix" {
				command = "matrix"
				planPath = filepath.Join(dir, "matrix.json")
				writeFile(t, planPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"raw","review_dir":"review"}}`)
			}
			stdout, stderr, code := runWithDeps(t, nil, "screenshots", command, "--plan", planPath, "--output", "json")
			calls, err := os.ReadFile(callsPath)
			if err != nil {
				t.Fatal(err)
			}
			if test.wantError != "" {
				wantReceipt := command == "matrix"
				validOutput := stdout == ""
				if wantReceipt {
					validOutput = strings.Contains(stdout, `"failureCode":"capture_tool_requirement"`) && strings.Contains(stdout, "capture tool requirements are not met")
					manifest, readErr := os.ReadFile(filepath.Join(dir, "review", "manifest.json"))
					if readErr != nil || !strings.Contains(string(manifest), `"capture_tool_requirement"`) || !strings.Contains(string(manifest), "capture tool requirements are not met") {
						t.Fatalf("review=%s error=%v; want retained tool preflight failure", manifest, readErr)
					}
				}
				if code != 1 || !validOutput || !strings.Contains(stderr, test.wantError) || len(calls) != 0 {
					t.Fatalf("exit=%d stdout=%q stderr=%q calls=%q; want %q before device or action calls", code, stdout, stderr, calls, test.wantError)
				}
			} else if code != 0 || stderr != "" || !strings.Contains(stdout, `"status":"ok"`) || !strings.Contains(string(calls), "simctl launch SIM-UDID com.example.app\n") {
				t.Fatalf("exit=%d stdout=%q stderr=%q calls=%q; want completed supported steps", code, stdout, stderr, calls)
			}
		})
	}
}
