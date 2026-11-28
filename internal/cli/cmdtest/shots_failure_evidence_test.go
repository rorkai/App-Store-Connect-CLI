package cmdtest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestShotsFailureScreenshot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell executable fixtures require a Unix PATH")
	}
	type evidence struct {
		Attempt, StepIndex                     int
		Action, Status, Path, Error, StepError string
	}
	for _, test := range []struct {
		name, command                                     string
		enabled, captureFails, replaceRoot, retrySucceeds bool
		attempts                                          int
	}{
		{name: "standalone opt in", enabled: true, attempts: 1},
		{name: "standalone default", attempts: 1},
		{name: "collection failure preserves step error", enabled: true, captureFails: true, attempts: 1},
		{name: "selected output root is pinned", enabled: true, replaceRoot: true, attempts: 1},
		{name: "matrix keeps both failed attempts", command: "matrix", enabled: true, attempts: 2},
		{name: "matrix keeps evidence after successful retry", command: "matrix", enabled: true, retrySucceeds: true, attempts: 2},
		{name: "matrix collection failure preserves plan failure", command: "matrix", enabled: true, captureFails: true, attempts: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir, binDir := t.TempDir(), t.TempDir()
			fixture := filepath.Join(dir, "fixture.png")
			writeShotsMatrixPNG(t, fixture)
			callsPath := filepath.Join(dir, "calls")
			writeFile(t, callsPath, "")
			countPath := filepath.Join(dir, "count")
			writeFile(t, countPath, "")
			outputDir := filepath.Join(dir, "output")
			outside := filepath.Join(dir, "outside")
			if err := os.Mkdir(outside, 0o700); err != nil {
				t.Fatal(err)
			}
			writeShotsMatrixExecutable(t, filepath.Join(binDir, "xcrun"), `#!/bin/sh
if [ "$2" = "list" ]; then
 printf '%s\n' '{"devices":{"runtime":[{"udid":"SIM-UDID","state":"Booted","isAvailable":true}]}}'
 exit 0
fi
printf '%s\n' "$*" >> "$FAILURE_CALLS"
if [ "$2" = "ui" ] && [ "$#" = "4" ]; then printf '%s\n' light; fi
`)
			writeShotsMatrixExecutable(t, filepath.Join(binDir, "axe"), `#!/bin/sh
if [ "$1" = "--version" ]; then printf '%s\n' '2.0.0'; exit 0; fi
printf 'axe %s\n' "$*" >> "$FAILURE_CALLS"
if [ "$1" = "type" ]; then
 if [ "$FAILURE_RETRY" = "true" ] && [ -s "$FAILURE_COUNT" ]; then exit 0; fi
 printf x > "$FAILURE_COUNT"
 if [ "$FAILURE_REPLACE_ROOT" = "true" ]; then
  /bin/mv "$FAILURE_OUTPUT" "$FAILURE_OUTPUT-original"
  /bin/ln -s "$FAILURE_OUTSIDE" "$FAILURE_OUTPUT"
 fi
 echo 'original typing failure' >&2
 exit 7
fi
if [ "$FAILURE_CAPTURE_FAILS" = "true" ]; then echo 'capture failed' >&2; exit 9; fi
while [ "$#" -gt 0 ]; do
 if [ "$1" = "--output" ]; then /bin/cp "$FAILURE_FIXTURE" "$2"; exit 0; fi
 shift
done
exit 1
`)
			t.Setenv("PATH", binDir)
			t.Setenv("FAILURE_CALLS", callsPath)
			t.Setenv("FAILURE_COUNT", countPath)
			t.Setenv("FAILURE_FIXTURE", fixture)
			t.Setenv("FAILURE_OUTPUT", outputDir)
			t.Setenv("FAILURE_OUTSIDE", outside)
			boolean := func(v bool) string {
				if v {
					return "true"
				}
				return "false"
			}
			t.Setenv("FAILURE_RETRY", boolean(test.retrySucceeds))
			t.Setenv("FAILURE_CAPTURE_FAILS", boolean(test.captureFails))
			t.Setenv("FAILURE_REPLACE_ROOT", boolean(test.replaceRoot))
			base := map[string]any{"version": 1, "app": map[string]any{"bundle_id": "com.example.app", "udid": "SIM-UDID", "output_dir": outputDir}, "defaults": map[string]any{"failure_screenshot": test.enabled}, "steps": []any{map[string]any{"action": "launch"}, map[string]any{"action": "type", "text": "fixture"}, map[string]any{"action": "screenshot", "name": "home"}}}
			data, err := json.Marshal(base)
			if err != nil {
				t.Fatal(err)
			}
			planPath := filepath.Join(dir, "base.json")
			writeFile(t, planPath, string(data))
			command := "run"
			if test.command == "matrix" {
				command = "matrix"
				planPath = filepath.Join(dir, "matrix.json")
				matrix := map[string]any{"version": 1, "base_plan": "base.json", "devices": []any{map[string]any{"id": "phone", "udid": "SIM-UDID"}}, "locales": []string{"en-US"}, "appearances": []string{"light"}, "content_variants": []any{map[string]any{"id": "default"}}, "execution": map[string]any{"max_attempts": test.attempts}, "output": map[string]any{"raw_dir": outputDir, "review_dir": filepath.Join(dir, "review")}}
				data, err = json.Marshal(matrix)
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, planPath, string(data))
			}
			stdout, stderr, code := runWithDeps(t, nil, "screenshots", command, "--plan", planPath, "--output", "json")
			var receipt struct {
				FailureEvidence *evidence
				Steps           []struct{ Error string }
				Cells           []struct {
					ID, Status, FailureCode string
					FailureEvidence         []evidence
				}
			}
			if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
				t.Fatalf("receipt=%q stderr=%q: %v", stdout, stderr, err)
			}
			wantCode := 1
			if test.retrySucceeds {
				wantCode = 0
			}
			if code != wantCode || (wantCode == 1 && stderr == "") {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			var items []evidence
			if command == "run" {
				if !strings.Contains(stderr, "step 2 (type): axe: exit status 7 (output: original typing failure)") || len(receipt.Steps) != 2 || receipt.Steps[1].Error != "axe: exit status 7 (output: original typing failure)" {
					t.Fatalf("original failure changed: stdout=%q stderr=%q", stdout, stderr)
				}
				if receipt.FailureEvidence != nil {
					items = append(items, *receipt.FailureEvidence)
				}
			} else {
				if len(receipt.Cells) != 1 || receipt.Cells[0].ID == "" {
					t.Fatalf("missing cell identity: %s", stdout)
				}
				cell := receipt.Cells[0]
				items = cell.FailureEvidence
				if !test.retrySucceeds && cell.FailureCode != "plan_failed" {
					t.Fatalf("original matrix failure changed: %s", stdout)
				}
				manifest, err := os.ReadFile(filepath.Join(dir, "review", "manifest.json"))
				if err != nil {
					t.Fatal(err)
				}
				var review struct {
					Cells []struct{ FailureEvidence []evidence }
				}
				if err := json.Unmarshal(manifest, &review); err != nil || len(review.Cells) != 1 || len(review.Cells[0].FailureEvidence) != len(items) {
					t.Fatalf("review evidence missing: %s; %v", manifest, err)
				}
			}
			wantCount := test.attempts
			if !test.enabled {
				wantCount = 0
			}
			if test.retrySucceeds {
				wantCount = 1
			}
			if len(items) != wantCount {
				t.Fatalf("evidence=%+v; want %d failed attempts", items, wantCount)
			}
			for i, item := range items {
				wantStepError := "axe: exit status 7 (output: original typing failure)"
				if command == "matrix" {
					wantStepError = "step execution failed"
				}
				if item.StepError != wantStepError {
					t.Fatalf("failed attempt lost its original step error: %+v", item)
				}
				if item.Attempt != i+1 || item.StepIndex != 2 || item.Action != "type" {
					t.Fatalf("wrong attempt or step: %+v", item)
				}
				if test.captureFails || test.replaceRoot {
					if item.Status != "failed" || item.Path != "" || item.Error == "" {
						t.Fatalf("failed collection claims an artifact: %+v", item)
					}
				} else {
					if item.Status != "captured" || item.Path == "" || filepath.IsAbs(item.Path) || strings.Contains(item.Path, "..") || item.Error != "" {
						t.Fatalf("invalid relative evidence: %+v", item)
					}
					image, err := os.ReadFile(filepath.Join(outputDir, filepath.FromSlash(item.Path)))
					if err != nil {
						t.Fatal(err)
					}
					info, err := os.Stat(filepath.Join(outputDir, filepath.FromSlash(item.Path)))
					if err != nil || info.Mode().Perm() != 0o600 {
						t.Fatalf("failure evidence must be private: %v; %v", info, err)
					}
					original, err := os.ReadFile(fixture)
					if err != nil || string(image) != string(original) {
						t.Fatalf("captured evidence differs from provider image: %v", err)
					}
				}
			}
			calls, err := os.ReadFile(callsPath)
			if err != nil {
				t.Fatal(err)
			}
			captures := strings.Count(string(calls), "axe screenshot ")
			wantCaptures := wantCount
			if test.retrySucceeds {
				wantCaptures++
			}
			if captures != wantCaptures {
				t.Fatalf("capture calls=%d want=%d; %s", captures, wantCaptures, calls)
			}
			entries, err := os.ReadDir(outside)
			if err != nil || len(entries) != 0 {
				t.Fatalf("evidence escaped selected root: %v %v", entries, err)
			}
		})
	}
}
