package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

func TestWorkflowEnvSubprocess(t *testing.T) {
	const mixed = "ASC_WORKFLOW_DemoName"
	const upper = "ASC_WORKFLOW_DEMONAME"
	const helper = "ASC_WORKFLOW_ENV_CHILD"
	if os.Getenv(helper) == "1" {
		_ = json.NewEncoder(os.Stdout).Encode([]string{os.Getenv(mixed), os.Getenv(upper), os.Getenv(readonly.EnvVar)})
		os.Exit(0)
	}
	t.Setenv(mixed, "inherited")
	t.Setenv(readonly.EnvVar, "1")
	merged := mergeEnv(map[string]string{upper: "global"}, map[string]string{mixed: "step", "asc_read_only": "0", helper: "1"})
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable, "-test.run=^TestWorkflowEnvSubprocess$")
	command.Env = buildEnvSlice(merged)
	data, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("child: %v: %s", err, data)
	}
	var values []string
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatalf("child response: %v: %s", err, data)
	}
	wantUpper := "global"
	if runtime.GOOS == "windows" {
		wantUpper = "step"
	}
	want := fmt.Sprintf("%q", []string{"step", wantUpper, "1"})
	if got := fmt.Sprintf("%q", values); got != want {
		t.Fatalf("child environment=%s, want %s", got, want)
	}
	allowed, err := evalStepIf(mixed+" == step", merged)
	if err != nil || !allowed {
		t.Fatalf("step condition=%v, error=%v", allowed, err)
	}
}

func TestWorkflowEnvCaseAmbiguity(t *testing.T) {
	for _, scope := range []string{"definition", "workflow", "step", "with"} {
		t.Run(scope, func(t *testing.T) {
			env := map[string]string{"DemoName": "first", "DEMONAME": "second"}
			step := Step{Run: "echo harmless"}
			wf := Workflow{Steps: []Step{step}}
			def := &Definition{Workflows: map[string]Workflow{"demo": wf}}
			switch scope {
			case "definition":
				def.Env = env
			case "workflow":
				wf.Env = env
			case "step":
				wf.Steps[0].Env = env
			case "with":
				wf.Steps[0] = Step{Workflow: "child", With: env}
				def.Workflows["child"] = Workflow{Steps: []Step{{Run: "echo harmless"}}}
			}
			def.Workflows["demo"] = wf
			errs := Validate(def)
			if runtime.GOOS != "windows" {
				if len(errs) != 0 {
					t.Fatalf("Unix separate names rejected: %v", errs)
				}
				return
			}
			if len(errs) != 1 || string(errs[0].Code) != "ambiguous_env" {
				t.Fatalf("ambiguous keys were not rejected: %v", errs)
			}
		})
	}
}

func TestWorkflowEnvOrderedInputs(t *testing.T) {
	inputs := []string{"DemoName=first", "DEMONAME=second", "DemoName=last"}
	params, err := ParseParams(inputs)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "variables.env")
	if err := os.WriteFile(path, []byte("DemoName=first\nDEMONAME=second\nDemoName=last\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := LoadEnvFiles([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	for name, env := range map[string]map[string]string{"parameters": params, "file": file} {
		if runtime.GOOS == "windows" {
			if len(env) != 1 || env["DemoName"] != "last" {
				t.Fatalf("%s did not keep last value: %v", name, env)
			}
		} else if len(env) != 2 || env["DemoName"] != "last" || env["DEMONAME"] != "second" {
			t.Fatalf("%s changed Unix separate names: %v", name, env)
		}
	}
}

func TestWorkflowEnvLegacyResume(t *testing.T) {
	for _, ambiguous := range []bool{false, true} {
		name := "single mixed-case key"
		if ambiguous {
			name = "ambiguous saved keys"
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("ASC_WORKFLOW_RESUME_ALLOWED", "0")
			def := &Definition{Workflows: map[string]Workflow{"demo": {Steps: []Step{{Run: "printf prepared"}, {Run: `printf resumed; test "$ASC_WORKFLOW_RESUME_ALLOWED" = 1 && test "$DemoName" = single`}}}}}
			dir := t.TempDir()
			opts := runOpts("demo")
			opts.WorkflowFile = filepath.Join(dir, "workflow.json")
			opts.StateDir = filepath.Join(dir, "runs")
			opts.Params = map[string]string{"DemoName": "single"}
			first, err := Run(context.Background(), def, opts)
			if err == nil || first == nil || !first.Recoverable {
				t.Fatalf("initial checkpoint: result=%+v error=%v", first, err)
			}
			state, err := loadRunState(first.RunFile)
			if err != nil {
				t.Fatal(err)
			}
			if ambiguous {
				state.Params["DEMONAME"] = "saved-value-must-stay-private"
				if err := saveRunState(first.RunFile, *state); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("ASC_WORKFLOW_RESUME_ALLOWED", "1")
			resumedOpts := runOpts("demo")
			resumedOpts.WorkflowFile, resumedOpts.StateDir, resumedOpts.ResumeRunID = opts.WorkflowFile, opts.StateDir, first.RunID
			if !ambiguous {
				resumedOpts.Params, err = ParseParams([]string{"DemoName=single"})
				if err != nil {
					t.Fatal(err)
				}
			}
			output := &bytes.Buffer{}
			resumedOpts.Stdout = output
			result, err := Run(context.Background(), def, resumedOpts)
			if ambiguous && runtime.GOOS == "windows" {
				if !errors.Is(err, ErrResumeRejected) || !strings.Contains(err.Error(), "ambiguous environment keys") || strings.Contains(err.Error(), "saved-value-must-stay-private") || output.Len() != 0 || result != nil && result.Resumed {
					t.Fatalf("resume was not refused before execution: result=%+v error=%v output=%q", result, err, output.String())
				}
				return
			}
			if err != nil || result == nil || !result.Resumed || result.Status != "ok" || output.String() != "resumed" {
				t.Fatalf("valid resume: result=%+v error=%v output=%q", result, err, output.String())
			}
		})
	}
}
