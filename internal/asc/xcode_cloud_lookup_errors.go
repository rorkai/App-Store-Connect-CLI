package asc

import (
	"errors"
	"fmt"
)

var (
	// ErrCiProductNotFound indicates an app without an Xcode Cloud product,
	// which is the normal state for apps that do not use Xcode Cloud.
	ErrCiProductNotFound = errors.New("xcode cloud product not found")
	// ErrCiWorkflowNotFound indicates no Xcode Cloud workflow matched a name.
	ErrCiWorkflowNotFound = errors.New("xcode cloud workflow not found")
	// ErrCiWorkflowAmbiguous indicates more than one workflow matched a name.
	ErrCiWorkflowAmbiguous = errors.New("xcode cloud workflow name is ambiguous")
	// ErrGitReferenceNotFound indicates no git reference matched a name.
	ErrGitReferenceNotFound = errors.New("git reference not found")
	// ErrGitReferenceAmbiguous indicates more than one git reference matched a name.
	ErrGitReferenceAmbiguous = errors.New("git reference name is ambiguous")
)

// xcodeCloudLookupError renders its own message while matching kind with
// errors.Is, so callers can classify a lookup without changing its text.
type xcodeCloudLookupError struct {
	message string
	kind    error
}

func (e *xcodeCloudLookupError) Error() string { return e.message }
func (e *xcodeCloudLookupError) Unwrap() error { return e.kind }

func newXcodeCloudLookupError(kind error, format string, args ...any) error {
	return &xcodeCloudLookupError{message: fmt.Sprintf(format, args...), kind: kind}
}
