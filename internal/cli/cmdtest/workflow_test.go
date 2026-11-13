package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func writeWorkflowJSON(t *testing.T, dir, content string) string {
	t.Helper()
	ascDir := filepath.Join(dir, ".asc")
	if err := os.MkdirAll(ascDir, 0o755); err != nil {
		t.Fatalf("mkdir .asc: %v", err)
	}
	path := filepath.Join(ascDir, "workflow.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write workflow.json: %v", err)
	}
	return path
}

func TestWorkflowUsageErrors(t *testing.T) {
	path := writeWorkflowJSON(t, t.TempDir(), `{
		"workflows": {
			"beta": {"steps": ["echo hello"]}
		}
	}`)

	runUsageExitCases(t, []usageExitCase{
		{
			name:    "run missing name",
			args:    []string{"workflow", "run"},
			wantErr: "workflow name is required",
		},
		{
			name:    "run invalid param",
			args:    []string{"workflow", "run", "--file", path, "beta", "NOCOLON"},
			wantErr: "NOCOLON",
		},
		{
			name:    "run unknown flag after name",
			args:    []string{"workflow", "run", "--file", path, "beta", "--unknown"},
			wantErr: "unknown flag",
		},
		{
			name:    "run dry-run flag after name invalid value",
			args:    []string{"workflow", "run", "--file", path, "beta", "--dry-run=maybe"},
			wantErr: "invalid value",
		},
		{
			name:    "run file flag after name missing value",
			args:    []string{"workflow", "run", "--file", path, "beta", "--file", "--dry-run"},
			wantErr: "--file requires a value",
		},
		{
			name:    "run file flag after name unknown long flag is missing value",
			args:    []string{"workflow", "run", "--file", path, "beta", "--file", "--unknown"},
			wantErr: "--file requires a value",
		},
		{
			name:    "run resume flag after name missing value",
			args:    []string{"workflow", "run", "--file", path, "beta", "--resume"},
			wantErr: "--resume requires a value",
		},
		{
			name:    "validate unexpected args",
			args:    []string{"workflow", "validate", "--file", path, "extra"},
			wantErr: "unexpected argument",
		},
		{
			name:    "list unexpected args",
			args:    []string{"workflow", "list", "--file", path, "extra"},
			wantErr: "unexpected argument",
		},
	})
}

func TestWorkflowMissingFileIsRuntimeError(t *testing.T) {
	const missingPath = "/nonexistent/workflow.json"
	for _, args := range [][]string{
		{"workflow", "run", "--file", missingPath, "beta"},
		{"workflow", "validate", "--file", missingPath},
		{"workflow", "list", "--file", missingPath},
	} {
		t.Run(args[1], func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitError {
					t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitError)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			want := "Error: workflow " + args[1] + ": read workflow: open " + missingPath
			if !strings.HasPrefix(stderr, want) {
				t.Fatalf("stderr = %q, want prefix %q", stderr, want)
			}
		})
	}
}

func TestWorkflowRun_PrivateWorkflow(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"helper": {"private": true, "steps": ["echo helper"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--file", path, "helper"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error for private workflow")
		}
		if !strings.Contains(err.Error(), "private") {
			t.Fatalf("expected 'private' in error, got %v", err)
		}
		// Must NOT be a ReportedError — cmd/run.go needs to print it to stderr.
		if _, ok := errors.AsType[ReportedError](err); ok {
			t.Fatal("private workflow error must not be ReportedError (would cause silent exit)")
		}
	})
}

func TestWorkflowRun_UnknownWorkflow(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"beta": {"steps": ["echo hello"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--file", path, "nonexistent"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error for unknown workflow")
		}
		if !strings.Contains(err.Error(), "unknown workflow") {
			t.Fatalf("expected 'unknown workflow' in error, got %v", err)
		}
		// Must NOT be a ReportedError — cmd/run.go needs to print it to stderr.
		if _, ok := errors.AsType[ReportedError](err); ok {
			t.Fatal("unknown workflow error must not be ReportedError (would cause silent exit)")
		}
	})
}

func TestWorkflowRun_DryRun(t *testing.T) {
	tests := []struct {
		name string
		args func(path string) []string
	}{
		{name: "flag before name", args: func(path string) []string {
			return []string{"workflow", "run", "--dry-run", "--file", path, "beta"}
		}},
		{name: "flag after name", args: func(path string) []string {
			return []string{"workflow", "run", "--file", path, "beta", "--dry-run"}
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeWorkflowJSON(t, dir, `{
				"workflows": {
					"beta": {"steps": ["echo hello world"]}
				}
			}`)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args(path)); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if !strings.Contains(stderr, "[dry-run] step 1: echo hello world") {
				t.Fatalf("expected dry-run preview in stderr, got %q", stderr)
			}

			var result map[string]any
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("expected JSON stdout, got %q: %v", stdout, err)
			}
			if result["status"] != "ok" {
				t.Fatalf("expected status=ok, got %v", result["status"])
			}
		})
	}
}

func TestWorkflowRun_FileFlagAfterName_AllowsDashPrefixedPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "-workflow.json")
	if err := os.WriteFile(path, []byte(`{
		"workflows": {
			"beta": {"steps": ["echo hello"]}
		}
	}`), 0o600); err != nil {
		t.Fatalf("write workflow file: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "beta", "--file", "-workflow.json", "--dry-run"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected JSON stdout, got %q: %v", stdout, err)
	}
	if result["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", result["status"])
	}
}

func TestWorkflowValidate_ValidFile(t *testing.T) {
	// Workflow validation is JSON-only even when the global default prefers tables.
	t.Setenv("ASC_DEFAULT_OUTPUT", "table")

	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"beta": {"steps": ["echo hello"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "validate", "--file", path}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected JSON stdout, got %q: %v", stdout, err)
	}
	if result["valid"] != true {
		t.Fatalf("expected valid=true, got %v", result["valid"])
	}
}

func TestWorkflowValidate_ExplicitJSONOutput(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"beta": {"steps": ["echo hello"]}
		}
	}`)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{
			"workflow", "validate",
			"--file", path,
			"--output", "json",
		}, "1.2.3")
	})

	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, rootcmd.ExitSuccess, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if strings.Contains(stdout, "\n  ") {
		t.Fatalf("expected minified JSON when stdout is piped, got %q", stdout)
	}

	var result struct {
		Valid bool `json:"valid"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected JSON stdout, got %q: %v", stdout, err)
	}
	if !result.Valid {
		t.Fatalf("expected valid=true, got %q", stdout)
	}
}

func TestWorkflowValidate_UnsupportedOutputExitsWithUsageCode(t *testing.T) {
	for _, format := range []string{"table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run([]string{
					"workflow", "validate",
					"--file", "/definitely/missing/workflow.json",
					"--output", format,
				}, "1.2.3")
			})

			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, `(got "`+format+`")`) {
				t.Fatalf("expected standard output validation error, got %q", stderr)
			}
			if strings.Contains(stderr, "read workflow") {
				t.Fatalf("expected output validation before file I/O, got %q", stderr)
			}
		})
	}
}

func TestWorkflowValidate_InvalidWorkflowWithExplicitJSONExitsWithErrorCode(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"beta": {"steps": []}
		}
	}`)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{
			"workflow", "validate",
			"--output", "json",
			"--file", path,
		}, "1.2.3")
	})

	if code != rootcmd.ExitError {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, rootcmd.ExitError, stderr)
	}
	if want := "Error: workflow validate: found 1 error(s)\n"; stderr != want {
		t.Fatalf("stderr = %q, want one error line %q", stderr, want)
	}

	var result struct {
		Valid  bool  `json:"valid"`
		Errors []any `json:"errors"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected JSON stdout, got %q: %v", stdout, err)
	}
	if result.Valid || len(result.Errors) == 0 {
		t.Fatalf("expected valid=false with errors, got %q", stdout)
	}
}

func TestWorkflowValidate_MalformedWorkflowWithExplicitJSONExitsWithErrorCode(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{not valid json`)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{
			"workflow", "validate",
			"--file", path,
			"--output", "json",
		}, "1.2.3")
	})

	if code != rootcmd.ExitError {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, rootcmd.ExitError, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout when the workflow cannot be parsed, got %q", stdout)
	}
	if !strings.Contains(stderr, "parse workflow JSON") {
		t.Fatalf("expected workflow parse error on stderr, got %q", stderr)
	}
}

func TestWorkflowValidate_InvalidFile(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"beta": {"steps": []}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "validate", "--file", path}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error for invalid workflow")
		}
		if _, ok := errors.AsType[ReportedError](err); !ok {
			t.Fatalf("expected ReportedError, got %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected JSON stdout, got %q: %v", stdout, err)
	}
	if result["valid"] != false {
		t.Fatalf("expected valid=false, got %v", result["valid"])
	}
	errs, ok := result["errors"].([]any)
	if !ok || len(errs) == 0 {
		t.Fatalf("expected non-empty errors array, got %v", result["errors"])
	}
}

func TestWorkflowList(t *testing.T) {
	const withPrivate = `{
		"workflows": {
			"beta":   {"steps": ["echo beta"]},
			"helper": {"private": true, "steps": ["echo helper"]}
		}
	}`
	tests := []struct {
		name      string
		content   string
		all       bool
		wantNames []string
	}{
		{
			name:      "single workflow",
			content:   `{"workflows": {"beta": {"steps": ["echo hi"]}}}`,
			wantNames: []string{"beta"},
		},
		{
			name: "sorted by name",
			content: `{
				"workflows": {
					"release": {"steps": ["echo r"]},
					"alpha": {"steps": ["echo a"]},
					"beta": {"steps": ["echo b"]}
				}
			}`,
			wantNames: []string{"alpha", "beta", "release"},
		},
		{
			name:      "hides private",
			content:   withPrivate,
			wantNames: []string{"beta"},
		},
		{
			name:      "all includes private",
			content:   withPrivate,
			all:       true,
			wantNames: []string{"beta", "helper"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeWorkflowJSON(t, t.TempDir(), test.content)
			args := []string{"workflow", "list", "--file", path}
			if test.all {
				args = []string{"workflow", "list", "--all", "--file", path}
			}

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			var workflows []map[string]any
			if err := json.Unmarshal([]byte(stdout), &workflows); err != nil {
				t.Fatalf("expected JSON array, got %q: %v", stdout, err)
			}
			var names []string
			for _, workflow := range workflows {
				name, _ := workflow["name"].(string)
				names = append(names, name)
			}
			if strings.Join(names, ",") != strings.Join(test.wantNames, ",") {
				t.Fatalf("workflow names = %v, want %v", names, test.wantNames)
			}
		})
	}
}

func TestWorkflowRun_Success(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"test": {"steps": ["echo workflow_success"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--file", path, "test"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected JSON stdout, got %q: %v", stdout, err)
	}
	if result["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", result["status"])
	}
	if result["workflow"] != "test" {
		t.Fatalf("expected workflow=test, got %v", result["workflow"])
	}
	if !strings.Contains(stderr, "workflow_success") {
		t.Fatalf("expected command output on stderr, got %q", stderr)
	}
}

func TestWorkflowRun_PrettyJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"test": {"steps": ["echo hi"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--pretty", "--file", path, "test"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	// Pretty JSON should have indentation
	if !strings.Contains(stdout, "  \"workflow\"") {
		t.Fatalf("expected indented JSON, got %q", stdout)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected valid JSON, got %q: %v", stdout, err)
	}
}

func TestWorkflowRun_WithParams(t *testing.T) {
	for _, param := range []string{"TEST:yes", "TEST=yes"} {
		t.Run(param, func(t *testing.T) {
			path := writeWorkflowJSON(t, t.TempDir(), `{
				"workflows": {
					"test": {"steps": ["echo RESULT_IS_$TEST"]}
				}
			}`)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{"workflow", "run", "--file", path, "test", param}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if !strings.Contains(stderr, "RESULT_IS_yes") {
				t.Fatalf("expected runtime param in command output, got %q", stderr)
			}
			var result map[string]any
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("expected JSON, got %q: %v", stdout, err)
			}
			if result["status"] != "ok" {
				t.Fatalf("expected status=ok, got %v", result["status"])
			}
		})
	}
}

func TestWorkflowRun_SkippedWorkflowStepIncludesName(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"main": {
				"steps": [
					{"workflow": "helper", "if": "RUN_HELPER"},
					"echo done"
				]
			},
			"helper": {"steps": ["echo from-helper"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--file", path, "main"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected JSON, got %q: %v", stdout, err)
	}
	steps, ok := result["steps"].([]any)
	if !ok {
		t.Fatalf("expected steps array, got %T: %v", result["steps"], result["steps"])
	}
	if len(steps) < 1 {
		t.Fatalf("expected at least 1 step, got %d", len(steps))
	}
	skipped, ok := steps[0].(map[string]any)
	if !ok {
		t.Fatalf("expected steps[0] object, got %T: %v", steps[0], steps[0])
	}
	if skipped["status"] != "skipped" {
		t.Fatalf("expected skipped, got %v", skipped["status"])
	}
	if skipped["workflow"] != "helper" {
		t.Fatalf("expected skipped step to include workflow='helper', got %v", skipped["workflow"])
	}
}

func TestWorkflowRun_NoHooks_OmitsHooksKey(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"test": {"steps": ["echo ok"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--file", path, "test"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected JSON, got %q: %v", stdout, err)
	}
	if result["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", result["status"])
	}
	if _, ok := result["hooks"]; ok {
		t.Fatalf("expected hooks key to be omitted when no hooks are configured, got %v", result["hooks"])
	}
}

func TestWorkflowRun_ResumeFlagAfterName(t *testing.T) {
	dir := t.TempDir()
	flagPath := filepath.Join(dir, "allow")
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"release": {
				"steps": [
					{
						"name": "upload",
						"run": "printf '{\"buildId\":\"build-42\"}'",
						"outputs": {
							"BUILD_ID": "$.buildId"
						}
					},
					{
						"name": "distribute",
						"run": "if [ -f `+flagPath+` ] && [ ${steps.upload.BUILD_ID} = 'build-42' ]; then echo distributed; else exit 9; fi"
					}
				]
			}
		}
	}`)

	root1 := RootCommand("1.2.3")
	root1.FlagSet.SetOutput(io.Discard)

	stdout1, _ := captureOutput(t, func() {
		if err := root1.Parse([]string{"workflow", "run", "--file", path, "release"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root1.Run(context.Background())
		if err == nil {
			t.Fatal("expected first run error")
		}
		if _, ok := errors.AsType[ReportedError](err); !ok {
			t.Fatalf("expected ReportedError, got %v", err)
		}
	})

	var first map[string]any
	if err := json.Unmarshal([]byte(stdout1), &first); err != nil {
		t.Fatalf("expected JSON stdout, got %q: %v", stdout1, err)
	}
	runID, _ := first["run_id"].(string)
	if strings.TrimSpace(runID) == "" {
		t.Fatalf("expected run_id in first result, got %v", first["run_id"])
	}

	if err := os.WriteFile(flagPath, []byte("ok"), 0o600); err != nil {
		t.Fatalf("write resume flag: %v", err)
	}

	root2 := RootCommand("1.2.3")
	root2.FlagSet.SetOutput(io.Discard)

	stdout2, _ := captureOutput(t, func() {
		if err := root2.Parse([]string{"workflow", "run", "--file", path, "release", "--resume", runID}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root2.Run(context.Background()); err != nil {
			t.Fatalf("resume run error: %v", err)
		}
	})

	var resumed map[string]any
	if err := json.Unmarshal([]byte(stdout2), &resumed); err != nil {
		t.Fatalf("expected JSON stdout, got %q: %v", stdout2, err)
	}
	if resumed["status"] != "ok" {
		t.Fatalf("expected resumed status=ok, got %v", resumed["status"])
	}
	if resumed["resumed"] != true {
		t.Fatalf("expected resumed=true, got %v", resumed["resumed"])
	}
	steps, ok := resumed["steps"].([]any)
	if !ok || len(steps) < 1 {
		t.Fatalf("expected steps array, got %T: %v", resumed["steps"], resumed["steps"])
	}
	firstStep, ok := steps[0].(map[string]any)
	if !ok {
		t.Fatalf("expected step object, got %T: %v", steps[0], steps[0])
	}
	if firstStep["status"] != "resumed" {
		t.Fatalf("expected first step status resumed, got %v", firstStep["status"])
	}
}

func TestWorkflowRun_StepFailure_PartialJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"test": {
				"steps": [
					"echo step-one-ok",
					{"run": "exit 1", "name": "failing-step"},
					"echo should-not-run"
				]
			}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--file", path, "test"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error on step failure")
		}
		// Should be a ReportedError (exit code 1, not 2)
		if _, ok := errors.AsType[ReportedError](err); !ok {
			t.Fatalf("expected ReportedError, got %T: %v", err, err)
		}
	})

	// Partial JSON result should be printed even on failure
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected JSON on failure, got %q: %v", stdout, err)
	}
	if result["status"] != "error" {
		t.Fatalf("expected status=error, got %v", result["status"])
	}
	if errMsg, ok := result["error"].(string); !ok || errMsg == "" {
		t.Fatalf("expected top-level error message in JSON, got %v", result["error"])
	}

	// Check partial steps: step 1 ok, step 2 error, step 3 not reached
	steps, ok := result["steps"].([]any)
	if !ok {
		t.Fatalf("expected steps array, got %T: %v", result["steps"], result["steps"])
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 partial steps, got %d", len(steps))
	}
	step1, ok := steps[0].(map[string]any)
	if !ok {
		t.Fatalf("expected steps[0] object, got %T: %v", steps[0], steps[0])
	}
	if step1["status"] != "ok" {
		t.Fatalf("expected step 1 ok, got %v", step1["status"])
	}
	step2, ok := steps[1].(map[string]any)
	if !ok {
		t.Fatalf("expected steps[1] object, got %T: %v", steps[1], steps[1])
	}
	if step2["status"] != "error" {
		t.Fatalf("expected step 2 error, got %v", step2["status"])
	}
	if step2["name"] != "failing-step" {
		t.Fatalf("expected step 2 name=failing-step, got %v", step2["name"])
	}
	// Error detail should be present
	if step2["error"] == nil || step2["error"] == "" {
		t.Fatal("expected error detail in failing step")
	}
}

func TestWorkflowRun_BeforeAllHookFailure_PrintsErrorInJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"before_all": "exit 1",
		"error": "echo error_hook_fired",
		"workflows": {
			"test": {"steps": ["echo should-not-run"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--file", path, "test"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error on before_all failure")
		}
		if _, ok := errors.AsType[ReportedError](err); !ok {
			t.Fatalf("expected ReportedError, got %T: %v", err, err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected JSON on failure, got %q: %v", stdout, err)
	}
	if result["status"] != "error" {
		t.Fatalf("expected status=error, got %v", result["status"])
	}
	if errMsg, ok := result["error"].(string); !ok || !strings.Contains(errMsg, "before_all") {
		t.Fatalf("expected JSON error message to mention before_all, got %v", result["error"])
	}

	steps, ok := result["steps"].([]any)
	if !ok {
		t.Fatalf("expected steps array, got %T: %v", result["steps"], result["steps"])
	}
	if len(steps) != 0 {
		t.Fatalf("expected 0 steps, got %d", len(steps))
	}

	hooks, ok := result["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("expected hooks object in JSON, got %T: %v", result["hooks"], result["hooks"])
	}
	beforeAll, ok := hooks["before_all"].(map[string]any)
	if !ok {
		t.Fatalf("expected hooks.before_all object, got %T: %v", hooks["before_all"], hooks["before_all"])
	}
	if beforeAll["status"] != "error" {
		t.Fatalf("expected hooks.before_all.status=error, got %v", beforeAll["status"])
	}
	if beforeAll["error"] == nil || beforeAll["error"] == "" {
		t.Fatalf("expected hooks.before_all.error to be present, got %v", beforeAll["error"])
	}

	// Hooks and steps stream output to stderr (stdout is JSON-only).
	if strings.Contains(stderr, "should-not-run") {
		t.Fatalf("expected step output to not run, got stderr %q", stderr)
	}
	if !strings.Contains(stderr, "error_hook_fired") {
		t.Fatalf("expected error hook output on stderr, got %q", stderr)
	}
}

func TestWorkflowRun_AfterAllHookFailure_PrintsErrorInJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"after_all": "exit 1",
		"error": "echo error_hook_fired",
		"workflows": {
			"test": {"steps": ["echo step_ok"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--file", path, "test"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error on after_all failure")
		}
		if _, ok := errors.AsType[ReportedError](err); !ok {
			t.Fatalf("expected ReportedError, got %T: %v", err, err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected JSON on failure, got %q: %v", stdout, err)
	}
	if result["status"] != "error" {
		t.Fatalf("expected status=error, got %v", result["status"])
	}
	if errMsg, ok := result["error"].(string); !ok || !strings.Contains(errMsg, "after_all") {
		t.Fatalf("expected JSON error message to mention after_all, got %v", result["error"])
	}

	steps, ok := result["steps"].([]any)
	if !ok {
		t.Fatalf("expected steps array, got %T: %v", result["steps"], result["steps"])
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	step1, ok := steps[0].(map[string]any)
	if !ok {
		t.Fatalf("expected steps[0] object, got %T: %v", steps[0], steps[0])
	}
	if step1["status"] != "ok" {
		t.Fatalf("expected step status ok, got %v", step1["status"])
	}

	hooks, ok := result["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("expected hooks object in JSON, got %T: %v", result["hooks"], result["hooks"])
	}
	afterAll, ok := hooks["after_all"].(map[string]any)
	if !ok {
		t.Fatalf("expected hooks.after_all object, got %T: %v", hooks["after_all"], hooks["after_all"])
	}
	if afterAll["status"] != "error" {
		t.Fatalf("expected hooks.after_all.status=error, got %v", afterAll["status"])
	}
	if afterAll["error"] == nil || afterAll["error"] == "" {
		t.Fatalf("expected hooks.after_all.error to be present, got %v", afterAll["error"])
	}

	if !strings.Contains(stderr, "step_ok") {
		t.Fatalf("expected step output on stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "error_hook_fired") {
		t.Fatalf("expected error hook output on stderr, got %q", stderr)
	}
}

func TestWorkflowRun_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	ascDir := filepath.Join(dir, ".asc")
	if err := os.MkdirAll(ascDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	badPath := filepath.Join(ascDir, "workflow.json")
	if err := os.WriteFile(badPath, []byte(`{not valid json`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--file", badPath, "test"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
		if !strings.Contains(err.Error(), "parse workflow JSON") {
			t.Fatalf("expected parse error, got %v", err)
		}
	})
}

func TestWorkflowValidate_MultipleErrors(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"bad1": {"steps": []},
			"bad2": {"steps": [{"name": "orphan"}]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "validate", "--file", path}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error for invalid workflows")
		}
		if _, ok := errors.AsType[ReportedError](err); !ok {
			t.Fatalf("expected ReportedError, got %T: %v", err, err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected JSON, got %q: %v", stdout, err)
	}
	if result["valid"] != false {
		t.Fatalf("expected valid=false, got %v", result["valid"])
	}
	errs, ok := result["errors"].([]any)
	if !ok {
		t.Fatalf("expected errors array, got %T: %v", result["errors"], result["errors"])
	}
	if len(errs) < 2 {
		t.Fatalf("expected at least 2 validation errors, got %d: %v", len(errs), errs)
	}
	// Each error should have code, workflow, and message
	for i, e := range errs {
		errMap, ok := e.(map[string]any)
		if !ok {
			t.Fatalf("expected errors[%d] object, got %T: %v", i, e, e)
		}
		if errMap["code"] == nil || errMap["code"] == "" {
			t.Fatalf("error %d missing code: %v", i, errMap)
		}
		if errMap["message"] == nil || errMap["message"] == "" {
			t.Fatalf("error %d missing message: %v", i, errMap)
		}
	}
}

func TestWorkflowValidate_Pretty(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"beta": {"steps": ["echo hello"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "validate", "--output", "json", "--pretty", "--file", path}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stdout, "  ") {
		t.Fatalf("expected indented JSON, got %q", stdout)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected valid JSON, got %q: %v", stdout, err)
	}
	if result["valid"] != true {
		t.Fatalf("expected valid=true, got %v", result["valid"])
	}
}
