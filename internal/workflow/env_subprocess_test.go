package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
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
