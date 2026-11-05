package workflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// docWorkflowPages are the pages that ship .asc/workflow.json examples.
// Keep this list in sync with the workflow documentation set.
var docWorkflowPages = []string{
	"website/src/content/docs/concepts/workflows.mdx",
	"website/src/content/docs/configuration/workflows.mdx",
	"website/src/content/docs/guides/automation.mdx",
	"docs/WORKFLOWS.md",
}

var (
	docFencePattern    = regexp.MustCompile("(?s)```(?:json|jsonc)[^\n]*\n(.*?)```")
	docIfFieldPattern  = regexp.MustCompile(`"if"\s*:\s*"([^"]*)"`)
	docIdentifierRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

type docJSONBlock struct {
	page string
	body string
}

func docJSONBlocks(t *testing.T) []docJSONBlock {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	var blocks []docJSONBlock
	for _, page := range docWorkflowPages {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(page)))
		if err != nil {
			t.Fatalf("read %s: %v", page, err)
		}
		for _, match := range docFencePattern.FindAllStringSubmatch(string(data), -1) {
			blocks = append(blocks, docJSONBlock{page: page, body: strings.TrimSpace(match[1])})
		}
	}
	if len(blocks) == 0 {
		t.Fatal("expected the workflow documentation to ship JSON examples")
	}
	return blocks
}

// isCompleteExample reports whether a documented block is a full workflow file
// rather than a schema fragment or a single-step snippet.
func isCompleteExample(body string) bool {
	return strings.HasPrefix(body, "{") &&
		strings.Contains(body, `"workflows"`) &&
		!strings.Contains(body, "...")
}

func TestDocWorkflowExamplesLoadAndValidate(t *testing.T) {
	complete := 0
	for _, block := range docJSONBlocks(t) {
		if !isCompleteExample(block.body) {
			continue
		}
		complete++

		path := filepath.Join(t.TempDir(), "workflow.json")
		if err := os.WriteFile(path, []byte(block.body), 0o600); err != nil {
			t.Fatalf("write example: %v", err)
		}
		if _, err := Load(path); err != nil {
			t.Errorf("%s: documented workflow example does not load and validate: %v\n%s", block.page, err, block.body)
		}
	}
	if complete == 0 {
		t.Fatal("expected the workflow documentation to ship at least one complete example")
	}
}

// TestDocWorkflowExamplesUseVariableNamesForIf pins the documented `if` contract
// to the engine: `if` names a variable, so a shell expression such as
// `test "$AUTO_SUBMIT" = "true"` is used verbatim as a variable name, never
// resolves, and silently skips the step.
func TestDocWorkflowExamplesUseVariableNamesForIf(t *testing.T) {
	for _, block := range docJSONBlocks(t) {
		for _, match := range docIfFieldPattern.FindAllStringSubmatch(block.body, -1) {
			if !docIdentifierRegex.MatchString(strings.TrimSpace(match[1])) {
				t.Errorf("%s: %s is not a variable name; `if` takes an env-var name, not an expression", block.page, match[0])
			}
		}
	}
}

// TestDocWorkflowExamplesDemonstrateTruthyGates guards the failure the audit
// found: a complete example gating a mutating step on a value that is never
// truthy (an ID, a path) runs green while doing nothing. Every gate in a
// complete example must be shown with a truthy value on the same page.
func TestDocWorkflowExamplesDemonstrateTruthyGates(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	gates := 0
	for _, page := range docWorkflowPages {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(page)))
		if err != nil {
			t.Fatalf("read %s: %v", page, err)
		}
		text := string(data)

		for _, match := range docFencePattern.FindAllStringSubmatch(text, -1) {
			body := strings.TrimSpace(match[1])
			if !isCompleteExample(body) {
				continue
			}
			for _, ifMatch := range docIfFieldPattern.FindAllStringSubmatch(body, -1) {
				name := strings.TrimSpace(ifMatch[1])
				if !docIdentifierRegex.MatchString(name) {
					continue // reported by TestDocWorkflowExamplesUseVariableNamesForIf
				}
				gates++
				if !pageShowsTruthyValue(text, name) {
					t.Errorf("%s: step gated on %q but the page never shows a truthy value for it; the step is skipped on every documented invocation", page, name)
				}
			}
		}
	}
	if gates == 0 {
		t.Fatal("expected at least one complete documented example to gate a step with `if`")
	}
}

// TestDocsDescribeAfterAllAsSuccessOnly keeps the pages aligned with Run: a step
// failure returns before the after_all block, so after_all never runs on the
// failure path (see TestRun_AfterAllDoesNotRunOnStepFailure). Documenting it as
// a "success or failure" hook sends cleanup into the one path it never covers.
func TestDocsDescribeAfterAllAsSuccessOnly(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	for _, page := range docWorkflowPages {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(page)))
		if err != nil {
			t.Fatalf("read %s: %v", page, err)
		}
		inAfterAllSection := false
		for i, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				inAfterAllSection = mentionsAfterAll(line)
			}
			if !inAfterAllSection && !mentionsAfterAll(line) {
				continue
			}
			if strings.Contains(strings.ToLower(line), "or failure") {
				t.Errorf("%s:%d claims after_all runs on failure, but the runner skips it on the failure path: %s", page, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// jsonFieldNames returns the JSON object keys a struct type can emit.
func jsonFieldNames(t *testing.T, value any) map[string]bool {
	t.Helper()

	typ := reflect.TypeOf(value)
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		t.Fatalf("jsonFieldNames: %T is not a struct", value)
	}

	names := make(map[string]bool, typ.NumField())
	for i := range typ.NumField() {
		tag := typ.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		names[strings.Split(tag, ",")[0]] = true
	}
	return names
}

// TestDocsRunResultSampleMatchesSchema fails when a documented run-result sample
// invents a field the CLI never emits. The audited page advertised per-step
// "exit_code", top-level "before_all"/"after_all", and "success", so a CI gate
// written from it (jq -e '.success') evaluated null and reported healthy runs as
// broken.
func TestDocsRunResultSampleMatchesSchema(t *testing.T) {
	resultFields := jsonFieldNames(t, RunResult{})
	stepFields := jsonFieldNames(t, StepResult{})
	hookFields := jsonFieldNames(t, HookResult{})
	hooksFields := jsonFieldNames(t, HooksResult{})

	samples := 0
	for _, block := range docJSONBlocks(t) {
		var sample map[string]any
		if err := json.Unmarshal([]byte(block.body), &sample); err != nil {
			continue // definition example, fragment, or non-object sample
		}
		if _, hasWorkflows := sample["workflows"]; hasWorkflows {
			continue
		}
		if _, hasWorkflow := sample["workflow"]; !hasWorkflow {
			continue
		}
		if _, hasSteps := sample["steps"]; !hasSteps {
			continue
		}
		samples++

		for key := range sample {
			if !resultFields[key] {
				t.Errorf("%s: documented run result has field %q, which RunResult never emits", block.page, key)
			}
		}
		steps, _ := sample["steps"].([]any)
		for _, entry := range steps {
			step, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			for key := range step {
				if !stepFields[key] {
					t.Errorf("%s: documented step has field %q, which StepResult never emits", block.page, key)
				}
			}
		}
		hooks, _ := sample["hooks"].(map[string]any)
		for name, entry := range hooks {
			if !hooksFields[name] {
				t.Errorf("%s: documented hooks has field %q, which HooksResult never emits", block.page, name)
			}
			hook, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			for key := range hook {
				if !hookFields[key] {
					t.Errorf("%s: documented hook has field %q, which HookResult never emits", block.page, key)
				}
			}
		}
	}
	if samples == 0 {
		t.Fatal("expected the workflow documentation to show a run-result sample")
	}
}

func mentionsAfterAll(line string) bool {
	return strings.Contains(line, "after_all") || strings.Contains(line, `after\_all`)
}

func pageShowsTruthyValue(page, name string) bool {
	for _, truthy := range []string{"1", "true", "yes", "y", "on"} {
		if strings.Contains(page, name+":"+truthy) || strings.Contains(page, name+"="+truthy) {
			return true
		}
	}
	return false
}
