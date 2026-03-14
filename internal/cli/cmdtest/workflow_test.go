package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestWorkflow_ShowsHelp(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "workflow") {
		t.Fatalf("expected help to mention 'workflow', got %q", stderr)
	}
	if !strings.Contains(stderr, "The file supports JSONC comments") {
		t.Fatalf("expected help to include embedded workflow file tips, got %q", stderr)
	}
	if !strings.Contains(stderr, "${steps.resolve_build.BUILD_ID}") {
		t.Fatalf("expected help to document step output references, got %q", stderr)
	}
	if !strings.Contains(stderr, "WORKFLOWS.md") {
		t.Fatalf("expected help to link workflow docs, got %q", stderr)
	}
}

func TestWorkflowRun_MissingName(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "workflow name is required") {
		t.Fatalf("expected 'workflow name is required' in stderr, got %q", stderr)
	}
}

func TestWorkflowRun_MissingFile(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--file", "/nonexistent/workflow.json", "beta"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}

func TestWorkflowRun_InvalidParam(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"beta": {"steps": ["echo hello"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--file", path, "beta", "NOCOLON"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp for invalid param, got %v", err)
		}
	})

	if !strings.Contains(stderr, "NOCOLON") {
		t.Fatalf("expected error mentioning 'NOCOLON', got %q", stderr)
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
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"beta": {"steps": ["echo hello world"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--dry-run", "--file", path, "beta"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "[dry-run]") {
		t.Fatalf("expected '[dry-run]' in stderr, got %q", stderr)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected JSON stdout, got %q: %v", stdout, err)
	}
	if result["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", result["status"])
	}
}

func TestWorkflowRun_DryRunFlagAfterName(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"beta": {"steps": ["echo hello world"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--file", path, "beta", "--dry-run"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "[dry-run]") {
		t.Fatalf("expected '[dry-run]' in stderr, got %q", stderr)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("expected JSON stdout, got %q: %v", stdout, err)
	}
	if result["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", result["status"])
	}
}

func TestWorkflowRun_DryRun_AllowsInterpolatedWithValues(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"main": {
				"steps": [
					{
						"name": "upload",
						"run": "printf '{\"buildId\":\"build-42\"}'",
						"outputs": {
							"BUILD_ID": "$.buildId"
						}
					},
					{
						"workflow": "distribute",
						"with": {
							"BUILD_ID": "${steps.upload.BUILD_ID}"
						}
					}
				]
			},
			"distribute": {
				"steps": [
					{
						"run": "echo $BUILD_ID"
					}
				]
			}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--dry-run", "--file", path, "main"}); err != nil {
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
	if !strings.Contains(stderr, "[dry-run] step 1: printf") {
		t.Fatalf("expected first dry-run preview, got %q", stderr)
	}
	if !strings.Contains(stderr, "[dry-run] step 2: workflow distribute") {
		t.Fatalf("expected workflow dry-run preview, got %q", stderr)
	}
	if !strings.Contains(stderr, "[dry-run] step 1: echo $BUILD_ID") {
		t.Fatalf("expected child dry-run preview to stay raw, got %q", stderr)
	}
}

func TestWorkflowRun_UnknownFlagAfterName(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"beta": {"steps": ["echo hello"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--file", path, "beta", "--unknown"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "unknown flag") {
		t.Fatalf("expected unknown flag error, got %q", stderr)
	}
}

func TestWorkflowRun_DryRunFlagAfterName_InvalidValue(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"beta": {"steps": ["echo hello"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--file", path, "beta", "--dry-run=maybe"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "invalid value") {
		t.Fatalf("expected invalid value error, got %q", stderr)
	}
}

func TestWorkflowRun_FileFlagAfterName_MissingValue(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"beta": {"steps": ["echo hello"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--file", path, "beta", "--file", "--dry-run"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--file requires a value") {
		t.Fatalf("expected missing file value error, got %q", stderr)
	}
}

func TestWorkflowRun_FileFlagAfterName_UnknownLongFlagIsMissingValue(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"beta": {"steps": ["echo hello"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--file", path, "beta", "--file", "--unknown"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--file requires a value") {
		t.Fatalf("expected missing file value error, got %q", stderr)
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

func TestWorkflowRun_Valid_WithJSONCComments(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		// Comments are allowed in workflow files.
		"workflows": {
			"beta": {"steps": ["echo hello world"]} // inline comment
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--file", path, "beta"}); err != nil {
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

func TestWorkflowValidate_MissingFile(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "validate", "--file", "/nonexistent/workflow.json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}

func TestWorkflowValidate_UnexpectedArgs(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"beta": {"steps": ["echo hello"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "validate", "--file", path, "extra"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "unexpected argument") {
		t.Fatalf("expected unexpected argument error, got %q", stderr)
	}
}

func TestWorkflowList_SingleWorkflow(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"beta": {"steps": ["echo hi"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "list", "--file", path}); err != nil {
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
	if len(workflows) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(workflows))
	}
	if workflows[0]["name"] != "beta" {
		t.Fatalf("expected name=beta, got %v", workflows[0]["name"])
	}
}

func TestWorkflowList_Sorted(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"release": {"steps": ["echo r"]},
			"alpha": {"steps": ["echo a"]},
			"beta": {"steps": ["echo b"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "list", "--file", path}); err != nil {
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
	if len(workflows) != 3 {
		t.Fatalf("expected 3 workflows, got %d", len(workflows))
	}
	if workflows[0]["name"] != "alpha" {
		t.Fatalf("expected first=alpha, got %v", workflows[0]["name"])
	}
	if workflows[1]["name"] != "beta" {
		t.Fatalf("expected second=beta, got %v", workflows[1]["name"])
	}
	if workflows[2]["name"] != "release" {
		t.Fatalf("expected third=release, got %v", workflows[2]["name"])
	}
}

func TestWorkflowList_HidesPrivate(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"beta":   {"steps": ["echo beta"]},
			"helper": {"private": true, "steps": ["echo helper"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "list", "--file", path}); err != nil {
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
	if len(workflows) != 1 {
		t.Fatalf("expected 1 public workflow, got %d: %v", len(workflows), workflows)
	}
	if workflows[0]["name"] != "beta" {
		t.Fatalf("expected name=beta, got %v", workflows[0]["name"])
	}
}

func TestWorkflowList_AllIncludesPrivate(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"beta":   {"steps": ["echo beta"]},
			"helper": {"private": true, "steps": ["echo helper"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "list", "--all", "--file", path}); err != nil {
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
	if len(workflows) != 2 {
		t.Fatalf("expected 2 workflows with --all, got %d: %v", len(workflows), workflows)
	}
	// Should be sorted: beta, helper
	if workflows[0]["name"] != "beta" {
		t.Fatalf("expected first=beta, got %v", workflows[0]["name"])
	}
	if workflows[1]["name"] != "helper" {
		t.Fatalf("expected second=helper, got %v", workflows[1]["name"])
	}
}

func TestWorkflowList_MissingFile(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "list", "--file", "/nonexistent/workflow.json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}

func TestWorkflowList_UnexpectedArgs(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"beta": {"steps": ["echo hi"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "list", "--file", path, "extra"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "unexpected argument") {
		t.Fatalf("expected unexpected argument error, got %q", stderr)
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
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"test": {"steps": ["echo $MSG"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--file", path, "test", "MSG:hello"}); err != nil {
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
}

func TestWorkflowRun_WithParamsEqualsSeparator(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"test": {"steps": ["echo RESULT_IS_$TEST"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--file", path, "test", "TEST=yes"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	// Verify the param reached the command
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
}

func TestWorkflowRun_ParamControlsConditional(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"test": {
				"steps": [
					{"run": "echo CONDITIONAL_RAN", "if": "DO_IT"},
					"echo done"
				]
			}
		}
	}`)

	// Without the param — conditional step skipped
	root1 := RootCommand("1.2.3")
	root1.FlagSet.SetOutput(io.Discard)

	stdout1, _ := captureOutput(t, func() {
		if err := root1.Parse([]string{"workflow", "run", "--file", path, "test"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root1.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	var result1 map[string]any
	if err := json.Unmarshal([]byte(stdout1), &result1); err != nil {
		t.Fatalf("expected JSON, got %q: %v", stdout1, err)
	}
	steps1, ok := result1["steps"].([]any)
	if !ok {
		t.Fatalf("expected steps array, got %T: %v", result1["steps"], result1["steps"])
	}
	if len(steps1) < 1 {
		t.Fatalf("expected at least 1 step, got %d", len(steps1))
	}
	firstStep1, ok := steps1[0].(map[string]any)
	if !ok {
		t.Fatalf("expected steps[0] object, got %T: %v", steps1[0], steps1[0])
	}
	if firstStep1["status"] != "skipped" {
		t.Fatalf("expected conditional step skipped without param, got %v", firstStep1["status"])
	}

	// With DO_IT:true — conditional step runs
	root2 := RootCommand("1.2.3")
	root2.FlagSet.SetOutput(io.Discard)

	stdout2, _ := captureOutput(t, func() {
		if err := root2.Parse([]string{"workflow", "run", "--file", path, "test", "DO_IT:true"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root2.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	var result2 map[string]any
	if err := json.Unmarshal([]byte(stdout2), &result2); err != nil {
		t.Fatalf("expected JSON, got %q: %v", stdout2, err)
	}
	steps2, ok := result2["steps"].([]any)
	if !ok {
		t.Fatalf("expected steps array, got %T: %v", result2["steps"], result2["steps"])
	}
	if len(steps2) < 1 {
		t.Fatalf("expected at least 1 step, got %d", len(steps2))
	}
	firstStep2, ok := steps2[0].(map[string]any)
	if !ok {
		t.Fatalf("expected steps[0] object, got %T: %v", steps2[0], steps2[0])
	}
	if firstStep2["status"] != "ok" {
		t.Fatalf("expected conditional step ok with DO_IT:true, got %v", firstStep2["status"])
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

func TestWorkflowRun_ResumeReusesOriginalParams(t *testing.T) {
	dir := t.TempDir()
	allowPath := filepath.Join(dir, "allow")
	path := writeWorkflowJSON(t, dir, fmt.Sprintf(`{
		"workflows": {
			"release": {
				"steps": [
					{
						"name": "prepare",
						"run": "echo prepared"
					},
					{
						"name": "deploy",
						"run": "if [ -f '%s' ] && [ \"$TARGET\" = 'prod' ]; then echo deployed; else echo blocked >&2; exit 17; fi"
					}
				]
			}
		}
	}`, allowPath))

	root1 := RootCommand("1.2.3")
	root1.FlagSet.SetOutput(io.Discard)

	stdout1, _ := captureOutput(t, func() {
		if err := root1.Parse([]string{"workflow", "run", "--file", path, "release", "TARGET:prod"}); err != nil {
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
	resume, ok := first["resume"].(map[string]any)
	if !ok {
		t.Fatalf("expected resume info, got %T: %v", first["resume"], first["resume"])
	}
	if strings.TrimSpace(fmt.Sprint(resume["command"])) == "" {
		t.Fatalf("expected non-empty resume command, got %v", resume["command"])
	}

	if err := os.WriteFile(allowPath, []byte("ok"), 0o600); err != nil {
		t.Fatalf("write allow file: %v", err)
	}

	root2 := RootCommand("1.2.3")
	root2.FlagSet.SetOutput(io.Discard)

	stdout2, stderr2 := captureOutput(t, func() {
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
	if !strings.Contains(stderr2, "deployed") {
		t.Fatalf("expected resumed step to see original TARGET param, got stderr %q", stderr2)
	}
	steps, ok := resumed["steps"].([]any)
	if !ok || len(steps) != 2 {
		t.Fatalf("expected two steps, got %T: %v", resumed["steps"], resumed["steps"])
	}
	firstStep, ok := steps[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first step object, got %T: %v", steps[0], steps[0])
	}
	if firstStep["status"] != "resumed" {
		t.Fatalf("expected first step status resumed, got %v", firstStep["status"])
	}
}

func TestWorkflowRun_ResumeFlagAfterName_MissingValue(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"beta": {"steps": ["echo hello"]}
		}
	}`)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"workflow", "run", "--file", path, "beta", "--resume"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--resume requires a value") {
		t.Fatalf("expected missing resume value error, got %q", stderr)
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

func TestWorkflowValidate_CycleDetection(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowJSON(t, dir, `{
		"workflows": {
			"a": {"steps": [{"workflow": "b"}]},
			"b": {"steps": [{"workflow": "a"}]}
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
			t.Fatal("expected error for cyclic workflows")
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
	foundCycle := false
	for i, e := range errs {
		errMap, ok := e.(map[string]any)
		if !ok {
			t.Fatalf("expected errors[%d] object, got %T: %v", i, e, e)
		}
		if errMap["code"] == "cyclic_reference" {
			foundCycle = true
		}
	}
	if !foundCycle {
		t.Fatalf("expected cyclic_reference error, got %v", errs)
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
		if err := root.Parse([]string{"workflow", "validate", "--pretty", "--file", path}); err != nil {
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
