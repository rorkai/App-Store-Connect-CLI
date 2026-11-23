package asc

import "errors"

// UploadPhase names the step of a media upload that was running when it
// stopped: reserving the asset, transferring its bytes to Apple's upload
// hosts, committing the transfer, or waiting for Apple to process it.
type UploadPhase string

const (
	UploadPhaseReserve    UploadPhase = "reserve"
	UploadPhaseTransfer   UploadPhase = "transfer"
	UploadPhaseCommit     UploadPhase = "commit"
	UploadPhaseProcessing UploadPhase = "processing"
)

// uploadPhaseError marks err with the upload step it came from without
// changing its text.
type uploadPhaseError struct {
	phase UploadPhase
	err   error
}

func (e *uploadPhaseError) Error() string { return e.err.Error() }

func (e *uploadPhaseError) Unwrap() error { return e.err }

// WithUploadPhase marks err as a failure in phase. A nil err stays nil.
func WithUploadPhase(phase UploadPhase, err error) error {
	if err == nil {
		return nil
	}
	return &uploadPhaseError{phase: phase, err: err}
}

// UploadPhaseOf returns the upload step err came from, or "" when err is not
// marked with one.
func UploadPhaseOf(err error) UploadPhase {
	if marked, ok := errors.AsType[*uploadPhaseError](err); ok {
		return marked.phase
	}
	return ""
}
