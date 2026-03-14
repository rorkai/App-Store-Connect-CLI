package workflow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"
	"time"
)

// MaxCallDepth is the maximum nesting depth for sub-workflow calls.
const MaxCallDepth = 16

// RunOptions configures a workflow execution.
type RunOptions struct {
	WorkflowName string
	Params       map[string]string
	DryRun       bool
	Stdout       io.Writer
	Stderr       io.Writer
	WorkflowFile string
	StateDir     string
	ResumeRunID  string
}

// StepResult records one executed step.
type StepResult struct {
	Index          int               `json:"index"`
	Name           string            `json:"name,omitempty"`
	Command        string            `json:"command,omitempty"`
	Workflow       string            `json:"workflow,omitempty"`
	ParentWorkflow string            `json:"parent_workflow,omitempty"`
	Status         string            `json:"status"`
	DurationMS     int64             `json:"duration_ms"`
	Error          string            `json:"error,omitempty"`
	Outputs        map[string]string `json:"outputs,omitempty"`
}

// HookResult records execution of a hook command (before_all/after_all/error).
type HookResult struct {
	Command    string `json:"command,omitempty"`
	Status     string `json:"status"`
	DurationMS int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// HooksResult records hook outcomes for a run.
type HooksResult struct {
	BeforeAll *HookResult `json:"before_all,omitempty"`
	AfterAll  *HookResult `json:"after_all,omitempty"`
	Error     *HookResult `json:"error,omitempty"`
}

// ResumeInfo describes how to continue a partially completed workflow run.
type ResumeInfo struct {
	Command string `json:"command,omitempty"`
}

// RunResult is the structured output of a workflow execution.
type RunResult struct {
	Workflow    string                       `json:"workflow"`
	Status      string                       `json:"status"`
	Error       string                       `json:"error,omitempty"`
	FailedStep  string                       `json:"failed_step,omitempty"`
	Recoverable bool                         `json:"recoverable,omitempty"`
	RunID       string                       `json:"run_id,omitempty"`
	RunFile     string                       `json:"run_file,omitempty"`
	Resumed     bool                         `json:"resumed,omitempty"`
	Resume      *ResumeInfo                  `json:"resume,omitempty"`
	Outputs     map[string]map[string]string `json:"outputs,omitempty"`
	Hooks       *HooksResult                 `json:"hooks,omitempty"`
	Steps       []StepResult                 `json:"steps"`
	DurationMS  int64                        `json:"duration_ms"`
}

type runner struct {
	def            *Definition
	opts           RunOptions
	result         *RunResult
	state          *persistedRunState
	statePath      string
	definitionHash string
	outputs        map[string]map[string]string
}

func (r *RunResult) ensureHooks() *HooksResult {
	if r.Hooks == nil {
		r.Hooks = &HooksResult{}
	}
	return r.Hooks
}

func recordErrorHook(ctx context.Context, command string, env map[string]string, opts RunOptions, result *RunResult) {
	ehr, hookErr := runHookAndRecord(ctx, command, env, opts.DryRun, opts.Stdout, opts.Stderr)
	if ehr == nil {
		return
	}
	result.ensureHooks().Error = ehr
	_ = hookErr // error hook failures never override the original error
}

// Run executes a named workflow from the Definition.
func Run(ctx context.Context, def *Definition, opts RunOptions) (*RunResult, error) {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}

	wf, ok := def.Workflows[opts.WorkflowName]
	if !ok {
		return nil, fmt.Errorf("workflow: unknown workflow %q", opts.WorkflowName)
	}
	if wf.Private {
		return nil, fmt.Errorf("workflow: %q is private and cannot be run directly", opts.WorkflowName)
	}

	result := &RunResult{
		Workflow: opts.WorkflowName,
		Steps:    make([]StepResult, 0),
	}
	start := time.Now()
	defer func() {
		result.DurationMS = time.Since(start).Milliseconds()
	}()

	r, err := newRunner(def, opts, result)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result, err
	}

	env := mergeEnv(def.Env, wf.Env, r.opts.Params)

	if resumed := r.resumedHook("before_all", def.BeforeAll); resumed != nil {
		result.ensureHooks().BeforeAll = resumed
	} else if hr, hookErr := runHookAndRecord(ctx, def.BeforeAll, env, opts.DryRun, opts.Stdout, opts.Stderr); hr != nil {
		result.ensureHooks().BeforeAll = hr
		if hookErr != nil {
			wrapped := fmt.Errorf("workflow: before_all hook failed: %w", hookErr)
			r.markFailure(wrapped, "before_all")
			recordErrorHook(ctx, def.Error, env, opts, result)
			return result, wrapped
		}
		if persistErr := r.persistHook("before_all", hr); persistErr != nil {
			r.markFailure(persistErr, "before_all")
			recordErrorHook(ctx, def.Error, env, opts, result)
			return result, persistErr
		}
	}

	if execErr := r.executeSteps(ctx, opts.WorkflowName, wf.Steps, env, "", 0); execErr != nil {
		r.markFailure(execErr, result.FailedStep)
		recordErrorHook(ctx, def.Error, env, opts, result)
		return result, execErr
	}

	if resumed := r.resumedHook("after_all", def.AfterAll); resumed != nil {
		result.ensureHooks().AfterAll = resumed
	} else if hr, hookErr := runHookAndRecord(ctx, def.AfterAll, env, opts.DryRun, opts.Stdout, opts.Stderr); hr != nil {
		result.ensureHooks().AfterAll = hr
		if hookErr != nil {
			wrapped := fmt.Errorf("workflow: after_all hook failed: %w", hookErr)
			r.markFailure(wrapped, "after_all")
			recordErrorHook(ctx, def.Error, env, opts, result)
			return result, wrapped
		}
		if persistErr := r.persistHook("after_all", hr); persistErr != nil {
			r.markFailure(persistErr, "after_all")
			recordErrorHook(ctx, def.Error, env, opts, result)
			return result, persistErr
		}
	}

	if finishErr := r.finishSuccess(); finishErr != nil {
		r.markFailure(finishErr, "")
		return result, finishErr
	}
	result.Status = "ok"
	return result, nil
}

func newRunner(def *Definition, opts RunOptions, result *RunResult) (*runner, error) {
	r := &runner{
		def:     def,
		opts:    opts,
		result:  result,
		outputs: map[string]map[string]string{},
	}

	if opts.DryRun {
		return r, nil
	}

	if strings.TrimSpace(opts.ResumeRunID) != "" && strings.TrimSpace(opts.StateDir) == "" {
		return nil, fmt.Errorf("workflow: --resume requires a state directory")
	}
	if strings.TrimSpace(opts.StateDir) == "" {
		return r, nil
	}

	hash, err := definitionFingerprint(def)
	if err != nil {
		return nil, err
	}
	r.definitionHash = hash

	if strings.TrimSpace(opts.ResumeRunID) != "" {
		runFile := runStateFilePath(opts.StateDir, opts.ResumeRunID)
		state, err := loadRunState(runFile)
		if err != nil {
			return nil, err
		}
		if state == nil {
			return nil, fmt.Errorf("workflow: resume run %q not found", opts.ResumeRunID)
		}
		if len(r.opts.Params) == 0 && len(state.Params) > 0 {
			r.opts.Params = cloneStringMap(state.Params)
		}
		if err := r.validateResumeState(state); err != nil {
			return nil, err
		}
		r.state = state
		r.statePath = runFile
		r.result.RunID = state.RunID
		r.result.RunFile = runFile
		r.result.Resumed = true
		r.outputs = r.outputsFromState(state)
		r.result.Outputs = cloneNestedStringMap(r.outputs)
		return r, nil
	}

	state, err := newPersistedRunState(opts.WorkflowName, opts.WorkflowFile, hash, opts.Params)
	if err != nil {
		return nil, err
	}
	runFile := runStateFilePath(opts.StateDir, state.RunID)
	if err := saveRunState(runFile, state); err != nil {
		return nil, err
	}
	r.state = &state
	r.statePath = runFile
	r.result.RunID = state.RunID
	r.result.RunFile = runFile
	return r, nil
}

func (r *runner) validateResumeState(state *persistedRunState) error {
	switch {
	case state.Workflow != r.opts.WorkflowName:
		return fmt.Errorf("workflow: resume run %q belongs to workflow %q, not %q", state.RunID, state.Workflow, r.opts.WorkflowName)
	case state.WorkflowFile != r.opts.WorkflowFile:
		return fmt.Errorf("workflow: resume run %q was created from a different workflow file", state.RunID)
	case state.DefinitionHash != r.definitionHash:
		return fmt.Errorf("workflow: resume run %q does not match the current workflow definition", state.RunID)
	case !maps.Equal(state.Params, r.opts.Params):
		return fmt.Errorf("workflow: resume run %q does not match the current workflow parameters", state.RunID)
	default:
		return nil
	}
}

func (r *runner) outputsFromState(state *persistedRunState) map[string]map[string]string {
	outputs := map[string]map[string]string{}
	if state == nil || len(state.Steps) == 0 {
		return outputs
	}
	for _, stepKey := range slices.Sorted(maps.Keys(state.Steps)) {
		step := state.Steps[stepKey]
		if strings.TrimSpace(step.Name) == "" || len(step.Outputs) == 0 {
			continue
		}
		outputs[step.Name] = cloneStringMap(step.Outputs)
	}
	return outputs
}

func (r *runner) resumedHook(which, command string) *HookResult {
	if r.state == nil || r.state.Hooks == nil {
		return nil
	}

	var persisted *persistedHookState
	switch which {
	case "before_all":
		persisted = r.state.Hooks.BeforeAll
	case "after_all":
		persisted = r.state.Hooks.AfterAll
	default:
		return nil
	}
	if persisted == nil || persisted.Status != "ok" {
		return nil
	}
	return &HookResult{
		Command:    command,
		Status:     "resumed",
		DurationMS: 0,
	}
}

func (r *runner) persistHook(which string, hook *HookResult) error {
	if r.state == nil || hook == nil || hook.Status != "ok" {
		return nil
	}
	if r.state.Hooks == nil {
		r.state.Hooks = &persistedRunHooks{}
	}
	persisted := &persistedHookState{
		Command: hook.Command,
		Status:  "ok",
	}
	switch which {
	case "before_all":
		r.state.Hooks.BeforeAll = persisted
	case "after_all":
		r.state.Hooks.AfterAll = persisted
	default:
		return nil
	}
	return saveRunState(r.statePath, *r.state)
}

func (r *runner) executeSteps(ctx context.Context, workflowName string, steps []Step, env map[string]string, callPath string, depth int) error {
	for i, step := range steps {
		idx := i + 1
		stepKey := appendStepKey(callPath, workflowName, idx)
		stepStart := time.Now()

		sr := StepResult{
			Index:    idx,
			Name:     step.Name,
			Command:  step.Run,
			Workflow: strings.TrimSpace(step.Workflow),
		}
		if workflowName != r.opts.WorkflowName {
			sr.ParentWorkflow = workflowName
		}

		if ifVar := strings.TrimSpace(step.If); ifVar != "" {
			val, ok := env[ifVar]
			if !ok {
				val = os.Getenv(ifVar)
			}
			if !isTruthy(val) {
				sr.Status = "skipped"
				sr.DurationMS = time.Since(stepStart).Milliseconds()
				r.result.Steps = append(r.result.Steps, sr)
				continue
			}
		}

		if ref := sr.Workflow; ref != "" {
			if depth+1 > MaxCallDepth {
				err := fmt.Errorf("workflow: %s step %d: max call depth %d exceeded", workflowName, idx, MaxCallDepth)
				sr.Status = "error"
				sr.Error = fmt.Sprintf("max call depth %d exceeded", MaxCallDepth)
				sr.DurationMS = time.Since(stepStart).Milliseconds()
				r.result.Steps = append(r.result.Steps, sr)
				r.result.FailedStep = failedStepName(step.Name, stepKey)
				return err
			}

			subWf, ok := r.def.Workflows[ref]
			if !ok {
				err := fmt.Errorf("workflow: %s step %d: unknown workflow %q", workflowName, idx, ref)
				sr.Status = "error"
				sr.Error = fmt.Sprintf("unknown workflow %q", ref)
				sr.DurationMS = time.Since(stepStart).Milliseconds()
				r.result.Steps = append(r.result.Steps, sr)
				r.result.FailedStep = failedStepName(step.Name, stepKey)
				return err
			}

			resolvedWith := cloneStringMap(step.With)
			var err error
			if !r.opts.DryRun {
				resolvedWith, err = interpolateMapValues(step.With, r.outputs)
				if err != nil {
					wrapped := fmt.Errorf("workflow: %s step %d: %w", workflowName, idx, err)
					sr.Status = "error"
					sr.Error = err.Error()
					sr.DurationMS = time.Since(stepStart).Milliseconds()
					r.result.Steps = append(r.result.Steps, sr)
					r.result.FailedStep = failedStepName(step.Name, stepKey)
					return wrapped
				}
			}

			subEnv := mergeEnv(subWf.Env, env, resolvedWith)
			if r.opts.DryRun {
				fmt.Fprintf(r.opts.Stderr, "[dry-run] step %d: workflow %s\n", idx, ref)
			}

			if err := r.executeSteps(ctx, ref, subWf.Steps, subEnv, stepKey, depth+1); err != nil {
				return err
			}
			continue
		}

		if r.state != nil {
			if persisted, ok := r.state.Steps[stepKey]; ok && persisted.Status == "ok" {
				sr.Status = "resumed"
				sr.Outputs = cloneStringMap(persisted.Outputs)
				sr.DurationMS = 0
				r.result.Steps = append(r.result.Steps, sr)
				if strings.TrimSpace(persisted.Name) != "" && len(persisted.Outputs) > 0 {
					r.outputs[persisted.Name] = cloneStringMap(persisted.Outputs)
					r.result.Outputs = cloneNestedStringMap(r.outputs)
				}
				continue
			}
		}

		if r.opts.DryRun {
			fmt.Fprintf(r.opts.Stderr, "[dry-run] step %d: %s\n", idx, step.Run)
			sr.Status = "dry-run"
			sr.DurationMS = time.Since(stepStart).Milliseconds()
			r.result.Steps = append(r.result.Steps, sr)
			continue
		}

		command, err := interpolateCommand(step.Run, r.outputs)
		if err != nil {
			wrapped := fmt.Errorf("workflow: %s step %d: %w", workflowName, idx, err)
			sr.Status = "error"
			sr.Error = err.Error()
			sr.DurationMS = time.Since(stepStart).Milliseconds()
			r.result.Steps = append(r.result.Steps, sr)
			r.result.FailedStep = failedStepName(step.Name, stepKey)
			return wrapped
		}

		stdout := r.opts.Stdout
		var captured bytes.Buffer
		if len(step.Outputs) > 0 {
			stdout = io.MultiWriter(r.opts.Stdout, &captured)
		}

		if err := runShellCommand(ctx, command, env, stdout, r.opts.Stderr); err != nil {
			wrapped := fmt.Errorf("workflow: %s step %d: %w", workflowName, idx, err)
			sr.Status = "error"
			sr.Error = err.Error()
			sr.DurationMS = time.Since(stepStart).Milliseconds()
			r.result.Steps = append(r.result.Steps, sr)
			r.result.FailedStep = failedStepName(step.Name, stepKey)
			return wrapped
		}

		if len(step.Outputs) > 0 {
			extracted, err := extractDeclaredOutputs(step.Outputs, captured.Bytes())
			if err != nil {
				wrapped := fmt.Errorf("workflow: %s step %d: %w", workflowName, idx, err)
				sr.Status = "error"
				sr.Error = err.Error()
				sr.DurationMS = time.Since(stepStart).Milliseconds()
				r.result.Steps = append(r.result.Steps, sr)
				r.result.FailedStep = failedStepName(step.Name, stepKey)
				return wrapped
			}
			sr.Outputs = extracted
			if strings.TrimSpace(step.Name) != "" {
				r.outputs[step.Name] = cloneStringMap(extracted)
				r.result.Outputs = cloneNestedStringMap(r.outputs)
			}
		}

		sr.Status = "ok"
		sr.DurationMS = time.Since(stepStart).Milliseconds()
		r.result.Steps = append(r.result.Steps, sr)

		if err := r.persistStep(stepKey, sr); err != nil {
			r.result.FailedStep = failedStepName(step.Name, stepKey)
			return err
		}
	}
	return nil
}

func (r *runner) persistStep(stepKey string, sr StepResult) error {
	if r.state == nil || sr.Status != "ok" {
		return nil
	}
	r.state.Steps[stepKey] = persistedStepState{
		Name:           sr.Name,
		Workflow:       sr.Workflow,
		ParentWorkflow: sr.ParentWorkflow,
		Status:         "ok",
		Outputs:        cloneStringMap(sr.Outputs),
	}
	return saveRunState(r.statePath, *r.state)
}

func (r *runner) finishSuccess() error {
	r.result.Outputs = cloneNestedStringMap(r.outputs)
	if r.state == nil {
		return nil
	}
	r.state.Status = "ok"
	r.state.FailedStep = ""
	return saveRunState(r.statePath, *r.state)
}

func (r *runner) markFailure(err error, failedStep string) {
	r.result.Status = "error"
	r.result.Error = err.Error()
	r.result.Outputs = cloneNestedStringMap(r.outputs)
	if strings.TrimSpace(failedStep) != "" {
		r.result.FailedStep = failedStep
	}

	if r.state == nil {
		return
	}
	r.state.Status = "error"
	r.state.FailedStep = r.result.FailedStep
	_ = saveRunState(r.statePath, *r.state)

	if r.hasRecoverableState() {
		r.result.Recoverable = true
		r.result.Resume = &ResumeInfo{Command: r.resumeCommand()}
	}
}

func (r *runner) hasRecoverableState() bool {
	if r.state == nil {
		return false
	}
	if len(r.state.Steps) > 0 {
		return true
	}
	return r.state.Hooks != nil && r.state.Hooks.BeforeAll != nil && r.state.Hooks.BeforeAll.Status == "ok"
}

func (r *runner) resumeCommand() string {
	if strings.TrimSpace(r.result.RunID) == "" {
		return ""
	}

	parts := []string{"asc", "workflow", "run"}
	if strings.TrimSpace(r.opts.WorkflowFile) != "" {
		parts = append(parts, "--file", shellQuote(r.opts.WorkflowFile))
	}
	parts = append(parts, shellQuote(r.opts.WorkflowName), "--resume", shellQuote(r.result.RunID))
	return strings.Join(parts, " ")
}

func runHookAndRecord(ctx context.Context, command string, env map[string]string, dryRun bool, stdout, stderr io.Writer) (*HookResult, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, nil
	}

	start := time.Now()
	err := runHook(ctx, command, env, dryRun, stdout, stderr)

	hr := &HookResult{
		Command:    command,
		DurationMS: time.Since(start).Milliseconds(),
	}
	if dryRun {
		hr.Status = "dry-run"
		if err != nil {
			hr.Status = "error"
			hr.Error = err.Error()
			return hr, err
		}
		return hr, nil
	}
	if err != nil {
		hr.Status = "error"
		hr.Error = err.Error()
		return hr, err
	}

	hr.Status = "ok"
	return hr, nil
}

func appendStepKey(callPath, workflowName string, idx int) string {
	segment := fmt.Sprintf("%s[%d]", workflowName, idx)
	if strings.TrimSpace(callPath) == "" {
		return segment
	}
	return callPath + "/" + segment
}

func failedStepName(name, stepKey string) string {
	if strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return stepKey
}
