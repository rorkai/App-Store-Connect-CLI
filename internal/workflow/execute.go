package workflow

import (
	"context"
	"fmt"
	"io"
	"os"
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
}

// StepResult records one executed step.
type StepResult struct {
	Index          int    `json:"index"`
	Name           string `json:"name,omitempty"`
	Command        string `json:"command,omitempty"`
	Workflow       string `json:"workflow,omitempty"`
	ParentWorkflow string `json:"parent_workflow,omitempty"`
	Status         string `json:"status"`
	DurationMS     int64  `json:"duration_ms"`
	Error          string `json:"error,omitempty"`
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

// RunResult is the structured output of a workflow execution.
type RunResult struct {
	Workflow   string       `json:"workflow"`
	Status     string       `json:"status"`
	Error      string       `json:"error,omitempty"`
	Hooks      *HooksResult `json:"hooks,omitempty"`
	Steps      []StepResult `json:"steps"`
	DurationMS int64        `json:"duration_ms"`
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

	env := mergeEnv(def.Env, wf.Env, opts.Params)

	result := &RunResult{
		Workflow: opts.WorkflowName,
		Steps:    make([]StepResult, 0),
	}
	start := time.Now()
	defer func() {
		// Include hooks and error hooks in total duration.
		result.DurationMS = time.Since(start).Milliseconds()
	}()

	// before_all hook
	if hr, err := runHookAndRecord(ctx, def.BeforeAll, env, opts.DryRun, opts.Stdout, opts.Stderr); hr != nil {
		result.ensureHooks().BeforeAll = hr
		if err != nil {
			wrapped := fmt.Errorf("workflow: before_all hook failed: %w", err)
			result.Status = "error"
			result.Error = wrapped.Error()

			recordErrorHook(ctx, def.Error, env, opts, result)
			return result, wrapped
		}
	}

	// Execute steps
	if err := executeSteps(ctx, def, opts.WorkflowName, wf.Steps, env, 0, opts, result); err != nil {
		result.Status = "error"
		result.Error = err.Error()

		recordErrorHook(ctx, def.Error, env, opts, result)
		return result, err
	}

	// after_all hook
	if hr, err := runHookAndRecord(ctx, def.AfterAll, env, opts.DryRun, opts.Stdout, opts.Stderr); hr != nil {
		result.ensureHooks().AfterAll = hr
		if err != nil {
			wrapped := fmt.Errorf("workflow: after_all hook failed: %w", err)
			result.Status = "error"
			result.Error = wrapped.Error()

			recordErrorHook(ctx, def.Error, env, opts, result)
			return result, wrapped
		}
	}

	result.Status = "ok"
	return result, nil
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
		// Even in dry-run, surface writer failures (e.g. broken pipe) instead of
		// silently discarding them.
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

func executeSteps(ctx context.Context, def *Definition, workflowName string, steps []Step, env map[string]string, depth int, opts RunOptions, result *RunResult) error {
	for i, step := range steps {
		idx := i + 1
		stepStart := time.Now()

		sr := StepResult{
			Index:    idx,
			Name:     step.Name,
			Command:  step.Run,
			Workflow: strings.TrimSpace(step.Workflow),
		}
		if workflowName != opts.WorkflowName {
			sr.ParentWorkflow = workflowName
		}

		// Check conditional
		if ifVar := strings.TrimSpace(step.If); ifVar != "" {
			val, ok := env[ifVar]
			if !ok {
				val = os.Getenv(ifVar)
			}
			if !isTruthy(val) {
				sr.Status = "skipped"
				sr.DurationMS = time.Since(stepStart).Milliseconds()
				result.Steps = append(result.Steps, sr)
				continue
			}
		}

		if ref := sr.Workflow; ref != "" {

			if depth+1 > MaxCallDepth {
				sr.Status = "error"
				sr.Error = fmt.Sprintf("max call depth %d exceeded", MaxCallDepth)
				sr.DurationMS = time.Since(stepStart).Milliseconds()
				result.Steps = append(result.Steps, sr)
				return fmt.Errorf("workflow: %s step %d: max call depth %d exceeded", workflowName, idx, MaxCallDepth)
			}

			subWf, ok := def.Workflows[ref]
			if !ok {
				sr.Status = "error"
				sr.Error = fmt.Sprintf("unknown workflow %q", ref)
				sr.DurationMS = time.Since(stepStart).Milliseconds()
				result.Steps = append(result.Steps, sr)
				return fmt.Errorf("workflow: %s step %d: unknown workflow %q", workflowName, idx, ref)
			}

			// Sub-workflow env provides defaults; caller env (including CLI params)
			// overrides; call-site "with" wins over all.
			subEnv := mergeEnv(subWf.Env, env, step.With)

			if opts.DryRun {
				_, _ = fmt.Fprintf(opts.Stderr, "[dry-run] step %d: workflow %s\n", idx, ref)
			}

			if err := executeSteps(ctx, def, ref, subWf.Steps, subEnv, depth+1, opts, result); err != nil {
				return err
			}
			continue
		}

		// run: step
		if opts.DryRun {
			_, _ = fmt.Fprintf(opts.Stderr, "[dry-run] step %d: %s\n", idx, step.Run)
			sr.Status = "dry-run"
			sr.DurationMS = time.Since(stepStart).Milliseconds()
			result.Steps = append(result.Steps, sr)
			continue
		}

		if err := runShellCommand(ctx, step.Run, env, opts.Stdout, opts.Stderr); err != nil {
			sr.Status = "error"
			sr.Error = err.Error()
			sr.DurationMS = time.Since(stepStart).Milliseconds()
			result.Steps = append(result.Steps, sr)
			return fmt.Errorf("workflow: %s step %d: %w", workflowName, idx, err)
		}

		sr.Status = "ok"
		sr.DurationMS = time.Since(stepStart).Milliseconds()
		result.Steps = append(result.Steps, sr)
	}
	return nil
}
