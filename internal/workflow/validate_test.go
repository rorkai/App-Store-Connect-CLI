package workflow

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeWorkflowFile(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "workflow.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write workflow file: %v", err)
	}
	return path
}

func assertValidationCode(t *testing.T, errs []*ValidationError, code ValidationCode) {
	t.Helper()
	for _, e := range errs {
		if e.Code == code {
			return
		}
	}
	t.Fatalf("expected validation code %q, got %v", code, errs)
}

func TestLoad_Valid(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowFile(t, dir, `{
		"env": {"APP_ID": "com.example"},
		"workflows": {
			"beta": {
				"description": "Beta build",
				"steps": ["echo hello"]
			}
		}
	}`)

	def, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if def.Env["APP_ID"] != "com.example" {
		t.Fatalf("expected APP_ID=com.example, got %q", def.Env["APP_ID"])
	}
	if len(def.Workflows) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(def.Workflows))
	}
	wf := def.Workflows["beta"]
	if wf.Description != "Beta build" {
		t.Fatalf("expected description 'Beta build', got %q", wf.Description)
	}
	if len(wf.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(wf.Steps))
	}
	if wf.Steps[0].Run != "echo hello" {
		t.Fatalf("expected run 'echo hello', got %q", wf.Steps[0].Run)
	}
}

func TestLoad_Valid_WithJSONCComments(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowFile(t, dir, `{
		// Comments are allowed in workflow files.
		"workflows": {
			"beta": {
				/* block comments too */
				"steps": ["echo hello"] // inline comment
			}
		}
	}`)

	_, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
}

func TestLoad_StrictUnknownRootField(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowFile(t, dir, `{
		"unknown": 1,
		"workflows": {
			"beta": {"steps": ["echo hello"]}
		}
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unknown root field")
	}
	if !errors.Is(err, ErrWorkflowParseJSON) {
		t.Fatalf("expected ErrWorkflowParseJSON, got %v", err)
	}
}

func TestLoad_StrictUnknownWorkflowField(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowFile(t, dir, `{
		"workflows": {
			"beta": {"steps": ["echo hello"], "unknown": true}
		}
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unknown workflow field")
	}
	if !errors.Is(err, ErrWorkflowParseJSON) {
		t.Fatalf("expected ErrWorkflowParseJSON, got %v", err)
	}
}

func TestLoad_StrictUnknownStepField(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowFile(t, dir, `{
		"workflows": {
			"beta": {
				"steps": [{"run": "echo hello", "unknown": true}]
			}
		}
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unknown step field")
	}
	if !errors.Is(err, ErrWorkflowParseJSON) {
		t.Fatalf("expected ErrWorkflowParseJSON, got %v", err)
	}
}

func TestLoad_InvalidStepType(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowFile(t, dir, `{
		"workflows": {
			"test": {
				"steps": [123]
			}
		}
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid step type (number)")
	}
	if !errors.Is(err, ErrWorkflowParseJSON) {
		t.Fatalf("expected ErrWorkflowParseJSON, got %v", err)
	}
}

func TestLoad_BareStringStep(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowFile(t, dir, `{
		"workflows": {
			"test": {
				"steps": ["echo hello", {"run": "echo world", "name": "world"}]
			}
		}
	}`)

	def, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	steps := def.Workflows["test"].Steps
	if steps[0].Run != "echo hello" {
		t.Fatalf("bare string: expected 'echo hello', got %q", steps[0].Run)
	}
	if steps[1].Run != "echo world" || steps[1].Name != "world" {
		t.Fatalf("object step: expected run='echo world' name='world', got run=%q name=%q", steps[1].Run, steps[1].Name)
	}
}

func TestLoad_PrivateWorkflow(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowFile(t, dir, `{
		"workflows": {
			"helper": {
				"private": true,
				"steps": ["echo helper"]
			}
		}
	}`)

	def, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !def.Workflows["helper"].Private {
		t.Fatal("expected private=true")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/workflow.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !errors.Is(err, ErrWorkflowRead) {
		t.Fatalf("expected ErrWorkflowRead, got %v", err)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowFile(t, dir, `{invalid json}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !errors.Is(err, ErrWorkflowParseJSON) {
		t.Fatalf("expected ErrWorkflowParseJSON, got %v", err)
	}
}

func TestValidate_NoWorkflows_EmptyMap(t *testing.T) {
	def := &Definition{Workflows: map[string]Workflow{}}
	errs := Validate(def)
	assertValidationCode(t, errs, ErrNoWorkflows)
}

func TestValidate_NoWorkflows_Nil(t *testing.T) {
	def := &Definition{}
	errs := Validate(def)
	assertValidationCode(t, errs, ErrNoWorkflows)
}

func TestValidate_NoWorkflows_NullJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowFile(t, dir, `{"workflows": null}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for null workflows")
	}
}

func TestValidate_InvalidWorkflowName(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"123-bad"},
		{"has spaces"},
		{"-starts-with-dash"},
		{"_underscore_start"},
		{""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			def := &Definition{
				Workflows: map[string]Workflow{
					test.name: {Steps: []Step{{Run: "echo hi"}}},
				},
			}
			errs := Validate(def)
			assertValidationCode(t, errs, ErrInvalidWorkflowName)
		})
	}
}

func TestValidate_StepOutputsRequireNamedRunStep(t *testing.T) {
	def := &Definition{
		Workflows: map[string]Workflow{
			"release": {
				Steps: []Step{
					{
						Run:     `printf '{"buildId":"build-42"}'`,
						Outputs: map[string]string{"BUILD_ID": "$.buildId"},
					},
				},
			},
		},
	}

	errs := Validate(def)
	assertValidationCode(t, errs, ErrStepOutputsRequireName)
}

func TestValidate_StepOutputsRejectWorkflowStep(t *testing.T) {
	def := &Definition{
		Workflows: map[string]Workflow{
			"main": {
				Steps: []Step{
					{
						Name:     "helper_call",
						Workflow: "helper",
						Outputs:  map[string]string{"BUILD_ID": "$.buildId"},
					},
				},
			},
			"helper": {Steps: []Step{{Run: "echo helper"}}},
		},
	}

	errs := Validate(def)
	assertValidationCode(t, errs, ErrStepOutputsOnWorkflow)
}

func TestValidate_StepOutputsRequireUniqueProducerNames(t *testing.T) {
	def := &Definition{
		Workflows: map[string]Workflow{
			"main": {
				Steps: []Step{
					{
						Name:    "upload",
						Run:     `printf '{"buildId":"one"}'`,
						Outputs: map[string]string{"BUILD_ID": "$.buildId"},
					},
				},
			},
			"helper": {
				Steps: []Step{
					{
						Name:    "upload",
						Run:     `printf '{"buildId":"two"}'`,
						Outputs: map[string]string{"BUILD_ID": "$.buildId"},
					},
				},
			},
		},
	}

	errs := Validate(def)
	assertValidationCode(t, errs, ErrDuplicateOutputProducerName)
}

func TestValidate_StepOutputsRejectInvalidOutputExpr(t *testing.T) {
	def := &Definition{
		Workflows: map[string]Workflow{
			"release": {
				Steps: []Step{
					{
						Name:    "upload",
						Run:     `printf '{"buildId":"build-42"}'`,
						Outputs: map[string]string{"BUILD_ID": "buildId"},
					},
				},
			},
		},
	}

	errs := Validate(def)
	assertValidationCode(t, errs, ErrInvalidOutputExpr)
}

func TestValidate_ValidWorkflowNames(t *testing.T) {
	tests := []string{"beta", "release", "my-workflow", "test_123", "A"}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			def := &Definition{
				Workflows: map[string]Workflow{
					name: {Steps: []Step{{Run: "echo hi"}}},
				},
			}
			errs := Validate(def)
			for _, e := range errs {
				if e.Code == ErrInvalidWorkflowName {
					t.Fatalf("unexpected invalid name error for %q", name)
				}
			}
		})
	}
}

func TestValidate_EmptySteps(t *testing.T) {
	def := &Definition{
		Workflows: map[string]Workflow{
			"beta": {Steps: []Step{}},
		},
	}
	errs := Validate(def)
	assertValidationCode(t, errs, ErrEmptySteps)
}

func TestValidate_StepNoAction(t *testing.T) {
	def := &Definition{
		Workflows: map[string]Workflow{
			"beta": {Steps: []Step{{Name: "orphan"}}},
		},
	}
	errs := Validate(def)
	assertValidationCode(t, errs, ErrStepNoAction)
}

func TestValidate_StepEmptyRun(t *testing.T) {
	def := &Definition{
		Workflows: map[string]Workflow{
			"beta": {Steps: []Step{{Run: "  "}}},
		},
	}
	errs := Validate(def)
	assertValidationCode(t, errs, ErrStepEmptyRun)
}

func TestValidate_StepConflict(t *testing.T) {
	def := &Definition{
		Workflows: map[string]Workflow{
			"beta":  {Steps: []Step{{Run: "echo hi", Workflow: "other"}}},
			"other": {Steps: []Step{{Run: "echo"}}},
		},
	}
	errs := Validate(def)
	assertValidationCode(t, errs, ErrStepConflict)
}

func TestValidate_StepWithOnRunStep(t *testing.T) {
	def := &Definition{
		Workflows: map[string]Workflow{
			"beta": {Steps: []Step{{Run: "echo $MSG", With: map[string]string{"MSG": "hello"}}}},
		},
	}
	errs := Validate(def)
	assertValidationCode(t, errs, ErrStepWithOnRun)
}

func TestValidate_WorkflowNotFound(t *testing.T) {
	def := &Definition{
		Workflows: map[string]Workflow{
			"beta": {Steps: []Step{{Workflow: "nonexistent"}}},
		},
	}
	errs := Validate(def)
	assertValidationCode(t, errs, ErrWorkflowNotFound)
}

func TestValidate_DirectCycle(t *testing.T) {
	def := &Definition{
		Workflows: map[string]Workflow{
			"self": {Steps: []Step{{Workflow: "self"}}},
		},
	}
	errs := Validate(def)
	assertValidationCode(t, errs, ErrCyclicReference)
}

func TestValidate_SimpleCycle(t *testing.T) {
	def := &Definition{
		Workflows: map[string]Workflow{
			"a": {Steps: []Step{{Workflow: "b"}}},
			"b": {Steps: []Step{{Workflow: "a"}}},
		},
	}
	errs := Validate(def)
	assertValidationCode(t, errs, ErrCyclicReference)
}

func TestValidate_DeepCycle(t *testing.T) {
	def := &Definition{
		Workflows: map[string]Workflow{
			"a": {Steps: []Step{{Workflow: "b"}}},
			"b": {Steps: []Step{{Workflow: "c"}}},
			"c": {Steps: []Step{{Workflow: "a"}}},
		},
	}
	errs := Validate(def)
	assertValidationCode(t, errs, ErrCyclicReference)
}

func TestValidate_NoCycleWithSharedDependency(t *testing.T) {
	def := &Definition{
		Workflows: map[string]Workflow{
			"a":      {Steps: []Step{{Workflow: "shared"}}},
			"b":      {Steps: []Step{{Workflow: "shared"}}},
			"shared": {Steps: []Step{{Run: "echo shared"}}},
		},
	}
	errs := Validate(def)
	for _, e := range errs {
		if e.Code == ErrCyclicReference {
			t.Fatalf("unexpected cycle error: %s", e.Message)
		}
	}
}

func TestValidate_CollectsMultipleErrors(t *testing.T) {
	def := &Definition{
		Workflows: map[string]Workflow{
			"beta":  {Steps: []Step{}},
			"alpha": {Steps: []Step{{Name: "no-action"}}},
		},
	}
	errs := Validate(def)
	if len(errs) < 2 {
		t.Fatalf("expected at least 2 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidate_HooksField(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowFile(t, dir, `{
		"before_all": "echo before",
		"after_all": "echo after",
		"error": "echo error",
		"workflows": {
			"test": {"steps": ["echo hello"]}
		}
	}`)

	def, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if def.BeforeAll != "echo before" {
		t.Fatalf("expected before_all='echo before', got %q", def.BeforeAll)
	}
	if def.AfterAll != "echo after" {
		t.Fatalf("expected after_all='echo after', got %q", def.AfterAll)
	}
	if def.Error != "echo error" {
		t.Fatalf("expected error='echo error', got %q", def.Error)
	}
}

func TestValidate_WorkflowStepWithAndIf(t *testing.T) {
	def := &Definition{
		Workflows: map[string]Workflow{
			"main": {Steps: []Step{
				{Workflow: "helper", If: "DO_IT", With: map[string]string{"MSG": "hi"}},
			}},
			"helper": {Steps: []Step{{Run: "echo $MSG"}}},
		},
	}
	errs := Validate(def)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestLoad_NullStepElement(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowFile(t, dir, `{
		"workflows": {
			"test": {"steps": [null]}
		}
	}`)

	_, err := LoadUnvalidated(path)
	if err == nil {
		t.Fatal("expected error for null step element")
	}
}

func TestStep_UnmarshalJSON_StringClearsFields(t *testing.T) {
	// Start with a pre-populated Step
	s := Step{
		Name:     "old-name",
		If:       "OLD_IF",
		Workflow: "old-wf",
		With:     map[string]string{"A": "1"},
	}
	if err := json.Unmarshal([]byte(`"echo new"`), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.Run != "echo new" {
		t.Fatalf("expected Run='echo new', got %q", s.Run)
	}
	if s.Name != "" {
		t.Fatalf("expected Name cleared, got %q", s.Name)
	}
	if s.If != "" {
		t.Fatalf("expected If cleared, got %q", s.If)
	}
	if s.Workflow != "" {
		t.Fatalf("expected Workflow cleared, got %q", s.Workflow)
	}
	if s.With != nil {
		t.Fatalf("expected With cleared, got %v", s.With)
	}
}

func TestValidationError_ErrorInterface(t *testing.T) {
	ve := &ValidationError{
		Code:    ErrEmptySteps,
		Message: "workflow has no steps",
	}
	var err error = ve
	if err.Error() != "workflow has no steps" {
		t.Fatalf("expected Error() to return message, got %q", err.Error())
	}
}

func TestValidationError_JSON(t *testing.T) {
	ve := &ValidationError{
		Code:     ErrStepNoAction,
		Workflow: "beta",
		Step:     2,
		Message:  "workflow \"beta\" step 2 must have run or workflow",
	}
	data, err := json.Marshal(ve)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["code"] != "step_no_action" {
		t.Fatalf("expected code=step_no_action, got %v", out["code"])
	}
	if out["workflow"] != "beta" {
		t.Fatalf("expected workflow=beta, got %v", out["workflow"])
	}
}

func TestLoad_ReturnsAllValidationErrors(t *testing.T) {
	dir := t.TempDir()
	// Two workflows, each with a different validation error.
	path := writeWorkflowFile(t, dir, `{
		"workflows": {
			"bad1": {"steps": []},
			"bad2": {"steps": [{"name": "orphan"}]}
		}
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error from Load")
	}

	// The joined error should contain messages from both workflows.
	msg := err.Error()
	if !strings.Contains(msg, "bad1") || !strings.Contains(msg, "bad2") {
		t.Fatalf("expected error to mention both bad1 and bad2, got %q", msg)
	}

	// Each individual ValidationError should be extractable via errors.As.
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected errors.As to find ValidationError, got %T: %v", err, err)
	}
}
