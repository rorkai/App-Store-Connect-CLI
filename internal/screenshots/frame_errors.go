package screenshots

import (
	"errors"
	"fmt"
)

// ErrKoubouNotFound matches a failure caused by the Koubou "kou" binary
// being absent from PATH.
var ErrKoubouNotFound = errors.New("kou binary not found")

type koubouNotFoundError struct{}

func (koubouNotFoundError) Error() string {
	return fmt.Sprintf(
		"kou binary not found; install pinned Koubou %s first (%s)",
		pinnedKoubouVersion,
		pinnedKoubouInstallCommand(),
	)
}

func (koubouNotFoundError) Is(target error) bool { return target == ErrKoubouNotFound }

func errKoubouNotFound() error { return koubouNotFoundError{} }

// FrameInput identifies a caller-supplied frame input.
type FrameInput string

const (
	FrameInputScreenshot FrameInput = "screenshot"
	FrameInputConfig     FrameInput = "config"
)

// FrameInputError reports that a caller-supplied frame input could not be
// read. Its message is the wrapped error's message.
type FrameInputError struct {
	Input FrameInput
	Err   error
}

func (e *FrameInputError) Error() string { return e.Err.Error() }
func (e *FrameInputError) Unwrap() error { return e.Err }

func newFrameInputError(input FrameInput, err error) error {
	return &FrameInputError{Input: input, Err: err}
}
