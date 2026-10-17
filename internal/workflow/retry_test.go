package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type retrySignalWriter struct {
	bytes.Buffer
	needle string
	ready  chan struct{}
	once   sync.Once
}

func (w *retrySignalWriter) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if strings.Contains(w.String(), w.needle) {
		w.once.Do(func() { close(w.ready) })
	}
	return n, err
}

func loadWorkflowForRetryTest(t *testing.T, content string) (*Definition, string) {
	t.Helper()
	dir := t.TempDir()
	path := writeWorkflowFile(t, dir, content)
	def, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return def, path
}

func TestValidate_RetryAndTimeoutPaths(t *testing.T) {
	tests := []struct {
		name     string
		stepJSON string
		wantCode ValidationCode
		wantPath string
	}{
		{"retry missing attempts", `{"run":"echo ok","retry":{"delay":"1s"}}`, "invalid_retry_max_attempts", "workflows.main.steps[0].retry.max_attempts"},
		{"retry one attempt", `{"run":"echo ok","retry":{"max_attempts":1,"delay":"1s"}}`, "invalid_retry_max_attempts", "workflows.main.steps[0].retry.max_attempts"},
		{"retry too many attempts", `{"run":"echo ok","retry":{"max_attempts":101,"delay":"1s"}}`, "invalid_retry_max_attempts", "workflows.main.steps[0].retry.max_attempts"},
		{"retry missing delay", `{"run":"echo ok","retry":{"max_attempts":2}}`, "invalid_retry_delay", "workflows.main.steps[0].retry.delay"},
		{"retry zero delay", `{"run":"echo ok","retry":{"max_attempts":2,"delay":"0s"}}`, "invalid_retry_delay", "workflows.main.steps[0].retry.delay"},
		{"retry negative delay", `{"run":"echo ok","retry":{"max_attempts":2,"delay":"-1s"}}`, "invalid_retry_delay", "workflows.main.steps[0].retry.delay"},
		{"retry invalid delay", `{"run":"echo ok","retry":{"max_attempts":2,"delay":"later"}}`, "invalid_retry_delay", "workflows.main.steps[0].retry.delay"},
		{"retry excessive delay", `{"run":"echo ok","retry":{"max_attempts":2,"delay":"24h1s"}}`, "invalid_retry_delay", "workflows.main.steps[0].retry.delay"},
		{"null retry", `{"run":"echo ok","retry":null}`, "invalid_step_retry", "workflows.main.steps[0].retry"},
		{"mixed-case null retry", `{"run":"echo ok","Retry":null}`, "invalid_step_retry", "workflows.main.steps[0].retry"},
		{"zero timeout", `{"run":"echo ok","timeout":"0s"}`, "invalid_step_timeout", "workflows.main.steps[0].timeout"},
		{"negative timeout", `{"run":"echo ok","timeout":"-1s"}`, "invalid_step_timeout", "workflows.main.steps[0].timeout"},
		{"invalid timeout", `{"run":"echo ok","timeout":"forever"}`, "invalid_step_timeout", "workflows.main.steps[0].timeout"},
		{"excessive timeout", `{"run":"echo ok","timeout":"24h1s"}`, "invalid_step_timeout", "workflows.main.steps[0].timeout"},
		{"empty timeout", `{"run":"echo ok","timeout":""}`, "invalid_step_timeout", "workflows.main.steps[0].timeout"},
		{"null timeout", `{"run":"echo ok","timeout":null}`, "invalid_step_timeout", "workflows.main.steps[0].timeout"},
		{"mixed-case null timeout", `{"run":"echo ok","Timeout":null}`, "invalid_step_timeout", "workflows.main.steps[0].timeout"},
		{"retry on workflow", `{"workflow":"child","retry":{"max_attempts":2,"delay":"1s"}}`, "step_retry_on_workflow", "workflows.main.steps[0].retry"},
		{"timeout on workflow", `{"workflow":"child","timeout":"1s"}`, "step_timeout_on_workflow", "workflows.main.steps[0].timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeWorkflowFile(t, dir, fmt.Sprintf(`{
				"workflows": {
					"main": {"steps": [%s]},
					"child": {"steps": ["echo child"]}
				}
			}`, tt.stepJSON))
			def, err := LoadUnvalidated(path)
			if err != nil {
				t.Fatalf("LoadUnvalidated: %v", err)
			}
			errs := Validate(def)
			var got *ValidationError
			for _, validationErr := range errs {
				if validationErr.Code == tt.wantCode {
					got = validationErr
					break
				}
			}
			if got == nil {
				t.Fatalf("validation errors = %+v, want code %q", errs, tt.wantCode)
			}
			if got.Path != tt.wantPath {
				t.Fatalf("path = %q, want %q", got.Path, tt.wantPath)
			}
		})
	}
}

func TestValidate_RetryAndTimeoutCaseVariantLastValueWins(t *testing.T) {
	tests := []struct {
		name     string
		stepJSON string
		wantCode ValidationCode
	}{
		{
			name:     "retry null then policy",
			stepJSON: `{"run":"echo ok","retry":null,"Retry":{"max_attempts":2,"delay":"1s"}}`,
		},
		{
			name:     "retry policy then null",
			stepJSON: `{"run":"echo ok","Retry":{"max_attempts":2,"delay":"1s"},"retry":null}`,
			wantCode: ErrInvalidStepRetry,
		},
		{
			name:     "timeout null then duration",
			stepJSON: `{"run":"echo ok","timeout":null,"Timeout":"1s"}`,
		},
		{
			name:     "timeout duration then null",
			stepJSON: `{"run":"echo ok","Timeout":"1s","timeout":null}`,
			wantCode: ErrInvalidStepTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeWorkflowFile(t, dir, fmt.Sprintf(`{
				"workflows": {"main": {"steps": [%s]}}
			}`, tt.stepJSON))
			def, err := LoadUnvalidated(path)
			if err != nil {
				t.Fatalf("LoadUnvalidated: %v", err)
			}
			errs := Validate(def)
			if tt.wantCode == "" {
				if len(errs) != 0 {
					t.Fatalf("validation errors = %+v, want none", errs)
				}
				return
			}
			for _, validationErr := range errs {
				if validationErr.Code == tt.wantCode {
					return
				}
			}
			t.Fatalf("validation errors = %+v, want code %q", errs, tt.wantCode)
		})
	}
}

func TestValidate_RetryPathEncodesUnsafeWorkflowName(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkflowFile(t, dir, `{
		"workflows": {
			"unsafe\n.name]": {
				"steps": [{"run":"echo ok","retry":null}]
			}
		}
	}`)
	def, err := LoadUnvalidated(path)
	if err != nil {
		t.Fatalf("LoadUnvalidated: %v", err)
	}
	for _, validationErr := range Validate(def) {
		if validationErr.Code != ErrInvalidStepRetry {
			continue
		}
		const wantPath = `workflows["unsafe\n.name]"].steps[0].retry`
		if validationErr.Path != wantPath {
			t.Fatalf("path = %q, want %q", validationErr.Path, wantPath)
		}
		if strings.ContainsRune(validationErr.Message, '\n') {
			t.Fatalf("message contains raw newline: %q", validationErr.Message)
		}
		return
	}
	t.Fatalf("validation errors = %+v, want code %q", Validate(def), ErrInvalidStepRetry)
}

func TestRun_RetryEventuallySucceedsAndCapturesOnlySuccessfulOutput(t *testing.T) {
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "counter")
	def, workflowPath := loadWorkflowForRetryTest(t, fmt.Sprintf(`{
		"env": {"COUNTER_PATH": %q},
		"workflows": {
			"main": {
				"steps": [{
					"name": "eventual",
					"run": "count=$(cat \"$COUNTER_PATH\" 2>/dev/null || echo 0); count=$((count+1)); printf '%%s' \"$count\" > \"$COUNTER_PATH\"; if [ \"$count\" -lt 3 ]; then printf '{\\\"value\\\":\\\"stale\\\"}'; exit 42; fi; printf '{\\\"value\\\":\\\"fresh\\\"}'",
					"retry": {"max_attempts": 3, "delay": "1ms"},
					"timeout": "2s",
					"outputs": {"VALUE": "$.value"}
				}]
			}
		}
	}`, counterPath))

	opts := runOpts("main")
	opts.WorkflowFile = workflowPath
	opts.StateDir = filepath.Join(dir, "runs")
	result, err := Run(context.Background(), def, opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	step := result.Steps[0]
	if step.Status != "ok" || len(step.Attempts) != 3 {
		t.Fatalf("step = %+v, want three attempts and ok", step)
	}
	wantStatuses := []string{"error", "error", "ok"}
	for i, want := range wantStatuses {
		if step.Attempts[i].Attempt != i+1 || step.Attempts[i].Invocation != 1 || step.Attempts[i].Status != want {
			t.Fatalf("attempt %d = %+v, want invocation=1 attempt=%d status=%s", i, step.Attempts[i], i+1, want)
		}
	}
	if got := result.Outputs["eventual"]["VALUE"]; got != "fresh" {
		t.Fatalf("captured output = %q, want fresh", got)
	}
	stderr := opts.Stderr.(*bytes.Buffer).String()
	if !strings.Contains(stderr, "attempt 1/3") || !strings.Contains(stderr, "retrying in 1ms") {
		t.Fatalf("stderr retry diagnostics = %q", stderr)
	}

	state, err := loadRunState(result.RunFile)
	if err != nil {
		t.Fatalf("loadRunState: %v", err)
	}
	persisted := state.Steps["main[1]"]
	if persisted.Status != "ok" || len(persisted.Attempts) != 3 || persisted.Outputs["VALUE"] != "fresh" {
		t.Fatalf("persisted step = %+v", persisted)
	}
}

func TestRun_SuccessfulCommandWithBackgroundPipeHolderIsNotRetried(t *testing.T) {
	originalWaitDelay := shellWaitDelay
	shellWaitDelay = 20 * time.Millisecond
	t.Cleanup(func() { shellWaitDelay = originalWaitDelay })

	dir := t.TempDir()
	counterPath := filepath.Join(dir, "counter")
	def, _ := loadWorkflowForRetryTest(t, fmt.Sprintf(`{
		"env": {"COUNTER_PATH": %q},
		"workflows": {
			"main": {"steps": [{
				"name": "background",
				"run": "printf x >> \"$COUNTER_PATH\"; sleep 1 & printf '{\"value\":\"ok\"}'",
				"retry": {"max_attempts": 2, "delay": "1ms"},
				"outputs": {"VALUE": "$.value"}
			}]}
		}
	}`, counterPath))

	result, err := Run(context.Background(), def, runOpts("main"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != "ok" || len(result.Steps) != 1 {
		t.Fatalf("result = %+v, want one successful step", result)
	}
	step := result.Steps[0]
	if len(step.Attempts) != 1 || step.Attempts[0].Status != "ok" {
		t.Fatalf("attempts = %+v, want one successful attempt", step.Attempts)
	}
	if got := result.Outputs["background"]["VALUE"]; got != "ok" {
		t.Fatalf("output = %q, want ok", got)
	}
	data, readErr := os.ReadFile(counterPath)
	if readErr != nil {
		t.Fatalf("read counter: %v", readErr)
	}
	if got := string(data); got != "x" {
		t.Fatalf("counter = %q, want one execution", got)
	}
}

func TestRun_RetryExhaustionRunsErrorHookOnce(t *testing.T) {
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "counter")
	errorHookPath := filepath.Join(dir, "error-hook")
	afterHookPath := filepath.Join(dir, "after-hook")
	def, workflowPath := loadWorkflowForRetryTest(t, fmt.Sprintf(`{
		"env": {
			"COUNTER_PATH": %q,
			"ERROR_HOOK_PATH": %q,
			"AFTER_HOOK_PATH": %q
		},
		"error": "printf error >> \"$ERROR_HOOK_PATH\"",
		"after_all": "printf after >> \"$AFTER_HOOK_PATH\"",
		"workflows": {
			"main": {"steps": [{
				"name": "exhausted",
				"run": "printf x >> \"$COUNTER_PATH\"; exit 7",
				"retry": {"max_attempts": 3, "delay": "1ms"},
				"timeout": "1s"
			}]}
		}
	}`, counterPath, errorHookPath, afterHookPath))

	opts := runOpts("main")
	opts.WorkflowFile = workflowPath
	opts.StateDir = filepath.Join(dir, "runs")
	result, err := Run(context.Background(), def, opts)
	if err == nil {
		t.Fatal("expected retry exhaustion")
	}
	step := result.Steps[0]
	if step.Status != "error" || step.FailureReason != "command_failed" || len(step.Attempts) != 3 {
		t.Fatalf("step = %+v", step)
	}
	if !result.Recoverable || result.Resume == nil || strings.TrimSpace(result.Resume.Command) == "" {
		t.Fatalf("exhausted configured step must be resumable, got %+v", result.Resume)
	}
	if got := readFileOrEmpty(t, counterPath); got != "xxx" {
		t.Fatalf("attempt side effects = %q, want xxx", got)
	}
	if got := readFileOrEmpty(t, errorHookPath); got != "error" {
		t.Fatalf("error hook output = %q, want one execution", got)
	}
	if _, statErr := os.Stat(afterHookPath); !os.IsNotExist(statErr) {
		t.Fatalf("after_all must not run after exhaustion; stat error = %v", statErr)
	}
}

func TestRun_FirstFailureWithoutPolicyKeepsExistingRecoveryBehavior(t *testing.T) {
	dir := t.TempDir()
	def := &Definition{
		Workflows: map[string]Workflow{
			"main": {Steps: []Step{{Name: "plain", Run: "exit 9"}}},
		},
	}
	opts := runOpts("main")
	opts.WorkflowFile = filepath.Join(dir, "workflow.json")
	opts.StateDir = filepath.Join(dir, "runs")
	result, err := Run(context.Background(), def, opts)
	if err == nil {
		t.Fatal("expected failure")
	}
	if result.Recoverable || result.Resume != nil {
		t.Fatalf("unconfigured first-step failure unexpectedly became recoverable: %+v", result)
	}
	state, loadErr := loadRunState(result.RunFile)
	if loadErr != nil {
		t.Fatalf("loadRunState: %v", loadErr)
	}
	if len(state.Steps) != 0 {
		t.Fatalf("unconfigured failed step was persisted: %+v", state.Steps)
	}
}

func TestRun_WorkflowCallPolicyIsRejectedAtRuntime(t *testing.T) {
	def := &Definition{Workflows: map[string]Workflow{
		"main": {Steps: []Step{{
			Name:     "invalid_call",
			Workflow: "child",
			Retry:    &RetryPolicy{MaxAttempts: 2, Delay: "1s"},
		}}},
		"child": {Private: true, Steps: []Step{{Run: "echo child"}}},
	}}
	result, err := Run(context.Background(), def, runOpts("main"))
	if err == nil {
		t.Fatal("expected workflow-call policy rejection")
	}
	if len(result.Steps) != 1 || result.Steps[0].FailureReason != "invalid_policy" {
		t.Fatalf("result = %+v, want invalid_policy failure", result)
	}
}

func TestRun_TimeoutHasStableStructuredFailure(t *testing.T) {
	def, _ := loadWorkflowForRetryTest(t, `{
		"workflows": {
			"main": {"steps": [{
				"name": "bounded",
				"run": "sleep 10",
				"timeout": "25ms"
			}]}
		}
	}`)

	start := time.Now()
	result, err := Run(context.Background(), def, runOpts("main"))
	if err == nil {
		t.Fatal("expected timeout")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("timeout did not stop promptly: %s", time.Since(start))
	}
	step := result.Steps[0]
	if step.Status != "error" || step.FailureReason != "timeout" || len(step.Attempts) != 1 {
		t.Fatalf("step = %+v", step)
	}
	if step.Attempts[0].Status != "timeout" || step.Attempts[0].Error != "step timed out after 25ms" {
		t.Fatalf("attempt = %+v", step.Attempts[0])
	}
	if !result.Terminal || result.TerminalReason == "" || result.Recoverable || result.Resume != nil {
		t.Fatalf("timeout-only result must be terminal and nonrecoverable: %+v", result)
	}
}

func TestRun_TimeoutPolicyWithoutRetryIsTerminalAndCannotResume(t *testing.T) {
	tests := []struct {
		name          string
		steps         string
		hasCheckpoint bool
	}{
		{
			name:  "timeout first step",
			steps: `[{"name":"bounded","run":"if [ -f \"$RESUME_MARKER\" ]; then printf x >> \"$REPLAY_COUNTER\"; fi; sleep 10","timeout":"250ms"}]`,
		},
		{
			name:          "timeout after completed checkpoint",
			steps:         `[{"name":"checkpoint","run":"printf x >> \"$CHECKPOINT_COUNTER\""},{"name":"bounded","run":"if [ -f \"$RESUME_MARKER\" ]; then printf x >> \"$REPLAY_COUNTER\"; fi; sleep 10","timeout":"250ms"}]`,
			hasCheckpoint: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			markerPath := filepath.Join(dir, "resume-marker")
			replayCounterPath := filepath.Join(dir, "replay-counter")
			checkpointCounterPath := filepath.Join(dir, "checkpoint-counter")
			def, workflowPath := loadWorkflowForRetryTest(t, fmt.Sprintf(`{
				"env": {
					"RESUME_MARKER": %q,
					"REPLAY_COUNTER": %q,
					"CHECKPOINT_COUNTER": %q
				},
				"workflows": {"main": {"steps": %s}}
			}`, markerPath, replayCounterPath, checkpointCounterPath, tt.steps))

			opts := runOpts("main")
			opts.WorkflowFile = workflowPath
			opts.StateDir = filepath.Join(dir, "runs")
			result, err := Run(context.Background(), def, opts)
			if err == nil {
				t.Fatal("expected step failure")
			}
			failed := result.Steps[len(result.Steps)-1]
			if failed.FailureReason != "timeout" || len(failed.Attempts) != 1 || failed.Attempts[0].FailureReason != "timeout" {
				t.Fatalf("failed step = %+v", failed)
			}
			if !result.Terminal || result.TerminalReason == "" || result.Recoverable || result.Resume != nil {
				t.Fatalf("timeout-only result must be terminal and nonrecoverable: %+v", result)
			}
			state, loadErr := loadRunState(result.RunFile)
			if loadErr != nil {
				t.Fatalf("loadRunState: %v", loadErr)
			}
			if state.Status != "terminal" || state.TerminalReason == "" {
				t.Fatalf("timeout-only state must be terminal: %+v", state)
			}
			if tt.hasCheckpoint && readFileOrEmpty(t, checkpointCounterPath) != "x" {
				t.Fatalf("checkpoint did not complete before timeout: %q", readFileOrEmpty(t, checkpointCounterPath))
			}

			if writeErr := os.WriteFile(markerPath, []byte("resume"), 0o600); writeErr != nil {
				t.Fatalf("write resume marker: %v", writeErr)
			}
			resumeOpts := runOpts("main")
			resumeOpts.WorkflowFile = workflowPath
			resumeOpts.StateDir = opts.StateDir
			resumeOpts.ResumeRunID = result.RunID
			resumeResult, resumeErr := Run(context.Background(), def, resumeOpts)
			if resumeErr == nil || !strings.Contains(resumeErr.Error(), "cannot be resumed") || !strings.Contains(resumeErr.Error(), "timed out") {
				t.Fatalf("resume error = %v, want terminal timeout diagnostic", resumeErr)
			}
			if resumeResult == nil || len(resumeResult.Steps) != 0 {
				t.Fatalf("resume result = %+v, want rejection before workflow execution", resumeResult)
			}
			if got := readFileOrEmpty(t, replayCounterPath); got != "" {
				t.Fatalf("resume reran timeout-only command: %q", got)
			}
			if tt.hasCheckpoint && readFileOrEmpty(t, checkpointCounterPath) != "x" {
				t.Fatalf("resume reran completed checkpoint: %q", readFileOrEmpty(t, checkpointCounterPath))
			}
		})
	}
}

func TestRun_TimeoutConfiguredCommandFailureNeedsRecoveryCheckpoint(t *testing.T) {
	t.Run("first step is diagnostic only", func(t *testing.T) {
		dir := t.TempDir()
		markerPath := filepath.Join(dir, "resume-marker")
		replayCounterPath := filepath.Join(dir, "replay-counter")
		def, workflowPath := loadWorkflowForRetryTest(t, fmt.Sprintf(`{
			"env": {"RESUME_MARKER": %q, "REPLAY_COUNTER": %q},
			"workflows": {"main": {"steps": [{
				"name": "bounded",
				"run": "if [ -f \"$RESUME_MARKER\" ]; then printf x >> \"$REPLAY_COUNTER\"; fi; exit 7",
				"timeout": "1s"
			}]}}
		}`, markerPath, replayCounterPath))

		opts := runOpts("main")
		opts.WorkflowFile = workflowPath
		opts.StateDir = filepath.Join(dir, "runs")
		result, err := Run(context.Background(), def, opts)
		if err == nil || result.Steps[0].FailureReason != "command_failed" {
			t.Fatalf("result = %+v, err = %v", result, err)
		}
		if result.Terminal || result.Recoverable || result.Resume != nil {
			t.Fatalf("diagnostic-only failure must be nonrecoverable without becoming terminal: %+v", result)
		}
		state, loadErr := loadRunState(result.RunFile)
		if loadErr != nil || len(state.Steps["main[1]"].Attempts) != 1 {
			t.Fatalf("diagnostic attempt state = %+v, err = %v", state, loadErr)
		}
		if state.Steps["main[1]"].RetryEnabled {
			t.Fatalf("diagnostic-only step persisted retry opt-in: %+v", state.Steps["main[1]"])
		}

		if writeErr := os.WriteFile(markerPath, []byte("resume"), 0o600); writeErr != nil {
			t.Fatalf("write resume marker: %v", writeErr)
		}
		resumeOpts := runOpts("main")
		resumeOpts.WorkflowFile = workflowPath
		resumeOpts.StateDir = opts.StateDir
		resumeOpts.ResumeRunID = result.RunID
		resumeResult, resumeErr := Run(context.Background(), def, resumeOpts)
		if resumeErr == nil || !strings.Contains(resumeErr.Error(), "cannot be resumed") || !strings.Contains(resumeErr.Error(), "checkpoint") {
			t.Fatalf("resume error = %v, want missing-checkpoint diagnostic", resumeErr)
		}
		if resumeResult == nil || len(resumeResult.Steps) != 0 {
			t.Fatalf("resume result = %+v, want rejection before workflow execution", resumeResult)
		}
		if got := readFileOrEmpty(t, replayCounterPath); got != "" {
			t.Fatalf("resume reran diagnostic-only command: %q", got)
		}
	})

	t.Run("completed checkpoint preserves resume", func(t *testing.T) {
		dir := t.TempDir()
		markerPath := filepath.Join(dir, "resume-marker")
		replayCounterPath := filepath.Join(dir, "replay-counter")
		checkpointCounterPath := filepath.Join(dir, "checkpoint-counter")
		def, workflowPath := loadWorkflowForRetryTest(t, fmt.Sprintf(`{
			"env": {
				"RESUME_MARKER": %q,
				"REPLAY_COUNTER": %q,
				"CHECKPOINT_COUNTER": %q
			},
			"workflows": {"main": {"steps": [
				{"name":"checkpoint","run":"printf x >> \"$CHECKPOINT_COUNTER\""},
				{"name":"bounded","run":"if [ -f \"$RESUME_MARKER\" ]; then printf x >> \"$REPLAY_COUNTER\"; fi; exit 7","timeout":"1s"}
			]}}
		}`, markerPath, replayCounterPath, checkpointCounterPath))

		opts := runOpts("main")
		opts.WorkflowFile = workflowPath
		opts.StateDir = filepath.Join(dir, "runs")
		result, err := Run(context.Background(), def, opts)
		if err == nil || !result.Recoverable || result.Resume == nil || result.Terminal {
			t.Fatalf("checkpointed result = %+v, err = %v", result, err)
		}
		if got := readFileOrEmpty(t, checkpointCounterPath); got != "x" {
			t.Fatalf("checkpoint executions = %q, want x", got)
		}

		if writeErr := os.WriteFile(markerPath, []byte("resume"), 0o600); writeErr != nil {
			t.Fatalf("write resume marker: %v", writeErr)
		}
		resumeOpts := runOpts("main")
		resumeOpts.WorkflowFile = workflowPath
		resumeOpts.StateDir = opts.StateDir
		resumeOpts.ResumeRunID = result.RunID
		resumed, resumeErr := Run(context.Background(), def, resumeOpts)
		if resumeErr == nil || resumed.Steps[0].Status != "resumed" || resumed.Steps[1].FailureReason != "command_failed" {
			t.Fatalf("resumed result = %+v, err = %v", resumed, resumeErr)
		}
		if got := readFileOrEmpty(t, checkpointCounterPath); got != "x" {
			t.Fatalf("resume reran checkpoint: %q", got)
		}
		if got := readFileOrEmpty(t, replayCounterPath); got != "x" {
			t.Fatalf("resume did not rerun failed step exactly once: %q", got)
		}
	})
}

func TestRun_CancellationStopsDuringRetryDelay(t *testing.T) {
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "counter")
	def, _ := loadWorkflowForRetryTest(t, fmt.Sprintf(`{
		"env": {"COUNTER_PATH": %q},
		"workflows": {
			"main": {"steps": [{
				"name": "cancelled",
				"run": "printf x >> \"$COUNTER_PATH\"; exit 8",
				"retry": {"max_attempts": 5, "delay": "1h"}
			}]}
		}
	}`, counterPath))

	stderr := &retrySignalWriter{
		needle: "retrying in 1h0m0s",
		ready:  make(chan struct{}),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		select {
		case <-stderr.ready:
			cancel()
		case <-ctx.Done():
		}
	}()
	opts := runOpts("main")
	opts.Stderr = stderr
	result, err := Run(ctx, def, opts)
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrCommandFailed) {
		t.Fatalf("error = %v, want cancellation rather than a failed command", err)
	}
	select {
	case <-stderr.ready:
	default:
		t.Fatalf("runner never entered retry delay; stderr = %q", stderr.String())
	}
	step := result.Steps[0]
	if step.FailureReason != "canceled" || len(step.Attempts) != 1 {
		t.Fatalf("step = %+v, want canceled after one attempt", step)
	}
	if got := readFileOrEmpty(t, counterPath); got != "x" {
		t.Fatalf("attempt side effects = %q, want one attempt", got)
	}
}

func TestRun_CancellationStopsRunningAttempt(t *testing.T) {
	def, _ := loadWorkflowForRetryTest(t, `{
		"workflows": {
			"main": {"steps": [{
				"name": "cancelled",
				"run": "sleep 10",
				"timeout": "1h"
			}]}
		}
	}`)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	result, err := Run(ctx, def, runOpts("main"))
	if err == nil {
		t.Fatal("expected cancellation")
	}
	step := result.Steps[0]
	if step.FailureReason != "canceled" || len(step.Attempts) != 1 || step.Attempts[0].Status != "canceled" {
		t.Fatalf("step = %+v, want one canceled attempt", step)
	}
}

func TestRun_OutputExtractionFailureIsNotRetried(t *testing.T) {
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "counter")
	workflowPath := filepath.Join(dir, "workflow.json")
	def, _ := loadWorkflowForRetryTest(t, fmt.Sprintf(`{
		"env": {"COUNTER_PATH": %q},
		"workflows": {
			"main": {"steps": [{
				"name": "ambiguous_mutation",
				"run": "printf x >> \"$COUNTER_PATH\"; printf not-json",
				"retry": {"max_attempts": 3, "delay": "1ms"},
				"outputs": {"VALUE": "$.value"}
			}]}
		}
	}`, counterPath))

	opts := runOpts("main")
	opts.WorkflowFile = workflowPath
	opts.StateDir = filepath.Join(dir, "runs")
	result, err := Run(context.Background(), def, opts)
	if err == nil {
		t.Fatal("expected output extraction failure")
	}
	step := result.Steps[0]
	if step.FailureReason != "output_error" || len(step.Attempts) != 1 || step.Attempts[0].FailureReason != "output_error" {
		t.Fatalf("step = %+v", step)
	}
	if got := readFileOrEmpty(t, counterPath); got != "x" {
		t.Fatalf("successful command was retried after output failure: %q", got)
	}
	if !result.Terminal || result.TerminalReason == "" {
		t.Fatalf("output failure result must be terminal: %+v", result)
	}
	if result.Recoverable || result.Resume != nil {
		t.Fatalf("output failure must not expose automatic resume: %+v", result)
	}

	state, loadErr := loadRunState(result.RunFile)
	if loadErr != nil {
		t.Fatalf("loadRunState: %v", loadErr)
	}
	if state.Status != "terminal" || state.TerminalReason == "" {
		t.Fatalf("output failure state must be terminal: %+v", state)
	}
	if got := state.Steps["main[1]"].FailureReason; got != "output_error" {
		t.Fatalf("persisted failure reason = %q, want output_error", got)
	}

	resumeOpts := runOpts("main")
	resumeOpts.WorkflowFile = workflowPath
	resumeOpts.StateDir = opts.StateDir
	resumeOpts.ResumeRunID = result.RunID
	resumeResult, resumeErr := Run(context.Background(), def, resumeOpts)
	if resumeErr == nil {
		t.Fatal("expected terminal run resume to fail")
	}
	if !strings.Contains(resumeErr.Error(), "cannot be resumed") || !strings.Contains(resumeErr.Error(), "output extraction failed") {
		t.Fatalf("resume error = %q, want terminal output diagnostic", resumeErr)
	}
	if resumeResult == nil || resumeResult.Status != "error" {
		t.Fatalf("resume result = %+v, want structured error", resumeResult)
	}
	if got := readFileOrEmpty(t, counterPath); got != "x" {
		t.Fatalf("resume reran command after output failure: %q", got)
	}
}

func TestRun_OutputExtractionFailureWithoutPolicyIsTerminal(t *testing.T) {
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "counter")
	workflowPath := filepath.Join(dir, "workflow.json")
	def := &Definition{Workflows: map[string]Workflow{
		"main": {Steps: []Step{{
			Name:    "ambiguous_mutation",
			Run:     fmt.Sprintf("printf x >> %q; printf not-json", counterPath),
			Outputs: map[string]string{"VALUE": "$.value"},
		}}},
	}}
	opts := runOpts("main")
	opts.WorkflowFile = workflowPath
	opts.StateDir = filepath.Join(dir, "runs")
	result, err := Run(context.Background(), def, opts)
	if err == nil || !result.Terminal || result.Recoverable || result.Resume != nil {
		t.Fatalf("output failure result = %+v, err = %v", result, err)
	}
	state, loadErr := loadRunState(result.RunFile)
	if loadErr != nil || state.Status != "terminal" || state.Steps["main[1]"].FailureReason != "output_error" {
		t.Fatalf("output failure state = %+v, err = %v", state, loadErr)
	}

	resumeOpts := runOpts("main")
	resumeOpts.WorkflowFile = workflowPath
	resumeOpts.StateDir = opts.StateDir
	resumeOpts.ResumeRunID = result.RunID
	_, resumeErr := Run(context.Background(), def, resumeOpts)
	if resumeErr == nil || !strings.Contains(resumeErr.Error(), "cannot be resumed") {
		t.Fatalf("resume error = %v, want terminal diagnostic", resumeErr)
	}
	if got := readFileOrEmpty(t, counterPath); got != "x" {
		t.Fatalf("resume reran command after output failure: %q", got)
	}
}

func TestRun_OutputExtractionFailureWithoutStateIsTerminal(t *testing.T) {
	def := &Definition{Workflows: map[string]Workflow{
		"main": {Steps: []Step{{
			Name:    "invalid_output",
			Run:     "printf not-json",
			Outputs: map[string]string{"VALUE": "$.value"},
		}}},
	}}
	result, err := Run(context.Background(), def, runOpts("main"))
	if err == nil || !result.Terminal || result.TerminalReason == "" {
		t.Fatalf("output failure result = %+v, err = %v", result, err)
	}
	if result.Recoverable || result.Resume != nil {
		t.Fatalf("terminal output failure exposed resume: %+v", result)
	}
}

func TestRun_ResumeSkipsSuccessAndPreservesAttemptHistory(t *testing.T) {
	dir := t.TempDir()
	firstCounter := filepath.Join(dir, "first-counter")
	secondCounter := filepath.Join(dir, "second-counter")
	allowPath := filepath.Join(dir, "allow")
	workflowPath := filepath.Join(dir, "workflow.json")
	definitionJSON := fmt.Sprintf(`{
		"env": {
			"FIRST_COUNTER": %q,
			"SECOND_COUNTER": %q,
			"ALLOW_PATH": %q
		},
		"workflows": {
			"main": {"steps": [
				{"name":"first","run":"printf x >> \"$FIRST_COUNTER\""},
				{
					"name":"eventual",
					"run":"printf x >> \"$SECOND_COUNTER\"; test -f \"$ALLOW_PATH\"",
					"retry":{"max_attempts":2,"delay":"1ms"}
				}
			]}
		}
	}`, firstCounter, secondCounter, allowPath)
	if err := os.WriteFile(workflowPath, []byte(definitionJSON), 0o600); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	def, err := Load(workflowPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	stateDir := filepath.Join(dir, "runs")
	firstOpts := runOpts("main")
	firstOpts.WorkflowFile = workflowPath
	firstOpts.StateDir = stateDir
	first, firstErr := Run(context.Background(), def, firstOpts)
	if firstErr == nil {
		t.Fatal("expected first run to exhaust retries")
	}
	if got := readFileOrEmpty(t, firstCounter); got != "x" {
		t.Fatalf("first step executions = %q, want x", got)
	}
	if got := readFileOrEmpty(t, secondCounter); got != "xx" {
		t.Fatalf("failed step executions = %q, want xx", got)
	}
	if err := os.WriteFile(allowPath, []byte("ok"), 0o600); err != nil {
		t.Fatalf("write allow marker: %v", err)
	}

	resumeOpts := runOpts("main")
	resumeOpts.WorkflowFile = workflowPath
	resumeOpts.StateDir = stateDir
	resumeOpts.ResumeRunID = first.RunID
	resumed, resumeErr := Run(context.Background(), def, resumeOpts)
	if resumeErr != nil {
		t.Fatalf("resume Run: %v", resumeErr)
	}
	if resumed.Steps[0].Status != "resumed" {
		t.Fatalf("first resumed step = %+v", resumed.Steps[0])
	}
	if got := readFileOrEmpty(t, firstCounter); got != "x" {
		t.Fatalf("successful step re-ran: %q", got)
	}
	if got := readFileOrEmpty(t, secondCounter); got != "xxx" {
		t.Fatalf("failed step resume executions = %q, want xxx", got)
	}
	attempts := resumed.Steps[1].Attempts
	if len(attempts) != 3 || attempts[0].Invocation != 1 || attempts[1].Invocation != 1 || attempts[2].Invocation != 2 || attempts[2].Status != "ok" {
		t.Fatalf("resumed attempt history = %+v", attempts)
	}

	state, loadErr := loadRunState(resumed.RunFile)
	if loadErr != nil {
		t.Fatalf("loadRunState: %v", loadErr)
	}
	if got := len(state.Steps["main[2]"].Attempts); got != 3 {
		t.Fatalf("persisted attempt history length = %d, want 3", got)
	}
}

func TestAttemptResultJSONUsesStableFields(t *testing.T) {
	data, err := json.Marshal(AttemptResult{
		Invocation:    2,
		Attempt:       3,
		Status:        "timeout",
		DurationMS:    25,
		FailureReason: "timeout",
		Error:         "step timed out after 25ms",
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	for _, key := range []string{"invocation", "attempt", "status", "duration_ms", "failure_reason", "error"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("attempt JSON missing %q: %s", key, data)
		}
	}
}

func readFileOrEmpty(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
