package workflow

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

type stepExecutionPolicy struct {
	maxAttempts int
	delay       time.Duration
	timeout     time.Duration
	configured  bool
}

func executionPolicyForStep(step Step) (stepExecutionPolicy, error) {
	policy := stepExecutionPolicy{maxAttempts: 1}
	if step.Retry != nil {
		if step.Retry.MaxAttempts < minRetryAttempts || step.Retry.MaxAttempts > maxRetryAttempts {
			return policy, fmt.Errorf("retry.max_attempts must be between %d and %d", minRetryAttempts, maxRetryAttempts)
		}
		delay, err := parsePositivePolicyDuration(step.Retry.Delay)
		if err != nil {
			return policy, fmt.Errorf("retry.delay: %w", err)
		}
		policy.maxAttempts = step.Retry.MaxAttempts
		policy.delay = delay
		policy.configured = true
	}
	if step.Timeout != nil {
		timeout, err := parsePositivePolicyDuration(*step.Timeout)
		if err != nil {
			return policy, fmt.Errorf("timeout: %w", err)
		}
		policy.timeout = timeout
		policy.configured = true
	}
	return policy, nil
}

func (r *runner) executeRunStep(
	ctx context.Context,
	workflowName string,
	idx int,
	stepKey string,
	step Step,
	command string,
	env map[string]string,
	sr *StepResult,
) error {
	policy, err := executionPolicyForStep(step)
	if err != nil {
		sr.Status = "error"
		sr.FailureReason = "invalid_policy"
		sr.Error = err.Error()
		return fmt.Errorf("workflow: %s step %d: %w", workflowName, idx, err)
	}

	if r.state != nil {
		if persisted, ok := r.state.Steps[stepKey]; ok && persisted.Status != "ok" {
			sr.Attempts = cloneAttemptResults(persisted.Attempts)
		}
	}
	invocation := nextAttemptInvocation(sr.Attempts)
	recordAttempts := policy.configured || len(sr.Attempts) > 0
	label := failedStepName(step.Name, stepKey)

	for attempt := 1; attempt <= policy.maxAttempts; attempt++ {
		if recordAttempts {
			fmt.Fprintf(r.opts.Stderr, "[workflow] step %s: attempt %d/%d\n", label, attempt, policy.maxAttempts)
		}

		stdout := r.opts.Stdout
		var captured bytes.Buffer
		if len(step.Outputs) > 0 {
			stdout = io.MultiWriter(r.opts.Stdout, &captured)
		}

		attemptCtx := ctx
		cancel := func() {}
		if policy.timeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, policy.timeout)
		}
		attemptStart := time.Now()
		runErr := runShellCommand(attemptCtx, command, env, stdout, r.opts.Stderr)
		attemptDuration := time.Since(attemptStart).Milliseconds()
		attemptContextErr := attemptCtx.Err()
		cancel()

		if runErr == nil {
			if len(step.Outputs) > 0 {
				extracted, extractErr := extractDeclaredOutputs(step.Outputs, captured.Bytes())
				if extractErr != nil {
					sr.Attempts = append(sr.Attempts, AttemptResult{
						Invocation:    invocation,
						Attempt:       attempt,
						Status:        "error",
						DurationMS:    attemptDuration,
						FailureReason: "output_error",
						Error:         extractErr.Error(),
					})
					sr.Status = "error"
					sr.FailureReason = "output_error"
					sr.Error = extractErr.Error()
					if persistErr := r.persistStep(stepKey, *sr, step.Retry != nil); persistErr != nil {
						return persistErr
					}
					return markError(fmt.Errorf("workflow: %s step %d: %w", workflowName, idx, extractErr), ErrCommandFailed)
				}
				sr.Outputs = extracted
				if strings.TrimSpace(step.Name) != "" {
					r.outputs[step.Name] = cloneStringMap(extracted)
					r.result.Outputs = cloneNestedStringMap(r.outputs)
				}
			}

			if recordAttempts {
				sr.Attempts = append(sr.Attempts, AttemptResult{
					Invocation: invocation,
					Attempt:    attempt,
					Status:     "ok",
					DurationMS: attemptDuration,
				})
			}
			sr.Status = "ok"
			sr.FailureReason = ""
			sr.Error = ""
			return r.persistStep(stepKey, *sr, step.Retry != nil)
		}

		failureReason, attemptError := classifyAttemptFailure(ctx, attemptContextErr, runErr, policy.timeout)
		if recordAttempts {
			sr.Attempts = append(sr.Attempts, AttemptResult{
				Invocation:    invocation,
				Attempt:       attempt,
				Status:        attemptStatus(failureReason),
				DurationMS:    attemptDuration,
				FailureReason: failureReason,
				Error:         attemptError,
			})
		}
		sr.Status = "error"
		sr.FailureReason = failureReason
		sr.Error = attemptError
		if failureReason == "timeout" && step.Retry == nil {
			r.setTerminalReason(terminalTimeoutReason(label))
		}

		if failureReason == "canceled" || attempt == policy.maxAttempts {
			if recordAttempts {
				if persistErr := r.persistStep(stepKey, *sr, step.Retry != nil); persistErr != nil {
					return persistErr
				}
			}
			return attemptFailureError(ctx, fmt.Errorf("workflow: %s step %d: %s", workflowName, idx, attemptError), failureReason)
		}

		sr.Status = "retrying"
		if persistErr := r.persistStep(stepKey, *sr, step.Retry != nil); persistErr != nil {
			return persistErr
		}
		fmt.Fprintf(
			r.opts.Stderr,
			"[workflow] step %s: attempt %d/%d failed (%s); retrying in %s\n",
			label,
			attempt,
			policy.maxAttempts,
			failureReason,
			policy.delay,
		)
		if waitErr := waitForRetry(ctx, policy.delay); waitErr != nil {
			sr.Status = "error"
			sr.FailureReason = "canceled"
			sr.Error = "step canceled during retry delay: " + waitErr.Error()
			if persistErr := r.persistStep(stepKey, *sr, step.Retry != nil); persistErr != nil {
				return persistErr
			}
			return markError(fmt.Errorf("workflow: %s step %d: %s", workflowName, idx, sr.Error), waitErr)
		}
	}

	return fmt.Errorf("workflow: %s step %d: retry policy exhausted", workflowName, idx)
}

func nextAttemptInvocation(attempts []AttemptResult) int {
	invocation := 1
	for _, attempt := range attempts {
		if attempt.Invocation >= invocation {
			invocation = attempt.Invocation + 1
		}
	}
	return invocation
}

func classifyAttemptFailure(ctx context.Context, attemptContextErr, runErr error, timeout time.Duration) (string, string) {
	if ctx.Err() != nil {
		return "canceled", "step canceled: " + ctx.Err().Error()
	}
	if errors.Is(attemptContextErr, context.DeadlineExceeded) {
		return "timeout", "step timed out after " + timeout.String()
	}
	return "command_failed", runErr.Error()
}

// attemptFailureError keeps the rendered step error and records why the final
// attempt failed: a canceled run matches the caller's context error, and a
// failed command or a command that exceeded its workflow-defined timeout
// matches ErrCommandFailed. A step timeout deliberately does not match
// context.DeadlineExceeded: that would attach the ASC_TIMEOUT request hint,
// which does not control the step's deadline.
func attemptFailureError(ctx context.Context, err error, failureReason string) error {
	switch failureReason {
	case "canceled":
		return markError(err, ctx.Err())
	case "command_failed", "timeout":
		return markError(err, ErrCommandFailed)
	default:
		return err
	}
}

func attemptStatus(failureReason string) string {
	switch failureReason {
	case "timeout", "canceled":
		return failureReason
	default:
		return "error"
	}
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
