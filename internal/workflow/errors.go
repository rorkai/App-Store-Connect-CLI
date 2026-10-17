package workflow

import (
	"context"
	"errors"
)

var (
	// ErrUnknownWorkflow indicates a workflow name that the definition does not declare.
	ErrUnknownWorkflow = errors.New("unknown workflow")
	// ErrPrivateWorkflow indicates a private workflow requested as the entry point.
	ErrPrivateWorkflow = errors.New("private workflow")
	// ErrResumeRunNotFound indicates a --resume run ID with no persisted state.
	ErrResumeRunNotFound = errors.New("resume run not found")
	// ErrResumeRejected indicates persisted run state that cannot be resumed
	// with the requested workflow, file, definition, or parameters.
	ErrResumeRejected = errors.New("resume run rejected")
	// ErrCommandFailed indicates a user-defined run step or hook command that
	// did not succeed, including a step that exceeded its workflow-defined
	// timeout or whose declared outputs were invalid.
	ErrCommandFailed = errors.New("workflow command failed")
)

// markedError keeps err's message while also matching kind with errors.Is.
type markedError struct {
	err  error
	kind error
}

func (e markedError) Error() string   { return e.err.Error() }
func (e markedError) Unwrap() []error { return []error{e.err, e.kind} }

func markError(err, kind error) error {
	if err == nil || kind == nil {
		return err
	}
	return markedError{err: err, kind: kind}
}

// markCommandError classifies a failed user command. A canceled or expired
// caller context takes precedence over the command's own failure.
func markCommandError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return markError(err, ctxErr)
	}
	return markError(err, ErrCommandFailed)
}
