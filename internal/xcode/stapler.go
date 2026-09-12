package xcode

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
)

const staplerResolutionOutputLimit = 4 * 1024

// afterStaplerCommandWaitFn is a narrow test seam for cancellation that lands
// after a stapler child has returned from Wait but before its result is
// formatted. Production leaves it nil.
var afterStaplerCommandWaitFn func()

// afterStaplerCommandStartFn is a narrow test seam for cancellation that lands
// after a stapler child has started but before Wait observes its result.
// Production leaves it nil.
var afterStaplerCommandStartFn func(*exec.Cmd)

// beforeStaplerCommandCancelFn is a narrow test seam for controlling the
// command cancellation callback before the runner wraps it for tracking.
// Production leaves it nil.
var beforeStaplerCommandCancelFn func(*exec.Cmd)

// afterStaplerResolutionFn is a narrow test seam for cancellation that lands
// after the xcrun stapler lookup has returned its process result but before
// ensureStaplerAvailable observes the caller's context. Production leaves it
// nil.
var afterStaplerResolutionFn func()

// beforeStaplerResolutionRunFn is a narrow test seam for a resolver process
// that exits while cancellation cleanup is racing with Wait. Production leaves
// it nil.
var beforeStaplerResolutionRunFn func(*exec.Cmd)

// StaplerOperation identifies the local ticket operation that was requested.
type StaplerOperation string

const (
	StaplerOperationResolve  StaplerOperation = "resolve"
	StaplerOperationStaple   StaplerOperation = "staple"
	StaplerOperationValidate StaplerOperation = "validate"
)

// StaplerResult is the result of a successful local ticket operation.
type StaplerResult struct {
	Path      string
	Operation string
	Stapled   bool
	Validated bool
}

// StaplerStageVerifier checks the artifact immediately before and after each
// stapler operation. It is used by callers that pin the target identity during
// the multi-stage staple flow. A true before value identifies the pre-stage
// check; false identifies the post-stage check.
type StaplerStageVerifier func(operation StaplerOperation, before bool) error

// StaplerStageVerificationError identifies a verifier failure at one child
// operation boundary. The wrapped error remains available to callers that need
// to classify the underlying cause, while Error provides a stable diagnostic
// that does not include verifier details.
type StaplerStageVerificationError struct {
	Operation StaplerOperation
	Before    bool
	Err       error
}

func (e *StaplerStageVerificationError) Error() string {
	if e == nil {
		return "stapler stage verification failed"
	}
	position := "after"
	if e.Before {
		position = "before"
	}
	return fmt.Sprintf("stapler %s verification failed %s operation", e.Operation, position)
}

func (e *StaplerStageVerificationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// StaplerCommandError preserves the operation and child exit status for a
// failed stapler invocation. ExitCode is -1 when no ordinary process status is
// available, such as a start failure or signal termination.
type StaplerCommandError struct {
	Operation string
	ExitCode  int
	Err       error
}

func (e *StaplerCommandError) Error() string {
	if e == nil {
		return "stapler command failed"
	}
	if e.Err == nil {
		return fmt.Sprintf("stapler %s failed", e.Operation)
	}
	return fmt.Sprintf("stapler %s failed: %v", e.Operation, e.Err)
}

func (e *StaplerCommandError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// StaplerResolutionError marks an operational failure while locating the
// local stapler tool. Its public text is intentionally closed so a platform
// lookup error cannot disclose paths or other host details; the original
// lookup cause remains available to internal callers through Unwrap.
type StaplerResolutionError struct {
	Err error
}

func (e *StaplerResolutionError) Error() string {
	return "stapler tool resolution failed"
}

func (e *StaplerResolutionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// StaplerPartialMutationError identifies either an interrupted staple child or
// a follow-up validation failure after stapling. Its stable message warns that
// the artifact may have been modified while Unwrap retains the cancellation or
// child error for internal classification.
type StaplerPartialMutationError struct {
	Operation   StaplerOperation
	Interrupted bool
	Err         error
}

// ErrStaplerDiagnosticOutput marks a failure to copy a stapler child's output
// to the caller's diagnostic writer after the child itself reported success.
// Command layers match it so they can report a completed-but-unverified staple
// instead of claiming that a follow-up validation ran and failed.
var ErrStaplerDiagnosticOutput = errors.New("stapler diagnostic output could not be copied")

// staplerDiagnosticOutputError records a failure copying diagnostics after a
// stapler child was started. A successful child may already have changed the
// artifact before its output becomes unwriteable, so the staple caller must
// preserve the partial-mutation warning. The public marker is stable while the
// original writer/process cause remains available through Unwrap.
type staplerDiagnosticOutputError struct {
	err error
}

func (e *staplerDiagnosticOutputError) Error() string {
	return ErrStaplerDiagnosticOutput.Error()
}

// Is exposes the stable public marker without widening the private type. Only
// the sentinel matches; the original writer/process cause stays behind Unwrap.
func (e *staplerDiagnosticOutputError) Is(target error) bool {
	return target == ErrStaplerDiagnosticOutput
}

func (e *staplerDiagnosticOutputError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func isStaplerDiagnosticOutputError(err error) bool {
	var outputErr *staplerDiagnosticOutputError
	return errors.As(err, &outputErr)
}

func (e *StaplerPartialMutationError) Error() string {
	if e == nil {
		return "stapler follow-up validation failed after staple; artifact may have been modified but was not verified"
	}
	if e.Interrupted {
		return "stapler staple was interrupted; artifact may have been modified but was not verified"
	}
	if e.Operation == StaplerOperationStaple {
		return "stapler post-staple verification failed; artifact may have been modified but was not verified"
	}
	return "stapler " + string(e.Operation) + " failed after staple; artifact may have been modified but was not verified"
}

func (e *StaplerPartialMutationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Staple retrieves and attaches a ticket, then validates the same artifact.
// The artifact path must have been validated by the command layer before this
// local runner is called.
func Staple(ctx context.Context, path string, logWriter io.Writer) (*StaplerResult, error) {
	return StapleWithVerifier(ctx, path, logWriter, nil)
}

// StapleWithVerifier retrieves and attaches a ticket, then validates the same
// artifact. When verifier is non-nil, it runs immediately before and after
// both child operations so callers can reject a replaced target between the
// staple and validation stages.
func StapleWithVerifier(ctx context.Context, path string, logWriter io.Writer, verifier StaplerStageVerifier) (*StaplerResult, error) {
	if err := ensureStaplerAvailable(ctx); err != nil {
		return nil, err
	}
	if err := verifyStaplerStage(verifier, StaplerOperationStaple, true); err != nil {
		return nil, err
	}
	stapleErr := runStaplerOperation(ctx, StaplerOperationStaple, path, logWriter)
	// A child-start failure or a context error without an attempted-child marker
	// means the staple child never started. Still run the stage verifier
	// so callers can report a concurrent target replacement, but do not claim
	// that stapling completed or may have modified the artifact.
	if stapleErr != nil && isStaplerOperationNotStarted(stapleErr) {
		if verifyErr := verifyStaplerStage(verifier, StaplerOperationStaple, false); verifyErr != nil {
			return nil, errors.Join(stapleErr, verifyErr)
		}
		return nil, stapleErr
	}
	if stapleErr != nil && !isStaplerOperationAttempted(stapleErr) &&
		(errors.Is(stapleErr, context.Canceled) || errors.Is(stapleErr, context.DeadlineExceeded)) {
		if verifyErr := verifyStaplerStage(verifier, StaplerOperationStaple, false); verifyErr != nil {
			return nil, errors.Join(stapleErr, verifyErr)
		}
		return nil, stapleErr
	}
	if verifyErr := verifyStaplerStage(verifier, StaplerOperationStaple, false); verifyErr != nil {
		partialErr := error(verifyErr)
		if stapleErr != nil {
			partialErr = errors.Join(stapleErr, verifyErr)
		}
		return nil, &StaplerPartialMutationError{
			Operation:   StaplerOperationStaple,
			Interrupted: isStaplerOperationAttemptedCancellation(stapleErr) || isStaplerOperationAttemptedSignal(stapleErr),
			Err:         partialErr,
		}
	}
	if stapleErr != nil {
		// Every failure that reaches this point comes from a staple child that
		// was started, because a child that never started returned above. A
		// started child can fail after it has already written part of the
		// artifact, and the post-stage verifier recaptures the resulting state as
		// the next baseline instead of comparing it with the pre-staple evidence,
		// so a successful post-stage check cannot prove the target is untouched.
		// Keep the unverified-mutation warning while preserving the child's
		// status and cause for the caller.
		return nil, &StaplerPartialMutationError{
			Operation:   StaplerOperationStaple,
			Interrupted: isStaplerOperationAttemptedCancellation(stapleErr) || isStaplerOperationAttemptedSignal(stapleErr),
			Err:         stapleErr,
		}
	}
	if err := verifyStaplerStage(verifier, StaplerOperationValidate, true); err != nil {
		return nil, &StaplerPartialMutationError{
			Operation: StaplerOperationStaple,
			Err:       err,
		}
	}
	validateErr := runStaplerOperation(ctx, StaplerOperationValidate, path, logWriter)
	if verifyErr := verifyStaplerStage(verifier, StaplerOperationValidate, false); verifyErr != nil {
		partialErr := error(verifyErr)
		if validateErr != nil {
			partialErr = errors.Join(validateErr, verifyErr)
		}
		return nil, &StaplerPartialMutationError{
			Operation: StaplerOperationValidate,
			Err:       partialErr,
		}
	}
	if validateErr != nil {
		if isStaplerDiagnosticOutputError(validateErr) {
			// The validation child itself succeeded; only copying its output to
			// the caller's diagnostic writer failed. The artifact is stapled and
			// verified, so preserve the writer failure without the partial
			// mutation claim that tells callers the ticket went unverified.
			return nil, validateErr
		}
		return nil, &StaplerPartialMutationError{
			Operation: StaplerOperationValidate,
			Err:       validateErr,
		}
	}
	return &StaplerResult{
		Path:      path,
		Operation: string(StaplerOperationStaple),
		Stapled:   true,
		Validated: true,
	}, nil
}

func verifyStaplerStage(verifier StaplerStageVerifier, operation StaplerOperation, before bool) error {
	if verifier == nil {
		return nil
	}
	if err := verifier(operation, before); err != nil {
		return &StaplerStageVerificationError{
			Operation: operation,
			Before:    before,
			Err:       err,
		}
	}
	return nil
}

// ValidateStaple validates an already stapled artifact without modifying it.
// The artifact path must have been validated by the command layer before this
// local runner is called.
func ValidateStaple(ctx context.Context, path string, logWriter io.Writer) (*StaplerResult, error) {
	return ValidateWithVerifier(ctx, path, logWriter, nil)
}

// ValidateWithVerifier validates an already stapled artifact without
// modifying it. When verifier is non-nil, it runs immediately before and after
// the validation child process, after stapler resolution has completed.
func ValidateWithVerifier(ctx context.Context, path string, logWriter io.Writer, verifier StaplerStageVerifier) (*StaplerResult, error) {
	if err := ensureStaplerAvailable(ctx); err != nil {
		return nil, err
	}
	if err := verifyStaplerStage(verifier, StaplerOperationValidate, true); err != nil {
		return nil, err
	}
	validateErr := runStaplerOperation(ctx, StaplerOperationValidate, path, logWriter)
	if verifyErr := verifyStaplerStage(verifier, StaplerOperationValidate, false); verifyErr != nil {
		if validateErr != nil {
			return nil, errors.Join(validateErr, verifyErr)
		}
		return nil, verifyErr
	}
	if validateErr != nil {
		return nil, validateErr
	}
	return &StaplerResult{
		Path:      path,
		Operation: string(StaplerOperationValidate),
		Validated: true,
	}, nil
}

func ensureStaplerAvailable(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	if runtimeGOOS != "darwin" {
		return fmt.Errorf("stapler is supported on macOS only; current platform is %s", runtimeGOOS)
	}
	xcrunPath, err := trustedXcrunPathFn()
	if err != nil {
		if isTrustedXcodeToolUnavailable(err, "xcrun") {
			return fmt.Errorf("xcrun not available; install Xcode and ensure the active developer directory is configured")
		}
		return &StaplerResolutionError{Err: err}
	}

	cmd := commandContextFn(ctx, xcrunPath, "--find", "stapler")
	var contextCancelSucceeded atomic.Bool
	if cancel := cmd.Cancel; cancel != nil {
		cmd.Cancel = func() error {
			err := cancel()
			if err == nil {
				contextCancelSucceeded.Store(true)
			}
			return err
		}
	}
	stdout := newTailBuffer(staplerResolutionOutputLimit)
	// Resolver diagnostics may contain the selected developer directory or
	// other host paths. Keep them available to the bounded error formatter, but
	// never stream them to the caller's diagnostic writer before the resolver
	// result has been classified.
	stderr := newXcodeDiagnosticBuffer(staplerResolutionOutputLimit, nil)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if beforeStaplerResolutionRunFn != nil {
		beforeStaplerResolutionRunFn(cmd)
	}
	if err := runXcodeCommand(cmd); err != nil {
		if afterStaplerResolutionFn != nil {
			afterStaplerResolutionFn()
		}
		failure := error(fmt.Errorf("xcrun --find stapler failed: %w", err))
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			failure = fmt.Errorf("xcrun --find stapler failed: %s: %w", detail, err)
		}
		commandErr := newStaplerCommandError(StaplerOperationResolve, failure)
		if ctxErr := ctx.Err(); ctxErr != nil {
			if staplerHasProcessExitStatus(err) {
				// Preserve a concrete resolver exit status when cancellation becomes
				// visible in the same late-result window. A cancellation callback can
				// report success for an already-exited process, so status wins over
				// that callback marker.
				return errors.Join(ctxErr, commandErr)
			}
			if contextCancelSucceeded.Load() {
				return &staplerOperationAttemptedCancellationError{
					err: errors.Join(commandErr, ctxErr),
				}
			}
			if staplerProcessWasSignaled(err) {
				// A signal without an ordinary exit code remains a meaningful
				// process result when cancellation did not terminate the child.
				return errors.Join(ctxErr, commandErr)
			}
			if !errors.Is(err, ctxErr) {
				// The resolver failed to launch or wait with a concrete operational
				// cause, such as a descriptor limit, and cancellation only became
				// visible afterwards. Keep the resolution classification alongside
				// the late cancellation instead of reporting cancellation alone. A
				// command that was already canceled before the child started stays
				// an ordinary context failure.
				return errors.Join(ctxErr, commandErr)
			}
			return ctxErr
		}
		return commandErr
	}
	pathValue, err := parseTrustedToolPath(stdout.String(), "stapler")
	if err != nil {
		return &StaplerResolutionError{Err: err}
	}
	if _, err := validateResolvedStaplerPathForOperation(ctx, pathValue); err != nil {
		return &StaplerResolutionError{Err: err}
	}
	return nil
}

func validateResolvedStaplerPathForOperation(ctx context.Context, pathValue string) (string, error) {
	if filepath.Clean(pathValue) == trustedStaplerPath {
		return trustedStaplerPath, nil
	}
	developerDir, err := trustedDeveloperDirectory(ctx, nil)
	if err != nil {
		return "", err
	}
	developerDir, _, _, err = normalizeToolchainDeveloperDir(developerDir)
	if err != nil {
		return "", err
	}
	return validateResolvedStaplerPath(pathValue, developerDir)
}

// staplerOperationAttemptedCancellationError records that a stapler child was
// invoked before its context was canceled. A cancellation before the child is
// started remains an ordinary preflight error; once staple has been attempted,
// the caller must warn that the artifact may have been modified.
type staplerOperationAttemptedCancellationError struct {
	err error
}

// staplerOperationAttemptedError records that a stapler child ran and
// returned a concrete process status. It deliberately does not carry the
// cancellation marker: a real child result must remain an ordinary process
// failure even when context cancellation raced with command cleanup.
type staplerOperationAttemptedError struct {
	err error
}

func (e *staplerOperationAttemptedError) Error() string {
	return "stapler operation failed after child invocation"
}

func (e *staplerOperationAttemptedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// staplerOperationNotStartedError records that constructing or starting the
// child failed before it could run. A post-stage verifier error joined with
// this marker must remain an ordinary failure: no child ran and therefore no
// partial mutation is possible.
type staplerOperationNotStartedError struct {
	err error
}

// staplerOperationStartError retains a concrete failure from cmd.Start. The
// contextOnly bit distinguishes a pre-canceled command from an operational
// start failure that raced with cancellation while its diagnostic was being
// formatted.
type staplerOperationStartError struct {
	err         error
	contextOnly bool
}

func (e *staplerOperationStartError) Error() string {
	if e == nil || e.err == nil {
		return "stapler operation start failed"
	}
	return e.err.Error()
}

func (e *staplerOperationStartError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *staplerOperationNotStartedError) Error() string {
	return "stapler operation did not start"
}

func (e *staplerOperationNotStartedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func isStaplerOperationNotStarted(err error) bool {
	var notStarted *staplerOperationNotStartedError
	return errors.As(err, &notStarted)
}

func (e *staplerOperationAttemptedCancellationError) Error() string {
	return "stapler operation canceled after child invocation"
}

func (e *staplerOperationAttemptedCancellationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func isStaplerOperationAttemptedCancellation(err error) bool {
	var attempted *staplerOperationAttemptedCancellationError
	return errors.As(err, &attempted)
}

func isStaplerOperationAttempted(err error) bool {
	var attempted *staplerOperationAttemptedError
	if errors.As(err, &attempted) {
		return true
	}
	return isStaplerOperationAttemptedCancellation(err) || isStaplerOperationAttemptedSignal(err)
}

// IsStaplerOperationAttemptedCancellation reports whether a started stapler
// child was terminated through its context cancellation path. Callers can use
// this marker before inspecting a nested StaplerCommandError, whose signal
// status alone cannot distinguish a context kill from an independent signal.
func IsStaplerOperationAttemptedCancellation(err error) bool {
	return isStaplerOperationAttemptedCancellation(err)
}

// staplerOperationAttemptedSignalError records that the child was started and
// then terminated by a signal. A signaled staple may have modified the target
// before termination, so it must take the same partial-mutation path as an
// in-flight cancellation. The wrapped command error remains available to
// callers that need the process status or underlying *exec.ExitError.
type staplerOperationAttemptedSignalError struct {
	err error
}

func (e *staplerOperationAttemptedSignalError) Error() string {
	return "stapler operation terminated by signal after child invocation"
}

func (e *staplerOperationAttemptedSignalError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func isStaplerOperationAttemptedSignal(err error) bool {
	var attempted *staplerOperationAttemptedSignalError
	return errors.As(err, &attempted)
}

func staplerProcessWasSignaled(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr == nil || exitErr.ProcessState == nil {
		return false
	}
	status, ok := exitErr.Sys().(interface{ Signaled() bool })
	return ok && status.Signaled()
}

func runStaplerOperation(ctx context.Context, operation StaplerOperation, path string, logWriter io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	started, contextCancelSucceeded, err := runStaplerChildCommand(ctx, operation, path, logWriter)
	if err == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		if !started {
			var startErr *staplerOperationStartError
			if errors.As(err, &startErr) && !startErr.contextOnly {
				return &staplerOperationNotStartedError{
					err: errors.Join(newStaplerCommandError(operation, err), ctxErr),
				}
			}
			return ctxErr
		}
		if staplerHasProcessExitStatus(err) {
			// Wait returned a concrete child result. Keep it as a normal process
			// failure even when CommandContext's cancellation callback reported
			// success; only a signal/no-status result is attributable to context
			// cancellation here.
			return &staplerOperationAttemptedError{
				err: errors.Join(newStaplerCommandError(operation, err), ctxErr),
			}
		}
		if contextCancelSucceeded {
			// CommandContext's cancellation callback successfully attempted to
			// stop the child. Preserve the cancellation classification even when
			// the resulting process status is a signal, which otherwise could be
			// indistinguishable from an independent signal observed late.
			return &staplerOperationAttemptedCancellationError{
				err: errors.Join(newStaplerCommandError(operation, err), ctxErr),
			}
		}
		if staplerProcessWasSignaled(err) {
			// A signaled child has no ordinary exit code, but its process result
			// is still meaningful. Preserve the signal and cancellation together
			// so callers retain the partial-mutation classification and cause.
			return &staplerOperationAttemptedSignalError{
				err: errors.Join(newStaplerCommandError(operation, err), ctxErr),
			}
		}
		if isStaplerDiagnosticOutputError(err) {
			// A successful child can still leave a diagnostic-copy error when its
			// output sink fails. Preserve that concrete cause alongside the late
			// cancellation so staple callers retain the partial-mutation warning
			// and internal errors.Is/As classification.
			return &staplerOperationAttemptedCancellationError{
				err: errors.Join(err, ctxErr),
			}
		}
		return &staplerOperationAttemptedCancellationError{err: ctxErr}
	}
	commandErr := newStaplerCommandError(operation, err)
	if !started {
		return &staplerOperationNotStartedError{err: commandErr}
	}
	if staplerProcessWasSignaled(err) {
		return &staplerOperationAttemptedSignalError{err: commandErr}
	}
	return commandErr
}

func runStaplerChildCommand(ctx context.Context, operation StaplerOperation, path string, logWriter io.Writer) (bool, bool, error) {
	cmd, err := trustedXcodeCommand(ctx, "xcrun", []string{"stapler", string(operation), path}, nil)
	if err != nil {
		return false, false, err
	}
	if beforeStaplerCommandCancelFn != nil {
		beforeStaplerCommandCancelFn(cmd)
	}
	var contextCancelSucceeded atomic.Bool
	if cancel := cmd.Cancel; cancel != nil {
		cmd.Cancel = func() error {
			err := cancel()
			if err == nil {
				contextCancelSucceeded.Store(true)
			}
			return err
		}
	}
	outputWindow := newXcodeDiagnosticBuffer(xcodebuildErrorTailLimit, logWriter)
	cmd.Stdout = outputWindow
	cmd.Stderr = outputWindow
	cmd.WaitDelay = xcodeCommandPipeWaitDelay
	if err := cmd.Start(); err != nil {
		// A start failure can race with cancellation before the formatter reads
		// the context. Preserve the concrete failure when that happens; a
		// command that was already canceled before Start remains context-only.
		contextErr := ctx.Err()
		contextOnly := contextErr != nil && errors.Is(err, contextErr)
		// Format against a non-canceling view even when the first context check
		// succeeds. Cancellation can arrive in the next instruction, and the
		// formatter otherwise replaces the concrete Start error with ctx.Err().
		formatted := formatCommandOutputError(context.WithoutCancel(ctx), err, outputWindow, string(operation), "xcrun stapler", true)
		if !contextOnly {
			if lateContextErr := ctx.Err(); lateContextErr != nil {
				formatted = errors.Join(formatted, lateContextErr)
			}
		}
		return false, false, &staplerOperationStartError{err: formatted, contextOnly: contextOnly}
	}
	if afterStaplerCommandStartFn != nil {
		afterStaplerCommandStartFn(cmd)
	}
	waitErr := normalizeXcodeCommandWaitError(cmd, cmd.Wait())
	if afterStaplerCommandWaitFn != nil {
		afterStaplerCommandWaitFn()
	}
	if waitErr != nil {
		formatContext := ctx
		if ctx.Err() != nil && (staplerHasProcessExitStatus(waitErr) || staplerProcessWasSignaled(waitErr) ||
			(cmd.ProcessState != nil && cmd.ProcessState.Success())) {
			// formatCommandOutputError intentionally prefers context errors. Once
			// Wait has returned a process result, use a non-canceling view so that
			// its status, signal, or diagnostic-copy failure remains wrapped for
			// runStaplerOperation to preserve.
			formatContext = context.WithoutCancel(ctx)
		}
		formatted := formatCommandOutputError(formatContext, waitErr, outputWindow, string(operation), "xcrun stapler", true)
		if cmd.ProcessState != nil && cmd.ProcessState.Success() {
			if ctxErr := ctx.Err(); ctxErr != nil {
				formatted = errors.Join(formatted, ctxErr)
			}
			return true, contextCancelSucceeded.Load(), &staplerDiagnosticOutputError{err: formatted}
		}
		return true, contextCancelSucceeded.Load(), formatted
	}
	return true, contextCancelSucceeded.Load(), nil
}

func staplerHasProcessExitStatus(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr != nil && exitErr.ExitCode() >= 0
}

func newStaplerCommandError(operation StaplerOperation, err error) *StaplerCommandError {
	exitCode := -1
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	}
	return &StaplerCommandError{
		Operation: string(operation),
		ExitCode:  exitCode,
		Err:       err,
	}
}
